package middleware

import (
	"crypto/subtle"
	"net/http"

	"github.com/gorilla/sessions"
	"github.com/labstack/echo/v4"
)

// Session key constants used to store/retrieve user data from sessions.
const (
	SessionUserID         = "user_id"
	SessionUserName       = "user_name"
	SessionUserPrivileges = "user_privileges"
	SessionUserFullname   = "user_fullname"
)

// SessionAuth returns Echo middleware that checks whether the session contains
// a valid user_id. Unauthenticated requests are redirected to /user/login.
// For HTMX requests the redirect is sent via the HX-Redirect header.
func SessionAuth(store sessions.Store, sessionName string) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			session, err := store.Get(c.Request(), sessionName)
			if err != nil {
				return redirectToLogin(c)
			}

			userID, ok := session.Values[SessionUserID]
			if !ok || userID == nil {
				return redirectToLogin(c)
			}

			// Store session values in the echo.Context for downstream handlers.
			c.Set(SessionUserID, userID)
			if v, ok := session.Values[SessionUserName]; ok {
				c.Set(SessionUserName, v)
			}
			if v, ok := session.Values[SessionUserPrivileges]; ok {
				c.Set(SessionUserPrivileges, v)
			}
			if v, ok := session.Values[SessionUserFullname]; ok {
				c.Set(SessionUserFullname, v)
			}

			return next(c)
		}
	}
}

// redirectToLogin sends the user to the login page. For HTMX requests the
// HX-Redirect header is used so the browser performs a full navigation.
func redirectToLogin(c echo.Context) error {
	if c.Request().Header.Get("HX-Request") == "true" {
		c.Response().Header().Set("HX-Redirect", "/user/login")
		return c.NoContent(http.StatusUnauthorized)
	}
	return c.Redirect(http.StatusFound, "/user/login")
}

// BasicAuth returns Echo middleware that performs HTTP Basic Authentication
// against a single username/password pair. Intended for REST API routes.
func BasicAuth(username, password string) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			user, pass, ok := c.Request().BasicAuth()
			if !ok {
				return echo.NewHTTPError(http.StatusUnauthorized, "missing credentials")
			}

			userMatch := subtle.ConstantTimeCompare([]byte(user), []byte(username)) == 1
			passMatch := subtle.ConstantTimeCompare([]byte(pass), []byte(password)) == 1

			if !userMatch || !passMatch {
				return echo.NewHTTPError(http.StatusUnauthorized, "invalid credentials")
			}

			return next(c)
		}
	}
}

// GetCurrentUserID retrieves the current user's ID from the echo.Context.
func GetCurrentUserID(c echo.Context) int {
	if v, ok := c.Get(SessionUserID).(int); ok {
		return v
	}
	return 0
}

// GetCurrentUserName retrieves the current user's username from the echo.Context.
func GetCurrentUserName(c echo.Context) string {
	if v, ok := c.Get(SessionUserName).(string); ok {
		return v
	}
	return ""
}

// GetCurrentUserPrivileges retrieves the current user's privileges from the echo.Context.
func GetCurrentUserPrivileges(c echo.Context) string {
	if v, ok := c.Get(SessionUserPrivileges).(string); ok {
		return v
	}
	return ""
}

// GetCurrentUserFullname retrieves the current user's full name from the echo.Context.
func GetCurrentUserFullname(c echo.Context) string {
	if v, ok := c.Get(SessionUserFullname).(string); ok {
		return v
	}
	return ""
}
