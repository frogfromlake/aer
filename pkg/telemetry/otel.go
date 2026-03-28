package telemetry

import (
	"context"
	"fmt"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.24.0"
)

// InitProvider configures the global OpenTelemetry Tracer using gRPC.
func InitProvider(serviceName string) (func(context.Context) error, error) {
	ctx := context.Background()

	// Connect to the OpenTelemetry Collector container on port 4317
	exporter, err := otlptracegrpc.New(ctx,
		otlptracegrpc.WithEndpoint("localhost:4317"),
		otlptracegrpc.WithInsecure(),
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
		sdktrace.WithSampler(sdktrace.AlwaysSample()),
		sdktrace.WithResource(res),
		sdktrace.WithSpanProcessor(sdktrace.NewBatchSpanProcessor(exporter)),
	)

	// Set global provider and context propagator (crucial for injecting Trace-ID into MinIO)
	otel.SetTracerProvider(tracerProvider)
	otel.SetTextMapPropagator(propagation.TraceContext{})

	return tracerProvider.Shutdown, nil
}
