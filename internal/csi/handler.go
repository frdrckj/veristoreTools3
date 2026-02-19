package csi

import (
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/gorilla/sessions"
	"github.com/labstack/echo/v4"
	"github.com/verifone/veristoretools3/internal/admin"
	mw "github.com/verifone/veristoretools3/internal/middleware"
	"github.com/verifone/veristoretools3/internal/shared"
	"github.com/verifone/veristoretools3/templates/layouts"
	verTmpl "github.com/verifone/veristoretools3/templates/verification"
)

// Handler holds dependencies for CSI verification HTTP handlers.
type Handler struct {
	service     *Service
	store       sessions.Store
	sessionName string
	appName     string
	appVersion  string
}

// NewHandler creates a new CSI verification handler.
func NewHandler(service *Service, store sessions.Store, sessionName string, appName, appVersion string) *Handler {
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

// Index handles GET/POST /verification/index.
//
// GET: Show the CSI search form.
// POST with "csi": Search terminal, show results with verification form.
// POST with "teknisiId" + "spkNo": Save verification report.
func (h *Handler) Index(c echo.Context) error {
	page := h.pageData(c, "Verifikasi CSI")

	// Load app versions for the dropdown.
	appVersions, _ := h.service.GetAppVersions()

	if c.Request().Method == http.MethodGet {
		// Initial state: show search form only.
		return shared.Render(c, http.StatusOK, verTmpl.VerificationPage(page, nil, nil, false, "", appVersions))
	}

	// POST - determine which action based on form fields.
	csiVal := c.FormValue("csi")
	teknisiIdStr := c.FormValue("teknisiId")
	spkNo := c.FormValue("spkNo")

	// If teknisiId is provided, this is a report submission.
	if teknisiIdStr != "" && spkNo != "" {
		return h.handleReportSubmission(c, page, appVersions)
	}

	// Otherwise, this is a CSI search.
	if csiVal == "" {
		shared.SetFlash(c, h.store, h.sessionName, shared.FlashError, "CSI number is required")
		return shared.Render(c, http.StatusOK, verTmpl.VerificationPage(page, nil, nil, false, "", appVersions))
	}

	return h.handleSearch(c, page, csiVal, appVersions)
}

// handleSearch processes the CSI search POST request.
func (h *Handler) handleSearch(c echo.Context, page layouts.PageData, csi string, appVersions []string) error {
	appVersion := c.FormValue("appVersion")

	var result *SearchResult
	var err error

	if appVersion != "" {
		// Search with specific app version (matching v2 behaviour).
		result, err = h.service.SearchTerminalWithVersion(csi, appVersion)
	} else {
		// Search by CSI only.
		result, err = h.service.SearchTerminal(csi)
	}

	if err != nil {
		shared.SetFlash(c, h.store, h.sessionName, shared.FlashError, fmt.Sprintf("Search failed: %v", err))
		return shared.Render(c, http.StatusOK, verTmpl.VerificationPage(page, nil, nil, false, "", appVersions))
	}

	if !result.Found {
		shared.SetFlash(c, h.store, h.sessionName, shared.FlashInfo, fmt.Sprintf("CSI %s tidak ditemukan!", csi))
		return shared.Render(c, http.StatusOK, verTmpl.VerificationPage(page, nil, nil, false, "", appVersions))
	}

	// Terminal found: calculate activation password.
	password := h.service.CalcVerificationPassword(
		result.CSI, result.TID, result.MID,
		result.DeviceType, result.AppVersion,
	)

	// Load technicians for the dropdown.
	technicians, _ := h.service.GetTechnicians()

	// Convert to template view types.
	tmplResult := toTemplateResult(result)
	tmplTechs := toTemplateTechnicians(technicians)

	return shared.Render(c, http.StatusOK, verTmpl.VerificationPage(page, tmplResult, tmplTechs, false, password, appVersions))
}

// handleReportSubmission processes the verification report save POST request.
func (h *Handler) handleReportSubmission(c echo.Context, page layouts.PageData, appVersions []string) error {
	csi := c.FormValue("csi")
	appVersion := c.FormValue("appVersion")
	teknisiIdStr := c.FormValue("teknisiId")
	spkNo := c.FormValue("spkNo")
	remark := c.FormValue("remark")
	status := c.FormValue("status")
	deviceID := c.FormValue("deviceId")

	teknisiId, _ := strconv.Atoi(teknisiIdStr)
	if teknisiId == 0 {
		shared.SetFlash(c, h.store, h.sessionName, shared.FlashError, "Technician is required")
		return c.Redirect(http.StatusFound, "/verification/index")
	}

	// Re-search the terminal to get full data.
	var result *SearchResult
	var err error
	if appVersion != "" {
		result, err = h.service.SearchTerminalWithVersion(csi, appVersion)
	} else {
		result, err = h.service.SearchTerminal(csi)
	}

	if err != nil || result == nil || !result.Found {
		shared.SetFlash(c, h.store, h.sessionName, shared.FlashError, "Terminal not found during save")
		return c.Redirect(http.StatusFound, "/verification/index")
	}

	// Get technician details.
	technician, err := h.service.GetTechnicianDetail(teknisiId)
	if err != nil || technician == nil {
		shared.SetFlash(c, h.store, h.sessionName, shared.FlashError, "Technician not found")
		return c.Redirect(http.StatusFound, "/verification/index")
	}

	// If status is DONE and deviceID has changed, update the terminal.
	if status == "DONE" && result.Terminal != nil && deviceID != result.Terminal.TermDeviceID {
		if err := h.service.UpdateTerminalDeviceID(result.TerminalID, deviceID); err != nil {
			shared.SetFlash(c, h.store, h.sessionName, shared.FlashError, fmt.Sprintf("Failed to update terminal: %v", err))
			return c.Redirect(http.StatusFound, "/verification/index")
		}
		// Update result for the report.
		result.DeviceID = deviceID
	}

	// Build the parameter string (matching v2 format: host|merchant|tid|mid---...).
	paramStr := ""
	if len(result.Parameters) > 0 {
		for i, p := range result.Parameters {
			paramStr += p.ParamHostName + "|" + p.ParamMerchantName + "|" + p.ParamTID + "|" + p.ParamMID
			if i < len(result.Parameters)-1 {
				paramStr += "---"
			}
		}
	}

	// Map gender code (matching v2: 0 = LAKI-LAKI, else = PEREMPUAN).
	gender := "PEREMPUAN"
	if technician.TechGender == "0" {
		gender = "LAKI-LAKI"
	}

	// Determine TMS create operator and date from terminal if available.
	tmsCreateOperator := ""
	tmsCreateDt := time.Now()
	if result.Terminal != nil {
		tmsCreateOperator = result.Terminal.TermTmsCreateOperator
		tmsCreateDt = result.Terminal.TermTmsCreateDtOperator
	}

	// Create the verification report.
	report := &VerificationReport{
		VfiRptTermDeviceID:            result.DeviceID,
		VfiRptTermSerialNum:           result.CSI,
		VfiRptTermProductNum:          result.ProductNum,
		VfiRptTermModel:               result.DeviceType,
		VfiRptTermAppName:             result.AppName,
		VfiRptTermAppVersion:          result.AppVersion,
		VfiRptTermParameter:           paramStr,
		VfiRptTermTmsCreateOperator:   tmsCreateOperator,
		VfiRptTermTmsCreateDtOperator: tmsCreateDt,
		VfiRptTechName:                technician.TechName,
		VfiRptTechNip:                 technician.TechNip,
		VfiRptTechNumber:              technician.TechNumber,
		VfiRptTechAddress:             technician.TechAddress,
		VfiRptTechCompany:             technician.TechCompany,
		VfiRptTechSercivePoint:        technician.TechSercivePoint,
		VfiRptTechPhone:               technician.TechPhone,
		VfiRptTechGender:              gender,
		VfiRptTicketNo:                "",
		VfiRptSpkNo:                   spkNo,
		VfiRptWorkOrder:               "",
		VfiRptRemark:                  remark,
		VfiRptStatus:                  status,
		CreatedBy:                     mw.GetCurrentUserFullname(c),
		CreatedDt:                     time.Now(),
	}

	if err := h.service.CreateReport(report); err != nil {
		shared.SetFlash(c, h.store, h.sessionName, shared.FlashError, fmt.Sprintf("Verifikasi gagal disimpan: %v", err))
		return c.Redirect(http.StatusFound, "/verification/index")
	}

	shared.SetFlash(c, h.store, h.sessionName, shared.FlashSuccess, "Verifikasi berhasil disimpan!")
	return shared.Render(c, http.StatusOK, verTmpl.VerificationPage(page, nil, nil, true, "", appVersions))
}

// GetTechnician handles GET /verification/gettechnician?id=X.
// Returns technician NIP|Number|Company as plain text (matching v2 behaviour).
func (h *Handler) GetTechnician(c echo.Context) error {
	idStr := c.QueryParam("id")
	id, _ := strconv.Atoi(idStr)
	if id == 0 {
		return c.String(http.StatusBadRequest, "")
	}

	technician, err := h.service.GetTechnicianDetail(id)
	if err != nil || technician == nil {
		return c.String(http.StatusNotFound, "")
	}

	// Return pipe-delimited string matching v2 format.
	return c.String(http.StatusOK, technician.TechNip+"|"+technician.TechNumber+"|"+technician.TechCompany)
}

// ---------------------------------------------------------------------------
// Type conversion helpers (internal -> template view types)
// ---------------------------------------------------------------------------

// toTemplateResult converts a SearchResult to the template's TerminalResult.
func toTemplateResult(r *SearchResult) *verTmpl.TerminalResult {
	if r == nil {
		return nil
	}

	tr := &verTmpl.TerminalResult{
		Found:        r.Found,
		Source:       r.Source,
		CSI:          r.CSI,
		TID:          r.TID,
		MID:          r.MID,
		DeviceType:   r.DeviceType,
		MerchantName: r.MerchantName,
		AppVersion:   r.AppVersion,
		AppName:      r.AppName,
		DeviceID:     r.DeviceID,
		ProductNum:   r.ProductNum,
	}

	// Convert parameters.
	for _, p := range r.Parameters {
		tp := verTmpl.TerminalParam{
			HostName:     p.ParamHostName,
			MerchantName: p.ParamMerchantName,
			TID:          p.ParamTID,
			MID:          p.ParamMID,
		}
		if p.ParamAddress1 != nil {
			tp.Address1 = *p.ParamAddress1
		}
		if p.ParamAddress2 != nil {
			tp.Address2 = *p.ParamAddress2
		}
		if p.ParamAddress3 != nil {
			tp.Address3 = *p.ParamAddress3
		}
		if p.ParamAddress4 != nil {
			tp.Address4 = *p.ParamAddress4
		}
		if p.ParamAddress5 != nil {
			tp.Address5 = *p.ParamAddress5
		}
		if p.ParamAddress6 != nil {
			tp.Address6 = *p.ParamAddress6
		}
		tr.Parameters = append(tr.Parameters, tp)
	}

	return tr
}

// toTemplateTechnicians converts a slice of admin.Technician to the template's
// TechnicianOption slice.
func toTemplateTechnicians(techs []admin.Technician) []verTmpl.TechnicianOption {
	if len(techs) == 0 {
		return nil
	}
	opts := make([]verTmpl.TechnicianOption, len(techs))
	for i, t := range techs {
		opts[i] = verTmpl.TechnicianOption{
			ID:   t.TechID,
			Name: t.TechName,
		}
	}
	return opts
}
