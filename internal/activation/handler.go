package activation

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
	actTmpl "github.com/verifone/veristoretools3/templates/appactivation"
	credTmpl "github.com/verifone/veristoretools3/templates/appcredential"
)

// Handler holds dependencies for activation and credential HTTP handlers.
type Handler struct {
	repo        *Repository
	store       sessions.Store
	sessionName string
	appName     string
	appVersion  string
}

// NewHandler creates a new activation handler.
func NewHandler(repo *Repository, store sessions.Store, sessionName, appName, appVersion string) *Handler {
	return &Handler{
		repo:        repo,
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

// ---------------------------------------------------------------------------
// AppActivation
// ---------------------------------------------------------------------------

// toActivationData converts an AppActivation model to a view struct.
func toActivationData(a AppActivation) actTmpl.ActivationData {
	return actTmpl.ActivationData{
		AppActID:       a.AppActID,
		AppActCSI:      a.AppActCSI,
		AppActTID:      a.AppActTID,
		AppActMID:      a.AppActMID,
		AppActModel:    a.AppActModel,
		AppActVersion:  a.AppActVersion,
		AppActEngineer: a.AppActEngineer,
		CreatedBy:      a.CreatedBy,
		CreatedDt:      a.CreatedDt.Format("2006-01-02 15:04:05"),
	}
}

// toActivationDataSlice converts a slice of AppActivation models to view structs.
func toActivationDataSlice(acts []AppActivation) []actTmpl.ActivationData {
	result := make([]actTmpl.ActivationData, len(acts))
	for i, a := range acts {
		result[i] = toActivationData(a)
	}
	return result
}

// ActivationIndex lists app activations with search and pagination.
func (h *Handler) ActivationIndex(c echo.Context) error {
	query := c.QueryParam("q")
	pageNum, _ := strconv.Atoi(c.QueryParam("page"))
	if pageNum < 1 {
		pageNum = 1
	}

	acts, pagination, err := h.repo.SearchActivations(query, pageNum, 20)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to load activations")
	}

	paginationData := components.PaginationData{
		CurrentPage: pagination.CurrentPage,
		TotalPages:  pagination.TotalPages,
		Total:       pagination.Total,
		BaseURL:     "/appactivation/index",
		HTMXTarget:  "act-table-container",
	}

	page := h.pageData(c, "App Activations")
	actData := toActivationDataSlice(acts)

	if shared.IsHTMX(c) {
		return shared.Render(c, http.StatusOK, actTmpl.TablePartial(actData, paginationData, query))
	}

	return shared.Render(c, http.StatusOK, actTmpl.IndexPage(page, actData, paginationData, query))
}

// ActivationView displays activation details by ID.
func (h *Handler) ActivationView(c echo.Context) error {
	id, err := strconv.Atoi(c.QueryParam("id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid activation ID")
	}

	a, err := h.repo.FindActivationByID(id)
	if err != nil {
		return echo.NewHTTPError(http.StatusNotFound, "activation not found")
	}

	page := h.pageData(c, "Activation Detail")
	return shared.Render(c, http.StatusOK, actTmpl.ViewPage(page, toActivationData(*a)))
}

// ActivationCreate handles GET (show form) and POST (process form) for creating an activation.
func (h *Handler) ActivationCreate(c echo.Context) error {
	page := h.pageData(c, "Create Activation")

	if c.Request().Method == http.MethodGet {
		return shared.Render(c, http.StatusOK, actTmpl.FormPage(page, actTmpl.ActivationData{}, false, nil))
	}

	a := &AppActivation{
		AppActCSI:      c.FormValue("csi"),
		AppActTID:      c.FormValue("tid"),
		AppActMID:      c.FormValue("mid"),
		AppActModel:    c.FormValue("model"),
		AppActVersion:  c.FormValue("version"),
		AppActEngineer: c.FormValue("engineer"),
		CreatedBy:      mw.GetCurrentUserName(c),
		CreatedDt:      time.Now(),
	}

	var errors []string
	if a.AppActCSI == "" {
		errors = append(errors, "CSI is required")
	}
	if a.AppActTID == "" {
		errors = append(errors, "TID is required")
	}

	if len(errors) > 0 {
		return shared.Render(c, http.StatusOK, actTmpl.FormPage(page, toActivationData(*a), false, errors))
	}

	if err := h.repo.CreateActivation(a); err != nil {
		return shared.Render(c, http.StatusOK, actTmpl.FormPage(page, toActivationData(*a), false, []string{err.Error()}))
	}

	shared.SetFlash(c, h.store, h.sessionName, shared.FlashSuccess, "Activation created successfully")
	return c.Redirect(http.StatusFound, "/appactivation/index")
}

// ActivationUpdate handles GET (show form) and POST (process form) for editing an activation.
func (h *Handler) ActivationUpdate(c echo.Context) error {
	id, err := strconv.Atoi(c.QueryParam("id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid activation ID")
	}

	existing, err := h.repo.FindActivationByID(id)
	if err != nil {
		return echo.NewHTTPError(http.StatusNotFound, "activation not found")
	}

	page := h.pageData(c, "Edit Activation")

	if c.Request().Method == http.MethodGet {
		return shared.Render(c, http.StatusOK, actTmpl.FormPage(page, toActivationData(*existing), true, nil))
	}

	existing.AppActCSI = c.FormValue("csi")
	existing.AppActTID = c.FormValue("tid")
	existing.AppActMID = c.FormValue("mid")
	existing.AppActModel = c.FormValue("model")
	existing.AppActVersion = c.FormValue("version")
	existing.AppActEngineer = c.FormValue("engineer")

	var errors []string
	if existing.AppActCSI == "" {
		errors = append(errors, "CSI is required")
	}
	if existing.AppActTID == "" {
		errors = append(errors, "TID is required")
	}

	if len(errors) > 0 {
		return shared.Render(c, http.StatusOK, actTmpl.FormPage(page, toActivationData(*existing), true, errors))
	}

	if err := h.repo.UpdateActivation(existing); err != nil {
		return shared.Render(c, http.StatusOK, actTmpl.FormPage(page, toActivationData(*existing), true, []string{err.Error()}))
	}

	shared.SetFlash(c, h.store, h.sessionName, shared.FlashSuccess, "Activation updated successfully")
	return c.Redirect(http.StatusFound, "/appactivation/index")
}

// ActivationDelete removes an activation by ID.
func (h *Handler) ActivationDelete(c echo.Context) error {
	id, err := strconv.Atoi(c.QueryParam("id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid activation ID")
	}

	if err := h.repo.DeleteActivation(id); err != nil {
		shared.SetFlash(c, h.store, h.sessionName, shared.FlashError, fmt.Sprintf("Failed to delete activation: %v", err))
	} else {
		shared.SetFlash(c, h.store, h.sessionName, shared.FlashSuccess, "Activation deleted successfully")
	}

	return c.Redirect(http.StatusFound, "/appactivation/index")
}

// ---------------------------------------------------------------------------
// AppCredential
// ---------------------------------------------------------------------------

// toCredentialData converts an AppCredential model to a view struct.
func toCredentialData(cr AppCredential) credTmpl.CredentialData {
	enable := ""
	if cr.AppCredEnable != nil {
		enable = *cr.AppCredEnable
	}
	return credTmpl.CredentialData{
		AppCredID:     cr.AppCredID,
		AppCredUser:   cr.AppCredUser,
		AppCredName:   cr.AppCredName,
		AppCredEnable: enable,
		CreatedBy:     cr.CreatedBy,
		CreatedDt:     cr.CreatedDt.Format("2006-01-02 15:04:05"),
	}
}

// toCredentialDataSlice converts a slice of AppCredential models to view structs.
func toCredentialDataSlice(creds []AppCredential) []credTmpl.CredentialData {
	result := make([]credTmpl.CredentialData, len(creds))
	for i, cr := range creds {
		result[i] = toCredentialData(cr)
	}
	return result
}

// CredentialIndex lists app credentials with search and pagination.
func (h *Handler) CredentialIndex(c echo.Context) error {
	query := c.QueryParam("q")
	pageNum, _ := strconv.Atoi(c.QueryParam("page"))
	if pageNum < 1 {
		pageNum = 1
	}

	creds, pagination, err := h.repo.SearchCredentials(query, pageNum, 20)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to load credentials")
	}

	paginationData := components.PaginationData{
		CurrentPage: pagination.CurrentPage,
		TotalPages:  pagination.TotalPages,
		Total:       pagination.Total,
		BaseURL:     "/appcredential/index",
		HTMXTarget:  "cred-table-container",
	}

	page := h.pageData(c, "App Credentials")
	credData := toCredentialDataSlice(creds)

	if shared.IsHTMX(c) {
		return shared.Render(c, http.StatusOK, credTmpl.TablePartial(credData, paginationData, query))
	}

	return shared.Render(c, http.StatusOK, credTmpl.IndexPage(page, credData, paginationData, query))
}

// CredentialView displays credential details by ID.
func (h *Handler) CredentialView(c echo.Context) error {
	id, err := strconv.Atoi(c.QueryParam("id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid credential ID")
	}

	cr, err := h.repo.FindCredentialByID(id)
	if err != nil {
		return echo.NewHTTPError(http.StatusNotFound, "credential not found")
	}

	page := h.pageData(c, "Credential Detail")
	return shared.Render(c, http.StatusOK, credTmpl.ViewPage(page, toCredentialData(*cr)))
}

// CredentialCreate handles GET (show form) and POST (process form) for creating a credential.
func (h *Handler) CredentialCreate(c echo.Context) error {
	page := h.pageData(c, "Create Credential")

	if c.Request().Method == http.MethodGet {
		return shared.Render(c, http.StatusOK, credTmpl.FormPage(page, credTmpl.CredentialData{}, false, nil))
	}

	enable := c.FormValue("enable")
	cr := &AppCredential{
		AppCredUser:   c.FormValue("cred_user"),
		AppCredName:   c.FormValue("cred_name"),
		AppCredEnable: &enable,
		CreatedBy:     mw.GetCurrentUserName(c),
		CreatedDt:     time.Now(),
	}

	var errors []string
	if cr.AppCredUser == "" {
		errors = append(errors, "User is required")
	}
	if cr.AppCredName == "" {
		errors = append(errors, "Name is required")
	}

	if len(errors) > 0 {
		return shared.Render(c, http.StatusOK, credTmpl.FormPage(page, toCredentialData(*cr), false, errors))
	}

	if err := h.repo.CreateCredential(cr); err != nil {
		return shared.Render(c, http.StatusOK, credTmpl.FormPage(page, toCredentialData(*cr), false, []string{err.Error()}))
	}

	shared.SetFlash(c, h.store, h.sessionName, shared.FlashSuccess, "Credential created successfully")
	return c.Redirect(http.StatusFound, "/appcredential/index")
}

// CredentialUpdate handles GET (show form) and POST (process form) for editing a credential.
func (h *Handler) CredentialUpdate(c echo.Context) error {
	id, err := strconv.Atoi(c.QueryParam("id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid credential ID")
	}

	existing, err := h.repo.FindCredentialByID(id)
	if err != nil {
		return echo.NewHTTPError(http.StatusNotFound, "credential not found")
	}

	page := h.pageData(c, "Edit Credential")

	if c.Request().Method == http.MethodGet {
		return shared.Render(c, http.StatusOK, credTmpl.FormPage(page, toCredentialData(*existing), true, nil))
	}

	enable := c.FormValue("enable")
	existing.AppCredUser = c.FormValue("cred_user")
	existing.AppCredName = c.FormValue("cred_name")
	existing.AppCredEnable = &enable

	var errors []string
	if existing.AppCredUser == "" {
		errors = append(errors, "User is required")
	}
	if existing.AppCredName == "" {
		errors = append(errors, "Name is required")
	}

	if len(errors) > 0 {
		return shared.Render(c, http.StatusOK, credTmpl.FormPage(page, toCredentialData(*existing), true, errors))
	}

	if err := h.repo.UpdateCredential(existing); err != nil {
		return shared.Render(c, http.StatusOK, credTmpl.FormPage(page, toCredentialData(*existing), true, []string{err.Error()}))
	}

	shared.SetFlash(c, h.store, h.sessionName, shared.FlashSuccess, "Credential updated successfully")
	return c.Redirect(http.StatusFound, "/appcredential/index")
}

// CredentialDelete removes a credential by ID.
func (h *Handler) CredentialDelete(c echo.Context) error {
	id, err := strconv.Atoi(c.QueryParam("id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid credential ID")
	}

	if err := h.repo.DeleteCredential(id); err != nil {
		shared.SetFlash(c, h.store, h.sessionName, shared.FlashError, fmt.Sprintf("Failed to delete credential: %v", err))
	} else {
		shared.SetFlash(c, h.store, h.sessionName, shared.FlashSuccess, "Credential deleted successfully")
	}

	return c.Redirect(http.StatusFound, "/appcredential/index")
}
