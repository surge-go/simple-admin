# Redis 客户端

本包基于 `github.com/redis/go-redis/v9` 创建 Redis 客户端，支持 Standalone、Sentinel 和 Cluster 拓扑，并可选启用 TLS、OpenTelemetry tracing 和 metrics。`NewClient` 只执行本地配置校验并创建客户端，不会连接或探测 Redis 服务；连接错误会在后续命令执行时返回。

## 快速开始

```go
client, err := redis.NewClient(&redis.Config{
    Addrs: []string{"127.0.0.1:6379"},
    Pool: &redis.PoolConfig{
        Size:         100,
        MinIdleConns: 10,
    },
})
if err != nil {
    return err
}
defer client.Close()

value, err := client.Get(ctx, "cache-key").Result()
if err != nil {
    return err
}
_ = value
```

未设置 `Mode` 时使用 Standalone。单机模式必须提供一个 `host:port` 地址。`NewClient` 返回 `go-redis` 的 `UniversalClient`，Redis 命令可直接使用 go-redis API。

## 拓扑模式

### Standalone

```go
client, err := redis.NewClient(&redis.Config{
    Mode:     redis.ModeStandalone, // 可省略
    Addrs:    []string{"redis.example.com:6379"},
    DB:       2,
    Username: "app-user",
    Password: os.Getenv("REDIS_PASSWORD"),
})
```

支持 TCP、TCP4、TCP6 和 Unix domain socket。使用 Unix socket 时，`Addrs` 中填写 socket 文件路径，且只能用于 Standalone。

### Sentinel

```go
client, err := redis.NewClient(&redis.Config{
    Mode:     redis.ModeSentinel,
    Addrs:    []string{"sentinel-1:26379", "sentinel-2:26379"},
    Username: "data-user",
    Password: os.Getenv("REDIS_PASSWORD"),
    Sentinel: &redis.SentinelConfig{
        MasterName: "primary",
        Username:   "sentinel-user",
        Password:   os.Getenv("REDIS_SENTINEL_PASSWORD"),
    },
})
```

`Config.Username/Password` 用于 Redis 数据节点；`Sentinel.Username/Password` 用于 Sentinel 服务。`Sentinel.MasterName` 必填。`ReplicaOnly` 将连接限制为副本；`UseDisconnectedReplicas` 控制是否使用已断开连接的副本。

### Cluster

```go
client, err := redis.NewClient(&redis.Config{
    Mode:  redis.ModeCluster,
    Addrs: []string{"redis-1:6379", "redis-2:6379", "redis-3:6379"},
    Cluster: &redis.ClusterConfig{
        ReadOnly:       true,
        RouteByLatency: true,
    },
})
```

Cluster 只支持 DB 0。`Cluster` 配置组可以省略；组内零值会交由 go-redis 使用默认行为。连接池配置按节点生效，集群的总连接数会随节点数增加。

## TLS

```go
client, err := redis.NewClient(&redis.Config{
    Addrs: []string{"redis.example.com:6379"},
    TLS: &redis.TLSConfig{
        Enabled:    true,
        ServerName: "redis.example.com",
        CAFile:     "/etc/redis/ca.pem",
        CertFile:   "/etc/redis/client.pem", // 双向 TLS 可选
        KeyFile:    "/etc/redis/client-key.pem",
    },
})
```

TLS 最低版本为 TLS 1.2。默认校验服务端证书；配置 `CAFile` 时，会在系统证书池基础上追加该 CA。客户端证书和密钥必须同时配置。`InsecureSkipVerify` 会关闭服务端证书校验，仅应用于明确受控的测试或兼容场景，不建议在生产环境启用。TLS 相关选项必须在 `TLS.Enabled` 为 `true` 时使用。

## OpenTelemetry

```go
client, err := redis.NewClient(&redis.Config{
    Addrs: []string{"127.0.0.1:6379"},
    Monitoring: &redis.MonitoringConfig{
        TracingEnabled: true,
        MetricsEnabled: true,
        // TracerProvider: tracerProvider,
        // MeterProvider:  meterProvider,
    },
})
```

应用负责创建、配置和关闭 TracerProvider、MeterProvider 及 exporter，并可通过 `MonitoringConfig` 注入 Provider。未注入时使用 redisotel/OpenTelemetry 的全局 Provider。

Tracing 默认不将完整 Redis 命令写入 span；设置 `DBStatementEnabled` 后才记录命令语句，需评估 key、参数等敏感信息风险。`CallerEnabled` 控制是否记录调用方信息；`DialEnabled` 控制是否为底层连接拨号创建 span。

启用 metrics 后，客户端会包装 metrics 生命周期管理。调用 `Close` 会停止该客户端的 Redis metrics 采集并关闭连接池，但不会关闭应用共享的 MeterProvider。未启用 metrics 时，返回原生 go-redis 客户端。

## 配置字段

未提供可选配置组时使用 go-redis 对应选项的默认值；配置组中的零值也会传递给 go-redis，因此除本包明确校验的约束外，运行时效果遵循当前依赖版本的定义。

| 字段 | 说明 |
| --- | --- |
| `Mode` | 拓扑模式：`standalone`、`sentinel` 或 `cluster`；默认 `standalone`。 |
| `Addrs` | Redis 或 Sentinel 地址列表；必填，地址须为 `host:port`，Unix 模式填写 socket 路径。 |
| `Network` | 网络类型：`tcp`、`tcp4`、`tcp6` 或 `unix`；默认 `tcp`。非 Standalone 模式仅支持 TCP。 |
| `DB` | Redis 数据库编号；须大于等于 0，Cluster 必须为 0。 |
| `Username`、`Password` | Redis 数据节点认证信息；密码应从安全配置源注入。 |
| `ClientName` | Redis 客户端名称。 |
| `Protocol` | RESP 协议版本：`0` 使用 go-redis 默认值，或指定 `2`、`3`。 |
| `DisableIdentity`、`IdentitySuffix` | 控制 go-redis 客户端身份上报及其后缀。 |
| `Timeout` | 连接、读写及连接池等待超时设置，见下表。 |
| `Retry` | 命令重试次数及退避设置，见下表。 |
| `Pool` | 连接池容量、空闲连接与连接生命周期，见下表。 |
| `Buffer` | 每条连接的读写缓冲区大小，单位为字节；零值保留 go-redis 默认值。 |
| `TLS` | TLS 与证书设置，见 TLS 一节。 |
| `Monitoring` | Tracing、metrics 与 Provider 设置，见 OpenTelemetry 一节。 |
| `Sentinel` | Sentinel 专用设置；仅 Sentinel 模式可用。 |
| `Cluster` | Cluster 专用设置；仅 Cluster 模式可用。 |

### Timeout

| 字段 | 说明 |
| --- | --- |
| `DialTimeout` | 建立连接的超时时间。 |
| `DialerRetries` | 建立连接时的重试次数。 |
| `DialerRetryTimeout` | 连接重试间隔。 |
| `ReadTimeout` | 读取超时；允许 `-1`、`-2` 或非负值，含义遵循 go-redis。 |
| `WriteTimeout` | 写入超时；允许 `-1`、`-2` 或非负值，含义遵循 go-redis。 |
| `PoolTimeout` | 等待连接池连接的超时时间。 |
| `ContextTimeoutEnabled` | 是否使用命令 context 的超时。 |

### Retry

| 字段 | 说明 |
| --- | --- |
| `MaxRetries` | 最大重试次数；允许 `-1` 或非负值。 |
| `MinRetryBackoff` | 最小重试退避时间；允许 `-1` 或非负值。 |
| `MaxRetryBackoff` | 最大重试退避时间；允许 `-1` 或非负值，配置上下限时最小值不得大于最大值。 |

### Pool

| 字段 | 说明 |
| --- | --- |
| `FIFO` | 是否按先进先出策略使用连接。 |
| `Size` | 连接池大小；Cluster 模式下按节点计算。 |
| `MaxConcurrentDials` | 并发拨号连接数上限。 |
| `MinIdleConns`、`MaxIdleConns` | 最小和最大空闲连接数；非零最大值不得小于最小值。 |
| `MaxActiveConns` | 活跃连接数上限；非零上限不得小于最小空闲连接数。 |
| `ConnMaxIdleTime` | 连接最大空闲时间。 |
| `ConnMaxLifetime` | 连接最大生命周期。 |
| `ConnMaxLifetimeJitter` | 连接生命周期随机抖动；仅在设置 `ConnMaxLifetime` 时使用。 |

### Sentinel 与 Cluster

| 配置组 | 字段 | 说明 |
| --- | --- | --- |
| `Sentinel` | `MasterName` | 主节点名称，必填。 |
| `Sentinel` | `Username`、`Password` | Sentinel 服务认证信息，与数据节点认证分开配置。 |
| `Sentinel` | `ReplicaOnly` | 仅使用副本。 |
| `Sentinel` | `UseDisconnectedReplicas` | 是否使用已断开连接的副本。 |
| `Cluster` | `MaxRedirects` | 集群重定向次数限制。 |
| `Cluster` | `ReadOnly` | 是否允许将只读命令路由到副本。 |
| `Cluster` | `RouteByLatency`、`RouteRandomly` | 副本路由策略；不能同时启用。 |
| `Cluster` | `FailingTimeoutSeconds` | 节点故障判定超时时间（秒）。 |
| `Cluster` | `ClusterStateReloadInterval` | 集群状态重新加载间隔。 |

## 配置校验与生命周期

`Config.Validate` 会校验拓扑与地址、网络类型、DB 编号、协议版本、超时/重试/连接池取值、TLS 文件配置及 Sentinel/Cluster 配置组是否与模式匹配。它不解析 DNS、不读取 TLS 文件，也不探测 Redis；TLS 文件在 `NewClient` 构建 TLS 配置时读取，因此相关文件错误会在创建客户端时返回。

客户端由应用负责关闭。无论是否启用监控，都应在使用结束时调用 `client.Close()`；metrics 启用时该调用还会停止本客户端注册的 Redis metrics 采集。不要因关闭 Redis 客户端而关闭由应用共享的 OpenTelemetry Provider。
