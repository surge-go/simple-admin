package models

import (
	"time"

	"gorm.io/datatypes"
)

// Config 系统配置模型。
type Config struct {
	ID        int64          `gorm:"primaryKey;column:id" json:"id"`
	Key       string         `gorm:"column:key;type:varchar(128);not null;uniqueIndex:idx_sys_config_key" json:"key"`
	Name      string         `gorm:"column:name;type:varchar(128);not null" json:"name"`
	GroupName string         `gorm:"column:group_name;type:varchar(64);not null;index:idx_sys_config_group_name" json:"group_name"`
	Value     datatypes.JSON `gorm:"column:value;type:jsonb;not null" json:"value"`
	Version   int            `gorm:"column:version;type:integer;not null;default:1" json:"version"`
	Remark    string         `gorm:"column:remark;type:varchar(255);not null;default:''" json:"remark"`
	UpdatedBy *int64         `gorm:"column:updated_by;index:idx_sys_config_updated_by" json:"updated_by,omitempty"`
	CreatedAt time.Time      `gorm:"column:created_at;not null;default:CURRENT_TIMESTAMP" json:"created_at"`
	UpdatedAt *time.Time     `gorm:"column:updated_at" json:"updated_at,omitempty"`
}

// TableName 指定表名。
func (*Config) TableName() string {
	return "sys_config"
}
