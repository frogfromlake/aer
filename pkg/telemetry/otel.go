package telemetry

import (
	"context"
	"fmt"
	"strings"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.24.0"
)

// InitProvider configures the global OpenTelemetry Tracer using HTTP.
// The endpoint parameter accepts a full URL (e.g. "http://localhost:4318")
// or a bare host:port (e.g. "localhost:4318").
// sampleRate controls what fraction of traces are sampled (0.0–1.0). Use 1.0
// for development (AlwaysSample equivalent) and 0.1 in production. ParentBased
// wrapping ensures child spans inherit the parent's sampling decision, preventing
// orphaned trace fragments.
func InitProvider(serviceName, endpoint string, sampleRate float64) (func(context.Context) error, error) {
	ctx := context.Background()

	// otlptracehttp.WithEndpoint expects host:port without scheme.
	host := strings.TrimPrefix(strings.TrimPrefix(endpoint, "https://"), "http://")

	exporter, err := otlptracehttp.New(ctx,
		otlptracehttp.WithEndpoint(host),
		otlptracehttp.WithInsecure(),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create otel exporter: %w", err)
	}

	res, err := resource.New(ctx,
		resource.WithAttributes(semconv.ServiceName(serviceName)),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create resource: %w", err)
	}

	tracerProvider := sdktrace.NewTracerProvider(
		sdktrace.WithSampler(sdktrace.ParentBased(sdktrace.TraceIDRatioBased(sampleRate))),
		sdktrace.WithResource(res),
		sdktrace.WithSpanProcessor(sdktrace.NewBatchSpanProcessor(exporter)),
	)

	// Set global provider and context propagator (crucial for injecting Trace-ID into MinIO)
	otel.SetTracerProvider(tracerProvider)
	otel.SetTextMapPropagator(propagation.TraceContext{})

	return tracerProvider.Shutdown, nil
}
