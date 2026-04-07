package queue

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
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
func (h *ExportTerminalHandler) ProcessTask(ctx context.Context, task *asynq.Task) (retErr error) {
	defer func() {
		if r := recover(); r != nil {
			log.Error().Interface("panic", r).Str("task", TaskExportTerminal).Msg("export task panicked")
			retErr = fmt.Errorf("export_terminal: panic: %v", r)
		}
	}()

	var payload ExportTerminalPayload
	if err := json.Unmarshal(task.Payload(), &payload); err != nil {
		return fmt.Errorf("export_terminal: unmarshal payload: %w", err)
	}

	logger := log.With().Str("task", TaskExportTerminal).Int("export_id", payload.ExportID).Logger()
	logger.Info().Int("count", len(payload.SerialNos)).Bool("selectAll", payload.SelectAll).Int("workers", exportWorkerCount).Msg("starting terminal export job")

	// Log job start to queue_log (ignore errors — non-critical).
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
		logger.Error().Msg("no active TMS session — cannot export")
		return fmt.Errorf("export_terminal: no active TMS session")
	}
	logger.Info().Msg("TMS session OK, proceeding with export")

	// If SelectAll, collect all CSIs by paginating through TMS API.
	if payload.SelectAll {
		logger.Info().Str("search", payload.SearchSerialNo).Int("searchType", payload.SearchType).Msg("selectAll: collecting CSIs from TMS API")
		var allIDs []string
		pageSize := 100
		for page := 1; ; page++ {
			var resp *tms.TMSResponse
			var err error
			if payload.SearchSerialNo != "" {
				resp, err = h.tmsService.Client().GetTerminalListSearchBulk(session, page, payload.SearchSerialNo, payload.SearchType)
			} else {
				resp, err = h.tmsService.Client().GetTerminalListWithSize(page, pageSize)
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
					if sn, ok := m["sn"].(string); ok && sn != "" {
						allIDs = append(allIDs, sn)
					}
				}
			}
			totalPage := 0
			if tp, ok := resp.Data["totalPage"]; ok {
				totalPage, _ = tms.ToInt(tp)
			}
			if page >= totalPage {
				break
			}
		}

		payload.SerialNos = allIDs
		logger.Info().Int("total", len(allIDs)).Msg("selectAll: finished collecting CSIs from TMS API")

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

				total := len(payload.SerialNos)

				// Fetch terminal detail.
				detailResp, err := h.tmsClient.GetTerminalDetail(session, j.SerialNo)
				if err != nil || detailResp.ResultCode != 0 {
					logger.Warn().Str("csi", j.SerialNo).Int("index", j.Index+1).Int("total", total).Msg("export: failed to get terminal detail")
					row.Error = true
					results[j.Index] = row
					count := atomic.AddInt64(&processedCount, 1)
					h.updateProgress(export, int(count), total)
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

				paramCount := 0
				if appID != "" && operationMark != "" {
					// Use the resolved terminal ID from GetTerminalDetail to
					// skip the redundant getIdFromSN call, and fetch all tabs
					// concurrently instead of sequentially.
					resolvedID := tms.ToString(data["_resolvedTerminalId"])
					paramResp, err := h.tmsClient.GetTerminalParameterFast(session, resolvedID, operationMark, appID, tabNames)
					if err == nil && paramResp.ResultCode == 0 && paramResp.Data != nil {
						if pl, ok := paramResp.Data["paraList"].([]interface{}); ok {
							row.Params = pl
							paramCount = len(pl)
						}
					}
				}

				results[j.Index] = row
				count := atomic.AddInt64(&processedCount, 1)

				logger.Info().
					Str("csi", j.SerialNo).
					Int64("progress", count).
					Int("total", total).
					Int("params", paramCount).
					Msg("export: fetched terminal")

				// Update progress every 5 terminals to reduce DB writes.
				if count%5 == 0 || int(count) == total {
					h.updateProgress(export, int(count), total)
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
	// Matches veristoreTools2 format: 3-row header with merged cells,
	// static columns [NO, CSI, SN, App Version], dynamic columns from
	// template_parameter table, boolean conversion, and VerificationReport
	// data for SN/App Version.
	// ------------------------------------------------------------------
	f := excelize.NewFile()
	defer f.Close()
	sheetName := "Sheet1"

	// Static columns matching v2: NO, CSI, SN, App Version.
	staticHeaders := []string{"NO", "CSI", "SN", "App Version"}

	// Load template_parameter groups (distinct title/index_title/index).
	type tparamGroup struct {
		TparamTitle      string `gorm:"column:tparam_title"`
		TparamIndexTitle string `gorm:"column:tparam_index_title"`
		TparamIndex      int    `gorm:"column:tparam_index"`
	}
	var groups []tparamGroup
	h.db.Raw(`SELECT tparam_title, tparam_index_title, tparam_index
		FROM template_parameter
		GROUP BY tparam_title, tparam_index_title, tparam_index
		ORDER BY MIN(tparam_id)`).Scan(&groups)

	// Build column structure from template_parameter.
	type colDef struct {
		Title       string // Row 1: group title
		SubTitleRaw string // Row 2: raw sub-title (may start with * for dynamic lookup)
		Field       string // tparam_field
		Index       int    // 1-based parameter index
		Type        string // tparam_type ('b' for boolean)
	}
	var columns []colDef
	var row1Merges [][2]int // merge ranges for row 1 (title)
	var row2Merges [][2]int // merge ranges for row 2 (sub-title)

	cntMerge := 0
	row1Cnt := 0
	row2Cnt := 0
	for _, group := range groups {
		subTitles := strings.Split(group.TparamIndexTitle, "|")
		var params []admin.TemplateParameter
		h.db.Where("tparam_title = ?", group.TparamTitle).Order("tparam_id ASC").Find(&params)

		for i := 0; i < group.TparamIndex; i++ {
			subTitle := ""
			if i < len(subTitles) {
				subTitle = subTitles[i]
			}
			for _, param := range params {
				// Check tparam_except: skip this index if listed.
				skip := false
				if param.TparamExcept != nil && *param.TparamExcept != "" {
					for _, exc := range strings.Split(*param.TparamExcept, "|") {
						if exc == strconv.Itoa(i+1) {
							skip = true
							break
						}
					}
				}
				if skip {
					continue
				}
				cntMerge++
				columns = append(columns, colDef{
					Title:       group.TparamTitle,
					SubTitleRaw: subTitle,
					Field:       param.TparamField,
					Index:       i + 1,
					Type:        param.TparamType,
				})
			}
			row2Merges = append(row2Merges, [2]int{row2Cnt, cntMerge - 1})
			row2Cnt = cntMerge
		}
		row1Merges = append(row1Merges, [2]int{row1Cnt, cntMerge - 1})
		row1Cnt = cntMerge
	}

	logger.Info().Int("template_columns", len(columns)).Int("groups", len(groups)).Msg("built column structure from template_parameter")

	// Find the "best" headers from the terminal with fewest N/A descriptions
	// (matching v2 behavior where the least-N/A terminal determines header rows).
	bestHeader2 := make([]string, len(columns))
	bestHeader3 := make([]string, len(columns))
	for i := range columns {
		bestHeader2[i] = columns[i].SubTitleRaw
		bestHeader3[i] = ""
	}
	bestNACount := len(columns) + 1

	for _, row := range results {
		if row.Error || row.Params == nil {
			continue
		}
		paramMap := make(map[string][2]string)
		for _, raw := range row.Params {
			p, ok := raw.(map[string]interface{})
			if !ok {
				continue
			}
			paramMap[tms.ToString(p["dataName"])] = [2]string{
				tms.ToString(p["description"]),
				tms.ToString(p["value"]),
			}
		}

		naCount := 0
		h2 := make([]string, len(columns))
		h3 := make([]string, len(columns))
		for i, col := range columns {
			// Resolve dynamic sub-title (prefix *).
			st := col.SubTitleRaw
			if len(st) > 0 && st[0] == '*' {
				if pv, ok := paramMap[st[1:]]; ok {
					st = pv[1]
				}
			}
			h2[i] = st

			key := col.Field + "-" + strconv.Itoa(col.Index)
			if pv, ok := paramMap[key]; ok {
				h3[i] = pv[0]
			} else {
				h3[i] = ""
				naCount++
			}
		}
		if naCount < bestNACount {
			bestNACount = naCount
			bestHeader2 = h2
			bestHeader3 = h3
		}
	}

	// Write 3-row header with merged cells.
	colOffset := len(staticHeaders) + 1

	// Static headers: write in all 3 rows, merge vertically.
	for i, hdr := range staticHeaders {
		for r := 1; r <= 3; r++ {
			cell, _ := excelize.CoordinatesToCellName(i+1, r)
			f.SetCellValue(sheetName, cell, hdr)
		}
		startCell, _ := excelize.CoordinatesToCellName(i+1, 1)
		endCell, _ := excelize.CoordinatesToCellName(i+1, 3)
		_ = f.MergeCell(sheetName, startCell, endCell)
	}

	// Dynamic headers.
	for i, col := range columns {
		colNum := colOffset + i
		cell, _ := excelize.CoordinatesToCellName(colNum, 1)
		f.SetCellValue(sheetName, cell, col.Title)
		cell, _ = excelize.CoordinatesToCellName(colNum, 2)
		f.SetCellValue(sheetName, cell, bestHeader2[i])
		cell, _ = excelize.CoordinatesToCellName(colNum, 3)
		f.SetCellValue(sheetName, cell, bestHeader3[i])
	}

	// Apply title merges (row 1).
	for _, m := range row1Merges {
		if m[0] <= m[1] {
			startCell, _ := excelize.CoordinatesToCellName(colOffset+m[0], 1)
			endCell, _ := excelize.CoordinatesToCellName(colOffset+m[1], 1)
			_ = f.MergeCell(sheetName, startCell, endCell)
		}
	}
	// Apply sub-title merges (row 2).
	for _, m := range row2Merges {
		if m[0] <= m[1] {
			startCell, _ := excelize.CoordinatesToCellName(colOffset+m[0], 2)
			endCell, _ := excelize.CoordinatesToCellName(colOffset+m[1], 2)
			_ = f.MergeCell(sheetName, startCell, endCell)
		}
	}

	// Bold style for all 3 header rows.
	boldStyle, _ := f.NewStyle(&excelize.Style{
		Font: &excelize.Font{Bold: true},
	})
	for r := 1; r <= 3; r++ {
		f.SetRowStyle(sheetName, r, r, boldStyle)
	}

	// Write data rows starting at row 4 (after 3 header rows).
	// Only include terminals with successful data fetch (matching v2).
	rowNo := 0
	for _, row := range results {
		if row.Error || row.Data == nil {
			continue
		}
		rowNo++
		rowNum := rowNo + 3

		// NO
		cell, _ := excelize.CoordinatesToCellName(1, rowNum)
		f.SetCellValue(sheetName, cell, rowNo)

		// CSI
		cell, _ = excelize.CoordinatesToCellName(2, rowNum)
		f.SetCellValue(sheetName, cell, row.SerialNo)

		// SN and App Version from verification_report (matching v2).
		var vrResult struct {
			DeviceID   string `gorm:"column:vfi_rpt_term_device_id"`
			AppVersion string `gorm:"column:vfi_rpt_term_app_version"`
		}
		vrErr := h.db.Raw(`
			SELECT vfi_rpt_term_device_id, vfi_rpt_term_app_version
			FROM verification_report
			WHERE vfi_rpt_term_serial_num = ?
			ORDER BY vfi_rpt_id DESC LIMIT 1`, row.SerialNo).Scan(&vrResult).Error

		sn := ""
		appVersion := ""

		// Try TMS detail data first.
		if row.Data != nil {
			sn = tms.ToString(row.Data["sn"])
			appVersion = tms.ToString(row.Data["appVersion"])
		}

		// Override with verification report data if available.
		if vrErr == nil && vrResult.DeviceID != "" {
			sn = vrResult.DeviceID
		}
		if vrErr == nil && vrResult.AppVersion != "" {
			appVersion = vrResult.AppVersion
		}

		// Final fallback.
		if sn == "" {
			sn = "Unverified"
		}
		if appVersion == "" {
			appVersion = "Unverified"
		}

		cell, _ = excelize.CoordinatesToCellName(3, rowNum)
		f.SetCellValue(sheetName, cell, sn)
		cell, _ = excelize.CoordinatesToCellName(4, rowNum)
		f.SetCellValue(sheetName, cell, appVersion)

		// Dynamic parameter columns.
		if row.Params != nil {
			paramMap := make(map[string][2]string)
			for _, raw := range row.Params {
				p, ok := raw.(map[string]interface{})
				if !ok {
					continue
				}
				paramMap[tms.ToString(p["dataName"])] = [2]string{
					tms.ToString(p["description"]),
					tms.ToString(p["value"]),
				}
			}
			for i, col := range columns {
				key := col.Field + "-" + strconv.Itoa(col.Index)
				cell, _ := excelize.CoordinatesToCellName(colOffset+i, rowNum)
				if pv, ok := paramMap[key]; ok {
					if col.Type == "b" {
						if pv[1] == "1" {
							f.SetCellValue(sheetName, cell, "Yes")
						} else {
							f.SetCellValue(sheetName, cell, "No")
						}
					} else {
						f.SetCellValue(sheetName, cell, pv[1])
					}
				}
			}
		}
	}

	// Handle empty results (matching v2 behavior).
	if rowNo == 0 {
		f.SetCellValue(sheetName, "A4", "NULL")
	}

	// Save file.
	filename := fmt.Sprintf("export_csi_%s.xlsx", time.Now().Format("20060102_1504"))
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
