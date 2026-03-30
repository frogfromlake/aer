# AĒR Implementation, Refactoring & Scaling Roadmap

Diese Roadmap definiert die Schritte, um die AĒR-Grundarchitektur in ein skalierbares, wartbares und nach modernen Standards (CLEAN Code, DRY, Event-Driven) entwickeltes System zu überführen.

---

# Completed Phases (1–13)

## Phase 1: Basis-Fundament & DRY-Prinzip (Go Workspace & Tooling) - [x] DONE
* [x] **Go Workspace aufsetzen:** `pkg/`-Ordner für geteilte Logik.
* [x] **`go.work` initialisieren:** Verknüpfung der Services.
* [x] **Zentrales Konfigurationsmanagement:** `viper` Config-Loader für `.env`.
* [x] **Standardisiertes Logging:** Custom JSON/Text-Logger (Go `slog`).

## Phase 2: Clean Architecture (`ingestion-api`) - [x] DONE
* [x] **Ordnerstruktur anpassen:** `cmd/api`, `internal/config`, `internal/storage`, `internal/core`.
* [x] **Dependency Injection umsetzen:** Sauberes Wiring in `main.go`.
* [x] **Hardcoded Credentials entfernen:** Nutzung der `.env`.

## Phase 3: Infrastructure as Code (IaC) - [x] DONE
* [x] **App-Logik bereinigen:** Bucket-Erstellung aus Go-Code entfernt.
* [x] **Init-Skripte / IaC:** Docker-Init-Container (`minio-init`) legt Buckets an.

## Phase 4: Event-Driven Communication - [x] DONE
* [x] **Technologie-Entscheidung:** NATS JetStream gewählt.
* [x] **Infrastruktur:** NATS zur `compose.yaml` hinzugefügt.
* [x] **Producer (MinIO):** Automatischer Event-Trigger bei neuen Dateien im "bronze"-Layer.
* [x] **Consumer (Python):** Python-Worker lauscht asynchron und loggt eingehende Events.

## Phase 5: Observability & Produktionsreife - [x] DONE
* [x] **Observability-Infrastruktur:** OTel-Collector, Grafana Tempo (Traces), Prometheus (Metriken) und Grafana (Dashboards) in Docker aufsetzen.
* [x] **Konfiguration:** YAML-Configs für den Collector und das Routing anlegen.
* [x] **Dokumentation:** Architektur-Entscheidungen (ADR) für OTel im arc42 dokumentieren.

## Phase 6: Proof of Concept (End-to-End "Closing the Loop") - [x] DONE
* [x] **Bronze Layer (Go):** Die `ingestion-api` lädt ein echtes JSON-Dokument in den `bronze`-Bucket hoch.
* [x] **NATS Trigger & Silver Layer (Python):** Der Python-Worker empfängt das Event, lädt das JSON herunter, wendet eine einfache Transformation an (z.B. Lowercase) und speichert es im `silver`-Bucket.
* [x] **Gold Layer (ClickHouse):** Einführung von ClickHouse in die Infrastruktur. Der Python-Worker extrahiert eine Dummy-Metrik und speichert sie als Zeitreihe in der Gold-Datenbank.
* [x] **Tracing Instrumentation:** Einbau der OTel-Bibliotheken in Go und Python, sodass dieser exakte Flow als durchgehender Trace in Grafana sichtbar wird.

## Phase 7: Data Governance & Resilienz (The Silver Contract) - [x] DONE
* [x] **Silver Schema Contract:** Einführung von `Pydantic` im Python-Worker zur strikten Validierung und Normalisierung heterogener Bronze-Daten in ein einheitliches AĒR-Format.
* [x] **Dead Letter Queue (DLQ):** Fehlerhaftes JSON (Parsing-Errors) wird abgefangen und in einen Quarantäne-Bucket (`bronze-quarantine`) verschoben, anstatt den Worker crashen zu lassen.
* [x] **Idempotenz:** ClickHouse und Python-Worker so anpassen, dass doppelte NATS-Events (Redeliveries) ignoriert werden und Metriken nicht doppelt gezählt werden.

## Phase 8: The Metadata Index (PostgreSQL) - [x] DONE
* [x] **Datenbankschema:** Erstellung der Tabellen für `sources`, `ingestion_jobs` und `documents` in PostgreSQL.
* [x] **Go Tracking:** Die Ingestion-API speichert Metadaten (Zeitpunkt, Quelle, MinIO-Pfad) in Postgres, bevor das Dokument in den Data Lake geladen wird.
* [x] **Trace-Verknüpfung:** Die OTel Trace-ID wird als Fremdschlüssel in der Datenbank abgelegt, um später Audit-Trails zu ermöglichen.

## Phase 9: The Serving Layer (Backend-for-Frontend) - [x] DONE
* [x] **Contract-First API:** Definition der REST-Schnittstellen (z.B. für Zeitreihen-Abfragen) in einer `openapi.yaml`.
* [x] **BFF Code Generation:** Nutzung von `oapi-codegen`, um aus der OpenAPI-Spezifikation die Go-Boilerplate (Router, Structs) für die `bff-api` zu generieren.
* [x] **ClickHouse Integration:** Implementierung des offiziellen `clickhouse-go` Treibers in der BFF-API, um aggregierte Daten performant auszulesen.

## Phase 10: Testing & Continuous Integration (CI) - [x] DONE
* [x] **Python Unit Testing:** Einführung von `pytest` zur strikten Überprüfung der Datenharmonisierung (Bronze -> Silver) und der deterministischen Metrik-Extraktion.
* [x] **Go Integration Testing:** Nutzung von `testcontainers-go`, um die Interaktionen der Ingestion-API mit MinIO und PostgreSQL in isolierten Test-Containern zu validieren.
* [x] **CI Pipeline (GitHub Actions):** Aufbau von automatisierten Workflows für Linting (`golangci-lint`, `ruff`) und Ausführung der Testsuiten bei jedem Push/Pull-Request.

## Phase 11: Data Lifecycle Management & Graceful Degradation - [x] DONE
* [x] **Resilienz (Go):** Implementierung von Exponential Backoff (via `cenkalti/backoff/v5`) beim Verbindungsaufbau zu PostgreSQL und MinIO.
* [x] **Data Lake Lifecycle:** Erweiterung des `minio-init` Containers um `mc ilm` Policies, um rohe Bronze-Daten nach einer definierten Zeitspanne (z.B. 90 Tage) automatisch zu bereinigen/archivieren.
* [x] **Analytics TTL & Migrations:** Auslagerung der ClickHouse-Tabellenerstellung aus dem Python-Code in dedizierte IaC/Init-Skripte und Einführung von Time-To-Live (TTL) Regeln zur Daten-Aggregation.

## Phase 12: System-Resilienz, Konsistenz & Performance-Optimierung (Technical Debt) - [x] DONE
* [x] **Infrastruktur & Netzwerke:** Einführung eines expliziten Docker-Netzwerks (`aer-network`) in der `compose.yaml` für bessere Isolation und DNS-Auflösung.
* [x] **Robuste Init-Skripte:** Hinzufügen von Docker `healthcheck`s (z.B. für MinIO) und Umstellung der `depends_on`-Logik auf `condition: service_healthy`, um Boot-Race-Conditions zu vermeiden.
* [x] **Verlustfreie Event-Verarbeitung (Python):** Umstellung des `analysis-worker` von Core NATS auf echtes NATS JetStream (`js.subscribe`, dauerhafte Consumer) inklusive manuellem `msg.ack()`, um Datenverlust bei Neustarts auszuschließen.
* [x] **Concurrency Control (Python):** Entkopplung der NATS-Callbacks von CPU-intensiven Aufgaben mittels asynchroner Queues (`asyncio.Queue`) oder Thread-/Process-Pools, um ein Blockieren des Event-Loops zu verhindern.
* [x] **Idempotenz-Optimierung (Python):** Ablösung der netzwerkintensiven MinIO-Abfragen (`stat_object` für Silver/Quarantäne) durch performante PostgreSQL-Lookups zur Vermeidung eines Flaschenhalses bei hohem Durchsatz.
* [x] **Partial Failures auflösen (Go - Ingestion API):** Einführung eines "Pending"-Status in PostgreSQL vor dem MinIO-Upload. Update auf "Uploaded" erst nach Erfolg, um "Dark Data" (Dateien ohne Metadaten-Eintrag) zu verhindern.
* [x] **Partial Failures auflösen (Python - Worker):** Transaktionssichere Auflösung der Sequenz "MinIO Upload (Silver) -> ClickHouse Insert (Gold)". Anpassung der Retry-Logik und Status-Verfolgung, sodass bei einem ClickHouse-Timeout die Metriken nicht für immer verloren gehen.

## Phase 13: Distributed Systems Hardening & Idempotency - [x] DONE
* [x] **Idempotente Metriken (Worker):** Ablösung von `datetime.now()` durch deterministische Zeitstempel (aus den MinIO-Event-Metadaten) beim ClickHouse-Insert, um Duplikate bei NATS-Redeliveries zu verhindern.
* [x] **OOM-Prevention (BFF-API):** Implementierung von Downsampling (z.B. Aggregation auf Minuten-/Stundenbasis) und Limits in den ClickHouse-Queries der Go BFF-API, um Speicherüberläufe bei großen Zeiträumen zu verhindern.
* [x] **Clean Graceful Shutdown (Worker):** Refactoring des Python-Workers von hartem `task.cancel()` auf Sentinel-Werte (`None`) in der Task-Queue, um abgerissene Datenbankverbindungen bei Neustarts zu vermeiden.
* [x] **Macro-Level Error Tracking (Ingestion):** Anpassung der Go `IngestionService`-Logik, um fehlerhafte Einzeldokumente zu tracken und den übergeordneten Job-Status am Ende korrekt auf `failed` oder `completed_with_errors` zu setzen.
* [x] **Boot-Race-Conditions (Infra):** Hinzufügen von nativen Docker `healthcheck`s für PostgreSQL und ClickHouse in der `compose.yaml` inklusive `depends_on: condition: service_healthy` für abhängige Services.

## Phase 17: Ingestion API Redesign (Vom Batch-Job zum echten Service) - [x] DONE
*Transformation der `ingestion-api` von einem einmaligen PoC-Skript in einen langlebigen, HTTP-fähigen Microservice.*

* [x] **HTTP-Server einführen:** `chi`-Router mit `POST /api/v1/ingest`, konfigurierbar via `INGESTION_PORT` (default: 8081).
* [x] **PoC-Testdaten entfernen:** Hardcoded `testCases` durch `IngestDocuments(ctx, sourceID, []Document)` ersetzt.
* [x] **Health Check Endpoints:** `/healthz` (Liveness) und `/readyz` (prüft Postgres + MinIO).
* [x] **Graceful Shutdown mit HTTP:** 5s Timeout analog zur BFF-API.
* [x] **OTel-Instrumentation:** `otelhttp`-Middleware für automatisches Span-Tracking.

## Phase 15: Configuration Hardening & Environment Consistency - [x] DONE
*Eliminierung aller hardcoded Werte und Herstellung einer konsistenten, umgebungsunabhängigen Konfiguration über alle Services hinweg. Voraussetzung für alle weiteren Phasen — ohne saubere Config kann kein Service sinnvoll konfiguriert oder skaliert werden.*

* [x] **OTel-Endpoint externalisieren (Go):** `pkg/telemetry/otel.go` akzeptiert den Collector-Endpoint als Parameter statt `localhost:4317` fest zu verdrahten. Konfiguration via `OTEL_EXPORTER_OTLP_ENDPOINT` aus der `.env`.
* [x] **ClickHouse-Adresse externalisieren (BFF):** `bff-api/cmd/server/main.go` liest die ClickHouse-Adresse (`CLICKHOUSE_HOST`, `CLICKHOUSE_PORT`) aus der Config statt `localhost:9002` zu hardcoden.
* [x] **Python-Worker Config-Refactoring:** NATS-URL (`NATS_URL`), OTel-Endpoint (`OTEL_EXPORTER_OTLP_ENDPOINT`) und `WORKER_COUNT` werden via `python-dotenv` / Umgebungsvariablen konfigurierbar gemacht. `storage.py`-Funktionen behalten ihre `os.getenv()`-Aufrufe mit sinnvollen Defaults — DI-Refactoring folgt in Phase 21.
* [x] **BFF Server-Port externalisieren:** Port `:8080` aus Config lesen statt fest im Code.
* [x] **`.env.example` vervollständigen:** Fehlende Variablen ergänzt: `CLICKHOUSE_USER`, `CLICKHOUSE_PASSWORD`, `CLICKHOUSE_DB`, `CLICKHOUSE_HOST`, `CLICKHOUSE_PORT`, `POSTGRES_HOST`, `POSTGRES_PORT`, `NATS_URL`, `WORKER_COUNT`, `BFF_PORT`, `GF_SECURITY_ADMIN_USER`, `GF_SECURITY_ADMIN_PASSWORD`.
* [x] **Grafana-Credentials entkoppeln:** Eigene `GF_SECURITY_ADMIN_USER` / `GF_SECURITY_ADMIN_PASSWORD` Variablen statt Wiederverwendung der MinIO-Credentials.
* [x] **`replace`-Direktive konsistent machen:** `bff-api/go.mod` erhält dieselbe `replace`-Direktive wie `ingestion-api/go.mod` für lokale `pkg`-Referenz.

## Phase 21: Code Quality & Logger Refactoring - [x] DONE
*Kann parallel zu Phase 17 bearbeitet werden. Behebung von Code-Qualitätsproblemen und Vereinheitlichung der Logging-Strategie, bevor die Codebasis mit Crawlern wächst — danach wird Refactoring teurer.*

* [x] **Logger-Refactoring (`pkg/logger`):** Der `TintHandler` ruft aktuell `fmt.Printf` direkt auf und umgeht damit das slog-System. Refactoring: Den darunterliegenden `slog.Handler` korrekt delegieren oder eine bewährte Bibliothek wie `lmittmann/tint` verwenden, die das slog-Interface korrekt implementiert.
* [x] **Python OTel-Setup isolieren:** Das Tracer/Provider-Setup aus dem globalen Modul-Scope in eine explizite `init_telemetry()`-Funktion verschieben, die in `main()` aufgerufen wird. Dies ermöglicht sauberes Testing ohne globale Seiteneffekte.
* [x] **Python Dependency Injection:** `DataProcessor.__init__` akzeptiert bereits Infrastructure-Clients — dasselbe Prinzip auf `main.py` anwenden, sodass die NATS-Subscription und Worker-Konfiguration testbar und konfigurierbar sind.
* [x] **`psycopg2-binary` dokumentieren:** Expliziter Kommentar in `requirements.txt`, dass `psycopg2-binary` nur für Development/CI geeignet ist. Für Production: `psycopg2` mit libpq-Abhängigkeit im Dockerfile.
* [x] **Makefile-Sprache vereinheitlichen:** Die Shell-Skripte (`clean_infra.sh`, etc.) enthalten deutsche Kommentare und UI-Texte. Umstellung auf Englisch gemäß der Projektsprachen-Constraint (ADR in `02_architecture_constraints.md`).

## Phase 14: Real Data Ingestion (The First Real Crawler) - [x] DONE
*Ablösung des Dummy-JSONs durch echte Daten. Wichtige Architektur-Entscheidung ("Dumb Pipes, Smart Endpoints"): Crawler werden NICHT in die `ingestion-api` integriert, sondern laufen als externe Skripte, die Daten per HTTP POST einliefern. Langfristige Vision: Hunderte spezialisierter Crawler liefern über das HTTP-Interface der Ingestion-API Daten in den Bronze-Layer.*

* [x] **Standalone Go Crawler:** Erstellung eines eigenständigen Go-Programms unter `crawlers/wikipedia-scraper/`, das die öffentliche Wikipedia JSON-API (z.B. Artikel des Tages) abruft und das JSON per POST an `http://localhost:8081/api/v1/ingest` sendet.
* [x] **Worker Adaptation (Python):** Anpassung von `models.py`, `processor.py` und `test_processor.py` im `analysis-worker` an das neue Wikipedia-Format. Logik: Text bereinigen, rudimentäre N-Gramme/Wortzähler extrahieren und als Metrik an ClickHouse senden.

## Phase 16: API Hardening & HTTP Middleware Stack - [x] DONE
*Absicherung und Professionalisierung der HTTP-Schicht der BFF-API für den Produktionseinsatz. Mit echten Daten im System wird die BFF-API von außen erreichbar — sie muss abgesichert sein.*

* [x] **Recovery Middleware:** `chi` Recovery-Middleware einbauen, um Panics in Handlern abzufangen und als `500 Internal Server Error` zurückzugeben statt den Prozess zu crashen.
* [x] **Request-Logging Middleware:** Structured Access-Logging (`slog`) für jede eingehende HTTP-Request (Method, Path, Status, Duration, Trace-ID).
* [x] **CORS Middleware:** Konfigurierbare Cross-Origin-Freigabe für das spätere Frontend (erlaubte Origins via `.env`, `CORS_ALLOWED_ORIGINS`).
* [ ] **Rate Limiting:** Token-Bucket oder Sliding-Window Rate Limiter als Middleware, konfigurierbar via Umgebungsvariablen.
* [x] **Health Check Endpoint:** `GET /api/v1/healthz` (Liveness) und `GET /api/v1/readyz` (Readiness, prüft ClickHouse-Verbindung) als standardisierte Kubernetes-kompatible Endpunkte.
* [x] **Request Timeout Middleware:** Globaler Context-Timeout pro Request (30s), um hängende ClickHouse-Queries zu begrenzen.

## Phase 18: Observability Completion
*Schließen aller Lücken im Monitoring- und Tracing-Stack. Jetzt gibt es echte Daten zum Beobachten — ohne Observability sind Probleme mit echten Crawlern unsichtbar.*

* [x] **BFF-API OTel-Instrumentierung:** Einbau von `otelhttp`-Middleware und Tracer in die BFF-API, damit Traces nicht am Python-Worker enden, sondern bis zum API-Response sichtbar sind.
* [x] **Python Prometheus-Metriken:** Export von Business-Metriken aus dem Worker: `events_processed_total`, `events_quarantined_total`, `event_processing_duration_seconds`, `dlq_size` (Counter/Histogram via `opentelemetry-sdk` Metrics-API oder `prometheus_client`).
* [x] **DLQ-Monitoring:** Periodische Prüfung der Objektanzahl im `bronze-quarantine`-Bucket. Alert bei Überschreitung eines Schwellwerts.
* [x] **Grafana Dashboard Provisioning:** Erstellung eines vorgefertigten JSON-Dashboards (`infra/observability/grafana-dashboards/`) mit Panels für: Pipeline-Durchsatz, DLQ-Rate, ClickHouse Query-Latenz, NATS Consumer Lag. Automatisches Provisioning via `grafana.ini` / Provisioning-Volume.
* [x] **Alerting Rules:** Definition von Prometheus Alerting Rules (`alert.rules.yml`): Worker-Down, DLQ-Overflow, ClickHouse-Latenz > Schwellwert, NATS-Consumer-Lag > Schwellwert.

---

# Open Phases (14–23) — Priorisierte Implementierungsreihenfolge

*Die folgenden Phasen wurden aus einem professionellen Code-Review abgeleitet und in eine Reihenfolge gebracht, die den Abhängigkeitsgraph des Systems respektiert. Die zentrale Achse lautet: Config aufräumen → Ingestion zum echten Service machen → erster Crawler → beobachten & absichern → skalieren.*

```
Phase 15 (Config) ✔
  → Phase 17 (Ingestion Redesign) ✔
    → Phase 14 (First Crawler) ← nächster Schritt, erster echter Datenfluss
      → Phase 16 (BFF Hardening)  ┐
      → Phase 18 (Observability)  ├ parallel möglich
      → Phase 19 (Testing)       ┘
        → Phase 20 (Infra Hardening)
          → Phase 23 (Security)
            → Phase 22 (Docs)

Phase 21 (Code Quality) ✔
```

---

### Tier 1: Fundament legen (bevor echte Daten fließen)
Currently Empty
---

### Tier 2: Erster echter Datenfluss
Currently Empty
---

### Tier 3: Härtung für Dauerbetrieb mit echten Daten
## Phase 16: API Hardening & HTTP Middleware Stack
*Absicherung und Professionalisierung der HTTP-Schicht der BFF-API für den Produktionseinsatz. Mit echten Daten im System wird die BFF-API von außen erreichbar — sie muss abgesichert sein.*

* [ ] **Rate Limiting:** (Distributed Cache): Wir nutzen eine schnelle, zentrale In-Memory-Datenbank wie Redis. Jede API-Instanz fragt bei jedem Request kurz bei Redis an.

## Phase 19: Testing Expansion & Contract Safety
*Erhöhung der Testabdeckung auf alle kritischen Pfade. Jetzt sinnvoll, weil der reale Datenfluss existiert und getestet werden kann.*

* [ ] **BFF Handler Tests:** Unit-Tests für die Handler-Logik in `handler.go` (Zeitraum-Fallback, Fehlerbehandlung) mit gemocktem Storage-Interface.
* [ ] **OpenAPI Contract Tests:** Automatisierter Abgleich, dass die generierte `generated.go` mit der `openapi.yaml` synchron ist. Integration in CI (z.B. `oapi-codegen` erneut ausführen und `git diff` prüfen).
* [ ] **Python Edge-Case Tests:** Erweiterung der `test_processor.py` um: leere Strings nach `.lower()`, verschachtelte/unerwartete JSON-Strukturen, simulierte Netzwerkfehler (MinIO `ConnectionError`, ClickHouse Timeout), `_move_to_quarantine` in Isolation.
* [ ] **Python Storage Tests:** Integration-Tests für `storage.py` mit Testcontainers (Postgres, MinIO, ClickHouse) analog zur Go-Strategie, um die `@retry`-Logik und Verbindungsaufbau zu validieren.
* [ ] **End-to-End Smoke Test:** Ein einzelner automatisierter Test, der den gesamten Flow testet: JSON → Ingestion → MinIO → NATS → Worker → ClickHouse → BFF API Response. Kann als separater CI-Job mit `docker compose up` laufen.

---

### Tier 4: Produktionsreife & Skalierung

## Phase 20: Infrastructure Hardening & Container Security
*Absicherung der Docker-Infrastruktur für Langzeitbetrieb und Vorbereitung auf Skalierung mit hunderten Crawlern.*

* [ ] **Image-Versionen pinnen:** Alle `latest`-Tags in `compose.yaml` durch spezifische Versionen ersetzen (z.B. `minio/minio:RELEASE.2026-03-01`, `nats:2.10.x`, `postgres:16.x-alpine`, `clickhouse/clickhouse-server:24.x`). Dokumentation der Upgrade-Policy.
* [ ] **Resource Limits:** `deploy.resources.limits` (Memory, CPU) für jeden Container in der `compose.yaml` setzen. Besonders kritisch: ClickHouse (OLAP kann unbegrenzt Speicher konsumieren).
* [ ] **Restart Policies:** `restart: unless-stopped` für alle persistenten Services (Datenbanken, NATS, Grafana).
* [ ] **Netzwerk-Segmentierung:** Aufteilung des flachen `aer-network` in mindestens zwei Subnetze: `aer-frontend` (BFF, Grafana, Docs) und `aer-backend` (Datenbanken, NATS, Worker). Nur die BFF-API verbindet beide Netze.
* [ ] **Service Dockerfiles:** Erstellung von Multi-Stage Dockerfiles für `ingestion-api`, `bff-api` und `analysis-worker`, damit die Services selbst containerisiert und unabhängig vom Host deployed werden können.
* [ ] **CI Docker-Layer-Caching:** Einführung von `docker/build-push-action` oder manuellem Layer-Caching in der GitHub Actions Pipeline, um Testcontainers-Pulls zu beschleunigen.

## Phase 23: Security Foundations
*Einführung grundlegender Sicherheitsmechanismen vor dem Deployment mit echten Daten. Setzt Container-Hardening (Phase 20) voraus.*

* [ ] **API Authentication:** Einführung eines API-Key oder JWT-basierten Auth-Mechanismus auf der BFF-API. Mindestens ein statischer API-Key als Middleware-Gate für die erste Iteration.
* [ ] **TLS für externe Endpoints:** HTTPS-Terminierung für BFF-API und Grafana (z.B. via Traefik oder Caddy als Reverse Proxy in der Compose-Stack).
* [ ] **Secrets Management:** Evaluierung und Einführung eines Secrets-Management-Ansatzes (Docker Secrets, HashiCorp Vault, oder SOPS-verschlüsselte `.env`-Dateien) statt Klartext-Credentials in `.env`.
* [ ] **Container Security Scanning:** Integration eines Image-Scanners (Trivy, Grype) in die CI-Pipeline, um bekannte CVEs in den Base-Images und Dependencies zu erkennen.
* [ ] **Dependency Auditing:** `go vuln check` für Go-Module und `pip-audit` / `safety` für Python-Dependencies als CI-Step.

## Phase 22: Arc42 Dokumentation vervollständigen
*Zuletzt, weil die Doku den finalen Zustand der Architektur reflektieren soll. Vorher ändert sich die Architektur noch zu stark.*

* [ ] **Kapitel 3 — System Scope and Context:** Kontextdiagramm erstellen (System Boundary, externe Akteure: Datenquellen, Analysten, Dashboard-User). Business Context und Technical Context klar trennen.
* [ ] **Kapitel 11 — Risks and Technical Debts:** Dokumentation der bekannten Risiken: fehlende Authentifizierung, keine TLS-Verschlüsselung, Silver-Layer ohne Retention-Policy, Abhängigkeit von MinIO-Event-Ordering.
* [ ] **Kapitel 12 — Glossary:** Zentrale Begriffe definieren: Bronze/Silver/Gold Layer, DLQ, Silver Contract, Progressive Disclosure, Probe, Macroscope, Harmonization, Idempotency.
* [ ] **`go.work`-Setup dokumentieren:** In der README.md einen Abschnitt ergänzen, der erklärt, wie Entwickler das Go Workspace lokal initialisieren (`go work init ./pkg ./services/ingestion-api ./services/bff-api`), da die Datei per `.gitignore` nicht versioniert wird.
* [ ] **ADR für Netzwerk-Segmentierung:** Neuer ADR-008 für die Entscheidung zur Docker-Netzwerk-Aufteilung (Phase 20).
