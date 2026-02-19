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

// Index lists sync terminal records with pagination. Supports HTMX partial updates.
func (h *Handler) Index(c echo.Context) error {
	pageNum, _ := strconv.Atoi(c.QueryParam("page"))
	if pageNum < 1 {
		pageNum = 1
	}

	syncs, pagination, err := h.service.repo.Search("", pageNum, 20)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to load sync records")
	}

	paginationData := components.PaginationData{
		CurrentPage: pagination.CurrentPage,
		TotalPages:  pagination.TotalPages,
		Total:       pagination.Total,
		BaseURL:     "/sync-terminal/index",
		HTMXTarget:  "sync-table-container",
	}

	// Get last sync time.
	lastSync, _ := h.service.GetLastSyncTime()
	lastSyncStr := ""
	if lastSync != nil {
		lastSyncStr = lastSync.Format("2006-01-02 15:04:05")
	}

	page := h.pageData(c, "Sync Terminal")
	syncData := toSyncDataSlice(syncs)

	// For HTMX requests, return only the table partial.
	if shared.IsHTMX(c) {
		return shared.Render(c, http.StatusOK, syncTmpl.SyncTablePartial(syncData, paginationData))
	}

	return shared.Render(c, http.StatusOK, syncTmpl.IndexPage(page, syncData, paginationData, lastSyncStr))
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

// Download generates and serves an XLSX report for the given sync record.
// TODO: Implement actual XLSX generation using excelize.
func (h *Handler) Download(c echo.Context) error {
	id, err := strconv.Atoi(c.QueryParam("id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid sync ID")
	}

	_, err = h.service.GetSyncByID(id)
	if err != nil {
		return echo.NewHTTPError(http.StatusNotFound, "sync record not found")
	}

	// TODO: Generate XLSX file using excelize and serve it.
	// For now, return a placeholder response.
	return echo.NewHTTPError(http.StatusNotImplemented, "XLSX download not yet implemented")
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
