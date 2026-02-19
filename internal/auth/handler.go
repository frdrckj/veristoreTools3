package auth

import (
	"net/http"

	"github.com/gorilla/sessions"
	"github.com/labstack/echo/v4"
	mw "github.com/verifone/veristoretools3/internal/middleware"
	"github.com/verifone/veristoretools3/internal/shared"
)

// Handler holds dependencies for authentication HTTP handlers.
type Handler struct {
	service     *Service
	store       sessions.Store
	sessionName string
}

// NewHandler creates a new auth handler.
func NewHandler(service *Service, store sessions.Store, sessionName string) *Handler {
	return &Handler{
		service:     service,
		store:       store,
		sessionName: sessionName,
	}
}

// LoginPage renders the login page.
// TODO: Replace with Templ component in Phase 4.
func (h *Handler) LoginPage(c echo.Context) error {
	html := `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Login - VeriStore Tools</title>
</head>
<body>
    <h1>VeriStore Tools Login</h1>
    <form method="POST" action="/user/login">
        <div>
            <label for="username">Username</label>
            <input type="text" id="username" name="username" required />
        </div>
        <div>
            <label for="password">Password</label>
            <input type="password" id="password" name="password" required />
        </div>
        <button type="submit">Login</button>
    </form>
</body>
</html>`
	return c.HTML(http.StatusOK, html)
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

	return c.Redirect(http.StatusFound, "/")
}

// Logout destroys the current session and redirects to the login page.
func (h *Handler) Logout(c echo.Context) error {
	session, err := h.store.Get(c.Request(), h.sessionName)
	if err == nil {
		// Clear all session values.
		session.Values = make(map[interface{}]interface{})
		session.Options.MaxAge = -1
		session.Save(c.Request(), c.Response())
	}

	return c.Redirect(http.StatusFound, "/user/login")
}
