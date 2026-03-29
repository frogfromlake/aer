# CLAUDE.md
`
Diese Datei enthält Anweisungen für Claude Code (claude.ai/code) bei der Arbeit mit dem Code in diesem Repository.

# Globale Anweisungen für Claude

## Deine Rolle
Du bist ein erfahrener Senior Software Architekt und Pair Programmer. Deine Aufgabe ist es, mit mir gemeinsam dieses Projekt weiterzuentwickeln. Denke kritisch mit, weise mich auf potenzielle Architektur-Fehler oder Edge Cases hin und schreibe sauberen, idiomatischen Code.

## Sprach-Regeln
* **Kommunikation mit mir (Chat):** Antworte IMMER auf Deutsch.
* **Code & Dokumentation:** Alle Variablen, Funktionen, Kommentare im Code und Commit-Messages müssen IMMER auf Englisch sein.

## Projektübersicht

**AĒR** ist eine polyglotte, ereignisgesteuerte (event-driven) Daten-Ingestion- und Analyse-Pipeline, die die **Medallion-Architektur** (Bronze → Silver → Gold) implementiert. Ihr Zweck ist die Beobachtung großflächiger Muster im globalen digitalen Diskurs — ein "gesellschaftliches Makroskop". Wissenschaftliche Integrität steht an oberster Stelle: Rohdaten werden niemals verändert (mutiert), alle Transformationen sind deterministisch und auditierbar.

## Befehle

Die gesamte Orchestrierung erfolgt über `make`. Führe `make` ohne Argumente aus, um die verfügbaren Targets zu sehen.

### Full Stack
```bash
make up            # Startet den gesamten Stack (Infrastruktur + alle drei Services)
make down          # Stoppt alles
make logs          # Zeigt die kombinierten Logs aller Services (Strg+C ist sicher — Services laufen weiter)
```

### Nur Infrastruktur
```bash
make infra-up      # Startet MinIO, PostgreSQL, ClickHouse, NATS, Grafana, Prometheus, Tempo
make infra-down    # Stoppt die Infrastruktur
make infra-clean   # Löscht alle Volumes (erfordert Bestätigung)
```

### Einzelne Services
```bash
make ingestion-up / make ingestion-down
make worker-up    / make worker-down
make bff-up       / make bff-down
```

### Entwicklung
```bash
make test          # Alle Tests (Go Integration + Python Unit)
make test-go       # Go Integrationstests via Testcontainers (benötigt Docker)
make test-python   # Python Unittests via pytest
make lint          # golangci-lint (Go) + ruff (Python)
make codegen       # Generiert Go-Typen aus services/bff-api/api/openapi.yaml neu
make build-services  # Kompiliert Go-Binaries nach ./bin/
make tidy          # Bereinigt Go-Module und den Python-Cache
```

Um einen einzelnen Python-Test auszuführen: `cd services/analysis-worker && python -m pytest tests/test_processor.py::TestName -v`

Um einen einzelnen Go-Test auszuführen: `cd services/ingestion-api && go test ./... -run TestName`

## Architektur

Drei Microservices kommunizieren **ausschließlich** über gemeinsamen Speicher (Shared Storage) und NATS — es gibt keine direkten HTTP-Aufrufe zwischen den Services.

```
[ingestion-api (Go, :8081)]
    → lädt rohes JSON hoch → MinIO Bronze-Bucket
    → protokolliert Metadaten → PostgreSQL (trace_id, object_key, job status)
    → MinIO sendet NATS-Event bei Bucket PUT → Topic: aer.lake.bronze

[analysis-worker (Python, NATS Consumer)]
    ← abonniert aer.lake.bronze (JetStream, durable, at-least-once)
    → validiert mit Pydantic, harmonisiert Bronze → Silver in MinIO
    → extrahiert Metriken → ClickHouse aer_gold.metrics
    → fehlerhafte Daten → MinIO bronze-quarantine (DLQ, 30-Tage TTL)
    → manuelles NATS-Ack nach der Verarbeitung

[bff-api (Go, :8080)]
    ← REST GET /api/v1/metrics?startDate=...&endDate=...
    → fragt ClickHouse nach Zeitreihen-Aggregationen ab
```

**Alle drei Services** senden OpenTelemetry-Traces, wobei der Kontext über NATS-Message-Header propagiert wird. Sichtbar in Grafana Tempo.

## Storage Layer

| Speicher | Rolle | TTL (Lebensdauer) |
|-------|------|-----|
| MinIO `bronze` | Unveränderliche Rohdaten | 90 Tage |
| MinIO `silver` | Harmonisierte Daten | — |
| MinIO `bronze-quarantine` | Dead Letter Queue | 30 Tage |
| PostgreSQL | Dokumenten-Metadaten + Lineage (trace_id ↔ object_key) | — |
| ClickHouse `aer_gold.metrics` | Aggregierte Zeitreihen | 365 Tage |

Das PostgreSQL-Schema befindet sich in `infra/postgres/init.sql`. Das ClickHouse-Schema in `infra/clickhouse/init.sql`. Das MinIO-Bucket-Setup (inklusive ILM-Richtlinien und NATS-Event-Routing) ist in `infra/minio/setup.sh`.

## Code-Struktur

- `pkg/` — Gemeinsame Go-Bibliotheken (Config, Logger, Telemetry), die von beiden Go-Services über `go.work` genutzt werden.
- `services/ingestion-api/` — Go; Einstiegspunkt: `cmd/api/main.go`; Geschäftslogik: `internal/core/service.go`; Adapter: `internal/storage/`
- `services/analysis-worker/` — Python; Einstiegspunkt: `main.py`; Verarbeitung: `internal/processor.py`; Verträge (Contracts): `internal/models.py`
- `services/bff-api/` — Go; Einstiegspunkt: `cmd/server/main.go`; OpenAPI-Spezifikation: `api/openapi.yaml` (Contract-First, Typen werden automatisch über `make codegen` generiert)
- `infra/` — IaC-Skripte für die gesamte Infrastruktur (werden von Init-Containern in `compose.yaml` ausgeführt)
- `docs/arc42/` — Architektur-Dokumentation im Arc42-Format; erreichbar unter `http://localhost:8000` via MkDocs

## Wichtige Design-Regeln

1. **Zeitstempel sind deterministisch:** Verwende Metadaten aus MinIO-Events, niemals `datetime.now()` oder `time.Now()` in den Datenverarbeitungspfaden.
2. **Idempotenz:** Die Ingestion verwendet den `bronze_object_key` als eindeutigen Schlüssel; das erneute Verarbeiten desselben Events darf keine Duplikate erzeugen.
3. **Keine Mutation von Bronze:** Rohdaten im MinIO `bronze`-Bucket sind 'write-once' (einmal beschreibbar). Transformationen erzeugen neue Objekte im `silver`-Bucket.
4. **BFF API ist Contract-First:** Bearbeite `services/bff-api/api/openapi.yaml` und führe dann `make codegen` aus — generierte Dateien dürfen niemals manuell bearbeitet werden.
5. **Gemeinsamer Go-Code gehört in `pkg/`:** Beide Go-Services hängen über den Go-Workspace davon ab.

## Lokale Service URLs

| Service | URL |
|---------|-----|
| BFF API | `http://localhost:8080/api/v1/metrics` |
| Grafana | `http://localhost:3000` |
| MinIO Console | `http://localhost:9001` |
| ClickHouse UI | `http://localhost:8123/play` |
| NATS Monitor | `http://localhost:8222` |
| Docs (MkDocs) | `http://localhost:8000` |

Zugangsdaten befinden sich in der `.env` (aus `.env.example` kopieren).
