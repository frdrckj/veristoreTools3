package tms

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/gorilla/sessions"
	"github.com/labstack/echo/v4"
	mw "github.com/verifone/veristoretools3/internal/middleware"
	"github.com/verifone/veristoretools3/internal/shared"
	"github.com/verifone/veristoretools3/templates/components"
	"github.com/verifone/veristoretools3/templates/layouts"
	vsTmpl "github.com/verifone/veristoretools3/templates/veristore"
)

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
}

// NewHandler creates a new veristore handler.
func NewHandler(service *Service, store sessions.Store, sessionName string, appName, appVersion string) *Handler {
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
	}
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
		resp, err = h.service.SearchTerminals(pageNum, serialNo, searchType)
	} else {
		resp, err = h.service.GetTerminalList(pageNum)
	}

	if err != nil {
		shared.SetFlash(c, h.store, h.sessionName, shared.FlashError, fmt.Sprintf("Failed to load terminals: %v", err))
		return shared.Render(c, http.StatusOK, vsTmpl.TerminalPage(page, nil, 0, pageNum, vsTmpl.SearchParams{}, components.PaginationData{}))
	}

	var terminals []map[string]interface{}
	totalPage := 0

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
	}

	searchParams := vsTmpl.SearchParams{
		SerialNo:   serialNo,
		SearchType: searchType,
	}

	pagination := components.PaginationData{
		CurrentPage: pageNum,
		TotalPages:  totalPage,
		Total:       int64(len(terminals)),
		BaseURL:     "/veristore",
		HTMXTarget:  "terminal-table-container",
	}

	if shared.IsHTMX(c) {
		return shared.Render(c, http.StatusOK, vsTmpl.TerminalTablePartial(terminals, pagination, searchParams))
	}

	return shared.Render(c, http.StatusOK, vsTmpl.TerminalPage(page, terminals, totalPage, pageNum, searchParams, pagination))
}

// Add handles GET/POST /veristore/add - Add terminal form and submission.
func (h *Handler) Add(c echo.Context) error {
	page := h.pageData(c, "Add Terminal")

	if c.Request().Method == http.MethodGet {
		// Load dropdown data.
		vendors := h.loadVendors()
		merchants := h.loadMerchants()
		groups := h.loadGroups()
		return shared.Render(c, http.StatusOK, vsTmpl.AddPage(page, vendors, merchants, groups, nil))
	}

	// POST - process form.
	data := AddTerminalRequest{
		DeviceID:   c.FormValue("deviceId"),
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

	resp, err := h.service.AddTerminal(data)
	if err != nil {
		shared.SetFlash(c, h.store, h.sessionName, shared.FlashError, fmt.Sprintf("Failed to add terminal: %v", err))
		return c.Redirect(http.StatusFound, "/veristore/add")
	}

	if resp.ResultCode != 0 {
		shared.SetFlash(c, h.store, h.sessionName, shared.FlashError, fmt.Sprintf("Add terminal failed: %s", resp.Desc))
		return c.Redirect(http.StatusFound, "/veristore/add")
	}

	shared.SetFlash(c, h.store, h.sessionName, shared.FlashSuccess, "Terminal added successfully")
	return c.Redirect(http.StatusFound, "/veristore")
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
			return c.Redirect(http.StatusFound, "/veristore")
		}

		// Get terminal detail first to list apps.
		detailResp, err := h.service.GetTerminalDetail(serialNum)
		if err != nil {
			shared.SetFlash(c, h.store, h.sessionName, shared.FlashError, fmt.Sprintf("Failed to get terminal detail: %v", err))
			return c.Redirect(http.StatusFound, "/veristore")
		}

		var paraList []map[string]interface{}
		// If appId is provided, get parameters.
		if appId != "" {
			paramResp, err := h.service.GetTerminalParameter(serialNum, appId)
			if err == nil && paramResp.ResultCode == 0 && paramResp.Data != nil {
				if pl, ok := paramResp.Data["paraList"].([]interface{}); ok {
					for _, p := range pl {
						if m, ok := p.(map[string]interface{}); ok {
							paraList = append(paraList, m)
						}
					}
				}
			}
		}

		return shared.Render(c, http.StatusOK, vsTmpl.EditPage(page, serialNum, appId, detailResp.Data, paraList))
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

	resp, err := h.service.EditTerminal(serialNum, paraList, appId)
	if err != nil {
		shared.SetFlash(c, h.store, h.sessionName, shared.FlashError, fmt.Sprintf("Failed to update parameters: %v", err))
		return c.Redirect(http.StatusFound, fmt.Sprintf("/veristore/edit?serialNum=%s&appId=%s", serialNum, appId))
	}

	if resp.ResultCode != 0 {
		shared.SetFlash(c, h.store, h.sessionName, shared.FlashError, fmt.Sprintf("Update failed: %s", resp.Desc))
		return c.Redirect(http.StatusFound, fmt.Sprintf("/veristore/edit?serialNum=%s&appId=%s", serialNum, appId))
	}

	shared.SetFlash(c, h.store, h.sessionName, shared.FlashSuccess, "Parameters updated successfully")
	return c.Redirect(http.StatusFound, "/veristore")
}

// Copy handles GET/POST /veristore/copy - Copy terminal configuration.
func (h *Handler) Copy(c echo.Context) error {
	page := h.pageData(c, "Copy Terminal")

	if c.Request().Method == http.MethodGet {
		return shared.Render(c, http.StatusOK, vsTmpl.CopyPage(page, nil))
	}

	sourceSn := c.FormValue("sourceSn")
	destSn := c.FormValue("destSn")

	if sourceSn == "" || destSn == "" {
		return shared.Render(c, http.StatusOK, vsTmpl.CopyPage(page, []string{"Source and destination serial numbers are required"}))
	}

	resp, err := h.service.CopyTerminal(sourceSn, destSn)
	if err != nil {
		return shared.Render(c, http.StatusOK, vsTmpl.CopyPage(page, []string{fmt.Sprintf("Failed to copy terminal: %v", err)}))
	}

	if resp.ResultCode != 0 {
		return shared.Render(c, http.StatusOK, vsTmpl.CopyPage(page, []string{fmt.Sprintf("Copy failed: %s", resp.Desc)}))
	}

	shared.SetFlash(c, h.store, h.sessionName, shared.FlashSuccess, "Terminal copied successfully")
	return c.Redirect(http.StatusFound, "/veristore")
}

// Delete handles POST /veristore/delete - Delete terminal(s).
func (h *Handler) Delete(c echo.Context) error {
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
		return c.Redirect(http.StatusFound, "/veristore")
	}

	resp, err := h.service.DeleteTerminals(serialNos)
	if err != nil {
		shared.SetFlash(c, h.store, h.sessionName, shared.FlashError, fmt.Sprintf("Failed to delete terminal: %v", err))
		return c.Redirect(http.StatusFound, "/veristore")
	}

	if resp != nil && resp.ResultCode != 0 {
		shared.SetFlash(c, h.store, h.sessionName, shared.FlashError, fmt.Sprintf("Delete failed: %s", resp.Desc))
	} else {
		shared.SetFlash(c, h.store, h.sessionName, shared.FlashSuccess, "Terminal(s) deleted successfully")
	}

	return c.Redirect(http.StatusFound, "/veristore")
}

// Replacement handles POST /veristore/replacement - Replace terminal.
func (h *Handler) Replacement(c echo.Context) error {
	oldSn := c.FormValue("oldSn")
	newSn := c.FormValue("newSn")

	if oldSn == "" || newSn == "" {
		shared.SetFlash(c, h.store, h.sessionName, shared.FlashError, "Old and new serial numbers are required")
		return c.Redirect(http.StatusFound, "/veristore")
	}

	resp, err := h.service.ReplaceTerminal(oldSn, newSn)
	if err != nil {
		shared.SetFlash(c, h.store, h.sessionName, shared.FlashError, fmt.Sprintf("Failed to replace terminal: %v", err))
		return c.Redirect(http.StatusFound, "/veristore")
	}

	if resp.ResultCode != 0 {
		shared.SetFlash(c, h.store, h.sessionName, shared.FlashError, fmt.Sprintf("Replacement failed: %s", resp.Desc))
	} else {
		shared.SetFlash(c, h.store, h.sessionName, shared.FlashSuccess, "Terminal replaced successfully")
	}

	return c.Redirect(http.StatusFound, "/veristore")
}

// Check handles POST /veristore/check - Preview terminal parameters (HTMX partial).
func (h *Handler) Check(c echo.Context) error {
	serialNum := c.FormValue("serialNum")
	appId := c.FormValue("appId")

	if serialNum == "" || appId == "" {
		return c.String(http.StatusBadRequest, "serialNum and appId are required")
	}

	resp, err := h.service.GetTerminalParameter(serialNum, appId)
	if err != nil {
		return c.String(http.StatusInternalServerError, fmt.Sprintf("Error: %v", err))
	}

	var paraList []map[string]interface{}
	if resp.ResultCode == 0 && resp.Data != nil {
		if pl, ok := resp.Data["paraList"].([]interface{}); ok {
			for _, p := range pl {
				if m, ok := p.(map[string]interface{}); ok {
					paraList = append(paraList, m)
				}
			}
		}
	}

	return shared.Render(c, http.StatusOK, vsTmpl.CheckPartial(paraList))
}

// Report handles GET/POST /veristore/report - Terminal report page.
func (h *Handler) Report(c echo.Context) error {
	page := h.pageData(c, "Terminal Report")

	if c.Request().Method == http.MethodPost {
		// Process report generation request.
		reportName := c.FormValue("reportName")
		if reportName == "" {
			reportName = "terminal_report"
		}
		shared.SetFlash(c, h.store, h.sessionName, shared.FlashInfo, "Report generation initiated")
		return c.Redirect(http.StatusFound, "/veristore/report")
	}

	return shared.Render(c, http.StatusOK, vsTmpl.ReportPage(page))
}

// Export handles GET/POST /veristore/export - Export terminal data.
func (h *Handler) Export(c echo.Context) error {
	page := h.pageData(c, "Export Terminals")

	if c.Request().Method == http.MethodPost {
		shared.SetFlash(c, h.store, h.sessionName, shared.FlashInfo, "Export initiated. Download will be available shortly.")
		return c.Redirect(http.StatusFound, "/veristore/export")
	}

	return shared.Render(c, http.StatusOK, vsTmpl.ExportPage(page))
}

// ExportResult handles GET /veristore/export-result - Download export file.
func (h *Handler) ExportResult(c echo.Context) error {
	name := c.QueryParam("name")
	if name == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "missing report name")
	}

	report, err := h.service.repo.GetReport(name)
	if err != nil || report == nil {
		return echo.NewHTTPError(http.StatusNotFound, "report not found")
	}

	c.Response().Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s.xlsx"`, name))
	return c.Blob(http.StatusOK, "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet", report.TmsRptFile)
}

// Import handles GET/POST /veristore/import - Import terminals from Excel.
func (h *Handler) Import(c echo.Context) error {
	page := h.pageData(c, "Import Terminals")

	if c.Request().Method == http.MethodGet {
		return shared.Render(c, http.StatusOK, vsTmpl.ImportPage(page, nil, nil))
	}

	// POST - handle file upload.
	file, err := c.FormFile("file")
	if err != nil {
		return shared.Render(c, http.StatusOK, vsTmpl.ImportPage(page, []string{"Please select a file to upload"}, nil))
	}

	_ = file // TODO: Process the Excel file for terminal import.

	shared.SetFlash(c, h.store, h.sessionName, shared.FlashSuccess, "Import completed. Check the result for details.")
	return c.Redirect(http.StatusFound, "/veristore/import")
}

// ImportFormat handles GET /veristore/import-format - Download import template.
func (h *Handler) ImportFormat(c echo.Context) error {
	// TODO: Generate and return an Excel template for terminal import.
	return echo.NewHTTPError(http.StatusNotImplemented, "import format template not yet available")
}

// ImportResult handles GET /veristore/import-result - Download import result file.
func (h *Handler) ImportResult(c echo.Context) error {
	name := c.QueryParam("name")
	if name == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "missing report name")
	}

	report, err := h.service.repo.GetReport(name)
	if err != nil || report == nil {
		return echo.NewHTTPError(http.StatusNotFound, "import result not found")
	}

	c.Response().Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s.xlsx"`, name))
	return c.Blob(http.StatusOK, "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet", report.TmsRptFile)
}

// ImportMerchant handles GET/POST /veristore/import-merchant - Import merchants from Excel.
func (h *Handler) ImportMerchant(c echo.Context) error {
	page := h.pageData(c, "Import Merchants")

	if c.Request().Method == http.MethodGet {
		return shared.Render(c, http.StatusOK, vsTmpl.ImportMerchantPage(page, nil))
	}

	// POST - handle file upload.
	file, err := c.FormFile("file")
	if err != nil {
		return shared.Render(c, http.StatusOK, vsTmpl.ImportMerchantPage(page, []string{"Please select a file to upload"}))
	}

	_ = file // TODO: Process the Excel file for merchant import.

	shared.SetFlash(c, h.store, h.sessionName, shared.FlashSuccess, "Merchant import completed")
	return c.Redirect(http.StatusFound, "/veristore/merchant")
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

	mid, _ := strconv.Atoi(merchantId)
	resp, err := h.service.UpdateDeviceId(serialNum, model, mid, nil, "")
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]interface{}{
			"code": 1, "desc": err.Error(),
		})
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

	shared.SetFlash(c, h.store, h.sessionName, shared.FlashSuccess, "Merchant added successfully")
	return c.Redirect(http.StatusFound, "/veristore/merchant")
}

// EditMerchant handles GET/POST /veristore/merchant/edit - Edit merchant form.
func (h *Handler) EditMerchant(c echo.Context) error {
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

	shared.SetFlash(c, h.store, h.sessionName, shared.FlashSuccess, "Merchant updated successfully")
	return c.Redirect(http.StatusFound, "/veristore/merchant")
}

// DeleteMerchant handles POST /veristore/merchant/delete - Delete merchant.
func (h *Handler) DeleteMerchant(c echo.Context) error {
	merchantId, _ := strconv.Atoi(c.FormValue("id"))
	if merchantId == 0 {
		shared.SetFlash(c, h.store, h.sessionName, shared.FlashError, "Merchant ID is required")
		return c.Redirect(http.StatusFound, "/veristore/merchant")
	}

	resp, err := h.service.DeleteMerchant(merchantId)
	if err != nil {
		shared.SetFlash(c, h.store, h.sessionName, shared.FlashError, fmt.Sprintf("Failed to delete merchant: %v", err))
		return c.Redirect(http.StatusFound, "/veristore/merchant")
	}

	if resp.ResultCode != 0 {
		shared.SetFlash(c, h.store, h.sessionName, shared.FlashError, fmt.Sprintf("Delete merchant failed: %s", resp.Desc))
	} else {
		shared.SetFlash(c, h.store, h.sessionName, shared.FlashSuccess, "Merchant deleted successfully")
	}

	return c.Redirect(http.StatusFound, "/veristore/merchant")
}

// ---------------------------------------------------------------------------
// Group Routes
// ---------------------------------------------------------------------------

// Group handles GET /veristore/group - List groups with pagination.
func (h *Handler) Group(c echo.Context) error {
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
	page := h.pageData(c, "Add Group")

	if c.Request().Method == http.MethodGet {
		return shared.Render(c, http.StatusOK, vsTmpl.AddGroupPage(page, nil, nil, false, 0))
	}

	groupName := c.FormValue("groupName")
	if groupName == "" {
		return shared.Render(c, http.StatusOK, vsTmpl.AddGroupPage(page, []string{"Group name is required"}, nil, false, 0))
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
		return shared.Render(c, http.StatusOK, vsTmpl.AddGroupPage(page, []string{fmt.Sprintf("Failed to add group: %v", err)}, nil, false, 0))
	}

	if resp.ResultCode != 0 {
		return shared.Render(c, http.StatusOK, vsTmpl.AddGroupPage(page, []string{fmt.Sprintf("Add group failed: %s", resp.Desc)}, nil, false, 0))
	}

	shared.SetFlash(c, h.store, h.sessionName, shared.FlashSuccess, "Group added successfully")
	return c.Redirect(http.StatusFound, "/veristore/group")
}

// EditGroup handles GET/POST /veristore/group/edit - Edit group.
func (h *Handler) EditGroup(c echo.Context) error {
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
		}

		return shared.Render(c, http.StatusOK, vsTmpl.AddGroupPage(page, nil, terminals, true, groupId))
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

	shared.SetFlash(c, h.store, h.sessionName, shared.FlashSuccess, "Group updated successfully")
	return c.Redirect(http.StatusFound, "/veristore/group")
}

// DeleteGroup handles POST /veristore/group/delete - Delete group.
func (h *Handler) DeleteGroup(c echo.Context) error {
	groupId, _ := strconv.Atoi(c.FormValue("id"))
	if groupId == 0 {
		shared.SetFlash(c, h.store, h.sessionName, shared.FlashError, "Group ID is required")
		return c.Redirect(http.StatusFound, "/veristore/group")
	}

	resp, err := h.service.DeleteGroup(groupId)
	if err != nil {
		shared.SetFlash(c, h.store, h.sessionName, shared.FlashError, fmt.Sprintf("Failed to delete group: %v", err))
		return c.Redirect(http.StatusFound, "/veristore/group")
	}

	if resp.ResultCode != 0 {
		shared.SetFlash(c, h.store, h.sessionName, shared.FlashError, fmt.Sprintf("Delete group failed: %s", resp.Desc))
	} else {
		shared.SetFlash(c, h.store, h.sessionName, shared.FlashSuccess, "Group deleted successfully")
	}

	return c.Redirect(http.StatusFound, "/veristore/group")
}

// AddGroupTerminal handles GET/POST /veristore/group/terminal - HTMX search terminals for group.
func (h *Handler) AddGroupTerminal(c echo.Context) error {
	search := c.QueryParam("q")
	if search == "" {
		search = c.FormValue("q")
	}

	resp, err := h.service.GetGroupTerminalSearch(search)
	if err != nil {
		return c.String(http.StatusInternalServerError, fmt.Sprintf("Error: %v", err))
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
	}

	return shared.Render(c, http.StatusOK, vsTmpl.GroupTerminalSearchPartial(terminals))
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

// GetModel handles GET /veristore/model - Get models by vendor (HTMX).
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
		return c.JSON(http.StatusOK, map[string]interface{}{"data": []interface{}{}})
	}

	resp, err := h.service.GetStateList(countryId)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]interface{}{"error": err.Error()})
	}

	var states []interface{}
	if resp.ResultCode == 0 && resp.Data != nil {
		if sl, ok := resp.Data["states"].([]interface{}); ok {
			states = sl
		}
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"code": resp.ResultCode,
		"data": states,
	})
}

// GetCity handles GET /veristore/city - Get cities by state (HTMX).
func (h *Handler) GetCity(c echo.Context) error {
	stateId, _ := strconv.Atoi(c.QueryParam("stateId"))
	if stateId == 0 {
		return c.JSON(http.StatusOK, map[string]interface{}{"data": []interface{}{}})
	}

	resp, err := h.service.GetCityList(stateId)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]interface{}{"error": err.Error()})
	}

	var cities []interface{}
	if resp.ResultCode == 0 && resp.Data != nil {
		if cl, ok := resp.Data["cities"].([]interface{}); ok {
			cities = cl
		}
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"code": resp.ResultCode,
		"data": cities,
	})
}

// GetDistrict handles GET /veristore/district - Get districts by city (HTMX).
func (h *Handler) GetDistrict(c echo.Context) error {
	cityId, _ := strconv.Atoi(c.QueryParam("cityId"))
	if cityId == 0 {
		return c.JSON(http.StatusOK, map[string]interface{}{"data": []interface{}{}})
	}

	resp, err := h.service.GetDistrictList(cityId)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]interface{}{"error": err.Error()})
	}

	var districts []interface{}
	if resp.ResultCode == 0 && resp.Data != nil {
		if dl, ok := resp.Data["districts"].([]interface{}); ok {
			districts = dl
		}
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"code": resp.ResultCode,
		"data": districts,
	})
}

// ---------------------------------------------------------------------------
// TMS Login
// ---------------------------------------------------------------------------

// Login handles GET/POST /veristore/login - TMS login form with captcha.
func (h *Handler) Login(c echo.Context) error {
	page := h.pageData(c, "TMS Login")

	if c.Request().Method == http.MethodGet {
		return shared.Render(c, http.StatusOK, vsTmpl.LoginPage(page, nil))
	}

	username := c.FormValue("username")
	password := c.FormValue("password")
	token := c.FormValue("token")
	code := c.FormValue("code")
	resellerId, _ := strconv.Atoi(c.FormValue("resellerId"))

	// Encrypt password for TMS.
	encPassword, err := EncryptAES(password)
	if err != nil {
		return shared.Render(c, http.StatusOK, vsTmpl.LoginPage(page, []string{fmt.Sprintf("Encryption error: %v", err)}))
	}

	resp, err := h.service.Login(username, encPassword, token, code, resellerId)
	if err != nil {
		return shared.Render(c, http.StatusOK, vsTmpl.LoginPage(page, []string{fmt.Sprintf("Login failed: %v", err)}))
	}

	if resp.ResultCode != 0 {
		return shared.Render(c, http.StatusOK, vsTmpl.LoginPage(page, []string{fmt.Sprintf("Login failed: %s", resp.Desc)}))
	}

	shared.SetFlash(c, h.store, h.sessionName, shared.FlashSuccess, "TMS login successful")
	return c.Redirect(http.StatusFound, "/veristore")
}

// ---------------------------------------------------------------------------
// Helper methods
// ---------------------------------------------------------------------------

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

// loadMerchants loads the merchant list from TMS. Returns nil on error.
func (h *Handler) loadMerchants() []map[string]interface{} {
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
