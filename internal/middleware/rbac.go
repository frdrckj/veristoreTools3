package middleware

import (
	"net/http"

	"github.com/casbin/casbin/v2"
	"github.com/labstack/echo/v4"
)

// RBAC returns Echo middleware that uses a Casbin enforcer to check whether the
// current user's role (privilege) is allowed to access the requested path with
// the given HTTP method.
func RBAC(enforcer *casbin.Enforcer) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			role := GetCurrentUserPrivileges(c)
			if role == "" {
				return echo.NewHTTPError(http.StatusForbidden, "access denied: no role assigned")
			}

			path := c.Request().URL.Path
			method := c.Request().Method

			allowed, err := enforcer.Enforce(role, path, method)
			if err != nil {
				return echo.NewHTTPError(http.StatusInternalServerError, "authorization error")
			}

			if !allowed {
				return echo.NewHTTPError(http.StatusForbidden, "access denied")
			}

			return next(c)
		}
	}
}
