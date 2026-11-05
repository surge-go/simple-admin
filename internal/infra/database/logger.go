package database

import (
	"context"
	"errors"
	"fmt"
	"time"

	appLogger "github.com/surge-go/simple-admin/internal/infra/logger"
	gormlogger "gorm.io/gorm/logger"
	"gorm.io/gorm/utils"
)

// buildLogger 构造使用项目 logger 包的 GORM logger。
func buildLogger(cfg *LoggerConfig, log *appLogger.Logger) gormlogger.Interface {
	return appGORMLogger{
		logger: log,
		config: gormlogger.Config{
			SlowThreshold:             cfg.SlowThreshold,
			IgnoreRecordNotFoundError: cfg.IgnoreRecordNotFoundError,
			ParameterizedQueries:      cfg.ParameterizedQueries,
			LogLevel:                  toGORMLogLevel(cfg.Level),
		},
		logSQL: cfg.LogSQL,
	}
}

// appGORMLogger 使用项目 logger 包承接 GORM logger.Interface。
//
// LogSQL=false 时普通 SQL Trace 不输出；错误和慢查询仍会输出耗时、影响行数和错误信息，
// 但 SQL 文本会被固定隐藏，避免敏感参数或完整 SQL 进入日志。
type appGORMLogger struct {
	logger *appLogger.Logger
	config gormlogger.Config
	logSQL bool
}

func (l appGORMLogger) LogMode(level gormlogger.LogLevel) gormlogger.Interface {
	config := l.config
	config.LogLevel = level
	return appGORMLogger{
		logger: l.logger,
		config: config,
		logSQL: l.logSQL,
	}
}

func (l appGORMLogger) Info(ctx context.Context, msg string, data ...interface{}) {
	if l.config.LogLevel < gormlogger.Info || l.logger == nil {
		return
	}
	l.logger.InfoContext(ctx, fmt.Sprintf(msg, data...), appLogger.String("source", utils.FileWithLineNum()))
}

func (l appGORMLogger) Warn(ctx context.Context, msg string, data ...interface{}) {
	if l.config.LogLevel < gormlogger.Warn || l.logger == nil {
		return
	}
	l.logger.WarnContext(ctx, fmt.Sprintf(msg, data...), appLogger.String("source", utils.FileWithLineNum()))
}

func (l appGORMLogger) Error(ctx context.Context, msg string, data ...interface{}) {
	if l.config.LogLevel < gormlogger.Error || l.logger == nil {
		return
	}
	l.logger.ErrorContext(ctx, fmt.Sprintf(msg, data...), appLogger.String("source", utils.FileWithLineNum()))
}

func (l appGORMLogger) Trace(ctx context.Context, begin time.Time, fc func() (string, int64), err error) {
	if l.config.LogLevel <= gormlogger.Silent || l.logger == nil {
		return
	}

	elapsed := time.Since(begin)
	fields := func(sql string, rows int64) []appLogger.Field {
		return []appLogger.Field{
			appLogger.String("source", utils.FileWithLineNum()),
			appLogger.Duration("elapsed", elapsed),
			appLogger.Float64("elapsed_ms", float64(elapsed.Nanoseconds())/1e6),
			appLogger.Int64("rows", rows),
			appLogger.String("sql", sql),
		}
	}

	switch {
	case err != nil && l.config.LogLevel >= gormlogger.Error && (!errors.Is(err, gormlogger.ErrRecordNotFound) || !l.config.IgnoreRecordNotFoundError):
		sql, rows := l.sqlAndRows(fc)
		traceFields := fields(sql, rows)
		traceFields = append(traceFields, appLogger.Error(err))
		l.logger.ErrorContext(ctx, "gorm sql error", traceFields...)
	case l.config.SlowThreshold != 0 && elapsed > l.config.SlowThreshold && l.config.LogLevel >= gormlogger.Warn:
		sql, rows := l.sqlAndRows(fc)
		traceFields := fields(sql, rows)
		traceFields = append(traceFields, appLogger.Duration("slow_threshold", l.config.SlowThreshold))
		l.logger.WarnContext(ctx, "gorm slow sql", traceFields...)
	case l.logSQL && l.config.LogLevel == gormlogger.Info:
		sql, rows := l.sqlAndRows(fc)
		l.logger.InfoContext(ctx, "gorm sql", fields(sql, rows)...)
	}
}

func (l appGORMLogger) ParamsFilter(_ context.Context, sql string, params ...interface{}) (string, []interface{}) {
	if l.config.ParameterizedQueries {
		return sql, nil
	}
	return sql, params
}

func (l appGORMLogger) sqlAndRows(fc func() (string, int64)) (string, int64) {
	sql, rows := fc()
	if !l.logSQL {
		sql = "[SQL hidden]"
	}
	return sql, rows
}

func toGORMLogLevel(level LogLevel) gormlogger.LogLevel {
	switch level {
	case LogLevelSilent:
		return gormlogger.Silent
	case LogLevelError:
		return gormlogger.Error
	case LogLevelInfo:
		return gormlogger.Info
	case LogLevelWarn, "":
		return gormlogger.Warn
	default:
		return gormlogger.Warn
	}
}
