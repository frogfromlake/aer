// Client-side hooks. Lazy-loads OpenTelemetry after the first paint so the
// observability bundle never blocks initial rendering (ADR-020 §Layer 4,
// ROADMAP Phase 97 "OpenTelemetry Web SDK" — "do not block initial render").

import { env } from '$env/dynamic/public';

if (typeof window !== 'undefined') {
  const schedule =
    'requestIdleCallback' in window
      ? (cb: () => void) =>
          (
            window as unknown as { requestIdleCallback: (cb: () => void) => void }
          ).requestIdleCallback(cb)
      : (cb: () => void) => window.setTimeout(cb, 0);

  schedule(() => {
    void import('$lib/observability/otel').then(({ initOtel }) => {
      initOtel({
        endpoint: env.PUBLIC_OTLP_ENDPOINT ?? '',
        environment: env.PUBLIC_DEPLOYMENT_ENVIRONMENT ?? 'development'
      });
    });
  });
}
