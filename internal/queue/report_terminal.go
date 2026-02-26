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
	UserID      int    `json:"user_id"`
	UserName    string `json:"user_name"`
	AppVersion  string `json:"app_version"`
	Session     string `json:"session"`
	DateTime    string `json:"date_time"`
	PackageName string `json:"package_name"`
	TriggerSync bool   `json:"trigger_sync"` // When true, chains to sync:parameter after report
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
	// Phase 2: Collect all terminals with parallel page fetching.
	// Fetch page 1 to discover totalPage, then fetch remaining pages
	// concurrently.
	// ------------------------------------------------------------------
	h.adminRepo.UpdateSyncProcess(payload.UserID, "0", "1") // status 1 = Processing

	var allJobs []reportJob
	totalTerminals := 0

	// Fetch first page to get totalPage count.
	firstResp, err := h.tmsClient.GetTerminalListWithSize(1, reportPageSize)
	if err != nil || firstResp == nil || firstResp.Data == nil {
		logger.Error().Err(err).Msg("failed to get first terminal list page")
		h.adminRepo.FailPendingSyncs(payload.UserID)
		return fmt.Errorf("report_terminal: get terminal list page 1: %w", err)
	}

	totalPage := 1
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

	logger.Info().Int("total_terminals", len(allJobs)).Int("total_pages", totalPage).Msg("terminal collection complete")

	if len(allJobs) == 0 {
		logger.Warn().Msg("no terminals found")
		h.adminRepo.CompletePendingSyncs(payload.UserID)
		return nil
	}

	// ------------------------------------------------------------------
	// Phase 3: Check apps concurrently using worker pool.
	//
	// Hybrid matching to ensure each terminal appears in exactly ONE
	// version report (no duplicates across versions):
	//
	//  1. Get terminal detail (appInstalls + PN) — 1 API call.
	//  2. If appInstalls has the target package → use that version.
	//     appInstalls is the ground truth (what's actually installed).
	//  3. If appInstalls is empty (disconnected terminal, never reported) →
	//     fall back to itemList, but only match the HIGHEST version
	//     (the latest push is what the terminal will run when online).
	//     This prevents the terminal from appearing in every version
	//     that was ever pushed to it.
	// ------------------------------------------------------------------
	jobs := make(chan reportJob, len(allJobs))
	var mu sync.Mutex
	var rows []reportRow
	var processedCount int64
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

				// Step 1: Get terminal detail → appInstalls + PN.
				detailResp, err := h.tmsClient.GetTerminalDetailById(j.TerminalID)
				if err != nil || detailResp == nil || detailResp.Data == nil {
					logger.Debug().Str("terminal_id", j.TerminalID).Err(err).Msg("failed to get terminal detail")
					count := atomic.AddInt64(&processedCount, 1)
					h.reportProgress(payload.UserID, int(count), totalTerminals)
					continue
				}

				pn := ""
				if v, ok := detailResp.Data["pn"]; ok && v != nil {
					pn = fmt.Sprintf("%v", v)
				}

				// Step 2: Check appInstalls for the target package.
				installedVersion := ""
				if installs, ok := detailResp.Data["appInstalls"].([]interface{}); ok {
					for _, inst := range installs {
						im, _ := inst.(map[string]interface{})
						if im == nil {
							continue
						}
						iPkg := fmt.Sprintf("%v", im["packageName"])
						if payload.PackageName != "" && iPkg == payload.PackageName {
							installedVersion = fmt.Sprintf("%v", im["version"])
							break
						}
					}
				}

				matched := false
				matchedVersion := ""
				matchedAppID := ""

				if installedVersion != "" {
					// appInstalls has the package → definitive installed version.
					if installedVersion == payload.AppVersion {
						matched = true
						matchedVersion = installedVersion
					}
				} else {
					// appInstalls empty (disconnected terminal) → fall back to
					// itemList, but only match the HIGHEST version to prevent
					// the terminal from appearing in multiple version reports.
					apps, appErr := h.tmsClient.GetTerminalAppsById(j.TerminalID)
					if appErr == nil {
						highestVer := ""
						highestAppID := ""
						for _, app := range apps {
							aPkg := fmt.Sprintf("%v", app["packageName"])
							if payload.PackageName != "" && aPkg != payload.PackageName {
								continue
							}
							aVer := fmt.Sprintf("%v", app["version"])
							if highestVer == "" || compareVersions(aVer, highestVer) > 0 {
								highestVer = aVer
								highestAppID = fmt.Sprintf("%v", app["id"])
							}
						}
						// Only match if the requested version IS the highest version.
						if highestVer == payload.AppVersion {
							matched = true
							matchedVersion = highestVer
							matchedAppID = highestAppID
						}
					}
				}

				// For appInstalls match, we still need the appID from itemList.
				if matched && matchedAppID == "" {
					apps, appErr := h.tmsClient.GetTerminalAppsById(j.TerminalID)
					if appErr == nil {
						for _, app := range apps {
							aPkg := fmt.Sprintf("%v", app["packageName"])
							aVer := fmt.Sprintf("%v", app["version"])
							if aPkg == payload.PackageName && aVer == matchedVersion {
								matchedAppID = fmt.Sprintf("%v", app["id"])
								break
							}
						}
					}
				}

				if matched {
					mu.Lock()
					rows = append(rows, reportRow{
						CSI:        j.DeviceID,
						SN:         j.SN,
						PN:         pn,
						Model:      j.Model,
						Merchant:   j.Merchant,
						Status:     j.Status,
						AppVersion: matchedVersion,
						AppID:      matchedAppID,
					})
					mu.Unlock()
					logger.Debug().Str("csi", j.DeviceID).Str("version", matchedVersion).Msg("terminal matched")
				}

				count := atomic.AddInt64(&processedCount, 1)
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
		}
		syncPayloadBytes, _ := json.Marshal(syncPayload)
		syncTask := asynq.NewTask(TaskSyncParameter, syncPayloadBytes)
		if _, err := h.queueClient.Enqueue(syncTask); err != nil {
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
