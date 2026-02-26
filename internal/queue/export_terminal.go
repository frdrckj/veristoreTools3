package queue

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/hibiken/asynq"
	"github.com/rs/zerolog/log"
	"github.com/xuri/excelize/v2"
	"gorm.io/gorm"

	"github.com/verifone/veristoretools3/internal/admin"
	"github.com/verifone/veristoretools3/internal/tms"
)

// Number of concurrent workers for fetching terminal data from TMS.
// Each worker makes 3 + N API calls per terminal where N = number of tabs:
//   - getTerminalIdFromSN (1 call)
//   - terminal/detail (1 call)
//   - terminalApp/list (1 call)
//   - N × terminalAppParameter/view (concurrent, 1 per tab)
// operationMark is cached once for all terminals (not per-terminal).
// Tab calls within each worker are concurrent, so total connections ≈
// exportWorkerCount × N. Keep this low to avoid overwhelming TMS.
const exportWorkerCount = 10

// ExportTerminalPayload is the JSON payload for the export:terminal task.
type ExportTerminalPayload struct {
	SerialNos []string `json:"serial_nos"`
	Session   string   `json:"session"`
	User      string   `json:"user"`
	ExportID  int      `json:"export_id"`

	// SelectAll mode: the background job collects all terminal IDs itself
	// instead of receiving them upfront. This avoids blocking the HTTP
	// handler while fetching hundreds of pages from TMS.
	SelectAll      bool   `json:"select_all,omitempty"`
	SearchSerialNo string `json:"search_serial_no,omitempty"`
	SearchType     int    `json:"search_type,omitempty"`
	Username       string `json:"username,omitempty"`
}

// exportRow holds fetched data for a single terminal, keyed by its index.
type exportRow struct {
	Index    int
	SerialNo string
	Data     map[string]interface{} // terminal detail
	Params   []interface{}          // paraList from GetTerminalParameter
	Error    bool
}

// ExportTerminalHandler queries terminals from TMS API and writes them to an
// Excel file. This is the Go equivalent of veristoreTools2's ExportTerminal.php.
type ExportTerminalHandler struct {
	tmsService *tms.Service
	tmsClient  *tms.Client
	adminRepo  *admin.Repository
	db         *gorm.DB
	exportDir  string
}

// NewExportTerminalHandler creates a new handler for the export:terminal task.
func NewExportTerminalHandler(tmsService *tms.Service, tmsClient *tms.Client, adminRepo *admin.Repository, db *gorm.DB, exportDir string) *ExportTerminalHandler {
	return &ExportTerminalHandler{
		tmsService: tmsService,
		tmsClient:  tmsClient,
		adminRepo:  adminRepo,
		db:         db,
		exportDir:  exportDir,
	}
}

// ProcessTask implements asynq.Handler. It fetches terminal details and
// parameters from the TMS API using concurrent workers, then writes the
// results to an Excel file.
func (h *ExportTerminalHandler) ProcessTask(ctx context.Context, task *asynq.Task) error {
	var payload ExportTerminalPayload
	if err := json.Unmarshal(task.Payload(), &payload); err != nil {
		return fmt.Errorf("export_terminal: unmarshal payload: %w", err)
	}

	logger := log.With().Str("task", TaskExportTerminal).Int("export_id", payload.ExportID).Logger()
	logger.Info().Int("count", len(payload.SerialNos)).Bool("selectAll", payload.SelectAll).Int("workers", exportWorkerCount).Msg("starting terminal export job")

	// Log job start to queue_log.
	createTime := strconv.FormatInt(time.Now().UnixMilli(), 10)
	_ = h.adminRepo.CreateQueueLog(&admin.QueueLog{
		CreateTime:  createTime,
		ExecTime:    createTime,
		ProcessName: "EXP",
		ServiceName: strPtr("export:terminal"),
	})

	session := payload.Session
	if session == "" {
		session = h.tmsService.GetSession()
	}
	if session == "" {
		return fmt.Errorf("export_terminal: no active TMS session")
	}

	// If SelectAll, collect all terminal IDs now (in the background job,
	// not in the HTTP handler — avoids blocking the web response).
	if payload.SelectAll {
		logger.Info().Str("search", payload.SearchSerialNo).Int("searchType", payload.SearchType).Msg("selectAll: collecting terminal IDs from TMS")
		var allIDs []string
		for page := 1; ; page++ {
			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
			}
			var resp *tms.TMSResponse
			var err error
			if payload.SearchSerialNo != "" {
				resp, err = h.tmsService.SearchTerminalsBulk(page, payload.SearchSerialNo, payload.SearchType, payload.Username)
			} else {
				resp, err = h.tmsService.GetTerminalListBulk(page)
			}
			if err != nil || resp == nil || resp.ResultCode != 0 || resp.Data == nil {
				break
			}
			tl, ok := resp.Data["terminalList"].([]interface{})
			if !ok || len(tl) == 0 {
				break
			}
			for _, t := range tl {
				if m, ok := t.(map[string]interface{}); ok {
					if devId, ok := m["deviceId"].(string); ok && devId != "" {
						allIDs = append(allIDs, devId)
					}
				}
			}
			totalPage := 0
			if tp, ok := resp.Data["totalPage"]; ok {
				totalPage, _ = tms.ToInt(tp)
			}
			logger.Info().Int("page", page).Int("totalPage", totalPage).Int("collected", len(allIDs)).Msg("selectAll: collected page")
			if page >= totalPage {
				break
			}
		}
		payload.SerialNos = allIDs
		logger.Info().Int("total", len(allIDs)).Msg("selectAll: finished collecting terminal IDs")

		// Update the export record total now that we know the actual count.
		if payload.ExportID > 0 {
			total := strconv.Itoa(len(allIDs))
			_ = h.adminRepo.UpdateExportProgress(payload.ExportID, "0", total)
		}
	}

	// Load the export record.
	var export *admin.Export
	if payload.ExportID > 0 {
		var err error
		export, err = h.adminRepo.FindExportByID(payload.ExportID)
		if err != nil {
			return fmt.Errorf("export_terminal: find export record: %w", err)
		}
	}

	// ------------------------------------------------------------------
	// Phase 1: Fetch all terminal data concurrently using a worker pool.
	// ------------------------------------------------------------------
	type job struct {
		Index    int
		SerialNo string
	}

	// Fetch tab names and operationMark once (not per-terminal).
	tabNames := tms.GetAllTabNames(h.db)
	operationMark, err := h.tmsClient.GetOperationMark(session)
	if err != nil {
		logger.Warn().Err(err).Msg("failed to get operation mark, parameter export may be incomplete")
	}
	logger.Info().Int("tabs", len(tabNames)).Msg("cached tab names and operation mark for export")

	startTime := time.Now()

	jobs := make(chan job, len(payload.SerialNos))
	results := make([]exportRow, len(payload.SerialNos))
	var processedCount int64
	var wg sync.WaitGroup

	// Start workers.
	for w := 0; w < exportWorkerCount; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := range jobs {
				select {
				case <-ctx.Done():
					return
				default:
				}

				row := exportRow{
					Index:    j.Index,
					SerialNo: j.SerialNo,
				}

				// Fetch terminal detail.
				detailResp, err := h.tmsClient.GetTerminalDetail(session, j.SerialNo)
				if err != nil || detailResp.ResultCode != 0 {
					logger.Warn().Str("serial", j.SerialNo).Msg("failed to get terminal detail")
					row.Error = true
					results[j.Index] = row
					count := atomic.AddInt64(&processedCount, 1)
					h.updateProgress(export, int(count), len(payload.SerialNos))
					continue
				}

				data := detailResp.Data
				if data == nil {
					data = map[string]interface{}{}
				}
				row.Data = data

				// Find app ID and fetch parameters.
				appID := ""
				if apps, ok := data["terminalShowApps"].([]interface{}); ok {
					for _, a := range apps {
						if appMap, ok := a.(map[string]interface{}); ok {
							if id := appMap["id"]; id != nil {
								appID = fmt.Sprintf("%v", id)
								break
							}
						}
					}
				}

				if appID != "" && operationMark != "" {
					// Use the resolved terminal ID from GetTerminalDetail to
					// skip the redundant getIdFromSN call, and fetch all tabs
					// concurrently instead of sequentially.
					resolvedID := tms.ToString(data["_resolvedTerminalId"])
					paramResp, err := h.tmsClient.GetTerminalParameterFast(session, resolvedID, operationMark, appID, tabNames)
					if err == nil && paramResp.ResultCode == 0 && paramResp.Data != nil {
						if pl, ok := paramResp.Data["paraList"].([]interface{}); ok {
							row.Params = pl
						}
					}
				}

				results[j.Index] = row
				count := atomic.AddInt64(&processedCount, 1)
				// Update progress every 5 terminals to reduce DB writes.
				if count%5 == 0 || int(count) == len(payload.SerialNos) {
					h.updateProgress(export, int(count), len(payload.SerialNos))
				}
			}
		}()
	}

	// Send all jobs.
	for idx, sn := range payload.SerialNos {
		jobs <- job{Index: idx, SerialNo: sn}
	}
	close(jobs)

	// Wait for all workers to finish.
	wg.Wait()

	fetchDuration := time.Since(startTime)
	logger.Info().
		Int64("processed", processedCount).
		Str("fetch_duration", fetchDuration.String()).
		Int("tabs_per_terminal", len(tabNames)).
		Msg("Phase 1 complete: all terminal data fetched")

	if ctx.Err() != nil {
		logger.Warn().Int64("processed", processedCount).Msg("context cancelled, stopping export")
		return ctx.Err()
	}

	// ------------------------------------------------------------------
	// Phase 2: Write all results to Excel (single-threaded, fast).
	// ------------------------------------------------------------------
	f := excelize.NewFile()
	defer f.Close()
	sheetName := "Sheet1"

	headers := []string{"NO", "CSI", "SN", "Device ID", "Model", "Vendor", "Merchant", "Status"}
	for i, hdr := range headers {
		cell, _ := excelize.CoordinatesToCellName(i+1, 1)
		f.SetCellValue(sheetName, cell, hdr)
	}
	boldStyle, _ := f.NewStyle(&excelize.Style{
		Font: &excelize.Font{Bold: true},
	})
	f.SetRowStyle(sheetName, 1, 1, boldStyle)

	// Track if parameter headers have been written.
	paramHeadersWritten := false

	for idx, row := range results {
		rowNum := idx + 2

		if row.Error || row.Data == nil {
			cell, _ := excelize.CoordinatesToCellName(1, rowNum)
			f.SetCellValue(sheetName, cell, idx+1)
			cell, _ = excelize.CoordinatesToCellName(2, rowNum)
			f.SetCellValue(sheetName, cell, row.SerialNo)
			cell, _ = excelize.CoordinatesToCellName(3, rowNum)
			f.SetCellValue(sheetName, cell, "Error fetching data")
			continue
		}

		rowData := []interface{}{
			idx + 1,
			row.SerialNo,
			tms.ToString(row.Data["sn"]),
			tms.ToString(row.Data["deviceId"]),
			tms.ToString(row.Data["model"]),
			tms.ToString(row.Data["vendor"]),
			tms.ToString(row.Data["merchantName"]),
			tms.ToString(row.Data["status"]),
		}
		for colIdx, val := range rowData {
			cell, _ := excelize.CoordinatesToCellName(colIdx+1, rowNum)
			f.SetCellValue(sheetName, cell, val)
		}

		// Write parameter columns.
		if row.Params != nil {
			colOffset := len(headers) + 1
			for pIdx, raw := range row.Params {
				p, ok := raw.(map[string]interface{})
				if !ok {
					continue
				}
				if !paramHeadersWritten {
					headerCell, _ := excelize.CoordinatesToCellName(colOffset+pIdx, 1)
					f.SetCellValue(sheetName, headerCell, tms.ToString(p["dataName"]))
				}
				cell, _ := excelize.CoordinatesToCellName(colOffset+pIdx, rowNum)
				f.SetCellValue(sheetName, cell, tms.ToString(p["value"]))
			}
			if !paramHeadersWritten {
				paramHeadersWritten = true
			}
		}
	}

	// Save file.
	filename := fmt.Sprintf("export_%d_%s.xlsx", payload.ExportID, time.Now().Format("20060102_150405"))
	filePath := h.exportDir + "/" + filename
	if err := f.SaveAs(filePath); err != nil {
		return fmt.Errorf("export_terminal: save Excel file: %w", err)
	}

	// Read file and store in database BLOB.
	fileData, err := os.ReadFile(filePath)
	if err != nil {
		logger.Warn().Err(err).Msg("failed to read export file for DB storage")
	}

	// Update export record with the completed file.
	if export != nil {
		export.ExpFilename = filename
		current := strconv.Itoa(int(processedCount))
		total := strconv.Itoa(len(payload.SerialNos))
		export.ExpCurrent = &current
		export.ExpTotal = &total
		if fileData != nil {
			export.ExpData = fileData
		}
		_ = h.adminRepo.UpdateExport(export)
	}

	logger.Info().
		Int64("processed", processedCount).
		Str("file", filePath).
		Msg("terminal export job completed")

	return nil
}

// updateProgress updates only the export progress fields (lightweight, no BLOB rewrite).
func (h *ExportTerminalHandler) updateProgress(export *admin.Export, current, total int) {
	if export == nil {
		return
	}
	_ = h.adminRepo.UpdateExportProgress(export.ExpID, strconv.Itoa(current), strconv.Itoa(total))
}
