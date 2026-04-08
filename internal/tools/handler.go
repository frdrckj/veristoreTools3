package tools

import (
	"net/http"
	"os"
	"path/filepath"

	"github.com/gorilla/sessions"
	"github.com/labstack/echo/v4"
	mw "github.com/verifone/veristoretools3/internal/middleware"
	"github.com/verifone/veristoretools3/internal/shared"
	"github.com/verifone/veristoretools3/templates/layouts"
	toolsTmpl "github.com/verifone/veristoretools3/templates/tools"
	"gorm.io/gorm"
)

// Handler handles HTTP requests for the tools page.
type Handler struct {
	v3DB        *gorm.DB
	v2DB        *gorm.DB
	store       sessions.Store
	sessionName string
	appName     string
	appVersion  string
}

// NewHandler creates a new tools handler.
func NewHandler(v3DB, v2DB *gorm.DB, store sessions.Store, sessionName, appName, appVersion string) *Handler {
	return &Handler{
		v3DB:        v3DB,
		v2DB:        v2DB,
		store:       store,
		sessionName: sessionName,
		appName:     appName,
		appVersion:  appVersion,
	}
}

func (h *Handler) pageData(c echo.Context, title string) layouts.PageData {
	flashes := shared.GetFlashes(c, h.store, h.sessionName)
	return layouts.PageData{
		Title:          title,
		AppName:        h.appName,
		AppVersion:     h.appVersion,
		AppIcon:        "favicon.png",
		AppLogo:        "verifone_logo.png",
		UserName:       mw.GetCurrentUserName(c),
		UserFullname:   mw.GetCurrentUserFullname(c),
		UserPrivileges: mw.GetCurrentUserPrivileges(c),
		CopyrightTitle: "Verifone",
		CopyrightURL:   "https://www.verifone.com",
		Flashes:        flashes,
	}
}

// Index renders the tools page.
func (h *Handler) Index(c echo.Context) error {
	page := h.pageData(c, "Tools")
	return shared.Render(c, http.StatusOK, toolsTmpl.ToolsPage(page, nil))
}

// SyncDatabase performs bidirectional incremental sync between v2 and v3.
func (h *Handler) SyncDatabase(c echo.Context) error {
	page := h.pageData(c, "Tools")

	if h.v2DB == nil {
		page.Flashes = map[string][]string{shared.FlashError: {"V2 database is not configured or connection failed. Check v2_database in config.yaml."}}
		return shared.Render(c, http.StatusOK, toolsTmpl.ToolsPage(page, nil))
	}

	results, err := SyncDatabases(h.v2DB, h.v3DB)
	if err != nil {
		page.Flashes = map[string][]string{shared.FlashError: {err.Error()}}
		return shared.Render(c, http.StatusOK, toolsTmpl.ToolsPage(page, nil))
	}

	// Convert to template view types.
	var views []toolsTmpl.SyncResultView
	for _, r := range results {
		views = append(views, toolsTmpl.SyncResultView{
			Table:   r.Table,
			V2ToV3:  r.V2ToV3,
			V3ToV2:  r.V3ToV2,
			Errors:  r.Errors,
			V2Count: r.V2Count,
			V3Count: r.V3Count,
		})
	}

	page.Flashes = map[string][]string{shared.FlashSuccess: {"Database sync completed successfully!"}}
	return shared.Render(c, http.StatusOK, toolsTmpl.ToolsPage(page, views))
}

// DownloadLog serves the TMS business log file as a download.
func (h *Handler) DownloadLog(c echo.Context) error {
	logPath := "/host-logs/store-use-business.log"

	if _, err := os.Stat(logPath); os.IsNotExist(err) {
		shared.SetFlash(c, h.store, h.sessionName, shared.FlashError, "Log file not found. Make sure the log path is mounted in Docker.")
		return c.Redirect(http.StatusFound, "/tools/index")
	}

	return c.Attachment(logPath, filepath.Base(logPath))
}
