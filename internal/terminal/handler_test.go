package terminal

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gorilla/sessions"
	"github.com/labstack/echo/v4"
	mw "github.com/verifone/veristoretools3/internal/middleware"
	"github.com/stretchr/testify/assert"
)

// TestTerminalIndex_RequiresAuth verifies that the /terminal/index route
// redirects to the login page when the session does not contain a valid
// user_id.
func TestTerminalIndex_RequiresAuth(t *testing.T) {
	store := sessions.NewCookieStore([]byte("test-secret-key-32-chars-long!!"))
	sessionName := "_test_session"

	e := echo.New()

	// Simulate a protected handler behind SessionAuth middleware.
	dummyHandler := func(c echo.Context) error {
		return c.String(http.StatusOK, "terminal index page")
	}
	handler := mw.SessionAuth(store, sessionName)(dummyHandler)

	req := httptest.NewRequest(http.MethodGet, "/terminal/index", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	err := handler(c)
	assert.NoError(t, err)

	// Without a valid session, should redirect to login.
	assert.Equal(t, http.StatusFound, rec.Code)
	assert.Equal(t, "/user/login", rec.Header().Get("Location"))
}

// TestTerminalIndex_WithAuth verifies that the /terminal/index route is
// accessible when the session contains a valid user_id.
func TestTerminalIndex_WithAuth(t *testing.T) {
	store := sessions.NewCookieStore([]byte("test-secret-key-32-chars-long!!"))
	sessionName := "_test_session"

	e := echo.New()

	dummyHandler := func(c echo.Context) error {
		return c.String(http.StatusOK, "terminal index page")
	}
	handler := mw.SessionAuth(store, sessionName)(dummyHandler)

	// Create a session with valid user data.
	req := httptest.NewRequest(http.MethodGet, "/terminal/index", nil)
	rec := httptest.NewRecorder()

	session, _ := store.Get(req, sessionName)
	session.Values[mw.SessionUserID] = 1
	session.Values[mw.SessionUserName] = "operator"
	session.Values[mw.SessionUserPrivileges] = "OPERATOR"
	session.Values[mw.SessionUserFullname] = "Operator User"
	session.Save(req, rec)

	// Use the session cookie in a fresh request.
	cookies := rec.Result().Cookies()
	req2 := httptest.NewRequest(http.MethodGet, "/terminal/index", nil)
	for _, cookie := range cookies {
		req2.AddCookie(cookie)
	}
	rec2 := httptest.NewRecorder()
	c := e.NewContext(req2, rec2)

	err := handler(c)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, rec2.Code)
	assert.Equal(t, "terminal index page", rec2.Body.String())
}

// TestNewHandler_DoesNotPanic verifies that creating a terminal handler
// does not panic with nil service.
func TestNewHandler_DoesNotPanic(t *testing.T) {
	store := sessions.NewCookieStore([]byte("test-secret"))
	h := NewHandler(nil, store, "_session", "App", "1.0")
	assert.NotNil(t, h)
	assert.Equal(t, "App", h.appName)
	assert.Equal(t, "1.0", h.appVersion)
}

// TestPtrStr verifies the ptrStr helper.
func TestPtrStr(t *testing.T) {
	s := "hello"
	assert.Equal(t, "hello", ptrStr(&s))
	assert.Equal(t, "", ptrStr(nil))
}
