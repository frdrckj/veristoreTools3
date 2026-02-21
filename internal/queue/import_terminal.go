package queue

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/hibiken/asynq"
	"github.com/rs/zerolog/log"
	"github.com/xuri/excelize/v2"
	"gorm.io/gorm"

	"github.com/verifone/veristoretools3/internal/admin"
	"github.com/verifone/veristoretools3/internal/tms"
)

// ImportTerminalPayload is the JSON payload for the import:terminal task.
type ImportTerminalPayload struct {
	FilePath string `json:"file_path"`
	Session  string `json:"session"`
	User     string `json:"user"`
}

// ImportTerminalHandler processes Excel files containing terminal data and
// registers each terminal via the TMS API. This is the Go equivalent of
// veristoreTools2's ImportTerminal.php component.
type ImportTerminalHandler struct {
	tmsService *tms.Service
	tmsClient  *tms.Client
	adminRepo  *admin.Repository
	db         *gorm.DB
}

// NewImportTerminalHandler creates a new handler for the import:terminal task.
func NewImportTerminalHandler(tmsService *tms.Service, tmsClient *tms.Client, adminRepo *admin.Repository, db *gorm.DB) *ImportTerminalHandler {
	return &ImportTerminalHandler{
		tmsService: tmsService,
		tmsClient:  tmsClient,
		adminRepo:  adminRepo,
		db:         db,
	}
}

// ProcessTask implements asynq.Handler. It reads the Excel file specified in
// the payload, iterates over rows, and calls the TMS API to add each terminal.
// Progress is logged to the queue_log table.
//
// Excel column mapping (matching v2 ImportTerminal.php):
//   - Column A (1): Template SN (source serial number to copy from)
//   - Column B (2): Serial Number (destination CSI)
//   - Column C (3): Vendor
//   - Column D (4): Merchant ID
//   - Column E (5): Group ID(s)
//   - Columns F-AK: Parameter fields (TID, MID, print headers, etc.)
func (h *ImportTerminalHandler) ProcessTask(ctx context.Context, task *asynq.Task) error {
	var payload ImportTerminalPayload
	if err := json.Unmarshal(task.Payload(), &payload); err != nil {
		return fmt.Errorf("import_terminal: unmarshal payload: %w", err)
	}

	logger := log.With().Str("task", TaskImportTerminal).Str("file", payload.FilePath).Logger()
	logger.Info().Msg("starting terminal import job")

	// Log job start to queue_log.
	createTime := strconv.FormatInt(time.Now().UnixMilli(), 10)
	_ = h.adminRepo.CreateQueueLog(&admin.QueueLog{
		CreateTime:  createTime,
		ExecTime:    createTime,
		ProcessName: "ITRM",
		ServiceName: strPtr("import:terminal"),
	})

	// Open the Excel file.
	f, err := excelize.OpenFile(payload.FilePath)
	if err != nil {
		logger.Error().Err(err).Msg("failed to open Excel file")
		return fmt.Errorf("import_terminal: open file: %w", err)
	}
	defer f.Close()

	// Get the first sheet.
	sheetName := f.GetSheetName(0)
	if sheetName == "" {
		return fmt.Errorf("import_terminal: no sheets found in file")
	}

	rows, err := f.GetRows(sheetName)
	if err != nil {
		return fmt.Errorf("import_terminal: read rows: %w", err)
	}

	if len(rows) < 2 {
		logger.Warn().Msg("Excel file has no data rows (only header)")
		return nil
	}

	session := payload.Session
	if session == "" {
		session = h.tmsService.GetSession()
	}
	if session == "" {
		return fmt.Errorf("import_terminal: no active TMS session")
	}

	var successCount, failCount int

	// Process data rows (skip header row at index 0).
	for rowIdx, row := range rows[1:] {
		rowNum := rowIdx + 2 // 1-indexed, +1 for header

		select {
		case <-ctx.Done():
			logger.Warn().Int("row", rowNum).Msg("context cancelled, stopping import")
			return ctx.Err()
		default:
		}

		// Extract columns with safe access.
		templateSN := cellValue(row, 0) // Column A: Template SN
		serialNum := cellValue(row, 1)  // Column B: Serial Number (CSI)
		vendor := cellValue(row, 2)     // Column C: Vendor
		merchantID := cellValue(row, 3) // Column D: Merchant ID
		groupIDStr := cellValue(row, 4) // Column E: Group ID(s)

		// Validate required fields.
		if templateSN == "" || serialNum == "" {
			failCount++
			logger.Warn().Int("row", rowNum).Msg("skipping row: template SN or serial number is empty")
			continue
		}

		// Parse group IDs.
		var groupIDs []string
		if groupIDStr != "" {
			for _, g := range strings.Split(groupIDStr, ",") {
				g = strings.TrimSpace(g)
				if g != "" {
					groupIDs = append(groupIDs, g)
				}
			}
		}

		// Step 1: Copy terminal from template.
		copyResp, err := h.tmsClient.CopyTerminal(session, templateSN, serialNum)
		if err != nil {
			failCount++
			logger.Error().Err(err).Int("row", rowNum).Msg("copy terminal API call failed")
			continue
		}
		if copyResp.ResultCode != 0 {
			failCount++
			logger.Warn().Int("row", rowNum).Str("desc", copyResp.Desc).Msg("copy terminal returned error")
			continue
		}

		// Step 2: Get terminal detail for the new terminal.
		detailResp, err := h.tmsClient.GetTerminalDetail(session, serialNum)
		if err != nil || detailResp.ResultCode != 0 {
			failCount++
			_ = h.deleteTerminalOnError(session, serialNum)
			logger.Warn().Int("row", rowNum).Msg("failed to get terminal detail after copy")
			continue
		}

		// Step 3: Find the target app and get parameter info.
		appID := h.findAppID(detailResp)
		if appID == "" {
			failCount++
			_ = h.deleteTerminalOnError(session, serialNum)
			logger.Warn().Int("row", rowNum).Msg("target app not found on terminal")
			continue
		}

		// Step 4: Get terminal parameters (batch multi-tab).
		tabNames := tms.GetAllTabNames(h.db)
		paramResp, err := h.tmsClient.GetTerminalParameterMultiTab(session, serialNum, appID, tabNames)
		if err != nil || paramResp.Data == nil {
			failCount++
			_ = h.deleteTerminalOnError(session, serialNum)
			logger.Warn().Int("row", rowNum).Msg("failed to get terminal parameters")
			continue
		}
		allParams, _ := paramResp.Data["paraList"].([]interface{})
		if len(allParams) == 0 {
			failCount++
			_ = h.deleteTerminalOnError(session, serialNum)
			logger.Warn().Int("row", rowNum).Msg("failed to get terminal parameters")
			continue
		}

		// Step 5: Update parameters from Excel columns F onwards.
		paraList := h.buildParaList(paramResp, row)

		if len(paraList) > 0 {
			updateResp, err := h.tmsClient.UpdateParameter(session, serialNum, paraList, appID)
			if err != nil || updateResp.ResultCode != 0 {
				failCount++
				_ = h.deleteTerminalOnError(session, serialNum)
				logger.Warn().Int("row", rowNum).Msg("failed to update terminal parameters")
				continue
			}
		}

		// Step 6: Update device ID, merchant, and groups.
		model := ""
		if detailResp.Data != nil {
			model = tms.ToString(detailResp.Data["model"])
		}
		merchantIDInt, _ := strconv.Atoi(merchantID)
		groupIDsInt := make([]int, 0, len(groupIDs))
		for _, g := range groupIDs {
			if gInt, err := strconv.Atoi(g); err == nil {
				groupIDsInt = append(groupIDsInt, gInt)
			}
		}

		deviceID := ""
		if detailResp.Data != nil {
			deviceID = tms.ToString(detailResp.Data["deviceId"])
		}

		if vendor != "" || merchantID != "" || len(groupIDsInt) > 0 {
			updateDevResp, err := h.tmsClient.UpdateDeviceId(session, serialNum, model, merchantIDInt, groupIDsInt, deviceID)
			if err != nil || updateDevResp.ResultCode != 0 {
				failCount++
				_ = h.deleteTerminalOnError(session, serialNum)
				logger.Warn().Int("row", rowNum).Msg("failed to update device details")
				continue
			}
		}

		successCount++
		logger.Info().Int("row", rowNum).Str("serial", serialNum).Msg("terminal imported successfully")
	}

	logger.Info().
		Int("success", successCount).
		Int("failed", failCount).
		Msg("terminal import job completed")

	return nil
}

// deleteTerminalOnError attempts to delete a terminal that failed during import.
func (h *ImportTerminalHandler) deleteTerminalOnError(session, serialNum string) error {
	_, err := h.tmsClient.DeleteTerminal(session, serialNum)
	return err
}

// findAppID looks for the target app in a terminal detail response and returns
// its ID as a string.
func (h *ImportTerminalHandler) findAppID(resp *tms.TMSResponse) string {
	if resp.Data == nil {
		return ""
	}
	apps, ok := resp.Data["terminalShowApps"].([]interface{})
	if !ok {
		return ""
	}
	for _, a := range apps {
		appMap, ok := a.(map[string]interface{})
		if !ok {
			continue
		}
		if id := appMap["id"]; id != nil {
			return fmt.Sprintf("%v", id)
		}
	}
	return ""
}

// getFieldName maps Excel column letters to TMS parameter field names.
// This matches the v2 ImportTerminal.php getFieldName() method.
func getFieldName(col string) string {
	fields := map[string]string{
		"F":  "TP-PRINT_CONFIG-HEADER1-1",
		"G":  "TP-PRINT_CONFIG-HEADER2-1",
		"H":  "TP-PRINT_CONFIG-HEADER3-1",
		"I":  "TP-PRINT_CONFIG-HEADER4-1",
		"J":  "TP-PRINT_CONFIG-HEADER5-1",
		"K":  "TP-MERCHANT-TERMINAL_ID-1",
		"L":  "TP-MERCHANT-MERCHANT_ID-1",
		"M":  "TP-MERCHANT-TERMINAL_ID-2",
		"N":  "TP-MERCHANT-MERCHANT_ID-2",
		"O":  "TP-MERCHANT-TERMINAL_ID-3",
		"P":  "TP-MERCHANT-MERCHANT_ID-3",
		"Q":  "TP-MERCHANT-INSTALLMENT_PROMO_CODE-3",
		"R":  "TP-MERCHANT-TERMINAL_ID-4",
		"S":  "TP-MERCHANT-MERCHANT_ID-4",
		"T":  "TP-MERCHANT-INSTALLMENT_PROMO_CODE-4",
		"U":  "TP-MERCHANT-TERMINAL_ID-5",
		"V":  "TP-MERCHANT-MERCHANT_ID-5",
		"W":  "TP-MERCHANT-INSTALLMENT_PROMO_CODE-5",
		"X":  "TP-MERCHANT-TERMINAL_ID-6",
		"Y":  "TP-MERCHANT-MERCHANT_ID-6",
		"Z":  "TP-MERCHANT-INSTALLMENT_PROMO_CODE-6",
		"AA": "TP-MERCHANT-TERMINAL_ID-7",
		"AB": "TP-MERCHANT-MERCHANT_ID-7",
		"AC": "TP-MERCHANT-INSTALLMENT_PROMO_CODE-7",
		"AD": "TP-MERCHANT-TERMINAL_ID-8",
		"AE": "TP-MERCHANT-MERCHANT_ID-8",
		"AF": "TP-MERCHANT-INSTALLMENT_PROMO_CODE-8",
		"AG": "TP-MERCHANT-TERMINAL_ID-9",
		"AH": "TP-MERCHANT-MERCHANT_ID-9",
		"AI": "TP-MERCHANT-INSTALLMENT_PROMO_CODE-9",
		"AJ": "TP-MERCHANT-TERMINAL_ID-10",
		"AK": "TP-MERCHANT-MERCHANT_ID-10",
	}
	if f, ok := fields[col]; ok {
		return f
	}
	return ""
}

// indexToColumnLetter converts a 0-based column index to an Excel column
// letter (A, B, ..., Z, AA, AB, ...).
func indexToColumnLetter(idx int) string {
	const alphabet = "ABCDEFGHIJKLMNOPQRSTUVWXYZ"
	if idx < 26 {
		return string(alphabet[idx%26])
	}
	return string(alphabet[idx/26-1]) + string(alphabet[idx%26])
}

// buildParaList takes the TMS parameter response and updates values from the
// Excel row columns F onwards, matching the v2 updateParaList logic.
func (h *ImportTerminalHandler) buildParaList(paramResp *tms.TMSResponse, row []string) []map[string]interface{} {
	if paramResp.Data == nil {
		return nil
	}
	rawParaList, ok := paramResp.Data["paraList"].([]interface{})
	if !ok {
		return nil
	}

	// Build a map of fieldName -> value from the Excel row.
	importValues := map[string]string{}
	for colIdx := 5; colIdx < len(row); colIdx++ { // Columns F (idx 5) onwards
		colLetter := indexToColumnLetter(colIdx)
		fieldName := getFieldName(colLetter)
		if fieldName != "" && row[colIdx] != "" {
			value := row[colIdx]
			// Uppercase certain columns matching v2 logic.
			if colLetter == "C" || colLetter == "F" || colLetter == "G" ||
				colLetter == "H" || colLetter == "I" || colLetter == "J" {
				value = strings.ToUpper(value)
			}
			importValues[fieldName] = value
		}
	}

	// Build the updated paraList.
	var paraList []map[string]interface{}
	for _, raw := range rawParaList {
		p, ok := raw.(map[string]interface{})
		if !ok {
			continue
		}
		entry := map[string]interface{}{
			"dataName": p["dataName"],
			"value":    p["value"],
		}
		// Check if we have an updated value from the import.
		dataName, _ := p["dataName"].(string)
		if newVal, found := importValues[dataName]; found {
			entry["value"] = newVal
		}
		// Include viewName if present (needed for UpdateParameter grouping).
		if vn, ok := p["viewName"]; ok {
			entry["viewName"] = vn
		}
		paraList = append(paraList, entry)
	}

	return paraList
}

// cellValue safely returns the string value at the given column index in a row.
func cellValue(row []string, idx int) string {
	if idx < len(row) {
		return strings.TrimSpace(row[idx])
	}
	return ""
}

// strPtr returns a pointer to a string value.
func strPtr(s string) *string {
	return &s
}
