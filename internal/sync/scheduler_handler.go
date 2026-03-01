package sync

import (
	"fmt"
	"net/http"
	"strings"
	"time"

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

// Index displays the scheduler configuration page (Penjadwalan Sinkronisasi CSI).
// Reads the pipe-delimited tms_login_scheduled field and populates the form.
func (h *SchedulerHandler) Index(c echo.Context) error {
	data := schedTmpl.SchedulerData{}

	login, err := h.tmsRepo.GetActiveLogin()
	if err == nil && login != nil {
		data.LoginID = login.TmsLoginID

		// Parse the pipe-delimited scheduled string: SETTING|DATE_FROM|DATE_TO|TIME_FROM|TIME_TO
		if login.TmsLoginScheduled != nil && *login.TmsLoginScheduled != "" {
			parts := strings.Split(*login.TmsLoginScheduled, "|")
			data.Enabled = "1"
			if len(parts) > 0 {
				data.Setting = parts[0]
			}
			if len(parts) > 1 {
				data.DateFrom = parts[1]
			}
			if len(parts) > 2 {
				data.DateTo = parts[2]
			}
			if len(parts) > 3 {
				data.TimeFrom = parts[3]
			}
			if len(parts) > 4 {
				data.TimeTo = parts[4]
			}
		} else {
			data.Enabled = "0"
		}
	}

	page := h.pageData(c, "Penjadwalan Sinkronisasi CSI")
	return shared.Render(c, http.StatusOK, schedTmpl.IndexPage(page, data))
}

// Update handles POST for updating scheduler settings.
// Stores as pipe-delimited: SETTING|DATE_FROM|DATE_TO|TIME_FROM|TIME_TO
// Or empty string when disabled (V2 stores NULL).
func (h *SchedulerHandler) Update(c echo.Context) error {
	loginIDStr := c.FormValue("login_id")
	if loginIDStr == "" || loginIDStr == "0" {
		shared.SetFlash(c, h.store, h.sessionName, shared.FlashError, "Koneksi TMS bermasalah!")
		return c.Redirect(http.StatusFound, "/scheduler/index")
	}

	var loginID int
	_, err := fmt.Sscanf(loginIDStr, "%d", &loginID)
	if err != nil || loginID == 0 {
		shared.SetFlash(c, h.store, h.sessionName, shared.FlashError, "Invalid TMS login ID")
		return c.Redirect(http.StatusFound, "/scheduler/index")
	}

	enabled := c.FormValue("enabled")
	setting := c.FormValue("setting")
	dateFrom := c.FormValue("date_from")
	dateTo := c.FormValue("date_to")
	timeFrom := c.FormValue("time_from")
	timeTo := c.FormValue("time_to")

	// If disabled, clear the scheduled field.
	if enabled != "1" {
		if err := h.tmsRepo.UpdateScheduled(loginID, ""); err != nil {
			shared.SetFlash(c, h.store, h.sessionName, shared.FlashError, fmt.Sprintf("Simpan gagal dilakukan! %v", err))
		} else {
			shared.SetFlash(c, h.store, h.sessionName, shared.FlashSuccess, "Simpan berhasil dilakukan!")
		}
		return c.Redirect(http.StatusFound, "/scheduler/index")
	}

	// Validate: setting must be selected.
	if setting == "" {
		shared.SetFlash(c, h.store, h.sessionName, shared.FlashError, "Setting harus dipilih!")
		return c.Redirect(http.StatusFound, "/scheduler/index")
	}

	// Validate: dates must be provided.
	if dateFrom == "" || dateTo == "" {
		shared.SetFlash(c, h.store, h.sessionName, shared.FlashError, "Tanggal harus diisi!")
		return c.Redirect(http.StatusFound, "/scheduler/index")
	}

	// Validate: start period <= end period.
	startStr := dateFrom
	endStr := dateTo
	if timeFrom != "" {
		startStr += " " + timeFrom + ":00:00"
	}
	if timeTo != "" {
		endStr += " " + timeTo + ":00:00"
	}

	startTime, _ := time.Parse("2006-01-02 15:04:05", startStr)
	endTime, _ := time.Parse("2006-01-02 15:04:05", endStr)
	if !startTime.IsZero() && !endTime.IsZero() && startTime.After(endTime) {
		shared.SetFlash(c, h.store, h.sessionName, shared.FlashError, "Periode tidak sesuai!")
		return c.Redirect(http.StatusFound, "/scheduler/index")
	}

	// Store as pipe-delimited: SETTING|DATE_FROM|DATE_TO|TIME_FROM|TIME_TO
	scheduled := setting + "|" + dateFrom + "|" + dateTo + "|" + timeFrom + "|" + timeTo
	if err := h.tmsRepo.UpdateScheduled(loginID, scheduled); err != nil {
		shared.SetFlash(c, h.store, h.sessionName, shared.FlashError, fmt.Sprintf("Simpan gagal dilakukan! %v", err))
	} else {
		mw.LogActivityFromContext(c, mw.LogSchedulerSyncEdit, "")
		shared.SetFlash(c, h.store, h.sessionName, shared.FlashSuccess, "Simpan berhasil dilakukan!")
	}

	return c.Redirect(http.StatusFound, "/scheduler/index")
}
