package csi

import (
	"bytes"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/gorilla/sessions"
	"github.com/labstack/echo/v4"
	"github.com/xuri/excelize/v2"
	mw "github.com/verifone/veristoretools3/internal/middleware"
	"github.com/verifone/veristoretools3/internal/shared"
	"github.com/verifone/veristoretools3/templates/components"
	"github.com/verifone/veristoretools3/templates/layouts"
	reportTmpl "github.com/verifone/veristoretools3/templates/verificationreport"
)

// ReportHandler holds dependencies for verification report HTTP handlers.
type ReportHandler struct {
	repo        *Repository
	store       sessions.Store
	sessionName string
	appName     string
	appVersion  string
}

// NewReportHandler creates a new verification report handler.
func NewReportHandler(repo *Repository, store sessions.Store, sessionName, appName, appVersion string) *ReportHandler {
	return &ReportHandler{
		repo:        repo,
		store:       store,
		sessionName: sessionName,
		appName:     appName,
		appVersion:  appVersion,
	}
}

// pageData builds a layouts.PageData from the echo context and handler config.
func (h *ReportHandler) pageData(c echo.Context, title string) layouts.PageData {
	flashes := shared.GetFlashes(c, h.store, h.sessionName)
	return layouts.PageData{
		Title:          title,
		AppName:        h.appName,
		AppVersion:     h.appVersion,
		AppIcon:        "favicon.png",
		AppLogo:        "verifone_logo.png",
		AppVeristoreLogo: "veristore_logo.png",
		UserName:       mw.GetCurrentUserName(c),
		UserFullname:   mw.GetCurrentUserFullname(c),
		UserPrivileges: mw.GetCurrentUserPrivileges(c),
		CopyrightTitle: "Verifone",
		CopyrightURL:   "https://www.verifone.com",
		Flashes:        flashes,
	}
}

// toReportData converts a VerificationReport model to a ReportData view struct.
func toReportData(r VerificationReport) reportTmpl.ReportData {
	return reportTmpl.ReportData{
		VfiRptID:                      r.VfiRptID,
		VfiRptTermDeviceID:            r.VfiRptTermDeviceID,
		VfiRptTermSerialNum:           r.VfiRptTermSerialNum,
		VfiRptTermProductNum:          r.VfiRptTermProductNum,
		VfiRptTermModel:               r.VfiRptTermModel,
		VfiRptTermAppName:             r.VfiRptTermAppName,
		VfiRptTermAppVersion:          r.VfiRptTermAppVersion,
		VfiRptTermParameter:           r.VfiRptTermParameter,
		VfiRptTermTmsCreateOperator:   r.VfiRptTermTmsCreateOperator,
		VfiRptTermTmsCreateDtOperator: r.VfiRptTermTmsCreateDtOperator.Format("2006-01-02 15:04:05"),
		VfiRptTechName:                r.VfiRptTechName,
		VfiRptTechNip:                 r.VfiRptTechNip,
		VfiRptTechNumber:              r.VfiRptTechNumber,
		VfiRptTechAddress:             r.VfiRptTechAddress,
		VfiRptTechCompany:             r.VfiRptTechCompany,
		VfiRptTechSercivePoint:        r.VfiRptTechSercivePoint,
		VfiRptTechPhone:               r.VfiRptTechPhone,
		VfiRptTechGender:              r.VfiRptTechGender,
		VfiRptTicketNo:                r.VfiRptTicketNo,
		VfiRptSpkNo:                   r.VfiRptSpkNo,
		VfiRptWorkOrder:               r.VfiRptWorkOrder,
		VfiRptRemark:                  r.VfiRptRemark,
		VfiRptStatus:                  r.VfiRptStatus,
		CreatedBy:                     r.CreatedBy,
		CreatedDt:                     r.CreatedDt.Format("2006-01-02 15:04:05"),
	}
}

// toReportDataSlice converts a slice of VerificationReport models to ReportData view structs.
func toReportDataSlice(reports []VerificationReport) []reportTmpl.ReportData {
	result := make([]reportTmpl.ReportData, len(reports))
	for i, r := range reports {
		result[i] = toReportData(r)
	}
	return result
}

// Index lists verification reports with search and pagination. Supports HTMX partial updates.
func (h *ReportHandler) Index(c echo.Context) error {
	pageNum, _ := strconv.Atoi(c.QueryParam("page"))
	if pageNum < 1 {
		pageNum = 1
	}

	filter := ReportFilter{
		DateFrom:     c.QueryParam("dateFrom"),
		DateTo:       c.QueryParam("dateTo"),
		CSI:          c.QueryParam("csi"),
		SerialNumber: c.QueryParam("serialNum"),
		EdcType:      c.QueryParam("edcType"),
		AppVersion:   c.QueryParam("appVersion"),
		Technician:   c.QueryParam("technician"),
		TMSOperator:  c.QueryParam("tmsOperator"),
		VfiOperator:  c.QueryParam("vfiOperator"),
	}

	reports, pagination, err := h.repo.SearchFiltered(filter, pageNum, 20)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to load verification reports")
	}

	// Build query string for pagination.
	var qs string
	if filter.DateFrom != "" {
		qs += "&dateFrom=" + filter.DateFrom
	}
	if filter.DateTo != "" {
		qs += "&dateTo=" + filter.DateTo
	}
	if filter.CSI != "" {
		qs += "&csi=" + filter.CSI
	}
	if filter.SerialNumber != "" {
		qs += "&serialNum=" + filter.SerialNumber
	}
	if filter.EdcType != "" {
		qs += "&edcType=" + filter.EdcType
	}
	if filter.AppVersion != "" {
		qs += "&appVersion=" + filter.AppVersion
	}
	if filter.Technician != "" {
		qs += "&technician=" + filter.Technician
	}
	if filter.TMSOperator != "" {
		qs += "&tmsOperator=" + filter.TMSOperator
	}
	if filter.VfiOperator != "" {
		qs += "&vfiOperator=" + filter.VfiOperator
	}

	paginationData := components.PaginationData{
		CurrentPage: pagination.CurrentPage,
		TotalPages:  pagination.TotalPages,
		Total:       pagination.Total,
		BaseURL:     "/verificationreport/index",
		HTMXTarget:  "report-table-container",
		QueryString: qs,
	}

	page := h.pageData(c, "Verification Reports")
	reportData := toReportDataSlice(reports)

	dropdowns := reportTmpl.ReportDropdowns{
		Models:       h.repo.GetDistinctModels(),
		AppVersions:  h.repo.GetDistinctAppVersions(),
		Technicians:  h.repo.GetDistinctTechnicians(),
		TMSOperators: h.repo.GetDistinctTMSOperators(),
		VfiOperators: h.repo.GetDistinctVfiOperators(),
	}

	filterData := reportTmpl.ReportFilterData{
		DateFrom:     filter.DateFrom,
		DateTo:       filter.DateTo,
		CSI:          filter.CSI,
		SerialNumber: filter.SerialNumber,
		EdcType:      filter.EdcType,
		AppVersion:   filter.AppVersion,
		Technician:   filter.Technician,
		TMSOperator:  filter.TMSOperator,
		VfiOperator:  filter.VfiOperator,
	}

	if shared.IsHTMX(c) {
		return shared.Render(c, http.StatusOK, reportTmpl.ReportTablePartial(reportData, paginationData, filterData, dropdowns))
	}

	return shared.Render(c, http.StatusOK, reportTmpl.IndexPage(page, reportData, paginationData, filterData, dropdowns))
}

// Export generates an Excel file with all verification reports matching the current filters.
// Matches v2's kartik-v ExportMenu columns.
func (h *ReportHandler) Export(c echo.Context) error {
	filter := ReportFilter{
		DateFrom:     c.QueryParam("dateFrom"),
		DateTo:       c.QueryParam("dateTo"),
		CSI:          c.QueryParam("csi"),
		SerialNumber: c.QueryParam("serialNum"),
		EdcType:      c.QueryParam("edcType"),
		AppVersion:   c.QueryParam("appVersion"),
		Technician:   c.QueryParam("technician"),
		TMSOperator:  c.QueryParam("tmsOperator"),
		VfiOperator:  c.QueryParam("vfiOperator"),
	}

	// Fetch ALL matching reports (no pagination).
	reports, err := h.repo.FindAllFiltered(filter)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to load reports for export")
	}

	f := excelize.NewFile()
	defer f.Close()
	sheet := "Sheet1"

	// Headers matching v2 export columns.
	headers := []string{
		"No", "CSI", "Serial Number", "Product Number", "Model",
		"Nama Aplikasi", "Versi Aplikasi", "Parameter",
		"TMS Operator", "Tanggal TMS Operator",
		"Nama Teknisi", "NIP", "ID Number (KTP) Teknisi",
		"Alamat", "Perusahaan Teknisi", "Service Point Teknisi",
		"Telepon Teknisi", "Jenis Kelamin Teknisi",
		"No SPK", "Remark", "Status",
		"Verifikasi Operator", "Tanggal Verifikasi Operator",
	}

	// Bold header style.
	boldStyle, _ := f.NewStyle(&excelize.Style{
		Font: &excelize.Font{Bold: true},
		Fill: excelize.Fill{Type: "pattern", Pattern: 1, Color: []string{"#D9E1F2"}},
		Border: []excelize.Border{
			{Type: "left", Style: 1, Color: "#000000"},
			{Type: "top", Style: 1, Color: "#000000"},
			{Type: "right", Style: 1, Color: "#000000"},
			{Type: "bottom", Style: 1, Color: "#000000"},
		},
	})

	for i, h := range headers {
		cell, _ := excelize.CoordinatesToCellName(i+1, 1)
		f.SetCellValue(sheet, cell, h)
	}
	f.SetRowStyle(sheet, 1, 1, boldStyle)

	// Data rows.
	for idx, r := range reports {
		row := idx + 2
		f.SetCellValue(sheet, cellName(1, row), idx+1)
		f.SetCellValue(sheet, cellName(2, row), r.VfiRptTermSerialNum)
		f.SetCellValue(sheet, cellName(3, row), r.VfiRptTermDeviceID)
		f.SetCellValue(sheet, cellName(4, row), r.VfiRptTermProductNum)
		f.SetCellValue(sheet, cellName(5, row), r.VfiRptTermModel)
		f.SetCellValue(sheet, cellName(6, row), r.VfiRptTermAppName)
		f.SetCellValue(sheet, cellName(7, row), r.VfiRptTermAppVersion)
		f.SetCellValue(sheet, cellName(8, row), r.VfiRptTermParameter)
		f.SetCellValue(sheet, cellName(9, row), r.VfiRptTermTmsCreateOperator)
		f.SetCellValue(sheet, cellName(10, row), r.VfiRptTermTmsCreateDtOperator.Format("2006-01-02 15:04:05"))
		f.SetCellValue(sheet, cellName(11, row), r.VfiRptTechName)
		f.SetCellValue(sheet, cellName(12, row), r.VfiRptTechNip)
		f.SetCellValue(sheet, cellName(13, row), r.VfiRptTechNumber)
		f.SetCellValue(sheet, cellName(14, row), r.VfiRptTechAddress)
		f.SetCellValue(sheet, cellName(15, row), r.VfiRptTechCompany)
		f.SetCellValue(sheet, cellName(16, row), r.VfiRptTechSercivePoint)
		f.SetCellValue(sheet, cellName(17, row), r.VfiRptTechPhone)
		f.SetCellValue(sheet, cellName(18, row), r.VfiRptTechGender)
		f.SetCellValue(sheet, cellName(19, row), r.VfiRptSpkNo)
		f.SetCellValue(sheet, cellName(20, row), r.VfiRptRemark)
		f.SetCellValue(sheet, cellName(21, row), r.VfiRptStatus)
		f.SetCellValue(sheet, cellName(22, row), r.CreatedBy)
		f.SetCellValue(sheet, cellName(23, row), r.CreatedDt.Format("2006-01-02 15:04:05"))
	}

	// Auto-width for readability.
	for i := range headers {
		colName, _ := excelize.ColumnNumberToName(i + 1)
		f.SetColWidth(sheet, colName, colName, 18)
	}

	var buf bytes.Buffer
	if err := f.Write(&buf); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to generate Excel file")
	}

	filename := fmt.Sprintf("verification_report_%s.xlsx", time.Now().Format("20060102_1504"))
	c.Response().Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filename))
	mw.LogActivityFromContext(c, mw.LogVerificationExport, fmt.Sprintf("Export %d laporan verifikasi", len(reports)))
	return c.Blob(http.StatusOK, "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet", buf.Bytes())
}

// cellName is a helper that wraps excelize.CoordinatesToCellName.
func cellName(col, row int) string {
	name, _ := excelize.CoordinatesToCellName(col, row)
	return name
}

// View displays a verification report detail by ID.
func (h *ReportHandler) View(c echo.Context) error {
	id, err := strconv.Atoi(c.QueryParam("id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid report ID")
	}

	rpt, err := h.repo.FindByID(id)
	if err != nil {
		return echo.NewHTTPError(http.StatusNotFound, "verification report not found")
	}

	page := h.pageData(c, "Verification Report Detail")
	return shared.Render(c, http.StatusOK, reportTmpl.ViewPage(page, toReportData(*rpt)))
}

// Create handles both GET (show form) and POST (process form) for creating a verification report.
func (h *ReportHandler) Create(c echo.Context) error {
	page := h.pageData(c, "Create Verification Report")

	if c.Request().Method == http.MethodGet {
		return shared.Render(c, http.StatusOK, reportTmpl.FormPage(page, reportTmpl.ReportData{}, false, nil))
	}

	// Process POST.
	rpt, errors := h.parseReportForm(c)
	if len(errors) > 0 {
		return shared.Render(c, http.StatusOK, reportTmpl.FormPage(page, toReportData(*rpt), false, errors))
	}

	rpt.CreatedBy = mw.GetCurrentUserName(c)
	rpt.CreatedDt = time.Now()

	if err := h.repo.Create(rpt); err != nil {
		return shared.Render(c, http.StatusOK, reportTmpl.FormPage(page, toReportData(*rpt), false, []string{err.Error()}))
	}

	mw.LogActivityFromContext(c, mw.LogVerificationCreate, "Create laporan verifikasi CSI "+rpt.VfiRptTermDeviceID)
	shared.SetFlash(c, h.store, h.sessionName, shared.FlashSuccess, "Verification report created successfully")
	return c.Redirect(http.StatusFound, "/verificationreport/index")
}

// Update handles both GET (show form) and POST (process form) for editing a verification report.
func (h *ReportHandler) Update(c echo.Context) error {
	id, err := strconv.Atoi(c.QueryParam("id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid report ID")
	}

	rpt, err := h.repo.FindByID(id)
	if err != nil {
		return echo.NewHTTPError(http.StatusNotFound, "verification report not found")
	}

	page := h.pageData(c, "Edit Verification Report")

	if c.Request().Method == http.MethodGet {
		return shared.Render(c, http.StatusOK, reportTmpl.FormPage(page, toReportData(*rpt), true, nil))
	}

	// Process POST.
	updated, errors := h.parseReportForm(c)
	if len(errors) > 0 {
		updated.VfiRptID = rpt.VfiRptID
		return shared.Render(c, http.StatusOK, reportTmpl.FormPage(page, toReportData(*updated), true, errors))
	}

	// Preserve original fields and update the rest.
	rpt.VfiRptTermDeviceID = updated.VfiRptTermDeviceID
	rpt.VfiRptTermSerialNum = updated.VfiRptTermSerialNum
	rpt.VfiRptTermProductNum = updated.VfiRptTermProductNum
	rpt.VfiRptTermModel = updated.VfiRptTermModel
	rpt.VfiRptTermAppName = updated.VfiRptTermAppName
	rpt.VfiRptTermAppVersion = updated.VfiRptTermAppVersion
	rpt.VfiRptTermParameter = updated.VfiRptTermParameter
	rpt.VfiRptTermTmsCreateOperator = updated.VfiRptTermTmsCreateOperator
	rpt.VfiRptTechName = updated.VfiRptTechName
	rpt.VfiRptTechNip = updated.VfiRptTechNip
	rpt.VfiRptTechNumber = updated.VfiRptTechNumber
	rpt.VfiRptTechAddress = updated.VfiRptTechAddress
	rpt.VfiRptTechCompany = updated.VfiRptTechCompany
	rpt.VfiRptTechSercivePoint = updated.VfiRptTechSercivePoint
	rpt.VfiRptTechPhone = updated.VfiRptTechPhone
	rpt.VfiRptTechGender = updated.VfiRptTechGender
	rpt.VfiRptTicketNo = updated.VfiRptTicketNo
	rpt.VfiRptSpkNo = updated.VfiRptSpkNo
	rpt.VfiRptWorkOrder = updated.VfiRptWorkOrder
	rpt.VfiRptRemark = updated.VfiRptRemark
	rpt.VfiRptStatus = updated.VfiRptStatus

	if err := h.repo.Update(rpt); err != nil {
		return shared.Render(c, http.StatusOK, reportTmpl.FormPage(page, toReportData(*rpt), true, []string{err.Error()}))
	}

	mw.LogActivityFromContext(c, mw.LogVerificationEdit, "Edit laporan verifikasi CSI "+rpt.VfiRptTermDeviceID)
	shared.SetFlash(c, h.store, h.sessionName, shared.FlashSuccess, "Verification report updated successfully")
	return c.Redirect(http.StatusFound, "/verificationreport/index")
}

// Delete removes a verification report by ID and redirects to the list.
func (h *ReportHandler) Delete(c echo.Context) error {
	id, err := strconv.Atoi(c.QueryParam("id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid report ID")
	}

	// Load the report first so we can include its CSI in the activity log.
	rpt, _ := h.repo.FindByID(id)

	if err := h.repo.Delete(id); err != nil {
		shared.SetFlash(c, h.store, h.sessionName, shared.FlashError, "Failed to delete verification report")
	} else {
		detail := fmt.Sprintf("Delete laporan verifikasi ID %d", id)
		if rpt != nil && rpt.VfiRptTermDeviceID != "" {
			detail = "Delete laporan verifikasi CSI " + rpt.VfiRptTermDeviceID
		}
		mw.LogActivityFromContext(c, mw.LogVerificationDelete, detail)
		shared.SetFlash(c, h.store, h.sessionName, shared.FlashSuccess, "Verification report deleted successfully")
	}

	return c.Redirect(http.StatusFound, "/verificationreport/index")
}

// parseReportForm extracts form values and validates required fields.
func (h *ReportHandler) parseReportForm(c echo.Context) (*VerificationReport, []string) {
	rpt := &VerificationReport{
		VfiRptTermDeviceID:          c.FormValue("device_id"),
		VfiRptTermSerialNum:         c.FormValue("serial_num"),
		VfiRptTermProductNum:        c.FormValue("product_num"),
		VfiRptTermModel:             c.FormValue("model"),
		VfiRptTermAppName:           c.FormValue("app_name"),
		VfiRptTermAppVersion:        c.FormValue("app_version"),
		VfiRptTermParameter:         c.FormValue("parameter"),
		VfiRptTermTmsCreateOperator: c.FormValue("tms_create_operator"),
		VfiRptTechName:              c.FormValue("tech_name"),
		VfiRptTechNip:               c.FormValue("tech_nip"),
		VfiRptTechNumber:            c.FormValue("tech_number"),
		VfiRptTechAddress:           c.FormValue("tech_address"),
		VfiRptTechCompany:           c.FormValue("tech_company"),
		VfiRptTechSercivePoint:      c.FormValue("tech_service_point"),
		VfiRptTechPhone:             c.FormValue("tech_phone"),
		VfiRptTechGender:            c.FormValue("tech_gender"),
		VfiRptTicketNo:              c.FormValue("ticket_no"),
		VfiRptSpkNo:                 c.FormValue("spk_no"),
		VfiRptWorkOrder:             c.FormValue("work_order"),
		VfiRptRemark:                c.FormValue("remark"),
		VfiRptStatus:                c.FormValue("status"),
	}

	// Parse TMS create date/time.
	if dtStr := c.FormValue("tms_create_dt_operator"); dtStr != "" {
		if dt, err := time.Parse("2006-01-02T15:04", dtStr); err == nil {
			rpt.VfiRptTermTmsCreateDtOperator = dt
		} else if dt, err := time.Parse("2006-01-02 15:04:05", dtStr); err == nil {
			rpt.VfiRptTermTmsCreateDtOperator = dt
		}
	}

	var errors []string
	if rpt.VfiRptTermDeviceID == "" {
		errors = append(errors, "Device ID is required")
	}
	if rpt.VfiRptTermSerialNum == "" {
		errors = append(errors, "Serial Number is required")
	}
	if rpt.VfiRptTechName == "" {
		errors = append(errors, "Technician Name is required")
	}
	if rpt.VfiRptStatus == "" {
		errors = append(errors, "Status is required")
	}

	return rpt, errors
}

// statusOptions returns the list of available report status values.
func statusOptions() []string {
	return []string{
		"PASS",
		"FAIL",
		"PENDING",
	}
}

// StatusOptions returns the list of available report status values for use by templates.
func StatusOptions() []string {
	return statusOptions()
}

// FormatID formats a report ID for display with the given prefix.
func FormatID(id int) string {
	return fmt.Sprintf("RPT-%06d", id)
}
