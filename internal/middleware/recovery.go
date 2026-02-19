package middleware

import (
	"fmt"
	"net/http"
	"runtime/debug"

	"github.com/labstack/echo/v4"
	"github.com/rs/zerolog/log"
)

// Recovery returns Echo middleware that recovers from panics, logs the stack
// trace with zerolog, and returns a 500 Internal Server Error response.
func Recovery() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			defer func() {
				if r := recover(); r != nil {
					var errMsg string
					switch v := r.(type) {
					case error:
						errMsg = v.Error()
					default:
						errMsg = fmt.Sprintf("%v", v)
					}

					stack := debug.Stack()
					log.Error().
						Str("error", errMsg).
						Str("stack", string(stack)).
						Str("method", c.Request().Method).
						Str("uri", c.Request().RequestURI).
						Msg("panic recovered")

					c.Error(echo.NewHTTPError(http.StatusInternalServerError, "internal server error"))
				}
			}()
			return next(c)
		}
	}
}
