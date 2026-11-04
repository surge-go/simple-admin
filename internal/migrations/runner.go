// Package migrations 提供基于 goose 的 PostgreSQL 迁移入口。
//
// 迁移可由独立发布任务或应用启动流程执行。Runner 使用数据库会话锁，
// 避免多个应用实例并发修改数据库结构。
package migrations

import (
	"context"
	"errors"
	"fmt"
	"io/fs"

	"github.com/pressly/goose/v3"
	gooselock "github.com/pressly/goose/v3/lock"
	"gorm.io/gorm"
)

// ErrNilDB 表示调用方没有提供数据库连接。
var ErrNilDB = errors.New("migrations: database is nil")

// ErrNilContext 表示调用方没有提供 context。
var ErrNilContext = errors.New("migrations: context is nil")

// ErrUnsupportedDialect 表示迁移 Runner 收到了非 PostgreSQL 的 GORM 连接。
var ErrUnsupportedDialect = errors.New("migrations: unsupported database dialect")

// advisoryLockID 用于串行化同一数据库上的迁移执行。
// 该值只需要在本项目内稳定，并与其他数据库锁用途区分开。
const advisoryLockID int64 = 2026100401

// Runner 是一个绑定到 PostgreSQL *gorm.DB 的 goose 迁移执行器。
// DB 的生命周期由调用方管理，Runner 不会关闭传入连接。
type Runner struct {
	provider *goose.Provider
}

// NewRunner 创建 PostgreSQL 迁移执行器。
//
// 使用 PostgreSQL session advisory lock，确保多个发布进程不会并发执行迁移。
// 默认 goose_db_version 表记录已经应用的版本和执行时间。
//
// goMigrations 是显式注入的 Go migration 列表。Runner 不读取 goose 的全局
// migration registry，避免其他包注册的迁移被意外执行。
func NewRunner(db *gorm.DB, goMigrations ...*goose.Migration) (*Runner, error) {
	if db == nil {
		return nil, ErrNilDB
	}
	if db.Dialector == nil || db.Dialector.Name() != string(goose.DialectPostgres) {
		dialect := ""
		if db.Dialector != nil {
			dialect = db.Dialector.Name()
		}
		return nil, fmt.Errorf("%w: %q", ErrUnsupportedDialect, dialect)
	}
	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("migrations: get underlying sql database: %w", err)
	}

	root, err := fs.Sub(postgresFS, "postgres")
	if err != nil {
		return nil, fmt.Errorf("migrations: open embedded postgres migrations: %w", err)
	}
	locker, err := gooselock.NewPostgresSessionLocker(
		gooselock.WithLockID(advisoryLockID),
	)
	if err != nil {
		return nil, fmt.Errorf("migrations: create postgres migration locker: %w", err)
	}
	providerOptions := []goose.ProviderOption{
		goose.WithSessionLocker(locker),
		// 迁移集合必须由调用方显式提供，不能受进程中其他包的全局注册影响。
		goose.WithDisableGlobalRegistry(true),
	}
	if len(goMigrations) > 0 {
		providerOptions = append(providerOptions, goose.WithGoMigrations(goMigrations...))
	}
	provider, err := goose.NewProvider(
		goose.DialectPostgres,
		sqlDB,
		root,
		providerOptions...,
	)
	if err != nil {
		return nil, fmt.Errorf("migrations: create goose provider: %w", err)
	}
	return &Runner{provider: provider}, nil
}

// Up 执行全部待应用迁移。
func (r *Runner) Up(ctx context.Context) ([]*goose.MigrationResult, error) {
	if err := validateRunnerContext(r, ctx); err != nil {
		return nil, err
	}
	return r.provider.Up(ctx)
}

// UpByOne 只执行下一条待应用迁移。
func (r *Runner) UpByOne(ctx context.Context) (*goose.MigrationResult, error) {
	if err := validateRunnerContext(r, ctx); err != nil {
		return nil, err
	}
	return r.provider.UpByOne(ctx)
}

// Down 回滚最近一次已应用迁移。
func (r *Runner) Down(ctx context.Context) (*goose.MigrationResult, error) {
	if err := validateRunnerContext(r, ctx); err != nil {
		return nil, err
	}
	return r.provider.Down(ctx)
}

// DownTo 回滚到目标版本，目标版本本身保留。
func (r *Runner) DownTo(ctx context.Context, version int64) ([]*goose.MigrationResult, error) {
	if err := validateRunnerContext(r, ctx); err != nil {
		return nil, err
	}
	return r.provider.DownTo(ctx, version)
}

// Status 返回每条迁移的 pending/applied 状态。
func (r *Runner) Status(ctx context.Context) ([]*goose.MigrationStatus, error) {
	if err := validateRunnerContext(r, ctx); err != nil {
		return nil, err
	}
	return r.provider.Status(ctx)
}

// Current 返回数据库当前已应用的最大迁移版本；没有版本时返回 0。
func (r *Runner) Current(ctx context.Context) (int64, error) {
	if err := validateRunnerContext(r, ctx); err != nil {
		return 0, err
	}
	return r.provider.GetDBVersion(ctx)
}

// Pending 判断数据库是否还有未应用迁移。
func (r *Runner) Pending(ctx context.Context) (bool, error) {
	if err := validateRunnerContext(r, ctx); err != nil {
		return false, err
	}
	return r.provider.HasPending(ctx)
}

func validateRunnerContext(r *Runner, ctx context.Context) error {
	if r == nil || r.provider == nil {
		return ErrNilDB
	}
	if ctx == nil {
		return ErrNilContext
	}
	return nil
}
