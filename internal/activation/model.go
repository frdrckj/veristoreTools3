package activation

import "time"

// AppActivation maps to the `app_activation` table from veristoreTools2.
type AppActivation struct {
	AppActID       int       `gorm:"column:app_act_id;primaryKey;autoIncrement" json:"app_act_id"`
	AppActCSI      string    `gorm:"column:app_act_csi;type:text;not null" json:"app_act_csi"`
	AppActTID      string    `gorm:"column:app_act_tid;type:text;not null" json:"app_act_tid"`
	AppActMID      string    `gorm:"column:app_act_mid;type:text;not null" json:"app_act_mid"`
	AppActModel    string    `gorm:"column:app_act_model;type:text;not null" json:"app_act_model"`
	AppActVersion  string    `gorm:"column:app_act_version;type:text;not null" json:"app_act_version"`
	AppActEngineer string    `gorm:"column:app_act_engineer;type:text;not null" json:"app_act_engineer"`
	CreatedBy      string    `gorm:"column:created_by;type:varchar(100);not null" json:"created_by"`
	CreatedDt      time.Time `gorm:"column:created_dt;not null" json:"created_dt"`
}

func (AppActivation) TableName() string {
	return "app_activation"
}

// AppCredential maps to the `app_credential` table from veristoreTools2.
type AppCredential struct {
	AppCredID     int       `gorm:"column:app_cred_id;primaryKey;autoIncrement" json:"app_cred_id"`
	AppCredUser   string    `gorm:"column:app_cred_user;type:varchar(256);not null" json:"app_cred_user"`
	AppCredName   string    `gorm:"column:app_cred_name;type:varchar(100);not null" json:"app_cred_name"`
	AppCredEnable *string   `gorm:"column:app_cred_enable;type:varchar(1);default:1" json:"app_cred_enable"`
	CreatedBy     string    `gorm:"column:created_by;type:varchar(100);not null" json:"created_by"`
	CreatedDt     time.Time `gorm:"column:created_dt;not null" json:"created_dt"`
}

func (AppCredential) TableName() string {
	return "app_credential"
}
