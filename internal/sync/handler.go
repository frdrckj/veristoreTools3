package sync

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/gorilla/sessions"
	"github.com/hibiken/asynq"
	"github.com/labstack/echo/v4"
	mw "github.com/verifone/veristoretools3/internal/middleware"
	"github.com/verifone/veristoretools3/internal/shared"
	"github.com/verifone/veristoretools3/internal/tms"
	"github.com/verifone/veristoretools3/templates/components"
	"github.com/verifone/veristoretools3/templates/layouts"
	syncTmpl "github.com/verifone/veristoretools3/templates/sync"
)

// Handler holds dependencies for sync terminal HTTP handlers.
type Handler struct {
	service     *Service
	store       sessions.Store
	sessionName string
	appName     string
	appVersion  string
	tmsService  *tms.Service
	queueClient *asynq.Client
	packageName string
}

// NewHandler creates a new sync terminal handler.
func NewHandler(service *Service, store sessions.Store, sessionName, appName, appVersion string, tmsService *tms.Service, queueClient *asynq.Client, packageName string) *Handler {
	return &Handler{
		service:     service,
		store:       store,
		sessionName: sessionName,
		appName:     appName,
		appVersion:  appVersion,
		tmsService:  tmsService,
		queueClient: queueClient,
		packageName: packageName,
	}
}

// pageData builds a layouts.PageData from the echo context and handler config.
func (h *Handler) pageData(c echo.Context, title string) layouts.PageData {
	flashes := shared.GetFlashes(c, h.store, h.sessionName)
	return layouts.PageData{
		Title:            title,
		AppName:          h.appName,
		AppVersion:       h.appVersion,
		AppIcon:          "favicon.png",
		AppLogo:          "verifone_logo.png",
		AppVeristoreLogo: "veristore_logo.png",
		UserName:         mw.GetCurrentUserName(c),
		UserFullname:     mw.GetCurrentUserFullname(c),
		UserPrivileges:   mw.GetCurrentUserPrivileges(c),
		CopyrightTitle:   "Verifone",
		CopyrightURL:     "https://www.verifone.com",
		Flashes:          flashes,
	}
}

// toSyncData converts a SyncTerminal model to a SyncData view struct.
func toSyncData(s SyncTerminal) syncTmpl.SyncData {
	process := ""
	if s.SyncTermProcess != nil {
		process = *s.SyncTermProcess
	}
	return syncTmpl.SyncData{
		SyncTermID:          s.SyncTermID,
		SyncTermCreatorID:   s.SyncTermCreatorID,
		SyncTermCreatorName: s.SyncTermCreatorName,
		SyncTermCreatedTime: s.SyncTermCreatedTime.Format("2006-01-02 15:04:05"),
		SyncTermStatus:      s.SyncTermStatus,
		SyncTermProcess:     process,
		CreatedBy:           s.CreatedBy,
		CreatedDt:           s.CreatedDt.Format("2006-01-02 15:04:05"),
	}
}

// toSyncDataSlice converts a slice of SyncTerminal models to SyncData view structs.
func toSyncDataSlice(syncs []SyncTerminal) []syncTmpl.SyncData {
	result := make([]syncTmpl.SyncData, len(syncs))
	for i, s := range syncs {
		result[i] = toSyncData(s)
	}
	return result
}

// Index lists sync terminal records with pagination and column filters.
// Supports HTMX partial updates.
func (h *Handler) Index(c echo.Context) error {
	pageNum, _ := strconv.Atoi(c.QueryParam("page"))
	if pageNum < 1 {
		pageNum = 1
	}

	perPage := 20

	// Parse filter/search params from query string.
	filters := SyncSearchFilter{
		CreatorName: c.QueryParam("creator_name"),
		CreatedTime: c.QueryParam("created_time"),
		Status:      c.QueryParam("status"),
		SyncedBy:    c.QueryParam("synced_by"),
		SyncedDate:  c.QueryParam("synced_date"),
	}

	syncs, pagination, err := h.service.repo.SearchWithFilters(filters, pageNum, perPage)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to load sync records")
	}

	// Build template search params (for pre-filling filter inputs).
	tmplSearch := syncTmpl.SyncSearchParams{
		CreatorName: filters.CreatorName,
		CreatedTime: filters.CreatedTime,
		Status:      filters.Status,
		SyncedBy:    filters.SyncedBy,
		SyncedDate:  filters.SyncedDate,
	}

	paginationData := components.PaginationData{
		CurrentPage: pagination.CurrentPage,
		TotalPages:  pagination.TotalPages,
		Total:       pagination.Total,
		PerPage:     perPage,
		BaseURL:     "/sync-terminal/index",
		HTMXTarget:  "sync-table-container",
		QueryString: tmplSearch.QueryString(),
	}

	// Check if there's a pending sync process (disables the "Sekarang" button).
	syncProcess := h.service.HasPendingSync()

	syncData := toSyncDataSlice(syncs)

	// For HTMX requests, return only the table partial.
	if shared.IsHTMX(c) {
		return shared.Render(c, http.StatusOK, syncTmpl.SyncTablePartial(syncData, paginationData, tmplSearch))
	}

	page := h.pageData(c, "Sinkronisasi Data CSI")
	return shared.Render(c, http.StatusOK, syncTmpl.IndexPage(page, syncData, paginationData, tmplSearch, syncProcess))
}

// View displays a sync terminal record detail by ID.
func (h *Handler) View(c echo.Context) error {
	id, err := strconv.Atoi(c.QueryParam("id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid sync ID")
	}

	s, err := h.service.GetSyncByID(id)
	if err != nil {
		return echo.NewHTTPError(http.StatusNotFound, "sync record not found")
	}

	page := h.pageData(c, "Sync Terminal Detail")
	return shared.Render(c, http.StatusOK, syncTmpl.ViewPage(page, toSyncData(*s)))
}

// syncReportPayload mirrors the queue.ReportTerminalPayload to avoid import cycles.
type syncReportPayload struct {
	UserID      int    `json:"user_id"`
	UserName    string `json:"user_name"`
	AppVersion  string `json:"app_version"`
	Session     string `json:"session"`
	DateTime    string `json:"date_time"`
	PackageName string `json:"package_name"`
}

// Create triggers a new sync by creating a DB record and enqueuing the
// report:terminal background job to scan terminals and generate a report.
// Uses the latest app version for the configured package name.
func (h *Handler) Create(c echo.Context) error {
	userID := mw.GetCurrentUserID(c)
	userName := mw.GetCurrentUserFullname(c)

	// Find the latest version for the configured package from TMS.
	session := h.tmsService.GetSession()
	appVersion := ""
	appResp, err := h.tmsService.GetAppList()
	if err == nil && appResp != nil && appResp.Data != nil {
		if allApps, ok := appResp.Data["allApps"].([]interface{}); ok {
			for _, a := range allApps {
				am, ok := a.(map[string]interface{})
				if !ok {
					continue
				}
				pkgName := fmt.Sprintf("%v", am["packageName"])
				if h.packageName != "" && pkgName != h.packageName {
					continue
				}
				// Take the first matching app's version (latest in list).
				appVersion = fmt.Sprintf("%v", am["version"])
				break
			}
		}
	}

	if appVersion == "" {
		shared.SetFlash(c, h.store, h.sessionName, shared.FlashError, "Failed to find app version from TMS. Please use the Update page instead.")
		return c.Redirect(http.StatusFound, "/sync-terminal/index")
	}

	// Create SyncTerminal record so buttons are disabled immediately.
	now := time.Now()
	s := &SyncTerminal{
		SyncTermCreatorID:   userID,
		SyncTermCreatorName: userName,
		SyncTermCreatedTime: now,
		SyncTermStatus:      "0", // Queued
		CreatedBy:           userName,
		CreatedDt:           now,
	}
	if err := h.service.CreateSync(s); err != nil {
		shared.SetFlash(c, h.store, h.sessionName, shared.FlashError, fmt.Sprintf("Failed to create sync record: %v", err))
		return c.Redirect(http.StatusFound, "/sync-terminal/index")
	}

	// Enqueue the report:terminal background job.
	payload := syncReportPayload{
		UserID:      userID,
		UserName:    userName,
		AppVersion:  appVersion,
		Session:     session,
		DateTime:    now.Format("2006-01-02 15:04:05"),
		PackageName: h.packageName,
	}
	payloadBytes, _ := json.Marshal(payload)
	task := asynq.NewTask("report:terminal", payloadBytes)
	if _, err := h.queueClient.Enqueue(task); err != nil {
		shared.SetFlash(c, h.store, h.sessionName, shared.FlashError, fmt.Sprintf("Failed to enqueue report job: %v", err))
		return c.Redirect(http.StatusFound, "/sync-terminal/index")
	}

	shared.SetFlash(c, h.store, h.sessionName, shared.FlashSuccess, "Sinkronisasi dimulai. Report akan tersedia dalam beberapa saat.")
	return c.Redirect(http.StatusFound, "/sync-terminal/index")
}

// Delete removes a sync terminal record by ID.
func (h *Handler) Delete(c echo.Context) error {
	id, err := strconv.Atoi(c.QueryParam("id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid sync ID")
	}

	if err := h.service.DeleteSync(id); err != nil {
		shared.SetFlash(c, h.store, h.sessionName, shared.FlashError, "Failed to delete sync record")
	} else {
		shared.SetFlash(c, h.store, h.sessionName, shared.FlashSuccess, "Sync record deleted successfully")
	}

	return c.Redirect(http.StatusFound, "/sync-terminal/index")
}

// Download serves the XLSX report file associated with the given sync record.
// The report is stored in the tms_report table, linked by user ID and timestamp.
func (h *Handler) Download(c echo.Context) error {
	id, err := strconv.Atoi(c.QueryParam("id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid sync ID")
	}

	syncRec, err := h.service.GetSyncByID(id)
	if err != nil {
		return echo.NewHTTPError(http.StatusNotFound, "sync record not found")
	}

	fileData, fileName, err := h.service.GetReportFileForSync(syncRec)
	if err != nil {
		return echo.NewHTTPError(http.StatusNotFound, fmt.Sprintf("Report file not available: %v", err))
	}

	c.Response().Header().Set("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	c.Response().Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, fileName))
	return c.Blob(http.StatusOK, "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet", fileData)
}

// Reset resets all sync terminal statuses to "3".
func (h *Handler) Reset(c echo.Context) error {
	if err := h.service.ResetAllSync(); err != nil {
		shared.SetFlash(c, h.store, h.sessionName, shared.FlashError, fmt.Sprintf("Failed to reset sync statuses: %v", err))
	} else {
		shared.SetFlash(c, h.store, h.sessionName, shared.FlashSuccess, "All sync statuses have been reset")
	}

	return c.Redirect(http.StatusFound, "/sync-terminal/index")
}
