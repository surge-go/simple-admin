# Logger

项目内部的结构化日志封装，底层使用 Zap，同时提供标准 `log/slog` 适配、OpenTelemetry trace context 字段注入、纯文本日志和多目标输出。业务代码只依赖本包的 `Logger`、`Field` 和配置类型，不直接依赖 Zap。

## 快速开始

```go
package main

import (
	"log"

	"github.com/surge-go/simple-admin/internal/infra/logger"
)

func main() {
	logClient, err := logger.New(&logger.Config{
		Level:  logger.LevelInfo,
		Format: logger.FormatJSON,
		Output: logger.OutputStdout,
		AddCaller: true,
		InitialFields: map[string]string{
			"service": "simple-admin",
			"env":     "production",
		},
	})
	if err != nil {
		log.Fatal(err)
	}
	defer logClient.Sync()

	logClient.Info("server started",
		logger.String("address", ":8080"),
		logger.Int("port", 8080),
	)
}
```

`New(nil)` 或无显式配置时的默认值为 `info` 级别、JSON 格式、`stdout` 输出和 `error` 级别 stacktrace。配置会在创建 logger 时校验；无效配置返回错误。

## 日志接口

结构化 `Logger` 提供 `Debug`、`Info`、`Warn`、`Error`、`DPanic`、`Panic`、`Fatal` 等级别方法，以及对应的 `*Context` 方法。传入 context 的方法会在 context 含有效 OpenTelemetry span 时自动附加 `trace_id`、`span_id` 和 `trace_sampled` 字段。

```go
logClient.InfoContext(ctx, "request completed",
	logger.String("method", "GET"),
	logger.Int("status", 200),
	logger.Duration("duration", elapsed),
)
```

常用字段构造器包括 `String`、`Bool`、`Int`、`Int64`、`Float64`、`Duration`、`Time`、`Error`、`NamedError` 和 `Any`。也可使用 `With` 创建带固定字段的 logger，或使用 `Named` 设置 logger 名称：

```go
requestLog := logClient.Named("http").With(logger.String("request_id", requestID))
requestLog.Info("request completed", logger.Int("status", 200))
```

`Sugar()` 提供低类型约束接口，支持 `Infof`、`Infow` 等格式化和键值方法。优先使用结构化 `Logger` 和类型化字段；需要兼容格式化日志调用时再使用 Sugar。

## 输出配置

`Output` 用于单一目标，支持 `stdout`、`stderr` 和 `file`。`Outputs` 支持多个目标，但不能与 `Output` 同时设置。文件输出必须配置 `File` 路径；父目录会在写入时自动创建。

```go
cfg := &logger.Config{
	Level:   logger.LevelInfo,
	Format:  logger.FormatJSON,
	Outputs: []logger.Output{logger.OutputStdout, logger.OutputFile},
	File:    "runtime/logs/app.log",
	Rotation: &logger.RotationConfig{
		MaxSize:    100, // MB
		MaxAge:     7,   // 天
		MaxBackups: 10,
		Compress:   true,
	},
}
logClient, err := logger.New(cfg)
```

`Rotation` 为 `nil` 时，文件不会自动轮转；生产环境使用文件输出时建议配置轮转。`ErrorOutput` 指定 Zap 内部错误的输出目标，支持 `stdout`、`stderr` 或文件路径。

## 格式、采样与编码

- `FormatJSON` 适用于生产环境和日志采集；`FormatConsole` 适用于本地阅读。Console 同时写文件时不使用颜色，避免日志文件包含 ANSI 转义序列。
- `Sampling` 可按每秒窗口对重复日志采样。`Initial` 指窗口内完整保留的前 N 条，之后每 `Thereafter` 条保留一条；启用时两项必须大于 0。
- `Encoder` 可按需覆盖 `msg`、`level`、`ts`、`logger`、`caller`、`stacktrace` 等字段名，以及时间、时长和级别编码。
- `StacktraceLevel` 支持 `none`、`error`、`panic`、`fatal`；默认从 `error` 开始记录。

## slog 适配

`Slog()` 返回共享相同级别、输出和采样配置的 `*slog.Logger`，可用于要求标准库 slog 接口的组件：

```go
slogLogger := logClient.Slog().With("component", "worker")
slogLogger.InfoContext(ctx, "job completed", "job_id", jobID)
```

## 纯文本日志

`InfoText` 用于已经格式化好的访问日志等文本。它只输出消息本身，不添加 JSON 外层、时间、级别、caller、logger 名称或固定字段；仍遵循配置的 info 级别、输出目标和采样设置。

```go
logClient.InfoText(`[HTTP] 200 12ms GET "/health"`)
```

纯文本日志与结构化日志共用 writer，调用 `Sync` 时一并同步。`With` 和 `Named` 派生的 logger 仍可使用 `InfoText`，但不会把附加字段写入纯文本消息。

## 生命周期与注意事项

- 在进程退出前调用 `Sync()`，并处理其返回错误；控制台设备不支持同步的错误会被忽略，文件写入/同步错误会返回。
- `Fatal` 会记录日志并退出进程，`Panic` 会记录日志并触发 panic；不应在可恢复的业务错误处理中使用。
- 不要记录密码、访问令牌等敏感信息。
- `Nop()` 返回不产生输出的 logger，适用于可选日志依赖和测试。
