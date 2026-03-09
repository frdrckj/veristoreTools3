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

// GetTmsCredentials returns the TMS credentials for the current user.
// Like V2: username = app username, password = decrypted from user.tms_password
// (saved during app login).
func (s *Service) GetTmsCredentials(currentUsername string) (tmsUser, tmsPassword string) {
	// TMS username = app username (like V2: Yii::$app->user->identity->user_name).
	tmsUser = currentUsername

	var row struct {
		TmsPassword *string `gorm:"column:tms_password"`
	}
	if err := s.db.Table("user").Where("user_name = ?", currentUsername).First(&row).Error; err == nil {
		if row.TmsPassword != nil && *row.TmsPassword != "" {
			decrypted, err := DecryptAES(*row.TmsPassword)
			if err == nil && decrypted != "" {
				tmsPassword = decrypted
			} else {
				tmsPassword = *row.TmsPassword
			}
		}
	}

	return
}

// SaveTmsPassword encrypts and saves the plain-text password to user.tms_password.
// Called on app login (like V2: TmsHelper::encrypt_decrypt($model->password)).
// Pass empty string to clear (called on logout).
func (s *Service) SaveTmsPassword(currentUsername, plainPassword string) {
	if plainPassword == "" {
		s.db.Table("user").Where("user_name = ?", currentUsername).
			Update("tms_password", nil)
		return
	}
	encrypted, err := EncryptAES(plainPassword)
	if err != nil {
		encrypted = plainPassword
	}
	s.db.Table("user").Where("user_name = ?", currentUsername).
		Update("tms_password", encrypted)
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

// GetTerminalListBulk retrieves terminals with a large page size (100).
// Used by bulk operations (export, delete-all) to reduce API calls.
func (s *Service) GetTerminalListBulk(page int) (*TMSResponse, error) {
	return s.client.GetTerminalListWithSize(page, 100)
}

// SearchTerminals searches terminals with filters.
// queryType 0=SN, 1=Merchant, 2=Group, 3=TID, 4=CSI, 5=MID.
// All search types use TMS API (matching v2 behavior).
// Pass username so we can use the per-user TMS session for old API calls.
func (s *Service) SearchTerminals(page int, search string, queryType int, username string) (*TMSResponse, error) {
	// Prefer per-user session; fall back to global tms_login session.
	session := s.GetUserSession(username)
	if session == "" {
		session = s.GetSession()
	}
	return s.client.GetTerminalListSearch(session, page, search, queryType)
}

// searchTerminalsByParam searches the local terminal_parameter table by TID or MID
// and returns results in the same TMSResponse format as the API.
func (s *Service) searchTerminalsByParam(page int, search string, queryType int) (*TMSResponse, error) {
	const pageSize = 10

	type paramResult struct {
		TermDeviceID      string `gorm:"column:term_device_id"`
		TermSerialNum     string `gorm:"column:term_serial_num"`
		TermModel         string `gorm:"column:term_model"`
		ParamMerchantName string `gorm:"column:param_merchant_name"`
		ParamTID          string `gorm:"column:param_tid"`
		ParamMID          string `gorm:"column:param_mid"`
	}

	tx := s.db.Table("terminal_parameter").
		Select("terminal.term_device_id, terminal.term_serial_num, terminal.term_model, terminal_parameter.param_merchant_name, terminal_parameter.param_tid, terminal_parameter.param_mid").
		Joins("JOIN terminal ON terminal.term_id = terminal_parameter.param_term_id")

	switch queryType {
	case 3: // TID
		tx = tx.Where("terminal_parameter.param_tid = ?", search)
	case 5: // MID
		tx = tx.Where("terminal_parameter.param_mid = ?", search)
	}

	var total int64
	if err := tx.Count(&total).Error; err != nil {
		return nil, fmt.Errorf("searchTerminalsByParam count: %w", err)
	}

	var results []paramResult
	offset := (page - 1) * pageSize
	if err := tx.Offset(offset).Limit(pageSize).Find(&results).Error; err != nil {
		return nil, fmt.Errorf("searchTerminalsByParam query: %w", err)
	}

	var list []interface{}
	for _, r := range results {
		m := map[string]interface{}{
			"deviceId":     r.TermDeviceID,
			"sn":           r.TermSerialNum,
			"model":        r.TermModel,
			"merchantName": r.ParamMerchantName,
			"tid":          r.ParamTID,
			"mid":          r.ParamMID,
			"alertStatus":  1,
			"status":       1,
			"alertMsg":     "",
		}
		list = append(list, m)
	}

	totalPages := int(total) / pageSize
	if int(total)%pageSize != 0 {
		totalPages++
	}

	return &TMSResponse{
		ResultCode: 0,
		Desc:       "OK",
		Data: map[string]interface{}{
			"totalPage":    totalPages,
			"total":        int(total),
			"terminalList": list,
		},
	}, nil
}

// EnrichTerminalsWithTIDMID looks up local terminal_parameter records by CSI (deviceId)
// and adds tid/mid fields to each terminal map.
func (s *Service) EnrichTerminalsWithTIDMID(terminals []map[string]interface{}) {
	if len(terminals) == 0 {
		return
	}

	// Collect all deviceIds (CSIs).
	var deviceIDs []string
	for _, t := range terminals {
		csi := toString(t["deviceId"])
		if csi != "" {
			deviceIDs = append(deviceIDs, csi)
		}
	}
	if len(deviceIDs) == 0 {
		return
	}

	type paramRow struct {
		TermDeviceID string `gorm:"column:term_device_id"`
		ParamTID     string `gorm:"column:param_tid"`
		ParamMID     string `gorm:"column:param_mid"`
	}

	var rows []paramRow
	s.db.Table("terminal_parameter").
		Select("terminal.term_device_id, terminal_parameter.param_tid, terminal_parameter.param_mid").
		Joins("JOIN terminal ON terminal.term_id = terminal_parameter.param_term_id").
		Where("terminal.term_device_id IN ?", deviceIDs).
		Find(&rows)

	// Build a lookup map: deviceId → {tid, mid}.
	lookup := map[string]paramRow{}
	for _, r := range rows {
		if _, exists := lookup[r.TermDeviceID]; !exists {
			lookup[r.TermDeviceID] = r
		}
	}

	// Enrich terminal maps.
	for _, t := range terminals {
		csi := toString(t["deviceId"])
		if row, ok := lookup[csi]; ok {
			t["tid"] = row.ParamTID
			t["mid"] = row.ParamMID
		}
	}
}

// SearchTerminalsBulk is like SearchTerminals but with page size 100.
// Used by bulk operations (export, delete-all) to reduce API calls (10x fewer pages).
func (s *Service) SearchTerminalsBulk(page int, search string, queryType int, username string) (*TMSResponse, error) {
	session := s.GetUserSession(username)
	if session == "" {
		session = s.GetSession()
	}
	return s.client.GetTerminalListSearchBulk(session, page, search, queryType)
}

// searchTerminalsByParamBulk is like searchTerminalsByParam but with page size 100.
func (s *Service) searchTerminalsByParamBulk(page int, search string, queryType int) (*TMSResponse, error) {
	const pageSize = 100

	type paramResult struct {
		TermDeviceID      string `gorm:"column:term_device_id"`
		TermSerialNum     string `gorm:"column:term_serial_num"`
		TermModel         string `gorm:"column:term_model"`
		ParamMerchantName string `gorm:"column:param_merchant_name"`
		ParamTID          string `gorm:"column:param_tid"`
		ParamMID          string `gorm:"column:param_mid"`
	}

	tx := s.db.Table("terminal_parameter").
		Select("terminal.term_device_id, terminal.term_serial_num, terminal.term_model, terminal_parameter.param_merchant_name, terminal_parameter.param_tid, terminal_parameter.param_mid").
		Joins("JOIN terminal ON terminal.term_id = terminal_parameter.param_term_id")

	switch queryType {
	case 3:
		tx = tx.Where("terminal_parameter.param_tid = ?", search)
	case 5:
		tx = tx.Where("terminal_parameter.param_mid = ?", search)
	}

	var total int64
	if err := tx.Count(&total).Error; err != nil {
		return nil, fmt.Errorf("searchTerminalsByParamBulk count: %w", err)
	}

	var results []paramResult
	offset := (page - 1) * pageSize
	if err := tx.Offset(offset).Limit(pageSize).Find(&results).Error; err != nil {
		return nil, fmt.Errorf("searchTerminalsByParamBulk query: %w", err)
	}

	var list []interface{}
	for _, r := range results {
		m := map[string]interface{}{
			"deviceId":     r.TermDeviceID,
			"sn":           r.TermSerialNum,
			"model":        r.TermModel,
			"merchantName": r.ParamMerchantName,
			"tid":          r.ParamTID,
			"mid":          r.ParamMID,
			"alertStatus":  1,
			"status":       1,
			"alertMsg":     "",
		}
		list = append(list, m)
	}

	totalPages := int(total) / pageSize
	if int(total)%pageSize != 0 {
		totalPages++
	}

	return &TMSResponse{
		ResultCode: 0,
		Desc:       "OK",
		Data: map[string]interface{}{
			"totalPage":    totalPages,
			"total":        int(total),
			"terminalList": list,
		},
	}, nil
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
				count := atomic.AddInt64(&successCount, 1)
				logger.Info().Str("sn", sn).Int64("progress", count).Int("total", len(serialNos)).Msg("deleted")
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

// DeleteSingleTerminal deletes a single terminal using the signed API (no session).
func (s *Service) DeleteSingleTerminal(serialNo string) (*TMSResponse, error) {
	return s.client.DeleteTerminal("", serialNo)
}

// DeleteTerminalByID deletes a terminal using its internal TMS ID directly,
// skipping the SN→ID resolution. Much faster for bulk delete.
func (s *Service) DeleteTerminalByID(terminalId string) (*TMSResponse, error) {
	return s.client.DeleteTerminalByID(terminalId)
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
// Uses session-based auth (matching v2).
func (s *Service) AddMerchant(data MerchantData) (*TMSResponse, error) {
	session := s.GetSession()
	if session == "" {
		return nil, fmt.Errorf("no active TMS session")
	}
	return s.client.AddMerchant(session, data)
}

// EditMerchant updates an existing merchant.
// Uses session-based auth (matching v2).
func (s *Service) EditMerchant(data MerchantData) (*TMSResponse, error) {
	session := s.GetSession()
	if session == "" {
		return nil, fmt.Errorf("no active TMS session")
	}
	return s.client.EditMerchant(session, data)
}

// DeleteMerchant removes a merchant by ID.
// Uses session-based auth (matching v2).
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
// Uses session-based auth with operationMark (matching v2).
func (s *Service) AddGroup(name string, terminals []int) (*TMSResponse, error) {
	session := s.GetSession()
	if session == "" {
		return nil, fmt.Errorf("no active TMS session")
	}
	return s.client.AddGroup(session, name, terminals)
}

// EditGroup updates a group's name and terminal membership.
// Uses session-based auth with operationMark + preAdd/preDel (matching v2).
func (s *Service) EditGroup(id int, name string, newTerminals, oldTerminals []int) (*TMSResponse, error) {
	session := s.GetSession()
	if session == "" {
		return nil, fmt.Errorf("no active TMS session")
	}
	return s.client.EditGroup(session, id, name, newTerminals, oldTerminals)
}

// DeleteGroup removes a group by ID.
// Uses session-based auth (matching v2).
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
func (s *Service) UpdateDeviceId(serialNum, model string, merchantId string, groupList []int, deviceId string) (*TMSResponse, error) {
	session := s.GetSession()
	if session == "" {
		return nil, fmt.Errorf("no active TMS session")
	}
	return s.client.UpdateDeviceId(session, serialNum, model, merchantId, groupList, deviceId)
}
