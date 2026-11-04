package migrations

import (
	"embed"
	"io/fs"
)

// postgresFS 包含随二进制发布的 PostgreSQL SQL 迁移文件。
// SQL 文件由 goose 解析；Go 迁移由 NewRunner 的可选参数显式注入。
//
//go:embed postgres/*.sql
var postgresFS embed.FS

// Files 返回只读的 PostgreSQL 迁移文件系统，供 goose runner 和测试使用。
func Files() fs.FS {
	return postgresFS
}
