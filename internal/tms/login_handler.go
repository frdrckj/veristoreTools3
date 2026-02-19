package tms

import (
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/gorilla/sessions"
	"github.com/labstack/echo/v4"
	mw "github.com/verifone/veristoretools3/internal/middleware"
	"github.com/verifone/veristoretools3/internal/shared"
	"github.com/verifone/veristoretools3/templates/layouts"
	tmsTmpl "github.com/verifone/veristoretools3/templates/tmslogin"
)

// LoginHandler holds dependencies for TMS login management HTTP handlers.
type LoginHandler struct {
	repo        *Repository
	service     *Service
	store       sessions.Store
	sessionName string
	appName     string
	appVersion  string
}

// NewLoginHandler creates a new TMS login handler.
func NewLoginHandler(repo *Repository, service *Service, store sessions.Store, sessionName, appName, appVersion string) *LoginHandler {
	return &LoginHandler{
		repo:        repo,
		service:     service,
		store:       store,
		sessionName: sessionName,
		appName:     appName,
		appVersion:  appVersion,
	}
}

// pageData builds a layouts.PageData from the echo context and handler config.
func (h *LoginHandler) pageData(c echo.Context, title string) layouts.PageData {
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

// toLoginData converts a TmsLogin model to a tmsTmpl.LoginData view struct.
func toLoginData(l TmsLogin) tmsTmpl.LoginData {
	user := ""
	if l.TmsLoginUser != nil {
		user = *l.TmsLoginUser
	}
	scheduled := ""
	if l.TmsLoginScheduled != nil {
		scheduled = *l.TmsLoginScheduled
	}
	enable := ""
	if l.TmsLoginEnable != nil {
		enable = *l.TmsLoginEnable
	}
	return tmsTmpl.LoginData{
		TmsLoginID:        l.TmsLoginID,
		TmsLoginUser:      user,
		TmsLoginScheduled: scheduled,
		TmsLoginEnable:    enable,
		CreatedBy:         l.CreatedBy,
		CreatedDt:         l.CreatedDt.Format("2006-01-02 15:04:05"),
	}
}

// toLoginDataSlice converts a slice of TmsLogin models to view structs.
func toLoginDataSlice(logins []TmsLogin) []tmsTmpl.LoginData {
	result := make([]tmsTmpl.LoginData, len(logins))
	for i, l := range logins {
		result[i] = toLoginData(l)
	}
	return result
}

// Index lists TMS login sessions.
func (h *LoginHandler) Index(c echo.Context) error {
	logins, err := h.repo.FindAllLogins()
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to load TMS logins")
	}

	page := h.pageData(c, "TMS Login Management")
	loginData := toLoginDataSlice(logins)

	return shared.Render(c, http.StatusOK, tmsTmpl.IndexPage(page, loginData))
}

// Create handles POST for creating a new TMS login.
func (h *LoginHandler) Create(c echo.Context) error {
	user := c.FormValue("tms_login_user")
	if user == "" {
		shared.SetFlash(c, h.store, h.sessionName, shared.FlashError, "TMS login user is required")
		return c.Redirect(http.StatusFound, "/tmslogin/index")
	}

	enable := "1"
	login := &TmsLogin{
		TmsLoginUser:   &user,
		TmsLoginEnable: &enable,
		CreatedBy:      mw.GetCurrentUserName(c),
		CreatedDt:      time.Now(),
	}

	if err := h.repo.CreateLogin(login); err != nil {
		shared.SetFlash(c, h.store, h.sessionName, shared.FlashError, fmt.Sprintf("Failed to create TMS login: %v", err))
	} else {
		shared.SetFlash(c, h.store, h.sessionName, shared.FlashSuccess, "TMS login created successfully")
	}

	return c.Redirect(http.StatusFound, "/tmslogin/index")
}

// Delete removes a TMS login by ID.
func (h *LoginHandler) Delete(c echo.Context) error {
	id, err := strconv.Atoi(c.QueryParam("id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid TMS login ID")
	}

	if err := h.repo.DeleteLogin(id); err != nil {
		shared.SetFlash(c, h.store, h.sessionName, shared.FlashError, fmt.Sprintf("Failed to delete TMS login: %v", err))
	} else {
		shared.SetFlash(c, h.store, h.sessionName, shared.FlashSuccess, "TMS login deleted successfully")
	}

	return c.Redirect(http.StatusFound, "/tmslogin/index")
}

// GetOperator handles GET /tmslogin/getoperator - returns operator dropdown data.
// This is an HTMX endpoint that returns JSON with operator list from TMS.
func (h *LoginHandler) GetOperator(c echo.Context) error {
	username := c.QueryParam("username")
	if username == "" {
		return c.JSON(http.StatusOK, map[string]interface{}{"data": []interface{}{}})
	}

	resp, err := h.service.GetResellerList(username)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]interface{}{"error": err.Error()})
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"code": resp.ResultCode,
		"data": resp.RawData,
	})
}

// GetVerifyCode handles GET /tmslogin/getverifycode - returns captcha image data.
func (h *LoginHandler) GetVerifyCode(c echo.Context) error {
	resp, err := h.service.GetVerifyCode()
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]interface{}{"error": err.Error()})
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"code": resp.ResultCode,
		"data": resp.Data,
	})
}
