# Database

项目内部的 GORM 数据库客户端封装，负责创建连接、应用连接池配置、设置 GORM 日志与可选的读写分离、OpenTelemetry 链路追踪和连接池指标。

数据库结构由 `internal/migrations` 中的 Goose SQL 迁移管理。本项目不使用 GORM `AutoMigrate`；`MigrationConfig` 仅映射 GORM 的迁移相关选项，不负责执行项目迁移。应用数据库连接建立后，可把同一个 GORM `*gorm.DB` 传给 Goose Runner 执行嵌入的 SQL 迁移：

```go
import (
	"context"

	"github.com/surge-go/simple-admin/internal/migrations"
)

ctx := context.Background()
runner, err := migrations.NewRunner(db)
if err != nil {
	return err
}
if _, err := runner.Up(ctx); err != nil {
	return err
}
```

迁移通常由部署任务执行；如果由应用启动时执行，所有实例应使用相同迁移文件，Runner 会用 PostgreSQL session advisory lock 串行化迁移。Runner 不负责关闭数据库连接。

## 快速开始

```go
package main

import (
	"context"
	"log"
	"time"

	"github.com/surge-go/simple-admin/internal/infra/database"
	"github.com/surge-go/simple-admin/internal/models"
)

func main() {
	db, err := database.NewClient(&database.Config{
		Driver: database.DriverPostgres,
		DSN:    "host=localhost user=app password=secret dbname=app port=5432 sslmode=disable",
		Pool: &database.PoolConfig{
			MaxOpenConns:    20,
			MaxIdleConns:    10,
			ConnMaxLifetime: time.Hour,
			ConnMaxIdleTime: 10 * time.Minute,
		},
	})
	if err != nil {
		log.Fatal(err)
	}
	defer func() {
		if err := database.Close(db); err != nil {
			log.Printf("close database: %v", err)
		}
	}()

	ctx := context.Background()
	var users []models.User
	if err := db.WithContext(ctx).Find(&users).Error; err != nil {
		log.Fatal(err)
	}
}
```

## 配置文件示例

`Config` 支持 JSON/YAML/mapstructure 标签，可由应用配置层反序列化。Duration 使用 Go duration 字符串（如 `30m`、`1h`）；`Driver` 和 `DSN` 必须配置。

```yaml
database:
  driver: postgres
  dsn: "host=127.0.0.1 user=app password=${DB_PASSWORD} dbname=simple_admin port=5432 sslmode=disable"
  pool:
    max_open_conns: 20
    max_idle_conns: 10
    conn_max_lifetime: 1h
    conn_max_idle_time: 10m
  logger:
    level: warn
    log_sql: false
    slow_threshold: 200ms
    ignore_record_not_found_error: true
    parameterized_queries: true
  monitoring:
    tracing_enabled: true
    metrics_enabled: true
```

环境变量展开由应用的配置加载器负责；本包不会替 DSN 展开 `${DB_PASSWORD}`。不要把真实密码提交到配置文件或日志。

## 日志

`NewClient` 使用不输出内容的项目 logger，避免数据库包隐式创建日志输出。要将 GORM 错误、慢查询或 SQL 明细接入应用日志，使用 `NewClientWithLogger` 传入已初始化的 `internal/infra/logger` logger。GORM 日志会沿用该 logger 的输出、级别和 context trace 字段。两层级别都会生效：GORM `Level` 决定哪些数据库事件会提交给项目 logger，项目 logger 的级别再决定是否实际写出。要看到普通 SQL，项目 logger 至少设为 `info`；要看到慢查询，至少设为 `warn`；要看到 SQL 错误，至少设为 `error`。项目 logger 级别更严格时，对应事件会被过滤。

```go
import (
	"github.com/surge-go/simple-admin/internal/infra/database"
	"github.com/surge-go/simple-admin/internal/infra/logger"
)

logClient, err := logger.New(&logger.Config{Output: logger.OutputStdout})
if err != nil {
	return err
}

db, err := database.NewClientWithLogger(cfg, logClient)
if err != nil {
	return err
}
```

默认 GORM 日志级别是 `warn`：记录慢查询和错误，不记录普通 SQL；即使出现慢查询或错误，SQL 文本也默认隐藏。开发调试时可显式启用 SQL 日志：

```go
Logger: &database.LoggerConfig{
	Level:                database.LogLevelInfo,
	LogSQL:               true,
	SlowThreshold:        200 * time.Millisecond,
	ParameterizedQueries: true,
}
```

开启 `LogSQL` 会把 SQL 写入日志。生产环境应谨慎使用；启用 `ParameterizedQueries` 可避免参数值进入 SQL 日志。若 `LogSQL` 关闭，慢查询和错误仍记录事件信息，但 SQL 显示为 `[SQL hidden]`。

## 连接池

`Pool` 为可选配置；未设置时保留 `database/sql` 默认行为。生产环境通常应根据数据库连接上限、实例数和负载配置 `MaxOpenConns`、`MaxIdleConns`、`ConnMaxLifetime` 与 `ConnMaxIdleTime`。配置值中的时长使用 `time.Duration`。

## 读写分离

设置 `Resolver` 可注册 GORM dbresolver。`Config.DSN` 是默认写库；`Sources` 可配置额外写库，`Replicas` 配置只读库，当前支持的负载策略为 `random`。DSN 按主驱动解析；主库和副本需使用相同数据库类型。

```go
Resolver: &database.ResolverConfig{
	Replicas: []string{
		"host=replica-1 user=app password=secret dbname=app",
		"host=replica-2 user=app password=secret dbname=app",
	},
	Policy: database.ResolverPolicyRandom,
},
```

常规 GORM 写操作路由到写库，读操作可路由到副本。对无法可靠判断读写意图的 Raw SQL，应显式使用 `dbresolver.Read` 或 `dbresolver.Write`。带 `FOR UPDATE`、`FOR SHARE`、`FOR NO KEY UPDATE`、`FOR KEY SHARE` 或 `LOCK IN SHARE MODE` 的 Raw/Row SQL 会由本包保护性地路由到写库。

```go
import "gorm.io/plugin/dbresolver"

var users []models.User
err := db.Clauses(dbresolver.Read).WithContext(ctx).Find(&users).Error
if err != nil {
	return err
}

err = db.Clauses(dbresolver.Write).WithContext(ctx).
	Exec("UPDATE sys_user SET status = ? WHERE id = ?", 1, userID).Error
```

启用 Resolver 时必须通过 `database.Close(db)` 关闭客户端，以同时释放主库及副本连接池。

## 链路追踪与指标

```go
Monitoring: &database.MonitoringConfig{
	TracingEnabled: true,
	MetricsEnabled: true,
	TracerProvider: tracerProvider, // 可选；nil 时使用全局 Provider
	MeterProvider:  meterProvider,  // 可选；nil 时使用全局 Provider
},
```

链路追踪使用 GORM OpenTelemetry tracing 插件，并且不记录查询变量。指标来自 `database/sql` 连接池统计，包括打开/使用中/空闲连接、等待次数与时长、连接关闭计数等；包只注册 OpenTelemetry instruments，不负责创建 exporter、Prometheus registry 或 HTTP 指标端点。

使用自定义 MeterProvider 时，应确保其 reader/exporter 由应用层配置。关闭客户端时调用 `database.Close(db)`，以注销本包注册的指标回调并关闭连接池。

## 配置字段参考

下表覆盖 `Config` 及其所有嵌套配置字段。YAML 示例未写出的字段使用表中说明的默认行为；`nil` 配置组表示不设置该组，通常保留依赖库默认值。

### 顶层 `Config`

| 字段 | YAML 键 | 默认/说明 |
| --- | --- | --- |
| `Driver` | `driver` | 必填。`mysql`、`postgres`、`sqlite` 或 `sqlserver`。当前项目迁移 SQL 面向 PostgreSQL。 |
| `DSN` | `dsn` | 必填。格式由驱动决定；请从密钥管理或环境配置注入凭据。 |
| `Pool` | `pool` | `nil` 时使用 `database/sql` 默认连接池行为。 |
| `GORM` | `gorm` | `nil` 时采用 GORM 默认初始化选项；本包固定启用错误转换。 |
| `Naming` | `naming` | `nil` 时使用 GORM 默认命名策略。 |
| `Logger` | `logger` | `nil` 时使用包内安全默认设置：warn 级别、普通 SQL 关闭、SQL 文本隐藏。 |
| `Migration` | `migration` | 只影响 GORM 的建表外键选项，不执行 Goose 迁移。 |
| `Monitoring` | `monitoring` | `nil` 或两个开关均关闭时不安装 tracing/metrics。 |
| `Resolver` | `resolver` | `nil` 时不注册读写分离插件。 |

### `PoolConfig`

| 字段 | YAML 键 | 默认/说明 |
| --- | --- | --- |
| `MaxOpenConns` | `max_open_conns` | `0` 不限制最大打开连接数。 |
| `MaxIdleConns` | `max_idle_conns` | `0` 不保留空闲连接；不能大于非零的 `MaxOpenConns`。 |
| `ConnMaxLifetime` | `conn_max_lifetime` | `0` 不因连接生命周期而回收；非负 `time.Duration`。 |
| `ConnMaxIdleTime` | `conn_max_idle_time` | `0` 不因空闲时长而回收；非负 `time.Duration`。 |

### `GORMConfig`

| 字段 | YAML 键 | 默认/说明 |
| --- | --- | --- |
| `SkipDefaultTransaction` | `skip_default_transaction` | `false`。设为 `true` 禁止 GORM 为写操作自动开启默认事务。 |
| `DryRun` | `dry_run` | `false`。设为 `true` 只构造 SQL，不执行。 |
| `PrepareStmt` | `prepare_stmt` | `false`。设为 `true` 启用 prepared statement 缓存。 |
| `DisableNestedTransaction` | `disable_nested_transaction` | `false`。设为 `true` 禁止嵌套事务使用 SavePoint。 |
| `AllowGlobalUpdate` | `allow_global_update` | `false`。设为 `true` 允许无条件全表更新/删除，生产环境慎用。 |
| `DisableAutomaticPing` | `disable_automatic_ping` | `false`。设为 `true` 禁止初始化后自动 Ping。 |

### `NamingConfig`

| 字段 | YAML 键 | 默认/说明 |
| --- | --- | --- |
| `TablePrefix` | `table_prefix` | 空字符串，不添加表名前缀。 |
| `SingularTable` | `singular_table` | `false`，使用 GORM 默认复数表名。 |
| `NoLowerCase` | `no_lower_case` | `false`，使用 GORM 默认小写命名。 |
| `IdentifierMaxLength` | `identifier_max_length` | `0` 使用 GORM 默认标识符长度；不能为负数。 |

### `LoggerConfig`

| 字段 | YAML 键 | 默认/说明 |
| --- | --- | --- |
| `Level` | `level` | 空值按 `warn` 处理。可选 `silent`、`error`、`warn`、`info`。 |
| `LogSQL` | `log_sql` | `false`。普通 SQL 仅在 `Level: info` 且该项为 `true` 时输出。 |
| `SlowThreshold` | `slow_threshold` | `0` 使用 GORM logger 默认慢查询阈值；不能为负数。 |
| `IgnoreRecordNotFoundError` | `ignore_record_not_found_error` | `false`。设为 `true` 后不记录 `gorm.ErrRecordNotFound`。 |
| `ParameterizedQueries` | `parameterized_queries` | `false`。设为 `true` 时 SQL 日志隐藏参数值，建议启用。 |

### `MigrationConfig`

| 字段 | YAML 键 | 默认/说明 |
| --- | --- | --- |
| `DisableForeignKeyConstraintWhenMigrating` | `disable_foreign_key_constraint_when_migrating` | `false`，对应 GORM 的建表选项。本项目用 Goose SQL 管理结构，不用 GORM AutoMigrate。 |

### `MonitoringConfig`

| 字段 | YAML 键 | 默认/说明 |
| --- | --- | --- |
| `TracingEnabled` | `tracing_enabled` | `false`。启用 GORM OpenTelemetry tracing。 |
| `MetricsEnabled` | `metrics_enabled` | `false`。启用 `database/sql` 连接池指标。 |
| `TracerProvider` | 不支持序列化 | 运行时 `trace.TracerProvider`；未设置时使用 OpenTelemetry 全局 Provider。通过 Go 代码注入。 |
| `MeterProvider` | 不支持序列化 | 运行时 `metric.MeterProvider`；未设置时使用 OpenTelemetry 全局 Provider。通过 Go 代码注入。 |

### `ResolverConfig`

| 字段 | YAML 键 | 默认/说明 |
| --- | --- | --- |
| `Sources` | `sources` | 写库 DSN 列表；为空时使用顶层 `DSN` 作为默认写库。 |
| `Replicas` | `replicas` | 只读副本 DSN 列表；为空时没有副本可供读路由。 |
| `Policy` | `policy` | 空值使用 dbresolver 默认策略；当前显式支持 `random`。 |
| `TraceResolverMode` | `trace_resolver_mode` | `false`。是否在 SQL 日志中标识 resolver 模式。 |

`Config.Validate()` 只校验静态配置，不建立连接，也不验证数据库可达性。`NewClient` 默认保留 GORM 的自动 Ping 行为；测试、延迟连接或由健康检查负责探活时，可设置 `GORM.DisableAutomaticPing`。数据库错误转换固定启用，可使用 `errors.Is` 判断 GORM 标准错误，例如 `gorm.ErrDuplicatedKey`。
