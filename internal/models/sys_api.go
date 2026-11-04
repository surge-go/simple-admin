package models

import (
	"time"

	"gorm.io/gorm"
)

// API API资源模型
type API struct {
	ID        int64          `gorm:"primaryKey;column:id" json:"id"`
	ParentID  *int64         `gorm:"column:parent_id;index:idx_sys_api_parent_id,where:deleted_at IS NULL;index:idx_sys_api_parent_deleted" json:"parent_id,omitempty"`
	GroupName *string        `gorm:"column:group_name;type:varchar(64);uniqueIndex:idx_sys_api_group_name,where:deleted_at IS NULL AND group_name IS NOT NULL" json:"group_name,omitempty"`
	Name      string         `gorm:"column:name;type:varchar(128);not null" json:"name"`
	PermCode  *string        `gorm:"column:perm_code;type:varchar(128);uniqueIndex:idx_sys_api_perm_code,where:deleted_at IS NULL AND perm_code IS NOT NULL" json:"perm_code,omitempty"`
	Method    *string        `gorm:"column:method;type:varchar(8)" json:"method,omitempty"`
	Path      *string        `gorm:"column:path;type:varchar(512)" json:"path,omitempty"`
	Sort      int            `gorm:"column:sort;type:integer;not null;default:0;index:idx_sys_api_sort,where:deleted_at IS NULL" json:"sort"`
	Remark    *string        `gorm:"column:remark;type:varchar(255)" json:"remark,omitempty"`
	CreatedAt time.Time      `gorm:"column:created_at;not null;default:CURRENT_TIMESTAMP" json:"created_at"`
	UpdatedAt *time.Time     `gorm:"column:updated_at" json:"updated_at,omitempty"`
	DeletedAt gorm.DeletedAt `gorm:"column:deleted_at;index:idx_sys_api_deleted_at" json:"-"`
}

// TableName 指定表名
func (*API) TableName() string {
	return "sys_api"
}

// IsDeleted 判断API是否已删除
func (a *API) IsDeleted() bool {
	return a.DeletedAt.Valid
}

// IsGroup 判断是否是分组节点
func (a *API) IsGroup() bool {
	return a.GroupName != nil && *a.GroupName != ""
}

// IsAPI 判断是否是API节点
func (a *API) IsAPI() bool {
	return a.Method != nil && *a.Method != "" && a.Path != nil && *a.Path != ""
}

// IsRoot 判断是否是根节点
func (a *API) IsRoot() bool {
	return a.ParentID == nil
}

// MethodPath 获取完整的方法路径（如 "GET /api/v1/users"）
func (a *API) MethodPath() string {
	if a.Method != nil && a.Path != nil {
		return *a.Method + " " + *a.Path
	}
	return ""
}

// GetGroupName 获取分组标识
func (a *API) GetGroupName() string {
	if a.GroupName == nil {
		return ""
	}
	return *a.GroupName
}
