package tms

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// mapResponseCode
// ---------------------------------------------------------------------------

func TestMapResponseCode(t *testing.T) {
	tests := []struct {
		name     string
		input    int
		expected int
	}{
		{"200 maps to 0 (success)", 200, 0},
		{"0 maps to 99 (generic error)", 0, 99},
		{"400 passes through", 400, 400},
		{"500 passes through", 500, 500},
		{"800 passes through", 800, 800},
		{"1 passes through", 1, 1},
		{"-1 passes through", -1, -1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := mapResponseCode(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// ---------------------------------------------------------------------------
// renewToken detection
// ---------------------------------------------------------------------------

func TestRenewToken_DetectsRenewal(t *testing.T) {
	// Create client without DB (nil) to test detection only.
	client := NewClient("https://example.com", "https://example.com", nil, true, "", "")

	response := map[string]interface{}{
		"code": float64(200),
		"desc": "toke更新",
		"data": "new-token-value-12345",
	}

	newToken, ok := client.renewToken("old-session", response)
	assert.True(t, ok, "should detect token renewal")
	assert.Equal(t, "new-token-value-12345", newToken)
}

func TestRenewToken_NoRenewalOnNormalResponse(t *testing.T) {
	client := NewClient("https://example.com", "https://example.com", nil, true, "", "")

	response := map[string]interface{}{
		"code": float64(200),
		"desc": "success",
		"data": map[string]interface{}{"key": "value"},
	}

	newToken, ok := client.renewToken("session", response)
	assert.False(t, ok, "should not detect renewal on normal response")
	assert.Equal(t, "", newToken)
}

func TestRenewToken_NoRenewalOnErrorCode(t *testing.T) {
	client := NewClient("https://example.com", "https://example.com", nil, true, "", "")

	response := map[string]interface{}{
		"code": float64(400),
		"desc": "toke更新",
		"data": "some-token",
	}

	newToken, ok := client.renewToken("session", response)
	assert.False(t, ok, "should not detect renewal when code != 200")
	assert.Equal(t, "", newToken)
}

func TestRenewToken_NilResponse(t *testing.T) {
	client := NewClient("https://example.com", "https://example.com", nil, true, "", "")

	newToken, ok := client.renewToken("session", nil)
	assert.False(t, ok, "should not detect renewal on nil response")
	assert.Equal(t, "", newToken)
}

func TestRenewToken_EmptyTokenData(t *testing.T) {
	client := NewClient("https://example.com", "https://example.com", nil, true, "", "")

	response := map[string]interface{}{
		"code": float64(200),
		"desc": "toke更新",
		"data": "",
	}

	newToken, ok := client.renewToken("session", response)
	assert.False(t, ok, "should not detect renewal when data is empty")
	assert.Equal(t, "", newToken)
}

func TestRenewToken_StringCodeMatches(t *testing.T) {
	client := NewClient("https://example.com", "https://example.com", nil, true, "", "")

	// Some APIs return code as string "200" instead of number.
	response := map[string]interface{}{
		"code": "200",
		"desc": "toke更新",
		"data": "new-token",
	}

	newToken, ok := client.renewToken("session", response)
	assert.True(t, ok, "should detect renewal with string code")
	assert.Equal(t, "new-token", newToken)
}

// ---------------------------------------------------------------------------
// NewClient
// ---------------------------------------------------------------------------

func TestNewClient_Initialization(t *testing.T) {
	client := NewClient("https://tms.example.com/", "https://api.example.com/", nil, false, "ak", "as")

	assert.NotNil(t, client)
	assert.Equal(t, "https://tms.example.com", client.baseURL, "trailing slash should be trimmed")
	assert.Equal(t, "https://api.example.com", client.apiBaseURL, "trailing slash should be trimmed")
	assert.NotNil(t, client.httpClient)
	assert.Nil(t, client.db)
	assert.Equal(t, "ak", client.accessKey)
	assert.Equal(t, "as", client.accessSecret)
}

func TestNewClient_HTTPTimeout(t *testing.T) {
	client := NewClient("https://tms.example.com", "https://tms.example.com", nil, false, "", "")
	assert.Equal(t, 30*1000*1000*1000, int(client.httpClient.Timeout), "timeout should be 30 seconds")
}

func TestNewClient_TLSSkipVerify(t *testing.T) {
	client := NewClient("https://tms.example.com", "https://tms.example.com", nil, true, "", "")

	transport, ok := client.httpClient.Transport.(*http.Transport)
	require.True(t, ok, "transport should be *http.Transport")
	require.NotNil(t, transport.TLSClientConfig, "TLS config should be set when skip is true")
	assert.True(t, transport.TLSClientConfig.InsecureSkipVerify, "TLS verification should be skipped")
}

func TestNewClient_TLSVerifyEnabled(t *testing.T) {
	client := NewClient("https://tms.example.com", "https://tms.example.com", nil, false, "", "")

	transport, ok := client.httpClient.Transport.(*http.Transport)
	require.True(t, ok, "transport should be *http.Transport")
	assert.Nil(t, transport.TLSClientConfig, "TLS config should be nil when verification is enabled")
}

// ---------------------------------------------------------------------------
// doPost via httptest
// ---------------------------------------------------------------------------

func TestDoPost_SendsCorrectHeaders(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify method.
		assert.Equal(t, http.MethodPost, r.Method)

		// Verify headers.
		assert.Equal(t, headerAccept, r.Header.Get("Accept"))
		assert.Equal(t, headerContentType, r.Header.Get("Content-Type"))
		assert.Equal(t, "test-session-token", r.Header.Get("Authorization"))

		// Verify body.
		body, err := io.ReadAll(r.Body)
		require.NoError(t, err)

		var reqBody map[string]interface{}
		err = json.Unmarshal(body, &reqBody)
		require.NoError(t, err)
		assert.Equal(t, "test-value", reqBody["key"])

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"code": 200,
			"desc": "success",
			"data": map[string]interface{}{"result": "ok"},
		})
	}))
	defer server.Close()

	client := NewClient(server.URL, server.URL, nil, false, "test-key", "test-secret")
	result, err := client.doPost("test-session-token", "/test/path", map[string]interface{}{
		"key": "test-value",
	})

	require.NoError(t, err)
	assert.NotNil(t, result)

	code, ok := toInt(result["code"])
	assert.True(t, ok)
	assert.Equal(t, 200, code)
}

func TestDoPost_NoAuthWhenSessionEmpty(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Empty(t, r.Header.Get("Authorization"), "should not set Authorization when session is empty")

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{"code": 200})
	}))
	defer server.Close()

	client := NewClient(server.URL, server.URL, nil, false, "test-key", "test-secret")
	_, err := client.doPost("", "/test", nil)
	require.NoError(t, err)
}

func TestDoPost_TokenRenewalRetry(t *testing.T) {
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.Header().Set("Content-Type", "application/json")

		if callCount == 1 {
			// First call: return token renewal response.
			json.NewEncoder(w).Encode(map[string]interface{}{
				"code": 200,
				"desc": "toke更新",
				"data": "renewed-token",
			})
		} else {
			// Second call (retry): verify the new token is used.
			assert.Equal(t, "renewed-token", r.Header.Get("Authorization"))
			json.NewEncoder(w).Encode(map[string]interface{}{
				"code": 200,
				"desc": "success",
				"data": map[string]interface{}{"retried": true},
			})
		}
	}))
	defer server.Close()

	client := NewClient(server.URL, server.URL, nil, false, "test-key", "test-secret")
	result, err := client.doPost("old-token", "/test", map[string]interface{}{})

	require.NoError(t, err)
	assert.Equal(t, 2, callCount, "should have made 2 requests (initial + retry)")
	assert.Equal(t, "success", result["desc"])
}

// ---------------------------------------------------------------------------
// doGet via httptest
// ---------------------------------------------------------------------------

func TestDoGet_SendsCorrectHeaders(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method)
		assert.Equal(t, headerAccept, r.Header.Get("Accept"))
		assert.Equal(t, headerContentType, r.Header.Get("Content-Type"))
		assert.Equal(t, "get-session-token", r.Header.Get("Authorization"))

		// Verify query parameters.
		assert.Equal(t, "testValue", r.URL.Query().Get("testKey"))

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"code": 200,
			"desc": "ok",
			"data": []interface{}{},
		})
	}))
	defer server.Close()

	client := NewClient(server.URL, server.URL, nil, false, "test-key", "test-secret")

	params := make(map[string][]string)
	params["testKey"] = []string{"testValue"}

	result, err := client.doGet("get-session-token", "/api/resource", params)

	require.NoError(t, err)
	assert.NotNil(t, result)

	code, ok := toInt(result["code"])
	assert.True(t, ok)
	assert.Equal(t, 200, code)
}

func TestDoGet_NilParams(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Empty(t, r.URL.RawQuery, "should have no query string with nil params")

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{"code": 200})
	}))
	defer server.Close()

	client := NewClient(server.URL, server.URL, nil, false, "test-key", "test-secret")
	_, err := client.doGet("session", "/test", nil)
	require.NoError(t, err)
}

// ---------------------------------------------------------------------------
// Utility functions
// ---------------------------------------------------------------------------

func TestToInt(t *testing.T) {
	tests := []struct {
		name     string
		input    interface{}
		expected int
		ok       bool
	}{
		{"float64", float64(42), 42, true},
		{"int", 42, 42, true},
		{"string", "42", 42, true},
		{"invalid string", "abc", 0, false},
		{"nil", nil, 0, false},
		{"bool", true, 0, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, ok := toInt(tt.input)
			assert.Equal(t, tt.expected, result)
			assert.Equal(t, tt.ok, ok)
		})
	}
}

func TestToString(t *testing.T) {
	assert.Equal(t, "", toString(nil))
	assert.Equal(t, "hello", toString("hello"))
	assert.Equal(t, "42", toString(42))
	assert.Equal(t, "3.14", toString(3.14))
}

func TestIntDiff(t *testing.T) {
	tests := []struct {
		name     string
		a, b     []int
		expected []int
	}{
		{"disjoint", []int{1, 2, 3}, []int{4, 5, 6}, []int{1, 2, 3}},
		{"overlap", []int{1, 2, 3, 4}, []int{2, 4}, []int{1, 3}},
		{"identical", []int{1, 2}, []int{1, 2}, nil},
		{"a empty", []int{}, []int{1, 2}, nil},
		{"b empty", []int{1, 2}, []int{}, []int{1, 2}},
		{"both nil", nil, nil, nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := intDiff(tt.a, tt.b)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// ---------------------------------------------------------------------------
// Integration-style tests using httptest for public methods
// ---------------------------------------------------------------------------

func TestGetResellerList(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/market/common/getMarketsByUser", r.URL.Path)
		assert.Equal(t, "1", r.URL.Query().Get("resellerId"))
		assert.Equal(t, "testuser", r.URL.Query().Get("username"))

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"code": 200,
			"desc": "success",
			"data": []interface{}{
				map[string]interface{}{"id": 1, "marketName": "Market A"},
				map[string]interface{}{"id": 2, "marketName": "Market B"},
			},
		})
	}))
	defer server.Close()

	client := NewClient(server.URL, server.URL, nil, false, "test-key", "test-secret")
	resp, err := client.GetResellerList("testuser")

	require.NoError(t, err)
	assert.Equal(t, 0, resp.ResultCode)
	assert.NotNil(t, resp.RawData)

	rawList, ok := resp.RawData.([]interface{})
	assert.True(t, ok)
	assert.Len(t, rawList, 2)
}

func TestGetVerifyCode(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/market/common/getCaptcha", r.URL.Path)

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"code": 200,
			"desc": "success",
			"data": map[string]interface{}{
				"uuid":  "abc-123",
				"image": "iVBORw0KGgo=",
			},
		})
	}))
	defer server.Close()

	client := NewClient(server.URL, server.URL, nil, false, "test-key", "test-secret")
	resp, err := client.GetVerifyCode()

	require.NoError(t, err)
	assert.Equal(t, 0, resp.ResultCode)
	assert.Equal(t, "abc-123", resp.Data["token"])
	assert.Equal(t, "data:image/png;base64,iVBORw0KGgo=", resp.Data["image"])
}

func TestLogin_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/market/login", r.URL.Path)

		body, _ := io.ReadAll(r.Body)
		var reqBody map[string]interface{}
		json.Unmarshal(body, &reqBody)

		assert.Equal(t, "admin", reqBody["username"])
		assert.Equal(t, "pass123", reqBody["password"])
		assert.Equal(t, float64(5), reqBody["marketId"])

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"code": 200,
			"desc": "success",
			"data": map[string]interface{}{
				"userName": "admin",
				"token":    "session-token-xyz",
			},
		})
	}))
	defer server.Close()

	client := NewClient(server.URL, server.URL, nil, false, "test-key", "test-secret")
	resp, err := client.Login("admin", "pass123", "uuid-1", "captcha", 5)

	require.NoError(t, err)
	assert.Equal(t, 0, resp.ResultCode)
	assert.Equal(t, "admin", resp.Data["username"])
	assert.Equal(t, "session-token-xyz", resp.Data["cookies"])
}

func TestLogin_Failure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"code": 0,
			"desc": "invalid credentials",
		})
	}))
	defer server.Close()

	client := NewClient(server.URL, server.URL, nil, false, "test-key", "test-secret")
	resp, err := client.Login("admin", "wrong", "uuid", "code", 1)

	require.NoError(t, err)
	assert.Equal(t, 99, resp.ResultCode)
	assert.Equal(t, "invalid credentials", resp.Desc)
}

func TestGetTerminalList(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/v1/tps/terminal/list", r.URL.Path)

		body, _ := io.ReadAll(r.Body)
		var reqBody map[string]interface{}
		json.Unmarshal(body, &reqBody)

		assert.Equal(t, float64(1), reqBody["page"])
		assert.Equal(t, float64(10), reqBody["size"])
		assert.Equal(t, "test-key", reqBody["accessKey"])
		assert.NotEmpty(t, reqBody["timestamp"])
		assert.NotEmpty(t, reqBody["signature"])

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"code": "200",
			"desc": "success",
			"data": map[string]interface{}{
				"pages": 5,
				"list": []interface{}{
					map[string]interface{}{
						"deviceId":    "DEV001",
						"sn":          "SN001",
						"alertStatus": 1,
					},
				},
			},
		})
	}))
	defer server.Close()

	client := NewClient(server.URL, server.URL, nil, false, "test-key", "test-secret")
	resp, err := client.GetTerminalList("", 1)

	require.NoError(t, err)
	assert.Equal(t, 0, resp.ResultCode)
	assert.Equal(t, float64(5), resp.Data["totalPage"])

	termList, ok := resp.Data["terminalList"].([]interface{})
	assert.True(t, ok)
	assert.Len(t, termList, 1)

	first, _ := termList[0].(map[string]interface{})
	assert.Equal(t, float64(1), first["status"])
}

func TestCheckToken(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/market/common/checkToken", r.URL.Path)
		assert.Equal(t, "valid-session", r.Header.Get("Authorization"))

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"code": 200,
			"desc": "valid",
		})
	}))
	defer server.Close()

	client := NewClient(server.URL, server.URL, nil, false, "test-key", "test-secret")
	resp, err := client.CheckToken("valid-session")

	require.NoError(t, err)
	assert.Equal(t, 0, resp.ResultCode)
}

func TestDeleteMerchant(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/v1/tps/merchant/delete", r.URL.Path)

		body, _ := io.ReadAll(r.Body)
		var reqBody map[string]interface{}
		json.Unmarshal(body, &reqBody)
		assert.Equal(t, "42", reqBody["merchantId"])

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"code": "200",
			"desc": "deleted",
		})
	}))
	defer server.Close()

	client := NewClient(server.URL, server.URL, nil, false, "test-key", "test-secret")
	resp, err := client.DeleteMerchant("", 42)

	require.NoError(t, err)
	assert.Equal(t, 0, resp.ResultCode)
}

func TestGetCountryList(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "/v1/tps/common/country/selector", r.URL.Path)

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"code": "200",
			"data": []interface{}{
				map[string]interface{}{"id": "1", "label": "United States"},
				map[string]interface{}{"id": "2", "label": "Canada"},
			},
		})
	}))
	defer server.Close()

	client := NewClient(server.URL, server.URL, nil, false, "test-key", "test-secret")
	resp, err := client.GetCountryList("")

	require.NoError(t, err)
	assert.Equal(t, 0, resp.ResultCode)

	countries, ok := resp.Data["countries"].([]interface{})
	assert.True(t, ok)
	assert.Len(t, countries, 2)

	first, _ := countries[0].(map[string]interface{})
	assert.Equal(t, 1, first["id"])
	assert.Equal(t, "United States", first["name"])
}

func TestGetVendorList(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "/v1/tps/common/vendor/selector", r.URL.Path)

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"code": "200",
			"data": []interface{}{
				map[string]interface{}{"id": "V1", "label": "Verifone"},
			},
		})
	}))
	defer server.Close()

	client := NewClient(server.URL, server.URL, nil, false, "test-key", "test-secret")
	resp, err := client.GetVendorList("")

	require.NoError(t, err)
	assert.Equal(t, 0, resp.ResultCode)

	vendors, ok := resp.Data["vendors"].([]interface{})
	assert.True(t, ok)
	assert.Len(t, vendors, 1)

	first, _ := vendors[0].(map[string]interface{})
	assert.Equal(t, "Verifone", first["name"])
}

func TestGetModelList(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "/v1/tps/common/model/selector", r.URL.Path)

		body, _ := io.ReadAll(r.Body)
		var reqBody map[string]interface{}
		json.Unmarshal(body, &reqBody)
		assert.Equal(t, "V1", reqBody["vendor"])

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"code": "200",
			"data": []interface{}{
				map[string]interface{}{"id": "M1", "label": "X990"},
			},
		})
	}))
	defer server.Close()

	client := NewClient(server.URL, server.URL, nil, false, "test-key", "test-secret")
	resp, err := client.GetModelList("", "V1")

	require.NoError(t, err)
	assert.Equal(t, 0, resp.ResultCode)

	models, ok := resp.Data["models"].([]interface{})
	assert.True(t, ok)
	assert.Len(t, models, 1)

	first, _ := models[0].(map[string]interface{})
	assert.Equal(t, "X990", first["name"])
}

func TestDeleteGroup(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/v1/tps/group/delete", r.URL.Path)

		body, _ := io.ReadAll(r.Body)
		var reqBody map[string]interface{}
		json.Unmarshal(body, &reqBody)
		assert.Equal(t, "7", reqBody["groupId"])

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"code": "200",
			"desc": "deleted",
		})
	}))
	defer server.Close()

	client := NewClient(server.URL, server.URL, nil, false, "test-key", "test-secret")
	resp, err := client.DeleteGroup("", 7)

	require.NoError(t, err)
	assert.Equal(t, 0, resp.ResultCode)
}

func TestGetTimeZoneList(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "/v1/tps/common/timeZone/selector", r.URL.Path)

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"code": "200",
			"data": []interface{}{
				map[string]interface{}{"id": "tz1", "label": "UTC+8"},
			},
		})
	}))
	defer server.Close()

	client := NewClient(server.URL, server.URL, nil, false, "test-key", "test-secret")
	resp, err := client.GetTimeZoneList("")

	require.NoError(t, err)
	assert.Equal(t, 0, resp.ResultCode)

	tzList, ok := resp.Data["timeZones"].([]interface{})
	assert.True(t, ok)
	assert.Len(t, tzList, 1)
	first, _ := tzList[0].(map[string]interface{})
	assert.Equal(t, "UTC+8", first["name"])
}

func TestGetMerchantList(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/market/manage/merchant/selector", r.URL.Path)

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"code": 200,
			"data": []interface{}{
				map[string]interface{}{"id": "10", "label": "Merchant A"},
				map[string]interface{}{"id": "20", "label": "Merchant B"},
			},
		})
	}))
	defer server.Close()

	client := NewClient(server.URL, server.URL, nil, false, "test-key", "test-secret")
	resp, err := client.GetMerchantList("session")

	require.NoError(t, err)
	assert.Equal(t, 0, resp.ResultCode)

	merchants, ok := resp.Data["merchants"].([]interface{})
	assert.True(t, ok)
	assert.Len(t, merchants, 2)

	first, _ := merchants[0].(map[string]interface{})
	assert.Equal(t, 10, first["id"])
	assert.Equal(t, "Merchant A", first["name"])
}

func TestGetGroupList(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/market/manage/group/selector/normal", r.URL.Path)

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"code": 200,
			"data": []interface{}{
				map[string]interface{}{"id": "3", "label": "Group X"},
			},
		})
	}))
	defer server.Close()

	client := NewClient(server.URL, server.URL, nil, false, "test-key", "test-secret")
	resp, err := client.GetGroupList("session")

	require.NoError(t, err)
	assert.Equal(t, 0, resp.ResultCode)

	groups, ok := resp.Data["groups"].([]interface{})
	assert.True(t, ok)
	assert.Len(t, groups, 1)

	first, _ := groups[0].(map[string]interface{})
	assert.Equal(t, 3, first["id"])
	assert.Equal(t, "Group X", first["name"])
}

func TestDoPost_InvalidJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.Write([]byte("not json at all"))
	}))
	defer server.Close()

	client := NewClient(server.URL, server.URL, nil, false, "test-key", "test-secret")
	_, err := client.doPost("session", "/test", nil)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unmarshal response")
}

func TestDoGet_InvalidJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("{invalid"))
	}))
	defer server.Close()

	client := NewClient(server.URL, server.URL, nil, false, "test-key", "test-secret")
	_, err := client.doGet("session", "/test", nil)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unmarshal response")
}

func TestAddMerchant(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/v1/tps/merchant/add", r.URL.Path)

		body, _ := io.ReadAll(r.Body)
		var reqBody map[string]interface{}
		json.Unmarshal(body, &reqBody)

		assert.Equal(t, "Test Merchant", reqBody["merchantName"])
		assert.Equal(t, "test@example.com", reqBody["email"])

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"code": "200",
			"desc": "created",
		})
	}))
	defer server.Close()

	client := NewClient(server.URL, server.URL, nil, false, "test-key", "test-secret")
	resp, err := client.AddMerchant("", MerchantData{
		MerchantName: "Test Merchant",
		Email:        "test@example.com",
		Address:      "123 Main St",
		CountryId:    "1",
	})

	require.NoError(t, err)
	assert.Equal(t, 0, resp.ResultCode)
}

func TestEditMerchant(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/v1/tps/merchant/update", r.URL.Path)

		body, _ := io.ReadAll(r.Body)
		var reqBody map[string]interface{}
		json.Unmarshal(body, &reqBody)

		assert.Equal(t, "42", reqBody["id"]) // Existing merchant.
		assert.Equal(t, "Updated Merchant", reqBody["merchantName"])

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"code": "200",
			"desc": "updated",
		})
	}))
	defer server.Close()

	client := NewClient(server.URL, server.URL, nil, false, "test-key", "test-secret")
	resp, err := client.EditMerchant("", MerchantData{
		ID:           "42",
		MerchantName: "Updated Merchant",
	})

	require.NoError(t, err)
	assert.Equal(t, 0, resp.ResultCode)
}

func TestReplaceTerminal(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/market/manage/terminal/replace", r.URL.Path)

		body, _ := io.ReadAll(r.Body)
		var reqBody map[string]interface{}
		json.Unmarshal(body, &reqBody)

		assert.Equal(t, "OLD-SN", reqBody["oldSn"])
		assert.Equal(t, "NEW-SN", reqBody["newSn"])

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"code": 200,
			"desc": "replaced",
		})
	}))
	defer server.Close()

	client := NewClient(server.URL, server.URL, nil, false, "test-key", "test-secret")
	resp, err := client.ReplaceTerminal("session", "OLD-SN", "NEW-SN")

	require.NoError(t, err)
	assert.Equal(t, 0, resp.ResultCode)
}
