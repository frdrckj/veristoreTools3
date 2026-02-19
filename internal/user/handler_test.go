package user

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gorilla/sessions"
	"github.com/labstack/echo/v4"
	mw "github.com/verifone/veristoretools3/internal/middleware"
	"github.com/stretchr/testify/assert"
)

// TestUserIndex_RequiresAuth verifies that the /user/index route redirects to
// the login page when the session does not contain a valid user_id.
func TestUserIndex_RequiresAuth(t *testing.T) {
	store := sessions.NewCookieStore([]byte("test-secret-key-32-chars-long!!"))
	sessionName := "_test_session"

	e := echo.New()

	// Simulate a protected handler behind SessionAuth middleware.
	dummyHandler := func(c echo.Context) error {
		return c.String(http.StatusOK, "user index page")
	}
	handler := mw.SessionAuth(store, sessionName)(dummyHandler)

	req := httptest.NewRequest(http.MethodGet, "/user/index", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	err := handler(c)
	assert.NoError(t, err)

	// Without a valid session, should redirect to login.
	assert.Equal(t, http.StatusFound, rec.Code)
	assert.Equal(t, "/user/login", rec.Header().Get("Location"))
}

// TestUserIndex_WithAuth verifies that the /user/index route is accessible
// when the session contains a valid user_id.
func TestUserIndex_WithAuth(t *testing.T) {
	store := sessions.NewCookieStore([]byte("test-secret-key-32-chars-long!!"))
	sessionName := "_test_session"

	e := echo.New()

	dummyHandler := func(c echo.Context) error {
		return c.String(http.StatusOK, "user index page")
	}
	handler := mw.SessionAuth(store, sessionName)(dummyHandler)

	// Create a session with valid user data.
	req := httptest.NewRequest(http.MethodGet, "/user/index", nil)
	rec := httptest.NewRecorder()

	session, _ := store.Get(req, sessionName)
	session.Values[mw.SessionUserID] = 1
	session.Values[mw.SessionUserName] = "admin"
	session.Values[mw.SessionUserPrivileges] = "ADMIN"
	session.Values[mw.SessionUserFullname] = "Admin User"
	session.Save(req, rec)

	// Use the session cookie in a fresh request.
	cookies := rec.Result().Cookies()
	req2 := httptest.NewRequest(http.MethodGet, "/user/index", nil)
	for _, cookie := range cookies {
		req2.AddCookie(cookie)
	}
	rec2 := httptest.NewRecorder()
	c := e.NewContext(req2, rec2)

	err := handler(c)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, rec2.Code)
	assert.Equal(t, "user index page", rec2.Body.String())
}

// TestNewHandler_DoesNotPanic verifies that creating a new handler does not
// panic even with nil dependencies (useful for confirming wiring).
func TestNewHandler_DoesNotPanic(t *testing.T) {
	store := sessions.NewCookieStore([]byte("test-secret"))
	h := NewHandler(nil, nil, store, "_session", "App", "1.0")
	assert.NotNil(t, h)
	assert.Equal(t, "App", h.appName)
	assert.Equal(t, "1.0", h.appVersion)
}

// TestSetBranding verifies that SetBranding updates handler fields.
func TestSetBranding(t *testing.T) {
	store := sessions.NewCookieStore([]byte("test-secret"))
	h := NewHandler(nil, nil, store, "_session", "App", "1.0")

	h.SetBranding("client.png", "veristore.png", "http://tms", "#fff", "Acme", "https://acme.com")

	assert.Equal(t, "client.png", h.appClientLogo)
	assert.Equal(t, "veristore.png", h.appVeristoreLogo)
	assert.Equal(t, "http://tms", h.appTmsURL)
	assert.Equal(t, "#fff", h.appBgColor)
	assert.Equal(t, "Acme", h.copyrightTitle)
	assert.Equal(t, "https://acme.com", h.copyrightURL)
}

// TestSetBranding_EmptyStringsNoOverwrite verifies that empty strings do not
// overwrite existing branding values.
func TestSetBranding_EmptyStringsNoOverwrite(t *testing.T) {
	store := sessions.NewCookieStore([]byte("test-secret"))
	h := NewHandler(nil, nil, store, "_session", "App", "1.0")

	original := h.copyrightTitle
	h.SetBranding("", "", "", "", "", "")

	assert.Equal(t, original, h.copyrightTitle)
}
