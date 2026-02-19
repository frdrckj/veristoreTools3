package admin

import "time"

// ActivityLog maps to the `activity_log` table from veristoreTools2.
type ActivityLog struct {
	ActLogID     int       `gorm:"column:act_log_id;primaryKey;autoIncrement" json:"act_log_id"`
	ActLogAction string    `gorm:"column:act_log_action;type:varchar(100);not null" json:"act_log_action"`
	ActLogDetail *string   `gorm:"column:act_log_detail;type:text" json:"act_log_detail"`
	CreatedBy    string    `gorm:"column:created_by;type:varchar(100);not null" json:"created_by"`
	CreatedDt    time.Time `gorm:"column:created_dt;not null" json:"created_dt"`
}

func (ActivityLog) TableName() string {
	return "activity_log"
}

// Technician maps to the `technician` table from veristoreTools2.
type Technician struct {
	TechID           int        `gorm:"column:tech_id;primaryKey;autoIncrement" json:"tech_id"`
	TechName         string     `gorm:"column:tech_name;type:varchar(150);not null" json:"tech_name"`
	TechNip          string     `gorm:"column:tech_nip;type:varchar(50);not null" json:"tech_nip"`
	TechNumber       string     `gorm:"column:tech_number;type:varchar(100);not null;uniqueIndex:tech_number" json:"tech_number"`
	TechAddress      string     `gorm:"column:tech_address;type:text;not null" json:"tech_address"`
	TechCompany      string     `gorm:"column:tech_company;type:varchar(100);not null" json:"tech_company"`
	TechSercivePoint string     `gorm:"column:tech_sercive_point;type:varchar(100);not null" json:"tech_sercive_point"`
	TechPhone        string     `gorm:"column:tech_phone;type:varchar(15);not null" json:"tech_phone"`
	TechGender       string     `gorm:"column:tech_gender;type:varchar(1);not null" json:"tech_gender"`
	TechStatus       string     `gorm:"column:tech_status;type:varchar(1);not null;default:1" json:"tech_status"`
	CreatedBy        string     `gorm:"column:created_by;type:varchar(100);not null" json:"created_by"`
	CreatedDt        time.Time  `gorm:"column:created_dt;not null" json:"created_dt"`
	UpdatedBy        *string    `gorm:"column:updated_by;type:varchar(100)" json:"updated_by"`
	UpdatedDt        *time.Time `gorm:"column:updated_dt" json:"updated_dt"`
}

func (Technician) TableName() string {
	return "technician"
}

// Faq maps to the `faq` table from veristoreTools2.
type Faq struct {
	FaqID         int     `gorm:"column:faq_id;primaryKey;autoIncrement" json:"faq_id"`
	FaqParent     *int    `gorm:"column:faq_parent;index:fk_faq_parent_id_idx" json:"faq_parent"`
	FaqSeq        int     `gorm:"column:faq_seq;not null" json:"faq_seq"`
	FaqPrivileges string  `gorm:"column:faq_privileges;type:varchar(60);not null" json:"faq_privileges"`
	FaqName       string  `gorm:"column:faq_name;type:text;not null" json:"faq_name"`
	FaqLink       *string `gorm:"column:faq_link;type:text" json:"faq_link"`

	// Self-referencing parent
	Parent   *Faq  `gorm:"foreignKey:FaqParent;references:FaqID" json:"parent,omitempty"`
	Children []Faq `gorm:"foreignKey:FaqParent;references:FaqID" json:"children,omitempty"`
}

func (Faq) TableName() string {
	return "faq"
}

// TemplateParameter maps to the `template_parameter` table from veristoreTools2.
type TemplateParameter struct {
	TparamID         int     `gorm:"column:tparam_id;primaryKey;autoIncrement" json:"tparam_id"`
	TparamTitle      string  `gorm:"column:tparam_title;type:varchar(75);not null" json:"tparam_title"`
	TparamIndexTitle string  `gorm:"column:tparam_index_title;type:text;not null" json:"tparam_index_title"`
	TparamField      string  `gorm:"column:tparam_field;type:varchar(200);not null" json:"tparam_field"`
	TparamIndex      int     `gorm:"column:tparam_index;not null" json:"tparam_index"`
	TparamType       string  `gorm:"column:tparam_type;type:varchar(1);not null" json:"tparam_type"`
	TparamOperation  string  `gorm:"column:tparam_operation;type:text;not null" json:"tparam_operation"`
	TparamLength     string  `gorm:"column:tparam_length;type:text;not null" json:"tparam_length"`
	TparamExcept     *string `gorm:"column:tparam_except;type:text" json:"tparam_except"`
}

func (TemplateParameter) TableName() string {
	return "template_parameter"
}

// TidNote maps to the `tid_note` table from veristoreTools2.
type TidNote struct {
	TidNoteID        int       `gorm:"column:tid_note_id;primaryKey;autoIncrement" json:"tid_note_id"`
	TidNoteSerialNum string    `gorm:"column:tid_note_serial_num;type:text;not null" json:"tid_note_serial_num"`
	TidNoteData      *string   `gorm:"column:tid_note_data;type:text" json:"tid_note_data"`
	CreatedBy        string    `gorm:"column:created_by;type:varchar(100);not null" json:"created_by"`
	CreatedDt        time.Time `gorm:"column:created_dt;not null" json:"created_dt"`
}

func (TidNote) TableName() string {
	return "tid_note"
}

// QueueLog maps to the `queue_log` table from veristoreTools2.
type QueueLog struct {
	CreateTime  string  `gorm:"column:create_time;type:varchar(50);primaryKey" json:"create_time"`
	ExecTime    string  `gorm:"column:exec_time;type:varchar(20);not null" json:"exec_time"`
	ProcessName string  `gorm:"column:process_name;type:varchar(5);primaryKey" json:"process_name"`
	ServiceName *string `gorm:"column:service_name;type:varchar(255)" json:"service_name"`
}

func (QueueLog) TableName() string {
	return "queue_log"
}

// Export maps to the `export` table from veristoreTools2.
type Export struct {
	ExpID       int     `gorm:"column:exp_id;primaryKey;autoIncrement" json:"exp_id"`
	ExpFilename string  `gorm:"column:exp_filename;type:varchar(50);not null" json:"exp_filename"`
	ExpData     []byte  `gorm:"column:exp_data;type:longblob" json:"-"`
	ExpCurrent  *string `gorm:"column:exp_current;type:varchar(10);default:0" json:"exp_current"`
	ExpTotal    *string `gorm:"column:exp_total;type:varchar(10);default:0" json:"exp_total"`
}

func (Export) TableName() string {
	return "export"
}

// ExportResult maps to the `export_result` table from veristoreTools2.
type ExportResult struct {
	ExpResID   int    `gorm:"column:exp_res_id;primaryKey;autoIncrement" json:"exp_res_id"`
	ExpResData string `gorm:"column:exp_res_data;type:text;not null" json:"exp_res_data"`
}

func (ExportResult) TableName() string {
	return "export_result"
}

// Hash maps to the `hash` table from veristoreTools2.
type Hash struct {
	ID        int       `gorm:"column:id;primaryKey" json:"id"`
	Hash      []byte    `gorm:"column:hash;type:binary(1);not null" json:"hash"`
	Timestamp time.Time `gorm:"column:timestamp;not null" json:"timestamp"`
}

func (Hash) TableName() string {
	return "hash"
}

// Session maps to the `session` table from veristoreTools2.
type Session struct {
	ID     string `gorm:"column:id;type:char(40);primaryKey" json:"id"`
	Expire *int   `gorm:"column:expire" json:"expire"`
	Data   []byte `gorm:"column:data;type:blob" json:"-"`
}

func (Session) TableName() string {
	return "session"
}
