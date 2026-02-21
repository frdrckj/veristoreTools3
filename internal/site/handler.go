package site

import (
	"net/http"

	"github.com/gorilla/sessions"
	"github.com/labstack/echo/v4"
	"github.com/verifone/veristoretools3/internal/admin"
	"github.com/verifone/veristoretools3/internal/csi"
	mw "github.com/verifone/veristoretools3/internal/middleware"
	"github.com/verifone/veristoretools3/internal/shared"
	syncpkg "github.com/verifone/veristoretools3/internal/sync"
	"github.com/verifone/veristoretools3/internal/terminal"
	"github.com/verifone/veristoretools3/internal/tms"
	"github.com/verifone/veristoretools3/templates/layouts"
	siteTmpl "github.com/verifone/veristoretools3/templates/site"
)

// Handler holds dependencies for site/dashboard HTTP handlers.
type Handler struct {
	terminalRepo     *terminal.Repository
	syncRepo         *syncpkg.Repository
	csiRepo          *csi.Repository
	adminRepo        *admin.Repository
	tmsService       *tms.Service
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

// NewHandler creates a new site handler with the given dependencies.
func NewHandler(
	terminalRepo *terminal.Repository,
	syncRepo *syncpkg.Repository,
	csiRepo *csi.Repository,
	adminRepo *admin.Repository,
	tmsService *tms.Service,
	store sessions.Store,
	sessionName string,
	appName string,
	appVersion string,
) *Handler {
	return &Handler{
		terminalRepo:     terminalRepo,
		syncRepo:         syncRepo,
		csiRepo:          csiRepo,
		adminRepo:        adminRepo,
		tmsService:       tmsService,
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

// PageData builds a layouts.PageData from the echo context and handler config.
// Exported so other handlers can reuse the same layout configuration.
func (h *Handler) PageData(c echo.Context, title string) layouts.PageData {
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

// Dashboard renders the main dashboard page. The content shown depends on the
// current user's role:
//   - ADMIN, OPERATOR: CSI dashboard with terminal, verification, and technician counts
//   - TMS ADMIN, TMS SUPERVISOR, TMS OPERATOR: TMS dashboard with terminal/merchant metrics
//   - Other roles: Default generic dashboard
func (h *Handler) Dashboard(c echo.Context) error {
	page := h.PageData(c, "Dashboard")

	switch {
	case page.IsCSI():
		return h.csiDashboard(c, page)
	case page.IsTMS():
		return h.tmsDashboard(c, page)
	default:
		return shared.Render(c, http.StatusOK, siteTmpl.Dashboard(page))
	}
}

// csiDashboard fetches CSI metrics from the database and renders the CSI dashboard.
func (h *Handler) csiDashboard(c echo.Context, page layouts.PageData) error {
	// Total terminals
	totalTerminal, err := h.terminalRepo.Count()
	if err != nil {
		totalTerminal = 0
	}

	// Total verified terminals (distinct serial numbers in verification_report)
	totalVerifikasi, err := h.csiRepo.CountDistinctVerified()
	if err != nil {
		totalVerifikasi = 0
	}

	// Total active technicians
	totalTechnician, err := h.adminRepo.CountActiveTechnicians()
	if err != nil {
		totalTechnician = 0
	}

	// Last sync time
	lastSync := h.syncRepo.LastSyncTime()

	data := siteTmpl.CSIDashboardData{
		TotalTerminal:   int(totalTerminal),
		TotalVerifikasi: int(totalVerifikasi),
		TotalTechnician: int(totalTechnician),
		LastSync:        lastSync,
	}

	return shared.Render(c, http.StatusOK, siteTmpl.CSIDashboard(page, data))
}

// tmsDashboard fetches TMS metrics from the TMS API and renders the dashboard.
func (h *Handler) tmsDashboard(c echo.Context, page layouts.PageData) error {
	data := siteTmpl.TMSDashboardData{}

	if h.tmsService != nil {
		resp, err := h.tmsService.GetDashboard()
		if err == nil && resp != nil && resp.ResultCode == 0 && resp.Data != nil {
			if v, ok := resp.Data["terminalTotalNum"]; ok {
				data.TerminalTotalNum = toInt(v)
			}
			if v, ok := resp.Data["terminalActivedNum"]; ok {
				data.TerminalActivedNum = toInt(v)
			}
			if v, ok := resp.Data["appTotalNum"]; ok {
				data.AppTotalNum = toInt(v)
			}
			if v, ok := resp.Data["appDownloadsNum"]; ok {
				data.AppDownloadsNum = toInt(v)
			}
			if v, ok := resp.Data["downloadsTask"]; ok {
				data.DownloadsTasks = toInt(v)
			}
			if v, ok := resp.Data["merchTotalNum"]; ok {
				data.MerchTotalNum = toInt(v)
			}
		}
	}

	return shared.Render(c, http.StatusOK, siteTmpl.TMSDashboard(page, data))
}

// toInt converts an interface{} value to int, handling float64 and int types.
func toInt(v interface{}) int {
	switch n := v.(type) {
	case int:
		return n
	case int64:
		return int(n)
	case float64:
		return int(n)
	default:
		return 0
	}
}
