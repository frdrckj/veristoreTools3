package sync

import (
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/gorilla/sessions"
	"github.com/labstack/echo/v4"
	mw "github.com/verifone/veristoretools3/internal/middleware"
	"github.com/verifone/veristoretools3/internal/shared"
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
}

// NewHandler creates a new sync terminal handler.
func NewHandler(service *Service, store sessions.Store, sessionName, appName, appVersion string) *Handler {
	return &Handler{
		service:     service,
		store:       store,
		sessionName: sessionName,
		appName:     appName,
		appVersion:  appVersion,
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

// Create triggers a new sync by creating a DB record. In the future this will
// also enqueue an Asynq task.
func (h *Handler) Create(c echo.Context) error {
	userID := mw.GetCurrentUserID(c)
	userName := mw.GetCurrentUserName(c)

	s := &SyncTerminal{
		SyncTermCreatorID:   userID,
		SyncTermCreatorName: userName,
		SyncTermCreatedTime: time.Now(),
		SyncTermStatus:      "0", // Queued
		CreatedBy:           userName,
		CreatedDt:           time.Now(),
	}

	if err := h.service.CreateSync(s); err != nil {
		shared.SetFlash(c, h.store, h.sessionName, shared.FlashError, fmt.Sprintf("Failed to create sync: %v", err))
	} else {
		shared.SetFlash(c, h.store, h.sessionName, shared.FlashSuccess, "Sync terminal record created successfully")
	}

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
