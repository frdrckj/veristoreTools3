package tms

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/go-pdf/fpdf"
	"github.com/gorilla/sessions"
	"github.com/hibiken/asynq"
	"github.com/labstack/echo/v4"
	"github.com/rs/zerolog/log"
	"github.com/verifone/veristoretools3/internal/admin"
	mw "github.com/verifone/veristoretools3/internal/middleware"
	"github.com/verifone/veristoretools3/internal/shared"
	"github.com/verifone/veristoretools3/templates/components"
	"github.com/verifone/veristoretools3/templates/layouts"
	vsTmpl "github.com/verifone/veristoretools3/templates/veristore"
	"github.com/xuri/excelize/v2"
	"gorm.io/gorm"
)

// paramCacheEntry holds pre-fetched parameter data with a timestamp.
type paramCacheEntry struct {
	params    map[string][2]string // dataName → [description, value]
	fetchedAt time.Time
}

// Handler holds dependencies for veristore (TMS) HTTP handlers.
type Handler struct {
	service          *Service
	store            sessions.Store
	sessionName      string
	appName          string
	appVersion       string
	appIcon          string
	appLogo          string
	appClientLogo    string
	appVeristoreLogo string
	appTmsURL        string
	appBgColor       string
	copyrightTitle   string
	copyrightURL     string
	packageName      string // filter apps by package name (v2 appTmsPackageName)
	adminRepo        *admin.Repository
	approvalDB       *gorm.DB
	queueClient      *asynq.Client
	queueInspector   *asynq.Inspector
	paramCache       sync.Map // key: "serialNum|appId" → *paramCacheEntry
	merchantCache    []map[string]interface{}
	merchantCacheAt  time.Time
	merchantCacheMu  sync.Mutex
}

// NewHandler creates a new veristore handler.
func NewHandler(service *Service, store sessions.Store, sessionName string, appName, appVersion string, adminRepo *admin.Repository, queueClient *asynq.Client, queueInspector *asynq.Inspector, packageName string, approvalDB *gorm.DB) *Handler {
	return &Handler{
		service:          service,
		store:            store,
		sessionName:      sessionName,
		appName:          appName,
		appVersion:       appVersion,
		appIcon:          "favicon.png",
		appLogo:          "verifone_logo.png",
		appClientLogo:    "",
		appVeristoreLogo: "veristore_logo.png",
		appTmsURL:        "",
		appBgColor:       "",
		copyrightTitle:   "Verifone",
		copyrightURL:     "https://www.verifone.com",
		packageName:      packageName,
		adminRepo:        adminRepo,
		approvalDB:       approvalDB,
		queueClient:      queueClient,
		queueInspector:   queueInspector,
	}
}

// prefetchParams fetches all terminal parameters in the background and caches them.
func (h *Handler) prefetchParams(serialNum, appId string) {
	go func() {
		tabNames := GetAllTabNames(h.service.db)
		resp, err := h.service.GetTerminalParameter(serialNum, appId, tabNames)
		if err != nil || resp == nil || resp.ResultCode != 0 || resp.Data == nil {
			return
		}
		params := map[string][2]string{}
		if pl, ok := resp.Data["paraList"].([]interface{}); ok {
			for _, p := range pl {
				m, _ := p.(map[string]interface{})
				if m == nil {
					continue
				}
				dn := fmt.Sprintf("%v", m["dataName"])
				desc := fmt.Sprintf("%v", m["description"])
				val := fmt.Sprintf("%v", m["value"])
				params[dn] = [2]string{desc, val}
			}
		}
		h.paramCache.Store(serialNum+"|"+appId, &paramCacheEntry{params: params, fetchedAt: time.Now()})
	}()
}

// getCachedParams returns cached parameters if available and fresh (< 5 min), otherwise fetches.
func (h *Handler) getCachedParams(serialNum, appId string) map[string][2]string {
	key := serialNum + "|" + appId
	if v, ok := h.paramCache.Load(key); ok {
		entry := v.(*paramCacheEntry)
		if time.Since(entry.fetchedAt) < 5*time.Minute {
			return entry.params
		}
		h.paramCache.Delete(key)
	}
	// Cache miss — fetch synchronously.
	tabNames := GetAllTabNames(h.service.db)
	params := map[string][2]string{}
	resp, err := h.service.GetTerminalParameter(serialNum, appId, tabNames)
	if err == nil && resp != nil && resp.ResultCode == 0 && resp.Data != nil {
		if pl, ok := resp.Data["paraList"].([]interface{}); ok {
			for _, p := range pl {
				m, _ := p.(map[string]interface{})
				if m == nil {
					continue
				}
				dn := fmt.Sprintf("%v", m["dataName"])
				desc := fmt.Sprintf("%v", m["description"])
				val := fmt.Sprintf("%v", m["value"])
				params[dn] = [2]string{desc, val}
			}
		}
	}
	h.paramCache.Store(key, &paramCacheEntry{params: params, fetchedAt: time.Now()})
	return params
}

// exportPayload mirrors queue.ExportTerminalPayload to avoid import cycle.
type exportPayload struct {
	SerialNos      []string `json:"serial_nos"`
	Session        string   `json:"session"`
	User           string   `json:"user"`
	ExportID       int      `json:"export_id"`
	SelectAll      bool     `json:"select_all,omitempty"`
	SearchSerialNo string   `json:"search_serial_no,omitempty"`
	SearchType     int      `json:"search_type,omitempty"`
	Username       string   `json:"username,omitempty"`
}

// SetBranding configures optional branding fields on the handler.
func (h *Handler) SetBranding(clientLogo, veristoreLogo, tmsURL, bgColor, copyrightTitle, copyrightURL string) {
	if clientLogo != "" {
		h.appClientLogo = clientLogo
	}
	if veristoreLogo != "" {
		h.appVeristoreLogo = veristoreLogo
	}
	if tmsURL != "" {
		h.appTmsURL = tmsURL
	}
	if bgColor != "" {
		h.appBgColor = bgColor
	}
	if copyrightTitle != "" {
		h.copyrightTitle = copyrightTitle
	}
	if copyrightURL != "" {
		h.copyrightURL = copyrightURL
	}
}

// pageData builds a layouts.PageData from the echo context and handler config.
func (h *Handler) pageData(c echo.Context, title string) layouts.PageData {
	flashes := shared.GetFlashes(c, h.store, h.sessionName)
	return layouts.PageData{
		Title:            title,
		AppName:          h.appName,
		AppVersion:       h.appVersion,
		AppIcon:          h.appIcon,
		AppLogo:          h.appLogo,
		AppClientLogo:    h.appClientLogo,
		AppVeristoreLogo: h.appVeristoreLogo,
		AppTmsURL:        h.appTmsURL,
		AppBgColor:       h.appBgColor,
		UserName:         mw.GetCurrentUserName(c),
		UserFullname:     mw.GetCurrentUserFullname(c),
		UserPrivileges:   mw.GetCurrentUserPrivileges(c),
		CopyrightTitle:   h.copyrightTitle,
		CopyrightURL:     h.copyrightURL,
		Flashes:          flashes,
	}
}

// ---------------------------------------------------------------------------
// Terminal Routes
// ---------------------------------------------------------------------------

// Terminal handles GET /veristore/ - List terminals with pagination and search.
func (h *Handler) Terminal(c echo.Context) error {
	if err := h.requireTmsSession(c); err != nil {
		return err
	}

	currentUser := mw.GetCurrentUserName(c)
	page := h.pageData(c, "Terminal Management")
	pageNum, _ := strconv.Atoi(c.QueryParam("page"))
	if pageNum < 1 {
		pageNum = 1
	}

	serialNo := c.QueryParam("serialNo")
	searchType, _ := strconv.Atoi(c.QueryParam("searchType"))

	var resp *TMSResponse
	var err error

	if serialNo != "" {
		resp, err = h.service.SearchTerminals(pageNum, serialNo, searchType, currentUser)
	} else {
		resp, err = h.service.GetTerminalList(pageNum)
	}

	if err != nil {
		shared.SetFlash(c, h.store, h.sessionName, shared.FlashError, fmt.Sprintf("Failed to load terminals: %v", err))
		return shared.Render(c, http.StatusOK, vsTmpl.TerminalPage(page, nil, 0, pageNum, vsTmpl.SearchParams{}, components.PaginationData{}, false))
	}

	var terminals []map[string]interface{}
	totalPage := 0
	var totalCount int64

	if resp != nil && resp.ResultCode == 0 && resp.Data != nil {
		if tl, ok := resp.Data["terminalList"].([]interface{}); ok {
			for _, t := range tl {
				if m, ok := t.(map[string]interface{}); ok {
					terminals = append(terminals, m)
				}
			}
		}
		if tp, ok := resp.Data["totalPage"]; ok {
			totalPage, _ = toInt(tp)
		}
		if tc, ok := resp.Data["total"]; ok {
			n, _ := toInt(tc)
			totalCount = int64(n)
		}
	}
	if totalCount == 0 {
		totalCount = int64(len(terminals))
	}

	// Enrich terminal results with TID/MID from local database.
	h.service.EnrichTerminalsWithTIDMID(terminals)

	searchParams := vsTmpl.SearchParams{
		SerialNo:   serialNo,
		SearchType: searchType,
	}

	// Include search params in pagination links so pages stay filtered.
	paginationQuery := ""
	if serialNo != "" {
		paginationQuery = "&serialNo=" + serialNo + "&searchType=" + strconv.Itoa(searchType)
	}

	pagination := components.PaginationData{
		CurrentPage: pageNum,
		TotalPages:  totalPage,
		Total:       totalCount,
		BaseURL:     "/veristore/terminal",
		HTMXTarget:  "terminal-table-container",
		QueryString: paginationQuery,
	}

	// Check if buttons should be disabled (pending sync or import in progress, like v2).
	buttonsDisabled := h.adminRepo.HasPendingSync() || h.adminRepo.HasPendingImport()

	// Show import result notification if available (consumed on first read).
	// Format: "prefix|success|fail|suffix"
	if importResult := h.adminRepo.PopImportResult(); importResult != "" {
		parts := strings.Split(importResult, "|")
		if len(parts) == 4 {
			// Reconstruct a readable message for the flash
			msg := parts[0]
			if parts[1] != "" {
				msg += " " + parts[1] + "."
			}
			if parts[2] != "" {
				msg += " " + parts[2] + "."
			}
			if parts[3] != "" {
				msg += " " + parts[3]
			}
			if parts[2] != "" {
				shared.SetFlash(c, h.store, h.sessionName, shared.FlashError, msg)
			} else {
				shared.SetFlash(c, h.store, h.sessionName, shared.FlashSuccess, msg)
			}
		} else {
			shared.SetFlash(c, h.store, h.sessionName, shared.FlashSuccess, importResult)
		}
	}

	if shared.IsHTMX(c) {
		return shared.Render(c, http.StatusOK, vsTmpl.TerminalTablePartial(terminals, pagination, searchParams, buttonsDisabled))
	}

	return shared.Render(c, http.StatusOK, vsTmpl.TerminalPage(page, terminals, totalPage, pageNum, searchParams, pagination, buttonsDisabled))
}

// TerminalStatus returns a lightweight JSON response indicating whether
// sync/import operations are in progress. Used by the terminal page
// to auto-refresh when operations complete.
func (h *Handler) TerminalStatus(c echo.Context) error {
	disabled := h.adminRepo.HasPendingSync() || h.adminRepo.HasPendingImport()
	return c.JSON(http.StatusOK, map[string]bool{"disabled": disabled})
}

// Add handles GET/POST /veristore/add - Add terminal form and submission.
func (h *Handler) Add(c echo.Context) error {
	page := h.pageData(c, "CSI (Add)")

	if c.Request().Method == http.MethodGet {
		// Load dropdown data.
		vendors := h.loadVendors()
		merchants := h.loadMerchants()
		groups := h.loadGroups()
		apps := h.loadApps()
		return shared.Render(c, http.StatusOK, vsTmpl.AddPage(page, vendors, merchants, groups, apps, nil))
	}

	// POST - process form.
	deviceID := strings.TrimSpace(c.FormValue("deviceId"))
	if deviceID == "" {
		shared.SetFlash(c, h.store, h.sessionName, shared.FlashError, "CSI tidak boleh kosong")
		return c.Redirect(http.StatusFound, "/veristore/add")
	}

	data := AddTerminalRequest{
		DeviceID:   deviceID,
		Vendor:     c.FormValue("vendor"),
		Model:      c.FormValue("model"),
		MerchantID: c.FormValue("merchantId"),
		SN:         c.FormValue("sn"),
		MoveConf:   0,
	}

	if mc := c.FormValue("moveConf"); mc != "" {
		data.MoveConf, _ = strconv.Atoi(mc)
	}

	// Parse group IDs.
	if groupIDs := c.Request().Form["groupIds[]"]; len(groupIDs) > 0 {
		data.GroupIDs = groupIDs
	} else if groupIDs := c.Request().Form["groupIds"]; len(groupIDs) > 0 {
		data.GroupIDs = groupIDs
	}

	appId := c.FormValue("app")
	appName := c.FormValue("appName")

	// Save as a pending approval request instead of creating directly in TMS.
	groupIDStr := strings.Join(data.GroupIDs, ",")
	now := time.Now()
	h.approvalDB.Exec(
		"INSERT INTO csi_request (req_device_id, req_vendor, req_model, req_merchant_id, req_group_ids, req_sn, req_app, req_app_name, req_move_conf, req_status, created_by, created_dt) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, 'PENDING', ?, ?)",
		data.DeviceID, data.Vendor, data.Model, data.MerchantID, groupIDStr, data.SN, appId, appName, data.MoveConf, mw.GetCurrentUserName(c), now,
	)

	mw.LogActivityFromContext(c, mw.LogVeristoreRequestCSI, "Request add CSI: "+data.DeviceID)
	shared.SetFlash(c, h.store, h.sessionName, shared.FlashSuccess, "Permintaan CSI "+data.DeviceID+" berhasil diajukan, menunggu persetujuan.")
	return c.Redirect(http.StatusFound, "/veristore/terminal")
}

// Edit handles GET/POST /veristore/edit - Edit terminal parameters.
func (h *Handler) Edit(c echo.Context) error {
	serialNum := c.QueryParam("serialNum")
	appId := c.QueryParam("appId")

	if serialNum == "" {
		serialNum = c.FormValue("serialNum")
	}
	if appId == "" {
		appId = c.FormValue("appId")
	}

	page := h.pageData(c, "Edit Terminal Parameters")

	if c.Request().Method == http.MethodGet {
		if serialNum == "" {
			shared.SetFlash(c, h.store, h.sessionName, shared.FlashError, "Serial number is required")
			return c.Redirect(http.StatusFound, "/veristore/terminal")
		}

		// Get terminal detail first to list apps.
		detailResp, err := h.service.GetTerminalDetail(serialNum)
		if err != nil {
			shared.SetFlash(c, h.store, h.sessionName, shared.FlashError, fmt.Sprintf("Failed to get terminal detail: %v", err))
			return c.Redirect(http.StatusFound, "/veristore/terminal")
		}

		// Auto-select the app with the highest version if appId not provided (like v2).
		if appId == "" && detailResp.ResultCode == 0 && detailResp.Data != nil {
			if apps, ok := detailResp.Data["terminalShowApps"].([]interface{}); ok && len(apps) > 0 {
				bestId := ""
				bestVer := ""
				for _, a := range apps {
					am, ok := a.(map[string]interface{})
					if !ok {
						continue
					}
					ver := fmt.Sprintf("%v", am["version"])
					id := fmt.Sprintf("%v", am["id"])
					if bestVer == "" || compareAppVersions(ver, bestVer) > 0 {
						bestVer = ver
						bestId = id
					}
				}
				if bestId != "" {
					appId = bestId
					return c.Redirect(http.StatusFound, fmt.Sprintf("/veristore/edit?serialNum=%s&appId=%s", serialNum, appId))
				}
			}
		}

		var paramGroups []vsTmpl.ParamGroup
		// If appId is provided, build tree from template_parameter DB table.
		// Fetch terminal parameters to resolve dynamic (*-prefixed) titles (like v2).
		if appId != "" {
			// Fetch params synchronously so we can resolve *-prefixed tree titles.
			paramData := h.getCachedParams(serialNum, appId)

			// Save TID notes for this CSI so that checkTidDuplicate can detect
			// conflicts even if this CSI was never edited through v3 before.
			h.saveTidNotesFromParams(serialNum, paramData, mw.GetCurrentUserFullname(c))

			type tplGroup struct {
				Title      string
				IndexTitle string
				Index      int
			}
			var tplGroups []tplGroup
			h.service.db.Raw(`
				SELECT tparam_title AS title,
				       tparam_index_title AS index_title,
				       tparam_index AS ` + "`index`" + `
				FROM template_parameter
				GROUP BY tparam_title, tparam_index_title, tparam_index
				ORDER BY MIN(tparam_id)
			`).Scan(&tplGroups)

			for _, tg := range tplGroups {
				subTitles := strings.Split(tg.IndexTitle, "|")
				var subItems []vsTmpl.ParamSubItem
				for i := 0; i < tg.Index && i < len(subTitles); i++ {
					subTitle := subTitles[i]
					if len(subTitle) > 0 && subTitle[0] == '*' {
						// Strip '*' to get the dataName key, look up actual value from params.
						key := subTitle[1:]
						if entry, ok := paramData[key]; ok && entry[1] != "" {
							subTitle = entry[1]
						} else {
							subTitle = key
						}
					}
					subItems = append(subItems, vsTmpl.ParamSubItem{
						Title: subTitle,
						Index: i + 1,
					})
				}
				paramGroups = append(paramGroups, vsTmpl.ParamGroup{
					Name:     tg.Title,
					SubItems: subItems,
				})
			}
		}

		// Find selected app name/version.
		// First get the app info from terminalShowApps (itemList) by appId.
		appName := ""
		appPkg := ""
		if detailResp.Data != nil {
			if apps, ok := detailResp.Data["terminalShowApps"].([]interface{}); ok {
				for _, a := range apps {
					if am, ok := a.(map[string]interface{}); ok {
						if fmt.Sprintf("%v", am["id"]) == appId {
							appName = fmt.Sprintf("%v", am["name"])
							if v, ok := am["version"].(string); ok && v != "" {
								appName += " " + v
							}
							appPkg = fmt.Sprintf("%v", am["packageName"])
							break
						}
					}
				}
			}

			// Override version using the same hybrid logic as reports:
			// 1. If appInstalls has the package → use that version (ground truth)
			// 2. If appInstalls is empty → use the HIGHEST version from terminalShowApps
			if appPkg != "" {
				overridden := false

				// Step 1: Check appInstalls (ground truth for connected terminals).
				if installs, ok := detailResp.Data["appInstalls"].([]interface{}); ok {
					for _, inst := range installs {
						im, _ := inst.(map[string]interface{})
						if im == nil {
							continue
						}
						if fmt.Sprintf("%v", im["packageName"]) == appPkg {
							iName := fmt.Sprintf("%v", im["appName"])
							iVer := fmt.Sprintf("%v", im["version"])
							if iName != "" && iVer != "" {
								appName = iName + " " + iVer
								overridden = true
							}
							break
						}
					}
				}

				// Step 2: If appInstalls didn't have it (disconnected terminal),
				// find the HIGHEST version from terminalShowApps for this package.
				if !overridden {
					if apps2, ok := detailResp.Data["terminalShowApps"].([]interface{}); ok {
						highestName := ""
						highestVer := ""
						for _, a := range apps2 {
							am, ok := a.(map[string]interface{})
							if !ok {
								continue
							}
							if fmt.Sprintf("%v", am["packageName"]) != appPkg {
								continue
							}
							ver := fmt.Sprintf("%v", am["version"])
							if highestVer == "" || compareAppVersions(ver, highestVer) > 0 {
								highestVer = ver
								highestName = fmt.Sprintf("%v", am["name"])
							}
						}
						if highestVer != "" && highestName != "" {
							appName = highestName + " " + highestVer
						}
					}
				}
			}
		}

		return shared.Render(c, http.StatusOK, vsTmpl.EditPage(page, serialNum, appId, appName, detailResp.Data, paramGroups))
	}

	// POST - update parameters.
	var paraList []map[string]interface{}
	if err := c.Request().ParseForm(); err == nil {
		// Parse parameter form fields (dynamic fields from the parameter list).
		for key, values := range c.Request().PostForm {
			if strings.HasPrefix(key, "param_") && len(values) > 0 {
				dataName := strings.TrimPrefix(key, "param_")
				viewName := c.FormValue("viewName_" + dataName)
				paraList = append(paraList, map[string]interface{}{
					"dataName": dataName,
					"viewName": viewName,
					"value":    values[0],
				})
			}
		}
	}

	// Always validate TID uniqueness before submitting (matching V2 behaviour).
	// V2 always sends the FULL paraList (all params as JSON), so the check
	// always runs against every TID field.  In V3 the form only sends the
	// currently visible tab, so we build a complete picture by fetching the
	// full params from TMS and overlaying any form-submitted changes.
	fullParams := h.getCachedParams(serialNum, appId)
	// Overlay form-submitted values so we check the user's edits.
	for _, p := range paraList {
		dn, _ := p["dataName"].(string)
		val, _ := p["value"].(string)
		if dn == "" {
			continue
		}
		if existing, ok := fullParams[dn]; ok {
			fullParams[dn] = [2]string{existing[0], val}
		} else {
			fullParams[dn] = [2]string{"", val}
		}
	}
	if errMsg := h.checkTidDuplicateFromParams(serialNum, fullParams); errMsg != "" {
		shared.SetFlash(c, h.store, h.sessionName, shared.FlashError, errMsg)
		return c.Redirect(http.StatusFound, fmt.Sprintf("/veristore/edit?serialNum=%s&appId=%s", serialNum, appId))
	}

	// Save TID notes from the full params BEFORE the update so that other
	// CSIs being edited concurrently can detect this CSI's TIDs.
	h.saveTidNotesFromParams(serialNum, fullParams, mw.GetCurrentUserFullname(c))

	if len(paraList) == 0 {
		// No parameters edited — just redirect with success.
		shared.SetFlash(c, h.store, h.sessionName, shared.FlashSuccess, "Update parameter berhasil!")
		return c.Redirect(http.StatusFound, "/veristore/terminal")
	}

	resp, err := h.service.EditTerminal(serialNum, paraList, appId)
	if err != nil {
		shared.SetFlash(c, h.store, h.sessionName, shared.FlashError, fmt.Sprintf("Failed to update parameters: %v", err))
		return c.Redirect(http.StatusFound, fmt.Sprintf("/veristore/edit?serialNum=%s&appId=%s", serialNum, appId))
	}

	if resp.ResultCode != 0 {
		shared.SetFlash(c, h.store, h.sessionName, shared.FlashError, fmt.Sprintf("Update failed: %s", resp.Desc))
		return c.Redirect(http.StatusFound, fmt.Sprintf("/veristore/edit?serialNum=%s&appId=%s", serialNum, appId))
	}

	// Update tid_note entries after successful parameter update (matching V2 TidNoteHelper::add).
	h.saveTidNotesFromParams(serialNum, fullParams, mw.GetCurrentUserFullname(c))

	mw.LogActivityFromContext(c, mw.LogVeristoreEditParam, "Edit parameter csi "+serialNum)

	shared.SetFlash(c, h.store, h.sessionName, shared.FlashSuccess, "Update parameter berhasil!")
	return c.Redirect(http.StatusFound, "/veristore/terminal")
}

// checkTidDuplicateFromParams checks if any enabled merchant TID is already
// used by a different CSI in the tid_note table (matching V2 TidNoteHelper::check).
// Works with the full cached params map (map[string][2]string) so the check
// always covers ALL TID fields regardless of which form tab was submitted.
// Returns an error message string if conflict found, "" otherwise.
func (h *Handler) checkTidDuplicateFromParams(serialNum string, params map[string][2]string) string {
	// Collect TIDs from enabled merchants.
	var enabledTIDs []string
	for i := 1; i <= 10; i++ {
		enableKey := fmt.Sprintf("TP-MERCHANT-ENABLE-%d", i)
		entry, ok := params[enableKey]
		if !ok || entry[1] != "1" {
			continue
		}
		tidKey := fmt.Sprintf("TP-MERCHANT-TERMINAL_ID-%d", i)
		tidEntry, ok := params[tidKey]
		if !ok {
			continue
		}
		tid := strings.TrimSpace(tidEntry[1])
		if tid != "" && tid != "0" {
			enabledTIDs = append(enabledTIDs, tid)
		}
	}

	if len(enabledTIDs) == 0 {
		return ""
	}

	// Check against tid_note table for conflicts with other CSIs.
	var conflicting admin.TidNote
	err := h.service.db.Where(
		"tid_note_serial_num != ? AND tid_note_data IN ?",
		serialNum,
		enabledTIDs,
	).First(&conflicting).Error
	if err == nil {
		tid := ""
		if conflicting.TidNoteData != nil {
			tid = *conflicting.TidNoteData
		}
		return fmt.Sprintf("TID %s sudah digunakan pada CSI %s", tid, conflicting.TidNoteSerialNum)
	}
	return ""
}

// saveTidNotes updates tid_note entries after a successful parameter update
// (matching V2 TidNoteHelper::add). Deletes old entries for the CSI and
// inserts new ones for each enabled merchant's TID.
func (h *Handler) saveTidNotes(serialNum string, paraList []map[string]interface{}, user string) {
	// Delete existing entries for this CSI.
	h.service.db.Where("tid_note_serial_num = ?", serialNum).Delete(&admin.TidNote{})

	// Insert new entries.
	now := time.Now()
	for i := 1; i <= 10; i++ {
		enableKey := fmt.Sprintf("TP-MERCHANT-ENABLE-%d", i)
		if getParamValue(paraList, enableKey) != "1" {
			continue
		}
		tidKey := fmt.Sprintf("TP-MERCHANT-TERMINAL_ID-%d", i)
		tid := strings.TrimSpace(getParamValue(paraList, tidKey))
		if tid == "" || tid == "0" {
			continue
		}
		h.service.db.Create(&admin.TidNote{
			TidNoteSerialNum: serialNum,
			TidNoteData:      &tid,
			CreatedBy:        user,
			CreatedDt:        now,
		})
	}
}

// saveTidNotesFromParams is like saveTidNotes but works with the cached params
// format (map[string][2]string where key=dataName, value=[description, value]).
// Used when we have params from getCachedParams (Edit GET, Copy POST).
func (h *Handler) saveTidNotesFromParams(serialNum string, params map[string][2]string, user string) {
	// Delete existing entries for this CSI.
	h.service.db.Where("tid_note_serial_num = ?", serialNum).Delete(&admin.TidNote{})

	// Insert new entries.
	now := time.Now()
	for i := 1; i <= 10; i++ {
		enableKey := fmt.Sprintf("TP-MERCHANT-ENABLE-%d", i)
		entry, ok := params[enableKey]
		if !ok || entry[1] != "1" {
			continue
		}
		tidKey := fmt.Sprintf("TP-MERCHANT-TERMINAL_ID-%d", i)
		tidEntry, ok := params[tidKey]
		if !ok {
			continue
		}
		tid := strings.TrimSpace(tidEntry[1])
		if tid == "" || tid == "0" {
			continue
		}
		h.service.db.Create(&admin.TidNote{
			TidNoteSerialNum: serialNum,
			TidNoteData:      &tid,
			CreatedBy:        user,
			CreatedDt:        now,
		})
	}
}

// getParamValue extracts a value from the paraList by dataName key.
func getParamValue(paraList []map[string]interface{}, dataName string) string {
	for _, p := range paraList {
		if dn, ok := p["dataName"].(string); ok && dn == dataName {
			if v, ok := p["value"].(string); ok {
				return v
			}
		}
	}
	return ""
}

// EditParam handles GET /veristore/edit/param - AJAX endpoint returning parameter form HTML.
// Renders a v2-compatible form with checkboxes for boolean fields, readonly support,
// field exclusion via tparam_except, and proper buttons (Back, Check, Submit).
func (h *Handler) EditParam(c echo.Context) error {
	serialNum := c.QueryParam("serialNum")
	appId := c.QueryParam("appId")
	group := c.QueryParam("group")
	sub := c.QueryParam("sub")
	index := c.QueryParam("index")

	if serialNum == "" || appId == "" || group == "" {
		return c.HTML(http.StatusBadRequest, `<div class="alert alert-danger">Missing parameters.</div>`)
	}

	// Find the tab name for this group from template_parameter.
	var tabName string
	h.service.db.Raw(`
		SELECT MAX(tparam_field) FROM template_parameter WHERE tparam_title = ?
	`, group).Scan(&tabName)
	if tabName == "" {
		return c.HTML(http.StatusNotFound, `<div class="alert alert-warning">Group not found.</div>`)
	}

	// Extract the tab name (second segment of tparam_field).
	parts := strings.SplitN(tabName, "-", 3)
	if len(parts) < 2 {
		return c.HTML(http.StatusBadRequest, `<div class="alert alert-danger">Invalid field format.</div>`)
	}
	tab := parts[1]

	// Fetch parameters for this tab from old API.
	resp, err := h.service.GetTerminalParameterTab(serialNum, appId, tab)
	if err != nil {
		return c.HTML(http.StatusInternalServerError, fmt.Sprintf(`<div class="alert alert-danger">%s</div>`, err.Error()))
	}

	// Get template_parameter rows for this group to know field definitions.
	var tplParams []admin.TemplateParameter
	h.service.db.Where("tparam_title = ?", group).Order("tparam_id").Find(&tplParams)

	// Determine user privilege index for tparam_operation (w|r|w):
	// 0 = TMS ADMIN, 1 = TMS SUPERVISOR, 2 = TMS OPERATOR
	userPriv := mw.GetCurrentUserPrivileges(c)
	privIdx := 0
	switch userPriv {
	case "TMS SUPERVISOR":
		privIdx = 1
	case "TMS OPERATOR":
		privIdx = 2
	}

	// Build a lookup of tparam_field → field definition from template_parameter.
	// The API returns dataName in the same format: "TP-TABNAME-FIELD-NUMBER",
	// so fieldKey (dataName minus the trailing "-NUMBER") matches tparam_field directly.
	type fieldDef struct {
		tp       admin.TemplateParameter
		readOnly bool
		excluded bool
		minLen   string
		maxLen   string
	}
	tplFieldMap := map[string]fieldDef{}
	for _, tp := range tplParams {
		// Check tparam_except: hide field if current index is in the except list.
		excluded := false
		if tp.TparamExcept != nil && *tp.TparamExcept != "" {
			for _, ex := range strings.Split(*tp.TparamExcept, "|") {
				if strings.TrimSpace(ex) == index {
					excluded = true
					break
				}
			}
		}

		// Check tparam_operation: determine readonly from user privilege index.
		readOnly := false
		ops := strings.Split(tp.TparamOperation, "|")
		if privIdx < len(ops) && ops[privIdx] == "r" {
			readOnly = true
		}

		// Parse tparam_length for min/max length.
		minL, maxL := "", ""
		lengths := strings.Split(tp.TparamLength, "|")
		if len(lengths) >= 2 {
			minL = lengths[0]
			maxL = lengths[1]
		} else if len(lengths) == 1 {
			maxL = lengths[0]
		}

		// Key by full tparam_field (e.g., "TP-PRINT_CONFIG-PRINT_CUSTOMER_COPY").
		tplFieldMap[tp.TparamField] = fieldDef{
			tp:       tp,
			readOnly: readOnly,
			excluded: excluded,
			minLen:   minL,
			maxLen:   maxL,
		}
	}

	// Filter parameters for the selected sub-item index.
	var filteredParams []map[string]interface{}
	if resp.ResultCode == 0 && resp.Data != nil {
		if pl, ok := resp.Data["paraList"].([]interface{}); ok {
			for _, p := range pl {
				m, _ := p.(map[string]interface{})
				if m == nil {
					continue
				}
				dn := fmt.Sprintf("%v", m["dataName"])
				// dataName format is "TP-TAB-FIELD-NUMBER", match by NUMBER suffix.
				if !strings.HasSuffix(dn, "-"+index) {
					continue
				}
				// fieldKey = dataName minus the trailing "-NUMBER".
				fieldKey := strings.TrimSuffix(dn, "-"+index)
				// Only show fields that exist in template_parameter (like v2).
				fd, exists := tplFieldMap[fieldKey]
				if !exists || fd.excluded {
					continue
				}
				filteredParams = append(filteredParams, m)
			}
		}
	}

	// Header: GROUP - Sub-item name (like v2).
	header := group
	if sub != "" {
		header = group + " - " + sub
	}

	// Build HTML form.
	html := fmt.Sprintf(`<div class="card card-success">
<div class="card-header"><h4 class="card-title mb-0">%s</h4></div>
<div class="card-body">
<form id="editParamForm" method="POST" action="/veristore/edit">
<input type="hidden" name="serialNum" value="%s"/>
<input type="hidden" name="appId" value="%s"/>
<input type="hidden" name="paraListMod" id="paraListMod" value=""/>`, header, serialNum, appId)

	for _, p := range filteredParams {
		dn := fmt.Sprintf("%v", p["dataName"])
		desc := fmt.Sprintf("%v", p["description"])
		val := fmt.Sprintf("%v", p["value"])
		vn := fmt.Sprintf("%v", p["viewName"])

		// Look up field definition by tparam_field (direct match).
		fieldKey := strings.TrimSuffix(dn, "-"+index)
		fd, hasDef := tplFieldMap[fieldKey]

		fieldType := "s" // default string
		readOnly := false
		minLen := ""
		maxLen := ""
		if hasDef {
			fieldType = fd.tp.TparamType
			readOnly = fd.readOnly
			minLen = fd.minLen
			maxLen = fd.maxLen
		}

		if fieldType == "b" {
			// Boolean: render as checkbox.
			checked := ""
			if val == "true" || val == "1" {
				checked = "checked"
			}
			hiddenVal := "0"
			if checked != "" {
				hiddenVal = "1"
			}
			disabledAttr := ""
			if readOnly {
				disabledAttr = " disabled"
			}
			html += fmt.Sprintf(`<div class="form-group">
<div class="custom-control custom-checkbox">
<input type="hidden" name="param_%s" value="%s" id="hidden_%s"/>
<input type="checkbox" class="custom-control-input" id="cb_%s" %s%s onchange="document.getElementById('hidden_%s').value=this.checked?'1':'0'"/>
<label class="custom-control-label" for="cb_%s">%s</label>
</div>
<input type="hidden" name="viewName_%s" value="%s"/>
</div>`, dn, hiddenVal, dn, dn, checked, disabledAttr, dn, dn, desc, dn, vn)
		} else {
			// String or integer: render as text input.
			readOnlyAttr := ""
			if readOnly {
				readOnlyAttr = ` readonly`
			}
			maxAttr := ""
			if maxLen != "" && maxLen != "0" {
				maxAttr = fmt.Sprintf(` maxlength="%s"`, maxLen)
			}
			minAttr := ""
			if minLen != "" && minLen != "0" {
				minAttr = fmt.Sprintf(` minlength="%s"`, minLen)
			}
			// Integer fields: add numeric-only validation like v2.
			keypressAttr := ""
			if fieldType == "i" {
				keypressAttr = ` onkeypress="return (event.charCode >= 48 && event.charCode <= 57)"`
			}
			html += fmt.Sprintf(`<div class="form-group">
<label><strong>%s</strong></label>
<input type="text" class="form-control" name="param_%s" value="%s"%s%s%s%s/>
<span class="help-block"></span>
<input type="hidden" name="viewName_%s" value="%s"/>
</div>`, desc, dn, val, maxAttr, minAttr, readOnlyAttr, keypressAttr, dn, vn)
		}
	}

	// Close form (buttons are on the main page like v2).
	html += `</form></div></div>`

	return c.HTML(http.StatusOK, html)
}

// Copy handles GET/POST /veristore/copy - Copy terminal configuration.
func (h *Handler) Copy(c echo.Context) error {
	page := h.pageData(c, "CSI (Copy)")

	// Source SN comes from query param (linked from terminal list).
	sourceSn := c.QueryParam("sourceSn")

	if c.Request().Method == http.MethodGet {
		return shared.Render(c, http.StatusOK, vsTmpl.CopyPage(page, nil, sourceSn))
	}

	// On POST, source comes from hidden field.
	sourceSn = c.FormValue("sourceSn")
	destSn := strings.TrimSpace(c.FormValue("destSn"))

	if sourceSn == "" || destSn == "" {
		return shared.Render(c, http.StatusOK, vsTmpl.CopyPage(page, []string{"Source SN and New CSI are required"}, sourceSn))
	}

	// Save as a pending approval request instead of copying directly.
	now := time.Now()
	h.approvalDB.Exec(
		"INSERT INTO csi_request (req_device_id, req_vendor, req_model, req_merchant_id, req_group_ids, req_sn, req_app, req_app_name, req_move_conf, req_template_sn, req_source, req_status, created_by, created_dt) VALUES (?, '', '', '', '', '', '', '', 0, ?, 'copy', 'PENDING', ?, ?)",
		destSn, sourceSn, mw.GetCurrentUserName(c), now,
	)

	mw.LogActivityFromContext(c, mw.LogVeristoreRequestCSI, "Request copy CSI: "+sourceSn+" → "+destSn)
	shared.SetFlash(c, h.store, h.sessionName, shared.FlashSuccess, "Permintaan copy CSI "+sourceSn+" → "+destSn+" berhasil diajukan, menunggu persetujuan.")
	return c.Redirect(http.StatusFound, "/veristore/terminal")
}

// Delete handles POST /veristore/delete - Delete terminal(s).
func (h *Handler) Delete(c echo.Context) error {
	// Check if "select all across all pages" was requested.
	if c.FormValue("selectAll") == "true" {
		return h.deleteAllTerminals(c)
	}

	serialNos := c.Request().Form["serialNos[]"]
	if len(serialNos) == 0 {
		serialNos = c.Request().Form["serialNos"]
	}
	if len(serialNos) == 0 {
		// Try single value.
		if sn := c.FormValue("serialNo"); sn != "" {
			serialNos = []string{sn}
		}
	}

	if len(serialNos) == 0 {
		shared.SetFlash(c, h.store, h.sessionName, shared.FlashError, "No terminals selected for deletion")
		return c.Redirect(http.StatusFound, "/veristore/terminal")
	}

	resp, err := h.service.DeleteTerminals(serialNos)
	if err != nil {
		shared.SetFlash(c, h.store, h.sessionName, shared.FlashError, fmt.Sprintf("Failed to delete terminal: %v", err))
		return c.Redirect(http.StatusFound, "/veristore/terminal")
	}

	if resp != nil && resp.ResultCode != 0 {
		shared.SetFlash(c, h.store, h.sessionName, shared.FlashError, fmt.Sprintf("Delete failed: %s", resp.Desc))
	} else {
		for _, sn := range serialNos {
			mw.LogActivityFromContext(c, mw.LogVeristoreDeleteCSI, "Delete csi "+sn)
		}
		shared.SetFlash(c, h.store, h.sessionName, shared.FlashSuccess, fmt.Sprintf("Delete %d CSI berhasil!", len(serialNos)))
	}

	return c.Redirect(http.StatusFound, "/veristore/terminal")
}

// terminalDeleteJob holds the info needed to delete a terminal and log it.
type terminalDeleteJob struct {
	id       string // internal TMS ID — used for direct delete (no SN lookup)
	deviceId string // CSI / serial — used for logging
}

// deleteAllTerminals fetches terminal pages and deletes them concurrently as
// pages are collected (streaming), so deletion starts immediately with the
// first page instead of waiting for all pages to be fetched.
// It uses the internal TMS ID from the list response to delete directly,
// avoiding the extra SN→ID lookup per terminal (2x faster).
func (h *Handler) deleteAllTerminals(c echo.Context) error {
	searchSerialNo := c.FormValue("searchSerialNo")
	searchType, _ := strconv.Atoi(c.FormValue("searchType"))

	logger := log.With().Str("task", "delete:all").Logger()
	logger.Info().Str("search", searchSerialNo).Int("searchType", searchType).Msg("starting delete-all")

	// Workers consume jobs as they arrive from page collection.
	const workerCount = 10
	jobs := make(chan terminalDeleteJob, 100)
	var wg sync.WaitGroup
	var successCount, failCount int64

	for i := 0; i < workerCount; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for job := range jobs {
				resp, err := h.service.DeleteTerminalByID(job.id)
				if err != nil {
					atomic.AddInt64(&failCount, 1)
					logger.Warn().Err(err).Str("sn", job.deviceId).Msg("delete failed")
					continue
				}
				if resp != nil && resp.ResultCode != 0 {
					atomic.AddInt64(&failCount, 1)
					logger.Warn().Str("sn", job.deviceId).Str("desc", resp.Desc).Msg("delete failed")
					continue
				}
				count := atomic.AddInt64(&successCount, 1)
				logger.Info().Str("sn", job.deviceId).Int64("progress", count).Msg("deleted")
			}
		}()
	}

	// Collect pages and feed terminals to workers immediately.
	var allDeviceIds []string
	var collectErr error
	for page := 1; ; page++ {
		var resp *TMSResponse
		var err error
		if searchSerialNo != "" {
			resp, err = h.service.SearchTerminalsBulk(page, searchSerialNo, searchType, mw.GetCurrentUserName(c))
		} else {
			resp, err = h.service.GetTerminalListBulk(page)
		}
		if err != nil {
			logger.Error().Err(err).Int("page", page).Msg("failed to load terminals page")
			collectErr = err
			break
		}
		if resp == nil || resp.ResultCode != 0 || resp.Data == nil {
			break
		}

		tl, ok := resp.Data["terminalList"].([]interface{})
		if !ok || len(tl) == 0 {
			break
		}
		for _, t := range tl {
			if m, ok := t.(map[string]interface{}); ok {
				devId := toString(m["deviceId"])
				tmsId := toString(m["id"])
				if devId != "" && tmsId != "" {
					allDeviceIds = append(allDeviceIds, devId)
					jobs <- terminalDeleteJob{id: tmsId, deviceId: devId}
				}
			}
		}

		totalPage := 0
		if tp, ok := resp.Data["totalPage"]; ok {
			totalPage, _ = toInt(tp)
		}
		logger.Info().Int("page", page).Int("totalPage", totalPage).Int("collected", len(allDeviceIds)).Msg("collected page")
		if page >= totalPage {
			break
		}
	}

	close(jobs) // signal workers no more jobs
	wg.Wait()   // wait for all deletions to finish

	if collectErr != nil {
		shared.SetFlash(c, h.store, h.sessionName, shared.FlashError, fmt.Sprintf("Failed to load terminals for deletion: %v", collectErr))
		return c.Redirect(http.StatusFound, "/veristore/terminal")
	}

	if len(allDeviceIds) == 0 {
		logger.Warn().Msg("no terminals found for deletion")
		shared.SetFlash(c, h.store, h.sessionName, shared.FlashError, "No terminals found for deletion")
		return c.Redirect(http.StatusFound, "/veristore/terminal")
	}

	logger.Info().Int64("success", successCount).Int64("failed", failCount).Int("total", len(allDeviceIds)).Msg("delete-all completed")

	if failCount > 0 && successCount == 0 {
		shared.SetFlash(c, h.store, h.sessionName, shared.FlashError, fmt.Sprintf("All %d deletes failed", failCount))
	} else {
		for _, sn := range allDeviceIds {
			mw.LogActivityFromContext(c, mw.LogVeristoreDeleteCSI, "Delete csi "+sn)
		}
		shared.SetFlash(c, h.store, h.sessionName, shared.FlashSuccess, fmt.Sprintf("%d deleted, %d failed", successCount, failCount))
	}

	return c.Redirect(http.StatusFound, "/veristore/terminal")
}

// Replacement handles POST /veristore/replacement - Replace terminal.
func (h *Handler) Replacement(c echo.Context) error {
	oldSn := c.FormValue("oldSn")
	newSn := c.FormValue("newSn")

	if oldSn == "" || newSn == "" {
		shared.SetFlash(c, h.store, h.sessionName, shared.FlashError, "Old and new serial numbers are required")
		return c.Redirect(http.StatusFound, "/veristore/terminal")
	}

	resp, err := h.service.ReplaceTerminal(oldSn, newSn)
	if err != nil {
		shared.SetFlash(c, h.store, h.sessionName, shared.FlashError, fmt.Sprintf("Failed to replace terminal: %v", err))
		return c.Redirect(http.StatusFound, "/veristore/terminal")
	}

	if resp.ResultCode != 0 {
		shared.SetFlash(c, h.store, h.sessionName, shared.FlashError, fmt.Sprintf("Replacement failed: %s", resp.Desc))
	} else {
		mw.LogActivityFromContext(c, mw.LogVeristoreReplace, "Replacement CSI "+oldSn+" to "+newSn)
		shared.SetFlash(c, h.store, h.sessionName, shared.FlashSuccess, "Replace CSI berhasil!")
	}

	return c.Redirect(http.StatusFound, "/veristore/terminal")
}

// Check handles GET/POST /veristore/check - Preview terminal parameters (HTMX partial).
// When called via POST from the edit page, form values (param_{dataName}) are overlaid
// on top of the cached server values so the preview reflects the user's edits.
func (h *Handler) Check(c echo.Context) error {
	serialNum := c.QueryParam("serialNum")
	appId := c.QueryParam("appId")
	if serialNum == "" {
		serialNum = c.FormValue("serialNum")
	}
	if appId == "" {
		appId = c.FormValue("appId")
	}

	if serialNum == "" || appId == "" {
		return c.HTML(http.StatusBadRequest, `<div class="alert alert-danger">serialNum and appId are required</div>`)
	}

	// Use cached parameters (pre-fetched when Edit page loaded).
	paramLookup := h.getCachedParams(serialNum, appId)

	// Overlay any form-submitted edits (from the edit page's doCheck).
	// Form fields are named "param_{dataName}" with the edited value.
	// Copy the map first to avoid corrupting the shared cache.
	if c.Request().Method == http.MethodPost {
		formParams, _ := c.FormParams()
		hasEdits := false
		for name := range formParams {
			if strings.HasPrefix(name, "param_") {
				hasEdits = true
				break
			}
		}
		if hasEdits {
			overlay := make(map[string][2]string, len(paramLookup))
			for k, v := range paramLookup {
				overlay[k] = v
			}
			for name, vals := range formParams {
				if !strings.HasPrefix(name, "param_") || len(vals) == 0 {
					continue
				}
				dataName := strings.TrimPrefix(name, "param_")
				if existing, ok := overlay[dataName]; ok {
					overlay[dataName] = [2]string{existing[0], vals[0]}
				}
			}
			paramLookup = overlay
		}
	}

	// Get template_parameter groups.
	type tplGroup struct {
		Title      string
		IndexTitle string
		Index      int
	}
	var tplGroups []tplGroup
	h.service.db.Raw(`
		SELECT tparam_title AS title,
		       tparam_index_title AS index_title,
		       tparam_index AS ` + "`index`" + `
		FROM template_parameter
		GROUP BY tparam_title, tparam_index_title, tparam_index
		ORDER BY MIN(tparam_id)
	`).Scan(&tplGroups)

	// Build two-column HTML report (first 7 groups → left, rest → right, like v2).
	body := [2]string{"", ""}
	for idx, tg := range tplGroups {
		col := 0
		if idx >= 7 {
			col = 1
		}

		var tplParams []admin.TemplateParameter
		h.service.db.Where("tparam_title = ?", tg.Title).Order("tparam_id").Find(&tplParams)

		subTitles := strings.Split(tg.IndexTitle, "|")
		groupHasContent := false
		groupHTML := fmt.Sprintf(`<h4><strong>%s</strong></h4>`, tg.Title)

		for i := 0; i < tg.Index && i < len(subTitles); i++ {
			subTitle := subTitles[i]
			if len(subTitle) > 0 && subTitle[0] == '*' {
				dataName := subTitle[1:]
				if p, ok := paramLookup[dataName]; ok && p[1] != "" {
					subTitle = p[1]
				} else {
					subTitle = dataName
				}
			}

			subHTML := fmt.Sprintf(`<h5>%s - %s</h5>`, tg.Title, subTitle)
			subHasFields := false
			for _, tp := range tplParams {
				if tp.TparamExcept != nil && *tp.TparamExcept != "" {
					excluded := false
					for _, ex := range strings.Split(*tp.TparamExcept, "|") {
						if strings.TrimSpace(ex) == fmt.Sprintf("%d", i+1) {
							excluded = true
							break
						}
					}
					if excluded {
						continue
					}
				}

				dataName := fmt.Sprintf("%s-%d", tp.TparamField, i+1)
				p, ok := paramLookup[dataName]
				if !ok {
					continue
				}

				displayVal := p[1]
				if tp.TparamType == "b" {
					if displayVal == "1" || displayVal == "true" {
						displayVal = "Yes"
					} else {
						displayVal = "No"
					}
				}

				subHTML += fmt.Sprintf(`<h5>&emsp;&emsp;%s: <strong>%s</strong></h5>`, p[0], displayVal)
				subHasFields = true
			}

			if subHasFields {
				groupHTML += subHTML
				groupHasContent = true
			}
		}

		if groupHasContent {
			body[col] += groupHTML
		}
	}

	// Return HTML fragment (loaded via AJAX into the edit page).
	printURL := fmt.Sprintf("/veristore/check/pdf?serialNum=%s&appId=%s", serialNum, appId)
	html := fmt.Sprintf(`<div class="box box-success">
<div class="d-flex justify-content-between align-items-center mb-3">
<h2 class="mb-0">CSI %s Parameters</h2>
<div>
<a href="%s" target="_blank" class="btn btn-success btn-sm"><i class="fas fa-print"></i></a>
<button type="button" class="btn btn-danger btn-sm ml-1" onclick="document.getElementById('checkResult').innerHTML=''"><i class="fas fa-times"></i></button>
</div>
</div>
<div class="row">
<div class="col-lg-6">%s</div>
<div class="col-lg-6">%s</div>
</div>
</div>`, serialNum, printURL, body[0], body[1])

	return c.HTML(http.StatusOK, html)
}

// CheckPDF handles GET /veristore/check/pdf - generates a downloadable PDF of terminal parameters.
func (h *Handler) CheckPDF(c echo.Context) error {
	serialNum := c.QueryParam("serialNum")
	appId := c.QueryParam("appId")
	if serialNum == "" || appId == "" {
		return c.String(http.StatusBadRequest, "serialNum and appId are required")
	}

	// Use cached parameters (pre-fetched when Edit page loaded).
	paramLookup := h.getCachedParams(serialNum, appId)

	// Get template_parameter groups.
	type tplGroup struct {
		Title      string
		IndexTitle string
		Index      int
	}
	var tplGroups []tplGroup
	h.service.db.Raw(`
		SELECT tparam_title AS title,
		       tparam_index_title AS index_title,
		       tparam_index AS ` + "`index`" + `
		FROM template_parameter
		GROUP BY tparam_title, tparam_index_title, tparam_index
		ORDER BY MIN(tparam_id)
	`).Scan(&tplGroups)

	// Build PDF (A4 portrait, matching v2 layout).
	pdf := fpdf.New("P", "mm", "A4", "")
	pdf.SetAutoPageBreak(true, 15)
	pdf.SetHeaderFuncMode(func() {
		pdf.SetY(5)
		pdf.SetFont("Helvetica", "I", 9)
		pdf.SetTextColor(100, 100, 100)
		pdf.CellFormat(0, 10, "Veristore Tools", "", 0, "R", false, 0, "")
		pdf.Ln(4)
		pdf.SetDrawColor(150, 150, 150)
		pdf.Line(10, pdf.GetY()+3, 200, pdf.GetY()+3)
	}, true)
	pdf.SetFooterFunc(func() {
		pdf.SetY(-15)
		pdf.SetFont("Helvetica", "I", 8)
		pdf.SetTextColor(128, 128, 128)
		pdf.CellFormat(0, 10, fmt.Sprintf("%d", pdf.PageNo()), "", 0, "R", false, 0, "")
	})

	pdf.AddPage()
	pdf.SetY(20)

	// Title.
	pdf.SetFont("Helvetica", "", 20)
	pdf.SetTextColor(0, 0, 0)
	pdf.CellFormat(0, 12, fmt.Sprintf("CSI %s Parameters", serialNum), "", 1, "L", false, 0, "")
	pdf.Ln(4)

	// Iterate groups (single column, like v2 PDF).
	for _, tg := range tplGroups {
		var tplParams []admin.TemplateParameter
		h.service.db.Where("tparam_title = ?", tg.Title).Order("tparam_id").Find(&tplParams)

		subTitles := strings.Split(tg.IndexTitle, "|")
		groupHasContent := false

		// Collect sub-item content first to check if group has any data.
		type pdfField struct {
			desc string
			val  string
		}
		type pdfSub struct {
			title  string
			fields []pdfField
		}
		var subs []pdfSub

		for i := 0; i < tg.Index && i < len(subTitles); i++ {
			subTitle := subTitles[i]
			if len(subTitle) > 0 && subTitle[0] == '*' {
				dataName := subTitle[1:]
				if p, ok := paramLookup[dataName]; ok && p[1] != "" {
					subTitle = p[1]
				} else {
					subTitle = dataName
				}
			}

			var fields []pdfField
			for _, tp := range tplParams {
				if tp.TparamExcept != nil && *tp.TparamExcept != "" {
					excluded := false
					for _, ex := range strings.Split(*tp.TparamExcept, "|") {
						if strings.TrimSpace(ex) == fmt.Sprintf("%d", i+1) {
							excluded = true
							break
						}
					}
					if excluded {
						continue
					}
				}

				dataName := fmt.Sprintf("%s-%d", tp.TparamField, i+1)
				p, ok := paramLookup[dataName]
				if !ok {
					continue
				}

				displayVal := p[1]
				if tp.TparamType == "b" {
					if displayVal == "1" || displayVal == "true" {
						displayVal = "Yes"
					} else {
						displayVal = "No"
					}
				}
				fields = append(fields, pdfField{desc: p[0], val: displayVal})
			}

			if len(fields) > 0 {
				subs = append(subs, pdfSub{title: fmt.Sprintf("%s - %s", tg.Title, subTitle), fields: fields})
				groupHasContent = true
			}
		}

		if !groupHasContent {
			continue
		}

		// Check if we need a new page for the group header + first few lines.
		if pdf.GetY() > 260 {
			pdf.AddPage()
		}

		// Group header (bold).
		pdf.Ln(3)
		pdf.SetFont("Helvetica", "B", 13)
		pdf.SetTextColor(0, 0, 0)
		pdf.CellFormat(0, 8, tg.Title, "", 1, "L", false, 0, "")
		pdf.Ln(1)

		for _, sub := range subs {
			// Sub-item header.
			pdf.SetFont("Helvetica", "", 10)
			pdf.SetTextColor(50, 50, 50)
			pdf.CellFormat(0, 6, sub.title, "", 1, "L", false, 0, "")

			// Fields (indented).
			for _, f := range sub.fields {
				pdf.SetX(20)
				pdf.SetFont("Helvetica", "", 9)
				pdf.SetTextColor(0, 0, 0)
				descW := pdf.GetStringWidth(f.desc+": ") + 2
				pdf.CellFormat(descW, 5, f.desc+": ", "", 0, "L", false, 0, "")
				pdf.SetFont("Helvetica", "B", 9)
				pdf.CellFormat(0, 5, f.val, "", 1, "L", false, 0, "")
			}
			pdf.Ln(1)
		}
	}

	// Output PDF to buffer and return as download.
	var buf bytes.Buffer
	if err := pdf.Output(&buf); err != nil {
		return c.String(http.StatusInternalServerError, "Failed to generate PDF")
	}

	c.Response().Header().Set("Content-Type", "application/pdf")
	c.Response().Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s.pdf"`, serialNum))
	return c.Blob(http.StatusOK, "application/pdf", buf.Bytes())
}

// reportPayload mirrors the background job payload to avoid import cycle.
type reportPayload struct {
	UserID      int      `json:"user_id"`
	UserName    string   `json:"user_name"`
	AppVersion  string   `json:"app_version"`
	Session     string   `json:"session"`
	DateTime    string   `json:"date_time"`
	PackageName string   `json:"package_name"`
	TriggerSync bool     `json:"trigger_sync"`
	PartialCSIs []string `json:"partial_csis,omitempty"`
}

// Report handles GET/POST /veristore/report - CSI (Update) page (like v2 actionReport).
func (h *Handler) Report(c echo.Context) error {
	page := h.pageData(c, "CSI (Update)")

	if c.Request().Method == http.MethodPost {
		appVersion := c.FormValue("appVersion")
		if appVersion == "" {
			shared.SetFlash(c, h.store, h.sessionName, shared.FlashError, "Please select an App Name")
			return c.Redirect(http.StatusFound, "/veristore/report")
		}

		mode := c.FormValue("mode") // "partial" or "full"

		// For partial mode, find today's CSIs from activity log.
		var partialCSIs []string
		if mode == "partial" {
			partialCSIs = h.adminRepo.FindTodayCSIs()
			if len(partialCSIs) == 0 {
				shared.SetFlash(c, h.store, h.sessionName, shared.FlashError, "Tidak ada CSI yang ditambahkan/dicopy/diimport hari ini.")
				return c.Redirect(http.StatusFound, "/veristore/report")
			}
		}

		// Create SyncTerminal record IMMEDIATELY so buttons are disabled
		// on redirect. The background job will update status through
		// report → sync pipeline.
		now := time.Now()
		userID := mw.GetCurrentUserID(c)
		userName := mw.GetCurrentUserFullname(c)
		_ = h.adminRepo.CreateSyncTerminal(&admin.SyncTerminal{
			SyncTermCreatorID:   userID,
			SyncTermCreatorName: userName,
			SyncTermCreatedTime: now,
			SyncTermStatus:      "0",
			SyncTermProcess:     "0",
			CreatedBy:           userName,
			CreatedDt:           now,
		})

		// Queue background report job with TriggerSync=true so it
		// automatically chains to sync:parameter after the report.
		// This combines update + sync into a single action / single row.
		session := h.service.GetSession()
		payload := reportPayload{
			UserID:      userID,
			UserName:    userName,
			AppVersion:  appVersion,
			Session:     session,
			DateTime:    now.Format("2006-01-02 15:04:05"),
			PackageName: h.packageName,
			TriggerSync: true,
			PartialCSIs: partialCSIs,
		}
		payloadBytes, _ := json.Marshal(payload)
		task := asynq.NewTask("report:terminal", payloadBytes)
		if _, err := h.queueClient.Enqueue(task, asynq.Timeout(5*time.Hour), asynq.MaxRetry(0)); err != nil {
			shared.SetFlash(c, h.store, h.sessionName, shared.FlashError, fmt.Sprintf("Failed to enqueue report job: %v", err))
			return c.Redirect(http.StatusFound, "/veristore/report")
		}

		modeLabel := "Full"
		if mode == "partial" {
			modeLabel = fmt.Sprintf("Partial (%d CSI)", len(partialCSIs))
		}
		mw.LogActivityFromContext(c, mw.LogVeristoreReport, modeLabel+" Update & Sync version "+appVersion)
		shared.SetFlash(c, h.store, h.sessionName, shared.FlashSuccess, modeLabel+" Update & Sync dimulai. Report akan tersedia dalam beberapa saat.")
		return c.Redirect(http.StatusFound, "/veristore/terminal")
	}

	// GET — populate app dropdown from TMS API.
	var apps []vsTmpl.ReportApp
	resp, err := h.service.GetAppList()
	if err == nil && resp != nil && resp.ResultCode == 0 && resp.Data != nil {
		if allApps, ok := resp.Data["allApps"].([]interface{}); ok {
			for _, a := range allApps {
				am, ok := a.(map[string]interface{})
				if !ok {
					continue
				}
				pkgName := fmt.Sprintf("%v", am["packageName"])
				if h.packageName != "" && pkgName != h.packageName {
					continue
				}
				name := fmt.Sprintf("%v", am["name"])
				version := fmt.Sprintf("%v", am["version"])
				displayName := name
				if version != "" {
					displayName = name + " - " + version
				}
				apps = append(apps, vsTmpl.ReportApp{
					Version:     version,
					DisplayName: displayName,
				})
			}
		}
	}

	return shared.Render(c, http.StatusOK, vsTmpl.ReportPage(page, apps))
}

// Export handles GET/POST /veristore/export - Export terminal data.
func (h *Handler) Export(c echo.Context) error {
	page := h.pageData(c, "CSI (Export)")
	data := vsTmpl.ExportData{}

	// Check if there is an export in progress.
	inProgress, _ := h.adminRepo.FindInProgressExport()
	if inProgress != nil {
		data.InProgress = true
		data.RequestDate = parseExportFilenameDate(inProgress.ExpFilename)
		// Read progress directly from DB to avoid any pointer/caching issues.
		var prog struct {
			Current string
			Total   string
		}
		h.service.db.Raw("SELECT COALESCE(exp_current,'0') as current, COALESCE(exp_total,'0') as total FROM `export` WHERE exp_id = ?", inProgress.ExpID).Scan(&prog)
		data.Progress = prog.Current + " / " + prog.Total
		return shared.Render(c, http.StatusOK, vsTmpl.ExportPage(page, data))
	}

	if c.Request().Method == http.MethodPost {
		// Check if this is a "Create" action (start the export job).
		if c.FormValue("buttonCreate") != "" {
			isSelectAll := c.FormValue("selectAll") == "true"
			searchSerialNo := c.FormValue("searchSerialNo")
			searchType, _ := strconv.Atoi(c.FormValue("searchType"))

			serialNoList := c.FormValue("serialNoList")
			var serialNos []string
			if !isSelectAll {
				_ = json.Unmarshal([]byte(serialNoList), &serialNos)
				if len(serialNos) == 0 {
					data.Count = 0
					return shared.Render(c, http.StatusOK, vsTmpl.ExportPage(page, data))
				}
			}

			// Create export record.
			filename := fmt.Sprintf("csi_%s.xlsx", time.Now().Format("20060102_1504"))
			total := "0"
			if !isSelectAll {
				total = strconv.Itoa(len(serialNos))
			} else {
				// Get total from TMS API for progress display.
				var tmsCount int64
				if searchSerialNo != "" {
					resp, err := h.service.SearchTerminals(1, searchSerialNo, searchType, mw.GetCurrentUserName(c))
					if err == nil && resp != nil && resp.ResultCode == 0 && resp.Data != nil {
						if tc, ok := resp.Data["total"]; ok {
							n, _ := toInt(tc)
							tmsCount = int64(n)
						}
					}
				} else {
					resp, err := h.service.GetTerminalList(1)
					if err == nil && resp != nil && resp.ResultCode == 0 && resp.Data != nil {
						if tc, ok := resp.Data["total"]; ok {
							n, _ := toInt(tc)
							tmsCount = int64(n)
						}
					}
				}
				total = strconv.FormatInt(tmsCount, 10)
			}
			current := "0"
			export := &admin.Export{
				ExpFilename: filename,
				ExpCurrent:  &current,
				ExpTotal:    &total,
			}
			if err := h.adminRepo.CreateExport(export); err != nil {
				shared.SetFlash(c, h.store, h.sessionName, shared.FlashError, fmt.Sprintf("Failed to create export record: %v", err))
				return c.Redirect(http.StatusFound, "/veristore/export?refresh=true")
			}

			// Enqueue the background job.
			session := h.service.GetSession()
			payload := exportPayload{
				SerialNos:      serialNos,
				Session:        session,
				ExportID:       export.ExpID,
				SelectAll:      isSelectAll,
				SearchSerialNo: searchSerialNo,
				SearchType:     searchType,
				Username:       mw.GetCurrentUserName(c),
			}
			payloadBytes, _ := json.Marshal(payload)
			task := asynq.NewTask("export:terminal", payloadBytes)
			if _, err := h.queueClient.Enqueue(task, asynq.Timeout(5*time.Hour), asynq.MaxRetry(0)); err != nil {
				shared.SetFlash(c, h.store, h.sessionName, shared.FlashError, fmt.Sprintf("Failed to enqueue export job: %v", err))
				return c.Redirect(http.StatusFound, "/veristore/terminal")
			}

			// Log export activity.
			if isSelectAll {
				detail := "Export all CSI"
				if searchSerialNo != "" {
					detail = fmt.Sprintf("Export all CSI (search: %s)", searchSerialNo)
				}
				mw.LogActivityFromContext(c, mw.LogVeristoreExport, detail)
			} else {
				mw.LogActivityFromContext(c, mw.LogVeristoreExport, fmt.Sprintf("Export %d CSI", len(serialNos)))
			}

			data.InProgress = true
			data.RequestDate = time.Now().Format("2006-01-02 15:04")
			return shared.Render(c, http.StatusOK, vsTmpl.ExportPage(page, data))
		}

		// Initial POST from terminal page — collect serial numbers and show count.
		selectAll := c.FormValue("selectAll")
		if selectAll == "true" {
			// Get total count from TMS API.
			searchSerialNo := c.FormValue("searchSerialNo")
			searchType, _ := strconv.Atoi(c.FormValue("searchType"))

			var tmsTotal int64
			if searchSerialNo != "" {
				resp, err := h.service.SearchTerminals(1, searchSerialNo, searchType, mw.GetCurrentUserName(c))
				if err == nil && resp != nil && resp.ResultCode == 0 && resp.Data != nil {
					if tc, ok := resp.Data["total"]; ok {
						n, _ := toInt(tc)
						tmsTotal = int64(n)
					}
				}
			} else {
				resp, err := h.service.GetTerminalList(1)
				if err == nil && resp != nil && resp.ResultCode == 0 && resp.Data != nil {
					if tc, ok := resp.Data["total"]; ok {
						n, _ := toInt(tc)
						tmsTotal = int64(n)
					}
				}
			}

			data.Count = int(tmsTotal)
			data.ShowCreate = tmsTotal > 0
			data.SelectAll = true
			data.SearchSerialNo = searchSerialNo
			data.SearchType = searchType
			return shared.Render(c, http.StatusOK, vsTmpl.ExportPage(page, data))
		}

		serialNoList := c.FormValue("serialNoList")
		if serialNoList != "" {
			var serialNos []string
			_ = json.Unmarshal([]byte(serialNoList), &serialNos)
			data.Count = len(serialNos)
			data.SerialNoList = serialNoList
			data.ShowCreate = data.Count > 0
			return shared.Render(c, http.StatusOK, vsTmpl.ExportPage(page, data))
		}

		return c.Redirect(http.StatusFound, "/veristore/terminal")
	}

	// GET request
	if c.QueryParam("refresh") == "true" {
		// Check latest export status.
		latest, err := h.adminRepo.FindLatestExport()
		if err != nil || latest == nil {
			data.Count = 0
			return shared.Render(c, http.StatusOK, vsTmpl.ExportPage(page, data))
		}

		// Check if export is complete: current == total (data may or may not be in BLOB).
		isComplete := false
		if latest.ExpCurrent != nil && latest.ExpTotal != nil {
			cur, _ := strconv.Atoi(*latest.ExpCurrent)
			tot, _ := strconv.Atoi(*latest.ExpTotal)
			if cur > 0 && cur >= tot {
				isComplete = true
			}
		}
		// Also treat as complete if file data exists.
		if latest.ExpData != nil && len(latest.ExpData) > 0 {
			isComplete = true
		}

		if isComplete {
			data.DownloadReady = true
			data.LastExportDate = parseExportFilenameDate(latest.ExpFilename)
			return shared.Render(c, http.StatusOK, vsTmpl.ExportPage(page, data))
		}

		// Still in progress.
		data.InProgress = true
		data.RequestDate = parseExportFilenameDate(latest.ExpFilename)
		return shared.Render(c, http.StatusOK, vsTmpl.ExportPage(page, data))
	}

	// Bare GET — redirect to terminal page (same as v2).
	return c.Redirect(http.StatusFound, "/veristore/terminal")
}

// collectAllTerminalIDs fetches all terminal deviceIds across all pages,
// respecting the current search filter (searchSerialNo + searchType).
func (h *Handler) collectAllTerminalIDs(c echo.Context) []string {
	searchSerialNo := c.FormValue("searchSerialNo")
	searchType, _ := strconv.Atoi(c.FormValue("searchType"))

	var allIDs []string
	for page := 1; ; page++ {
		var resp *TMSResponse
		var err error
		if searchSerialNo != "" {
			resp, err = h.service.SearchTerminalsBulk(page, searchSerialNo, searchType, mw.GetCurrentUserName(c))
		} else {
			resp, err = h.service.GetTerminalListBulk(page)
		}
		if err != nil || resp == nil || resp.ResultCode != 0 || resp.Data == nil {
			break
		}

		tl, ok := resp.Data["terminalList"].([]interface{})
		if !ok || len(tl) == 0 {
			break
		}
		for _, t := range tl {
			if m, ok := t.(map[string]interface{}); ok {
				if devId, ok := m["deviceId"].(string); ok && devId != "" {
					allIDs = append(allIDs, devId)
				}
			}
		}

		totalPage := 0
		if tp, ok := resp.Data["totalPage"]; ok {
			totalPage, _ = toInt(tp)
		}
		if page >= totalPage {
			break
		}
	}
	return allIDs
}

// parseExportFilenameDate extracts a human-readable date from filenames like "csi_20260219_2037.xlsx".
func parseExportFilenameDate(filename string) string {
	// Expected format: csi_YYYYMMDD_HHMM.xlsx
	parts := strings.Split(strings.TrimSuffix(filename, ".xlsx"), "_")
	if len(parts) >= 3 {
		d := parts[1]
		t := parts[2]
		if len(d) == 8 && len(t) >= 4 {
			return d[0:4] + "-" + d[4:6] + "-" + d[6:8] + " " + t[0:2] + ":" + t[2:4]
		}
	}
	return filename
}

// ExportResult handles GET /veristore/exportresult - Download latest export file.
func (h *Handler) ExportResult(c echo.Context) error {
	latest, err := h.adminRepo.FindLatestExport()
	if err != nil || latest == nil || latest.ExpData == nil {
		shared.SetFlash(c, h.store, h.sessionName, shared.FlashError, "No export file available")
		return c.Redirect(http.StatusFound, "/veristore/terminal")
	}

	c.Response().Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, latest.ExpFilename))
	return c.Blob(http.StatusOK, "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet", latest.ExpData)
}

// ExportReset handles GET /veristore/exportreset - Remove stuck/incomplete exports.
func (h *Handler) ExportReset(c echo.Context) error {
	_ = h.adminRepo.DeleteIncompleteExports()

	// Also purge any pending/active export tasks from the Redis queue.
	if h.queueInspector != nil {
		for _, qname := range []string{"default", "critical", "low"} {
			// Cancel active tasks.
			if active, err := h.queueInspector.ListActiveTasks(qname); err == nil {
				for _, t := range active {
					if t.Type == "export:terminal" || t.Type == "export:all" {
						_ = h.queueInspector.CancelProcessing(t.ID)
					}
				}
			}
			// Delete pending tasks.
			if pending, err := h.queueInspector.ListPendingTasks(qname); err == nil {
				for _, t := range pending {
					if t.Type == "export:terminal" || t.Type == "export:all" {
						_ = h.queueInspector.DeleteTask(qname, t.ID)
					}
				}
			}
			// Delete retry tasks.
			if retries, err := h.queueInspector.ListRetryTasks(qname); err == nil {
				for _, t := range retries {
					if t.Type == "export:terminal" || t.Type == "export:all" {
						_ = h.queueInspector.DeleteTask(qname, t.ID)
					}
				}
			}
		}
	}

	return c.Redirect(http.StatusFound, "/veristore/export?refresh=true")
}

// Import handles GET/POST /veristore/import - Import terminals from Excel.
func (h *Handler) Import(c echo.Context) error {
	page := h.pageData(c, "Import CSI")
	data := vsTmpl.ImportData{}

	// Check if there is an import in progress.
	inProgress, _ := h.adminRepo.FindInProgressImport()
	if inProgress != nil {
		data.InProgress = true
		data.RequestDate = inProgress.ImpFilename
		cur, tot := "0", "0"
		if inProgress.ImpCurrent != nil {
			cur = *inProgress.ImpCurrent
		}
		if inProgress.ImpTotal != nil {
			tot = *inProgress.ImpTotal
		}
		data.Progress = cur + " / " + tot
		return shared.Render(c, http.StatusOK, vsTmpl.ImportPage(page, data))
	}

	if c.Request().Method == http.MethodGet {
		// Check latest import for completed status.
		latest, err := h.adminRepo.FindLatestImport()
		if err == nil && latest != nil {
			cur, tot := "0", "0"
			if latest.ImpCurrent != nil {
				cur = *latest.ImpCurrent
			}
			if latest.ImpTotal != nil {
				tot = *latest.ImpTotal
			}
			if cur == tot && cur != "0" {
				data.LastComplete = true
				data.LastFilename = latest.ImpFilename
				data.Progress = cur + " / " + tot

				// Pop IMPRS notification (consumed once) to show before redirect.
				// Format: "prefix|success|fail|suffix"
				if resultMsg := h.adminRepo.PopImportResult(); resultMsg != "" {
					data.ResultMessage = resultMsg // keep raw for non-empty check
					parts := strings.Split(resultMsg, "|")
					if len(parts) == 4 {
						data.ResultPrefix = parts[0]
						data.ResultSuccess = parts[1]
						data.ResultFail = parts[2]
						data.ResultSuffix = parts[3]
						data.ResultIsError = parts[2] != ""
					} else {
						// Legacy fallback
						data.ResultPrefix = resultMsg
						data.ResultIsError = strings.Contains(resultMsg, "gagal")
					}
				}

				// Check if import result .txt file exists for download.
				resultFile := fmt.Sprintf("static/import/import_result_%d.txt", latest.ImpID)
				if _, err := os.Stat(resultFile); err == nil {
					data.ResultFileExists = true
					data.ResultFileID = latest.ImpID
				}
			}
		}
		return shared.Render(c, http.StatusOK, vsTmpl.ImportPage(page, data))
	}

	// POST - handle file upload.
	file, err := c.FormFile("file")
	if err != nil {
		data.Errors = []string{"Please select a file to upload"}
		return shared.Render(c, http.StatusOK, vsTmpl.ImportPage(page, data))
	}

	// Validate file extension.
	ext := strings.ToLower(filepath.Ext(file.Filename))
	if ext != ".xlsx" && ext != ".xls" {
		data.Errors = []string{"Only .xlsx files are supported"}
		return shared.Render(c, http.StatusOK, vsTmpl.ImportPage(page, data))
	}

	// Save file to static/import/.
	src, err := file.Open()
	if err != nil {
		data.Errors = []string{"Failed to read uploaded file"}
		return shared.Render(c, http.StatusOK, vsTmpl.ImportPage(page, data))
	}
	defer src.Close()

	origName := strings.TrimSuffix(file.Filename, ext)
	filename := fmt.Sprintf("%s_%s%s", origName, time.Now().Format("20060102_1504"), ext)
	destPath := filepath.Join("static", "import", filename)
	dst, err := os.Create(destPath)
	if err != nil {
		data.Errors = []string{"Failed to save uploaded file"}
		return shared.Render(c, http.StatusOK, vsTmpl.ImportPage(page, data))
	}
	defer dst.Close()

	if _, err = io.Copy(dst, src); err != nil {
		data.Errors = []string{"Failed to save uploaded file"}
		return shared.Render(c, http.StatusOK, vsTmpl.ImportPage(page, data))
	}

	// Count rows in Excel to set total.
	ef, err := excelize.OpenFile(destPath)
	rowCount := 0
	if err == nil {
		sheetName := ef.GetSheetName(0)
		if rows, err := ef.GetRows(sheetName); err == nil {
			rowCount = len(rows) - 1 // Subtract header row.
			if rowCount < 0 {
				rowCount = 0
			}
		}
		ef.Close()
	}

	// Get TMS session for background job.
	session := h.service.GetSession()
	if session == "" {
		_ = os.Remove(destPath)
		data.Errors = []string{"No active TMS session. Please login to Veristore first."}
		return shared.Render(c, http.StatusOK, vsTmpl.ImportPage(page, data))
	}

	// Create import record for progress tracking.
	current := "0"
	total := strconv.Itoa(rowCount)
	imp := &admin.Import{
		ImpCodeID:   "CSI",
		ImpFilename: filename,
		ImpCurrent:  &current,
		ImpTotal:    &total,
	}
	if err := h.adminRepo.CreateImport(imp); err != nil {
		_ = os.Remove(destPath)
		data.Errors = []string{fmt.Sprintf("Failed to create import record: %v", err)}
		return shared.Render(c, http.StatusOK, vsTmpl.ImportPage(page, data))
	}

	// Get current user.
	user := mw.GetCurrentUserName(c)

	// Enqueue background import job.
	payload := map[string]interface{}{
		"file_path": destPath,
		"session":   session,
		"user":      user,
		"import_id": imp.ImpID,
	}
	payloadBytes, _ := json.Marshal(payload)
	task := asynq.NewTask("import:terminal", payloadBytes)
	if _, err := h.queueClient.Enqueue(task, asynq.Timeout(5*time.Hour), asynq.MaxRetry(0)); err != nil {
		_ = os.Remove(destPath)
		data.Errors = []string{fmt.Sprintf("Failed to enqueue import job: %v", err)}
		return shared.Render(c, http.StatusOK, vsTmpl.ImportPage(page, data))
	}

	mw.LogActivityFromContext(c, mw.LogVeristoreImportCSI, "Import CSI from "+file.Filename)

	// Redirect to GET so the browser doesn't re-submit the form on refresh.
	return c.Redirect(http.StatusFound, "/veristore/import")
}

// ImportReset handles GET /veristore/importreset - Remove stuck/incomplete imports.
func (h *Handler) ImportReset(c echo.Context) error {
	_ = h.adminRepo.DeleteIncompleteImports()
	return c.Redirect(http.StatusFound, "/veristore/import")
}

// ImportFormat handles GET /veristore/import-format - Download import template.
// Dynamically generates an Excel template with headers and reference sheets (like v2).
func (h *Handler) ImportFormat(c echo.Context) error {
	f := excelize.NewFile()

	// -- CSI sheet (main data sheet) --
	csiSheet := "CSI"
	f.SetSheetName("Sheet1", csiSheet)

	csiHeaders := []string{
		"No", "Template", "CSI", "Profil Merchant", "Group Merchant",
		"Nama Merchant", "Alamat 1", "Alamat 2", "Alamat 3", "Alamat 4",
		"TID Reguler Debit/Credit 1", "MID Reguler Debit/Credit 1",
		"TID Reguler Debit/Credit 2", "MID Reguler Debit/Credit 2",
		"TID Ciltap 3", "MID Ciltap 3", "Plan Code Ciltap 3",
		"TID Ciltap 6", "MID Ciltap 6", "Plan Code Ciltap 6",
		"TID Ciltap 9", "MID Ciltap 9", "Plan Code Ciltap 9",
		"TID Ciltap 12", "MID Ciltap 12", "Plan Code Ciltap 12",
		"TID Ciltap 18", "MID Ciltap 18", "Plan Code Ciltap 18",
		"TID Ciltap 24", "MID Ciltap 24", "Plan Code Ciltap 24",
		"TID Ciltap 36", "MID Ciltap 36", "Plan Code Ciltap 36",
		"TID QR", "MID QR",
	}

	headerStyle, _ := f.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Bold: true, Color: "000000"},
		Fill:      excelize.Fill{Type: "pattern", Pattern: 1, Color: []string{"FFE699"}},
		Alignment: &excelize.Alignment{Horizontal: "center"},
		Border: []excelize.Border{
			{Type: "left", Color: "000000", Style: 1},
			{Type: "right", Color: "000000", Style: 1},
			{Type: "top", Color: "000000", Style: 1},
			{Type: "bottom", Color: "000000", Style: 1},
		},
	})

	for i, header := range csiHeaders {
		cell, _ := excelize.CoordinatesToCellName(i+1, 1)
		f.SetCellValue(csiSheet, cell, header)
		f.SetCellStyle(csiSheet, cell, cell, headerStyle)
	}

	// Set column widths for readability.
	f.SetColWidth(csiSheet, "A", "A", 5)
	f.SetColWidth(csiSheet, "B", "B", 20)
	f.SetColWidth(csiSheet, "C", "C", 15)
	f.SetColWidth(csiSheet, "D", "E", 25)
	f.SetColWidth(csiSheet, "F", "F", 30)
	f.SetColWidth(csiSheet, "G", "J", 25)
	f.SetColWidth(csiSheet, "K", "AK", 22)

	// -- Reference sheets --
	refHeaderStyle, _ := f.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Bold: true, Color: "000000"},
		Fill:      excelize.Fill{Type: "pattern", Pattern: 1, Color: []string{"9BC2E6"}},
		Alignment: &excelize.Alignment{Horizontal: "center"},
		Border: []excelize.Border{
			{Type: "left", Color: "000000", Style: 1},
			{Type: "right", Color: "000000", Style: 1},
			{Type: "top", Color: "000000", Style: 1},
			{Type: "bottom", Color: "000000", Style: 1},
		},
	})

	// Template reference sheet — populated from TMS terminals with CSI containing "xTMP" (like v2).
	tplSheet := "Template"
	f.NewSheet(tplSheet)
	f.SetCellValue(tplSheet, "A1", "Id")
	f.SetCellValue(tplSheet, "B1", "Template")
	f.SetCellStyle(tplSheet, "A1", "B1", refHeaderStyle)
	f.SetColWidth(tplSheet, "A", "A", 25)
	f.SetColWidth(tplSheet, "B", "B", 25)

	templates := h.loadTemplates(mw.GetCurrentUserName(c))
	for i, tpl := range templates {
		row := i + 2
		cellA, _ := excelize.CoordinatesToCellName(1, row)
		cellB, _ := excelize.CoordinatesToCellName(2, row)
		f.SetCellValue(tplSheet, cellA, tpl)
		f.SetCellValue(tplSheet, cellB, tpl)
	}

	// Merchant reference sheet.
	merchSheet := "Profil Merchant"
	f.NewSheet(merchSheet)
	f.SetCellValue(merchSheet, "A1", "Id")
	f.SetCellValue(merchSheet, "B1", "Merchant")
	f.SetCellStyle(merchSheet, "A1", "B1", refHeaderStyle)
	f.SetColWidth(merchSheet, "A", "A", 25)
	f.SetColWidth(merchSheet, "B", "B", 40)

	merchants := h.loadMerchants()
	for i, m := range merchants {
		row := i + 2
		cellA, _ := excelize.CoordinatesToCellName(1, row)
		cellB, _ := excelize.CoordinatesToCellName(2, row)
		f.SetCellValue(merchSheet, cellA, toString(m["id"]))
		f.SetCellValue(merchSheet, cellB, toString(m["name"]))
	}

	// Group reference sheet.
	groupSheet := "Group Merchant"
	f.NewSheet(groupSheet)
	f.SetCellValue(groupSheet, "A1", "Id")
	f.SetCellValue(groupSheet, "B1", "Group")
	f.SetCellStyle(groupSheet, "A1", "B1", refHeaderStyle)
	f.SetColWidth(groupSheet, "A", "A", 25)
	f.SetColWidth(groupSheet, "B", "B", 40)

	groups := h.loadGroups()
	for i, g := range groups {
		row := i + 2
		cellA, _ := excelize.CoordinatesToCellName(1, row)
		cellB, _ := excelize.CoordinatesToCellName(2, row)
		f.SetCellValue(groupSheet, cellA, toString(g["id"]))
		f.SetCellValue(groupSheet, cellB, toString(g["name"]))
	}

	// Write to buffer and send.
	var buf bytes.Buffer
	if err := f.Write(&buf); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to generate template")
	}

	c.Response().Header().Set("Content-Disposition", `attachment; filename="import_format_csi.xlsx"`)
	return c.Blob(http.StatusOK, "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet", buf.Bytes())
}

// ImportResult handles GET /veristore/import-result - Download import result file.
func (h *Handler) ImportResult(c echo.Context) error {
	// If ?name= param is provided, serve TMS report (legacy behaviour).
	if name := c.QueryParam("name"); name != "" {
		report, err := h.service.repo.GetReport(name)
		if err != nil || report == nil {
			return echo.NewHTTPError(http.StatusNotFound, "import result not found")
		}
		c.Response().Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s.xlsx"`, name))
		return c.Blob(http.StatusOK, "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet", report.TmsRptFile)
	}

	// Otherwise serve the per-row import result .txt file.
	latest, err := h.adminRepo.FindLatestImport()
	if err != nil || latest == nil {
		return echo.NewHTTPError(http.StatusNotFound, "no import record found")
	}

	resultName := fmt.Sprintf("import_result_%d.txt", latest.ImpID)
	resultPath := filepath.Join("static", "import", resultName)

	if _, err := os.Stat(resultPath); os.IsNotExist(err) {
		return echo.NewHTTPError(http.StatusNotFound, "import result file not found")
	}

	return c.Attachment(resultPath, resultName)
}

// ImportFormatMerchant handles GET /veristore/import-format-merchant - Download merchant import template.
func (h *Handler) ImportFormatMerchant(c echo.Context) error {
	filePath := "static/import_format_merchant.xlsx"
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return c.String(http.StatusNotFound, "Format file not found")
	}
	return c.Attachment(filePath, "import_format_merchant.xlsx")
}

// ImportResultMerchant handles GET /veristore/import-result-merchant - Download merchant import result file.
func (h *Handler) ImportResultMerchant(c echo.Context) error {
	latest, err := h.adminRepo.FindLatestMerchantImport()
	if err != nil || latest == nil {
		return echo.NewHTTPError(http.StatusNotFound, "no merchant import record found")
	}

	base := strings.TrimSuffix(latest.ImpFilename, filepath.Ext(latest.ImpFilename))
	resultName := "import_result_" + base + ".txt"
	resultPath := filepath.Join("static", "import", resultName)

	if _, err := os.Stat(resultPath); os.IsNotExist(err) {
		return echo.NewHTTPError(http.StatusNotFound, "merchant import result file not found")
	}

	return c.Attachment(resultPath, resultName)
}

// ImportMerchant handles GET/POST /veristore/import-merchant - Import merchants from Excel.
func (h *Handler) ImportMerchant(c echo.Context) error {
	page := h.pageData(c, "Import Merchants")
	data := vsTmpl.ImportMerchantData{}

	if c.Request().Method == http.MethodGet {
		// Check for in-progress merchant import.
		inProgress, _ := h.adminRepo.FindInProgressMerchantImport()
		if inProgress != nil {
			data.InProgress = true
			data.RequestDate = inProgress.ImpFilename
			var prog struct {
				Current string
				Total   string
			}
			h.service.db.Raw("SELECT COALESCE(imp_cur_row,'0') as current, COALESCE(imp_total_row,'0') as total FROM `import` WHERE imp_id = ?", inProgress.ImpID).Scan(&prog)
			data.Progress = prog.Current + " / " + prog.Total
			return shared.Render(c, http.StatusOK, vsTmpl.ImportMerchantPage(page, data))
		}

		// Check for completed merchant import result notification.
		if resultMsg := h.adminRepo.PopMerchantImportResult(); resultMsg != "" {
			data.ResultMessage = resultMsg
			parts := strings.Split(resultMsg, "|")
			if len(parts) == 4 {
				data.ResultPrefix = parts[0]
				data.ResultSuccess = parts[1]
				data.ResultFail = parts[2]
				data.ResultSuffix = parts[3]
			} else {
				data.ResultPrefix = resultMsg
			}
			// Check if a per-row result file exists.
			matches, _ := filepath.Glob(filepath.Join("static", "import", "import_result_mch_*.txt"))
			data.ResultFileExists = len(matches) > 0
		}

		return shared.Render(c, http.StatusOK, vsTmpl.ImportMerchantPage(page, data))
	}

	// POST - handle file upload.
	file, err := c.FormFile("file")
	if err != nil {
		data.Errors = []string{"Please select a file to upload"}
		return shared.Render(c, http.StatusOK, vsTmpl.ImportMerchantPage(page, data))
	}

	// Save uploaded file.
	src, err := file.Open()
	if err != nil {
		data.Errors = []string{"Failed to open uploaded file"}
		return shared.Render(c, http.StatusOK, vsTmpl.ImportMerchantPage(page, data))
	}
	defer src.Close()

	filename := fmt.Sprintf("mch_%s.xlsx", time.Now().Format("20060102_1504"))
	destPath := filepath.Join("static", "import", filename)
	dst, err := os.Create(destPath)
	if err != nil {
		data.Errors = []string{fmt.Sprintf("Failed to save file: %v", err)}
		return shared.Render(c, http.StatusOK, vsTmpl.ImportMerchantPage(page, data))
	}
	if _, err = io.Copy(dst, src); err != nil {
		dst.Close()
		_ = os.Remove(destPath)
		data.Errors = []string{fmt.Sprintf("Failed to save file: %v", err)}
		return shared.Render(c, http.StatusOK, vsTmpl.ImportMerchantPage(page, data))
	}
	dst.Close()

	// Count rows for progress tracking.
	ef, err := excelize.OpenFile(destPath)
	if err != nil {
		_ = os.Remove(destPath)
		data.Errors = []string{"Failed to read Excel file: " + err.Error()}
		return shared.Render(c, http.StatusOK, vsTmpl.ImportMerchantPage(page, data))
	}
	sheetName := ef.GetSheetName(0)
	allRows, _ := ef.GetRows(sheetName)
	ef.Close()
	rowCount := len(allRows) - 1 // exclude header
	if rowCount < 1 {
		_ = os.Remove(destPath)
		data.Errors = []string{"Excel file has no data rows (only header)"}
		return shared.Render(c, http.StatusOK, vsTmpl.ImportMerchantPage(page, data))
	}

	// Get TMS session for background job.
	session := h.service.GetSession()
	if session == "" {
		_ = os.Remove(destPath)
		data.Errors = []string{"No active TMS session. Please login to Veristore first."}
		return shared.Render(c, http.StatusOK, vsTmpl.ImportMerchantPage(page, data))
	}

	// Create import record for progress tracking.
	current := "0"
	total := strconv.Itoa(rowCount)
	imp := &admin.Import{
		ImpCodeID:   "MCH",
		ImpFilename: filename,
		ImpCurrent:  &current,
		ImpTotal:    &total,
	}
	if err := h.adminRepo.CreateImport(imp); err != nil {
		_ = os.Remove(destPath)
		data.Errors = []string{fmt.Sprintf("Failed to create import record: %v", err)}
		return shared.Render(c, http.StatusOK, vsTmpl.ImportMerchantPage(page, data))
	}

	// Get current user.
	user := mw.GetCurrentUserName(c)

	// Enqueue background import job.
	payload := map[string]interface{}{
		"file_path":  destPath,
		"session":    session,
		"user":       user,
		"country_id": 5, // Indonesia (matches v2's appCountryId)
		"import_id":  imp.ImpID,
	}
	payloadBytes, _ := json.Marshal(payload)
	task := asynq.NewTask("import:merchant", payloadBytes)
	if _, err := h.queueClient.Enqueue(task, asynq.Timeout(5*time.Hour), asynq.MaxRetry(0)); err != nil {
		_ = os.Remove(destPath)
		data.Errors = []string{fmt.Sprintf("Failed to enqueue import job: %v", err)}
		return shared.Render(c, http.StatusOK, vsTmpl.ImportMerchantPage(page, data))
	}

	mw.LogActivityFromContext(c, mw.LogVeristoreImportMerch, "Import Merchant from "+file.Filename)

	// Redirect to GET so the browser doesn't re-submit the form on refresh.
	return c.Redirect(http.StatusFound, "/veristore/import-merchant")
}

// ChangeMerchant handles POST /veristore/change-merchant - AJAX change terminal merchant.
func (h *Handler) ChangeMerchant(c echo.Context) error {
	serialNum := c.FormValue("serialNum")
	merchantId := c.FormValue("merchantId")
	model := c.FormValue("model")

	if serialNum == "" || merchantId == "" {
		return c.JSON(http.StatusBadRequest, map[string]interface{}{
			"code": 1, "desc": "serialNum and merchantId are required",
		})
	}

	log.Info().Str("serialNum", serialNum).Str("merchantId", merchantId).Str("model", model).Msg("ChangeMerchant request")

	resp, err := h.service.UpdateDeviceId(serialNum, model, merchantId, nil, serialNum)
	if err != nil {
		log.Error().Err(err).Msg("ChangeMerchant UpdateDeviceId error")
		return c.JSON(http.StatusInternalServerError, map[string]interface{}{
			"code": 1, "desc": err.Error(),
		})
	}

	log.Info().Int("resultCode", resp.ResultCode).Str("desc", resp.Desc).Msg("ChangeMerchant result")
	if resp.ResultCode == 0 {
		mw.LogActivityFromContext(c, mw.LogVeristoreEditMerchTerm, "Edit merchant csi "+serialNum)
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"code": resp.ResultCode, "desc": resp.Desc,
	})
}

// ---------------------------------------------------------------------------
// Merchant Routes
// ---------------------------------------------------------------------------

// Merchant handles GET /veristore/merchant - List merchants with pagination.
func (h *Handler) Merchant(c echo.Context) error {
	if err := h.requireTmsSession(c); err != nil {
		return err
	}

	page := h.pageData(c, "Merchant Management")
	pageNum, _ := strconv.Atoi(c.QueryParam("page"))
	if pageNum < 1 {
		pageNum = 1
	}

	search := c.QueryParam("q")

	var resp *TMSResponse
	var err error

	if search != "" {
		resp, err = h.service.SearchMerchants(pageNum, search)
	} else {
		resp, err = h.service.GetMerchantManageList(pageNum)
	}

	if err != nil {
		shared.SetFlash(c, h.store, h.sessionName, shared.FlashError, fmt.Sprintf("Failed to load merchants: %v", err))
		return shared.Render(c, http.StatusOK, vsTmpl.MerchantPage(page, nil, 0, pageNum, search, components.PaginationData{}))
	}

	var merchants []map[string]interface{}
	totalPage := 0

	if resp != nil && resp.ResultCode == 0 && resp.Data != nil {
		if ml, ok := resp.Data["merchantList"].([]interface{}); ok {
			for _, m := range ml {
				if mm, ok := m.(map[string]interface{}); ok {
					merchants = append(merchants, mm)
				}
			}
		}
		if tp, ok := resp.Data["totalPage"]; ok {
			totalPage, _ = toInt(tp)
		}
	}

	pagination := components.PaginationData{
		CurrentPage: pageNum,
		TotalPages:  totalPage,
		Total:       int64(len(merchants)),
		BaseURL:     "/veristore/merchant",
		HTMXTarget:  "merchant-table-container",
	}

	if shared.IsHTMX(c) {
		return shared.Render(c, http.StatusOK, vsTmpl.MerchantTablePartial(merchants, pagination, search))
	}

	return shared.Render(c, http.StatusOK, vsTmpl.MerchantPage(page, merchants, totalPage, pageNum, search, pagination))
}

// AddMerchant handles GET/POST /veristore/merchant/add - Add merchant form.
func (h *Handler) AddMerchant(c echo.Context) error {
	if err := h.requireTmsSession(c); err != nil {
		return err
	}

	page := h.pageData(c, "Add Merchant")

	if c.Request().Method == http.MethodGet {
		countries := h.loadCountries()
		timeZones := h.loadTimeZones()
		return shared.Render(c, http.StatusOK, vsTmpl.AddMerchantPage(page, nil, countries, timeZones, false))
	}

	data := MerchantData{
		MerchantName: c.FormValue("merchantName"),
		Address:      c.FormValue("address"),
		PostCode:     c.FormValue("postCode"),
		TimeZone:     c.FormValue("timeZone"),
		Contact:      c.FormValue("contact"),
		Email:        c.FormValue("email"),
		CellPhone:    c.FormValue("cellPhone"),
		TelePhone:    c.FormValue("telePhone"),
		CountryId:    c.FormValue("countryId"),
		StateId:      c.FormValue("stateId"),
		CityId:       c.FormValue("cityId"),
		DistrictId:   c.FormValue("districtId"),
	}

	resp, err := h.service.AddMerchant(data)
	if err != nil {
		shared.SetFlash(c, h.store, h.sessionName, shared.FlashError, fmt.Sprintf("Failed to add merchant: %v", err))
		return c.Redirect(http.StatusFound, "/veristore/merchant/add")
	}

	if resp.ResultCode != 0 {
		shared.SetFlash(c, h.store, h.sessionName, shared.FlashError, fmt.Sprintf("Add merchant failed: %s", resp.Desc))
		return c.Redirect(http.StatusFound, "/veristore/merchant/add")
	}

	mw.LogActivityFromContext(c, mw.LogVeristoreAddMerch, "Add merchant "+data.MerchantName)
	shared.SetFlash(c, h.store, h.sessionName, shared.FlashSuccess, "Add Merchant berhasil!")
	return c.Redirect(http.StatusFound, "/veristore/merchant")
}

// EditMerchant handles GET/POST /veristore/merchant/edit - Edit merchant form.
func (h *Handler) EditMerchant(c echo.Context) error {
	if err := h.requireTmsSession(c); err != nil {
		return err
	}

	merchantIdStr := c.QueryParam("id")
	if merchantIdStr == "" {
		merchantIdStr = c.FormValue("id")
	}
	merchantId, _ := strconv.Atoi(merchantIdStr)

	page := h.pageData(c, "Edit Merchant")

	if c.Request().Method == http.MethodGet {
		if merchantId == 0 {
			shared.SetFlash(c, h.store, h.sessionName, shared.FlashError, "Merchant ID is required")
			return c.Redirect(http.StatusFound, "/veristore/merchant")
		}

		resp, err := h.service.GetMerchantDetail(merchantId)
		if err != nil || resp.ResultCode != 0 {
			shared.SetFlash(c, h.store, h.sessionName, shared.FlashError, "Failed to load merchant details")
			return c.Redirect(http.StatusFound, "/veristore/merchant")
		}

		countries := h.loadCountries()
		timeZones := h.loadTimeZones()
		return shared.Render(c, http.StatusOK, vsTmpl.AddMerchantPage(page, resp.Data, countries, timeZones, true))
	}

	data := MerchantData{
		ID:           c.FormValue("id"),
		MerchantName: c.FormValue("merchantName"),
		Address:      c.FormValue("address"),
		PostCode:     c.FormValue("postCode"),
		TimeZone:     c.FormValue("timeZone"),
		Contact:      c.FormValue("contact"),
		Email:        c.FormValue("email"),
		CellPhone:    c.FormValue("cellPhone"),
		TelePhone:    c.FormValue("telePhone"),
		CountryId:    c.FormValue("countryId"),
		StateId:      c.FormValue("stateId"),
		CityId:       c.FormValue("cityId"),
		DistrictId:   c.FormValue("districtId"),
	}

	resp, err := h.service.EditMerchant(data)
	if err != nil {
		shared.SetFlash(c, h.store, h.sessionName, shared.FlashError, fmt.Sprintf("Failed to update merchant: %v", err))
		return c.Redirect(http.StatusFound, fmt.Sprintf("/veristore/merchant/edit?id=%s", data.ID))
	}

	if resp.ResultCode != 0 {
		shared.SetFlash(c, h.store, h.sessionName, shared.FlashError, fmt.Sprintf("Update merchant failed: %s", resp.Desc))
		return c.Redirect(http.StatusFound, fmt.Sprintf("/veristore/merchant/edit?id=%s", data.ID))
	}

	mw.LogActivityFromContext(c, mw.LogVeristoreEditMerch, "Edit merchant "+data.MerchantName)
	shared.SetFlash(c, h.store, h.sessionName, shared.FlashSuccess, "Update Merchant berhasil!")
	return c.Redirect(http.StatusFound, "/veristore/merchant")
}

// DeleteMerchant handles POST /veristore/merchant/delete - Delete merchant.
func (h *Handler) DeleteMerchant(c echo.Context) error {
	if err := h.requireTmsSession(c); err != nil {
		return err
	}

	merchantId, _ := strconv.Atoi(c.FormValue("id"))
	if merchantId == 0 {
		shared.SetFlash(c, h.store, h.sessionName, shared.FlashError, "Merchant ID is required")
		return c.Redirect(http.StatusFound, "/veristore/merchant")
	}

	log.Info().Int("merchantId", merchantId).Msg("DeleteMerchant: calling TMS API")

	resp, err := h.service.DeleteMerchant(merchantId)
	if err != nil {
		log.Error().Err(err).Int("merchantId", merchantId).Msg("DeleteMerchant: API error")
		shared.SetFlash(c, h.store, h.sessionName, shared.FlashError, fmt.Sprintf("Failed to delete merchant: %v", err))
		return c.Redirect(http.StatusFound, "/veristore/merchant")
	}

	log.Info().Int("resultCode", resp.ResultCode).Str("desc", resp.Desc).Int("merchantId", merchantId).Msg("DeleteMerchant: API response")

	if resp.ResultCode != 0 {
		shared.SetFlash(c, h.store, h.sessionName, shared.FlashError, fmt.Sprintf("Delete merchant failed: %s", resp.Desc))
	} else {
		mw.LogActivityFromContext(c, mw.LogVeristoreDelMerch, fmt.Sprintf("Delete merchant %d", merchantId))
		shared.SetFlash(c, h.store, h.sessionName, shared.FlashSuccess, "Delete Merchant berhasil!")
	}

	return c.Redirect(http.StatusFound, "/veristore/merchant")
}

// ---------------------------------------------------------------------------
// Group Routes
// ---------------------------------------------------------------------------

// Group handles GET /veristore/group - List groups with pagination.
func (h *Handler) Group(c echo.Context) error {
	if err := h.requireTmsSession(c); err != nil {
		return err
	}

	page := h.pageData(c, "Group Management")
	pageNum, _ := strconv.Atoi(c.QueryParam("page"))
	if pageNum < 1 {
		pageNum = 1
	}

	search := c.QueryParam("q")

	var resp *TMSResponse
	var err error

	if search != "" {
		resp, err = h.service.SearchGroups(pageNum, search)
	} else {
		resp, err = h.service.GetGroupManageList(pageNum)
	}

	if err != nil {
		shared.SetFlash(c, h.store, h.sessionName, shared.FlashError, fmt.Sprintf("Failed to load groups: %v", err))
		return shared.Render(c, http.StatusOK, vsTmpl.GroupPage(page, nil, 0, pageNum, search, components.PaginationData{}))
	}

	var groups []map[string]interface{}
	totalPage := 0

	if resp != nil && resp.ResultCode == 0 && resp.Data != nil {
		if gl, ok := resp.Data["groupList"].([]interface{}); ok {
			for _, g := range gl {
				if gm, ok := g.(map[string]interface{}); ok {
					groups = append(groups, gm)
				}
			}
		}
		if tp, ok := resp.Data["totalPage"]; ok {
			totalPage, _ = toInt(tp)
		}
	}

	pagination := components.PaginationData{
		CurrentPage: pageNum,
		TotalPages:  totalPage,
		Total:       int64(len(groups)),
		BaseURL:     "/veristore/group",
		HTMXTarget:  "group-table-container",
	}

	if shared.IsHTMX(c) {
		return shared.Render(c, http.StatusOK, vsTmpl.GroupTablePartial(groups, pagination, search))
	}

	return shared.Render(c, http.StatusOK, vsTmpl.GroupPage(page, groups, totalPage, pageNum, search, pagination))
}

// AddGroup handles GET/POST /veristore/group/add - Add group form.
func (h *Handler) AddGroup(c echo.Context) error {
	if err := h.requireTmsSession(c); err != nil {
		return err
	}

	page := h.pageData(c, "Add Group")

	if c.Request().Method == http.MethodGet {
		return shared.Render(c, http.StatusOK, vsTmpl.AddGroupPage(page, nil, nil, false, 0, ""))
	}

	groupName := c.FormValue("groupName")
	if groupName == "" {
		return shared.Render(c, http.StatusOK, vsTmpl.AddGroupPage(page, []string{"Group name is required"}, nil, false, 0, ""))
	}

	// Parse terminal IDs.
	var terminalIDs []int
	if tids := c.Request().Form["terminalIds[]"]; len(tids) > 0 {
		for _, tid := range tids {
			if id, err := strconv.Atoi(tid); err == nil {
				terminalIDs = append(terminalIDs, id)
			}
		}
	} else if tidJSON := c.FormValue("terminalIds"); tidJSON != "" {
		var ids []int
		if err := json.Unmarshal([]byte(tidJSON), &ids); err == nil {
			terminalIDs = ids
		}
	}

	resp, err := h.service.AddGroup(groupName, terminalIDs)
	if err != nil {
		return shared.Render(c, http.StatusOK, vsTmpl.AddGroupPage(page, []string{fmt.Sprintf("Failed to add group: %v", err)}, nil, false, 0, groupName))
	}

	if resp.ResultCode != 0 {
		return shared.Render(c, http.StatusOK, vsTmpl.AddGroupPage(page, []string{fmt.Sprintf("Add group failed: %s", resp.Desc)}, nil, false, 0, groupName))
	}

	mw.LogActivityFromContext(c, mw.LogVeristoreAddGroup, "Add group "+groupName)
	shared.SetFlash(c, h.store, h.sessionName, shared.FlashSuccess, "Add Group berhasil!")
	return c.Redirect(http.StatusFound, "/veristore/group")
}

// EditGroup handles GET/POST /veristore/group/edit - Edit group.
func (h *Handler) EditGroup(c echo.Context) error {
	if err := h.requireTmsSession(c); err != nil {
		return err
	}

	groupIdStr := c.QueryParam("id")
	if groupIdStr == "" {
		groupIdStr = c.FormValue("id")
	}
	groupId, _ := strconv.Atoi(groupIdStr)
	groupName := c.QueryParam("name")
	if groupName == "" {
		groupName = c.FormValue("groupName")
	}

	page := h.pageData(c, "Edit Group")

	if c.Request().Method == http.MethodGet {
		if groupId == 0 {
			shared.SetFlash(c, h.store, h.sessionName, shared.FlashError, "Group ID is required")
			return c.Redirect(http.StatusFound, "/veristore/group")
		}

		resp, err := h.service.GetGroupDetail(groupId)
		if err != nil {
			shared.SetFlash(c, h.store, h.sessionName, shared.FlashError, fmt.Sprintf("Failed to load group details: %v", err))
			return c.Redirect(http.StatusFound, "/veristore/group")
		}

		var terminals []map[string]interface{}
		if resp.ResultCode == 0 && resp.Data != nil {
			if tl, ok := resp.Data["terminals"].([]interface{}); ok {
				for _, t := range tl {
					if m, ok := t.(map[string]interface{}); ok {
						terminals = append(terminals, m)
					}
				}
			}
			// Extract group name from detail response if not in query param.
			if groupName == "" {
				if gn, ok := resp.Data["groupName"].(string); ok && gn != "" {
					groupName = gn
				}
			}
		}

		return shared.Render(c, http.StatusOK, vsTmpl.AddGroupPage(page, nil, terminals, true, groupId, groupName))
	}

	// POST - update group.
	var newTerminalIDs []int
	if tids := c.Request().Form["terminalIds[]"]; len(tids) > 0 {
		for _, tid := range tids {
			if id, err := strconv.Atoi(tid); err == nil {
				newTerminalIDs = append(newTerminalIDs, id)
			}
		}
	} else if tidJSON := c.FormValue("terminalIds"); tidJSON != "" {
		var ids []int
		if err := json.Unmarshal([]byte(tidJSON), &ids); err == nil {
			newTerminalIDs = ids
		}
	}

	var oldTerminalIDs []int
	if otids := c.Request().Form["oldTerminalIds[]"]; len(otids) > 0 {
		for _, tid := range otids {
			if id, err := strconv.Atoi(tid); err == nil {
				oldTerminalIDs = append(oldTerminalIDs, id)
			}
		}
	} else if otidJSON := c.FormValue("oldTerminalIds"); otidJSON != "" {
		var ids []int
		if err := json.Unmarshal([]byte(otidJSON), &ids); err == nil {
			oldTerminalIDs = ids
		}
	}

	resp, err := h.service.EditGroup(groupId, groupName, newTerminalIDs, oldTerminalIDs)
	if err != nil {
		shared.SetFlash(c, h.store, h.sessionName, shared.FlashError, fmt.Sprintf("Failed to update group: %v", err))
		return c.Redirect(http.StatusFound, fmt.Sprintf("/veristore/group/edit?id=%d", groupId))
	}

	if resp.ResultCode != 0 {
		shared.SetFlash(c, h.store, h.sessionName, shared.FlashError, fmt.Sprintf("Update group failed: %s", resp.Desc))
		return c.Redirect(http.StatusFound, fmt.Sprintf("/veristore/group/edit?id=%d", groupId))
	}

	mw.LogActivityFromContext(c, mw.LogVeristoreEditGroup, "Edit group "+groupName)
	shared.SetFlash(c, h.store, h.sessionName, shared.FlashSuccess, "Update Group berhasil!")
	return c.Redirect(http.StatusFound, "/veristore/group")
}

// DeleteGroup handles POST /veristore/group/delete - Delete group.
func (h *Handler) DeleteGroup(c echo.Context) error {
	if err := h.requireTmsSession(c); err != nil {
		return err
	}

	groupId, _ := strconv.Atoi(c.FormValue("id"))
	groupName := c.FormValue("name")
	if groupId == 0 {
		shared.SetFlash(c, h.store, h.sessionName, shared.FlashError, "Group ID is required")
		return c.Redirect(http.StatusFound, "/veristore/group")
	}

	resp, err := h.service.DeleteGroup(groupId)
	if err != nil {
		shared.SetFlash(c, h.store, h.sessionName, shared.FlashError, fmt.Sprintf("Failed to delete group: %v", err))
		return c.Redirect(http.StatusFound, "/veristore/group")
	}

	logName := groupName
	if logName == "" {
		logName = fmt.Sprintf("%d", groupId)
	}

	if resp.ResultCode != 0 {
		shared.SetFlash(c, h.store, h.sessionName, shared.FlashError, fmt.Sprintf("Delete group failed: %s", resp.Desc))
	} else {
		mw.LogActivityFromContext(c, mw.LogVeristoreDelGroup, "Delete group "+logName)
		shared.SetFlash(c, h.store, h.sessionName, shared.FlashSuccess, "Delete Group berhasil!")
	}

	return c.Redirect(http.StatusFound, "/veristore/group")
}

// AddGroupTerminal handles GET /veristore/group/terminal - AJAX terminal search for group modal.
func (h *Handler) AddGroupTerminal(c echo.Context) error {
	if err := h.requireTmsSession(c); err != nil {
		return err
	}

	search := c.QueryParam("q")
	pageNum, _ := strconv.Atoi(c.QueryParam("page"))
	if pageNum < 1 {
		pageNum = 1
	}

	currentUser := mw.GetCurrentUserName(c)

	var resp *TMSResponse
	var err error
	if search != "" {
		// Search mode: use search API.
		resp, err = h.service.SearchTerminals(pageNum, search, 0, currentUser)
	} else {
		// Default: show all terminals paginated.
		resp, err = h.service.GetTerminalList(pageNum)
	}

	if err != nil {
		return c.String(http.StatusInternalServerError, fmt.Sprintf("Error: %v", err))
	}

	var terminals []map[string]interface{}
	var totalPage int
	if resp != nil && resp.ResultCode == 0 && resp.Data != nil {
		if tl, ok := resp.Data["terminalList"].([]interface{}); ok {
			for _, t := range tl {
				if m, ok := t.(map[string]interface{}); ok {
					// Map field names to match what the template expects.
					m["terminalId"] = fmt.Sprintf("%v", m["id"])
					terminals = append(terminals, m)
				}
			}
		}
		if tp, ok := resp.Data["totalPage"]; ok {
			totalPage, _ = toInt(tp)
		}
	}

	return shared.Render(c, http.StatusOK, vsTmpl.GroupTerminalSearchPartial(terminals, pageNum, totalPage, search))
}

// ---------------------------------------------------------------------------
// AJAX / API Routes
// ---------------------------------------------------------------------------

// GetOperator handles GET /veristore/operator - Get reseller dropdown (HTMX).
func (h *Handler) GetOperator(c echo.Context) error {
	username := c.QueryParam("username")
	if username == "" {
		return c.JSON(http.StatusOK, map[string]interface{}{"data": []interface{}{}})
	}

	resp, err := h.service.GetResellerList(username)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]interface{}{"error": err.Error()})
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"code": resp.ResultCode,
		"data": resp.RawData,
	})
}

// GetVerifyCode handles GET /veristore/verify-code - Get captcha image.
func (h *Handler) GetVerifyCode(c echo.Context) error {
	resp, err := h.service.GetVerifyCode()
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]interface{}{"error": err.Error()})
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"code": resp.ResultCode,
		"data": resp.Data,
	})
}

// GetModel handles GET /veristore/model - Get models by vendor.
func (h *Handler) GetModel(c echo.Context) error {
	vendorId := c.QueryParam("vendor")
	if vendorId == "" {
		return c.JSON(http.StatusOK, map[string]interface{}{"data": []interface{}{}})
	}

	resp, err := h.service.GetModelList(vendorId)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]interface{}{"error": err.Error()})
	}

	var models []interface{}
	if resp.ResultCode == 0 && resp.Data != nil {
		if ml, ok := resp.Data["models"].([]interface{}); ok {
			models = ml
		}
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"code": resp.ResultCode,
		"data": models,
	})
}

// GetState handles GET /veristore/state - Get states by country (HTMX).
func (h *Handler) GetState(c echo.Context) error {
	countryId, _ := strconv.Atoi(c.QueryParam("countryId"))
	if countryId == 0 {
		return c.HTML(http.StatusOK, `<option value="">-- Select State --</option>`)
	}

	resp, err := h.service.GetStateList(countryId)
	if err != nil {
		return c.HTML(http.StatusOK, `<option value="">-- Select State --</option>`)
	}

	html := `<option value="">-- Select State --</option>`
	if resp.ResultCode == 0 && resp.Data != nil {
		if sl, ok := resp.Data["states"].([]interface{}); ok {
			for _, s := range sl {
				if sm, ok := s.(map[string]interface{}); ok {
					id := fmt.Sprintf("%v", sm["id"])
					name := fmt.Sprintf("%v", sm["name"])
					html += fmt.Sprintf(`<option value="%s">%s</option>`, id, name)
				}
			}
		}
	}

	return c.HTML(http.StatusOK, html)
}

// GetCity handles GET /veristore/city - Get cities by state (HTMX).
func (h *Handler) GetCity(c echo.Context) error {
	stateId, _ := strconv.Atoi(c.QueryParam("stateId"))
	if stateId == 0 {
		return c.HTML(http.StatusOK, `<option value="">-- Select City --</option>`)
	}

	resp, err := h.service.GetCityList(stateId)
	if err != nil {
		return c.HTML(http.StatusOK, `<option value="">-- Select City --</option>`)
	}

	html := `<option value="">-- Select City --</option>`
	if resp.ResultCode == 0 && resp.Data != nil {
		if cl, ok := resp.Data["cities"].([]interface{}); ok {
			for _, ci := range cl {
				if cm, ok := ci.(map[string]interface{}); ok {
					id := fmt.Sprintf("%v", cm["id"])
					name := fmt.Sprintf("%v", cm["name"])
					html += fmt.Sprintf(`<option value="%s">%s</option>`, id, name)
				}
			}
		}
	}

	return c.HTML(http.StatusOK, html)
}

// GetDistrict handles GET /veristore/district - Get districts by city (HTMX).
func (h *Handler) GetDistrict(c echo.Context) error {
	cityId, _ := strconv.Atoi(c.QueryParam("cityId"))
	if cityId == 0 {
		return c.HTML(http.StatusOK, `<option value="">-- Select District --</option>`)
	}

	resp, err := h.service.GetDistrictList(cityId)
	if err != nil {
		return c.HTML(http.StatusOK, `<option value="">-- Select District --</option>`)
	}

	html := `<option value="">-- Select District --</option>`
	if resp.ResultCode == 0 && resp.Data != nil {
		if dl, ok := resp.Data["districts"].([]interface{}); ok {
			for _, d := range dl {
				if dm, ok := d.(map[string]interface{}); ok {
					id := fmt.Sprintf("%v", dm["id"])
					name := fmt.Sprintf("%v", dm["name"])
					html += fmt.Sprintf(`<option value="%s">%s</option>`, id, name)
				}
			}
		}
	}

	return c.HTML(http.StatusOK, html)
}

// ---------------------------------------------------------------------------
// TMS Login
// ---------------------------------------------------------------------------

// Login handles GET/POST /veristore/login - TMS login form with captcha.
// Like V2: username = app username, password = saved during app login.
// Both fields are readonly. Password is sent as plain text to TMS.
func (h *Handler) Login(c echo.Context) error {
	page := h.pageData(c, "TMS Login")

	currentUser := mw.GetCurrentUserName(c)
	tmsUser, tmsPassword := h.service.GetTmsCredentials(currentUser)
	passwordMask := strings.Repeat("*", len(tmsPassword))

	if c.Request().Method == http.MethodGet {
		return shared.Render(c, http.StatusOK, vsTmpl.LoginPage(page, nil, tmsUser, passwordMask))
	}

	// Use stored credentials (fields are readonly, like V2).
	token := c.FormValue("token")
	code := c.FormValue("code")
	resellerId, _ := strconv.Atoi(c.FormValue("resellerId"))

	resp, err := h.service.Login(tmsUser, tmsPassword, token, code, resellerId, currentUser)
	if err != nil {
		return shared.Render(c, http.StatusOK, vsTmpl.LoginPage(page, []string{fmt.Sprintf("Login failed: %v", err)}, tmsUser, passwordMask))
	}

	if resp.ResultCode != 0 {
		return shared.Render(c, http.StatusOK, vsTmpl.LoginPage(page, []string{fmt.Sprintf("Login failed: %s", resp.Desc)}, tmsUser, passwordMask))
	}

	mw.LogActivityFromContext(c, mw.LogVeristoreLogin, "Login TMS Veristore sebagai "+tmsUser)
	shared.SetFlash(c, h.store, h.sessionName, shared.FlashSuccess, "TMS login successful")

	// Redirect back to the page the user originally tried to access.
	redirectTo := c.QueryParam("redirect")
	if redirectTo == "" {
		redirectTo = "/veristore/terminal"
	}
	return c.Redirect(http.StatusFound, redirectTo)
}

// ---------------------------------------------------------------------------
// Helper methods
// ---------------------------------------------------------------------------

// requireTmsSession checks if the user has an active TMS session.
// Returns a redirect response to the TMS login page if not, preserving
// the original URL so the user is sent back after login.
// Returns nil if the session is valid and the handler can proceed.
func (h *Handler) requireTmsSession(c echo.Context) error {
	currentUser := mw.GetCurrentUserName(c)
	if h.service.GetUserSession(currentUser) == "" {
		redirect := c.Request().URL.String()
		return c.Redirect(http.StatusFound, "/veristore/login?redirect="+url.QueryEscape(redirect))
	}
	return nil
}

// loadVendors loads the vendor list from TMS. Returns nil on error.
func (h *Handler) loadVendors() []map[string]interface{} {
	resp, err := h.service.GetVendorList()
	if err != nil || resp.ResultCode != 0 || resp.Data == nil {
		return nil
	}
	var vendors []map[string]interface{}
	if vl, ok := resp.Data["vendors"].([]interface{}); ok {
		for _, v := range vl {
			if m, ok := v.(map[string]interface{}); ok {
				vendors = append(vendors, m)
			}
		}
	}
	return vendors
}

// MerchantsAPI handles GET /veristore/merchants-api - Returns merchant list as JSON for AJAX dropdowns.
func (h *Handler) MerchantsAPI(c echo.Context) error {
	merchants := h.loadMerchants()
	return c.JSON(http.StatusOK, map[string]interface{}{
		"merchants": merchants,
	})
}

// loadMerchants loads the merchant list from TMS, cached for 5 minutes.
func (h *Handler) loadMerchants() []map[string]interface{} {
	h.merchantCacheMu.Lock()
	defer h.merchantCacheMu.Unlock()

	if h.merchantCache != nil && time.Since(h.merchantCacheAt) < 5*time.Minute {
		return h.merchantCache
	}

	resp, err := h.service.GetMerchantList()
	if err != nil || resp.ResultCode != 0 || resp.Data == nil {
		return nil
	}
	var merchants []map[string]interface{}
	if ml, ok := resp.Data["merchants"].([]interface{}); ok {
		for _, m := range ml {
			if mm, ok := m.(map[string]interface{}); ok {
				merchants = append(merchants, mm)
			}
		}
	}
	h.merchantCache = merchants
	h.merchantCacheAt = time.Now()
	return merchants
}

// loadGroups loads the group list from TMS. Returns nil on error.
func (h *Handler) loadGroups() []map[string]interface{} {
	resp, err := h.service.GetGroupList()
	if err != nil || resp.ResultCode != 0 || resp.Data == nil {
		return nil
	}
	var groups []map[string]interface{}
	if gl, ok := resp.Data["groups"].([]interface{}); ok {
		for _, g := range gl {
			if gm, ok := g.(map[string]interface{}); ok {
				groups = append(groups, gm)
			}
		}
	}
	return groups
}

// loadTemplates loads template terminals (CSI containing "xTMP") from TMS
// across all pages. V2 uses these as the reference list in the Template sheet
// of the import format. Returns a list of deviceId strings.
func (h *Handler) loadTemplates(username string) []string {
	var templates []string
	for page := 1; ; page++ {
		resp, err := h.service.SearchTerminals(page, "xTMP", 4, username) // 4 = CSI search
		if err != nil || resp == nil || resp.ResultCode != 0 || resp.Data == nil {
			break
		}
		tl, ok := resp.Data["terminalList"].([]interface{})
		if !ok || len(tl) == 0 {
			break
		}
		for _, t := range tl {
			if m, ok := t.(map[string]interface{}); ok {
				if devId := toString(m["deviceId"]); devId != "" {
					templates = append(templates, devId)
				}
			}
		}
		totalPage := 0
		if tp, ok := resp.Data["totalPage"]; ok {
			totalPage, _ = toInt(tp)
		}
		if page >= totalPage {
			break
		}
	}
	return templates
}

// loadCountries loads the country list from TMS. Returns nil on error.
func (h *Handler) loadCountries() []map[string]interface{} {
	resp, err := h.service.GetCountryList()
	if err != nil || resp.ResultCode != 0 || resp.Data == nil {
		return nil
	}
	var countries []map[string]interface{}
	if cl, ok := resp.Data["countries"].([]interface{}); ok {
		for _, c := range cl {
			if cm, ok := c.(map[string]interface{}); ok {
				countries = append(countries, cm)
			}
		}
	}
	return countries
}

// loadTimeZones loads the time zone list from TMS. Returns nil on error.
func (h *Handler) loadTimeZones() []map[string]interface{} {
	resp, err := h.service.GetTimeZoneList()
	if err != nil || resp.ResultCode != 0 || resp.Data == nil {
		return nil
	}
	var timeZones []map[string]interface{}
	if tl, ok := resp.Data["timeZones"].([]interface{}); ok {
		for _, t := range tl {
			if tm, ok := t.(map[string]interface{}); ok {
				timeZones = append(timeZones, tm)
			}
		}
	}
	return timeZones
}

// loadApps loads apps from TMS filtered by the configured package name (like v2).
// Returns a list with id and displayName ("name - version") for the dropdown.
func (h *Handler) loadApps() []map[string]interface{} {
	resp, err := h.service.GetAppList()
	if err != nil || resp.ResultCode != 0 || resp.Data == nil {
		return nil
	}
	var apps []map[string]interface{}
	if al, ok := resp.Data["allApps"].([]interface{}); ok {
		for _, a := range al {
			if am, ok := a.(map[string]interface{}); ok {
				pkgName := fmt.Sprintf("%v", am["packageName"])
				if h.packageName != "" && pkgName != h.packageName {
					continue
				}
				name := fmt.Sprintf("%v", am["name"])
				version := fmt.Sprintf("%v", am["version"])
				displayName := name
				if version != "" {
					displayName = name + " - " + version
				}
				apps = append(apps, map[string]interface{}{
					"id":          fmt.Sprintf("%v", am["id"]),
					"displayName": displayName,
				})
			}
		}
	}
	return apps
}

// compareAppVersions compares two dot-separated version strings (e.g. "4.3.0.0" vs "4.2.1.2").
// Returns >0 if a > b, <0 if a < b, 0 if equal.
func compareAppVersions(a, b string) int {
	aParts := strings.Split(a, ".")
	bParts := strings.Split(b, ".")
	maxLen := len(aParts)
	if len(bParts) > maxLen {
		maxLen = len(bParts)
	}
	for i := 0; i < maxLen; i++ {
		var av, bv int
		if i < len(aParts) {
			av, _ = strconv.Atoi(aParts[i])
		}
		if i < len(bParts) {
			bv, _ = strconv.Atoi(bParts[i])
		}
		if av != bv {
			return av - bv
		}
	}
	return 0
}
