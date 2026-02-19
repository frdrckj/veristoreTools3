package sync

import (
	"fmt"
	"net/http"

	"github.com/gorilla/sessions"
	"github.com/labstack/echo/v4"
	mw "github.com/verifone/veristoretools3/internal/middleware"
	"github.com/verifone/veristoretools3/internal/shared"
	"github.com/verifone/veristoretools3/internal/tms"
	"github.com/verifone/veristoretools3/templates/layouts"
	schedTmpl "github.com/verifone/veristoretools3/templates/scheduler"
)

// SchedulerHandler holds dependencies for scheduler configuration HTTP handlers.
type SchedulerHandler struct {
	tmsRepo     *tms.Repository
	store       sessions.Store
	sessionName string
	appName     string
	appVersion  string
}

// NewSchedulerHandler creates a new scheduler handler.
func NewSchedulerHandler(tmsRepo *tms.Repository, store sessions.Store, sessionName, appName, appVersion string) *SchedulerHandler {
	return &SchedulerHandler{
		tmsRepo:     tmsRepo,
		store:       store,
		sessionName: sessionName,
		appName:     appName,
		appVersion:  appVersion,
	}
}

// pageData builds a layouts.PageData from the echo context and handler config.
func (h *SchedulerHandler) pageData(c echo.Context, title string) layouts.PageData {
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

// Index displays the scheduler configuration page.
func (h *SchedulerHandler) Index(c echo.Context) error {
	login, err := h.tmsRepo.GetActiveLogin()
	scheduled := ""
	loginID := 0
	if err == nil && login != nil {
		loginID = login.TmsLoginID
		if login.TmsLoginScheduled != nil {
			scheduled = *login.TmsLoginScheduled
		}
	}

	page := h.pageData(c, "Scheduler Configuration")
	return shared.Render(c, http.StatusOK, schedTmpl.IndexPage(page, scheduled, loginID))
}

// Update handles POST for updating scheduler settings.
func (h *SchedulerHandler) Update(c echo.Context) error {
	scheduled := c.FormValue("scheduled")
	loginIDStr := c.FormValue("login_id")

	if loginIDStr == "" {
		shared.SetFlash(c, h.store, h.sessionName, shared.FlashError, "No active TMS login found")
		return c.Redirect(http.StatusFound, "/scheduler/index")
	}

	var loginID int
	_, err := fmt.Sscanf(loginIDStr, "%d", &loginID)
	if err != nil || loginID == 0 {
		shared.SetFlash(c, h.store, h.sessionName, shared.FlashError, "Invalid TMS login ID")
		return c.Redirect(http.StatusFound, "/scheduler/index")
	}

	if err := h.tmsRepo.UpdateScheduled(loginID, scheduled); err != nil {
		shared.SetFlash(c, h.store, h.sessionName, shared.FlashError, fmt.Sprintf("Failed to update scheduler: %v", err))
	} else {
		shared.SetFlash(c, h.store, h.sessionName, shared.FlashSuccess, "Scheduler settings updated successfully")
	}

	return c.Redirect(http.StatusFound, "/scheduler/index")
}
