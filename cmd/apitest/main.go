package main

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
	"os"
	"sort"
	"strings"
	"time"
)

const (
	baseURL      = "https://tps.veristore.net"
	accessKey    = "63048727"
	accessSecret = "64ab2e0d0f74447c900da052b3976426"
)

type TestResult struct {
	Category   string
	Name       string
	Method     string
	URL        string
	StatusCode int
	APICode    string
	APIDesc    string
	Duration   time.Duration
	Error      string
	Response   string
}

func generateSignature(params map[string]interface{}) string {
	// Filter empty values and signature
	filtered := make(map[string]interface{})
	for k, v := range params {
		if k == "signature" {
			continue
		}
		if v == nil {
			continue
		}
		if s, ok := v.(string); ok && s == "" {
			continue
		}
		filtered[k] = v
	}

	// Sort keys
	keys := make([]string, 0, len(filtered))
	for k := range filtered {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	// Build original text
	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		v := filtered[k]
		var valueStr string
		switch val := v.(type) {
		case string:
			valueStr = val
		case float64:
			if val == float64(int64(val)) {
				valueStr = fmt.Sprintf("%d", int64(val))
			} else {
				valueStr = fmt.Sprintf("%v", val)
			}
		case int64:
			valueStr = fmt.Sprintf("%d", val)
		case int:
			valueStr = fmt.Sprintf("%d", val)
		case []interface{}:
			jsonBytes, _ := json.Marshal(val)
			valueStr = string(jsonBytes)
		case map[string]interface{}:
			jsonBytes, _ := marshalSorted(val)
			valueStr = string(jsonBytes)
		default:
			valueStr = fmt.Sprintf("%v", val)
		}
		parts = append(parts, fmt.Sprintf("%s=%s", k, valueStr))
	}

	originalText := strings.Join(parts, "&")

	// HMAC-SHA256
	mac := hmac.New(sha256.New, []byte(accessSecret))
	mac.Write([]byte(originalText))
	signature := strings.ToUpper(hex.EncodeToString(mac.Sum(nil)))

	return signature
}

func marshalSorted(m map[string]interface{}) ([]byte, error) {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	buf := bytes.NewBufferString("{")
	for i, k := range keys {
		if i > 0 {
			buf.WriteString(",")
		}
		keyBytes, _ := json.Marshal(k)
		buf.Write(keyBytes)
		buf.WriteString(":")
		valBytes, _ := json.Marshal(m[k])
		buf.Write(valBytes)
	}
	buf.WriteString("}")
	return buf.Bytes(), nil
}

func makeRequest(endpoint string, params map[string]interface{}) (int, map[string]interface{}, time.Duration, error) {
	timestamp := time.Now().UnixMilli()
	params["accessKey"] = accessKey
	params["timestamp"] = timestamp

	sig := generateSignature(params)
	params["signature"] = sig

	body, err := json.Marshal(params)
	if err != nil {
		return 0, nil, 0, fmt.Errorf("marshal error: %w", err)
	}

	url := baseURL + endpoint

	client := &http.Client{
		Timeout: 30 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		},
	}

	req, err := http.NewRequest("POST", url, bytes.NewReader(body))
	if err != nil {
		return 0, nil, 0, fmt.Errorf("request creation error: %w", err)
	}
	req.Header.Set("Content-Type", "application/json; charset=utf-8")
	req.Header.Set("Accept-Language", "en-GB")

	start := time.Now()
	resp, err := client.Do(req)
	duration := time.Since(start)
	if err != nil {
		return 0, nil, duration, fmt.Errorf("request error: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return resp.StatusCode, nil, duration, fmt.Errorf("read error: %w", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return resp.StatusCode, nil, duration, fmt.Errorf("unmarshal error (body: %s): %w", string(respBody[:min(200, len(respBody))]), err)
	}

	return resp.StatusCode, result, duration, nil
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

type endpoint struct {
	Category string
	Name     string
	Path     string
	Params   map[string]interface{}
	IsWrite  bool
}

func main() {
	endpoints := []endpoint{
		// COMMON API - Read Only
		{"Common", "Vendor Selection List", "/v1/tps/common/vendor/selector", map[string]interface{}{}, false},
		{"Common", "Model Selection List", "/v1/tps/common/model/selector", map[string]interface{}{"vendor": "Verifone"}, false},
		{"Common", "Timezone Selection List", "/v1/tps/common/timeZone/selector", map[string]interface{}{}, false},
		{"Common", "Country Selection List", "/v1/tps/common/country/selector", map[string]interface{}{}, false},
		{"Common", "State Selection List", "/v1/tps/common/state/selector", map[string]interface{}{"id": 5}, false},
		{"Common", "City Selection List", "/v1/tps/common/city/selector", map[string]interface{}{"id": 1}, false},
		{"Common", "District Selection List", "/v1/tps/common/district/selector", map[string]interface{}{"id": 1}, false},

		// APPLICATION API - Read Only
		{"Application", "List Applications", "/v1/tps/app/list", map[string]interface{}{"page": 1, "size": 5}, false},
		{"Application", "Application Detail (v2)", "/v2/tps/app/detail", map[string]interface{}{"packageName": "com.vfi.android.payment.cimb", "version": "1.0", "appId": "1"}, false},

		// GROUP API
		{"Group", "List Groups", "/v1/tps/group/list", map[string]interface{}{"page": 1, "size": 5}, false},
		{"Group", "Group Detail", "/v1/tps/group/detail", map[string]interface{}{"groupId": "1"}, false},
		{"Group", "Create Group", "/v1/tps/group/add/normal", map[string]interface{}{"groupName": "api_test_group_temp", "subGroupIds": []interface{}{}, "terminalIds": []interface{}{}}, true},
		{"Group", "Update Group", "/v1/tps/group/update/normal", map[string]interface{}{"id": "1", "groupName": "api_test_group_temp", "subGroupIds": []interface{}{}, "terminalIds": []interface{}{}}, true},
		{"Group", "Delete Group", "/v1/tps/group/delete", map[string]interface{}{"id": "999999"}, true},

		// MERCHANT API
		{"Merchant", "List Merchants", "/v1/tps/merchant/list", map[string]interface{}{"page": 1, "size": 5}, false},
		{"Merchant", "Merchant Detail", "/v1/tps/merchant/detail", map[string]interface{}{"merchantId": "1"}, false},
		{"Merchant", "Create Merchant", "/v1/tps/merchant/add", map[string]interface{}{
			"merchantName": "API_Test_Merchant_Temp",
			"address":      "Test Address",
			"cellPhone":    "0000000000",
			"cityId":       "1",
			"contact":      "Test",
			"countryId":    "5",
			"email":        "test@test.com",
			"stateId":      "1",
			"timeZone":     "UTC+7",
		}, true},
		{"Merchant", "Update Merchant", "/v1/tps/merchant/update", map[string]interface{}{
			"id":           "1",
			"merchantName": "API_Test_Merchant_Updated",
			"address":      "Test Address",
			"cellPhone":    "0000000000",
			"cityId":       "1",
			"contact":      "Test",
			"countryId":    "5",
			"email":        "test@test.com",
			"stateId":      "1",
			"timeZone":     "UTC+7",
		}, true},
		{"Merchant", "Delete Merchant", "/v1/tps/merchant/delete", map[string]interface{}{"id": "999999"}, true},

		// TERMINAL API
		{"Terminal", "List Terminals", "/v1/tps/terminal/list", map[string]interface{}{"page": 1, "size": 5}, false},
		{"Terminal", "Terminal Detail", "/v1/tps/terminal/detail", map[string]interface{}{"terminalId": "1"}, false},
		{"Terminal", "Create Terminal", "/v1/tps/terminal/add", map[string]interface{}{
			"merchantId": "1",
			"model":      "X990 v4",
			"iotFlag":    0,
			"sn":         "APITEST0001",
		}, true},
		{"Terminal", "Update Terminal", "/v1/tps/terminal/update", map[string]interface{}{
			"id":         "1",
			"merchantId": "1",
			"model":      "X990 v4",
			"iotFlag":    0,
			"status":     0,
		}, true},
		{"Terminal", "Delete Terminal", "/v1/tps/terminal/delete", map[string]interface{}{"id": "999999"}, true},
		{"Terminal", "List Terminal Apps (v2)", "/v2/tps/terminalApp/list", map[string]interface{}{"terminalId": "1"}, false},
		{"Terminal", "List Terminal App Params (v2)", "/v2/tps/terminalAppParameter/list", map[string]interface{}{"terminalId": "1", "appId": "1"}, false},
		{"Terminal", "Update Terminal App Params (v2)", "/v2/tps/terminalAppParameter/update", map[string]interface{}{
			"terminalId":  "1",
			"appId":       "1",
			"updParamMap": map[string]interface{}{},
		}, true},

		// TASK API
		{"Task", "List Tasks", "/v1/tps/task/list", map[string]interface{}{"page": 1, "size": 5}, false},
		{"Task", "Task Detail", "/v1/tps/task/detail", map[string]interface{}{"taskId": "1"}, false},
		{"Task", "Task Terminal Page", "/v1/tps/task/list/terminal", map[string]interface{}{"taskId": "1", "page": 1, "size": 5}, false},
		{"Task", "App Param Download App List", "/v1/tps/task/appParameterDownload/appList", map[string]interface{}{"searchSnList": []interface{}{"APITEST0001"}}, false},
		{"Task", "Create App Download Task (v2)", "/v2/tps/task/appDownload/add", map[string]interface{}{
			"name":             "api_test_task_temp",
			"downloadStrategy": 0,
			"installMode":      0,
			"installStrategy":  0,
			"searchSnList":     []interface{}{"APITEST0001"},
			"taskSoftwareList": []interface{}{map[string]interface{}{
				"index": 0, "launcherFlag": 0, "softwareId": "1", "softwareType": 0, "uninstallFlag": 0,
			}},
		}, true},
		{"Task", "Create App Param Download Task", "/v1/tps/task/appParameterDownload/add", map[string]interface{}{
			"name":             "api_test_param_task_temp",
			"downloadStrategy": 0,
			"installStrategy":  0,
			"searchSnList":     []interface{}{"APITEST0001"},
			"taskSoftwareList": []interface{}{map[string]interface{}{"softwareId": "1"}},
		}, true},
		{"Task", "Create App Uninstall Task", "/v1/tps/task/appUninstall/add", map[string]interface{}{
			"name":              "api_test_uninstall_temp",
			"forceUninstall":    0,
			"unInstallMode":     0,
			"uninstallStrategy": 0,
			"searchSnList":      []interface{}{"APITEST0001"},
			"taskSoftwareList":  []interface{}{map[string]interface{}{"appId": "1", "index": 0}},
		}, true},
		{"Task", "Create Message Push Task", "/v1/tps/task/messagePush/add", map[string]interface{}{
			"name":                 "api_test_msg_push_temp",
			"message":             "test",
			"notificationStrategy": 0,
			"expireStrategy":       0,
			"pushMessageTo":       "0",
			"searchSnList":        []interface{}{"APITEST0001"},
		}, true},
		{"Task", "Cancel Task", "/v1/tps/task/cancel", map[string]interface{}{"taskId": "999999"}, true},
	}

	results := make([]TestResult, 0)

	fmt.Println("=== VeriStore TPS API Test Suite ===")
	fmt.Printf("Base URL: %s\n", baseURL)
	fmt.Printf("Access Key: %s\n", accessKey)
	fmt.Printf("Started: %s\n\n", time.Now().Format(time.RFC3339))

	for i, ep := range endpoints {
		label := "READ"
		if ep.IsWrite {
			label = "WRITE"
		}
		fmt.Printf("[%d/%d] [%s] %s - %s %s\n", i+1, len(endpoints), label, ep.Category, ep.Name, ep.Path)

		statusCode, respMap, duration, err := makeRequest(ep.Path, ep.Params)

		result := TestResult{
			Category:   ep.Category,
			Name:       ep.Name,
			URL:        ep.Path,
			Method:     "POST",
			StatusCode: statusCode,
			Duration:   duration,
		}

		if err != nil {
			result.Error = err.Error()
			fmt.Printf("  -> ERROR: %s (%.0fms)\n", err.Error(), float64(duration.Milliseconds()))
		} else {
			if code, ok := respMap["code"]; ok {
				result.APICode = fmt.Sprintf("%v", code)
			}
			if desc, ok := respMap["desc"]; ok {
				if desc != nil {
					result.APIDesc = fmt.Sprintf("%v", desc)
				}
			}
			respJSON, _ := json.Marshal(respMap)
			if len(respJSON) > 500 {
				result.Response = string(respJSON[:500]) + "..."
			} else {
				result.Response = string(respJSON)
			}
			fmt.Printf("  -> HTTP %d | API Code: %s | %s (%.0fms)\n", statusCode, result.APICode, result.APIDesc, float64(duration.Milliseconds()))
		}

		results = append(results, result)
	}

	// Generate report
	generateReport(results)
}

func generateReport(results []TestResult) {
	var sb strings.Builder

	sb.WriteString("# VeriStore TPS API Test Report\n\n")
	sb.WriteString(fmt.Sprintf("**Date:** %s  \n", time.Now().Format("2006-01-02 15:04:05")))
	sb.WriteString(fmt.Sprintf("**Base URL:** `%s`  \n", baseURL))
	sb.WriteString(fmt.Sprintf("**Access Key:** `%s`  \n\n", accessKey))

	// Summary
	total := len(results)
	success := 0
	authErr := 0
	serverErr := 0
	otherErr := 0
	connErr := 0

	for _, r := range results {
		if r.APICode == "200" {
			success++
		} else if r.Error != "" {
			connErr++
		} else if r.APICode == "401" || r.APICode == "403" || strings.Contains(r.APIDesc, "auth") || strings.Contains(r.APIDesc, "Auth") || strings.Contains(strings.ToLower(r.APIDesc), "access") {
			authErr++
		} else if r.StatusCode >= 500 {
			serverErr++
		} else {
			otherErr++
		}
	}

	sb.WriteString("## Summary\n\n")
	sb.WriteString(fmt.Sprintf("| Metric | Count |\n"))
	sb.WriteString(fmt.Sprintf("|--------|-------|\n"))
	sb.WriteString(fmt.Sprintf("| Total Endpoints | %d |\n", total))
	sb.WriteString(fmt.Sprintf("| Successful (API 200) | %d |\n", success))
	sb.WriteString(fmt.Sprintf("| Auth/Access Errors | %d |\n", authErr))
	sb.WriteString(fmt.Sprintf("| Server Errors (5xx) | %d |\n", serverErr))
	sb.WriteString(fmt.Sprintf("| Connection Errors | %d |\n", connErr))
	sb.WriteString(fmt.Sprintf("| Other Errors | %d |\n", otherErr))
	sb.WriteString("\n")

	// Results by category
	categories := []string{"Common", "Application", "Group", "Merchant", "Terminal", "Task"}
	for _, cat := range categories {
		sb.WriteString(fmt.Sprintf("## %s API\n\n", cat))
		sb.WriteString("| # | Endpoint | Path | HTTP | API Code | Description | Latency |\n")
		sb.WriteString("|---|----------|------|------|----------|-------------|---------|\n")

		idx := 1
		for _, r := range results {
			if r.Category != cat {
				continue
			}
			errInfo := r.APIDesc
			if r.Error != "" {
				errInfo = r.Error
				if len(errInfo) > 80 {
					errInfo = errInfo[:80] + "..."
				}
			}

			sb.WriteString(fmt.Sprintf("| %d | %s | `%s` | %d | %s | %s | %dms |\n",
				idx, r.Name, r.URL, r.StatusCode, r.APICode, errInfo, r.Duration.Milliseconds()))
			idx++
		}
		sb.WriteString("\n")
	}

	// Detailed errors
	sb.WriteString("## Detailed Error Responses\n\n")
	hasErrors := false
	for _, r := range results {
		if r.APICode != "200" || r.Error != "" {
			hasErrors = true
			sb.WriteString(fmt.Sprintf("### %s - %s\n", r.Category, r.Name))
			sb.WriteString(fmt.Sprintf("- **Path:** `%s`\n", r.URL))
			sb.WriteString(fmt.Sprintf("- **HTTP Status:** %d\n", r.StatusCode))
			sb.WriteString(fmt.Sprintf("- **API Code:** %s\n", r.APICode))
			if r.Error != "" {
				sb.WriteString(fmt.Sprintf("- **Error:** %s\n", r.Error))
			}
			if r.Response != "" {
				sb.WriteString(fmt.Sprintf("- **Response:**\n```json\n%s\n```\n", r.Response))
			}
			sb.WriteString("\n")
		}
	}
	if !hasErrors {
		sb.WriteString("No errors found.\n\n")
	}

	// Write report
	reportPath := "API_TEST_REPORT.md"
	if err := os.WriteFile(reportPath, []byte(sb.String()), 0644); err != nil {
		fmt.Printf("\nError writing report: %v\n", err)
		return
	}
	fmt.Printf("\nReport written to: %s\n", reportPath)
}
