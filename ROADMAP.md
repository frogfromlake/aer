# AĒR Refactoring & Scaling Roadmap

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