# Tracing 基础设施

本包封装 OpenTelemetry Tracing 初始化，提供 OTLP/gRPC 与 OTLP/HTTP protobuf 导出、资源属性、采样、W3C Trace Context/Baggage 传播和 Runtime 生命周期管理。`New` 负责本地配置校验及 exporter/provider 创建；它不会验证 collector 是否可达，导出错误会在 span 导出或关闭时返回。

## 快速开始

```go
runtime, err := tracing.New(ctx, &tracing.Config{
    Enabled:        true,
    ServiceName:    "orders-api",
    ServiceVersion: "1.2.0",
    Environment:    "production",
    Endpoint:       "otel-collector:4317",
    Protocol:       tracing.ProtocolGRPC,
    SampleRatio:    0.1,
})
if err != nil {
    return err
}
defer runtime.Shutdown(context.Background())

tracer := runtime.TracerProvider().Tracer("orders-api")
ctx, span := tracer.Start(ctx, "handle-order")
defer span.End()
_ = ctx
```

应用需要提供非 nil 的创建和关闭 context。`Shutdown` 会刷新并关闭本 Runtime 拥有的 SDK Provider/exporter，可重复并发调用；所有调用者共享首次关闭结果。建议在应用退出流程中使用有截止时间的 context，并在进程终止前完成关闭。

## 配置

`DefaultConfig()` 提供 gRPC 协议和 `SampleRatio: 1`。启用前仍须设置非空 `ServiceName` 和 `Endpoint`。未显式设置 `Protocol` 时默认使用 gRPC。

| 字段 | 说明 |
| --- | --- |
| `Enabled` | 是否创建 OTLP exporter 和 SDK TracerProvider。false 时使用 noop Provider，不需要 endpoint。 |
| `ServiceName` | 必填服务名，写入资源属性 `service.name`。 |
| `ServiceVersion` | 可选服务版本，写入 `service.version`。 |
| `Environment` | 可选部署环境，写入 `deployment.environment.name`。 |
| `Endpoint` | exporter 地址。gRPC 接受 `host:port` 或 `http[s]://host:port`；HTTP protobuf 接受 `host:port` 或带 scheme 的 URL。 |
| `Protocol` | `ProtocolGRPC`（默认）或 `ProtocolHTTPProto`。 |
| `Insecure` | 使用明文传输。若 URL scheme 为 `http`，必须为 true；HTTPS/TLS 场景保持 false。 |
| `Headers` | 发送给 collector 的 OTLP 请求头，例如鉴权头。请从安全配置源注入凭据。 |
| `SampleRatio` | Trace ID 比例采样，范围 `[0, 1]`；采样器为 ParentBased，子 span 遵循父 span 决策。 |
| `Batch` | 批处理队列与导出参数；零值使用 OpenTelemetry SDK 默认值。 |
| `Resource` | 附加字符串资源属性；不能覆盖 `service.name`、`service.version`、`deployment.environment.name`。 |

HTTP protobuf 示例：

```go
cfg := &tracing.Config{
    Enabled:     true,
    ServiceName: "orders-api",
    Endpoint:    "https://otel-collector:4318/v1/traces",
    Protocol:    tracing.ProtocolHTTPProto,
    SampleRatio: 1,
    Headers: map[string]string{
        "authorization": os.Getenv("OTEL_EXPORTER_AUTHORIZATION"),
    },
}
```

HTTP endpoint URL 可以包含 collector 路径（例如 `/v1/traces`），但不能包含 query、fragment 或 userinfo。gRPC URL 只接受 scheme 和 host，不接受路径；`http://` gRPC 地址需要 `Insecure: true`。不带 scheme 的 endpoint 使用 exporter 对应协议的默认安全配置；生产环境应明确配置 TLS 或明文策略。

### Batch

| 字段 | 说明 |
| --- | --- |
| `MaxQueueSize` | 批处理队列容量；零值使用 SDK 默认值。 |
| `MaxExportBatchSize` | 单批最大 span 数；配置时不能超过非零的 `MaxQueueSize`。 |
| `ScheduleDelay` | 批处理导出调度间隔。 |
| `ExportTimeout` | 单次导出超时，同时作为 OTLP exporter 超时。 |

批处理参数必须为非负值。配置的队列大小、单批大小和调度间隔会传递给 SDK batch span processor。

## Provider 接入

推荐将 Runtime 的 Provider 显式注入到使用 OpenTelemetry 的组件，避免隐式依赖进程级全局状态：

```go
runtime, err := tracing.New(ctx, cfg)
if err != nil {
    return err
}

httpClient := otelhttp.NewTransport(
    transport,
    otelhttp.WithTracerProvider(runtime.TracerProvider()),
    otelhttp.WithPropagators(runtime.Propagator()),
)
_ = httpClient
```

如应用其余部分使用 OpenTelemetry 全局 API，可调用 `runtime.InstallGlobal()` 安装全局 TracerProvider 与 propagator。该包只允许安装一个由本包管理的 Runtime；重复安装同一 Runtime 成功，安装另一个 Runtime 返回 `ErrGlobalProviderInstalled`。已关闭的 Runtime 安装会返回 `ErrRuntimeClosed`。安装是可选的，`New` 不会自动修改全局 Provider。

关闭 Runtime 不会恢复或替换全局 Provider/Propagator；全局安装适用于进程级生命周期，通常应在应用启动时安装一次，并在退出时关闭对应 Runtime。

## 上下文传播

默认 propagator 组合 W3C Trace Context 和 Baggage。可使用 `Inject` 将当前 span/baggage 写入 carrier，并通过 `Extract` 从上游 carrier 恢复 context：

```go
carrier := propagation.MapCarrier{}
runtime.Inject(ctx, carrier)

remoteCtx := runtime.Extract(context.Background(), carrier)
```

HTTP middleware 或 RPC instrumentation 通常可以直接使用 `runtime.Propagator()`，由 instrumentation 自动完成注入与提取。

## 生命周期与错误

- `New(ctx, cfg)` 校验配置并创建 Runtime。`cfg` 会在内部复制，后续修改原配置不会改变已创建的 exporter/provider。
- `Enabled: false` 时返回 noop Provider；传播器仍可用于提取或注入上下文。
- exporter 创建失败时，`New` 返回包装后的错误；endpoint 可通过本地校验不代表 collector 网络连接可用。
- 应用持有 Runtime 所有权，退出时调用 `Shutdown(ctx)`；不要在关闭共享 Runtime 后继续依赖它导出 span。
- `TracerProvider()`、`Propagator()` 对 nil receiver 或缺失内部值提供 noop/默认实现；nil Runtime 调用 `Shutdown` 是安全空操作。
- 该包不关闭应用自己的其他 OpenTelemetry Provider，也不管理应用外部创建的 exporter。
