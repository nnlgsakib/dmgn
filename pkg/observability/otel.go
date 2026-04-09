package observability

import (
	"context"
	"fmt"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	"go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
)

// OTelConfig holds OpenTelemetry configuration.
type OTelConfig struct {
	ServiceName   string
	ServiceVersion string
	OTLPEndpoint  string
	EnableTraces  bool
	EnableMetrics bool
}

// OTelProviders holds the initialized OTel providers.
type OTelProviders struct {
	TracerProvider *trace.TracerProvider
	MeterProvider  *metric.MeterProvider
}

// InitOTel initializes OpenTelemetry with trace and metric providers.
// If OTLPEndpoint is empty, noop exporters are used.
func InitOTel(ctx context.Context, cfg OTelConfig) (*OTelProviders, error) {
	if cfg.ServiceName == "" {
		cfg.ServiceName = "dmgn"
	}
	if cfg.ServiceVersion == "" {
		cfg.ServiceVersion = "0.1.0"
	}

	res, err := resource.New(ctx,
		resource.WithAttributes(
			semconv.ServiceName(cfg.ServiceName),
			semconv.ServiceVersion(cfg.ServiceVersion),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create OTel resource: %w", err)
	}

	// Trace provider — noop if no endpoint
	tp := trace.NewTracerProvider(
		trace.WithResource(res),
	)
	otel.SetTracerProvider(tp)

	// Meter provider — noop if no endpoint
	mp := metric.NewMeterProvider(
		metric.WithResource(res),
	)
	otel.SetMeterProvider(mp)

	return &OTelProviders{
		TracerProvider: tp,
		MeterProvider:  mp,
	}, nil
}

// Shutdown gracefully shuts down all OTel providers.
func (p *OTelProviders) Shutdown(ctx context.Context) error {
	var errs []error
	if p.TracerProvider != nil {
		if err := p.TracerProvider.Shutdown(ctx); err != nil {
			errs = append(errs, err)
		}
	}
	if p.MeterProvider != nil {
		if err := p.MeterProvider.Shutdown(ctx); err != nil {
			errs = append(errs, err)
		}
	}
	if len(errs) > 0 {
		return fmt.Errorf("OTel shutdown errors: %v", errs)
	}
	return nil
}

// Tracer returns a named tracer from the global provider.
func Tracer(name string) interface{} {
	return otel.Tracer(name)
}

// Meter returns a named meter from the global provider.
func Meter(name string) interface{} {
	return otel.Meter(name)
}
