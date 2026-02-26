package auth

import (
	"net/http"

	"github.com/a-h/templ"
	"github.com/gorilla/sessions"
	"github.com/labstack/echo/v4"
	mw "github.com/verifone/veristoretools3/internal/middleware"
	"github.com/verifone/veristoretools3/internal/shared"
	"github.com/verifone/veristoretools3/templates/layouts"
	userTmpl "github.com/verifone/veristoretools3/templates/user"
)

// Handler holds dependencies for authentication HTTP handlers.
type Handler struct {
	service           *Service
	store             sessions.Store
	sessionName       string
	appName           string
	appVersion        string
	appIcon           string
	appLogo           string
	clearTmsSession   func(username string) error          // called on login/logout to clear per-user TMS session
	saveTmsPassword   func(username, plainPassword string)  // called on login to save encrypted TMS password (like v2)
}

// NewHandler creates a new auth handler.
func NewHandler(service *Service, store sessions.Store, sessionName, appName, appVersion string) *Handler {
	return &Handler{
		service:     service,
		store:       store,
		sessionName: sessionName,
		appName:     appName,
		appVersion:  appVersion,
		appIcon:     "favicon.png",
		appLogo:     "verifone_logo.png",
	}
}

// SetTmsSessionClearer sets the callback to clear a user's TMS session (like v2).
func (h *Handler) SetTmsSessionClearer(fn func(username string) error) {
	h.clearTmsSession = fn
}

// SetTmsPasswordSaver sets the callback to save the user's TMS password on app login (like v2).
func (h *Handler) SetTmsPasswordSaver(fn func(username, plainPassword string)) {
	h.saveTmsPassword = fn
}

// Render is a helper to render templ components as Echo responses.
func Render(c echo.Context, statusCode int, component templ.Component) error {
	c.Response().Header().Set(echo.HeaderContentType, echo.MIMETextHTMLCharsetUTF8)
	c.Response().WriteHeader(statusCode)
	return component.Render(c.Request().Context(), c.Response())
}

// LoginPage renders the login page using the Templ login template.
func (h *Handler) LoginPage(c echo.Context) error {
	// Get flash messages for error display.
	flashes := shared.GetFlashes(c, h.store, h.sessionName)
	errorMsg := ""
	if errs, ok := flashes[shared.FlashError]; ok && len(errs) > 0 {
		errorMsg = errs[0]
	}

	loginData := layouts.LoginData{
		AppName:    h.appName,
		AppVersion: h.appVersion,
		AppIcon:    h.appIcon,
		AppLogo:    h.appLogo,
	}

	return Render(c, http.StatusOK, userTmpl.LoginPage(loginData, errorMsg))
}

// Login processes the login form submission. On success it creates a session
// and redirects to the dashboard. On failure it redirects back to the login
// page with a flash error.
func (h *Handler) Login(c echo.Context) error {
	username := c.FormValue("username")
	password := c.FormValue("password")

	u, err := h.service.Authenticate(username, password)
	if err != nil {
		shared.SetFlash(c, h.store, h.sessionName, shared.FlashError, "Invalid username or password")
		return c.Redirect(http.StatusFound, "/user/login")
	}

	session, err := h.store.Get(c.Request(), h.sessionName)
	if err != nil {
		shared.SetFlash(c, h.store, h.sessionName, shared.FlashError, "Session error")
		return c.Redirect(http.StatusFound, "/user/login")
	}

	session.Values[mw.SessionUserID] = u.UserID
	session.Values[mw.SessionUserName] = u.UserName
	session.Values[mw.SessionUserPrivileges] = u.UserPrivileges
	session.Values[mw.SessionUserFullname] = u.UserFullname

	if err := session.Save(c.Request(), c.Response()); err != nil {
		shared.SetFlash(c, h.store, h.sessionName, shared.FlashError, "Failed to save session")
		return c.Redirect(http.StatusFound, "/user/login")
	}

	// Save encrypted TMS password on app login (like v2: same password for app and TMS).
	if h.saveTmsPassword != nil {
		h.saveTmsPassword(u.UserName, password)
	}

	// Clear per-user TMS session on app login so user must re-login to TMS (like v2).
	if h.clearTmsSession != nil {
		h.clearTmsSession(u.UserName)
	}

	// Log login activity (v2 parity).
	mw.LogActivityFromContext(c, mw.LogLogin, "")

	return c.Redirect(http.StatusFound, "/")
}

// Logout destroys the current session and redirects to the login page.
func (h *Handler) Logout(c echo.Context) error {
	// Log logout activity before clearing session (v2 parity).
	mw.LogActivityFromContext(c, mw.LogLogout, "")

	// Clear per-user TMS session and password on app logout (like v2).
	currentUser := mw.GetCurrentUserName(c)
	if currentUser != "" {
		if h.clearTmsSession != nil {
			h.clearTmsSession(currentUser)
		}
		// Clear TMS password like V2: Yii::$app->user->identity->tms_password = null
		if h.saveTmsPassword != nil {
			h.saveTmsPassword(currentUser, "")
		}
	}

	session, err := h.store.Get(c.Request(), h.sessionName)
	if err == nil {
		// Clear all session values.
		session.Values = make(map[interface{}]interface{})
		session.Options.MaxAge = -1
		session.Save(c.Request(), c.Response())
	}

	return c.Redirect(http.StatusFound, "/user/login")
}
