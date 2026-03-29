# AĒR Implementation, Refactoring & Scaling Roadmap

Diese Roadmap definiert die Schritte, um die AĒR-Grundarchitektur in ein skalierbares, wartbares und nach modernen Standards (CLEAN Code, DRY, Event-Driven) entwickeltes System zu überführen.

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

## Phase 5: Observability & Produktionsreife (In Progress)
*Sichtbarkeit im System herstellen, um den asynchronen Datenfluss transparent zu machen.*

* [x] **Observability-Infrastruktur:** OTel-Collector, Grafana Tempo (Traces), Prometheus (Metriken) und Grafana (Dashboards) in Docker aufsetzen.
* [x] **Konfiguration:** YAML-Configs für den Collector und das Routing anlegen.
* [x] **Dokumentation:** Architektur-Entscheidungen (ADR) für OTel im arc42 dokumentieren.

## Phase 6: Proof of Concept (End-to-End "Closing the Loop") - [x] DONE
*Beweis, dass die Architektur funktioniert: Ein kompletter Datenfluss durch alle Schichten.*

* [x] **Bronze Layer (Go):** Die `ingestion-api` lädt ein echtes JSON-Dokument in den `bronze`-Bucket hoch.
* [x] **NATS Trigger & Silver Layer (Python):** Der Python-Worker empfängt das Event, lädt das JSON herunter, wendet eine einfache Transformation an (z.B. Lowercase) und speichert es im `silver`-Bucket.
* [x] **Gold Layer (ClickHouse):** Einführung von ClickHouse in die Infrastruktur. Der Python-Worker extrahiert eine Dummy-Metrik und speichert sie als Zeitreihe in der Gold-Datenbank.
* [x] **Tracing Instrumentation:** Einbau der OTel-Bibliotheken in Go und Python, sodass dieser exakte Flow als durchgehender Trace in Grafana sichtbar wird.

## Phase 7: Data Governance & Resilienz (The Silver Contract) - [x] DONE
*Sicherstellen, dass das System deterministisch bleibt und bei Fehlern oder Duplikaten nicht zerbricht.*

* [x] **Silver Schema Contract:** Einführung von `Pydantic` im Python-Worker zur strikten Validierung und Normalisierung heterogener Bronze-Daten in ein einheitliches AĒR-Format.
* [x] **Dead Letter Queue (DLQ):** Fehlerhaftes JSON (Parsing-Errors) wird abgefangen und in einen Quarantäne-Bucket (`bronze-quarantine`) verschoben, anstatt den Worker crashen zu lassen.
* [x] **Idempotenz:** ClickHouse und Python-Worker so anpassen, dass doppelte NATS-Events (Redeliveries) ignoriert werden und Metriken nicht doppelt gezählt werden.

## Phase 8: The Metadata Index (PostgreSQL) - [x] DONE
*Aufbau des relationalen Gedächtnisses, um den Weg der Daten von Gold zurück zu Bronze garantieren zu können (Progressive Disclosure).*

* [x] **Datenbankschema:** Erstellung der Tabellen für `sources`, `ingestion_jobs` und `documents` in PostgreSQL.
* [x] **Go Tracking:** Die Ingestion-API speichert Metadaten (Zeitpunkt, Quelle, MinIO-Pfad) in Postgres, bevor das Dokument in den Data Lake geladen wird.
* [x] **Trace-Verknüpfung:** Die OTel Trace-ID wird als Fremdschlüssel in der Datenbank abgelegt, um später Audit-Trails zu ermöglichen.

## Phase 9: The Serving Layer (Backend-for-Frontend) - [x] DONE
*Bereitstellung der aggregierten Gold-Daten über eine performante und vertragsbasierte Schnittstelle für das Frontend.*

* [x] **Contract-First API:** Definition der REST-Schnittstellen (z.B. für Zeitreihen-Abfragen) in einer `openapi.yaml`.
* [x] **BFF Code Generation:** Nutzung von `oapi-codegen`, um aus der OpenAPI-Spezifikation die Go-Boilerplate (Router, Structs) für die `bff-api` zu generieren.
* [x] **ClickHouse Integration:** Implementierung des offiziellen `clickhouse-go` Treibers in der BFF-API, um aggregierte Daten performant auszulesen.

## Phase 10: Testing & Continuous Integration (CI) - [x] DONE
*Sicherstellung der wissenschaftlichen Determinismus-Vorgaben und der Code-Qualität durch Automatisierung.*

* [x] **Python Unit Testing:** Einführung von `pytest` zur strikten Überprüfung der Datenharmonisierung (Bronze -> Silver) und der deterministischen Metrik-Extraktion.
* [x] **Go Integration Testing:** Nutzung von `testcontainers-go`, um die Interaktionen der Ingestion-API mit MinIO und PostgreSQL in isolierten Test-Containern zu validieren.
* [x] **CI Pipeline (GitHub Actions):** Aufbau von automatisierten Workflows für Linting (`golangci-lint`, `ruff`) und Ausführung der Testsuiten bei jedem Push/Pull-Request.

## Phase 11: Data Lifecycle Management & Graceful Degradation - [x] DONE
*Ressourcenschonung für den Langzeitbetrieb und Absicherung gegen kurzzeitige Ausfälle der Infrastruktur.*

* [x] **Resilienz (Go):** Implementierung von Exponential Backoff (via `cenkalti/backoff/v5`) beim Verbindungsaufbau zu PostgreSQL und MinIO.
* [x] **Data Lake Lifecycle:** Erweiterung des `minio-init` Containers um `mc ilm` Policies, um rohe Bronze-Daten nach einer definierten Zeitspanne (z.B. 90 Tage) automatisch zu bereinigen/archivieren.
* [x] **Analytics TTL & Migrations:** Auslagerung der ClickHouse-Tabellenerstellung aus dem Python-Code in dedizierte IaC/Init-Skripte und Einführung von Time-To-Live (TTL) Regeln zur Daten-Aggregation.

## Phase 12: System-Resilienz, Konsistenz & Performance-Optimierung (Technical Debt)
*Behebung kritischer Designfehler in verteilten Transaktionen und Härtung der Infrastruktur vor der Skalierung mit echten Datenquellen.*

* [x] **Infrastruktur & Netzwerke:** Einführung eines expliziten Docker-Netzwerks (`aer-network`) in der `compose.yaml` für bessere Isolation und DNS-Auflösung.
* [x] **Robuste Init-Skripte:** Hinzufügen von Docker `healthcheck`s (z.B. für MinIO) und Umstellung der `depends_on`-Logik auf `condition: service_healthy`, um Boot-Race-Conditions zu vermeiden.
* [x] **Verlustfreie Event-Verarbeitung (Python):** Umstellung des `analysis-worker` von Core NATS auf echtes NATS JetStream (`js.subscribe`, dauerhafte Consumer) inklusive manuellem `msg.ack()`, um Datenverlust bei Neustarts auszuschließen.
* [x] **Concurrency Control (Python):** Entkopplung der NATS-Callbacks von CPU-intensiven Aufgaben mittels asynchroner Queues (`asyncio.Queue`) oder Thread-/Process-Pools, um ein Blockieren des Event-Loops zu verhindern.
* [ ] **Idempotenz-Optimierung (Python):** Ablösung der netzwerkintensiven MinIO-Abfragen (`stat_object` für Silver/Quarantäne) durch performante PostgreSQL-Lookups zur Vermeidung eines Flaschenhalses bei hohem Durchsatz.
* [ ] **Partial Failures auflösen (Go - Ingestion API):** Einführung eines "Pending"-Status in PostgreSQL vor dem MinIO-Upload. Update auf "Uploaded" erst nach Erfolg, um "Dark Data" (Dateien ohne Metadaten-Eintrag) zu verhindern.
* [ ] **Partial Failures auflösen (Python - Worker):** Transaktionssichere Auflösung der Sequenz "MinIO Upload (Silver) -> ClickHouse Insert (Gold)". Anpassung der Retry-Logik und Status-Verfolgung, sodass bei einem ClickHouse-Timeout die Metriken nicht für immer verloren gehen.

## Phase 13: Real Data Ingestion (The First Real Crawler)
*Ablösung des Dummy-JSONs durch echte, unstrukturierte Daten aus dem Internet.*

* [ ] **Source Definition:** Auswahl einer einfachen, echten Datenquelle (z.B. ein RSS-Feed von Nachrichtenseiten oder Wikipedia).
* [ ] **Go Crawler Implementation:** Einbau eines asynchronen Scrapers in die `ingestion-api`, der echte Texte sammelt.
* [ ] **Bronze Upload:** Speicherung der rohen HTML/XML/JSON-Antworten in MinIO.
* [ ] **Python NLP Basics:** Der Analysis-Worker extrahiert echten Text aus dem HTML/XML, bereinigt ihn und generiert erste echte N-Gram Metriken für ClickHouse.