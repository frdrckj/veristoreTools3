package csi

import (
	"fmt"

	"github.com/verifone/veristoretools3/internal/admin"
	"github.com/verifone/veristoretools3/internal/terminal"
	"github.com/verifone/veristoretools3/internal/tms"
	"gorm.io/gorm"
)

// SearchResult holds the result of a terminal search by CSI.
type SearchResult struct {
	Found        bool
	Source       string // "local" or "tms"
	CSI          string
	TID          string
	MID          string
	DeviceType   string
	MerchantName string
	AppVersion   string
	AppName      string
	DeviceID     string
	ProductNum   string
	TerminalID   int
	// Parameters from local terminal_parameter table.
	Parameters []terminal.TerminalParameter
	// Terminal reference for report creation.
	Terminal *terminal.Terminal
}

// Service wraps the CSI verification repository, TMS service, and technician
// repository to provide verification-related business logic.
type Service struct {
	repo     *Repository
	termRepo *terminal.Repository
	techRepo *admin.Repository
	tmsSvc   *tms.Service
	db       *gorm.DB
}

// NewService creates a new CSI verification service.
func NewService(db *gorm.DB, tmsSvc *tms.Service) *Service {
	return &Service{
		repo:     NewRepository(db),
		termRepo: terminal.NewRepository(db),
		techRepo: admin.NewRepository(db),
		tmsSvc:   tmsSvc,
		db:       db,
	}
}

// SearchTerminal searches for a terminal by CSI (serial number).
// It first checks the local terminal table; if not found, it queries the TMS API.
func (s *Service) SearchTerminal(csi string) (*SearchResult, error) {
	// Step 1: Check local terminal table.
	terminals, err := s.termRepo.FindByCSI(csi)
	if err != nil {
		return nil, fmt.Errorf("csi: search local terminals: %w", err)
	}

	if len(terminals) > 0 {
		t := terminals[0]
		result := &SearchResult{
			Found:      true,
			Source:     "local",
			CSI:        t.TermSerialNum,
			DeviceID:   t.TermDeviceID,
			DeviceType: t.TermModel,
			AppVersion: t.TermAppVersion,
			AppName:    t.TermAppName,
			ProductNum: t.TermProductNum,
			TerminalID: t.TermID,
			Terminal:   &t,
		}

		// Load terminal parameters to get TID/MID.
		params, err := s.termRepo.FindParametersByTermID(t.TermID)
		if err == nil && len(params) > 0 {
			result.Parameters = params
			// Use the first non-empty TID/MID found.
			for _, p := range params {
				if result.TID == "" && p.ParamTID != "" {
					result.TID = p.ParamTID
				}
				if result.MID == "" && p.ParamMID != "" {
					result.MID = p.ParamMID
				}
				if result.MerchantName == "" && p.ParamMerchantName != "" {
					result.MerchantName = p.ParamMerchantName
				}
			}
		}

		return result, nil
	}

	// Step 2: Not found locally, try TMS API.
	resp, err := s.tmsSvc.SearchTerminals(1, csi, 0)
	if err != nil {
		return &SearchResult{Found: false, CSI: csi}, nil
	}

	if resp == nil || resp.ResultCode != 0 || resp.Data == nil {
		return &SearchResult{Found: false, CSI: csi}, nil
	}

	termList, ok := resp.Data["terminalList"].([]interface{})
	if !ok || len(termList) == 0 {
		return &SearchResult{Found: false, CSI: csi}, nil
	}

	// Take the first matching terminal from TMS.
	first, ok := termList[0].(map[string]interface{})
	if !ok {
		return &SearchResult{Found: false, CSI: csi}, nil
	}

	result := &SearchResult{
		Found:      true,
		Source:     "tms",
		CSI:        csi,
		DeviceID:   mapGetStr(first, "sn"),
		DeviceType: mapGetStr(first, "model"),
		AppVersion: mapGetStr(first, "appVersion"),
	}

	// Try to get merchant name from the TMS result.
	if mn := mapGetStr(first, "merchantName"); mn != "" {
		result.MerchantName = mn
	}

	return result, nil
}

// CalcVerificationPassword computes the 6-character hex activation code for
// terminal verification using the CalcActivationPassword function from the
// tms encryption module.
func (s *Service) CalcVerificationPassword(csi, tid, mid, model, version string) string {
	return tms.CalcActivationPassword(csi, tid, mid, model, version)
}

// GetTechnicians returns all active technicians (tech_status = '1').
func (s *Service) GetTechnicians() ([]admin.Technician, error) {
	var techs []admin.Technician
	if err := s.db.Where("tech_status = ?", "1").Order("tech_name ASC").Find(&techs).Error; err != nil {
		return nil, fmt.Errorf("csi: get technicians: %w", err)
	}
	return techs, nil
}

// GetTechnicianDetail retrieves a single technician by ID.
func (s *Service) GetTechnicianDetail(id int) (*admin.Technician, error) {
	return s.techRepo.FindTechnicianByID(id)
}

// CreateReport saves a new verification report to the database.
func (s *Service) CreateReport(report *VerificationReport) error {
	return s.repo.Create(report)
}

// GetDashboardStats returns counts for the dashboard: verified terminals,
// total terminals, and total verification reports.
func (s *Service) GetDashboardStats() (map[string]interface{}, error) {
	verified, err := s.repo.CountDistinctVerified()
	if err != nil {
		return nil, fmt.Errorf("csi: count verified: %w", err)
	}

	totalTerminals, err := s.termRepo.Count()
	if err != nil {
		return nil, fmt.Errorf("csi: count terminals: %w", err)
	}

	stats := map[string]interface{}{
		"verifiedTerminals": verified,
		"totalTerminals":    totalTerminals,
	}

	return stats, nil
}

// GetAppVersions returns distinct app versions from the terminal table for
// the verification search form dropdown.
func (s *Service) GetAppVersions() ([]string, error) {
	var versions []string
	if err := s.db.Model(&terminal.Terminal{}).
		Distinct("term_app_version").
		Order("term_app_version DESC").
		Pluck("term_app_version", &versions).Error; err != nil {
		return nil, fmt.Errorf("csi: get app versions: %w", err)
	}
	return versions, nil
}

// SearchTerminalWithVersion searches for a terminal by CSI and app version,
// matching the v2 behaviour that filters by both serial_num and app_version.
func (s *Service) SearchTerminalWithVersion(csi, appVersion string) (*SearchResult, error) {
	var t terminal.Terminal
	err := s.db.Where("term_serial_num = ? AND term_app_version = ?", csi, appVersion).First(&t).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return &SearchResult{Found: false, CSI: csi}, nil
		}
		return nil, fmt.Errorf("csi: search terminal with version: %w", err)
	}

	result := &SearchResult{
		Found:      true,
		Source:     "local",
		CSI:        t.TermSerialNum,
		DeviceID:   t.TermDeviceID,
		DeviceType: t.TermModel,
		AppVersion: t.TermAppVersion,
		AppName:    t.TermAppName,
		ProductNum: t.TermProductNum,
		TerminalID: t.TermID,
		Terminal:   &t,
	}

	// Load terminal parameters.
	params, err := s.termRepo.FindParametersByTermID(t.TermID)
	if err == nil && len(params) > 0 {
		result.Parameters = params
		for _, p := range params {
			if result.TID == "" && p.ParamTID != "" {
				result.TID = p.ParamTID
			}
			if result.MID == "" && p.ParamMID != "" {
				result.MID = p.ParamMID
			}
			if result.MerchantName == "" && p.ParamMerchantName != "" {
				result.MerchantName = p.ParamMerchantName
			}
		}
	}

	return result, nil
}

// UpdateTerminalDeviceID updates a terminal's device ID when the status is
// DONE and the device ID has changed (matching v2 logic).
func (s *Service) UpdateTerminalDeviceID(termID int, newDeviceID string) error {
	return s.db.Model(&terminal.Terminal{}).
		Where("term_id = ?", termID).
		Update("term_device_id", newDeviceID).Error
}

// mapGetStr safely retrieves a string value from a map[string]interface{}.
func mapGetStr(m map[string]interface{}, key string) string {
	if m == nil {
		return ""
	}
	v, ok := m[key]
	if !ok || v == nil {
		return ""
	}
	if s, ok := v.(string); ok {
		return s
	}
	return fmt.Sprintf("%v", v)
}
