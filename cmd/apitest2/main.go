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
	"sort"
	"strings"
	"time"
)

const (
	baseURL      = "https://tps.veristore.net"
	accessKey    = "63048727"
	accessSecret = "64ab2e0d0f74447c900da052b3976426"
)

func generateSignature(params map[string]interface{}) string {
	filtered := make(map[string]interface{})
	for k, v := range params {
		if k == "signature" || v == nil {
			continue
		}
		if s, ok := v.(string); ok && s == "" {
			continue
		}
		filtered[k] = v
	}
	keys := make([]string, 0, len(filtered))
	for k := range filtered {
		keys = append(keys, k)
	}
	sort.Strings(keys)
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
			jsonBytes, _ := json.Marshal(val)
			valueStr = string(jsonBytes)
		default:
			valueStr = fmt.Sprintf("%v", val)
		}
		parts = append(parts, fmt.Sprintf("%s=%s", k, valueStr))
	}
	originalText := strings.Join(parts, "&")
	mac := hmac.New(sha256.New, []byte(accessSecret))
	mac.Write([]byte(originalText))
	return strings.ToUpper(hex.EncodeToString(mac.Sum(nil)))
}

func makeRequest(endpoint string, params map[string]interface{}) (map[string]interface{}, error) {
	timestamp := time.Now().UnixMilli()
	params["accessKey"] = accessKey
	params["timestamp"] = timestamp
	params["signature"] = generateSignature(params)
	body, _ := json.Marshal(params)

	client := &http.Client{
		Timeout:   30 * time.Second,
		Transport: &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}},
	}
	req, _ := http.NewRequest("POST", baseURL+endpoint, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json; charset=utf-8")
	req.Header.Set("Accept-Language", "en-GB")

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)
	var result map[string]interface{}
	json.Unmarshal(respBody, &result)
	return result, nil
}

func getFirstID(resp map[string]interface{}, field string) string {
	data, ok := resp["data"].(map[string]interface{})
	if !ok {
		return ""
	}
	list, ok := data["list"].([]interface{})
	if !ok || len(list) == 0 {
		return ""
	}
	item, ok := list[0].(map[string]interface{})
	if !ok {
		return ""
	}
	if v, ok := item[field]; ok {
		return fmt.Sprintf("%v", v)
	}
	return ""
}

func main() {
	fmt.Println("=== Phase 2: Testing with Real IDs ===\n")

	// 1. Get real group ID
	fmt.Println("--- Fetching real IDs from list endpoints ---")
	groupResp, _ := makeRequest("/v1/tps/group/list", map[string]interface{}{"page": 1, "size": 5})
	groupID := getFirstID(groupResp, "id")
	fmt.Printf("Group ID: %s\n", groupID)

	// 2. Get real merchant ID
	merchantResp, _ := makeRequest("/v1/tps/merchant/list", map[string]interface{}{"page": 1, "size": 5})
	merchantID := getFirstID(merchantResp, "id")
	fmt.Printf("Merchant ID: %s\n", merchantID)

	// 3. Get real terminal ID
	terminalResp, _ := makeRequest("/v1/tps/terminal/list", map[string]interface{}{"page": 1, "size": 5})
	terminalID := getFirstID(terminalResp, "id")
	terminalSN := ""
	if data, ok := terminalResp["data"].(map[string]interface{}); ok {
		if list, ok := data["list"].([]interface{}); ok && len(list) > 0 {
			if item, ok := list[0].(map[string]interface{}); ok {
				terminalSN = fmt.Sprintf("%v", item["sn"])
			}
		}
	}
	fmt.Printf("Terminal ID: %s (SN: %s)\n", terminalID, terminalSN)

	// 4. Get real task ID
	taskResp, _ := makeRequest("/v1/tps/task/list", map[string]interface{}{"page": 1, "size": 5})
	taskID := getFirstID(taskResp, "id")
	fmt.Printf("Task ID: %s\n", taskID)

	// 5. Get real app ID
	appResp, _ := makeRequest("/v1/tps/app/list", map[string]interface{}{"page": 1, "size": 5})
	appID := ""
	appPkg := ""
	appVer := ""
	if data, ok := appResp["data"].(map[string]interface{}); ok {
		if list, ok := data["list"].([]interface{}); ok && len(list) > 0 {
			if item, ok := list[0].(map[string]interface{}); ok {
				appID = fmt.Sprintf("%v", item["id"])
				appPkg = fmt.Sprintf("%v", item["packageName"])
				appVer = fmt.Sprintf("%v", item["version"])
			}
		}
	}
	fmt.Printf("App ID: %s (pkg: %s, ver: %s)\n", appID, appPkg, appVer)

	// 6. Get real country/state/city for merchant creation
	countryResp, _ := makeRequest("/v1/tps/common/country/selector", map[string]interface{}{})
	countryID := ""
	if data, ok := countryResp["data"].([]interface{}); ok && len(data) > 0 {
		// Find Indonesia (id=5)
		for _, c := range data {
			if item, ok := c.(map[string]interface{}); ok {
				if fmt.Sprintf("%v", item["id"]) == "5" {
					countryID = "5"
					break
				}
			}
		}
		if countryID == "" {
			if item, ok := data[0].(map[string]interface{}); ok {
				countryID = fmt.Sprintf("%v", item["id"])
			}
		}
	}
	fmt.Printf("Country ID: %s\n", countryID)

	stateResp, _ := makeRequest("/v1/tps/common/state/selector", map[string]interface{}{"id": countryID})
	stateID := ""
	if data, ok := stateResp["data"].([]interface{}); ok && len(data) > 0 {
		if item, ok := data[0].(map[string]interface{}); ok {
			stateID = fmt.Sprintf("%v", item["id"])
		}
	}
	fmt.Printf("State ID: %s\n", stateID)

	cityResp, _ := makeRequest("/v1/tps/common/city/selector", map[string]interface{}{"id": stateID})
	cityID := ""
	if data, ok := cityResp["data"].([]interface{}); ok && len(data) > 0 {
		if item, ok := data[0].(map[string]interface{}); ok {
			cityID = fmt.Sprintf("%v", item["id"])
		}
	}
	fmt.Printf("City ID: %s\n", cityID)

	fmt.Println("\n--- Testing Detail Endpoints with Real IDs ---")

	// Test Group Detail
	if groupID != "" {
		resp, _ := makeRequest("/v1/tps/group/detail", map[string]interface{}{"groupId": groupID})
		printResult("Group Detail", "/v1/tps/group/detail", resp)
	}

	// Test Merchant Detail
	if merchantID != "" {
		resp, _ := makeRequest("/v1/tps/merchant/detail", map[string]interface{}{"merchantId": merchantID})
		printResult("Merchant Detail", "/v1/tps/merchant/detail", resp)
	}

	// Test Terminal Detail
	if terminalID != "" {
		resp, _ := makeRequest("/v1/tps/terminal/detail", map[string]interface{}{"terminalId": terminalID})
		printResult("Terminal Detail", "/v1/tps/terminal/detail", resp)
	}

	// Test Task Detail
	if taskID != "" {
		resp, _ := makeRequest("/v1/tps/task/detail", map[string]interface{}{"taskId": taskID})
		printResult("Task Detail", "/v1/tps/task/detail", resp)
	}

	// Test Task Terminal Page
	if taskID != "" {
		resp, _ := makeRequest("/v1/tps/task/list/terminal", map[string]interface{}{"taskId": taskID, "page": 1, "size": 5})
		printResult("Task Terminal Page", "/v1/tps/task/list/terminal", resp)
	}

	// Test App Detail v2
	if appID != "" && appPkg != "" && appVer != "" {
		resp, _ := makeRequest("/v2/tps/app/detail", map[string]interface{}{"packageName": appPkg, "version": appVer, "appId": appID})
		printResult("App Detail (v2)", "/v2/tps/app/detail", resp)
	}

	// Test Terminal App List v2
	if terminalID != "" {
		resp, _ := makeRequest("/v2/tps/terminalApp/list", map[string]interface{}{"terminalId": terminalID})
		printResult("Terminal App List (v2)", "/v2/tps/terminalApp/list", resp)
	}

	// Test Terminal App Param List v2
	if terminalID != "" && appID != "" {
		resp, _ := makeRequest("/v2/tps/terminalAppParameter/list", map[string]interface{}{"terminalId": terminalID, "appId": appID})
		printResult("Terminal App Params (v2)", "/v2/tps/terminalAppParameter/list", resp)
	}

	// Test App Param Download App List with real SN
	if terminalSN != "" {
		resp, _ := makeRequest("/v1/tps/task/appParameterDownload/appList", map[string]interface{}{"searchSnList": []interface{}{terminalSN}})
		printResult("App Param Download App List", "/v1/tps/task/appParameterDownload/appList", resp)
	}

	// Test Create Merchant with valid region IDs
	if countryID != "" && stateID != "" && cityID != "" {
		resp, _ := makeRequest("/v1/tps/merchant/add", map[string]interface{}{
			"merchantName": "API_Test_Temp_" + fmt.Sprintf("%d", time.Now().UnixMilli()),
			"address":      "Test Address Jakarta",
			"cellPhone":    "081234567890",
			"cityId":       cityID,
			"contact":      "API Test",
			"countryId":    countryID,
			"email":        "apitest@test.com",
			"stateId":      stateID,
			"timeZone":     "UTC+7",
		})
		printResult("Create Merchant (valid region)", "/v1/tps/merchant/add", resp)

		// If merchant created, try detail and delete
		if resp != nil {
			code := fmt.Sprintf("%v", resp["code"])
			if code == "200" {
				if data, ok := resp["data"].(map[string]interface{}); ok {
					newMerchantID := fmt.Sprintf("%v", data["id"])
					fmt.Printf("  -> Created merchant ID: %s, now testing detail & delete...\n", newMerchantID)

					detailResp, _ := makeRequest("/v1/tps/merchant/detail", map[string]interface{}{"merchantId": newMerchantID})
					printResult("Merchant Detail (new)", "/v1/tps/merchant/detail", detailResp)

					deleteResp, _ := makeRequest("/v1/tps/merchant/delete", map[string]interface{}{"id": newMerchantID})
					printResult("Delete Merchant (cleanup)", "/v1/tps/merchant/delete", deleteResp)
				}
			}
		}
	}

	// Test Create Terminal with valid merchant
	if merchantID != "" {
		resp, _ := makeRequest("/v1/tps/terminal/add", map[string]interface{}{
			"merchantId": merchantID,
			"model":      "X990 v4",
			"iotFlag":    0,
			"sn":         "APITEST" + fmt.Sprintf("%d", time.Now().UnixMilli()%10000),
		})
		printResult("Create Terminal (valid merchant)", "/v1/tps/terminal/add", resp)

		if resp != nil && fmt.Sprintf("%v", resp["code"]) == "200" {
			if data, ok := resp["data"].(map[string]interface{}); ok {
				newTermID := fmt.Sprintf("%v", data["id"])
				fmt.Printf("  -> Created terminal ID: %s, cleaning up...\n", newTermID)
				deleteResp, _ := makeRequest("/v1/tps/terminal/delete", map[string]interface{}{"id": newTermID})
				printResult("Delete Terminal (cleanup)", "/v1/tps/terminal/delete", deleteResp)
			}
		}
	}

	fmt.Println("\n=== Phase 2 Complete ===")
}

func printResult(name, path string, resp map[string]interface{}) {
	if resp == nil {
		fmt.Printf("[FAIL] %s (%s) -> Connection error\n", name, path)
		return
	}
	code := fmt.Sprintf("%v", resp["code"])
	desc := ""
	if d, ok := resp["desc"]; ok && d != nil {
		desc = fmt.Sprintf("%v", d)
	}
	status := "PASS"
	if code != "200" {
		status = "FAIL"
	}
	// Check for HTTP 500 style
	if _, ok := resp["status"]; ok {
		if s, ok := resp["status"].(float64); ok && s >= 500 {
			status = "FAIL"
			code = fmt.Sprintf("HTTP %d", int(s))
		}
	}
	fmt.Printf("[%s] %s (%s) -> Code: %s | %s\n", status, name, path, code, desc)
}
