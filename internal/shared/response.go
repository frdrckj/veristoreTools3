package shared

import (
	"net/http"

	"github.com/a-h/templ"
	"github.com/labstack/echo/v4"
)

func Render(c echo.Context, status int, component templ.Component) error {
	c.Response().Header().Set(echo.HeaderContentType, echo.MIMETextHTMLCharsetUTF8)
	c.Response().WriteHeader(status)
	return component.Render(c.Request().Context(), c.Response())
}

func IsHTMX(c echo.Context) bool {
	return c.Request().Header.Get("HX-Request") == "true"
}

type APIResponse struct {
	Code        int         `json:"code"`
	Description string      `json:"description"`
	Data        interface{} `json:"data,omitempty"`
}

func APISuccess(c echo.Context, data interface{}) error {
	return c.JSON(http.StatusOK, APIResponse{
		Code:        0,
		Description: "Success",
		Data:        data,
	})
}

func APIError(c echo.Context, status int, message string) error {
	return c.JSON(status, APIResponse{
		Code:        1,
		Description: message,
	})
}
