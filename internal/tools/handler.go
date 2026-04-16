package tools

import (
	"bufio"
	"fmt"
	"html"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/gorilla/sessions"
	"github.com/labstack/echo/v4"
	mw "github.com/verifone/veristoretools3/internal/middleware"
	"github.com/verifone/veristoretools3/internal/shared"
	"github.com/verifone/veristoretools3/templates/layouts"
	toolsTmpl "github.com/verifone/veristoretools3/templates/tools"
	"gorm.io/gorm"
)

// Handler handles HTTP requests for the tools page.
type Handler struct {
	v3DB        *gorm.DB
	v2DB        *gorm.DB
	store       sessions.Store
	sessionName string
	appName     string
	appVersion  string
}

// NewHandler creates a new tools handler.
func NewHandler(v3DB, v2DB *gorm.DB, store sessions.Store, sessionName, appName, appVersion string) *Handler {
	return &Handler{
		v3DB:        v3DB,
		v2DB:        v2DB,
		store:       store,
		sessionName: sessionName,
		appName:     appName,
		appVersion:  appVersion,
	}
}

func (h *Handler) pageData(c echo.Context, title string) layouts.PageData {
	flashes := shared.GetFlashes(c, h.store, h.sessionName)
	return layouts.PageData{
		Title:          title,
		AppName:        h.appName,
		AppVersion:     h.appVersion,
		AppIcon:        "favicon.png",
		AppLogo:        "verifone_logo.png",
		UserName:       mw.GetCurrentUserName(c),
		UserFullname:   mw.GetCurrentUserFullname(c),
		UserPrivileges: mw.GetCurrentUserPrivileges(c),
		CopyrightTitle: "Verifone",
		CopyrightURL:   "https://www.verifone.com",
		Flashes:        flashes,
	}
}

// Index renders the tools page.
func (h *Handler) Index(c echo.Context) error {
	page := h.pageData(c, "Tools")
	return shared.Render(c, http.StatusOK, toolsTmpl.ToolsPage(page, nil))
}

// SyncDatabase performs bidirectional incremental sync between v2 and v3.
func (h *Handler) SyncDatabase(c echo.Context) error {
	page := h.pageData(c, "Tools")

	if h.v2DB == nil {
		page.Flashes = map[string][]string{shared.FlashError: {"V2 database is not configured or connection failed. Check v2_database in config.yaml."}}
		return shared.Render(c, http.StatusOK, toolsTmpl.ToolsPage(page, nil))
	}

	results, err := SyncDatabases(h.v2DB, h.v3DB)
	if err != nil {
		page.Flashes = map[string][]string{shared.FlashError: {err.Error()}}
		return shared.Render(c, http.StatusOK, toolsTmpl.ToolsPage(page, nil))
	}

	// Convert to template view types.
	var views []toolsTmpl.SyncResultView
	for _, r := range results {
		views = append(views, toolsTmpl.SyncResultView{
			Table:   r.Table,
			V2ToV3:  r.V2ToV3,
			V3ToV2:  r.V3ToV2,
			Errors:  r.Errors,
			V2Count: r.V2Count,
			V3Count: r.V3Count,
		})
	}

	page.Flashes = map[string][]string{shared.FlashSuccess: {"Database sync completed successfully!"}}
	return shared.Render(c, http.StatusOK, toolsTmpl.ToolsPage(page, views))
}

// DownloadLog serves the TMS business log file as a download.
func (h *Handler) DownloadLog(c echo.Context) error {
	logPath := "/host-logs/store-use-business.log"

	if _, err := os.Stat(logPath); os.IsNotExist(err) {
		shared.SetFlash(c, h.store, h.sessionName, shared.FlashError, "Log file not found. Make sure the log path is mounted in Docker.")
		return c.Redirect(http.StatusFound, "/tools/index")
	}

	return c.Attachment(logPath, filepath.Base(logPath))
}

// ViewAppLog returns an HTML fragment with the last N hours of app log.
func (h *Handler) ViewAppLog(c echo.Context) error {
	logPath := "logs/veristoretools.log"
	hours := parseHours(c.QueryParam("hours"))
	lines, err := tailFileFiltered(logPath, 2000, hours)
	if err != nil {
		return c.HTML(http.StatusOK, fmt.Sprintf(`<div class="alert alert-warning">%s</div>`, html.EscapeString(err.Error())))
	}
	if len(lines) == 0 {
		return c.HTML(http.StatusOK, `<div class="text-muted text-center py-3">No log entries found in the selected time range.</div>`)
	}
	return c.HTML(http.StatusOK, `<pre style="max-height:500px;overflow:auto;background:#1e1e1e;color:#d4d4d4;padding:12px;border-radius:4px;font-size:12px;white-space:pre-wrap;word-break:break-all;">`+html.EscapeString(strings.Join(lines, "\n"))+`</pre>`)
}

// parseHours extracts the hours query param, defaulting to 7.
// DownloadAppLog serves the app log file as a download, filtered by hours.
func (h *Handler) DownloadAppLog(c echo.Context) error {
	logPath := "logs/veristoretools.log"
	if _, err := os.Stat(logPath); os.IsNotExist(err) {
		shared.SetFlash(c, h.store, h.sessionName, shared.FlashError, "App log file not found.")
		return c.Redirect(http.StatusFound, "/tools/index")
	}

	hours := parseHours(c.QueryParam("hours"))

	// hours=0 means download the full file as-is.
	if hours == 0 {
		return c.Attachment(logPath, "veristoretools.log")
	}

	lines, err := tailFileFiltered(logPath, 5000, hours)
	if err != nil {
		shared.SetFlash(c, h.store, h.sessionName, shared.FlashError, fmt.Sprintf("Failed to read log: %v", err))
		return c.Redirect(http.StatusFound, "/tools/index")
	}

	content := strings.Join(lines, "\n")
	filename := fmt.Sprintf("veristoretools_last_%dh.log", hours)

	c.Response().Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filename))
	c.Response().Header().Set("Content-Type", "text/plain; charset=utf-8")
	return c.String(http.StatusOK, content)
}

func parseHours(s string) int {
	if s == "" {
		return 7
	}
	h, err := strconv.Atoi(s)
	if err != nil || h < 0 {
		return 7
	}
	if h > 168 { // max 7 days
		return 168
	}
	return h // 0 = all
}

// tailFileFiltered reads the last maxLines from a file and filters to lines
// that contain a timestamp within the last `hours` hours.
// It tries common timestamp formats (RFC3339, zerolog console, and plain datetime).
func tailFileFiltered(path string, maxLines, hours int) ([]string, error) {
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("Log file not found: %s", filepath.Base(path))
		}
		return nil, fmt.Errorf("Failed to open log: %v", err)
	}
	defer f.Close()

	// Read last maxLines via a ring buffer approach.
	lines := tailLines(f, maxLines)

	if hours <= 0 || len(lines) == 0 {
		return lines, nil
	}

	cutoff := time.Now().Add(-time.Duration(hours) * time.Hour)
	var filtered []string
	for _, line := range lines {
		t := extractTimestamp(line)
		if t.IsZero() || t.After(cutoff) {
			// Include lines with no parseable timestamp (context lines) and lines within range.
			filtered = append(filtered, line)
		}
	}
	return filtered, nil
}

// tailLines reads the last n lines from an io.ReadSeeker.
func tailLines(r io.ReadSeeker, n int) []string {
	scanner := bufio.NewScanner(r)
	// Use a ring buffer to keep only the last n lines.
	ring := make([]string, 0, n)
	for scanner.Scan() {
		if len(ring) >= n {
			ring = append(ring[1:], scanner.Text())
		} else {
			ring = append(ring, scanner.Text())
		}
	}
	return ring
}

// extractTimestamp tries to parse a timestamp from the beginning of a log line.
func extractTimestamp(line string) time.Time {
	loc, _ := time.LoadLocation("Asia/Jakarta")

	// Try RFC3339 at start (zerolog JSON: {"level":"info","time":"2026-04-16T10:30:00+07:00",...})
	if idx := strings.Index(line, `"time":"`); idx >= 0 {
		start := idx + 8
		end := strings.Index(line[start:], `"`)
		if end > 0 {
			if t, err := time.Parse(time.RFC3339, line[start:start+end]); err == nil {
				return t
			}
		}
	}

	// Try zerolog console format: "2026-04-16T10:30:00+07:00 INF ..."
	if len(line) >= 25 {
		if t, err := time.Parse(time.RFC3339, line[:25]); err == nil {
			return t
		}
	}

	// Try plain datetime: "2026-04-16 10:30:00"
	if len(line) >= 19 {
		if t, err := time.ParseInLocation("2006-01-02 15:04:05", line[:19], loc); err == nil {
			return t
		}
	}

	// Try date only: "2026-04-16"
	if len(line) >= 10 {
		if t, err := time.ParseInLocation("2006-01-02", line[:10], loc); err == nil {
			return t
		}
	}

	return time.Time{}
}
