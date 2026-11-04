package models

import "time"

// MenuAPI 菜单API关联模型
type MenuAPI struct {
	ID        int64     `gorm:"primaryKey;column:id" json:"id"`
	MenuID    int64     `gorm:"column:menu_id;not null;uniqueIndex:idx_sys_menu_api_unique;index:idx_sys_menu_api_menu_id" json:"menu_id"`
	APIID     int64     `gorm:"column:api_id;not null;uniqueIndex:idx_sys_menu_api_unique;index:idx_sys_menu_api_api_id" json:"api_id"`
	CreatedAt time.Time `gorm:"column:created_at;not null;default:CURRENT_TIMESTAMP" json:"created_at"`

	// 关联关系
	Menu *Menu `gorm:"foreignKey:MenuID;references:ID" json:"menu,omitempty"`
	API  *API  `gorm:"foreignKey:APIID;references:ID" json:"api,omitempty"`
}

// TableName 指定表名
func (*MenuAPI) TableName() string {
	return "sys_menu_api"
}
