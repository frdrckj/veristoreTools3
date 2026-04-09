package approval

import "time"

// CsiRequest represents a pending CSI creation request awaiting approval.
type CsiRequest struct {
	ReqID       int        `gorm:"column:req_id;primaryKey;autoIncrement"`
	DeviceID    string     `gorm:"column:req_device_id"`
	Vendor      string     `gorm:"column:req_vendor"`
	Model       string     `gorm:"column:req_model"`
	MerchantID  string     `gorm:"column:req_merchant_id"`
	GroupIDs    string     `gorm:"column:req_group_ids"`
	SN          string     `gorm:"column:req_sn"`
	App         string     `gorm:"column:req_app"`
	AppName     string     `gorm:"column:req_app_name"`
	MoveConf    int        `gorm:"column:req_move_conf"`
	TemplateSN  string     `gorm:"column:req_template_sn"`
	RowData     string     `gorm:"column:req_row_data"`
	Source      string     `gorm:"column:req_source"`
	Status      string     `gorm:"column:req_status"`
	CreatedBy   string     `gorm:"column:created_by"`
	CreatedDt   time.Time  `gorm:"column:created_dt"`
	ApprovedBy  *string    `gorm:"column:approved_by"`
	ApprovedDt  *time.Time `gorm:"column:approved_dt"`
}

func (CsiRequest) TableName() string {
	return "csi_request"
}
