package queue

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"github.com/hibiken/asynq"
	"github.com/rs/zerolog/log"
	"github.com/xuri/excelize/v2"
	"gorm.io/gorm"

	"github.com/verifone/veristoretools3/internal/admin"
	"github.com/verifone/veristoretools3/internal/tms"
)

// ExportTerminalPayload is the JSON payload for the export:terminal task.
type ExportTerminalPayload struct {
	SerialNos []string `json:"serial_nos"`
	Session   string   `json:"session"`
	User      string   `json:"user"`
	ExportID  int      `json:"export_id"`
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
// parameters from the TMS API for each serial number in the payload, then
// writes the results to an Excel file. Progress is tracked via the export
// table.
func (h *ExportTerminalHandler) ProcessTask(ctx context.Context, task *asynq.Task) error {
	var payload ExportTerminalPayload
	if err := json.Unmarshal(task.Payload(), &payload); err != nil {
		return fmt.Errorf("export_terminal: unmarshal payload: %w", err)
	}

	logger := log.With().Str("task", TaskExportTerminal).Int("export_id", payload.ExportID).Logger()
	logger.Info().Int("count", len(payload.SerialNos)).Msg("starting terminal export job")

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

	// Load the export record.
	var export *admin.Export
	if payload.ExportID > 0 {
		var err error
		export, err = h.adminRepo.FindExportByID(payload.ExportID)
		if err != nil {
			return fmt.Errorf("export_terminal: find export record: %w", err)
		}
	}

	// Create Excel file.
	f := excelize.NewFile()
	defer f.Close()
	sheetName := "Sheet1"

	// Write header row.
	headers := []string{"NO", "CSI", "SN", "Device ID", "Model", "Vendor", "Merchant", "Status"}
	for i, h := range headers {
		cell, _ := excelize.CoordinatesToCellName(i+1, 1)
		f.SetCellValue(sheetName, cell, h)
	}

	// Style header row as bold.
	boldStyle, _ := f.NewStyle(&excelize.Style{
		Font: &excelize.Font{Bold: true},
	})
	f.SetRowStyle(sheetName, 1, 1, boldStyle)

	var processedCount int

	for idx, serialNo := range payload.SerialNos {
		select {
		case <-ctx.Done():
			logger.Warn().Int("processed", processedCount).Msg("context cancelled, stopping export")
			return ctx.Err()
		default:
		}

		rowNum := idx + 2 // 1-indexed, +1 for header

		// Get terminal detail from TMS.
		detailResp, err := h.tmsClient.GetTerminalDetail(session, serialNo)
		if err != nil || detailResp.ResultCode != 0 {
			logger.Warn().Str("serial", serialNo).Msg("failed to get terminal detail, writing partial row")
			cell, _ := excelize.CoordinatesToCellName(1, rowNum)
			f.SetCellValue(sheetName, cell, idx+1)
			cell, _ = excelize.CoordinatesToCellName(2, rowNum)
			f.SetCellValue(sheetName, cell, serialNo)
			cell, _ = excelize.CoordinatesToCellName(3, rowNum)
			f.SetCellValue(sheetName, cell, "Error fetching data")
			processedCount++
			continue
		}

		data := detailResp.Data
		if data == nil {
			data = map[string]interface{}{}
		}

		// Write terminal data to row.
		rowData := []interface{}{
			idx + 1,
			serialNo,
			tms.ToString(data["sn"]),
			tms.ToString(data["deviceId"]),
			tms.ToString(data["model"]),
			tms.ToString(data["vendor"]),
			tms.ToString(data["merchantName"]),
			tms.ToString(data["status"]),
		}

		for colIdx, val := range rowData {
			cell, _ := excelize.CoordinatesToCellName(colIdx+1, rowNum)
			f.SetCellValue(sheetName, cell, val)
		}

		// Optionally add parameter columns for each terminal.
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

		if appID != "" {
			paramResp, err := h.tmsClient.GetTerminalParameter(session, serialNo, appID)
			if err == nil && paramResp.ResultCode == 0 && paramResp.Data != nil {
				if paraList, ok := paramResp.Data["paraList"].([]interface{}); ok {
					colOffset := len(headers) + 1
					for pIdx, raw := range paraList {
						p, ok := raw.(map[string]interface{})
						if !ok {
							continue
						}
						// On first data row, write parameter headers.
						if idx == 0 {
							headerCell, _ := excelize.CoordinatesToCellName(colOffset+pIdx, 1)
							f.SetCellValue(sheetName, headerCell, tms.ToString(p["dataName"]))
						}
						cell, _ := excelize.CoordinatesToCellName(colOffset+pIdx, rowNum)
						f.SetCellValue(sheetName, cell, tms.ToString(p["value"]))
					}
				}
			}
		}

		processedCount++

		// Update export progress.
		if export != nil {
			current := strconv.Itoa(processedCount)
			total := strconv.Itoa(len(payload.SerialNos))
			export.ExpCurrent = &current
			export.ExpTotal = &total
			_ = h.adminRepo.UpdateExport(export)
		}
	}

	// Save the Excel file.
	filename := fmt.Sprintf("export_%d_%s.xlsx", payload.ExportID, time.Now().Format("20060102_150405"))
	filePath := h.exportDir + "/" + filename
	if err := f.SaveAs(filePath); err != nil {
		return fmt.Errorf("export_terminal: save Excel file: %w", err)
	}

	// Update export record with the completed file.
	if export != nil {
		export.ExpFilename = filename
		current := strconv.Itoa(processedCount)
		total := strconv.Itoa(len(payload.SerialNos))
		export.ExpCurrent = &current
		export.ExpTotal = &total
		_ = h.adminRepo.UpdateExport(export)
	}

	logger.Info().
		Int("processed", processedCount).
		Str("file", filePath).
		Msg("terminal export job completed")

	return nil
}
