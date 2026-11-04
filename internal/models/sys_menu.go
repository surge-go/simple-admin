package models

import (
	"time"

	"gorm.io/gorm"
)

// Menu 菜单模型
type Menu struct {
	ID                 int64          `gorm:"primaryKey;column:id" json:"id"`
	ParentID           *int64         `gorm:"column:parent_id;index:idx_sys_menu_parent_id,where:deleted_at IS NULL;index:idx_sys_menu_parent_deleted" json:"parent_id,omitempty"`
	Type               string         `gorm:"column:type;type:varchar(16);not null;index:idx_sys_menu_type,where:deleted_at IS NULL" json:"type"`
	RouteName          *string        `gorm:"column:route_name;type:varchar(128);uniqueIndex:idx_sys_menu_route_name,where:deleted_at IS NULL AND route_name IS NOT NULL" json:"route_name,omitempty"`
	Title              string         `gorm:"column:title;type:varchar(128);not null" json:"title"`
	Path               *string        `gorm:"column:path;type:varchar(512)" json:"path,omitempty"`
	Component          *string        `gorm:"column:component;type:varchar(512)" json:"component,omitempty"`
	Link               *string        `gorm:"column:link;type:varchar(1024)" json:"link,omitempty"`
	Redirect           *string        `gorm:"column:redirect;type:varchar(512)" json:"redirect,omitempty"`
	Icon               *string        `gorm:"column:icon;type:varchar(128)" json:"icon,omitempty"`
	ActiveIcon         *string        `gorm:"column:active_icon;type:varchar(128)" json:"active_icon,omitempty"`
	Sort               int            `gorm:"column:sort;type:integer;not null;default:0" json:"sort"`
	Visible            bool           `gorm:"column:visible;not null;default:true" json:"visible"`
	HideChildrenInMenu bool           `gorm:"column:hide_children_in_menu;not null;default:false" json:"hide_children_in_menu"`
	KeepAlive          bool           `gorm:"column:keep_alive;not null;default:false" json:"keep_alive"`
	AffixTab           bool           `gorm:"column:affix_tab;not null;default:false" json:"affix_tab"`
	Status             int16          `gorm:"column:status;type:smallint;not null;default:1" json:"status"`
	CreatedAt          time.Time      `gorm:"column:created_at;not null;default:CURRENT_TIMESTAMP" json:"created_at"`
	UpdatedAt          *time.Time     `gorm:"column:updated_at" json:"updated_at,omitempty"`
	DeletedAt          gorm.DeletedAt `gorm:"column:deleted_at;index:idx_sys_menu_deleted_at" json:"-"`
}

// TableName 指定表名
func (*Menu) TableName() string {
	return "sys_menu"
}

// IsEnabled 判断菜单是否启用
func (m *Menu) IsEnabled() bool {
	return m.Status == 1
}

// IsDisabled 判断菜单是否禁用
func (m *Menu) IsDisabled() bool {
	return m.Status == 0
}

// IsDeleted 判断菜单是否已删除
func (m *Menu) IsDeleted() bool {
	return m.DeletedAt.Valid
}

// IsDirectory 判断是否是目录
func (m *Menu) IsDirectory() bool {
	return m.Type == "directory"
}

// IsPage 判断是否是页面
func (m *Menu) IsPage() bool {
	return m.Type == "page"
}

// IsButton 判断是否是按钮
func (m *Menu) IsButton() bool {
	return m.Type == "button"
}

// IsExternal 判断是否是外链
func (m *Menu) IsExternal() bool {
	return m.Type == "external"
}

// IsRoot 判断是否是根节点
func (m *Menu) IsRoot() bool {
	return m.ParentID == nil
}
