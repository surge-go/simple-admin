package models

import (
	"time"

	"gorm.io/gorm"
)

// User 用户模型
type User struct {
	ID          int64          `gorm:"primaryKey;column:id" json:"id"`
	Username    string         `gorm:"column:username;type:varchar(64);not null;uniqueIndex:idx_sys_user_username,where:deleted_at IS NULL" json:"username"`
	Password    string         `gorm:"column:password;type:varchar(255);not null" json:"-"`
	RealName    *string        `gorm:"column:real_name;type:varchar(64)" json:"real_name,omitempty"`
	Nickname    *string        `gorm:"column:nickname;type:varchar(64)" json:"nickname,omitempty"`
	Avatar      *string        `gorm:"column:avatar;type:varchar(512)" json:"avatar,omitempty"`
	Email       *string        `gorm:"column:email;type:varchar(255);uniqueIndex:idx_sys_user_email,where:deleted_at IS NULL AND email IS NOT NULL" json:"email,omitempty"`
	Phone       *string        `gorm:"column:phone;type:varchar(32);uniqueIndex:idx_sys_user_phone,where:deleted_at IS NULL AND phone IS NOT NULL" json:"phone,omitempty"`
	Gender      int16          `gorm:"column:gender;type:smallint;not null;default:0" json:"gender"`
	Status      int16          `gorm:"column:status;type:smallint;not null;default:1;index:idx_sys_user_status,where:deleted_at IS NULL" json:"status"`
	LastLoginAt *time.Time     `gorm:"column:last_login_at" json:"last_login_at,omitempty"`
	LastLoginIP *string        `gorm:"column:last_login_ip;type:varchar(128)" json:"last_login_ip,omitempty"`
	LockedUntil *time.Time     `gorm:"column:locked_until;index:idx_sys_user_locked_until,where:locked_until IS NOT NULL AND deleted_at IS NULL" json:"locked_until,omitempty"`
	AuthVersion int64          `gorm:"column:auth_version;type:bigint;not null;default:1;index:idx_sys_user_auth_version,where:deleted_at IS NULL" json:"-"`
	Remark      *string        `gorm:"column:remark;type:varchar(255)" json:"remark,omitempty"`
	CreatedAt   time.Time      `gorm:"column:created_at;not null;default:CURRENT_TIMESTAMP" json:"created_at"`
	UpdatedAt   *time.Time     `gorm:"column:updated_at" json:"updated_at,omitempty"`
	DeletedAt   gorm.DeletedAt `gorm:"column:deleted_at;index:idx_sys_user_deleted_at" json:"-"`
}

// TableName 指定表名
func (*User) TableName() string {
	return "sys_user"
}

// IsEnabled 判断用户是否启用
func (u *User) IsEnabled() bool {
	return u.Status == 1
}

// IsDisabled 判断用户是否禁用
func (u *User) IsDisabled() bool {
	return u.Status == 0
}

// IsDeleted 判断用户是否已删除
func (u *User) IsDeleted() bool {
	return u.DeletedAt.Valid
}

// IsLocked 判断用户是否被锁定
func (u *User) IsLocked() bool {
	if u.LockedUntil == nil {
		return false
	}
	return time.Now().Before(*u.LockedUntil)
}

// IsMale 判断是否是男性
func (u *User) IsMale() bool {
	return u.Gender == 1
}

// IsFemale 判断是否是女性
func (u *User) IsFemale() bool {
	return u.Gender == 2
}

// GetDisplayName 获取显示名称（优先昵称，其次真实姓名，最后用户名）
func (u *User) GetDisplayName() string {
	if u.Nickname != nil && *u.Nickname != "" {
		return *u.Nickname
	}
	if u.RealName != nil && *u.RealName != "" {
		return *u.RealName
	}
	return u.Username
}
