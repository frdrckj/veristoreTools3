package user

import "time"

// User maps to the `user` table from veristoreTools2.
type User struct {
	UserID                int        `gorm:"column:user_id;primaryKey;autoIncrement" json:"user_id"`
	UserFullname          string     `gorm:"column:user_fullname;type:varchar(100);not null" json:"user_fullname"`
	UserName              string     `gorm:"column:user_name;type:varchar(60);not null;uniqueIndex:user_code" json:"user_name"`
	Password              string     `gorm:"column:password;type:varchar(256);not null" json:"-"`
	UserPrivileges        string     `gorm:"column:user_privileges;type:varchar(60);not null" json:"user_privileges"`
	UserLastChangePassword *time.Time `gorm:"column:user_lastchangepassword" json:"user_lastchangepassword"`
	CreatedDtm            time.Time  `gorm:"column:createddtm;not null" json:"createddtm"`
	CreatedBy             string     `gorm:"column:createdby;type:varchar(60);not null" json:"createdby"`
	AuthKey               *string    `gorm:"column:auth_key;type:varchar(32)" json:"auth_key"`
	PasswordHash          *string    `gorm:"column:password_hash;type:varchar(256)" json:"-"`
	PasswordResetToken    *string    `gorm:"column:password_reset_token;type:varchar(256)" json:"-"`
	Email                 *string    `gorm:"column:email;type:varchar(256)" json:"email"`
	Status                *int       `gorm:"column:status" json:"status"`
	CreatedAt             *int       `gorm:"column:created_at" json:"created_at"`
	UpdatedAt             *int       `gorm:"column:updated_at" json:"updated_at"`
	TmsSession            *string    `gorm:"column:tms_session;type:varchar(5120)" json:"-"`
	TmsPassword           *string    `gorm:"column:tms_password;type:varchar(256)" json:"-"`
	TmsUsername           *string    `gorm:"column:tms_username;type:varchar(200)" json:"-"`
}

func (User) TableName() string {
	return "user"
}
