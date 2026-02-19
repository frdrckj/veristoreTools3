package activation

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"
	mw "github.com/verifone/veristoretools3/internal/middleware"
	"github.com/verifone/veristoretools3/internal/shared"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	testBasicAuthUser = "Vfiengineering"
	testBasicAuthPass = "Welcome@123!"
)

// setupEcho creates an Echo instance with the activation API route wired up
// behind BasicAuth middleware, matching the real server setup.
func setupEcho() (*echo.Echo, *APIHandler) {
	e := echo.New()

	// APIHandler with nil repo and db -- the handler's nil guard on
	// repo/db means the best-effort database logging is safely skipped
	// when running without a database connection.
	handler := &APIHandler{
		repo: nil,
		db:   nil,
	}

	api := e.Group("/feature/api", mw.BasicAuth(testBasicAuthUser, testBasicAuthPass))
	api.POST("/activation-code", handler.ActivationCode)

	return e, handler
}

// TestActivationCode_NoAuth_Returns401 verifies that the API endpoint
// rejects requests without Basic Auth credentials.
func TestActivationCode_NoAuth_Returns401(t *testing.T) {
	e, _ := setupEcho()

	body := `{"csi":"12345678","tid":"TID001","mid":"MID001","model":"X990","version":"1.0.0"}`
	req := httptest.NewRequest(http.MethodPost, "/feature/api/activation-code",
		strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	e.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

// TestActivationCode_WrongAuth_Returns401 verifies that the API endpoint
// rejects requests with incorrect Basic Auth credentials.
func TestActivationCode_WrongAuth_Returns401(t *testing.T) {
	e, _ := setupEcho()

	body := `{"csi":"12345678","tid":"TID001","mid":"MID001","model":"X990","version":"1.0.0"}`
	req := httptest.NewRequest(http.MethodPost, "/feature/api/activation-code",
		strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.SetBasicAuth("wronguser", "wrongpass")
	rec := httptest.NewRecorder()

	e.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

// TestActivationCode_ValidAuth_Returns200 verifies that the API endpoint
// returns a valid activation code when proper authentication and request
// body are provided.
func TestActivationCode_ValidAuth_Returns200(t *testing.T) {
	e, _ := setupEcho()

	body := `{"csi":"12345678","tid":"TID001","mid":"MID001","model":"X990","version":"1.0.0"}`
	req := httptest.NewRequest(http.MethodPost, "/feature/api/activation-code",
		strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.SetBasicAuth(testBasicAuthUser, testBasicAuthPass)
	rec := httptest.NewRecorder()

	e.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	// Parse the response.
	var resp shared.APIResponse
	err := json.Unmarshal(rec.Body.Bytes(), &resp)
	require.NoError(t, err)

	assert.Equal(t, 0, resp.Code, "API response code should be 0 (success)")
	assert.Equal(t, "Success", resp.Description)

	// The activation code should be in the data field.
	dataMap, ok := resp.Data.(map[string]interface{})
	require.True(t, ok, "data should be a JSON object")

	code, ok := dataMap["activation_code"].(string)
	require.True(t, ok, "activation_code should be a string")
	assert.Len(t, code, 6, "activation code should be 6 characters")
	assert.Regexp(t, `^[0-9A-F]{6}$`, code, "activation code should be uppercase hex")
}

// TestActivationCode_MissingFields_ReturnsError verifies that the API endpoint
// returns an error when required fields (csi, tid, mid) are missing.
func TestActivationCode_MissingFields_ReturnsError(t *testing.T) {
	e, _ := setupEcho()

	tests := []struct {
		name string
		body string
	}{
		{"missing_csi", `{"tid":"TID001","mid":"MID001","model":"X990","version":"1.0.0"}`},
		{"missing_tid", `{"csi":"12345678","mid":"MID001","model":"X990","version":"1.0.0"}`},
		{"missing_mid", `{"csi":"12345678","tid":"TID001","model":"X990","version":"1.0.0"}`},
		{"all_empty", `{"csi":"","tid":"","mid":"","model":"","version":""}`},
		{"empty_body", `{}`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/feature/api/activation-code",
				strings.NewReader(tt.body))
			req.Header.Set("Content-Type", "application/json")
			req.SetBasicAuth(testBasicAuthUser, testBasicAuthPass)
			rec := httptest.NewRecorder()

			e.ServeHTTP(rec, req)

			assert.Equal(t, http.StatusBadRequest, rec.Code)

			var resp shared.APIResponse
			err := json.Unmarshal(rec.Body.Bytes(), &resp)
			require.NoError(t, err)
			assert.Equal(t, 1, resp.Code, "API response code should be 1 (error)")
		})
	}
}

// TestActivationCode_InvalidJSON_ReturnsError verifies that malformed JSON
// is rejected.
func TestActivationCode_InvalidJSON_ReturnsError(t *testing.T) {
	e, _ := setupEcho()

	req := httptest.NewRequest(http.MethodPost, "/feature/api/activation-code",
		strings.NewReader("this is not json"))
	req.Header.Set("Content-Type", "application/json")
	req.SetBasicAuth(testBasicAuthUser, testBasicAuthPass)
	rec := httptest.NewRecorder()

	e.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

// TestActivationCode_ModelAndVersionOptional verifies that the model and
// version fields are optional -- the endpoint should still return a valid
// activation code when they are empty.
func TestActivationCode_ModelAndVersionOptional(t *testing.T) {
	e, _ := setupEcho()

	body := `{"csi":"12345678","tid":"TID001","mid":"MID001"}`
	req := httptest.NewRequest(http.MethodPost, "/feature/api/activation-code",
		strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.SetBasicAuth(testBasicAuthUser, testBasicAuthPass)
	rec := httptest.NewRecorder()

	e.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	var resp shared.APIResponse
	err := json.Unmarshal(rec.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Equal(t, 0, resp.Code)
}

// TestActivationCode_WrongMethod verifies that GET requests are not accepted
// on the activation-code endpoint (only POST is registered).
func TestActivationCode_WrongMethod(t *testing.T) {
	e, _ := setupEcho()

	req := httptest.NewRequest(http.MethodGet, "/feature/api/activation-code", nil)
	req.SetBasicAuth(testBasicAuthUser, testBasicAuthPass)
	rec := httptest.NewRecorder()

	e.ServeHTTP(rec, req)

	// Echo v4 returns 404 or 405 depending on configuration for unmatched
	// methods. Both indicate the GET request was correctly rejected.
	assert.True(t, rec.Code == http.StatusMethodNotAllowed || rec.Code == http.StatusNotFound,
		"expected 404 or 405 for wrong HTTP method, got %d", rec.Code)
}
