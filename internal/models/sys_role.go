package models

import (
	"time"

	"gorm.io/gorm"
)

// Role 角色模型
type Role struct {
	ID        int64          `gorm:"primaryKey;column:id" json:"id"`
	Code      string         `gorm:"column:code;type:varchar(64);not null;uniqueIndex:idx_sys_role_code,where:deleted_at IS NULL" json:"code"`
	Name      string         `gorm:"column:name;type:varchar(64);not null" json:"name"`
	IsBuiltin bool           `gorm:"column:is_builtin;not null;default:false" json:"is_builtin"`
	Sort      int            `gorm:"column:sort;type:integer;not null;default:0" json:"sort"`
	Status    int16          `gorm:"column:status;type:smallint;not null;default:1;index:idx_sys_role_status,where:deleted_at IS NULL" json:"status"`
	Remark    string         `gorm:"column:remark;type:varchar(255);not null;default:''" json:"remark"`
	CreatedAt time.Time      `gorm:"column:created_at;not null;default:CURRENT_TIMESTAMP" json:"created_at"`
	UpdatedAt *time.Time     `gorm:"column:updated_at" json:"updated_at,omitempty"`
	DeletedAt gorm.DeletedAt `gorm:"column:deleted_at;index:idx_sys_role_deleted_at" json:"-"`
}

// TableName 指定表名
func (*Role) TableName() string {
	return "sys_role"
}

// IsEnabled 判断角色是否启用
func (r *Role) IsEnabled() bool {
	return r.Status == 1
}

// IsDisabled 判断角色是否禁用
func (r *Role) IsDisabled() bool {
	return r.Status == 0
}

// IsDeleted 判断角色是否已删除
func (r *Role) IsDeleted() bool {
	return r.DeletedAt.Valid
}

// CanDelete 判断角色是否可以删除（内置角色不可删除）
func (r *Role) CanDelete() bool {
	return !r.IsBuiltin
}

// CanModifyCode 判断是否可以修改角色编码（内置角色不可修改编码）
func (r *Role) CanModifyCode() bool {
	return !r.IsBuiltin
}
