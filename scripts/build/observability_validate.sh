#!/usr/bin/env bash
#
# Phase 154 — observability config validation ratchet.
#
# Validates the observability stack's configuration as code so a broken
# Prometheus scrape/rule file, OTel collector config, or Grafana dashboard
# fails CI instead of surfacing only at deploy time. Uses the SAME image tags
# pinned in compose.yaml (hard-rule SSoT) so the validators match runtime.
#
# Pure validation: it never starts the stack and mutates nothing. Run via
# `make observability-validate` (locally and in CI).
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
OBS="$ROOT/infra/observability"

# Extract the pinned image tags from compose.yaml (the container-tag SSoT).
prom_img="$(awk '/^  prometheus:/{f=1} f && /image:/{print $2; exit}' "$ROOT/compose.yaml")"
otel_img="$(awk '/^  otel-collector:/{f=1} f && /image:/{print $2; exit}' "$ROOT/compose.yaml")"

if [ -z "$prom_img" ] || [ -z "$otel_img" ]; then
  echo "ERROR: could not read prometheus/otel-collector image tags from compose.yaml" >&2
  exit 1
fi

echo "==> Prometheus config + rules (promtool, $prom_img)"
# Mount at the exact runtime paths so the config's absolute rule_files ref
# (/etc/prometheus/alert.rules.yml) resolves the same way it does in the
# running container.
docker run --rm \
  -v "$OBS/prometheus.yml:/etc/prometheus/prometheus.yml:ro" \
  -v "$OBS/prometheus/alert.rules.yml:/etc/prometheus/alert.rules.yml:ro" \
  --entrypoint promtool "$prom_img" check config /etc/prometheus/prometheus.yml

echo "==> OTel collector config (otelcol validate, $otel_img)"
docker run --rm \
  -v "$OBS/otel-collector.yaml:/cfg.yaml:ro" \
  "$otel_img" validate --config=/cfg.yaml

echo "==> Grafana dashboards (JSON) + provisioning/config (YAML) + ClickHouse XML"
python3 - "$ROOT" <<'PY'
import json
import sys
import xml.dom.minidom as minidom
from pathlib import Path

import yaml

root = Path(sys.argv[1])
obs = root / "infra" / "observability"

# Every provisioned dashboard must be valid JSON with a stable uid + panels.
dash_dir = obs / "grafana" / "provisioning" / "dashboards"
dashboards = sorted(dash_dir.glob("*.json"))
if not dashboards:
    print("ERROR: no Grafana dashboard JSON found", file=sys.stderr)
    sys.exit(1)
for d in dashboards:
    doc = json.loads(d.read_text())
    if not doc.get("uid"):
        print(f"ERROR: {d.name} has no uid", file=sys.stderr)
        sys.exit(1)
    if not isinstance(doc.get("panels"), list) or not doc["panels"]:
        print(f"ERROR: {d.name} has no panels", file=sys.stderr)
        sys.exit(1)
    print(f"    ok  {d.relative_to(root)} ({len(doc['panels'])} panels)")

# Observability YAML configs must parse.
yaml_files = [
    obs / "prometheus.yml",
    obs / "prometheus" / "alert.rules.yml",
    obs / "otel-collector.yaml",
    obs / "tempo.yaml",
    obs / "grafana-datasources.yaml",
    dash_dir / "dashboards.yaml",
]
for y in yaml_files:
    yaml.safe_load(y.read_text())
    print(f"    ok  {y.relative_to(root)}")

# ClickHouse prometheus config must be well-formed XML.
ch_xml = root / "infra" / "clickhouse" / "config.d" / "prometheus.xml"
minidom.parseString(ch_xml.read_text())
print(f"    ok  {ch_xml.relative_to(root)}")
PY

echo "==> Observability config validation passed."
