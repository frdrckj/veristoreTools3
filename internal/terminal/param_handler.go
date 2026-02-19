package terminal

import (
	"fmt"
	"net/http"
	"strconv"

	"github.com/gorilla/sessions"
	"github.com/labstack/echo/v4"
	mw "github.com/verifone/veristoretools3/internal/middleware"
	"github.com/verifone/veristoretools3/internal/shared"
	"github.com/verifone/veristoretools3/templates/components"
	"github.com/verifone/veristoretools3/templates/layouts"
	tpTmpl "github.com/verifone/veristoretools3/templates/terminalparameter"
)

// ParamHandler holds dependencies for terminal parameter HTTP handlers.
type ParamHandler struct {
	repo        *Repository
	store       sessions.Store
	sessionName string
	appName     string
	appVersion  string
}

// NewParamHandler creates a new terminal parameter handler.
func NewParamHandler(repo *Repository, store sessions.Store, sessionName, appName, appVersion string) *ParamHandler {
	return &ParamHandler{
		repo:        repo,
		store:       store,
		sessionName: sessionName,
		appName:     appName,
		appVersion:  appVersion,
	}
}

// pageData builds a layouts.PageData from the echo context and handler config.
func (h *ParamHandler) pageData(c echo.Context, title string) layouts.PageData {
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

// toTPData converts a TerminalParameter model to a tpTmpl.TermParamData view struct.
func toTPData(p TerminalParameter) tpTmpl.TermParamData {
	return tpTmpl.TermParamData{
		ParamID:       p.ParamID,
		ParamTermID:   p.ParamTermID,
		HostName:      p.ParamHostName,
		MerchantName:  p.ParamMerchantName,
		TID:           p.ParamTID,
		MID:           p.ParamMID,
		Address1:      ptrStr(p.ParamAddress1),
		Address2:      ptrStr(p.ParamAddress2),
		Address3:      ptrStr(p.ParamAddress3),
		Address4:      ptrStr(p.ParamAddress4),
		Address5:      ptrStr(p.ParamAddress5),
		Address6:      ptrStr(p.ParamAddress6),
	}
}

// toTPDataSlice converts a slice of TerminalParameter models to view structs.
func toTPDataSlice(params []TerminalParameter) []tpTmpl.TermParamData {
	result := make([]tpTmpl.TermParamData, len(params))
	for i, p := range params {
		result[i] = toTPData(p)
	}
	return result
}

// Index lists terminal parameters with search and pagination.
func (h *ParamHandler) Index(c echo.Context) error {
	query := c.QueryParam("q")
	pageNum, _ := strconv.Atoi(c.QueryParam("page"))
	if pageNum < 1 {
		pageNum = 1
	}

	params, pagination, err := h.repo.SearchParameters(query, pageNum, 20)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to load terminal parameters")
	}

	paginationData := components.PaginationData{
		CurrentPage: pagination.CurrentPage,
		TotalPages:  pagination.TotalPages,
		Total:       pagination.Total,
		BaseURL:     "/terminalparameter/index",
		HTMXTarget:  "tp-table-container",
	}

	page := h.pageData(c, "Terminal Parameters")
	tpData := toTPDataSlice(params)

	if shared.IsHTMX(c) {
		return shared.Render(c, http.StatusOK, tpTmpl.TablePartial(tpData, paginationData, query))
	}

	return shared.Render(c, http.StatusOK, tpTmpl.IndexPage(page, tpData, paginationData, query))
}

// View displays terminal parameter details by ID.
func (h *ParamHandler) View(c echo.Context) error {
	id, err := strconv.Atoi(c.QueryParam("id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid parameter ID")
	}

	p, err := h.repo.FindParameterByID(id)
	if err != nil {
		return echo.NewHTTPError(http.StatusNotFound, "terminal parameter not found")
	}

	page := h.pageData(c, "Terminal Parameter Detail")
	return shared.Render(c, http.StatusOK, tpTmpl.ViewPage(page, toTPData(*p)))
}

// Create handles GET (show form) and POST (process form) for creating a terminal parameter.
func (h *ParamHandler) Create(c echo.Context) error {
	page := h.pageData(c, "Create Terminal Parameter")

	if c.Request().Method == http.MethodGet {
		return shared.Render(c, http.StatusOK, tpTmpl.FormPage(page, tpTmpl.TermParamData{}, false, nil))
	}

	p, errors := h.parseForm(c)
	if len(errors) > 0 {
		return shared.Render(c, http.StatusOK, tpTmpl.FormPage(page, toTPData(*p), false, errors))
	}

	if err := h.repo.CreateParameter(p); err != nil {
		return shared.Render(c, http.StatusOK, tpTmpl.FormPage(page, toTPData(*p), false, []string{err.Error()}))
	}

	shared.SetFlash(c, h.store, h.sessionName, shared.FlashSuccess, "Terminal parameter created successfully")
	return c.Redirect(http.StatusFound, "/terminalparameter/index")
}

// Update handles GET (show form) and POST (process form) for editing a terminal parameter.
func (h *ParamHandler) Update(c echo.Context) error {
	id, err := strconv.Atoi(c.QueryParam("id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid parameter ID")
	}

	existing, err := h.repo.FindParameterByID(id)
	if err != nil {
		return echo.NewHTTPError(http.StatusNotFound, "terminal parameter not found")
	}

	page := h.pageData(c, "Edit Terminal Parameter")

	if c.Request().Method == http.MethodGet {
		return shared.Render(c, http.StatusOK, tpTmpl.FormPage(page, toTPData(*existing), true, nil))
	}

	updated, errors := h.parseForm(c)
	if len(errors) > 0 {
		updated.ParamID = existing.ParamID
		return shared.Render(c, http.StatusOK, tpTmpl.FormPage(page, toTPData(*updated), true, errors))
	}

	existing.ParamTermID = updated.ParamTermID
	existing.ParamHostName = updated.ParamHostName
	existing.ParamMerchantName = updated.ParamMerchantName
	existing.ParamTID = updated.ParamTID
	existing.ParamMID = updated.ParamMID
	existing.ParamAddress1 = updated.ParamAddress1
	existing.ParamAddress2 = updated.ParamAddress2
	existing.ParamAddress3 = updated.ParamAddress3
	existing.ParamAddress4 = updated.ParamAddress4
	existing.ParamAddress5 = updated.ParamAddress5
	existing.ParamAddress6 = updated.ParamAddress6

	if err := h.repo.UpdateParameter(existing); err != nil {
		return shared.Render(c, http.StatusOK, tpTmpl.FormPage(page, toTPData(*existing), true, []string{err.Error()}))
	}

	shared.SetFlash(c, h.store, h.sessionName, shared.FlashSuccess, "Terminal parameter updated successfully")
	return c.Redirect(http.StatusFound, "/terminalparameter/index")
}

// Delete removes a terminal parameter by ID.
func (h *ParamHandler) Delete(c echo.Context) error {
	id, err := strconv.Atoi(c.QueryParam("id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid parameter ID")
	}

	if err := h.repo.DeleteParameter(id); err != nil {
		shared.SetFlash(c, h.store, h.sessionName, shared.FlashError, fmt.Sprintf("Failed to delete terminal parameter: %v", err))
	} else {
		shared.SetFlash(c, h.store, h.sessionName, shared.FlashSuccess, "Terminal parameter deleted successfully")
	}

	return c.Redirect(http.StatusFound, "/terminalparameter/index")
}

// parseForm extracts form values and validates required fields.
func (h *ParamHandler) parseForm(c echo.Context) (*TerminalParameter, []string) {
	termID, _ := strconv.Atoi(c.FormValue("term_id"))

	addr1 := c.FormValue("address1")
	addr2 := c.FormValue("address2")
	addr3 := c.FormValue("address3")
	addr4 := c.FormValue("address4")
	addr5 := c.FormValue("address5")
	addr6 := c.FormValue("address6")

	p := &TerminalParameter{
		ParamTermID:      termID,
		ParamHostName:    c.FormValue("host_name"),
		ParamMerchantName: c.FormValue("merchant_name"),
		ParamTID:         c.FormValue("tid"),
		ParamMID:         c.FormValue("mid"),
		ParamAddress1:    nilIfEmpty(addr1),
		ParamAddress2:    nilIfEmpty(addr2),
		ParamAddress3:    nilIfEmpty(addr3),
		ParamAddress4:    nilIfEmpty(addr4),
		ParamAddress5:    nilIfEmpty(addr5),
		ParamAddress6:    nilIfEmpty(addr6),
	}

	var errors []string
	if p.ParamTermID == 0 {
		errors = append(errors, "Terminal ID is required")
	}
	if p.ParamHostName == "" {
		errors = append(errors, "Host Name is required")
	}
	if p.ParamTID == "" {
		errors = append(errors, "TID is required")
	}
	if p.ParamMID == "" {
		errors = append(errors, "MID is required")
	}

	return p, errors
}

// nilIfEmpty returns a pointer to s if non-empty, otherwise nil.
func nilIfEmpty(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
