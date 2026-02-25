package queue

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/hibiken/asynq"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/xuri/excelize/v2"
	"gorm.io/gorm"

	"github.com/verifone/veristoretools3/internal/admin"
	"github.com/verifone/veristoretools3/internal/tms"
)

// Number of concurrent workers for importing terminals via the TMS API.
// Each worker processes one row at a time (5-6 API calls per row):
//   - CopyTerminal (1 call)
//   - GetTerminalDetail (1 call)
//   - GetTerminalParameterMultiTab (1 call, fetches all tabs)
//   - UpdateParameter (1 call)
//   - UpdateDeviceId (1 call)
//
// Total concurrent connections ≈ importWorkerCount × 1-2 (pipelined).
const importWorkerCount = 10

// importJob represents a single row from the Excel file to be imported.
type importJob struct {
	RowNum     int
	TemplateSN string
	SerialNum  string
	MerchantID string
	GroupIDs   []string
	Row        []string // full row data for buildParaList
}

// ImportTerminalPayload is the JSON payload for the import:terminal task.
type ImportTerminalPayload struct {
	FilePath string `json:"file_path"`
	Session  string `json:"session"`
	User     string `json:"user"`
	ImportID int    `json:"import_id"`
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
// Excel column mapping (matching import_format_csi.xlsx):
//   - Column A (1): No (row number — skipped)
//   - Column B (2): Template (source terminal SN to copy from)
//   - Column C (3): CSI (destination serial number)
//   - Column D (4): Profil Merchant (merchant ID)
//   - Column E (5): Group Merchant (group ID)
//   - Column F: Nama Merchant, Columns G-J: Alamat 1-4
//   - Columns K-AK: TID/MID/Plan Code fields
func (h *ImportTerminalHandler) ProcessTask(ctx context.Context, task *asynq.Task) error {
	var payload ImportTerminalPayload
	if err := json.Unmarshal(task.Payload(), &payload); err != nil {
		return fmt.Errorf("import_terminal: unmarshal payload: %w", err)
	}

	logger := log.With().Str("task", TaskImportTerminal).Str("file", payload.FilePath).Logger()
	logger.Info().Msg("starting terminal import job")

	// Ensure the import record is always marked as complete when the job ends,
	// even if it fails. This prevents the UI from being stuck on "loading".
	defer func() {
		if payload.ImportID > 0 {
			imp, err := h.adminRepo.FindLatestImport()
			if err == nil && imp != nil && imp.ImpID == payload.ImportID {
				cur := "0"
				tot := "0"
				if imp.ImpCurrent != nil {
					cur = *imp.ImpCurrent
				}
				if imp.ImpTotal != nil {
					tot = *imp.ImpTotal
				}
				if cur != tot {
					// Force mark as complete so UI doesn't stay stuck.
					logger.Warn().Str("cur", cur).Str("tot", tot).Msg("import job ended with incomplete progress, forcing completion")
					h.adminRepo.UpdateImportProgress(payload.ImportID, tot, tot)
				}
			}
		}
	}()

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
		if payload.ImportID > 0 {
			h.adminRepo.UpdateImportProgress(payload.ImportID, "0", "0")
		}
		return nil
	}

	session := payload.Session
	if session == "" {
		session = h.tmsService.GetSession()
	}
	if session == "" {
		return fmt.Errorf("import_terminal: no active TMS session")
	}

	// ------------------------------------------------------------------
	// Phase 1: Pre-parse all rows and validate required fields.
	// ------------------------------------------------------------------
	var jobs []importJob
	var skipCount int
	for rowIdx, row := range rows[1:] {
		rowNum := rowIdx + 2 // 1-indexed, +1 for header

		templateSN := cellValue(row, 1) // Column B: Template (source SN)
		serialNum := cellValue(row, 2)  // Column C: CSI (destination SN)
		merchantID := cellValue(row, 3) // Column D: Profil Merchant
		groupIDStr := cellValue(row, 4) // Column E: Group Merchant

		if templateSN == "" || serialNum == "" {
			skipCount++
			logger.Warn().Int("row", rowNum).Msg("skipping row: template or CSI is empty")
			continue
		}

		var groupIDs []string
		if groupIDStr != "" {
			for _, g := range strings.Split(groupIDStr, ",") {
				g = strings.TrimSpace(g)
				if g != "" {
					groupIDs = append(groupIDs, g)
				}
			}
		}

		jobs = append(jobs, importJob{
			RowNum:     rowNum,
			TemplateSN: templateSN,
			SerialNum:  serialNum,
			MerchantID: merchantID,
			GroupIDs:   groupIDs,
			Row:        row,
		})
	}

	totalRows := len(rows) - 1
	totalJobs := len(jobs)
	if payload.ImportID > 0 {
		h.adminRepo.UpdateImportProgress(payload.ImportID, "0", strconv.Itoa(totalRows))
	}

	logger.Info().
		Int("total_rows", totalRows).
		Int("valid_jobs", totalJobs).
		Int("skipped", skipCount).
		Int("workers", importWorkerCount).
		Msg("parsed Excel, starting concurrent import")

	if totalJobs == 0 {
		if payload.ImportID > 0 {
			h.adminRepo.UpdateImportProgress(payload.ImportID, strconv.Itoa(totalRows), strconv.Itoa(totalRows))
		}
		logger.Warn().Int("skipped", skipCount).Msg("no valid rows to import")
		return nil
	}

	// ------------------------------------------------------------------
	// Phase 2: Process rows concurrently using a worker pool.
	// ------------------------------------------------------------------

	// Cache tab names once (shared across all workers).
	tabNames := tms.GetAllTabNames(h.db)

	// Build group name → ID map so users can use group names in the Excel.
	groupMap := h.buildGroupMap(session)
	if len(groupMap) > 0 {
		logger.Info().Int("groups", len(groupMap)).Msg("loaded group name→ID map")
	}

	// Create a cancellable context so we can stop workers when the user
	// clicks "Reset" (which deletes the import record from the DB).
	importCtx, importCancel := context.WithCancel(ctx)
	defer importCancel()

	// Monitor for cancellation: check every 3 seconds if the import
	// record was deleted (user clicked Reset). If so, cancel all workers.
	if payload.ImportID > 0 {
		go func() {
			ticker := time.NewTicker(3 * time.Second)
			defer ticker.Stop()
			for {
				select {
				case <-importCtx.Done():
					return
				case <-ticker.C:
					imp, err := h.adminRepo.FindLatestImport()
					if err != nil || imp == nil || imp.ImpID != payload.ImportID {
						logger.Info().Msg("import record deleted (reset), cancelling workers")
						importCancel()
						return
					}
				}
			}
		}()
	}

	jobCh := make(chan importJob, totalJobs)
	var processedCount int64
	var successCount int64
	var failCount int64
	var wg sync.WaitGroup

	// Start workers.
	for w := 0; w < importWorkerCount; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := range jobCh {
				select {
				case <-importCtx.Done():
					return
				default:
				}

				ok := h.importSingleTerminal(importCtx, session, j, tabNames, groupMap, &logger)
				if ok {
					atomic.AddInt64(&successCount, 1)
				} else {
					atomic.AddInt64(&failCount, 1)
				}

				count := atomic.AddInt64(&processedCount, 1)
				// Update progress every 5 rows to reduce DB writes.
				if payload.ImportID > 0 && (count%5 == 0 || int(count) == totalJobs) {
					processed := skipCount + int(count)
					h.adminRepo.UpdateImportProgress(payload.ImportID, strconv.Itoa(processed), strconv.Itoa(totalRows))
				}
			}
		}()
	}

	// Send all jobs to the channel. Stop early if cancelled.
sendLoop:
	for _, j := range jobs {
		select {
		case <-importCtx.Done():
			break sendLoop
		case jobCh <- j:
		}
	}
	close(jobCh)

	// Wait for all workers to finish.
	wg.Wait()

	cancelled := importCtx.Err() != nil && ctx.Err() == nil // cancelled by reset, not by asynq

	// Mark import as complete (only if not cancelled by reset).
	if payload.ImportID > 0 && !cancelled {
		h.adminRepo.UpdateImportProgress(payload.ImportID, strconv.Itoa(totalRows), strconv.Itoa(totalRows))
	}

	if cancelled {
		logger.Info().
			Int64("success", atomic.LoadInt64(&successCount)).
			Int64("failed", atomic.LoadInt64(&failCount)).
			Int("skipped", skipCount).
			Msg("terminal import job cancelled (reset)")
	} else {
		logger.Info().
			Int64("success", atomic.LoadInt64(&successCount)).
			Int64("failed", atomic.LoadInt64(&failCount)).
			Int("skipped", skipCount).
			Msg("terminal import job completed")
	}

	return nil
}

// importSingleTerminal processes one row: copy from template, get detail,
// fetch params, update params, update device/merchant/group. Returns true
// on success, false on failure. Called from worker goroutines.
func (h *ImportTerminalHandler) importSingleTerminal(ctx context.Context, session string, j importJob, tabNames []string, groupMap map[string]int, logger *zerolog.Logger) bool {
	// Step 1: Copy terminal from template.
	// If the terminal already exists ("Duplicate"), continue with update steps.
	isExisting := false
	copyResp, err := h.tmsClient.CopyTerminal(session, j.TemplateSN, j.SerialNum)
	if err != nil {
		logger.Error().Err(err).Int("row", j.RowNum).Msg("copy terminal API call failed")
		return false
	}
	if copyResp.ResultCode != 0 {
		if strings.Contains(strings.ToLower(copyResp.Desc), "duplicate") {
			isExisting = true
			logger.Info().Int("row", j.RowNum).Str("serial", j.SerialNum).Msg("terminal already exists, updating")
		} else {
			logger.Warn().Int("row", j.RowNum).Str("desc", copyResp.Desc).Msg("copy terminal returned error")
			return false
		}
	}

	// Step 2: Get terminal detail.
	detailResp, err := h.tmsClient.GetTerminalDetail(session, j.SerialNum)
	if err != nil || detailResp.ResultCode != 0 {
		if !isExisting {
			_ = h.deleteTerminalOnError(session, j.SerialNum)
		}
		logger.Warn().Int("row", j.RowNum).Msg("failed to get terminal detail")
		return false
	}

	// Step 3: Find the target app.
	appID := h.findAppID(detailResp)
	if appID == "" {
		if !isExisting {
			_ = h.deleteTerminalOnError(session, j.SerialNum)
		}
		logger.Warn().Int("row", j.RowNum).Msg("target app not found on terminal")
		return false
	}

	// Step 4: Get terminal parameters (batch multi-tab).
	paramResp, err := h.tmsClient.GetTerminalParameterMultiTab(session, j.SerialNum, appID, tabNames)
	if err != nil || paramResp.Data == nil {
		if !isExisting {
			_ = h.deleteTerminalOnError(session, j.SerialNum)
		}
		logger.Warn().Int("row", j.RowNum).Msg("failed to get terminal parameters")
		return false
	}
	allParams, _ := paramResp.Data["paraList"].([]interface{})
	if len(allParams) == 0 {
		if !isExisting {
			_ = h.deleteTerminalOnError(session, j.SerialNum)
		}
		logger.Warn().Int("row", j.RowNum).Msg("failed to get terminal parameters")
		return false
	}

	// Step 5: Update parameters from Excel columns F onwards.
	paraList := h.buildParaList(paramResp, j.Row)
	if len(paraList) > 0 {
		updateResp, err := h.tmsClient.UpdateParameter(session, j.SerialNum, paraList, appID)
		if err != nil || updateResp.ResultCode != 0 {
			if !isExisting {
				_ = h.deleteTerminalOnError(session, j.SerialNum)
			}
			desc := ""
			if updateResp != nil {
				desc = updateResp.Desc
			}
			logger.Warn().Int("row", j.RowNum).Str("desc", desc).Msg("failed to update terminal parameters")
			return false
		}
	}

	// Step 6: Update device ID, merchant, and groups.
	model := ""
	if detailResp.Data != nil {
		model = tms.ToString(detailResp.Data["model"])
	}
	merchantIDInt, _ := strconv.Atoi(j.MerchantID)

	// Resolve group IDs: accept both numeric IDs and group names.
	groupIDsInt := make([]int, 0, len(j.GroupIDs))
	for _, g := range j.GroupIDs {
		if gInt, err := strconv.Atoi(g); err == nil {
			groupIDsInt = append(groupIDsInt, gInt)
		} else if gInt, ok := groupMap[strings.ToLower(g)]; ok {
			groupIDsInt = append(groupIDsInt, gInt)
		} else {
			logger.Warn().Int("row", j.RowNum).Str("group", g).Msg("unknown group name, skipping")
		}
	}

	deviceID := ""
	if detailResp.Data != nil {
		deviceID = tms.ToString(detailResp.Data["deviceId"])
	}

	if j.MerchantID != "" || len(groupIDsInt) > 0 {
		updateDevResp, err := h.tmsClient.UpdateDeviceId(session, j.SerialNum, model, merchantIDInt, groupIDsInt, deviceID)
		if err != nil {
			if !isExisting {
				_ = h.deleteTerminalOnError(session, j.SerialNum)
			}
			logger.Warn().Err(err).Int("row", j.RowNum).Msg("failed to update device details")
			return false
		}
		if updateDevResp.ResultCode != 0 {
			if !isExisting {
				_ = h.deleteTerminalOnError(session, j.SerialNum)
			}
			logger.Warn().Int("row", j.RowNum).Int("code", updateDevResp.ResultCode).Str("desc", updateDevResp.Desc).Msg("failed to update device details")
			return false
		}
	}

	if isExisting {
		logger.Info().Int("row", j.RowNum).Str("serial", j.SerialNum).Msg("existing terminal updated successfully")
	} else {
		logger.Info().Int("row", j.RowNum).Str("serial", j.SerialNum).Msg("terminal imported successfully")
	}
	return true
}

// buildGroupMap fetches the TMS group list and returns a lowercase name → ID map.
func (h *ImportTerminalHandler) buildGroupMap(session string) map[string]int {
	groupMap := map[string]int{}
	resp, err := h.tmsClient.GetGroupList(session)
	if err != nil || resp.ResultCode != 0 || resp.Data == nil {
		return groupMap
	}
	groups, _ := resp.Data["groups"].([]interface{})
	for _, g := range groups {
		gm, _ := g.(map[string]interface{})
		if gm == nil {
			continue
		}
		name := tms.ToString(gm["name"])
		id := 0
		switch v := gm["id"].(type) {
		case int:
			id = v
		case float64:
			id = int(v)
		}
		if name != "" && id != 0 {
			groupMap[strings.ToLower(name)] = id
		}
	}
	return groupMap
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
