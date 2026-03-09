package csi

import (
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/gorilla/sessions"
	"github.com/labstack/echo/v4"
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

	shared.SetFlash(c, h.store, h.sessionName, shared.FlashSuccess, "Verification report updated successfully")
	return c.Redirect(http.StatusFound, "/verificationreport/index")
}

// Delete removes a verification report by ID and redirects to the list.
func (h *ReportHandler) Delete(c echo.Context) error {
	id, err := strconv.Atoi(c.QueryParam("id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid report ID")
	}

	if err := h.repo.Delete(id); err != nil {
		shared.SetFlash(c, h.store, h.sessionName, shared.FlashError, "Failed to delete verification report")
	} else {
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
