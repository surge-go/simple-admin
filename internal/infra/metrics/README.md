# Metrics 基础设施

本包封装 OpenTelemetry 指标初始化，支持通过 OTLP/gRPC 和 OTLP/HTTP protobuf 导出指标，并管理资源属性及 Runtime 生命周期。`New` 会执行本地配置校验并创建导出器和 Provider；配置校验通过不代表 Collector 一定可达，导出错误会在数据导出或关闭时返回。

## 快速开始

```go
runtime, err := metrics.New(ctx, &metrics.Config{
    Enabled:        true,
    ServiceName:    "orders-api",
    ServiceVersion: "1.2.0",
    Environment:    "production",
    Endpoint:       "otel-collector:4317",
    Protocol:       metrics.ProtocolGRPC,
    Insecure:       true,
})
if err != nil {
    return err
}
defer runtime.Shutdown(shutdownCtx)

meter := runtime.MeterProvider().Meter("orders-api")
requests, err := meter.Int64Counter("http.server.requests")
if err != nil {
    return err
}
requests.Add(ctx, 1)
```

`New` 和 `Shutdown` 都要求传入非 nil 的 context。`Shutdown` 会刷新并关闭此 Runtime 所拥有的 MeterProvider 和导出器，支持重复及并发调用；所有调用者都会收到首次关闭操作的结果。建议应用退出时使用带截止时间的 context，并在进程终止前完成关闭。

## 配置

`DefaultConfig()` 返回默认关闭指标、使用 gRPC 协议的配置。`ServiceName` 始终必填；启用指标时还必须设置 `Endpoint`。未显式设置 `Protocol` 时默认使用 gRPC。

| 字段 | 说明 |
| --- | --- |
| `Enabled` | 是否创建 OTLP 导出器和 SDK MeterProvider。设为 `false` 时使用 noop Provider，无需设置 endpoint。 |
| `ServiceName` | 必填的服务名称，写入资源属性 `service.name`。 |
| `ServiceVersion` | 可选的服务版本，写入 `service.version`。 |
| `Environment` | 可选的部署环境，写入 `deployment.environment.name`。 |
| `Endpoint` | 导出器地址。gRPC 接受 `host:port` 或 `http[s]://host:port`；HTTP protobuf 接受 `host:port` 或带 scheme 的 URL。HTTP URL 可包含 Collector 路径。 |
| `Protocol` | `ProtocolGRPC`（默认）或 `ProtocolHTTPProto`。 |
| `Insecure` | endpoint 带 `http://` 时必须为 `true`；带 `https://` 时必须为 `false`。裸 `host:port` 地址根据此标志选择明文传输或 TLS。 |
| `Headers` | 发送给 Collector 的 OTLP 请求头，例如鉴权头。凭据应从安全的配置源注入。 |
| `ExportInterval` | 周期性导出的间隔；零值使用 SDK 默认值，负值无效。 |
| `ExportTimeout` | 单次导出的超时时间；零值使用 SDK 默认值，负值无效。 |
| `Resource` | 附加的字符串资源属性；不能覆盖 `service.name`、`service.version`、`deployment.environment.name`，key 不能为空或带首尾空白。 |

HTTP protobuf 配置示例：

```go
cfg := &metrics.Config{
    Enabled:     true,
    ServiceName: "orders-api",
    Endpoint:    "https://otel-collector:4318/v1/metrics",
    Protocol:    metrics.ProtocolHTTPProto,
    Headers: map[string]string{
        "authorization": os.Getenv("OTEL_EXPORTER_AUTHORIZATION"),
    },
}
```

HTTP endpoint URL 可以包含 Collector 路径，但不能包含 query、fragment 或 userinfo。gRPC URL 只接受 scheme 和 host，不接受路径，并要求显式端口。`http://` gRPC 地址需要设置 `Insecure: true`；`https://` 地址不能设置 `Insecure: true`。

## Provider 接入

建议显式注入 Runtime 的 MeterProvider，避免隐式依赖进程级全局状态：

```go
runtime, err := metrics.New(ctx, cfg)
if err != nil {
    return err
}

meter := runtime.MeterProvider().Meter("orders-api")
_ = meter
```

如果应用使用 OpenTelemetry 全局 Metrics API，可调用 `runtime.InstallGlobal()` 安装全局 MeterProvider。本包只允许安装一个由本包管理的 Runtime；重复安装同一 Runtime 会成功，安装其他 Runtime 或覆盖外部已安装的全局 Provider 会返回 `ErrGlobalProviderInstalled`。已关闭的 Runtime 安装时会返回 `ErrRuntimeClosed`。`New` 不会自动修改全局 Provider。

关闭当前全局 Runtime 时，本包会恢复包初始化时记录的全局 MeterProvider，并允许安装新的 Runtime。全局安装适用于进程级生命周期，通常在应用启动时安装一次，并在退出时关闭对应 Runtime。

## 生命周期与错误

- `New(ctx, cfg)` 会校验配置并创建 Runtime。配置中的 map 会被复制，后续修改原配置不会影响已创建的导出器或 Provider。
- `Enabled: false` 时返回 noop Provider；`ServiceName` 仍然必填。
- 导出器创建失败时，`New` 会返回带上下文的错误；endpoint 本地校验通过不代表 Collector 网络连接可用。
- Runtime 由应用持有和管理，退出时应调用 `Shutdown(ctx)`；关闭后不要继续依赖该 Runtime 导出指标。
- `MeterProvider()` 在接收者为 nil 或内部 Provider 缺失时返回 noop Provider；nil Runtime 调用 `Shutdown` 不会产生错误。
- `WithReaderFactory` 可在测试或自定义集成中替换默认 OTLP reader。Runtime 接管 factory 返回的 reader，并在关闭时通过 MeterProvider 关闭它；若 factory 返回错误时同时返回了 reader，该 reader 也会被关闭。
- 本包只管理自身创建的 Provider、reader 和导出器，不会关闭应用自行创建的其他 OpenTelemetry Provider。
