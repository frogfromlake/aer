// Frontend OpenTelemetry wiring — Phase 97 scaffolding, Phase 86 parity.
//
// This module is lazy-loaded by src/hooks.client.ts after first paint so the
// observability bundle never blocks initial rendering (ADR-020 §Layer 4).
//
// Emission: OTLP/HTTP → otel-collector:4318 (same collector used by Go/Python
// services). Browser CORS from a non-loopback origin requires a Traefik route
// to the collector; that route is added with Phase 97's compose integration.
// Set PUBLIC_OTLP_ENDPOINT to the empty string to disable emission entirely.

import { context, trace } from '@opentelemetry/api';
import { OTLPTraceExporter } from '@opentelemetry/exporter-trace-otlp-http';
import { registerInstrumentations } from '@opentelemetry/instrumentation';
import { FetchInstrumentation } from '@opentelemetry/instrumentation-fetch';
import { resourceFromAttributes } from '@opentelemetry/resources';
import { BatchSpanProcessor } from '@opentelemetry/sdk-trace-base';
import { WebTracerProvider } from '@opentelemetry/sdk-trace-web';
import { ATTR_SERVICE_NAME, ATTR_SERVICE_VERSION } from '@opentelemetry/semantic-conventions';

import { version as SERVICE_VERSION } from '../../../package.json';

export interface OtelConfig {
  /** OTLP/HTTP traces endpoint (e.g. https://aer.example.com/otel/v1/traces). Empty string disables. */
  endpoint: string;
  /** Deployment environment label (development, staging, production). */
  environment: string;
}

let initialized = false;

export function initOtel(config: OtelConfig): void {
  if (initialized) return;
  if (!config.endpoint) return;
  initialized = true;

  const resource = resourceFromAttributes({
    [ATTR_SERVICE_NAME]: 'aer-dashboard',
    [ATTR_SERVICE_VERSION]: SERVICE_VERSION,
    'deployment.environment': config.environment
  });

  const provider = new WebTracerProvider({
    resource,
    spanProcessors: [new BatchSpanProcessor(new OTLPTraceExporter({ url: config.endpoint }))]
  });

  provider.register();

  registerInstrumentations({
    instrumentations: [
      new FetchInstrumentation({
        propagateTraceHeaderCorsUrls: [/.*/],
        clearTimingResources: true
      })
    ]
  });

  // Mark a root span for the session so it links into BFF/worker traces.
  const tracer = trace.getTracer('aer-dashboard');
  const sessionSpan = tracer.startSpan('session');
  context.with(trace.setSpan(context.active(), sessionSpan), () => {
    window.addEventListener('beforeunload', () => sessionSpan.end(), { once: true });
  });
}
