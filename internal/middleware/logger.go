package middleware

import (
	"time"

	"github.com/labstack/echo/v4"
	"github.com/rs/zerolog/log"
)

// RequestLogger returns Echo middleware that logs each HTTP request using
// zerolog. It records the method, URI, status code, latency and remote IP.
func RequestLogger() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			start := time.Now()

			err := next(c)
			if err != nil {
				c.Error(err)
			}

			req := c.Request()
			res := c.Response()
			latency := time.Since(start)

			event := log.Info()
			if res.Status >= 500 {
				event = log.Error()
			} else if res.Status >= 400 {
				event = log.Warn()
			}

			event.
				Str("method", req.Method).
				Str("uri", req.RequestURI).
				Int("status", res.Status).
				Dur("latency", latency).
				Str("remote_ip", c.RealIP()).
				Int64("bytes_out", res.Size).
				Msg("request")

			return nil
		}
	}
}
