package database

import (
	"regexp"
	"strings"

	"gorm.io/gorm"
	"gorm.io/plugin/dbresolver"
)

const (
	lockingRawSQLCallbackName    = "aurora:db_resolver_locking_raw_sql"
	lockingRawSQLRowCallbackName = "aurora:db_resolver_locking_row_sql"
)

var lockingRawSQLPattern = regexp.MustCompile(`(?i)\bfor\s+(?:no\s+key\s+)?(?:update|share|key\s+share)\b|\block\s+in\s+share\s+mode\b`)

// registerResolverCallbacks 为 dbresolver 的原始 SQL 路由判断增加保护。
// 上游回调只有在 SQL 末尾字节恰好为 "for update" 时才会将原始 SELECT 识别为写操作，
// 因此 "FOR UPDATE;" 等常见形式可能被发送到只读副本。加锁语句统一保守地路由到写库。
func registerResolverCallbacks(db *gorm.DB) error {
	// 在注册 dbresolver 后添加回调，借助 Before("*") 按 GORM 稳定的回调顺序，
	// 将这些保护逻辑置于其原始 SQL 和行查询路由回调之前。
	if err := db.Callback().Raw().Before("*").Register(lockingRawSQLCallbackName, func(tx *gorm.DB) {
		routeLockingRawSQL(tx)
	}); err != nil {
		return err
	}
	if err := db.Callback().Row().Before("*").Register(lockingRawSQLRowCallbackName, func(tx *gorm.DB) {
		routeLockingRawSQL(tx)
	}); err != nil {
		return err
	}
	return nil
}

func routeLockingRawSQL(db *gorm.DB) {
	if db.Statement == nil || !isLockingRawSQL(db.Statement.SQL.String()) {
		return
	}
	dbresolver.Write.ModifyStatement(db.Statement)
}

func isLockingRawSQL(query string) bool {
	query = strings.TrimSpace(query)
	query = strings.TrimRight(query, "; 	\r\n")
	return lockingRawSQLPattern.MatchString(query)
}
