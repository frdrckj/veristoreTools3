package admin

import (
	"fmt"
	"net/http"
	"strconv"

	"github.com/gorilla/sessions"
	"github.com/labstack/echo/v4"
	mw "github.com/verifone/veristoretools3/internal/middleware"
	"github.com/verifone/veristoretools3/internal/shared"
	"github.com/verifone/veristoretools3/templates/layouts"
	tplTmpl "github.com/verifone/veristoretools3/templates/templateparameter"
)

// TemplateParamHandler holds dependencies for template parameter HTTP handlers.
type TemplateParamHandler struct {
	repo        *Repository
	store       sessions.Store
	sessionName string
	appName     string
	appVersion  string
}

// NewTemplateParamHandler creates a new template parameter handler.
func NewTemplateParamHandler(repo *Repository, store sessions.Store, sessionName, appName, appVersion string) *TemplateParamHandler {
	return &TemplateParamHandler{
		repo:        repo,
		store:       store,
		sessionName: sessionName,
		appName:     appName,
		appVersion:  appVersion,
	}
}

// pageData builds a layouts.PageData from the echo context and handler config.
func (h *TemplateParamHandler) pageData(c echo.Context, title string) layouts.PageData {
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

// toTplParamData converts a TemplateParameter model to a view struct.
func toTplParamData(tp TemplateParameter) tplTmpl.TplParamData {
	except := ""
	if tp.TparamExcept != nil {
		except = *tp.TparamExcept
	}
	return tplTmpl.TplParamData{
		TparamID:         tp.TparamID,
		TparamTitle:      tp.TparamTitle,
		TparamIndexTitle: tp.TparamIndexTitle,
		TparamField:      tp.TparamField,
		TparamIndex:      tp.TparamIndex,
		TparamType:       tp.TparamType,
		TparamOperation:  tp.TparamOperation,
		TparamLength:     tp.TparamLength,
		TparamExcept:     except,
	}
}

// toTplParamDataSlice converts a slice of TemplateParameter models to view structs.
func toTplParamDataSlice(params []TemplateParameter) []tplTmpl.TplParamData {
	result := make([]tplTmpl.TplParamData, len(params))
	for i, tp := range params {
		result[i] = toTplParamData(tp)
	}
	return result
}

// Index lists all template parameters.
func (h *TemplateParamHandler) Index(c echo.Context) error {
	params, err := h.repo.AllTemplateParameters()
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to load template parameters")
	}

	page := h.pageData(c, "Template Parameters")
	tplData := toTplParamDataSlice(params)

	return shared.Render(c, http.StatusOK, tplTmpl.IndexPage(page, tplData))
}

// View displays template parameter details by ID.
func (h *TemplateParamHandler) View(c echo.Context) error {
	id, err := strconv.Atoi(c.QueryParam("id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid template parameter ID")
	}

	tp, err := h.repo.FindTemplateParameterByID(id)
	if err != nil {
		return echo.NewHTTPError(http.StatusNotFound, "template parameter not found")
	}

	page := h.pageData(c, "Template Parameter Detail")
	return shared.Render(c, http.StatusOK, tplTmpl.ViewPage(page, toTplParamData(*tp)))
}

// Create handles GET (show form) and POST (process form) for creating a template parameter.
func (h *TemplateParamHandler) Create(c echo.Context) error {
	page := h.pageData(c, "Create Template Parameter")

	if c.Request().Method == http.MethodGet {
		return shared.Render(c, http.StatusOK, tplTmpl.FormPage(page, tplTmpl.TplParamData{}, false, nil))
	}

	tp, errors := h.parseForm(c)
	if len(errors) > 0 {
		return shared.Render(c, http.StatusOK, tplTmpl.FormPage(page, toTplParamData(*tp), false, errors))
	}

	if err := h.repo.CreateTemplateParameter(tp); err != nil {
		return shared.Render(c, http.StatusOK, tplTmpl.FormPage(page, toTplParamData(*tp), false, []string{err.Error()}))
	}

	shared.SetFlash(c, h.store, h.sessionName, shared.FlashSuccess, "Template parameter created successfully")
	return c.Redirect(http.StatusFound, "/templateparameter/index")
}

// Update handles GET (show form) and POST (process form) for editing a template parameter.
func (h *TemplateParamHandler) Update(c echo.Context) error {
	id, err := strconv.Atoi(c.QueryParam("id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid template parameter ID")
	}

	existing, err := h.repo.FindTemplateParameterByID(id)
	if err != nil {
		return echo.NewHTTPError(http.StatusNotFound, "template parameter not found")
	}

	page := h.pageData(c, "Edit Template Parameter")

	if c.Request().Method == http.MethodGet {
		return shared.Render(c, http.StatusOK, tplTmpl.FormPage(page, toTplParamData(*existing), true, nil))
	}

	updated, errors := h.parseForm(c)
	if len(errors) > 0 {
		updated.TparamID = existing.TparamID
		return shared.Render(c, http.StatusOK, tplTmpl.FormPage(page, toTplParamData(*updated), true, errors))
	}

	existing.TparamTitle = updated.TparamTitle
	existing.TparamIndexTitle = updated.TparamIndexTitle
	existing.TparamField = updated.TparamField
	existing.TparamIndex = updated.TparamIndex
	existing.TparamType = updated.TparamType
	existing.TparamOperation = updated.TparamOperation
	existing.TparamLength = updated.TparamLength
	existing.TparamExcept = updated.TparamExcept

	if err := h.repo.UpdateTemplateParameter(existing); err != nil {
		return shared.Render(c, http.StatusOK, tplTmpl.FormPage(page, toTplParamData(*existing), true, []string{err.Error()}))
	}

	shared.SetFlash(c, h.store, h.sessionName, shared.FlashSuccess, "Template parameter updated successfully")
	return c.Redirect(http.StatusFound, "/templateparameter/index")
}

// Delete removes a template parameter by ID.
func (h *TemplateParamHandler) Delete(c echo.Context) error {
	id, err := strconv.Atoi(c.QueryParam("id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid template parameter ID")
	}

	if err := h.repo.DeleteTemplateParameter(id); err != nil {
		shared.SetFlash(c, h.store, h.sessionName, shared.FlashError, fmt.Sprintf("Failed to delete template parameter: %v", err))
	} else {
		shared.SetFlash(c, h.store, h.sessionName, shared.FlashSuccess, "Template parameter deleted successfully")
	}

	return c.Redirect(http.StatusFound, "/templateparameter/index")
}

// parseForm extracts form values and validates required fields.
func (h *TemplateParamHandler) parseForm(c echo.Context) (*TemplateParameter, []string) {
	index, _ := strconv.Atoi(c.FormValue("tparam_index"))
	except := c.FormValue("tparam_except")

	tp := &TemplateParameter{
		TparamTitle:      c.FormValue("tparam_title"),
		TparamIndexTitle: c.FormValue("tparam_index_title"),
		TparamField:      c.FormValue("tparam_field"),
		TparamIndex:      index,
		TparamType:       c.FormValue("tparam_type"),
		TparamOperation:  c.FormValue("tparam_operation"),
		TparamLength:     c.FormValue("tparam_length"),
	}
	if except != "" {
		tp.TparamExcept = &except
	}

	var errors []string
	if tp.TparamTitle == "" {
		errors = append(errors, "Title is required")
	}
	if tp.TparamField == "" {
		errors = append(errors, "Field is required")
	}
	if tp.TparamType == "" {
		errors = append(errors, "Type is required")
	}

	return tp, errors
}
