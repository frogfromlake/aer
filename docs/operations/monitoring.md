# Monitoring & Alerting — "where do I look when it burns"

A single-page index of every observability surface, how to reach it on a
production box, and what each alert means + the first action to take. Pairs with
the [Operations Playbook](operations_playbook.md) (component debugging) and the
[Backup & Restore Runbook](backup_restore.md).

> **Production network posture (SEC-001).** On a prod box the Grafana, MinIO, and
> Prometheus consoles are **NOT publicly routed** — only the app (`/` + `/api`)
> and (optionally) the docs are. You reach the internal consoles over an **SSH
> tunnel**, never a public URL. The tunnel commands are below.

## 1. The surfaces at a glance

| Surface | What it shows | Reach it (production) |
|---|---|---|
| **Grafana** | Dashboards + **alert state/history** (the primary pane) | `ssh -L 3000:localhost:3000 <box>` → http://localhost:3000 |
| **Prometheus** | Raw metrics + `/alerts` (rule state, validated mirror) | `ssh -L 9090:localhost:9090 <box>` → http://localhost:9090 |
| **MinIO console** | Bronze/Silver/Quarantine buckets, ILM | `ssh -L 9001:localhost:9001 <box>` → http://localhost:9001 |
| **Tempo** | Distributed traces (via Grafana Explore) | inside Grafana → Explore → Tempo |
| **Container logs** | Per-service stdout (rotated 10MB×3) | `docker compose logs -f <service>` on the box |
| **Healthchecks** | Liveness/readiness | `docker compose ps` (health column) |
| **Internal docs** | Arc42, ADRs, runbooks, design, security model (the full corpus) | `ssh -L 8001:localhost:8001 <box>` → http://localhost:8001 |
| **Public docs** | Public methodology portal (Working Papers, probe descriptions) | `:8000` on the box; firewall-blocked in the gated POC (not Traefik-routed) |

> **Internal docs (`:8001`)** are loopback-only (SEC-018) and **not internet-reachable** — the same SSH-tunnel pattern as the consoles. In production the
> MkDocs container is hardened (SEC-064): no livereload, and only `docs/` + the
> two `mkdocs.*.yml` configs are mounted read-only (no whole-repo mount, so it
> cannot read `.env`). This is where the Operations Playbook, this Monitoring
> index, and the Backup/Restore runbook live during an incident.

> The internal consoles are only listening on the box's loopback after a
> `--profile debug` up, OR reachable by tunnelling to the container IP. For a
> standard prod up, the services run on the internal `aer-backend` network;
> `ssh -L 3000:<grafana-container-ip>:3000 <box>` also works. See the playbook.

## 2. Alerts — what fires, what it means, what to do

Alerts are defined once in Prometheus (`infra/observability/prometheus/alert.rules.yml`,
promtool-validated) and **delivered** by the Grafana-managed mirror
(`infra/observability/grafana/provisioning/alerting/`) via **email → Brevo**
(SEC-037). You see them in Grafana → **Alerting → Alert rules**; you receive them
in your inbox (`ALERT_EMAIL_TO`).

| Alert | Severity | Means | First action |
|---|---|---|---|
| **WorkerDown** | critical | analysis-worker scrape target unreachable >1m | `docker compose logs analysis-worker`; restart `make worker-restart` |
| **DLQOverflow** | warning | >50 objects in bronze-quarantine | Inspect quarantine bucket; a malformed source or adapter bug (playbook §DLQ) |
| **HighEventProcessingLatency** | warning | p95 processing >5s | Check worker CPU/mem + ClickHouse health; possible backpressure |
| **TargetDown** | warning | any scrape target down >2m | `docker compose ps`; find the unhealthy container |
| **DiskSpaceLow / Critical** | warning / critical | host root <15% / <5% free | **#1 killer.** Prune nothing blindly (model cache!); check log/volume growth, see playbook §disk |
| **HostMemoryLow** | warning | <10% RAM available | OOM risk for postgres/bff/minio; check worker (cooccurrence loads 1y window) |
| **TLSCertExpiringSoon / Critical** | warning / critical | cert <7d / <2d to expiry | ACME renewal failing — check Traefik logs + `acme.json`; site goes offline at expiry |
| **BackupStale** | critical | no successful backup >26h | Run `make backup` manually; check Storage Box reachability + scheduler |
| **BackupNeverRan** | warning | backup heartbeat metric absent | backup.sh has never succeeded or textfile path wrong; see backup_restore.md §3 |
| **BackupFailed** | critical | last backup exited non-zero | Read `backup.sh` output; check restic repo + Storage Box SSH |
| **CrawlStale** | warning | worker up but 0 events processed in 6h | Crawl stopped — corpus silently freezing. Check crawler schedule + discovery |

## 3. Wiring alert email (production)

1. In `.env` set `GF_SMTP_ENABLED=true`, reuse the Brevo SMTP key
   (`GF_SMTP_USER` / `GF_SMTP_PASSWORD`), set `GF_SMTP_FROM_ADDRESS` to a
   `@aer-project.eu` sender, and set **`ALERT_EMAIL_TO`** to your inbox.
2. Set `GF_SERVER_ROOT_URL` so the "View alert" links in emails resolve.
3. Bring the stack up. Grafana provisions the contact point + policy + rules at
   boot. **Verify provisioning succeeded:**
   ```bash
   docker compose logs grafana | grep -i "alerting\|provision"
   ```
   A bad provisioning file logs a clear error here. Then in the UI:
   Grafana → Alerting → **Contact points** → `aer-email` → **Test** (sends a
   probe email through Brevo).
4. Confirm the rules loaded: Grafana → Alerting → **Alert rules** → expect the
   three groups `aer_pipeline`, `aer_infra`, `aer_backup` (13 rules) under the
   `AER Alerts` folder.

## 4. Quick triage flow

1. **Email says X is firing** → open Grafana (tunnel) → Alerting → find the rule →
   it links the panel/query.
2. **Confirm the raw signal** → Prometheus (tunnel) → `/alerts` or run the PromQL.
3. **Read the component** → `docker compose logs -f <service>` + the
   [Operations Playbook](operations_playbook.md) section for that component.
4. **Data-layer incident** (disk/backup/corruption) → the
   [Backup & Restore Runbook](backup_restore.md).
