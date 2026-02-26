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
// Each worker processes one row at a time using the optimized pipeline
// (6 API calls per row, with 8 param tabs fetched concurrently):
//   - CopyTerminalById (1 call, source ID pre-cached)
//   - getIdFromSN to resolve dest SN (1 call)
//   - GetImportTerminalInfo: old-API detail + app list (2 calls)
//   - GetParameterTabsConcurrent: 8 tabs in parallel (≈1 RTT)
//   - UpdateParameterById (1 call, ID pre-resolved)
//   - UpdateDeviceIdDirect (1 call, detail pre-fetched)
//
// Total concurrent connections ≈ importWorkerCount × ~8-10 (burst during tab fetch).
const importWorkerCount = 20

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

	// Log if the import ends with incomplete progress (e.g., panic recovery).
	// With MaxRetry(0) and 4h timeout, incomplete progress means something
	// unexpected happened. Don't force-complete — the user can use Reset.
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
					logger.Warn().Str("cur", cur).Str("tot", tot).Msg("import job ended with incomplete progress — user can click Reset")
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
	// Optimized pipeline: resolve IDs once, fetch tabs concurrently,
	// reuse pre-fetched data across steps. Reduces API calls from
	// ~20 per terminal to ~6 per terminal.
	// ------------------------------------------------------------------

	// Cache tab names once (shared across all workers).
	tabNames := tms.GetAllTabNames(h.db)

	// Cache operationMark once for the entire import (saves 1 call/terminal).
	operationMark, err := h.tmsClient.GetOperationMark(session)
	if err != nil {
		logger.Warn().Err(err).Msg("failed to get operationMark, params may fail")
	}

	// Build group name → ID map so users can use group names in the Excel.
	groupMap := h.buildGroupMap(session)

	// Pre-resolve unique template SN → old-API int ID (shared across workers).
	// Many rows share the same template, so this saves N-1 API calls per template.
	templateCache := h.buildTemplateCache(session, jobs, &logger)
	logger.Info().Int("templates", len(templateCache)).Msg("pre-resolved template IDs")

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

				ok := h.importSingleTerminalFast(importCtx, session, j, tabNames, operationMark, groupMap, templateCache, &logger)
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

	cancelledByReset := importCtx.Err() != nil && ctx.Err() == nil
	cancelledByTimeout := ctx.Err() != nil
	actualProcessed := skipCount + int(atomic.LoadInt64(&processedCount))

	// Only mark as complete if ALL rows were actually processed (not cancelled).
	if payload.ImportID > 0 {
		if !cancelledByReset && !cancelledByTimeout {
			// Normal completion — set to totalRows/totalRows.
			h.adminRepo.UpdateImportProgress(payload.ImportID, strconv.Itoa(totalRows), strconv.Itoa(totalRows))
		} else {
			// Cancelled — update with actual progress so UI shows correct state.
			h.adminRepo.UpdateImportProgress(payload.ImportID, strconv.Itoa(actualProcessed), strconv.Itoa(totalRows))
		}
	}

	if cancelledByReset {
		logger.Info().
			Int64("success", atomic.LoadInt64(&successCount)).
			Int64("failed", atomic.LoadInt64(&failCount)).
			Int("skipped", skipCount).
			Msg("terminal import job cancelled (reset)")
	} else if cancelledByTimeout {
		logger.Warn().
			Int64("success", atomic.LoadInt64(&successCount)).
			Int64("failed", atomic.LoadInt64(&failCount)).
			Int("skipped", skipCount).
			Int("processed", actualProcessed).
			Int("total", totalRows).
			Msg("terminal import job timed out by asynq — increase task timeout")
	} else {
		logger.Info().
			Int64("success", atomic.LoadInt64(&successCount)).
			Int64("failed", atomic.LoadInt64(&failCount)).
			Int("skipped", skipCount).
			Msg("terminal import job completed")
	}

	return nil
}

// buildTemplateCache pre-resolves all unique template SNs to their old-API
// int IDs. This is done once before the worker pool starts, so all workers
// can use cached IDs instead of each resolving the same template SN.
func (h *ImportTerminalHandler) buildTemplateCache(session string, jobs []importJob, logger *zerolog.Logger) map[string]int {
	// Collect unique template SNs.
	unique := map[string]bool{}
	for _, j := range jobs {
		unique[j.TemplateSN] = true
	}

	cache := map[string]int{}
	for sn := range unique {
		id, err := h.tmsClient.GetIdFromSN(session, sn)
		if err != nil {
			logger.Warn().Str("templateSN", sn).Err(err).Msg("failed to resolve template SN")
			continue
		}
		cache[sn] = id
	}
	return cache
}

// importSingleTerminalFast is the optimized import pipeline for a single row.
// It resolves each SN only once and reuses the ID across all steps, fetches
// parameter tabs concurrently, and reuses pre-fetched detail data.
//
// API calls per terminal (vs 20 in the old pipeline):
//   1. CopyTerminalById — 1 call (source ID pre-cached)
//   2. getIdFromSN(destSN) — 1 call (resolve once, reuse everywhere)
//   3. GetImportTerminalInfo — 2 calls (old-API detail + app list)
//   4. GetParameterTabsConcurrent — 8 tabs in parallel ≈ 1 RTT
//   5. UpdateParameterById — 1 call (ID pre-resolved)
//   6. UpdateDeviceIdDirect — 1 call (detail pre-fetched)
// Total: ~7 calls + ~1 RTT for parallel tabs ≈ 60-70% fewer round-trips.
func (h *ImportTerminalHandler) importSingleTerminalFast(
	ctx context.Context, session string, j importJob,
	tabNames []string, operationMark string,
	groupMap map[string]int, templateCache map[string]int,
	logger *zerolog.Logger,
) bool {
	// Step 1: Copy terminal from template using pre-cached source ID.
	isExisting := false
	sourceId, ok := templateCache[j.TemplateSN]
	if !ok {
		logger.Warn().Int("row", j.RowNum).Str("template", j.TemplateSN).Msg("template SN not in cache, skipping")
		return false
	}

	copyResp, err := h.tmsClient.CopyTerminalById(session, sourceId, j.SerialNum)
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

	// Step 2: Resolve dest SN → old-API int ID (one call, reused for all steps).
	destId, err := h.tmsClient.GetIdFromSN(session, j.SerialNum)
	if err != nil {
		if !isExisting {
			_ = h.deleteTerminalOnError(session, j.SerialNum)
		}
		logger.Warn().Err(err).Int("row", j.RowNum).Msg("failed to resolve dest SN to ID")
		return false
	}

	// Step 3: Get terminal detail (old API) + app list — 2 calls total.
	detailData, appID, err := h.tmsClient.GetImportTerminalInfo(session, destId)
	if err != nil {
		if !isExisting {
			_ = h.deleteTerminalOnError(session, j.SerialNum)
		}
		logger.Warn().Err(err).Int("row", j.RowNum).Msg("failed to get terminal info")
		return false
	}
	if appID == "" {
		if !isExisting {
			_ = h.deleteTerminalOnError(session, j.SerialNum)
		}
		logger.Warn().Int("row", j.RowNum).Msg("target app not found on terminal")
		return false
	}

	// Step 4: Fetch parameter tabs concurrently (8 tabs in parallel ≈ 1 RTT).
	paramResp, err := h.tmsClient.GetParameterTabsConcurrent(session, destId, operationMark, appID, tabNames)
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
		logger.Warn().Int("row", j.RowNum).Msg("no parameters returned")
		return false
	}

	// Step 5: Update parameters using pre-resolved string ID (1 call).
	paraList := h.buildParaList(paramResp, j.Row)
	if len(paraList) > 0 {
		destIdStr := strconv.Itoa(destId)
		updateResp, err := h.tmsClient.UpdateParameterById(destIdStr, paraList, appID)
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

	// Step 6: Update device ID, merchant, and groups using pre-fetched detail (1 call).
	merchantIDInt, _ := strconv.Atoi(j.MerchantID)

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

	if j.MerchantID != "" || len(groupIDsInt) > 0 {
		updateDevResp, err := h.tmsClient.UpdateDeviceIdDirect(session, detailData, merchantIDInt, groupIDsInt, j.SerialNum)
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
// Uses the old session-based group/page API (which returns groupName) and falls
// back to the signed API if the old API fails.
func (h *ImportTerminalHandler) buildGroupMap(session string) map[string]int {
	logger := log.With().Str("task", "import:terminal").Logger()
	groupMap := map[string]int{}

	// Use the group/page API which returns groupName (same as resolveGroupNameToID).
	// Fetch up to 200 groups in a single call to cover most cases.
	result, err := h.tmsClient.GetGroupPage(session, 1, 200)
	if err == nil {
		for name, id := range result {
			groupMap[name] = id
		}
	} else {
		logger.Warn().Err(err).Msg("old API group/page failed")
	}

	// If old API returned no groups, try the signed (new) API as fallback.
	if len(groupMap) == 0 {
		logger.Info().Msg("trying signed API for group list")
		for page := 1; ; page++ {
			resp, err := h.tmsClient.GetGroupManageList("", page)
			if err != nil || resp.ResultCode != 0 || resp.Data == nil {
				break
			}
			groups, _ := resp.Data["groupList"].([]interface{})
			if len(groups) == 0 {
				break
			}
			for _, g := range groups {
				gm, _ := g.(map[string]interface{})
				if gm == nil {
					continue
				}
				// Try groupName first (signed API), then name, then label.
				name := tms.ToString(gm["groupName"])
				if name == "" {
					name = tms.ToString(gm["name"])
				}
				if name == "" {
					name = tms.ToString(gm["label"])
				}
				id := 0
				if idVal, ok := gm["id"]; ok {
					switch v := idVal.(type) {
					case float64:
						id = int(v)
					case int:
						id = v
					case string:
						id, _ = strconv.Atoi(v)
					}
				}
				if name != "" && id != 0 {
					groupMap[strings.ToLower(name)] = id
				}
			}
			totalPage := 0
			if tp, ok := resp.Data["totalPage"]; ok {
				switch v := tp.(type) {
				case float64:
					totalPage = int(v)
				case int:
					totalPage = v
				}
			}
			if page >= totalPage {
				break
			}
		}
	}

	if len(groupMap) == 0 {
		logger.Warn().Msg("no groups found from TMS API, group names in Excel will not be resolved")
	} else {
		// Log available group names for debugging.
		names := make([]string, 0, len(groupMap))
		for name := range groupMap {
			names = append(names, name)
		}
		logger.Info().Strs("available_groups", names).Int("count", len(groupMap)).Msg("group name→ID map loaded")
	}

	return groupMap
}

// deleteTerminalOnError attempts to delete a terminal that failed during import.
func (h *ImportTerminalHandler) deleteTerminalOnError(session, serialNum string) error {
	_, err := h.tmsClient.DeleteTerminal(session, serialNum)
	return err
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
