package models

import "time"

// UserRole 用户角色关联模型
type UserRole struct {
	ID        int64     `gorm:"primaryKey;column:id" json:"id"`
	UserID    int64     `gorm:"column:user_id;not null;uniqueIndex:idx_sys_user_role_unique;index:idx_sys_user_role_user_id" json:"user_id"`
	RoleID    int64     `gorm:"column:role_id;not null;uniqueIndex:idx_sys_user_role_unique;index:idx_sys_user_role_role_id" json:"role_id"`
	CreatedAt time.Time `gorm:"column:created_at;not null;default:CURRENT_TIMESTAMP" json:"created_at"`

	// 关联关系
	User *User `gorm:"foreignKey:UserID;references:ID" json:"user,omitempty"`
	Role *Role `gorm:"foreignKey:RoleID;references:ID" json:"role,omitempty"`
}

// TableName 指定表名
func (*UserRole) TableName() string {
	return "sys_user_role"
}
