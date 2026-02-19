package sync

import "time"

// SyncTerminal maps to the `sync_terminal` table from veristoreTools2.
type SyncTerminal struct {
	SyncTermID          int       `gorm:"column:sync_term_id;autoIncrement;uniqueIndex" json:"sync_term_id"`
	SyncTermCreatorID   int       `gorm:"column:sync_term_creator_id;primaryKey;not null" json:"sync_term_creator_id"`
	SyncTermCreatorName string    `gorm:"column:sync_term_creator_name;type:text;not null" json:"sync_term_creator_name"`
	SyncTermCreatedTime time.Time `gorm:"column:sync_term_created_time;primaryKey;not null" json:"sync_term_created_time"`
	SyncTermStatus      string    `gorm:"column:sync_term_status;type:varchar(1);not null;default:0" json:"sync_term_status"`
	SyncTermProcess     *string   `gorm:"column:sync_term_process;type:varchar(10);default:0" json:"sync_term_process"`
	CreatedBy           string    `gorm:"column:created_by;type:varchar(100);not null" json:"created_by"`
	CreatedDt           time.Time `gorm:"column:created_dt;not null" json:"created_dt"`
}

func (SyncTerminal) TableName() string {
	return "sync_terminal"
}
