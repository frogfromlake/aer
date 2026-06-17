package telemetry

import (
	"context"
	"testing"

	"go.opentelemetry.io/otel"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

// The OTLP/HTTP exporter connects lazily (no dial on creation), so every test
// here runs fully offline against an unreachable endpoint.

func TestInitProvider_SetsGlobalsAndReturnsShutdown(t *testing.T) {
	shutdown, err := InitProvider("test-service", "localhost:4318", 1.0)
	if err != nil {
		t.Fatalf("InitProvider returned error: %v", err)
	}
	if shutdown == nil {
		t.Fatal("InitProvider returned a nil shutdown func")
	}
	t.Cleanup(func() { _ = shutdown(context.Background()) })

	if _, ok := otel.GetTracerProvider().(*sdktrace.TracerProvider); !ok {
		t.Errorf("global tracer provider = %T, want *sdktrace.TracerProvider", otel.GetTracerProvider())
	}
	if otel.GetTextMapPropagator() == nil {
		t.Error("global text-map propagator was not set")
	}
}

func TestInitProvider_TrimsEndpointScheme(t *testing.T) {
	// All three forms must construct without error — the http/https scheme is
	// stripped before being handed to otlptracehttp.WithEndpoint (host:port only).
	for _, endpoint := range []string{"http://localhost:4318", "https://localhost:4318", "localhost:4318"} {
		t.Run(endpoint, func(t *testing.T) {
			shutdown, err := InitProvider("test-service", endpoint, 1.0)
			if err != nil {
				t.Fatalf("InitProvider(%q) error: %v", endpoint, err)
			}
			_ = shutdown(context.Background())
		})
	}
}

func TestInitProvider_SamplerHonoursRate(t *testing.T) {
	tests := []struct {
		name        string
		rate        float64
		wantSampled bool
	}{
		{"rate 1.0 always samples a root span", 1.0, true},
		{"rate 0.0 never samples a root span", 0.0, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			shutdown, err := InitProvider("test-service", "localhost:4318", tt.rate)
			if err != nil {
				t.Fatalf("InitProvider error: %v", err)
			}
			t.Cleanup(func() { _ = shutdown(context.Background()) })

			_, span := otel.GetTracerProvider().Tracer("test").Start(context.Background(), "op")
			defer span.End()
			if got := span.SpanContext().IsSampled(); got != tt.wantSampled {
				t.Errorf("rate %v: span IsSampled() = %v, want %v", tt.rate, got, tt.wantSampled)
			}
		})
	}
}
