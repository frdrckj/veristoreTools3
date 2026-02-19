package terminal

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
	termTmpl "github.com/verifone/veristoretools3/templates/terminal"
)

// Handler holds dependencies for terminal HTTP handlers.
type Handler struct {
	service     *Service
	store       sessions.Store
	sessionName string
	appName     string
	appVersion  string
}

// NewHandler creates a new terminal handler.
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

// toTerminalData converts a Terminal model to a TerminalData view struct.
func toTerminalData(t Terminal) termTmpl.TerminalData {
	tmsUpdateOperator := ""
	if t.TermTmsUpdateOperator != nil {
		tmsUpdateOperator = *t.TermTmsUpdateOperator
	}
	tmsUpdateDtOperator := ""
	if t.TermTmsUpdateDtOperator != nil {
		tmsUpdateDtOperator = t.TermTmsUpdateDtOperator.Format("2006-01-02 15:04:05")
	}
	updatedBy := ""
	if t.UpdatedBy != nil {
		updatedBy = *t.UpdatedBy
	}
	updatedDt := ""
	if t.UpdatedDt != nil {
		updatedDt = t.UpdatedDt.Format("2006-01-02 15:04:05")
	}

	return termTmpl.TerminalData{
		TermID:                  t.TermID,
		TermDeviceID:            t.TermDeviceID,
		TermSerialNum:           t.TermSerialNum,
		TermProductNum:          t.TermProductNum,
		TermModel:               t.TermModel,
		TermAppName:             t.TermAppName,
		TermAppVersion:          t.TermAppVersion,
		TermTmsCreateOperator:   t.TermTmsCreateOperator,
		TermTmsCreateDtOperator: t.TermTmsCreateDtOperator.Format("2006-01-02 15:04:05"),
		TermTmsUpdateOperator:   tmsUpdateOperator,
		TermTmsUpdateDtOperator: tmsUpdateDtOperator,
		CreatedBy:               t.CreatedBy,
		CreatedDt:               t.CreatedDt.Format("2006-01-02 15:04:05"),
		UpdatedBy:               updatedBy,
		UpdatedDt:               updatedDt,
	}
}

// toTerminalDataSlice converts a slice of Terminal models to TerminalData view structs.
func toTerminalDataSlice(terminals []Terminal) []termTmpl.TerminalData {
	result := make([]termTmpl.TerminalData, len(terminals))
	for i, t := range terminals {
		result[i] = toTerminalData(t)
	}
	return result
}

// toParamData converts a TerminalParameter model to a ParamData view struct.
func toParamData(p TerminalParameter) termTmpl.ParamData {
	return termTmpl.ParamData{
		ParamID:           p.ParamID,
		ParamTermID:       p.ParamTermID,
		ParamHostName:     p.ParamHostName,
		ParamMerchantName: p.ParamMerchantName,
		ParamTID:          p.ParamTID,
		ParamMID:          p.ParamMID,
		ParamAddress1:     ptrStr(p.ParamAddress1),
		ParamAddress2:     ptrStr(p.ParamAddress2),
		ParamAddress3:     ptrStr(p.ParamAddress3),
		ParamAddress4:     ptrStr(p.ParamAddress4),
		ParamAddress5:     ptrStr(p.ParamAddress5),
		ParamAddress6:     ptrStr(p.ParamAddress6),
	}
}

// toParamDataSlice converts a slice of TerminalParameter models to ParamData view structs.
func toParamDataSlice(params []TerminalParameter) []termTmpl.ParamData {
	result := make([]termTmpl.ParamData, len(params))
	for i, p := range params {
		result[i] = toParamData(p)
	}
	return result
}

// ptrStr dereferences a *string, returning "" if nil.
func ptrStr(s *string) string {
	if s != nil {
		return *s
	}
	return ""
}

// Index lists terminals with search and pagination. Supports HTMX partial updates.
func (h *Handler) Index(c echo.Context) error {
	query := c.QueryParam("q")
	pageNum, _ := strconv.Atoi(c.QueryParam("page"))
	if pageNum < 1 {
		pageNum = 1
	}

	terminals, pagination, err := h.service.repo.Search(query, pageNum, 20)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to load terminals")
	}

	paginationData := components.PaginationData{
		CurrentPage: pagination.CurrentPage,
		TotalPages:  pagination.TotalPages,
		Total:       pagination.Total,
		BaseURL:     "/terminal/index",
		HTMXTarget:  "terminal-table-container",
	}

	page := h.pageData(c, "Terminals")
	termData := toTerminalDataSlice(terminals)

	// For HTMX requests, return only the table partial.
	if shared.IsHTMX(c) {
		return shared.Render(c, http.StatusOK, termTmpl.TerminalTablePartial(termData, paginationData, query))
	}

	return shared.Render(c, http.StatusOK, termTmpl.IndexPage(page, termData, paginationData, query))
}

// View displays a terminal detail page with its parameters.
func (h *Handler) View(c echo.Context) error {
	id, err := strconv.Atoi(c.QueryParam("id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid terminal ID")
	}

	t, err := h.service.GetTerminalByID(id)
	if err != nil {
		return echo.NewHTTPError(http.StatusNotFound, "terminal not found")
	}

	params, err := h.service.GetTerminalParameters(id)
	if err != nil {
		params = []TerminalParameter{} // Non-fatal: show terminal without params.
	}

	page := h.pageData(c, "Terminal Detail")
	return shared.Render(c, http.StatusOK, termTmpl.ViewPage(page, toTerminalData(*t), toParamDataSlice(params)))
}

// Create handles both GET (show form) and POST (process form) for creating a terminal.
func (h *Handler) Create(c echo.Context) error {
	page := h.pageData(c, "Create Terminal")

	if c.Request().Method == http.MethodGet {
		return shared.Render(c, http.StatusOK, termTmpl.FormPage(page, termTmpl.TerminalData{}, false, nil))
	}

	// Process POST.
	t, errors := h.parseTerminalForm(c)
	if len(errors) > 0 {
		return shared.Render(c, http.StatusOK, termTmpl.FormPage(page, toTerminalData(*t), false, errors))
	}

	t.CreatedBy = mw.GetCurrentUserName(c)
	t.CreatedDt = time.Now()

	if err := h.service.CreateTerminal(t); err != nil {
		return shared.Render(c, http.StatusOK, termTmpl.FormPage(page, toTerminalData(*t), false, []string{err.Error()}))
	}

	shared.SetFlash(c, h.store, h.sessionName, shared.FlashSuccess, "Terminal created successfully")
	return c.Redirect(http.StatusFound, "/terminal/index")
}

// Update handles both GET (show form) and POST (process form) for editing a terminal.
func (h *Handler) Update(c echo.Context) error {
	id, err := strconv.Atoi(c.QueryParam("id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid terminal ID")
	}

	t, err := h.service.GetTerminalByID(id)
	if err != nil {
		return echo.NewHTTPError(http.StatusNotFound, "terminal not found")
	}

	page := h.pageData(c, "Edit Terminal")

	if c.Request().Method == http.MethodGet {
		return shared.Render(c, http.StatusOK, termTmpl.FormPage(page, toTerminalData(*t), true, nil))
	}

	// Process POST.
	updated, errors := h.parseTerminalForm(c)
	if len(errors) > 0 {
		updated.TermID = t.TermID
		return shared.Render(c, http.StatusOK, termTmpl.FormPage(page, toTerminalData(*updated), true, errors))
	}

	// Preserve original fields and update the rest.
	t.TermDeviceID = updated.TermDeviceID
	t.TermSerialNum = updated.TermSerialNum
	t.TermProductNum = updated.TermProductNum
	t.TermModel = updated.TermModel
	t.TermAppName = updated.TermAppName
	t.TermAppVersion = updated.TermAppVersion
	t.TermTmsCreateOperator = updated.TermTmsCreateOperator

	now := time.Now()
	userName := mw.GetCurrentUserName(c)
	t.UpdatedBy = &userName
	t.UpdatedDt = &now

	if err := h.service.UpdateTerminal(t); err != nil {
		return shared.Render(c, http.StatusOK, termTmpl.FormPage(page, toTerminalData(*t), true, []string{err.Error()}))
	}

	shared.SetFlash(c, h.store, h.sessionName, shared.FlashSuccess, "Terminal updated successfully")
	return c.Redirect(http.StatusFound, "/terminal/index")
}

// Delete removes a terminal by ID and redirects to the list.
func (h *Handler) Delete(c echo.Context) error {
	id, err := strconv.Atoi(c.QueryParam("id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid terminal ID")
	}

	if err := h.service.DeleteTerminal(id); err != nil {
		shared.SetFlash(c, h.store, h.sessionName, shared.FlashError, fmt.Sprintf("Failed to delete terminal: %v", err))
	} else {
		shared.SetFlash(c, h.store, h.sessionName, shared.FlashSuccess, "Terminal deleted successfully")
	}

	return c.Redirect(http.StatusFound, "/terminal/index")
}

// parseTerminalForm extracts form values and validates required fields.
func (h *Handler) parseTerminalForm(c echo.Context) (*Terminal, []string) {
	t := &Terminal{
		TermDeviceID:          c.FormValue("device_id"),
		TermSerialNum:         c.FormValue("serial_num"),
		TermProductNum:        c.FormValue("product_num"),
		TermModel:             c.FormValue("model"),
		TermAppName:           c.FormValue("app_name"),
		TermAppVersion:        c.FormValue("app_version"),
		TermTmsCreateOperator: c.FormValue("tms_create_operator"),
	}

	// Parse TMS create date/time.
	if dtStr := c.FormValue("tms_create_dt_operator"); dtStr != "" {
		if dt, err := time.Parse("2006-01-02T15:04", dtStr); err == nil {
			t.TermTmsCreateDtOperator = dt
		} else if dt, err := time.Parse("2006-01-02 15:04:05", dtStr); err == nil {
			t.TermTmsCreateDtOperator = dt
		}
	}

	var errors []string
	if t.TermDeviceID == "" {
		errors = append(errors, "Device ID is required")
	}
	if t.TermSerialNum == "" {
		errors = append(errors, "Serial Number is required")
	}

	return t, errors
}
