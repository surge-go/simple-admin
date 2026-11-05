package metrics

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"sort"
	"strings"
	"sync"
	"sync/atomic"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp"
	"go.opentelemetry.io/otel/metric"
	metricnoop "go.opentelemetry.io/otel/metric/noop"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
)

var (
	ErrGlobalProviderInstalled = errors.New("metrics global provider is already installed")
	ErrRuntimeClosed           = errors.New("metrics runtime is closed")
	initialGlobalProvider      = otel.GetMeterProvider()
)

// ReaderFactory 为 Runtime 创建 reader。Runtime 接管返回的 reader，
// 并通过 MeterProvider.Shutdown 关闭它。
type ReaderFactory func(context.Context, Config) (sdkmetric.Reader, error)

type options struct {
	readerFactory ReaderFactory
}

// Option 用于自定义 Runtime 的创建过程。
type Option func(*options)

// WithReaderFactory 使用自定义 reader 替代默认的 OTLP 导出器和周期性 reader。
// 返回的 reader 必须负责管理并关闭其使用的导出器。
func WithReaderFactory(factory ReaderFactory) Option {
	return func(o *options) { o.readerFactory = factory }
}

// Runtime 管理由 New 创建的 MeterProvider 和 reader。
type Runtime struct {
	provider metric.MeterProvider
	shutdown func(context.Context) error

	closed       atomic.Bool
	shutdownOnce sync.Once
	shutdownErr  error
}

var globalInstall struct {
	sync.Mutex
	runtime *Runtime
}

// New 根据配置创建指标 Runtime。
func New(ctx context.Context, cfg *Config, opts ...Option) (*Runtime, error) {
	if ctx == nil {
		return nil, errors.New("metrics context is nil")
	}
	if cfg == nil {
		return nil, errors.New("metrics config is nil")
	}

	config := cloneConfig(cfg)
	if err := config.Validate(); err != nil {
		return nil, err
	}
	settings := options{}
	for _, opt := range opts {
		if opt != nil {
			opt(&settings)
		}
	}
	if !config.Enabled {
		return &Runtime{
			provider: metricnoop.NewMeterProvider(),
			shutdown: func(context.Context) error { return nil },
		}, nil
	}

	runtimeResource, err := buildResource(config)
	if err != nil {
		return nil, fmt.Errorf("create metrics resource: %w", err)
	}

	var reader sdkmetric.Reader
	if settings.readerFactory != nil {
		reader, err = settings.readerFactory(ctx, config)
		if err != nil {
			if reader != nil {
				err = errors.Join(err, reader.Shutdown(ctx))
			}
			return nil, fmt.Errorf("create metrics reader: %w", err)
		}
		if reader == nil {
			return nil, errors.New("create metrics reader: reader factory returned nil")
		}
	} else {
		reader, err = newOTLPReader(ctx, config)
		if err != nil {
			return nil, fmt.Errorf("create OTLP metrics reader: %w", err)
		}
	}

	provider := sdkmetric.NewMeterProvider(
		sdkmetric.WithResource(runtimeResource),
		sdkmetric.WithReader(reader),
	)
	return &Runtime{provider: provider, shutdown: provider.Shutdown}, nil
}

// MeterProvider 返回 Runtime 的 Provider，供显式注入指标埋点组件使用。
func (r *Runtime) MeterProvider() metric.MeterProvider {
	if r == nil || r.provider == nil {
		return metricnoop.NewMeterProvider()
	}
	return r.provider
}

// InstallGlobal 在当前进程中安装此 Runtime 的全局 Provider，且仅允许安装一个。
func (r *Runtime) InstallGlobal() error {
	if r == nil || r.provider == nil {
		return errors.New("metrics runtime is nil")
	}
	if r.closed.Load() {
		return ErrRuntimeClosed
	}

	globalInstall.Lock()
	defer globalInstall.Unlock()
	if r.closed.Load() {
		return ErrRuntimeClosed
	}
	current := otel.GetMeterProvider()
	if globalInstall.runtime != nil && globalInstall.runtime != r {
		return ErrGlobalProviderInstalled
	}
	if globalInstall.runtime == r {
		if current != r.provider {
			return ErrGlobalProviderInstalled
		}
		return nil
	}
	if current != initialGlobalProvider {
		return ErrGlobalProviderInstalled
	}
	otel.SetMeterProvider(r.provider)
	globalInstall.runtime = r
	return nil
}

// Shutdown 刷新并关闭此 Runtime 所拥有的资源。此方法支持并发和重复调用，
// 所有调用者都会收到首次关闭操作的结果。
func (r *Runtime) Shutdown(ctx context.Context) error {
	if r == nil {
		return nil
	}
	if ctx == nil {
		return errors.New("metrics shutdown context is nil")
	}
	r.shutdownOnce.Do(func() {
		globalInstall.Lock()
		r.closed.Store(true)
		if globalInstall.runtime == r {
			if otel.GetMeterProvider() == r.provider {
				otel.SetMeterProvider(initialGlobalProvider)
			}
			globalInstall.runtime = nil
		}
		globalInstall.Unlock()
		if r.shutdown != nil {
			r.shutdownErr = r.shutdown(ctx)
		}
	})
	return r.shutdownErr
}

func buildResource(cfg Config) (*resource.Resource, error) {
	attrs := make([]attribute.KeyValue, 0, 3+len(cfg.Resource))
	attrs = append(attrs, attribute.String("service.name", strings.TrimSpace(cfg.ServiceName)))
	if cfg.ServiceVersion != "" {
		attrs = append(attrs, attribute.String("service.version", cfg.ServiceVersion))
	}
	if cfg.Environment != "" {
		attrs = append(attrs, attribute.String("deployment.environment.name", cfg.Environment))
	}
	keys := make([]string, 0, len(cfg.Resource))
	for key := range cfg.Resource {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		attrs = append(attrs, attribute.String(key, cfg.Resource[key]))
	}

	custom := resource.NewSchemaless(attrs...)
	merged, err := resource.Merge(resource.Default(), custom)
	if err != nil {
		return nil, err
	}
	return merged, nil
}

func newOTLPReader(ctx context.Context, cfg Config) (sdkmetric.Reader, error) {
	var exporter sdkmetric.Exporter
	var err error
	if cfg.effectiveProtocol() == ProtocolHTTPProto {
		exporter, err = newHTTPExporter(ctx, cfg)
	} else {
		exporter, err = newGRPCExporter(ctx, cfg)
	}
	if err != nil {
		return nil, err
	}
	readerOptions := make([]sdkmetric.PeriodicReaderOption, 0, 2)
	if cfg.ExportInterval > 0 {
		readerOptions = append(readerOptions, sdkmetric.WithInterval(cfg.ExportInterval))
	}
	if cfg.ExportTimeout > 0 {
		readerOptions = append(readerOptions, sdkmetric.WithTimeout(cfg.ExportTimeout))
	}
	return sdkmetric.NewPeriodicReader(exporter, readerOptions...), nil
}

func newHTTPExporter(ctx context.Context, cfg Config) (sdkmetric.Exporter, error) {
	opts := []otlpmetrichttp.Option{otlpmetrichttp.WithHeaders(cloneStringMap(cfg.Headers))}
	endpoint := strings.TrimSpace(cfg.Endpoint)
	if strings.Contains(endpoint, "://") {
		parsed, err := url.Parse(endpoint)
		if err != nil {
			return nil, fmt.Errorf("parse http endpoint: %w", err)
		}
		if parsed.Path == "" {
			opts = append(opts, otlpmetrichttp.WithEndpoint(parsed.Host))
		} else {
			opts = append(opts, otlpmetrichttp.WithEndpointURL(endpoint))
		}
	} else {
		opts = append(opts, otlpmetrichttp.WithEndpoint(endpoint))
	}
	if cfg.Insecure {
		opts = append(opts, otlpmetrichttp.WithInsecure())
	}
	if cfg.ExportTimeout > 0 {
		opts = append(opts, otlpmetrichttp.WithTimeout(cfg.ExportTimeout))
	}
	return otlpmetrichttp.New(ctx, opts...)
}

func newGRPCExporter(ctx context.Context, cfg Config) (sdkmetric.Exporter, error) {
	opts := []otlpmetricgrpc.Option{otlpmetricgrpc.WithHeaders(cloneStringMap(cfg.Headers))}
	endpoint := strings.TrimSpace(cfg.Endpoint)
	if strings.Contains(endpoint, "://") {
		parsed, err := url.Parse(endpoint)
		if err != nil {
			return nil, fmt.Errorf("parse grpc endpoint: %w", err)
		}
		opts = append(opts, otlpmetricgrpc.WithEndpointURL(endpoint))
		if parsed.Scheme == "http" {
			opts = append(opts, otlpmetricgrpc.WithInsecure())
		}
	} else {
		opts = append(opts, otlpmetricgrpc.WithEndpoint(endpoint))
		if cfg.Insecure {
			opts = append(opts, otlpmetricgrpc.WithInsecure())
		}
	}
	if cfg.ExportTimeout > 0 {
		opts = append(opts, otlpmetricgrpc.WithTimeout(cfg.ExportTimeout))
	}
	return otlpmetricgrpc.New(ctx, opts...)
}
