package tms

import (
	"fmt"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/rs/zerolog/log"
	"gorm.io/gorm"
)

const deleteWorkerCount = 10

// GetAllTabNames returns the unique tab names from template_parameter table.
func GetAllTabNames(db *gorm.DB) []string {
	type fieldRow struct {
		F string
	}
	var rows []fieldRow
	db.Raw(`SELECT MAX(tparam_field) as f FROM template_parameter GROUP BY tparam_title ORDER BY MIN(tparam_id)`).Scan(&rows)
	var tabs []string
	seen := map[string]bool{}
	for _, r := range rows {
		parts := strings.SplitN(r.F, "-", 3)
		if len(parts) >= 2 && !seen[parts[1]] {
			tabs = append(tabs, parts[1])
			seen[parts[1]] = true
		}
	}
	return tabs
}

// AddTerminalRequest holds the parameters for adding a terminal.
type AddTerminalRequest struct {
	DeviceID   string   `json:"deviceId"`
	Vendor     string   `json:"vendor"`
	Model      string   `json:"model"`
	MerchantID string   `json:"merchantId"`
	GroupIDs   []string `json:"groupIds"`
	SN         string   `json:"sn"`
	MoveConf   int      `json:"moveConf"`
}

// Service wraps the TMS client and local database operations.
type Service struct {
	client       *Client
	repo         *Repository
	db           *gorm.DB
	resellerList []int64
}

// NewService creates a new TMS service layer.
func NewService(client *Client, db *gorm.DB, resellerList []int64) *Service {
	return &Service{
		client:       client,
		repo:         NewRepository(db),
		db:           db,
		resellerList: resellerList,
	}
}

// GetTmsCredentials returns the active TMS login username and the current
// user's decrypted TMS password (from the user table). This is used to
// pre-fill the TMS login form with readonly fields, matching v2 behavior.
func (s *Service) GetTmsCredentials(currentUsername string) (tmsUser, tmsPassword string) {
	login, err := s.repo.GetActiveLogin()
	if err == nil && login != nil && login.TmsLoginUser != nil {
		tmsUser = *login.TmsLoginUser
	}

	var row struct {
		TmsPassword *string `gorm:"column:tms_password"`
	}
	if err := s.db.Table("user").Where("user_name = ?", currentUsername).First(&row).Error; err == nil {
		if row.TmsPassword != nil && *row.TmsPassword != "" {
			decrypted, err := DecryptAES(*row.TmsPassword)
			if err == nil {
				tmsPassword = decrypted
			}
		}
	}

	return
}

// GetSession returns the active TMS session token from the tms_login table.
func (s *Service) GetSession() string {
	login, err := s.repo.GetActiveLogin()
	if err != nil || login == nil || login.TmsLoginSession == nil {
		return ""
	}
	return *login.TmsLoginSession
}

// GetUserSession returns the per-user TMS session token from the user table.
// This is used to check if a specific user has an active TMS session (like v2).
func (s *Service) GetUserSession(username string) string {
	return s.repo.GetUserTmsSession(username)
}

// ClearUserSession sets the user's tms_session to NULL, forcing re-login.
func (s *Service) ClearUserSession(username string) error {
	return s.repo.ClearUserTmsSession(username)
}

// GetTerminalList retrieves a paginated terminal list.
// Uses new signed API (no session needed).
func (s *Service) GetTerminalList(page int) (*TMSResponse, error) {
	return s.client.GetTerminalList("", page)
}

// SearchTerminals searches terminals with filters.
// queryType 0=SN, 1=Merchant, 2=Group, 3=TID, 4=CSI, 5=MID.
// Group/Merchant/TID/MID use old session-based API; CSI/SN use new signed API.
// Pass username so we can use the per-user TMS session for old API calls.
func (s *Service) SearchTerminals(page int, search string, queryType int, username string) (*TMSResponse, error) {
	// Prefer per-user session; fall back to global tms_login session.
	session := s.GetUserSession(username)
	if session == "" {
		session = s.GetSession()
	}
	return s.client.GetTerminalListSearch(session, page, search, queryType)
}

// GetTerminalDetail retrieves detailed information about a terminal.
// Uses new signed API (no session needed).
func (s *Service) GetTerminalDetail(serialNum string) (*TMSResponse, error) {
	return s.client.GetTerminalDetail("", serialNum)
}

// GetTerminalParameter retrieves terminal app parameters for all tabs.
// Uses old session-based API with optimized multi-tab batch (getIdFromSN/getOperationMark called once).
func (s *Service) GetTerminalParameter(serialNum, appId string, tabNames []string) (*TMSResponse, error) {
	session := s.GetSession()
	if session == "" {
		return nil, fmt.Errorf("no active TMS session")
	}
	return s.client.GetTerminalParameterMultiTab(session, serialNum, appId, tabNames)
}

// GetTerminalParameterTab retrieves parameters for a single tab.
// Used by the AJAX endpoint to load parameters for one sub-item.
func (s *Service) GetTerminalParameterTab(serialNum, appId, tabName string) (*TMSResponse, error) {
	session := s.GetSession()
	if session == "" {
		return nil, fmt.Errorf("no active TMS session")
	}
	return s.client.GetTerminalParameter(session, serialNum, appId, tabName)
}

// AddTerminal registers a new terminal.
// Uses old session-based API (like v2).
func (s *Service) AddTerminal(data AddTerminalRequest) (*TMSResponse, error) {
	session := s.GetSession()
	if session == "" {
		return nil, fmt.Errorf("no active TMS session")
	}
	return s.client.AddTerminal(session, data.DeviceID, data.Vendor, data.Model, data.MerchantID, data.GroupIDs, data.SN, data.MoveConf)
}

// AddParameter assigns an app to a terminal (preAdd + submit, like v2).
// Uses old session-based API.
func (s *Service) AddParameter(deviceId, appId string) (*TMSResponse, error) {
	session := s.GetSession()
	if session == "" {
		return nil, fmt.Errorf("no active TMS session")
	}
	return s.client.AddParameter(session, deviceId, appId)
}

// EditTerminal updates terminal parameters.
// Uses new signed API (no session needed).
func (s *Service) EditTerminal(serialNum string, paraList []map[string]interface{}, appId string) (*TMSResponse, error) {
	return s.client.UpdateParameter("", serialNum, paraList, appId)
}

// CopyTerminal copies configuration from one terminal to another.
func (s *Service) CopyTerminal(sourceSn, destSn string) (*TMSResponse, error) {
	session := s.GetSession()
	if session == "" {
		return nil, fmt.Errorf("no active TMS session")
	}
	return s.client.CopyTerminal(session, sourceSn, destSn)
}

// DeleteTerminals removes terminals by their serial numbers concurrently.
// Uses new signed API (no session needed).
func (s *Service) DeleteTerminals(serialNos []string) (*TMSResponse, error) {
	if len(serialNos) == 0 {
		return nil, nil
	}
	if len(serialNos) == 1 {
		return s.client.DeleteTerminal("", serialNos[0])
	}

	logger := log.With().Str("task", "delete:terminal").Logger()
	logger.Info().Int("count", len(serialNos)).Msg("starting concurrent delete")

	var successCount int64
	var failCount int64

	jobs := make(chan string, len(serialNos))
	var wg sync.WaitGroup

	workers := deleteWorkerCount
	if len(serialNos) < workers {
		workers = len(serialNos)
	}

	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for sn := range jobs {
				resp, err := s.client.DeleteTerminal("", sn)
				if err != nil {
					atomic.AddInt64(&failCount, 1)
					logger.Warn().Err(err).Str("sn", sn).Msg("delete failed")
					continue
				}
				if resp.ResultCode != 0 {
					atomic.AddInt64(&failCount, 1)
					logger.Warn().Str("sn", sn).Str("desc", resp.Desc).Msg("delete failed")
					continue
				}
				atomic.AddInt64(&successCount, 1)
			}
		}()
	}

	for _, sn := range serialNos {
		jobs <- sn
	}
	close(jobs)
	wg.Wait()

	logger.Info().Int64("success", successCount).Int64("failed", failCount).Msg("concurrent delete completed")

	if failCount > 0 && successCount == 0 {
		return &TMSResponse{ResultCode: 1, Desc: fmt.Sprintf("all %d deletes failed", failCount)}, nil
	}
	return &TMSResponse{ResultCode: 0, Desc: fmt.Sprintf("%d deleted, %d failed", successCount, failCount)}, nil
}

// ReplaceTerminal replaces a terminal's SN with a new one.
func (s *Service) ReplaceTerminal(oldSn, newSn string) (*TMSResponse, error) {
	session := s.GetSession()
	if session == "" {
		return nil, fmt.Errorf("no active TMS session")
	}
	return s.client.ReplaceTerminal(session, oldSn, newSn)
}

// GetMerchantList retrieves the merchant selector list.
func (s *Service) GetMerchantList() (*TMSResponse, error) {
	session := s.GetSession()
	if session == "" {
		return nil, fmt.Errorf("no active TMS session")
	}
	return s.client.GetMerchantList(session)
}

// GetMerchantManageList retrieves a paginated merchant list.
// Uses new signed API (no session needed).
func (s *Service) GetMerchantManageList(page int) (*TMSResponse, error) {
	return s.client.GetMerchantManageList("", page)
}

// SearchMerchants searches merchants by name.
// Uses new signed API (no session needed).
func (s *Service) SearchMerchants(page int, search string) (*TMSResponse, error) {
	return s.client.GetMerchantManageListSearch("", page, search)
}

// GetMerchantDetail retrieves detailed merchant information.
// Uses new signed API (no session needed).
func (s *Service) GetMerchantDetail(merchantId int) (*TMSResponse, error) {
	return s.client.GetMerchantManageDetail("", merchantId)
}

// AddMerchant creates a new merchant.
// Uses new signed API (no session needed).
func (s *Service) AddMerchant(data MerchantData) (*TMSResponse, error) {
	return s.client.AddMerchant("", data)
}

// EditMerchant updates an existing merchant.
// Uses new signed API (no session needed).
func (s *Service) EditMerchant(data MerchantData) (*TMSResponse, error) {
	return s.client.EditMerchant("", data)
}

// DeleteMerchant removes a merchant by ID.
// Uses new signed API (no session needed).
func (s *Service) DeleteMerchant(merchantId int) (*TMSResponse, error) {
	return s.client.DeleteMerchant("", merchantId)
}

// GetGroupList retrieves the group selector list.
func (s *Service) GetGroupList() (*TMSResponse, error) {
	session := s.GetSession()
	if session == "" {
		return nil, fmt.Errorf("no active TMS session")
	}
	return s.client.GetGroupList(session)
}

// GetGroupManageList retrieves a paginated group list.
// Uses new signed API (no session needed).
func (s *Service) GetGroupManageList(page int) (*TMSResponse, error) {
	return s.client.GetGroupManageList("", page)
}

// SearchGroups searches groups by name.
// Uses new signed API (no session needed).
func (s *Service) SearchGroups(page int, search string) (*TMSResponse, error) {
	return s.client.GetGroupManageListSearch("", page, search)
}

// GetGroupDetail retrieves group detail with terminals.
func (s *Service) GetGroupDetail(groupId int) (*TMSResponse, error) {
	session := s.GetSession()
	if session == "" {
		return nil, fmt.Errorf("no active TMS session")
	}
	return s.client.GetGroupManageTerminal(session, groupId)
}

// AddGroup creates a new terminal group.
// Uses new signed API (no session needed).
func (s *Service) AddGroup(name string, terminals []int) (*TMSResponse, error) {
	return s.client.AddGroup("", name, terminals)
}

// EditGroup updates a group's name and terminal membership.
// Uses new signed API (no session needed).
func (s *Service) EditGroup(id int, name string, newTerminals, oldTerminals []int) (*TMSResponse, error) {
	return s.client.EditGroup("", id, name, newTerminals, oldTerminals)
}

// DeleteGroup removes a group by ID.
// Uses new signed API (no session needed).
func (s *Service) DeleteGroup(groupId int) (*TMSResponse, error) {
	return s.client.DeleteGroup("", groupId)
}

// GetDashboard retrieves the TMS dashboard data.
func (s *Service) GetDashboard() (*TMSResponse, error) {
	session := s.GetSession()
	if session == "" {
		return nil, fmt.Errorf("no active TMS session")
	}
	return s.client.GetDashboard(session)
}

// GetVendorList retrieves the vendor selector list.
// Uses old session-based API (like v2).
func (s *Service) GetVendorList() (*TMSResponse, error) {
	session := s.GetSession()
	if session == "" {
		return nil, fmt.Errorf("no active TMS session")
	}
	return s.client.GetVendorList(session)
}

// GetModelList retrieves terminal models for a given vendor.
// Uses old session-based API (like v2).
func (s *Service) GetModelList(vendorId string) (*TMSResponse, error) {
	session := s.GetSession()
	if session == "" {
		return nil, fmt.Errorf("no active TMS session")
	}
	return s.client.GetModelList(session, vendorId)
}

// GetCountryList retrieves the country list.
// Uses new signed API (no session needed).
func (s *Service) GetCountryList() (*TMSResponse, error) {
	return s.client.GetCountryList("")
}

// GetStateList retrieves states by country.
// Uses new signed API (no session needed).
func (s *Service) GetStateList(countryId int) (*TMSResponse, error) {
	return s.client.GetStateList("", countryId)
}

// GetCityList retrieves cities by state.
// Uses new signed API (no session needed).
func (s *Service) GetCityList(stateId int) (*TMSResponse, error) {
	return s.client.GetCityList("", stateId)
}

// GetDistrictList retrieves districts by city.
// Uses new signed API (no session needed).
func (s *Service) GetDistrictList(cityId int) (*TMSResponse, error) {
	return s.client.GetDistrictList("", cityId)
}

// GetTimeZoneList retrieves the time zone list.
// Uses new signed API (no session needed).
func (s *Service) GetTimeZoneList() (*TMSResponse, error) {
	return s.client.GetTimeZoneList("")
}

// GetResellerList retrieves the reseller list for a username.
// When resellerList is configured, only matching resellers are returned
// (matching v2 appResellerList filtering behavior).
func (s *Service) GetResellerList(username string) (*TMSResponse, error) {
	resp, err := s.client.GetResellerList(username)
	if err != nil || resp == nil || resp.ResultCode != 0 {
		return resp, err
	}

	rawList, ok := resp.RawData.([]interface{})
	if !ok || len(s.resellerList) == 0 || len(rawList) == 0 {
		return resp, nil
	}

	// Build lookup set.
	allowed := make(map[int64]bool, len(s.resellerList))
	for _, id := range s.resellerList {
		allowed[id] = true
	}

	// Filter resellers by configured IDs.
	filtered := make([]interface{}, 0, len(rawList))
	for _, item := range rawList {
		m, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		id, _ := toInt64(m["id"])
		if allowed[id] {
			// Copy marketName to resellerName (v2 compatibility).
			if mn, ok := m["marketName"]; ok {
				m["resellerName"] = mn
			}
			filtered = append(filtered, m)
		}
	}
	resp.RawData = filtered

	return resp, nil
}

// GetVerifyCode retrieves a captcha image and token.
func (s *Service) GetVerifyCode() (*TMSResponse, error) {
	return s.client.GetVerifyCode()
}

// Login authenticates against the TMS API and saves the session.
// The currentUsername is the app user performing the login, used to store
// their per-user TMS session (matching v2 behavior).
func (s *Service) Login(username, password, token, code string, resellerId int, currentUsername string) (*TMSResponse, error) {
	resp, err := s.client.Login(username, password, token, code, resellerId)
	if err != nil {
		return nil, err
	}
	if resp.ResultCode == 0 && resp.Data != nil {
		if cookies, ok := resp.Data["cookies"].(string); ok && cookies != "" {
			// Update the system-wide TMS login session.
			login, dbErr := s.repo.GetActiveLogin()
			if dbErr == nil && login != nil {
				s.repo.UpdateSession(login.TmsLoginID, cookies)
			}
			// Also save the session to the user's own tms_session (per-user, like v2).
			if currentUsername != "" {
				s.repo.SetUserTmsSession(currentUsername, cookies)
			}
		}
	}
	return resp, nil
}

// CheckToken checks if the current TMS session is still valid.
func (s *Service) CheckToken() (*TMSResponse, error) {
	session := s.GetSession()
	if session == "" {
		return nil, fmt.Errorf("no active TMS session")
	}
	return s.client.CheckToken(session)
}

// GetGroupTerminalSearch searches for terminals available for group assignment.
func (s *Service) GetGroupTerminalSearch(search string) (*TMSResponse, error) {
	session := s.GetSession()
	if session == "" {
		return nil, fmt.Errorf("no active TMS session")
	}
	return s.client.GetGroupTerminalSearch(session, search)
}

// GetAppList retrieves all available apps and their versions.
// Uses old session-based API (like v2) with parallel version fetching.
func (s *Service) GetAppList() (*TMSResponse, error) {
	session := s.GetSession()
	if session == "" {
		return nil, fmt.Errorf("no active TMS session")
	}
	return s.client.GetAppList(session)
}

// GetAppListSearch retrieves apps installed on a specific terminal.
// Uses new signed API (no session needed).
func (s *Service) GetAppListSearch(serialNum string) (*TMSResponse, error) {
	return s.client.GetAppListSearch("", serialNum)
}

// UpdateDeviceId updates a terminal's device ID, model, merchant, and groups.
// Uses old session-based API (requires active TMS session).
func (s *Service) UpdateDeviceId(serialNum, model string, merchantId int, groupList []int, deviceId string) (*TMSResponse, error) {
	session := s.GetSession()
	if session == "" {
		return nil, fmt.Errorf("no active TMS session")
	}
	return s.client.UpdateDeviceId(session, serialNum, model, merchantId, groupList, deviceId)
}
