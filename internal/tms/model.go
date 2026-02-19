package tms

import "time"

// TmsLogin maps to the `tms_login` table from veristoreTools2.
type TmsLogin struct {
	TmsLoginID        int       `gorm:"column:tms_login_id;primaryKey;autoIncrement" json:"tms_login_id"`
	TmsLoginUser      *string   `gorm:"column:tms_login_user;type:varchar(200)" json:"tms_login_user"`
	TmsLoginSession   *string   `gorm:"column:tms_login_session;type:varchar(5120)" json:"-"`
	TmsLoginScheduled *string   `gorm:"column:tms_login_scheduled;type:text" json:"tms_login_scheduled"`
	TmsLoginEnable    *string   `gorm:"column:tms_login_enable;type:varchar(1);default:1" json:"tms_login_enable"`
	CreatedBy         string    `gorm:"column:created_by;type:varchar(100);not null" json:"created_by"`
	CreatedDt         time.Time `gorm:"column:created_dt;not null" json:"created_dt"`
}

func (TmsLogin) TableName() string {
	return "tms_login"
}

// TmsReport maps to the `tms_report` table from veristoreTools2.
type TmsReport struct {
	TmsRptID        int     `gorm:"column:tms_rpt_id;autoIncrement;uniqueIndex" json:"tms_rpt_id"`
	TmsRptName      string  `gorm:"column:tms_rpt_name;type:varchar(255);primaryKey;not null" json:"tms_rpt_name"`
	TmsRptFile      []byte  `gorm:"column:tms_rpt_file;type:longblob" json:"-"`
	TmsRptRow       *string `gorm:"column:tms_rpt_row;type:longtext" json:"tms_rpt_row"`
	TmsRptCurPage   *string `gorm:"column:tms_rpt_cur_page;type:varchar(10);default:0" json:"tms_rpt_cur_page"`
	TmsRptTotalPage *string `gorm:"column:tms_rpt_total_page;type:varchar(10);default:0" json:"tms_rpt_total_page"`
}

func (TmsReport) TableName() string {
	return "tms_report"
}
