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
	"github.com/rs/zerolog/log"
	"github.com/xuri/excelize/v2"
	"gorm.io/gorm"

	"github.com/verifone/veristoretools3/internal/admin"
	"github.com/verifone/veristoretools3/internal/tms"
)

const (
	// reportWorkerCount is the number of concurrent workers for checking
	// terminal apps. Each worker makes 1 API call per terminal (app list).
	// For matched terminals, 1 additional call (detail for PN).
	reportWorkerCount = 50

	// reportPageSize is the number of terminals per page when listing all
	// terminals. Larger = fewer page calls (500 pages for 25K terminals).
	reportPageSize = 50
)

// ReportTerminalPayload is the JSON payload for the report:terminal task.
type ReportTerminalPayload struct {
	UserID      int      `json:"user_id"`
	UserName    string   `json:"user_name"`
	AppVersion  string   `json:"app_version"`
	Session     string   `json:"session"`
	DateTime    string   `json:"date_time"`
	PackageName string   `json:"package_name"`
	TriggerSync bool     `json:"trigger_sync"`            // When true, chains to sync:parameter after report
	PartialCSIs []string `json:"partial_csis,omitempty"`   // When set, only process these CSIs (partial sync)
}

// reportJob represents a unit of work for a report worker.
type reportJob struct {
	Index      int
	TerminalID string // internal ID from the list API
	DeviceID   string // CSI / deviceId
	SN         string
	Model      string
	Merchant   string
	Status     string // pre-computed from alertStatus/alertMsg
}

// reportRow holds data for a single matched terminal in the report.
type reportRow struct {
	CSI        string
	SN         string
	PN         string
	Model      string
	Merchant   string
	Status     string
	AppVersion string // actual installed version of the target package
	AppID      string // appId for this terminal's version (for parameter fetch)
}

// ReportTerminalHandler generates a verification report of all terminals
// that have a specific app version installed (like v2 ReportTerminal.php).
// Optimized with parallel page fetching and a 50-worker concurrent pool.
// When TriggerSync is set, it chains to the sync:parameter job after report generation.
type ReportTerminalHandler struct {
	tmsService  *tms.Service
	tmsClient   *tms.Client
	adminRepo   *admin.Repository
	db          *gorm.DB
	queueClient *asynq.Client
}

// NewReportTerminalHandler creates a new handler for the report:terminal task.
func NewReportTerminalHandler(tmsService *tms.Service, tmsClient *tms.Client, adminRepo *admin.Repository, db *gorm.DB, queueClient *asynq.Client) *ReportTerminalHandler {
	return &ReportTerminalHandler{
		tmsService:  tmsService,
		tmsClient:   tmsClient,
		adminRepo:   adminRepo,
		db:          db,
		queueClient: queueClient,
	}
}

// ProcessTask implements asynq.Handler.
func (h *ReportTerminalHandler) ProcessTask(ctx context.Context, task *asynq.Task) error {
	var payload ReportTerminalPayload
	if err := json.Unmarshal(task.Payload(), &payload); err != nil {
		return fmt.Errorf("report_terminal: unmarshal payload: %w", err)
	}

	logger := log.With().Str("task", TaskReportTerminal).Str("app_version", payload.AppVersion).Logger()
	logger.Info().Msg("starting terminal report job")
	jobStartTime := time.Now()

	// Log job start to queue_log.
	createTime := strconv.FormatInt(time.Now().UnixMilli(), 10)
	_ = h.adminRepo.CreateQueueLog(&admin.QueueLog{
		CreateTime:  createTime,
		ExecTime:    createTime,
		ProcessName: "RPT",
		ServiceName: strPtr("report:terminal"),
	})

	session := payload.Session
	if session == "" {
		session = h.tmsService.GetSession()
	}

	// ------------------------------------------------------------------
	// Phase 1: Find the target app (1 API call).
	// ------------------------------------------------------------------
	appResp, err := h.tmsClient.GetAppList(session)
	if err != nil {
		h.adminRepo.FailPendingSyncs(payload.UserID)
		return fmt.Errorf("report_terminal: get app list: %w", err)
	}

	var appID string
	var appName string
	if appResp != nil && appResp.Data != nil {
		if allApps, ok := appResp.Data["allApps"].([]interface{}); ok {
			for _, a := range allApps {
				am, ok := a.(map[string]interface{})
				if !ok {
					continue
				}
				pkgName := fmt.Sprintf("%v", am["packageName"])
				version := fmt.Sprintf("%v", am["version"])
				if payload.PackageName != "" && pkgName != payload.PackageName {
					continue
				}
				// If AppVersion is specified, match exactly.
				// If empty, take the first matching app (latest version).
				if payload.AppVersion != "" && version != payload.AppVersion {
					continue
				}
				appID = fmt.Sprintf("%v", am["id"])
				appName = fmt.Sprintf("%v", am["name"])
				if payload.AppVersion == "" {
					payload.AppVersion = version
				}
				break
			}
		}
	}

	if appID == "" {
		logger.Warn().Msg("target app version not found in app list")
		h.adminRepo.CompletePendingSyncs(payload.UserID)
		return nil
	}

	sheetName := appName + "_" + payload.AppVersion
	logger.Info().Str("app_id", appID).Str("sheet", sheetName).Msg("found target app")

	// ------------------------------------------------------------------
	// Phase 2: Collect terminals.
	// - Partial mode: look up only the specified CSIs via search API.
	// - Full mode: fetch all pages from TMS concurrently.
	// ------------------------------------------------------------------
	h.adminRepo.UpdateSyncProcess(payload.UserID, "0", "1") // status 1 = Processing

	var allJobs []reportJob
	totalTerminals := 0
	totalPage := 0

	if len(payload.PartialCSIs) > 0 {
		// --- Partial mode: search each CSI individually ---
		logger.Info().Int("partial_count", len(payload.PartialCSIs)).Msg("partial mode: looking up specific CSIs")
		session := h.tmsService.GetSession()
		for _, csi := range payload.PartialCSIs {
			resp, err := h.tmsClient.GetTerminalListSearch(session, 1, csi, 4) // queryType 4 = CSI
			if err != nil || resp == nil || resp.Data == nil {
				logger.Warn().Str("csi", csi).Err(err).Msg("partial: failed to search CSI")
				continue
			}
			jobs := h.extractTerminalsFromPage(resp, len(allJobs))
			allJobs = append(allJobs, jobs...)
		}
		totalTerminals = len(allJobs)
		totalPage = 1
		logger.Info().Int("total_terminals", totalTerminals).Msg("partial terminal collection complete")
	} else {
		// --- Full mode: fetch all terminals from TMS ---
		// Fetch first page to get totalPage count.
		firstResp, err := h.tmsClient.GetTerminalListWithSize(1, reportPageSize)
		if err != nil || firstResp == nil || firstResp.Data == nil {
			logger.Error().Err(err).Msg("failed to get first terminal list page")
			h.adminRepo.FailPendingSyncs(payload.UserID)
			return fmt.Errorf("report_terminal: get terminal list page 1: %w", err)
		}

		totalPage = 1
		if tp, ok := firstResp.Data["totalPage"]; ok {
			if tpInt, err := strconv.Atoi(fmt.Sprintf("%v", tp)); err == nil && tpInt > 0 {
				totalPage = tpInt
			}
		}
		if tt, ok := firstResp.Data["total"]; ok {
			if ttInt, err := strconv.Atoi(fmt.Sprintf("%v", tt)); err == nil && ttInt > 0 {
				totalTerminals = ttInt
			}
		}

		// Process first page terminals.
		allJobs = append(allJobs, h.extractTerminalsFromPage(firstResp, len(allJobs))...)

		logger.Info().Int("total_pages", totalPage).Int("total_terminals", totalTerminals).Msg("fetching remaining pages concurrently")

		// Fetch remaining pages concurrently.
		if totalPage > 1 {
			type pageResult struct {
				Page      int
				Terminals []reportJob
				Err       error
			}

			pageResults := make([]pageResult, totalPage-1)
			var pageWg sync.WaitGroup

			for page := 2; page <= totalPage; page++ {
				select {
				case <-ctx.Done():
					h.adminRepo.FailPendingSyncs(payload.UserID)
					return ctx.Err()
				default:
				}

				pageWg.Add(1)
				go func(p int) {
					defer pageWg.Done()
					resp, err := h.tmsClient.GetTerminalListWithSize(p, reportPageSize)
					idx := p - 2 // pages 2..N map to index 0..N-2
					if err != nil || resp == nil || resp.Data == nil {
						pageResults[idx] = pageResult{Page: p, Err: err}
						return
					}
					pageResults[idx] = pageResult{
						Page:      p,
						Terminals: h.extractTerminalsFromPage(resp, 0), // index set later
					}
				}(page)
			}

			pageWg.Wait()

			// Collect terminals from all pages.
			for _, pr := range pageResults {
				if pr.Err != nil {
					logger.Warn().Int("page", pr.Page).Err(pr.Err).Msg("failed to get terminal list page")
					continue
				}
				for i := range pr.Terminals {
					pr.Terminals[i].Index = len(allJobs) + i
				}
				allJobs = append(allJobs, pr.Terminals...)
			}
		}

		if totalTerminals == 0 {
			totalTerminals = len(allJobs)
		}

		logger.Info().Int("total_terminals", len(allJobs)).Msg("terminal collection complete")
	}

	if len(allJobs) == 0 {
		logger.Warn().Msg("no terminals found")
		// Still chain to sync:parameter if TriggerSync is true so Phase 3/4 can run
		// (e.g., to sync all TMS terminals and delete today's removed CSIs).
		if payload.TriggerSync {
			logger.Info().Msg("chaining to sync:parameter despite no report matches (for Phase 3/4)")
			syncPayload := SyncParameterPayload{
				ReportName: "",
				Session:    payload.Session,
				UserID:     payload.UserID,
				UserName:   payload.UserName,
				AppVersion: payload.AppVersion,
				IsPartial:  len(payload.PartialCSIs) > 0,
			}
			syncPayloadBytes, _ := json.Marshal(syncPayload)
			syncTask := asynq.NewTask(TaskSyncParameter, syncPayloadBytes)
			if _, err := h.queueClient.Enqueue(syncTask, asynq.Timeout(5*time.Hour), asynq.MaxRetry(0)); err != nil {
				logger.Error().Err(err).Msg("failed to enqueue sync:parameter job")
				h.adminRepo.FailPendingSyncs(payload.UserID)
			}
		} else {
			h.adminRepo.CompletePendingSyncs(payload.UserID)
		}
		return nil
	}

	// ------------------------------------------------------------------
	// Phase 3: Check apps concurrently using worker pool.
	//
	// Version resolution uses the same hybrid logic as the edit page
	// (handler.go Edit):
	//   1. GetTerminalAppsById → check if requested version is in the
	//      pushed list. If not → skip (1 API call).
	//   2. GetTerminalDetailById → check appInstalls for the package.
	//      If found → that's the actual installed version (ground truth).
	//   3. If appInstalls empty → use highest pushed version as fallback.
	//   4. Match if resolved version == requested version.
	// ------------------------------------------------------------------
	jobs := make(chan reportJob, len(allJobs))
	var mu sync.Mutex
	var rows []reportRow
	var processedCount int64
	var matchedCount int64
	var wg sync.WaitGroup

	// Cancellation channel: closed when sync is reset/cancelled by user.
	cancelCh := make(chan struct{})
	go func() {
		ticker := time.NewTicker(3 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-cancelCh:
				return
			case <-ticker.C:
				if h.adminRepo.IsSyncCancelled(payload.UserID) {
					logger.Info().Msg("update cancelled by user (sync reset)")
					close(cancelCh)
					return
				}
			}
		}
	}()

	for w := 0; w < reportWorkerCount; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := range jobs {
				select {
				case <-ctx.Done():
					return
				case <-cancelCh:
					return
				default:
				}

				// Step 1: Get pushed app list (1 API call).
				apps, fetchErr := h.tmsClient.GetTerminalAppsById(j.TerminalID)
				if fetchErr != nil {
					logger.Debug().Str("terminal_id", j.TerminalID).Err(fetchErr).Msg("failed to get terminal apps")
					count := atomic.AddInt64(&processedCount, 1)
					if count%100 == 0 || int(count) == totalTerminals {
						logger.Info().Int64("scanned", count).Int("total", totalTerminals).Int64("matched", atomic.LoadInt64(&matchedCount)).Msg("report: scan progress")
					}
					h.reportProgress(payload.UserID, int(count), totalTerminals)
					continue
				}

				// Check if the requested version exists in the pushed list,
				// and find the highest version for fallback.
				requestedFound := false
				requestedAppID := ""
				highestVer := ""
				highestAppID := ""
				for _, app := range apps {
					aPkg := fmt.Sprintf("%v", app["packageName"])
					aVer := fmt.Sprintf("%v", app["version"])
					aID := fmt.Sprintf("%v", app["id"])
					logger.Debug().Str("csi", j.DeviceID).Str("pkg", aPkg).Str("ver", aVer).Str("id", aID).Str("want_pkg", payload.PackageName).Str("want_ver", payload.AppVersion).Msg("checking app")
					if payload.PackageName != "" && aPkg != payload.PackageName {
						continue
					}
					if aVer == payload.AppVersion {
						requestedFound = true
						requestedAppID = aID
					}
					if highestVer == "" || compareVersions(aVer, highestVer) > 0 {
						highestVer = aVer
						highestAppID = aID
					}
				}
				if !requestedFound {
					logger.Debug().Str("csi", j.DeviceID).Int("app_count", len(apps)).Str("highest_ver", highestVer).Msg("requested version not in pushed list")
				}

				// If the requested version isn't even in the pushed list, skip.
				if !requestedFound {
					count := atomic.AddInt64(&processedCount, 1)
					if count%100 == 0 || int(count) == totalTerminals {
						logger.Info().Int64("scanned", count).Int("total", totalTerminals).Int64("matched", atomic.LoadInt64(&matchedCount)).Msg("report: scan progress")
					}
					h.reportProgress(payload.UserID, int(count), totalTerminals)
					continue
				}

				// Step 2: Get terminal detail for appInstalls + PN (1 API call).
				detailResp, detailErr := h.tmsClient.GetTerminalDetailById(j.TerminalID)
				pn := ""
				matched := false
				matchedAppID := ""

				if detailErr == nil && detailResp != nil && detailResp.Data != nil {
					if v, ok := detailResp.Data["pn"]; ok && v != nil {
						pn = fmt.Sprintf("%v", v)
					}

					// Step 3: Check appInstalls for ground truth version.
					installedVersion := ""
					if installs, ok := detailResp.Data["appInstalls"].([]interface{}); ok {
						for _, inst := range installs {
							im, _ := inst.(map[string]interface{})
							if im == nil {
								continue
							}
							iPkg := fmt.Sprintf("%v", im["packageName"])
							if iPkg == payload.PackageName {
								installedVersion = fmt.Sprintf("%v", im["version"])
								break
							}
						}
					}

					if installedVersion != "" {
						// appInstalls has ground truth → match only if it equals requested.
						if installedVersion == payload.AppVersion {
							matched = true
							matchedAppID = requestedAppID
						}
					} else {
						// appInstalls empty → use highest pushed version as fallback.
						if highestVer == payload.AppVersion {
							matched = true
							matchedAppID = highestAppID
						}
					}
				} else {
					// Detail call failed → fall back to highest version.
					if highestVer == payload.AppVersion {
						matched = true
						matchedAppID = highestAppID
					}
				}

				if matched {
					atomic.AddInt64(&matchedCount, 1)
					mu.Lock()
					rows = append(rows, reportRow{
						CSI:        j.DeviceID,
						SN:         j.SN,
						PN:         pn,
						Model:      j.Model,
						Merchant:   j.Merchant,
						Status:     j.Status,
						AppVersion: payload.AppVersion,
						AppID:      matchedAppID,
					})
					mu.Unlock()
					logger.Debug().Str("csi", j.DeviceID).Str("version", payload.AppVersion).Msg("terminal matched")
				}

				count := atomic.AddInt64(&processedCount, 1)
				if count%100 == 0 || int(count) == totalTerminals {
					logger.Info().Int64("scanned", count).Int("total", totalTerminals).Int64("matched", atomic.LoadInt64(&matchedCount)).Msg("report: scan progress")
				}
				h.reportProgress(payload.UserID, int(count), totalTerminals)
			}
		}()
	}

	// Send all jobs (stop early if cancelled).
sendLoop:
	for _, j := range allJobs {
		select {
		case <-cancelCh:
			break sendLoop
		case jobs <- j:
		}
	}
	close(jobs)

	// Wait for all workers to finish.
	wg.Wait()

	// Check if cancelled by user.
	select {
	case <-cancelCh:
		logger.Info().Int("processed", int(processedCount)).Int("total", len(allJobs)).Str("elapsed", time.Since(jobStartTime).Round(time.Millisecond).String()).Msg("update stopped: cancelled by user")
		return nil
	default:
	}

	if ctx.Err() != nil {
		h.adminRepo.FailPendingSyncs(payload.UserID)
		return ctx.Err()
	}

	logger.Info().Int("matched", len(rows)).Int("total_terminals", len(allJobs)).Str("elapsed", time.Since(jobStartTime).Round(time.Millisecond).String()).Msg("terminal scan complete")

	if len(rows) == 0 {
		logger.Warn().Msg("no terminals matched the app version")
		h.adminRepo.CompletePendingSyncs(payload.UserID)
		return nil
	}

	// ------------------------------------------------------------------
	// Phase 4: Generate Excel report.
	// ------------------------------------------------------------------
	dt, _ := time.Parse("2006-01-02 15:04:05", payload.DateTime)
	if dt.IsZero() {
		dt = time.Now()
	}
	reportName := fmt.Sprintf("%d_%s.xlsx", payload.UserID, dt.Format("20060102150405"))

	f := excelize.NewFile()
	defer f.Close()

	// Rename default sheet.
	oldSheet := f.GetSheetName(0)
	if err := f.SetSheetName(oldSheet, sheetName); err != nil {
		f.NewSheet(sheetName)
	}

	// Header row with APPID.
	f.SetCellValue(sheetName, "A1", "APPID:"+appID)
	headerStyle, _ := f.NewStyle(&excelize.Style{Font: &excelize.Font{Bold: true}})
	f.SetRowStyle(sheetName, 1, 1, headerStyle)

	// Column headers.
	headers := []string{"CSI", "SN", "PN", "Model", "Merchant", "Status", "AppVersion", "AppID"}
	for i, hdr := range headers {
		cell, _ := excelize.CoordinatesToCellName(i+1, 2)
		f.SetCellValue(sheetName, cell, hdr)
	}
	f.SetRowStyle(sheetName, 2, 2, headerStyle)

	// Data rows.
	for i, row := range rows {
		rowNum := i + 3
		f.SetCellValue(sheetName, fmt.Sprintf("A%d", rowNum), row.CSI)
		f.SetCellValue(sheetName, fmt.Sprintf("B%d", rowNum), row.SN)
		f.SetCellValue(sheetName, fmt.Sprintf("C%d", rowNum), row.PN)
		f.SetCellValue(sheetName, fmt.Sprintf("D%d", rowNum), row.Model)
		f.SetCellValue(sheetName, fmt.Sprintf("E%d", rowNum), row.Merchant)
		f.SetCellValue(sheetName, fmt.Sprintf("F%d", rowNum), row.Status)
		f.SetCellValue(sheetName, fmt.Sprintf("G%d", rowNum), row.AppVersion)
		f.SetCellValue(sheetName, fmt.Sprintf("H%d", rowNum), row.AppID)
	}

	// Write to buffer and store in tms_report.
	buf, err := f.WriteToBuffer()
	if err != nil {
		h.adminRepo.FailPendingSyncs(payload.UserID)
		return fmt.Errorf("report_terminal: write excel: %w", err)
	}

	rpt := &admin.TmsReport{
		TmsRptName:      reportName,
		TmsRptFile:      buf.Bytes(),
		TmsRptTotalPage: strconv.Itoa(len(rows)/10 + 1),
		TmsRptCurPage:   strconv.Itoa(totalPage),
	}
	if err := h.adminRepo.CreateTmsReport(rpt); err != nil {
		h.adminRepo.FailPendingSyncs(payload.UserID)
		return fmt.Errorf("report_terminal: save report: %w", err)
	}

	logger.Info().Str("file", reportName).Int("rows", len(rows)).Str("elapsed", time.Since(jobStartTime).Round(time.Millisecond).String()).Msg("report generated successfully")

	// If TriggerSync is set (Sekarang button), chain to sync:parameter
	// to read the Excel and update local terminal/terminal_parameter tables.
	if payload.TriggerSync && h.queueClient != nil {
		syncPayload := SyncParameterPayload{
			ReportName: reportName,
			Session:    session,
			UserID:     payload.UserID,
			UserName:   payload.UserName,
			AppID:      appID,
			AppName:    appName,
			AppVersion: payload.AppVersion,
			IsPartial:  len(payload.PartialCSIs) > 0,
		}
		syncPayloadBytes, _ := json.Marshal(syncPayload)
		syncTask := asynq.NewTask(TaskSyncParameter, syncPayloadBytes)
		if _, err := h.queueClient.Enqueue(syncTask, asynq.Timeout(5*time.Hour), asynq.MaxRetry(0)); err != nil {
			logger.Error().Err(err).Msg("failed to enqueue sync:parameter job")
			h.adminRepo.FailPendingSyncs(payload.UserID)
			return fmt.Errorf("report_terminal: enqueue sync: %w", err)
		}
		// Set status to "2" (Sinkronisasi) — the sync:parameter job will set "3" when done.
		h.adminRepo.UpdateSyncProcess(payload.UserID, "0", "2")
		logger.Info().Msg("chained sync:parameter job enqueued")
		return nil
	}

	// No sync requested (Update button) — mark as complete directly.
	h.adminRepo.CompletePendingSyncs(payload.UserID)
	return nil
}

// extractTerminalsFromPage extracts terminal metadata from a terminal list API
// response and returns them as reportJob structs.
func (h *ReportTerminalHandler) extractTerminalsFromPage(resp *tms.TMSResponse, startIndex int) []reportJob {
	terminalList, _ := resp.Data["terminalList"].([]interface{})
	var jobs []reportJob

	for _, t := range terminalList {
		tm, ok := t.(map[string]interface{})
		if !ok {
			continue
		}

		terminalID := fmt.Sprintf("%v", tm["id"])
		deviceID := fmt.Sprintf("%v", tm["deviceId"])
		if terminalID == "" || terminalID == "<nil>" {
			continue
		}

		// Pre-compute status from alertStatus/alertMsg.
		status := "connected"
		if s, _ := strconv.Atoi(fmt.Sprintf("%v", tm["status"])); s != 1 {
			alertMsg := fmt.Sprintf("%v", tm["alertMsg"])
			if alertMsg != "" && alertMsg != "<nil>" {
				status = alertMsg
			} else {
				status = "disconnected"
			}
		}

		jobs = append(jobs, reportJob{
			Index:      startIndex + len(jobs),
			TerminalID: terminalID,
			DeviceID:   deviceID,
			SN:         fmt.Sprintf("%v", tm["sn"]),
			Model:      fmt.Sprintf("%v", tm["model"]),
			Merchant:   fmt.Sprintf("%v", tm["merchantName"]),
			Status:     status,
		})
	}

	return jobs
}

// reportProgress updates the sync_term_process field every 5 terminals
// to reduce database writes while still showing progress.
// Stores a percentage (0-100) so the view template can display a progress bar.
func (h *ReportTerminalHandler) reportProgress(userID, current, total int) {
	if current%5 == 0 || current == total {
		pct := 0
		if total > 0 {
			pct = (current * 100) / total
		}
		process := strconv.Itoa(pct)
		h.adminRepo.UpdateSyncProcess(userID, process, "1")
	}
}

// compareVersions compares two dot-separated version strings (e.g. "4.3.0.0").
// Returns >0 if a > b, <0 if a < b, 0 if equal.
func compareVersions(a, b string) int {
	aParts := strings.Split(a, ".")
	bParts := strings.Split(b, ".")
	maxLen := len(aParts)
	if len(bParts) > maxLen {
		maxLen = len(bParts)
	}
	for i := 0; i < maxLen; i++ {
		var av, bv int
		if i < len(aParts) {
			av, _ = strconv.Atoi(aParts[i])
		}
		if i < len(bParts) {
			bv, _ = strconv.Atoi(bParts[i])
		}
		if av != bv {
			return av - bv
		}
	}
	return 0
}

