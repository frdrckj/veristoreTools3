package terminal

import "time"

// Terminal maps to the `terminal` table from veristoreTools2.
type Terminal struct {
	TermID                 int        `gorm:"column:term_id;primaryKey;autoIncrement" json:"term_id"`
	TermDeviceID           string     `gorm:"column:term_device_id;type:text;not null" json:"term_device_id"`
	TermSerialNum          string     `gorm:"column:term_serial_num;type:text;not null" json:"term_serial_num"`
	TermProductNum         string     `gorm:"column:term_product_num;type:text;not null" json:"term_product_num"`
	TermModel              string     `gorm:"column:term_model;type:text;not null" json:"term_model"`
	TermAppName            string     `gorm:"column:term_app_name;type:text;not null" json:"term_app_name"`
	TermAppVersion         string     `gorm:"column:term_app_version;type:text;not null" json:"term_app_version"`
	TermTmsCreateOperator  string     `gorm:"column:term_tms_create_operator;type:text;not null" json:"term_tms_create_operator"`
	TermTmsCreateDtOperator time.Time `gorm:"column:term_tms_create_dt_operator;not null" json:"term_tms_create_dt_operator"`
	TermTmsUpdateOperator  *string    `gorm:"column:term_tms_update_operator;type:text" json:"term_tms_update_operator"`
	TermTmsUpdateDtOperator *time.Time `gorm:"column:term_tms_update_dt_operator" json:"term_tms_update_dt_operator"`
	CreatedBy              string     `gorm:"column:created_by;type:varchar(100);not null" json:"created_by"`
	CreatedDt              time.Time  `gorm:"column:created_dt;not null" json:"created_dt"`
	UpdatedBy              *string    `gorm:"column:updated_by;type:varchar(100)" json:"updated_by"`
	UpdatedDt              *time.Time `gorm:"column:updated_dt" json:"updated_dt"`
}

func (Terminal) TableName() string {
	return "terminal"
}

// TerminalParameter maps to the `terminal_parameter` table from veristoreTools2.
type TerminalParameter struct {
	ParamID          int    `gorm:"column:param_id;primaryKey;autoIncrement" json:"param_id"`
	ParamTermID      int    `gorm:"column:param_term_id;not null;index:fk_param_term_id_idx" json:"param_term_id"`
	ParamHostName    string `gorm:"column:param_host_name;type:text;not null" json:"param_host_name"`
	ParamMerchantName string `gorm:"column:param_merchant_name;type:text;not null" json:"param_merchant_name"`
	ParamTID         string `gorm:"column:param_tid;type:text;not null" json:"param_tid"`
	ParamMID         string `gorm:"column:param_mid;type:text;not null" json:"param_mid"`
	ParamAddress1    *string `gorm:"column:param_address_1;type:text" json:"param_address_1"`
	ParamAddress2    *string `gorm:"column:param_address_2;type:text" json:"param_address_2"`
	ParamAddress3    *string `gorm:"column:param_address_3;type:text" json:"param_address_3"`
	ParamAddress4    *string `gorm:"column:param_address_4;type:text" json:"param_address_4"`
	ParamAddress5    *string `gorm:"column:param_address_5;type:text" json:"param_address_5"`
	ParamAddress6    *string `gorm:"column:param_address_6;type:text" json:"param_address_6"`

	// Belongs to Terminal
	Terminal Terminal `gorm:"foreignKey:ParamTermID;references:TermID" json:"terminal,omitempty"`
}

func (TerminalParameter) TableName() string {
	return "terminal_parameter"
}
