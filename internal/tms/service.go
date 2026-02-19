package tms

import (
	"fmt"

	"gorm.io/gorm"
)

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

// GetSession returns the active TMS session token from the tms_login table.
func (s *Service) GetSession() string {
	login, err := s.repo.GetActiveLogin()
	if err != nil || login == nil || login.TmsLoginSession == nil {
		return ""
	}
	return *login.TmsLoginSession
}

// GetTerminalList retrieves a paginated terminal list.
func (s *Service) GetTerminalList(page int) (*TMSResponse, error) {
	session := s.GetSession()
	if session == "" {
		return nil, fmt.Errorf("no active TMS session")
	}
	return s.client.GetTerminalList(session, page)
}

// SearchTerminals searches terminals with filters.
func (s *Service) SearchTerminals(page int, search string, queryType int) (*TMSResponse, error) {
	session := s.GetSession()
	if session == "" {
		return nil, fmt.Errorf("no active TMS session")
	}
	return s.client.GetTerminalListSearch(session, page, search, queryType)
}

// GetTerminalDetail retrieves detailed information about a terminal.
func (s *Service) GetTerminalDetail(serialNum string) (*TMSResponse, error) {
	session := s.GetSession()
	if session == "" {
		return nil, fmt.Errorf("no active TMS session")
	}
	return s.client.GetTerminalDetail(session, serialNum)
}

// GetTerminalParameter retrieves terminal app parameters.
func (s *Service) GetTerminalParameter(serialNum, appId string) (*TMSResponse, error) {
	session := s.GetSession()
	if session == "" {
		return nil, fmt.Errorf("no active TMS session")
	}
	return s.client.GetTerminalParameter(session, serialNum, appId)
}

// AddTerminal registers a new terminal.
func (s *Service) AddTerminal(data AddTerminalRequest) (*TMSResponse, error) {
	session := s.GetSession()
	if session == "" {
		return nil, fmt.Errorf("no active TMS session")
	}
	return s.client.AddTerminal(session, data.DeviceID, data.Vendor, data.Model, data.MerchantID, data.GroupIDs, data.SN, data.MoveConf)
}

// EditTerminal updates terminal parameters.
func (s *Service) EditTerminal(serialNum string, paraList []map[string]interface{}, appId string) (*TMSResponse, error) {
	session := s.GetSession()
	if session == "" {
		return nil, fmt.Errorf("no active TMS session")
	}
	return s.client.UpdateParameter(session, serialNum, paraList, appId)
}

// CopyTerminal copies configuration from one terminal to another.
func (s *Service) CopyTerminal(sourceSn, destSn string) (*TMSResponse, error) {
	session := s.GetSession()
	if session == "" {
		return nil, fmt.Errorf("no active TMS session")
	}
	return s.client.CopyTerminal(session, sourceSn, destSn)
}

// DeleteTerminals removes terminals by their serial numbers.
func (s *Service) DeleteTerminals(serialNos []string) (*TMSResponse, error) {
	session := s.GetSession()
	if session == "" {
		return nil, fmt.Errorf("no active TMS session")
	}
	var lastResp *TMSResponse
	for _, sn := range serialNos {
		resp, err := s.client.DeleteTerminal(session, sn)
		if err != nil {
			return nil, err
		}
		lastResp = resp
		if resp.ResultCode != 0 {
			return resp, nil
		}
	}
	return lastResp, nil
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
func (s *Service) GetMerchantManageList(page int) (*TMSResponse, error) {
	session := s.GetSession()
	if session == "" {
		return nil, fmt.Errorf("no active TMS session")
	}
	return s.client.GetMerchantManageList(session, page)
}

// SearchMerchants searches merchants by name.
func (s *Service) SearchMerchants(page int, search string) (*TMSResponse, error) {
	session := s.GetSession()
	if session == "" {
		return nil, fmt.Errorf("no active TMS session")
	}
	return s.client.GetMerchantManageListSearch(session, page, search)
}

// GetMerchantDetail retrieves detailed merchant information.
func (s *Service) GetMerchantDetail(merchantId int) (*TMSResponse, error) {
	session := s.GetSession()
	if session == "" {
		return nil, fmt.Errorf("no active TMS session")
	}
	return s.client.GetMerchantManageDetail(session, merchantId)
}

// AddMerchant creates a new merchant.
func (s *Service) AddMerchant(data MerchantData) (*TMSResponse, error) {
	session := s.GetSession()
	if session == "" {
		return nil, fmt.Errorf("no active TMS session")
	}
	return s.client.AddMerchant(session, data)
}

// EditMerchant updates an existing merchant.
func (s *Service) EditMerchant(data MerchantData) (*TMSResponse, error) {
	session := s.GetSession()
	if session == "" {
		return nil, fmt.Errorf("no active TMS session")
	}
	return s.client.EditMerchant(session, data)
}

// DeleteMerchant removes a merchant by ID.
func (s *Service) DeleteMerchant(merchantId int) (*TMSResponse, error) {
	session := s.GetSession()
	if session == "" {
		return nil, fmt.Errorf("no active TMS session")
	}
	return s.client.DeleteMerchant(session, merchantId)
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
func (s *Service) GetGroupManageList(page int) (*TMSResponse, error) {
	session := s.GetSession()
	if session == "" {
		return nil, fmt.Errorf("no active TMS session")
	}
	return s.client.GetGroupManageList(session, page)
}

// SearchGroups searches groups by name.
func (s *Service) SearchGroups(page int, search string) (*TMSResponse, error) {
	session := s.GetSession()
	if session == "" {
		return nil, fmt.Errorf("no active TMS session")
	}
	return s.client.GetGroupManageListSearch(session, page, search)
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
func (s *Service) AddGroup(name string, terminals []int) (*TMSResponse, error) {
	session := s.GetSession()
	if session == "" {
		return nil, fmt.Errorf("no active TMS session")
	}
	return s.client.AddGroup(session, name, terminals)
}

// EditGroup updates a group's name and terminal membership.
func (s *Service) EditGroup(id int, name string, newTerminals, oldTerminals []int) (*TMSResponse, error) {
	session := s.GetSession()
	if session == "" {
		return nil, fmt.Errorf("no active TMS session")
	}
	return s.client.EditGroup(session, id, name, newTerminals, oldTerminals)
}

// DeleteGroup removes a group by ID.
func (s *Service) DeleteGroup(groupId int) (*TMSResponse, error) {
	session := s.GetSession()
	if session == "" {
		return nil, fmt.Errorf("no active TMS session")
	}
	return s.client.DeleteGroup(session, groupId)
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
func (s *Service) GetVendorList() (*TMSResponse, error) {
	session := s.GetSession()
	if session == "" {
		return nil, fmt.Errorf("no active TMS session")
	}
	return s.client.GetVendorList(session)
}

// GetModelList retrieves terminal models for a given vendor.
func (s *Service) GetModelList(vendorId string) (*TMSResponse, error) {
	session := s.GetSession()
	if session == "" {
		return nil, fmt.Errorf("no active TMS session")
	}
	return s.client.GetModelList(session, vendorId)
}

// GetCountryList retrieves the country list.
func (s *Service) GetCountryList() (*TMSResponse, error) {
	session := s.GetSession()
	if session == "" {
		return nil, fmt.Errorf("no active TMS session")
	}
	return s.client.GetCountryList(session)
}

// GetStateList retrieves states by country.
func (s *Service) GetStateList(countryId int) (*TMSResponse, error) {
	session := s.GetSession()
	if session == "" {
		return nil, fmt.Errorf("no active TMS session")
	}
	return s.client.GetStateList(session, countryId)
}

// GetCityList retrieves cities by state.
func (s *Service) GetCityList(stateId int) (*TMSResponse, error) {
	session := s.GetSession()
	if session == "" {
		return nil, fmt.Errorf("no active TMS session")
	}
	return s.client.GetCityList(session, stateId)
}

// GetDistrictList retrieves districts by city.
func (s *Service) GetDistrictList(cityId int) (*TMSResponse, error) {
	session := s.GetSession()
	if session == "" {
		return nil, fmt.Errorf("no active TMS session")
	}
	return s.client.GetDistrictList(session, cityId)
}

// GetTimeZoneList retrieves the time zone list.
func (s *Service) GetTimeZoneList() (*TMSResponse, error) {
	session := s.GetSession()
	if session == "" {
		return nil, fmt.Errorf("no active TMS session")
	}
	return s.client.GetTimeZoneList(session)
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
func (s *Service) Login(username, password, token, code string, resellerId int) (*TMSResponse, error) {
	resp, err := s.client.Login(username, password, token, code, resellerId)
	if err != nil {
		return nil, err
	}
	if resp.ResultCode == 0 && resp.Data != nil {
		if cookies, ok := resp.Data["cookies"].(string); ok && cookies != "" {
			// Update or create the TMS login session.
			login, dbErr := s.repo.GetActiveLogin()
			if dbErr == nil && login != nil {
				s.repo.UpdateSession(login.TmsLoginID, cookies)
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
func (s *Service) GetAppList() (*TMSResponse, error) {
	session := s.GetSession()
	if session == "" {
		return nil, fmt.Errorf("no active TMS session")
	}
	return s.client.GetAppList(session)
}

// GetAppListSearch retrieves apps installed on a specific terminal.
func (s *Service) GetAppListSearch(serialNum string) (*TMSResponse, error) {
	session := s.GetSession()
	if session == "" {
		return nil, fmt.Errorf("no active TMS session")
	}
	return s.client.GetAppListSearch(session, serialNum)
}

// UpdateDeviceId updates a terminal's device ID, model, merchant, and groups.
func (s *Service) UpdateDeviceId(serialNum, model string, merchantId int, groupList []int, deviceId string) (*TMSResponse, error) {
	session := s.GetSession()
	if session == "" {
		return nil, fmt.Errorf("no active TMS session")
	}
	return s.client.UpdateDeviceId(session, serialNum, model, merchantId, groupList, deviceId)
}
