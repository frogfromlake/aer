# AĒR Refactoring & Scaling Roadmap

Diese Roadmap definiert die nächsten Schritte, um die AĒR-Grundarchitektur von einem funktionalen Prototypen in ein skalierbares, wartbares und nach modernen Standards (CLEAN Code, DRY, Event-Driven) entwickeltes System zu überführen.

## Phase 1: Basis-Fundament & DRY-Prinzip (Go Workspace & Tooling)
*Bevor wir spezifische Services anfassen, bauen wir das Fundament, das alle Microservices nutzen werden.*

* [ ] **Go Workspace aufsetzen:** Erstellung eines `pkg/`-Ordners auf Root-Ebene für geteilte Logik (Shared Libraries).
* [ ] **`go.work` initialisieren:** Verknüpfung der Services (`ingestion-api`, `bff-api`) mit dem lokalen `pkg/`-Modul.
* [ ] **Zentrales Konfigurationsmanagement:** Implementierung eines Config-Loaders (z. B. mit `viper` oder `godotenv`) im `pkg/`-Ordner zum sicheren Laden von `.env`-Variablen.
* [ ] **Standardisiertes Logging:** Einführung eines zentralen, strukturierten JSON-Loggers (z. B. Go `slog` oder `zerolog`) im `pkg/`-Ordner, der von allen Services genutzt wird.

## Phase 2: Clean Architecture (Am Beispiel der `ingestion-api`)
*Refactoring des bestehenden Monolithen in der `main.go` in eine testbare, saubere Struktur.*

* [ ] **Ordnerstruktur anpassen:** Erstellen der Verzeichnisse `cmd/api`, `internal/config`, `internal/storage` (für Postgres/MinIO) und `internal/core`.
* [ ] **Dependency Injection umsetzen:** Die `main.go` übernimmt nur noch das "Wiring" (Zusammenstecken der Komponenten) und startet den Service.
* [ ] **Hardcoded Credentials entfernen:** Datenbank- und MinIO-Zugangsdaten durch Umgebungsvariablen ersetzen (nutzt den Config-Loader aus Phase 1).

## Phase 3: Infrastructure as Code (IaC) & Trennung der Verantwortlichkeiten
*Infrastruktur-Provisionierung darf nicht Teil des Applikationscodes sein.*

* [ ] **App-Logik bereinigen:** Entfernen der automatischen MinIO-Bucket-Erstellung ("bronze", "silver") aus dem Go-Code.
* [ ] **Init-Skripte / IaC einführen:** Erstellen von Docker-Init-Skripten (z. B. ein Shell-Skript, das beim Start des MinIO-Containers die Buckets anlegt) oder Einführung von Terraform/OpenTofu für das lokale Setup.

## Phase 4: Event-Driven Communication (Der Weg zur Asynchronität)
*Ablösung des Polling-Konzepts durch echte, ereignisgesteuerte Datenflüsse zwischen Go (Ingestion) und Python (Analysis).*

* [ ] **Technologie-Entscheidung:** Festlegung auf MinIO Bucket Notifications (Webhook/RabbitMQ) oder einen dedizierten Message Broker (NATS/Kafka).
* [ ] **Infrastruktur anpassen:** Hinzufügen des Brokers zur `compose.yaml`.
* [ ] **Producer (Go) implementieren:** Die `ingestion-api` triggert (oder MinIO triggert) ein Event, sobald eine neue Datei im "bronze"-Layer liegt.
* [ ] **Consumer (Python) implementieren:** Der Python-Worker lauscht auf diese Events und startet den Harmonisierungsprozess (Bronze -> Silver), sobald ein Event eingeht.

## Phase 5: Observability & Produktionsreife (Day-2 Operations)
*Sichtbarkeit im System herstellen und das Deployment vorbereiten.*

* [ ] **Distributed Tracing integrieren:** Einbau von OpenTelemetry. Eine Trace-ID wird beim Ingest generiert und über den Message Broker an den Python-Worker weitergereicht.
* [ ] **Monitoring-Stack aufsetzen:** Ergänzung der `compose.yaml` um Prometheus und Jaeger/Grafana für Metriken und Traces.
* [ ] **CI/CD & Orchestrierung:** Erste Vorbereitungen für Kubernetes (K8s) Helm-Charts oder CI/CD-Pipelines (z. B. GitHub Actions) für automatisiertes Testen und Bauen der Docker-Images.
