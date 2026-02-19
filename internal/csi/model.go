package csi

import "time"

// VerificationReport maps to the `verification_report` table from veristoreTools2.
type VerificationReport struct {
	VfiRptID                    int       `gorm:"column:vfi_rpt_id;primaryKey;autoIncrement" json:"vfi_rpt_id"`
	VfiRptTermDeviceID          string    `gorm:"column:vfi_rpt_term_device_id;type:text;not null" json:"vfi_rpt_term_device_id"`
	VfiRptTermSerialNum         string    `gorm:"column:vfi_rpt_term_serial_num;type:text;not null" json:"vfi_rpt_term_serial_num"`
	VfiRptTermProductNum        string    `gorm:"column:vfi_rpt_term_product_num;type:text;not null" json:"vfi_rpt_term_product_num"`
	VfiRptTermModel             string    `gorm:"column:vfi_rpt_term_model;type:text;not null" json:"vfi_rpt_term_model"`
	VfiRptTermAppName           string    `gorm:"column:vfi_rpt_term_app_name;type:text;not null" json:"vfi_rpt_term_app_name"`
	VfiRptTermAppVersion        string    `gorm:"column:vfi_rpt_term_app_version;type:text;not null" json:"vfi_rpt_term_app_version"`
	VfiRptTermParameter         string    `gorm:"column:vfi_rpt_term_parameter;type:text;not null" json:"vfi_rpt_term_parameter"`
	VfiRptTermTmsCreateOperator string    `gorm:"column:vfi_rpt_term_tms_create_operator;type:text;not null" json:"vfi_rpt_term_tms_create_operator"`
	VfiRptTermTmsCreateDtOperator time.Time `gorm:"column:vfi_rpt_term_tms_create_dt_operator;not null" json:"vfi_rpt_term_tms_create_dt_operator"`
	VfiRptTechName              string    `gorm:"column:vfi_rpt_tech_name;type:varchar(150);not null" json:"vfi_rpt_tech_name"`
	VfiRptTechNip               string    `gorm:"column:vfi_rpt_tech_nip;type:varchar(50);not null" json:"vfi_rpt_tech_nip"`
	VfiRptTechNumber            string    `gorm:"column:vfi_rpt_tech_number;type:varchar(100);not null" json:"vfi_rpt_tech_number"`
	VfiRptTechAddress           string    `gorm:"column:vfi_rpt_tech_address;type:text;not null" json:"vfi_rpt_tech_address"`
	VfiRptTechCompany           string    `gorm:"column:vfi_rpt_tech_company;type:varchar(100);not null" json:"vfi_rpt_tech_company"`
	VfiRptTechSercivePoint      string    `gorm:"column:vfi_rpt_tech_sercive_point;type:varchar(100);not null" json:"vfi_rpt_tech_sercive_point"`
	VfiRptTechPhone             string    `gorm:"column:vfi_rpt_tech_phone;type:varchar(15);not null" json:"vfi_rpt_tech_phone"`
	VfiRptTechGender            string    `gorm:"column:vfi_rpt_tech_gender;type:varchar(25);not null" json:"vfi_rpt_tech_gender"`
	VfiRptTicketNo              string    `gorm:"column:vfi_rpt_ticket_no;type:varchar(50);not null" json:"vfi_rpt_ticket_no"`
	VfiRptSpkNo                 string    `gorm:"column:vfi_rpt_spk_no;type:varchar(50);not null" json:"vfi_rpt_spk_no"`
	VfiRptWorkOrder             string    `gorm:"column:vfi_rpt_work_order;type:varchar(50);not null" json:"vfi_rpt_work_order"`
	VfiRptRemark                string    `gorm:"column:vfi_rpt_remark;type:varchar(200);not null" json:"vfi_rpt_remark"`
	VfiRptStatus                string    `gorm:"column:vfi_rpt_status;type:varchar(10);not null" json:"vfi_rpt_status"`
	CreatedBy                   string    `gorm:"column:created_by;type:varchar(100);not null" json:"created_by"`
	CreatedDt                   time.Time `gorm:"column:created_dt;not null" json:"created_dt"`
}

func (VerificationReport) TableName() string {
	return "verification_report"
}
