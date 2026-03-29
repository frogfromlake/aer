# AĒR — The Societal Discourse Macroscope

AĒR is a highly resilient, polyglot data ingestion and analysis pipeline. Designed as a "macroscope," it serves as an externalized, orbital sensor to observe large-scale patterns in the hopes, fears, conflicts, and aspirations of the connected civilization via the global digital discourse.

Built with **Go**, **Python**, and **ClickHouse**, AĒR implements a robust Medallion Architecture (Bronze, Silver, Gold) with self-healing mechanisms and automated data lifecycle management.

---

## 🏗 Architecture Overview

AĒR is built on an event-driven microservice architecture:

1. **Ingestion API (Go):** Receives raw data, stores it in the **MinIO Bronze Bucket**, and indexes metadata in **PostgreSQL**.
2. **Analysis Worker (Python):** Reacts to events via the **NATS** message broker, harmonizes data, moves it to the **Silver Layer**, and extracts final metrics into **ClickHouse (Gold Layer)**.
3. **BFF API (Go):** A high-speed, OpenAPI-compliant Backend-for-Frontend that serves time-series analytics from ClickHouse.
4. **Observability Stack:** Comprehensive tracing, logging, and metrics using OpenTelemetry, Prometheus, Tempo, and Grafana.

---

## 📋 Prerequisites

To run and develop AĒR locally, you need the following tools installed:

- **OS:** Linux, macOS, or Windows 11 with WSL2 (Ubuntu 22.04+ recommended)
- **Docker:** Docker Engine / Docker Desktop (with Compose plugin)
- **Go:** `1.26.1` or higher
- **Python:** `3.12` or higher (with the standard `venv` module)
- **Make:** GNU Make

---

## 🚀 Quick Start & Installation

### 1. Clone & Configure
Clone the repository and set up your local environment variables:
```bash
git clone [https://github.com/frogfromlake/aer.git](https://github.com/frogfromlake/aer.git)
cd aer
cp .env.example .env
````

*(You can adjust the passwords in the `.env` file if desired, but the defaults work out-of-the-box for local development).*

### 2\. Boot the Infrastructure

AĒR completely decouples stateful infrastructure from the application services. Start the databases and observability tools first:

```bash
make infra-up
```

*This starts MinIO, PostgreSQL, ClickHouse, NATS, and the Grafana/Prometheus/Tempo stack.*

### 3\. Start the Application Services

AĒR uses a sophisticated background-process manager (via local bash scripts) to run the Go and Python services locally without blocking your terminal or creating zombie processes.

```bash
make services-up
```

*This will automatically setup Python virtual environments, build the Go binaries, and run the Ingestion API, Analysis Worker, and BFF API in the background.*

### 4\. Monitor the Stack

You can view the live, combined logs of all background services at any time:

```bash
make logs
```

*(Press `Ctrl+C` to exit the log viewer; the services will continue running in the background).*

-----

## 🛑 Stopping & Cleaning Up

AĒR provides granular control over shutting down and resetting the environment:

**Stop Services gracefully:**

```bash
make services-down
```

**Stop Infrastructure:**

```bash
make infra-down
```

**⚠️ Hard Resets (Data Wipes):**
If you need a completely clean state (e.g., during testing), you can wipe the database volumes. **These commands will prompt for confirmation:**

```bash
make infra-clean             # Wipes ALL databases (Postgres, MinIO, ClickHouse)
make infra-clean-postgres    # Wipes ONLY PostgreSQL
make infra-clean-minio       # Wipes ONLY the MinIO Data Lake
make infra-clean-clickhouse  # Wipes ONLY the ClickHouse Analytics DB
```

-----

## 🛠 Developer Workflow

AĒR enforces strict quality standards via Linters and automated testing (using Testcontainers).

| Command | Description |
| :--- | :--- |
| `make test` | Runs the full test suite (Go integration tests & Python unit tests). |
| `make lint` | Runs `golangci-lint` (Go) and `ruff` (Python) to check code quality. |
| `make codegen` | Regenerates Go types and server stubs from the OpenAPI spec (`api/openapi.yaml`). |
| `make build-services` | Compiles the Go binaries into the `./bin/` directory. |
| `make tidy` | Cleans up Go modules and removes Python `__pycache__` directories. |

**Individual Service Control:**
If you are only working on one specific service, you can start/stop it individually:

  - `make ingestion-up` / `make ingestion-down`
  - `make worker-up` / `make worker-down`
  - `make bff-up` / `make bff-down`

-----

## 🌐 UIs & Access Points (Localhost)

Once the stack is fully operational, you can access the following interfaces:

  - **BFF API Endpoint:** [http://localhost:8080/api/v1/metrics](https://www.google.com/search?q=http://localhost:8080/api/v1/metrics)
  - **Grafana Dashboards:** [http://localhost:3000](https://www.google.com/search?q=http://localhost:3000) *(User: admin, Pass: check .env)*
  - **MinIO Console:** [http://localhost:9001](https://www.google.com/search?q=http://localhost:9001) *(User/Pass: check .env)*
  - **NATS Monitoring:** [http://localhost:8222](https://www.google.com/search?q=http://localhost:8222)
  - **Architecture Docs:** [http://localhost:8000](https://www.google.com/search?q=http://localhost:8000) *(Requires `make infra-up` to start the MkDocs container)*

-----

## 📖 Documentation

This project uses the **arc42** framework for architecture documentation. You can read the detailed design decisions, context bounds, and the foundational **AĒR Manifesto** directly by navigating to the `/docs/arc42` directory, or by viewing them through the local documentation server (`http://localhost:8000`).

-----

## 📄 License

This project is licensed under the MIT License - see the [LICENSE](https://www.google.com/search?q=LICENSE) file for details.
