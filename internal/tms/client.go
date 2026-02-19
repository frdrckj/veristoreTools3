package tms

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"crypto/tls"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"gorm.io/gorm"
)

// ---------------------------------------------------------------------------
// Constants
// ---------------------------------------------------------------------------

const (
	headerAccept      = "application/json, text/plain, */*"
	headerContentType = "application/json;charset=UTF-8"

	// tokenRenewalDesc is the token-renewal indicator returned by the TMS API.
	// It is a Chinese string meaning "token renewal".
	tokenRenewalDesc = "toke更新"
)

// ---------------------------------------------------------------------------
// Response types
// ---------------------------------------------------------------------------

// TMSResponse is the unified response returned by every public Client method.
type TMSResponse struct {
	ResultCode int                    `json:"resultCode"`
	Desc       string                 `json:"desc,omitempty"`
	Data       map[string]interface{} `json:"data,omitempty"`
	RawData    interface{}            `json:"rawData,omitempty"`
}

// MerchantData holds the fields for creating or editing a merchant.
type MerchantData struct {
	ID           string `json:"id"`
	MerchantName string `json:"merchantName"`
	Address      string `json:"address"`
	PostCode     string `json:"postCode"`
	TimeZone     string `json:"timeZone"`
	Contact      string `json:"contact"`
	Email        string `json:"email"`
	CellPhone    string `json:"cellPhone"`
	TelePhone    string `json:"telePhone"`
	CountryId    string `json:"countryId"`
	StateId      string `json:"stateId"`
	CityId       string `json:"cityId"`
	DistrictId   string `json:"districtId"`
}

// ---------------------------------------------------------------------------
// Client
// ---------------------------------------------------------------------------

// Client communicates with the TMS (Terminal Management System) API.
// It replaces PHP's TmsHelper class from veristoreTools2.
type Client struct {
	baseURL      string // Old session-based API (e.g. https://app.veristore.net)
	apiBaseURL   string // New signed API (e.g. https://tps.veristore.net)
	accessKey    string
	accessSecret string
	httpClient   *http.Client
	db           *gorm.DB
}

// NewClient creates a new TMS API client.
//
// baseURL is the root URL of the TMS API (e.g. "https://tms.example.com").
// db is a GORM database handle used for token-renewal persistence.
// skipTLSVerify controls whether TLS certificate verification is skipped
// (matches v2 CURLOPT_SSL_VERIFYPEER: false when true).
// accessKey and accessSecret are credentials for the new HMAC-SHA256 signed API.
func NewClient(baseURL, apiBaseURL string, db *gorm.DB, skipTLSVerify bool, accessKey, accessSecret string) *Client {
	transport := &http.Transport{}
	if skipTLSVerify {
		transport.TLSClientConfig = &tls.Config{InsecureSkipVerify: true} //nolint:gosec // configurable per deployment
	}
	return &Client{
		baseURL:      strings.TrimRight(baseURL, "/"),
		apiBaseURL:   strings.TrimRight(apiBaseURL, "/"),
		accessKey:    accessKey,
		accessSecret: accessSecret,
		httpClient: &http.Client{
			Timeout:   30 * time.Second,
			Transport: transport,
		},
		db: db,
	}
}

// ---------------------------------------------------------------------------
// Internal helpers (unexported)
// ---------------------------------------------------------------------------

// mapResponseCode translates TMS API response codes to our internal result
// codes: API 200 -> 0 (success), API 0 -> 99 (generic error), others pass
// through unchanged.
func mapResponseCode(code int) int {
	switch code {
	case 200:
		return 0
	case 0:
		return 99
	default:
		return code
	}
}

// renewToken checks if the API response indicates a token renewal. If so, it
// persists the new token in the database and returns (newToken, true).
// Otherwise it returns ("", false).
func (c *Client) renewToken(session string, response map[string]interface{}) (string, bool) {
	if response == nil {
		return "", false
	}

	code, _ := toInt(response["code"])
	desc, _ := response["desc"].(string)

	if code == 200 && desc == tokenRenewalDesc {
		newToken, _ := response["data"].(string)
		if newToken == "" {
			return "", false
		}

		// Try to update TmsLogin table first.
		if c.db != nil {
			result := c.db.Model(&TmsLogin{}).
				Where("tms_login_enable = ? AND tms_login_session = ?", "1", session).
				Update("tms_login_session", newToken)

			if result.RowsAffected == 0 {
				// Fall back to User table if no TmsLogin row matched.
				c.db.Table("user").
					Where("tms_session = ?", session).
					Update("tms_session", newToken)
			}
		}

		return newToken, true
	}

	return "", false
}

// doPost sends an authenticated POST request with a JSON body to the TMS API.
// It handles token renewal transparently.
func (c *Client) doPost(session, path string, body interface{}) (map[string]interface{}, error) {
	fullURL := c.baseURL + path

	jsonBody, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("tms: marshal post body: %w", err)
	}

	req, err := http.NewRequest(http.MethodPost, fullURL, bytes.NewReader(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("tms: new post request: %w", err)
	}

	req.Header.Set("Accept", headerAccept)
	req.Header.Set("Content-Type", headerContentType)
	if session != "" {
		req.Header.Set("Authorization", session)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("tms: post %s: %w", path, err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("tms: read response body: %w", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("tms: unmarshal response: %w", err)
	}

	// Handle token renewal: if detected, retry with new token.
	if newToken, ok := c.renewToken(session, result); ok {
		retryReq, err := http.NewRequest(http.MethodPost, fullURL, bytes.NewReader(jsonBody))
		if err != nil {
			return nil, fmt.Errorf("tms: new retry request: %w", err)
		}
		retryReq.Header.Set("Accept", headerAccept)
		retryReq.Header.Set("Content-Type", headerContentType)
		retryReq.Header.Set("Authorization", newToken)

		retryResp, err := c.httpClient.Do(retryReq)
		if err != nil {
			return nil, fmt.Errorf("tms: retry post %s: %w", path, err)
		}
		defer retryResp.Body.Close()

		retryBody, err := io.ReadAll(retryResp.Body)
		if err != nil {
			return nil, fmt.Errorf("tms: read retry response: %w", err)
		}

		var retryResult map[string]interface{}
		if err := json.Unmarshal(retryBody, &retryResult); err != nil {
			return nil, fmt.Errorf("tms: unmarshal retry response: %w", err)
		}
		return retryResult, nil
	}

	return result, nil
}

// doGet sends an authenticated GET request with query parameters to the TMS
// API. It handles token renewal transparently.
func (c *Client) doGet(session, path string, params url.Values) (map[string]interface{}, error) {
	fullURL := c.baseURL + path
	if len(params) > 0 {
		fullURL += "?" + params.Encode()
	}

	req, err := http.NewRequest(http.MethodGet, fullURL, nil)
	if err != nil {
		return nil, fmt.Errorf("tms: new get request: %w", err)
	}

	req.Header.Set("Accept", headerAccept)
	req.Header.Set("Content-Type", headerContentType)
	if session != "" {
		req.Header.Set("Authorization", session)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("tms: get %s: %w", path, err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("tms: read response body: %w", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("tms: unmarshal response: %w", err)
	}

	// Handle token renewal: if detected, retry with new token.
	if newToken, ok := c.renewToken(session, result); ok {
		retryReq, err := http.NewRequest(http.MethodGet, fullURL, nil)
		if err != nil {
			return nil, fmt.Errorf("tms: new retry request: %w", err)
		}
		retryReq.Header.Set("Accept", headerAccept)
		retryReq.Header.Set("Content-Type", headerContentType)
		retryReq.Header.Set("Authorization", newToken)

		retryResp, err := c.httpClient.Do(retryReq)
		if err != nil {
			return nil, fmt.Errorf("tms: retry get %s: %w", path, err)
		}
		defer retryResp.Body.Close()

		retryBody, err := io.ReadAll(retryResp.Body)
		if err != nil {
			return nil, fmt.Errorf("tms: read retry response: %w", err)
		}

		var retryResult map[string]interface{}
		if err := json.Unmarshal(retryBody, &retryResult); err != nil {
			return nil, fmt.Errorf("tms: unmarshal retry response: %w", err)
		}
		return retryResult, nil
	}

	return result, nil
}

// generateSignature creates an HMAC-SHA256 signature for the new TPS API.
// It filters out empty values and the "signature" key, sorts remaining keys
// by ASCII ascending, builds a key=value& string, and computes the HMAC.
func (c *Client) generateSignature(params map[string]interface{}) string {
	// Collect non-empty, non-signature keys.
	keys := make([]string, 0, len(params))
	for k, v := range params {
		if k == "signature" {
			continue
		}
		s := fmt.Sprintf("%v", v)
		if s == "" || s == "<nil>" {
			continue
		}
		keys = append(keys, k)
	}
	sort.Strings(keys)

	// Build the data string: key1=value1&key2=value2&...
	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		parts = append(parts, k+"="+fmt.Sprintf("%v", params[k]))
	}
	data := strings.Join(parts, "&")

	// Compute HMAC-SHA256.
	mac := hmac.New(sha256.New, []byte(c.accessSecret))
	mac.Write([]byte(data))
	return strings.ToUpper(hex.EncodeToString(mac.Sum(nil)))
}

// doSignedPost sends a POST request to the new TPS API using HMAC-SHA256
// signature authentication. It auto-injects accessKey and timestamp, generates
// the signature, and sends the request with a JSON body (no Authorization header).
// The new API returns code as string "200" instead of int 200.
func (c *Client) doSignedPost(path string, params map[string]interface{}) (map[string]interface{}, error) {
	if params == nil {
		params = map[string]interface{}{}
	}

	// Inject auth fields.
	params["accessKey"] = c.accessKey
	params["timestamp"] = fmt.Sprintf("%d", time.Now().UnixMilli())

	// Generate and inject signature.
	params["signature"] = c.generateSignature(params)

	fullURL := c.apiBaseURL + path

	jsonBody, err := json.Marshal(params)
	if err != nil {
		return nil, fmt.Errorf("tms: marshal signed post body: %w", err)
	}

	req, err := http.NewRequest(http.MethodPost, fullURL, bytes.NewReader(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("tms: new signed post request: %w", err)
	}

	req.Header.Set("Accept", headerAccept)
	req.Header.Set("Content-Type", headerContentType)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("tms: signed post %s: %w", path, err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("tms: read signed response body: %w", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(respBody, &result); err != nil {
		// Include status code and body snippet for debugging.
		snippet := string(respBody)
		if len(snippet) > 200 {
			snippet = snippet[:200]
		}
		return nil, fmt.Errorf("tms: signed post %s returned HTTP %d, body: %s", path, resp.StatusCode, snippet)
	}

	return result, nil
}

// getIdFromSN resolves a device ID (SN) to an internal terminal ID via the
// terminal/page API (old session-based API).
func (c *Client) getIdFromSN(session, deviceId string) (int, error) {
	body := map[string]interface{}{
		"page":   1,
		"search": "",
		"size":   10,
		"deviceId": map[string]interface{}{
			"type":  "=",
			"value": deviceId,
		},
	}

	result, err := c.doPost(session, "/market/manage/terminal/page", body)
	if err != nil {
		return 0, fmt.Errorf("tms: getIdFromSN: %w", err)
	}

	code, _ := toInt(result["code"])
	if code == 200 {
		data, _ := result["data"].(map[string]interface{})
		if data != nil {
			list, _ := data["list"].([]interface{})
			if len(list) > 0 {
				first, _ := list[0].(map[string]interface{})
				if first != nil {
					id, _ := toInt(first["id"])
					return id, nil
				}
			}
		}
	}

	return 0, fmt.Errorf("tms: could not resolve SN %q to terminal ID", deviceId)
}

// getTerminalIdFromSN resolves a serial number or device ID (CSI) to the
// internal terminal ID using the new signed terminal list API.
func (c *Client) getTerminalIdFromSN(serialNum string) (string, error) {
	result, err := c.doSignedPost("/v1/tps/terminal/list", map[string]interface{}{
		"page":   1,
		"size":   10,
		"search": serialNum,
	})
	if err != nil {
		return "", fmt.Errorf("tms: getTerminalIdFromSN: %w", err)
	}

	code, _ := toInt(result["code"])
	if code == 200 {
		data, _ := result["data"].(map[string]interface{})
		if data != nil {
			list, _ := data["list"].([]interface{})
			for _, item := range list {
				m, _ := item.(map[string]interface{})
				if m == nil {
					continue
				}
				// Match on SN or deviceId (CSI).
				if toString(m["sn"]) == serialNum || toString(m["deviceId"]) == serialNum {
					return toString(m["id"]), nil
				}
			}
			// If no exact match but only one result, use it.
			if len(list) == 1 {
				m, _ := list[0].(map[string]interface{})
				if m != nil {
					return toString(m["id"]), nil
				}
			}
		}
	}

	return "", fmt.Errorf("tms: could not resolve SN %q to terminal ID", serialNum)
}

// getOperationMark retrieves the current operation mark from the TMS API.
func (c *Client) getOperationMark(session string) (string, error) {
	result, err := c.doPost(session, "/market/common/operationMark", nil)
	if err != nil {
		return "", fmt.Errorf("tms: getOperationMark: %w", err)
	}

	code, _ := toInt(result["code"])
	if code == 200 {
		data, _ := result["data"].(string)
		return data, nil
	}

	return "", fmt.Errorf("tms: getOperationMark returned code %d", code)
}

// ---------------------------------------------------------------------------
// Authentication methods
// ---------------------------------------------------------------------------

// GetResellerList retrieves the list of resellers/markets for a given username.
func (c *Client) GetResellerList(username string) (*TMSResponse, error) {
	params := url.Values{}
	params.Set("resellerId", "1")
	params.Set("username", username)

	result, err := c.doGet("", "/market/common/getMarketsByUser", params)
	if err != nil {
		return nil, err
	}

	code, _ := toInt(result["code"])
	rc := mapResponseCode(code)

	resp := &TMSResponse{
		ResultCode: rc,
		Desc:       toString(result["desc"]),
	}

	if rc == 0 {
		if data, ok := result["data"].([]interface{}); ok {
			resp.RawData = data
		}
	}

	return resp, nil
}

// GetVerifyCode retrieves a CAPTCHA image and its token from the TMS API.
func (c *Client) GetVerifyCode() (*TMSResponse, error) {
	result, err := c.doGet("", "/market/common/getCaptcha", nil)
	if err != nil {
		return nil, err
	}

	code, _ := toInt(result["code"])
	rc := mapResponseCode(code)

	resp := &TMSResponse{
		ResultCode: rc,
		Desc:       toString(result["desc"]),
	}

	if rc == 0 {
		data, _ := result["data"].(map[string]interface{})
		if data != nil {
			resp.Data = map[string]interface{}{
				"token": toString(data["uuid"]),
				"image": "data:image/png;base64," + toString(data["image"]),
			}
		}
	}

	return resp, nil
}

// Login authenticates a user against the TMS API.
func (c *Client) Login(username, password, token, code string, resellerId int) (*TMSResponse, error) {
	body := map[string]interface{}{
		"username": username,
		"password": password,
		"uuid":     token,
		"captcha":  code,
		"marketId": resellerId,
	}

	result, err := c.doPost("", "/market/login", body)
	if err != nil {
		return nil, err
	}

	apiCode, _ := toInt(result["code"])
	rc := mapResponseCode(apiCode)

	resp := &TMSResponse{
		ResultCode: rc,
		Desc:       toString(result["desc"]),
	}

	if rc == 0 {
		data, _ := result["data"].(map[string]interface{})
		if data != nil {
			resp.Data = map[string]interface{}{
				"username": toString(data["userName"]),
				"cookies":  toString(data["token"]),
			}
		}
	}

	return resp, nil
}

// CheckToken verifies whether a session token is still valid.
func (c *Client) CheckToken(session string) (*TMSResponse, error) {
	result, err := c.doPost(session, "/market/common/checkToken", nil)
	if err != nil {
		return nil, err
	}

	apiCode, _ := toInt(result["code"])

	resp := &TMSResponse{
		ResultCode: mapResponseCode(apiCode),
		Desc:       toString(result["desc"]),
	}

	if apiCode == 200 {
		resp.RawData = result
	}

	return resp, nil
}

// ---------------------------------------------------------------------------
// Dashboard
// ---------------------------------------------------------------------------

// GetDashboard retrieves the dashboard summary (top counts and new app list).
func (c *Client) GetDashboard(session string) (*TMSResponse, error) {
	// First call: topSum
	topResult, err := c.doPost(session, "/market/manage/index/topSum", nil)
	if err != nil {
		return nil, err
	}

	topCode, _ := toInt(topResult["code"])
	if topCode != 200 {
		return &TMSResponse{
			ResultCode: mapResponseCode(topCode),
			Desc:       toString(topResult["desc"]),
		}, nil
	}

	topData, _ := topResult["data"].(map[string]interface{})
	if topData == nil {
		return &TMSResponse{
			ResultCode: mapResponseCode(topCode),
			Desc:       toString(topResult["desc"]),
		}, nil
	}

	dashboard := map[string]interface{}{
		"terminalActivedNum": toIntDefault(topData["activeCount"], 0),
		"terminalTotalNum":   toIntDefault(topData["termCount"], 0),
		"merchTotalNum":      toIntDefault(topData["merchCount"], 0),
		"appTotalNum":        toIntDefault(topData["appCount"], 0),
		"appDownloadsNum":    toIntDefault(topData["appDownloadCount"], 0),
		"downloadsTask":      toIntDefault(topData["pushCount"], 0),
	}

	// Second call: newAppList
	appResult, err := c.doPost(session, "/market/manage/index/newAppList", nil)
	if err != nil {
		return nil, err
	}

	appCode, _ := toInt(appResult["code"])
	if appCode == 200 {
		appList := []interface{}{}
		if dataList, ok := appResult["data"].([]interface{}); ok {
			for _, item := range dataList {
				appItem, _ := item.(map[string]interface{})
				if appItem != nil {
					appList = append(appList, map[string]interface{}{
						"logo":    toString(appItem["icon"]),
						"name":    toString(appItem["appName"]),
						"version": toString(appItem["version"]),
					})
				}
			}
		}
		dashboard["newAppList"] = appList
	} else {
		return &TMSResponse{
			ResultCode: mapResponseCode(appCode),
			Desc:       toString(appResult["desc"]),
		}, nil
	}

	return &TMSResponse{
		ResultCode: 0,
		Data:       dashboard,
	}, nil
}

// ---------------------------------------------------------------------------
// Terminal Management
// ---------------------------------------------------------------------------

// GetTerminalList retrieves a paginated list of terminals.
func (c *Client) GetTerminalList(session string, pageNum int) (*TMSResponse, error) {
	params := map[string]interface{}{
		"page": pageNum,
		"size": 10,
	}

	result, err := c.doSignedPost("/v1/tps/terminal/list", params)
	if err != nil {
		return nil, err
	}

	code, _ := toInt(result["code"])
	rc := mapResponseCode(code)

	resp := &TMSResponse{
		ResultCode: rc,
		Desc:       toString(result["desc"]),
	}

	if rc == 0 {
		data, _ := result["data"].(map[string]interface{})
		if data != nil {
			list, _ := data["list"].([]interface{})
			for i, item := range list {
				if m, ok := item.(map[string]interface{}); ok {
					m["status"] = m["alertStatus"]
					list[i] = m
				}
			}
			resp.Data = map[string]interface{}{
				"totalPage":    data["pages"],
				"terminalList": list,
			}
		}
	}

	return resp, nil
}

// GetTerminalListSearch searches terminals with filters based on queryType:
//
//	0 = SN, 1 = merchantName, 2 = groupName, 3 = TID param,
//	4 = deviceId, 5 = MID param
func (c *Client) GetTerminalListSearch(session string, pageNum int, search string, queryType int) (*TMSResponse, error) {
	// New API uses a simple "search" string parameter.
	params := map[string]interface{}{
		"page":   pageNum,
		"search": search,
		"size":   10,
	}

	result, err := c.doSignedPost("/v1/tps/terminal/list", params)
	if err != nil {
		return nil, err
	}

	code, _ := toInt(result["code"])
	rc := mapResponseCode(code)

	resp := &TMSResponse{
		ResultCode: rc,
		Desc:       toString(result["desc"]),
	}

	if rc == 0 {
		data, _ := result["data"].(map[string]interface{})
		if data != nil {
			list, _ := data["list"].([]interface{})
			for i, item := range list {
				if m, ok := item.(map[string]interface{}); ok {
					m["status"] = m["alertStatus"]
					list[i] = m
				}
			}
			resp.Data = map[string]interface{}{
				"totalPage":    data["pages"],
				"terminalList": list,
			}
		}
	}

	return resp, nil
}

// GetTerminalDetail retrieves detailed information about a terminal and its
// installed apps.
func (c *Client) GetTerminalDetail(session, serialNum string) (*TMSResponse, error) {
	// Resolve SN to internal terminal ID for the new API.
	terminalId, err := c.getTerminalIdFromSN(serialNum)
	if err != nil {
		return nil, err
	}

	detailResult, err := c.doSignedPost("/v1/tps/terminal/detail", map[string]interface{}{
		"terminalId": terminalId,
	})
	if err != nil {
		return nil, err
	}

	detailCode, _ := toInt(detailResult["code"])
	if detailCode != 200 {
		return &TMSResponse{
			ResultCode: mapResponseCode(detailCode),
			Desc:       toString(detailResult["desc"]),
		}, nil
	}

	data, _ := detailResult["data"].(map[string]interface{})
	if data == nil {
		return &TMSResponse{ResultCode: 99, Desc: "no data in detail response"}, nil
	}

	// Convert merchantId to int.
	if mid, ok := data["merchantId"]; ok {
		data["merchantId"], _ = toInt(mid)
	}

	// Convert groupIds to []int.
	if gids, ok := data["groupIds"].([]interface{}); ok {
		intGids := []int{}
		for _, g := range gids {
			v, _ := toInt(g)
			intGids = append(intGids, v)
		}
		data["groupId"] = intGids
	} else {
		data["groupId"] = []int{}
	}

	// Extract PN from diagnostic.
	data["pn"] = nil
	if diags, ok := data["diagnostic"].([]interface{}); ok {
		for _, d := range diags {
			if dm, ok := d.(map[string]interface{}); ok {
				if toString(dm["attribute"]) == "PN" {
					data["pn"] = toString(dm["value"])
					break
				}
			}
		}
	}

	// Second call: terminal app list.
	appResult, err := c.doSignedPost("/v2/tps/terminalApp/list", map[string]interface{}{
		"terminalId": terminalId,
	})
	if err != nil {
		return nil, err
	}

	appCode, _ := toInt(appResult["code"])
	if appCode == 200 {
		apps := []interface{}{}
		if appData, ok := appResult["data"].([]interface{}); ok {
			for _, a := range appData {
				am, _ := a.(map[string]interface{})
				if am == nil {
					continue
				}
				pkgName := toString(am["packageName"])
				if items, ok := am["itemList"].([]interface{}); ok {
					for _, item := range items {
						im, _ := item.(map[string]interface{})
						if im != nil {
							apps = append(apps, map[string]interface{}{
								"packageName": pkgName,
								"name":        toString(im["appName"]),
								"version":     toString(im["appVersion"]),
								"id":          toIntDefault(im["appId"], 0),
							})
						}
					}
				}
			}
		}
		data["terminalShowApps"] = apps
		data["resultCode"] = 0
	} else {
		data["resultCode"] = appCode
		data["desc"] = toString(appResult["desc"])
	}

	return &TMSResponse{
		ResultCode: 0,
		Data:       data,
	}, nil
}

// GetTerminalParameter retrieves terminal app parameters for a specific tab
// using the old session-based API (/market/manage/terminalAppParameter/view).
// tabName is the second segment of tparam_field (e.g. "MERCHANT" from "TP-MERCHANT-FIELD").
func (c *Client) GetTerminalParameter(session, serialNum, appId, tabName string) (*TMSResponse, error) {
	terminalId, err := c.getIdFromSN(session, serialNum)
	if err != nil {
		return nil, err
	}
	operationMark, err := c.getOperationMark(session)
	if err != nil {
		return nil, err
	}

	result, err := c.doPost(session, "/market/manage/terminalAppParameter/view", map[string]interface{}{
		"appId":         appId,
		"operationMark": operationMark,
		"tabName":       tabName,
		"terminalId":    terminalId,
	})
	if err != nil {
		return nil, err
	}

	code, _ := toInt(result["code"])
	if code != 200 {
		return &TMSResponse{
			ResultCode: mapResponseCode(code),
			Desc:       toString(result["desc"]),
		}, nil
	}

	data, _ := result["data"].(map[string]interface{})
	paraList := []interface{}{}
	if data != nil {
		cardValues, _ := data["cardValueList"].([]interface{})
		cardTabs, _ := data["cardTabList"].([]interface{})
		for _, cv := range cardValues {
			row, _ := cv.(map[string]interface{})
			if row == nil {
				continue
			}
			number := toString(row["NUMBER"])
			for _, ct := range cardTabs {
				field, _ := ct.(map[string]interface{})
				if field == nil {
					continue
				}
				key := toString(field["key"])
				paraList = append(paraList, map[string]interface{}{
					"dataName":    key + "-" + number,
					"viewName":    tabName,
					"value":       toString(row[key]),
					"description": toString(field["description"]),
				})
			}
		}
	}

	return &TMSResponse{
		ResultCode: 0,
		Data:       map[string]interface{}{"paraList": paraList},
	}, nil
}

// AddTerminal registers a new terminal in the TMS.
func (c *Client) AddTerminal(session, deviceId, vendor, model, merchantId string, groupIds []string, sn string, moveConf int) (*TMSResponse, error) {
	if groupIds == nil {
		groupIds = []string{}
	}

	did := deviceId
	if did == "" {
		did = " "
	}
	snVal := sn
	if snVal == "" {
		snVal = " "
	}

	params := map[string]interface{}{
		"vendor":     vendor,
		"deviceId":   did,
		"model":      model,
		"merchantId": merchantId,
		"groupIds":   groupIds,
		"sn":         snVal,
		"iotFlag":    moveConf,
	}

	result, err := c.doSignedPost("/v1/tps/terminal/add", params)
	if err != nil {
		return nil, err
	}

	code, _ := toInt(result["code"])
	rc := mapResponseCode(code)

	return &TMSResponse{
		ResultCode: rc,
		Desc:       toString(result["desc"]),
		RawData:    result,
	}, nil
}

// UpdateDeviceId updates a terminal's device ID, model, merchant, and groups.
func (c *Client) UpdateDeviceId(session, serialNum, model string, merchantId int, groupList []int, deviceId string) (*TMSResponse, error) {
	// Resolve SN to internal terminal ID.
	terminalId, err := c.getTerminalIdFromSN(serialNum)
	if err != nil {
		return nil, err
	}

	params := map[string]interface{}{
		"id":         terminalId,
		"deviceId":   deviceId,
		"model":      model,
		"merchantId": merchantId,
		"groupIds":   groupList,
		"status":     0,
	}

	updateResult, err := c.doSignedPost("/v1/tps/terminal/update", params)
	if err != nil {
		return nil, err
	}

	updateCode, _ := toInt(updateResult["code"])
	rc := mapResponseCode(updateCode)

	return &TMSResponse{
		ResultCode: rc,
		Desc:       toString(updateResult["desc"]),
	}, nil
}

// CopyTerminal copies configuration from one terminal to another.
func (c *Client) CopyTerminal(session, sourceSn, destSn string) (*TMSResponse, error) {
	sourceId, err := c.getIdFromSN(session, sourceSn)
	if err != nil {
		return nil, err
	}

	body := map[string]interface{}{
		"newDeviceId":       destSn,
		"newSn":             "",
		"oldTerminalId":     sourceId,
		"oldTerminalStatus": 0,
	}

	result, err := c.doPost(session, "/market/manage/terminal/copy", body)
	if err != nil {
		return nil, err
	}

	code, _ := toInt(result["code"])
	rc := mapResponseCode(code)
	// Map code 800 -> resultCode 1 (matching PHP logic).
	if code == 800 {
		rc = 1
	}

	return &TMSResponse{
		ResultCode: rc,
		Desc:       toString(result["desc"]),
		RawData:    result,
	}, nil
}

// DeleteTerminal removes a terminal by its device ID (SN).
func (c *Client) DeleteTerminal(session, serialNum string) (*TMSResponse, error) {
	// Resolve SN to internal terminal ID.
	terminalId, err := c.getTerminalIdFromSN(serialNum)
	if err != nil {
		return nil, err
	}

	result, err := c.doSignedPost("/v1/tps/terminal/delete", map[string]interface{}{
		"id": terminalId,
	})
	if err != nil {
		return nil, err
	}

	code, _ := toInt(result["code"])
	rc := mapResponseCode(code)

	return &TMSResponse{
		ResultCode: rc,
		Desc:       toString(result["desc"]),
	}, nil
}

// ReplaceTerminal replaces one terminal's SN with another.
func (c *Client) ReplaceTerminal(session, oldSn, newSn string) (*TMSResponse, error) {
	body := map[string]interface{}{
		"oldSn": oldSn,
		"newSn": newSn,
	}

	result, err := c.doPost(session, "/market/manage/terminal/replace", body)
	if err != nil {
		return nil, err
	}

	code, _ := toInt(result["code"])
	rc := mapResponseCode(code)

	return &TMSResponse{
		ResultCode: rc,
		Desc:       toString(result["desc"]),
		RawData:    result,
	}, nil
}

// ---------------------------------------------------------------------------
// App & Parameter Management
// ---------------------------------------------------------------------------

// GetAppList retrieves all available apps and their versions.
func (c *Client) GetAppList(session string) (*TMSResponse, error) {
	// New API: single call returns apps with version info.
	result, err := c.doSignedPost("/v1/tps/app/list", map[string]interface{}{
		"page": 1,
		"size": 100,
	})
	if err != nil {
		return nil, err
	}

	code, _ := toInt(result["code"])
	if code != 200 {
		return &TMSResponse{
			ResultCode: mapResponseCode(code),
			Desc:       toString(result["desc"]),
		}, nil
	}

	data, _ := result["data"].(map[string]interface{})
	if data == nil {
		return &TMSResponse{ResultCode: 99, Desc: "no app list data"}, nil
	}

	appItems, _ := data["list"].([]interface{})
	allApps := []interface{}{}

	for _, appItem := range appItems {
		app, _ := appItem.(map[string]interface{})
		if app == nil {
			continue
		}
		allApps = append(allApps, map[string]interface{}{
			"id":          toIntDefault(app["id"], 0),
			"name":        toString(app["name"]),
			"version":     toString(app["version"]),
			"packageName": toString(app["packageName"]),
		})
	}

	return &TMSResponse{
		ResultCode: 0,
		Data:       map[string]interface{}{"allApps": allApps},
	}, nil
}

// GetAppListSearch retrieves the list of apps installed on a specific terminal.
func (c *Client) GetAppListSearch(session, serialNum string) (*TMSResponse, error) {
	// Resolve SN to internal terminal ID.
	terminalId, err := c.getTerminalIdFromSN(serialNum)
	if err != nil {
		return nil, err
	}

	result, err := c.doSignedPost("/v2/tps/terminalApp/list", map[string]interface{}{
		"terminalId": terminalId,
	})
	if err != nil {
		return nil, err
	}

	code, _ := toInt(result["code"])
	if code != 200 {
		return &TMSResponse{
			ResultCode: mapResponseCode(code),
			Desc:       toString(result["desc"]),
		}, nil
	}

	appList := []interface{}{}
	if data, ok := result["data"].([]interface{}); ok {
		for _, a := range data {
			am, _ := a.(map[string]interface{})
			if am == nil {
				continue
			}
			if items, ok := am["itemList"].([]interface{}); ok {
				for _, item := range items {
					im, _ := item.(map[string]interface{})
					if im != nil {
						appList = append(appList, map[string]interface{}{
							"packageName": toString(im["appId"]),
							"version":     toString(im["appVersion"]),
							"name":        toString(im["appName"]),
						})
					}
				}
			}
		}
	}

	return &TMSResponse{
		ResultCode: 0,
		Data:       map[string]interface{}{"appList": appList},
	}, nil
}

// AddParameter adds an app to a terminal (pre-add + submit).
func (c *Client) AddParameter(session, deviceId, appId string) (*TMSResponse, error) {
	serialNumId, err := c.getIdFromSN(session, deviceId)
	if err != nil {
		return nil, err
	}

	operationMark, err := c.getOperationMark(session)
	if err != nil {
		return nil, err
	}

	// Step 1: preAdd
	preAddResult, err := c.doPost(session, "/market/manage/terminalApp/preAdd", map[string]interface{}{
		"appIds":        []string{appId},
		"operationMark": operationMark,
		"terminalId":    serialNumId,
	})
	if err != nil {
		return nil, err
	}

	preAddCode, _ := toInt(preAddResult["code"])
	if preAddCode != 200 {
		return &TMSResponse{
			ResultCode: mapResponseCode(preAddCode),
			Desc:       toString(preAddResult["desc"]),
		}, nil
	}

	// Step 2: submit
	submitResult, err := c.doPost(session, "/market/manage/terminalAppParameter/submit", map[string]interface{}{
		"operationMark": operationMark,
		"terminalId":    serialNumId,
	})
	if err != nil {
		return nil, err
	}

	submitCode, _ := toInt(submitResult["code"])
	rc := mapResponseCode(submitCode)

	return &TMSResponse{
		ResultCode: rc,
		Desc:       toString(submitResult["desc"]),
	}, nil
}

// UpdateParameter updates terminal app parameters.
// New API: single call instead of the old view → preSubmit → submit loop.
func (c *Client) UpdateParameter(session, serialNum string, paraList []map[string]interface{}, appId string) (*TMSResponse, error) {
	// Resolve SN to internal terminal ID.
	terminalId, err := c.getTerminalIdFromSN(serialNum)
	if err != nil {
		return nil, err
	}

	// Convert paraList [{dataName, value}, ...] to updParamMap {key: value, ...}.
	updParamMap := map[string]string{}
	for _, p := range paraList {
		key := toString(p["dataName"])
		val := toString(p["value"])
		if key != "" {
			updParamMap[key] = val
		}
	}

	result, err := c.doSignedPost("/v2/tps/terminalAppParameter/update", map[string]interface{}{
		"terminalId":  terminalId,
		"appId":       appId,
		"updParamMap": updParamMap,
	})
	if err != nil {
		return nil, err
	}

	code, _ := toInt(result["code"])
	rc := mapResponseCode(code)

	return &TMSResponse{
		ResultCode: rc,
		Desc:       toString(result["desc"]),
	}, nil
}

// ---------------------------------------------------------------------------
// Merchant Management
// ---------------------------------------------------------------------------

// GetMerchantList retrieves the merchant selector list.
func (c *Client) GetMerchantList(session string) (*TMSResponse, error) {
	result, err := c.doPost(session, "/market/manage/merchant/selector", nil)
	if err != nil {
		return nil, err
	}

	code, _ := toInt(result["code"])
	rc := mapResponseCode(code)

	resp := &TMSResponse{
		ResultCode: rc,
		Desc:       toString(result["desc"]),
	}

	if rc == 0 {
		if data, ok := result["data"].([]interface{}); ok {
			merchants := []interface{}{}
			for _, item := range data {
				m, _ := item.(map[string]interface{})
				if m != nil {
					m["id"], _ = toInt(m["id"])
					m["name"] = toString(m["label"])
					merchants = append(merchants, m)
				}
			}
			resp.Data = map[string]interface{}{"merchants": merchants}
		}
	}

	return resp, nil
}

// GetMerchantManageList retrieves a paginated list of merchants.
func (c *Client) GetMerchantManageList(session string, pageNum int) (*TMSResponse, error) {
	result, err := c.doSignedPost("/v1/tps/merchant/list", map[string]interface{}{
		"page": pageNum,
		"size": 10,
	})
	if err != nil {
		return nil, err
	}

	code, _ := toInt(result["code"])
	rc := mapResponseCode(code)

	resp := &TMSResponse{
		ResultCode: rc,
		Desc:       toString(result["desc"]),
	}

	if rc == 0 {
		data, _ := result["data"].(map[string]interface{})
		if data != nil {
			list, _ := data["list"].([]interface{})
			for i, item := range list {
				if m, ok := item.(map[string]interface{}); ok {
					m["id"], _ = toInt(m["id"])
					list[i] = m
				}
			}
			resp.Data = map[string]interface{}{
				"totalPage":    data["pages"],
				"merchantList": list,
			}
		}
	}

	return resp, nil
}

// GetMerchantManageListSearch searches merchants by name (paginated).
func (c *Client) GetMerchantManageListSearch(session string, pageNum int, search string) (*TMSResponse, error) {
	result, err := c.doSignedPost("/v1/tps/merchant/list", map[string]interface{}{
		"page":   pageNum,
		"search": search,
		"size":   10,
	})
	if err != nil {
		return nil, err
	}

	code, _ := toInt(result["code"])
	rc := mapResponseCode(code)

	resp := &TMSResponse{
		ResultCode: rc,
		Desc:       toString(result["desc"]),
	}

	if rc == 0 {
		data, _ := result["data"].(map[string]interface{})
		if data != nil {
			list, _ := data["list"].([]interface{})
			for i, item := range list {
				if m, ok := item.(map[string]interface{}); ok {
					m["id"], _ = toInt(m["id"])
					list[i] = m
				}
			}
			resp.Data = map[string]interface{}{
				"totalPage":    data["pages"],
				"merchantList": list,
			}
		}
	}

	return resp, nil
}

// GetMerchantManageDetail retrieves detailed information about a merchant.
func (c *Client) GetMerchantManageDetail(session string, merchantId int) (*TMSResponse, error) {
	result, err := c.doSignedPost("/v1/tps/merchant/detail", map[string]interface{}{
		"merchantId": strconv.Itoa(merchantId),
	})
	if err != nil {
		return nil, err
	}

	code, _ := toInt(result["code"])
	if code != 200 {
		return &TMSResponse{
			ResultCode: mapResponseCode(code),
			Desc:       toString(result["desc"]),
		}, nil
	}

	data, _ := result["data"].(map[string]interface{})
	if data != nil {
		data["id"], _ = toInt(data["id"])
	}

	return &TMSResponse{
		ResultCode: 0,
		Data:       map[string]interface{}{"merchant": data},
	}, nil
}

// AddMerchant creates a new merchant in the TMS.
func (c *Client) AddMerchant(session string, merchant MerchantData) (*TMSResponse, error) {
	params := map[string]interface{}{
		"merchantName": merchant.MerchantName,
		"address":      merchant.Address,
		"postCode":     merchant.PostCode,
		"timeZone":     merchant.TimeZone,
		"contact":      merchant.Contact,
		"email":        merchant.Email,
		"cellPhone":    merchant.CellPhone,
		"telePhone":    merchant.TelePhone,
		"countryId":    merchant.CountryId,
		"stateId":      merchant.StateId,
		"cityId":       merchant.CityId,
		"districtId":   merchant.DistrictId,
	}

	result, err := c.doSignedPost("/v1/tps/merchant/add", params)
	if err != nil {
		return nil, err
	}

	code, _ := toInt(result["code"])
	rc := mapResponseCode(code)

	return &TMSResponse{
		ResultCode: rc,
		Desc:       toString(result["desc"]),
		RawData:    result,
	}, nil
}

// EditMerchant updates an existing merchant in the TMS.
func (c *Client) EditMerchant(session string, merchant MerchantData) (*TMSResponse, error) {
	params := map[string]interface{}{
		"id":           merchant.ID,
		"merchantName": merchant.MerchantName,
		"address":      merchant.Address,
		"postCode":     merchant.PostCode,
		"timeZone":     merchant.TimeZone,
		"contact":      merchant.Contact,
		"email":        merchant.Email,
		"cellPhone":    merchant.CellPhone,
		"telePhone":    merchant.TelePhone,
		"countryId":    merchant.CountryId,
		"stateId":      merchant.StateId,
		"cityId":       merchant.CityId,
		"districtId":   merchant.DistrictId,
	}

	result, err := c.doSignedPost("/v1/tps/merchant/update", params)
	if err != nil {
		return nil, err
	}

	code, _ := toInt(result["code"])
	rc := mapResponseCode(code)

	return &TMSResponse{
		ResultCode: rc,
		Desc:       toString(result["desc"]),
		RawData:    result,
	}, nil
}

// DeleteMerchant removes a merchant by its ID.
func (c *Client) DeleteMerchant(session string, merchantId int) (*TMSResponse, error) {
	result, err := c.doSignedPost("/v1/tps/merchant/delete", map[string]interface{}{
		"merchantId": strconv.Itoa(merchantId),
	})
	if err != nil {
		return nil, err
	}

	code, _ := toInt(result["code"])
	rc := mapResponseCode(code)

	return &TMSResponse{
		ResultCode: rc,
		Desc:       toString(result["desc"]),
	}, nil
}

// ---------------------------------------------------------------------------
// Group Management
// ---------------------------------------------------------------------------

// GetGroupList retrieves the group selector list.
func (c *Client) GetGroupList(session string) (*TMSResponse, error) {
	result, err := c.doPost(session, "/market/manage/group/selector/normal", nil)
	if err != nil {
		return nil, err
	}

	code, _ := toInt(result["code"])
	rc := mapResponseCode(code)

	resp := &TMSResponse{
		ResultCode: rc,
		Desc:       toString(result["desc"]),
	}

	if rc == 0 {
		if data, ok := result["data"].([]interface{}); ok {
			groups := []interface{}{}
			for _, item := range data {
				m, _ := item.(map[string]interface{})
				if m != nil {
					m["id"], _ = toInt(m["id"])
					m["name"] = toString(m["label"])
					groups = append(groups, m)
				}
			}
			resp.Data = map[string]interface{}{"groups": groups}
		}
	}

	return resp, nil
}

// GetGroupManageList retrieves a paginated list of groups.
func (c *Client) GetGroupManageList(session string, pageNum int) (*TMSResponse, error) {
	result, err := c.doSignedPost("/v1/tps/group/list", map[string]interface{}{
		"page": pageNum,
		"size": 10,
	})
	if err != nil {
		return nil, err
	}

	code, _ := toInt(result["code"])
	rc := mapResponseCode(code)

	resp := &TMSResponse{
		ResultCode: rc,
		Desc:       toString(result["desc"]),
	}

	if rc == 0 {
		data, _ := result["data"].(map[string]interface{})
		if data != nil {
			list, _ := data["list"].([]interface{})
			for i, item := range list {
				if m, ok := item.(map[string]interface{}); ok {
					m["id"], _ = toInt(m["id"])
					m["totalTerminals"] = m["totalTerminalNum"]
					list[i] = m
				}
			}
			resp.Data = map[string]interface{}{
				"totalPage": data["pages"],
				"groupList": list,
			}
		}
	}

	return resp, nil
}

// GetGroupManageListSearch searches groups by name (paginated).
func (c *Client) GetGroupManageListSearch(session string, pageNum int, search string) (*TMSResponse, error) {
	result, err := c.doSignedPost("/v1/tps/group/list", map[string]interface{}{
		"page":   pageNum,
		"search": search,
		"size":   10,
	})
	if err != nil {
		return nil, err
	}

	code, _ := toInt(result["code"])
	rc := mapResponseCode(code)

	resp := &TMSResponse{
		ResultCode: rc,
		Desc:       toString(result["desc"]),
	}

	if rc == 0 {
		data, _ := result["data"].(map[string]interface{})
		if data != nil {
			list, _ := data["list"].([]interface{})
			for i, item := range list {
				if m, ok := item.(map[string]interface{}); ok {
					m["id"], _ = toInt(m["id"])
					m["totalTerminals"] = m["totalTerminalNum"]
					list[i] = m
				}
			}
			resp.Data = map[string]interface{}{
				"totalPage": data["pages"],
				"groupList": list,
			}
		}
	}

	return resp, nil
}

// GetGroupManageTerminal retrieves group detail and all terminals in the group.
func (c *Client) GetGroupManageTerminal(session string, groupId int) (*TMSResponse, error) {
	// Step 1: Get group detail to retrieve the operationMark.
	detailResult, err := c.doPost(session, "/market/manage/group/detail/normal", map[string]interface{}{
		"groupId": strconv.Itoa(groupId),
	})
	if err != nil {
		return nil, err
	}

	detailCode, _ := toInt(detailResult["code"])
	if detailCode != 200 {
		return &TMSResponse{
			ResultCode: mapResponseCode(detailCode),
			Desc:       toString(detailResult["desc"]),
		}, nil
	}

	detailData, _ := detailResult["data"].(map[string]interface{})
	operationMark := toString(detailData["operationMark"])

	// Step 2: Page through all group terminals.
	allTerminals := []interface{}{}
	pages := 1

	for i := 1; i <= pages; i++ {
		pageResult, err := c.doPost(session, "/market/manage/groupTerminal/page", map[string]interface{}{
			"groupId":       strconv.Itoa(groupId),
			"operationMark": operationMark,
			"operationType": 1,
			"page":          i,
			"size":          100,
		})
		if err != nil {
			return nil, err
		}

		pageCode, _ := toInt(pageResult["code"])
		if pageCode != 200 {
			return &TMSResponse{
				ResultCode: mapResponseCode(pageCode),
				Desc:       toString(pageResult["desc"]),
			}, nil
		}

		pageData, _ := pageResult["data"].(map[string]interface{})
		if pageData != nil {
			totalPages, _ := toInt(pageData["pages"])
			pages = totalPages

			list, _ := pageData["list"].([]interface{})
			for _, item := range list {
				if m, ok := item.(map[string]interface{}); ok {
					m["terminalId"], _ = toInt(m["terminalId"])
				}
			}
			allTerminals = append(allTerminals, list...)
		}
	}

	return &TMSResponse{
		ResultCode: 0,
		Data:       map[string]interface{}{"terminals": allTerminals},
	}, nil
}

// GetGroupTerminalSearch searches for terminals available for group assignment.
func (c *Client) GetGroupTerminalSearch(session, search string) (*TMSResponse, error) {
	operationMark, err := c.getOperationMark(session)
	if err != nil {
		return nil, err
	}

	allTerminals := []interface{}{}
	pages := 1

	for i := 1; i <= pages; i++ {
		result, err := c.doPost(session, "/market/manage/groupTerminal/selectionPage", map[string]interface{}{
			"operationMark": operationMark,
			"operationType": 0,
			"page":          i,
			"search":        search,
			"size":          100,
		})
		if err != nil {
			return nil, err
		}

		code, _ := toInt(result["code"])
		if code != 200 {
			return &TMSResponse{
				ResultCode: mapResponseCode(code),
				Desc:       toString(result["desc"]),
			}, nil
		}

		data, _ := result["data"].(map[string]interface{})
		if data != nil {
			totalPages, _ := toInt(data["pages"])
			pages = totalPages

			list, _ := data["list"].([]interface{})
			for _, item := range list {
				if m, ok := item.(map[string]interface{}); ok {
					m["terminalId"], _ = toInt(m["id"])
				}
			}
			allTerminals = append(allTerminals, list...)
		}
	}

	return &TMSResponse{
		ResultCode: 0,
		Data:       map[string]interface{}{"terminals": allTerminals},
	}, nil
}

// AddGroup creates a new terminal group.
// New API: direct creation with groupName (no operationMark or preAdd needed).
func (c *Client) AddGroup(session, groupName string, terminalList []int) (*TMSResponse, error) {
	addResult, err := c.doSignedPost("/v1/tps/group/add/normal", map[string]interface{}{
		"groupName": groupName,
	})
	if err != nil {
		return nil, err
	}

	addCode, _ := toInt(addResult["code"])
	rc := mapResponseCode(addCode)

	return &TMSResponse{
		ResultCode: rc,
		Desc:       toString(addResult["desc"]),
	}, nil
}

// EditGroup updates a group's name.
// New API: simplified to just id + groupName (no operationMark or preAdd/preDel).
func (c *Client) EditGroup(session string, groupId int, groupName string, newTerminals, oldTerminals []int) (*TMSResponse, error) {
	updateResult, err := c.doSignedPost("/v1/tps/group/update/normal", map[string]interface{}{
		"id":        strconv.Itoa(groupId),
		"groupName": groupName,
	})
	if err != nil {
		return nil, err
	}

	updateCode, _ := toInt(updateResult["code"])
	rc := mapResponseCode(updateCode)

	return &TMSResponse{
		ResultCode: rc,
		Desc:       toString(updateResult["desc"]),
		RawData:    updateResult,
	}, nil
}

// DeleteGroup removes a group by its ID.
func (c *Client) DeleteGroup(session string, groupId int) (*TMSResponse, error) {
	result, err := c.doSignedPost("/v1/tps/group/delete", map[string]interface{}{
		"groupId": strconv.Itoa(groupId),
	})
	if err != nil {
		return nil, err
	}

	code, _ := toInt(result["code"])
	rc := mapResponseCode(code)

	return &TMSResponse{
		ResultCode: rc,
		Desc:       toString(result["desc"]),
	}, nil
}

// ---------------------------------------------------------------------------
// Location Data
// ---------------------------------------------------------------------------

// GetCountryList retrieves the list of countries.
func (c *Client) GetCountryList(session string) (*TMSResponse, error) {
	result, err := c.doSignedPost("/v1/tps/common/country/selector", nil)
	if err != nil {
		return nil, err
	}

	code, _ := toInt(result["code"])
	rc := mapResponseCode(code)

	resp := &TMSResponse{
		ResultCode: rc,
		Desc:       toString(result["desc"]),
	}

	if rc == 0 {
		if data, ok := result["data"].([]interface{}); ok {
			countries := []interface{}{}
			for _, item := range data {
				m, _ := item.(map[string]interface{})
				if m != nil {
					m["id"], _ = toInt(m["id"])
					m["name"] = toString(m["label"])
					countries = append(countries, m)
				}
			}
			resp.Data = map[string]interface{}{"countries": countries}
		}
	}

	return resp, nil
}

// GetStateList retrieves the list of states for a given country.
func (c *Client) GetStateList(session string, countryId int) (*TMSResponse, error) {
	result, err := c.doSignedPost("/v1/tps/common/state/selector", map[string]interface{}{
		"id": strconv.Itoa(countryId),
	})
	if err != nil {
		return nil, err
	}

	code, _ := toInt(result["code"])
	rc := mapResponseCode(code)

	resp := &TMSResponse{
		ResultCode: rc,
		Desc:       toString(result["desc"]),
	}

	if rc == 0 {
		if data, ok := result["data"].([]interface{}); ok {
			states := []interface{}{}
			for _, item := range data {
				m, _ := item.(map[string]interface{})
				if m != nil {
					m["id"], _ = toInt(m["id"])
					m["name"] = toString(m["label"])
					states = append(states, m)
				}
			}
			resp.Data = map[string]interface{}{"states": states}
		}
	}

	return resp, nil
}

// GetCityList retrieves the list of cities for a given state.
func (c *Client) GetCityList(session string, stateId int) (*TMSResponse, error) {
	result, err := c.doSignedPost("/v1/tps/common/city/selector", map[string]interface{}{
		"id": strconv.Itoa(stateId),
	})
	if err != nil {
		return nil, err
	}

	code, _ := toInt(result["code"])
	rc := mapResponseCode(code)

	resp := &TMSResponse{
		ResultCode: rc,
		Desc:       toString(result["desc"]),
	}

	if rc == 0 {
		if data, ok := result["data"].([]interface{}); ok {
			cities := []interface{}{}
			for _, item := range data {
				m, _ := item.(map[string]interface{})
				if m != nil {
					m["id"], _ = toInt(m["id"])
					m["name"] = toString(m["label"])
					cities = append(cities, m)
				}
			}
			resp.Data = map[string]interface{}{"cities": cities}
		}
	}

	return resp, nil
}

// GetDistrictList retrieves the list of districts for a given city.
func (c *Client) GetDistrictList(session string, cityId int) (*TMSResponse, error) {
	result, err := c.doSignedPost("/v1/tps/common/district/selector", map[string]interface{}{
		"id": strconv.Itoa(cityId),
	})
	if err != nil {
		return nil, err
	}

	code, _ := toInt(result["code"])
	rc := mapResponseCode(code)

	resp := &TMSResponse{
		ResultCode: rc,
		Desc:       toString(result["desc"]),
	}

	if rc == 0 {
		if data, ok := result["data"].([]interface{}); ok {
			districts := []interface{}{}
			for _, item := range data {
				m, _ := item.(map[string]interface{})
				if m != nil {
					m["id"], _ = toInt(m["id"])
					m["name"] = toString(m["label"])
					districts = append(districts, m)
				}
			}
			resp.Data = map[string]interface{}{"districts": districts}
		}
	}

	return resp, nil
}

// GetTimeZoneList retrieves the list of available time zones.
func (c *Client) GetTimeZoneList(session string) (*TMSResponse, error) {
	result, err := c.doSignedPost("/v1/tps/common/timeZone/selector", nil)
	if err != nil {
		return nil, err
	}

	code, _ := toInt(result["code"])
	rc := mapResponseCode(code)

	resp := &TMSResponse{
		ResultCode: rc,
		Desc:       toString(result["desc"]),
	}

	if rc == 0 {
		if data, ok := result["data"].([]interface{}); ok {
			timeZones := []interface{}{}
			for _, item := range data {
				m, _ := item.(map[string]interface{})
				if m != nil {
					m["name"] = toString(m["label"])
					timeZones = append(timeZones, m)
				}
			}
			resp.Data = map[string]interface{}{"timeZones": timeZones}
		}
	}

	return resp, nil
}

// ---------------------------------------------------------------------------
// Vendor / Model
// ---------------------------------------------------------------------------

// GetVendorList retrieves the list of terminal vendors.
func (c *Client) GetVendorList(session string) (*TMSResponse, error) {
	result, err := c.doSignedPost("/v1/tps/common/vendor/selector", nil)
	if err != nil {
		return nil, err
	}

	code, _ := toInt(result["code"])
	rc := mapResponseCode(code)

	resp := &TMSResponse{
		ResultCode: rc,
		Desc:       toString(result["desc"]),
	}

	if rc == 0 {
		if data, ok := result["data"].([]interface{}); ok {
			vendors := []interface{}{}
			for _, item := range data {
				m, _ := item.(map[string]interface{})
				if m != nil {
					m["name"] = toString(m["label"])
					vendors = append(vendors, m)
				}
			}
			resp.Data = map[string]interface{}{"vendors": vendors}
		}
	}

	return resp, nil
}

// GetModelList retrieves the list of terminal models for a given vendor.
func (c *Client) GetModelList(session string, vendorId string) (*TMSResponse, error) {
	result, err := c.doSignedPost("/v1/tps/common/model/selector", map[string]interface{}{
		"vendor": vendorId,
	})
	if err != nil {
		return nil, err
	}

	code, _ := toInt(result["code"])
	rc := mapResponseCode(code)

	resp := &TMSResponse{
		ResultCode: rc,
		Desc:       toString(result["desc"]),
	}

	if rc == 0 {
		if data, ok := result["data"].([]interface{}); ok {
			models := []interface{}{}
			for _, item := range data {
				m, _ := item.(map[string]interface{})
				if m != nil {
					m["name"] = toString(m["label"])
					models = append(models, m)
				}
			}
			resp.Data = map[string]interface{}{"models": models}
		}
	}

	return resp, nil
}

// ---------------------------------------------------------------------------
// Utility functions
// ---------------------------------------------------------------------------

// toInt converts an interface{} value to int, handling both float64 (JSON
// numbers) and string representations.
func toInt(v interface{}) (int, bool) {
	switch val := v.(type) {
	case float64:
		return int(val), true
	case int:
		return val, true
	case string:
		n, err := strconv.Atoi(val)
		if err != nil {
			return 0, false
		}
		return n, true
	default:
		return 0, false
	}
}

// toInt64 converts an interface{} to int64 (JSON numbers can exceed int range).
func toInt64(v interface{}) (int64, bool) {
	switch val := v.(type) {
	case float64:
		return int64(val), true
	case int:
		return int64(val), true
	case int64:
		return val, true
	case string:
		n, err := strconv.ParseInt(val, 10, 64)
		if err != nil {
			return 0, false
		}
		return n, true
	default:
		return 0, false
	}
}

// toIntDefault converts v to int, returning defaultVal if conversion fails.
func toIntDefault(v interface{}, defaultVal int) int {
	n, ok := toInt(v)
	if !ok {
		return defaultVal
	}
	return n
}

// toString safely converts an interface{} to string.
func toString(v interface{}) string {
	if v == nil {
		return ""
	}
	if s, ok := v.(string); ok {
		return s
	}
	return fmt.Sprintf("%v", v)
}

// ToString is the exported version of toString for use by other packages.
func ToString(v interface{}) string {
	return toString(v)
}

// intDiff returns elements that are in a but not in b.
func intDiff(a, b []int) []int {
	set := make(map[int]struct{}, len(b))
	for _, v := range b {
		set[v] = struct{}{}
	}
	var diff []int
	for _, v := range a {
		if _, found := set[v]; !found {
			diff = append(diff, v)
		}
	}
	return diff
}
