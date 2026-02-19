package shared

import (
	"github.com/gorilla/sessions"
	"github.com/labstack/echo/v4"
)

const (
	FlashSuccess = "success"
	FlashError   = "error"
	FlashInfo    = "info"
	FlashWarning = "warning"
)

func SetFlash(c echo.Context, store sessions.Store, sessionName, flashType, message string) {
	session, _ := store.Get(c.Request(), sessionName)
	session.AddFlash(message, flashType)
	session.Save(c.Request(), c.Response())
}

func GetFlashes(c echo.Context, store sessions.Store, sessionName string) map[string][]string {
	session, _ := store.Get(c.Request(), sessionName)
	flashes := make(map[string][]string)
	for _, flashType := range []string{FlashSuccess, FlashError, FlashInfo, FlashWarning} {
		if msgs := session.Flashes(flashType); len(msgs) > 0 {
			for _, msg := range msgs {
				if s, ok := msg.(string); ok {
					flashes[flashType] = append(flashes[flashType], s)
				}
			}
		}
	}
	session.Save(c.Request(), c.Response())
	return flashes
}
