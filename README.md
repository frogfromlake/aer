# AĒR - Societal Discourse Macroscope

A modular system for real-time analysis and long-term observation of societal discourses. Built with Go, Python, and ClickHouse.

## Prerequisites

- **Windows 11 with WSL2 (Ubuntu 22.04+)**
- **Docker Desktop** (with WSL2 integration enabled)
- **Go 1.26.1+** (installed in WSL)
- **Python 3.12+** (with `venv` package)

## Getting Started

### 1. Clone and Initialize
```bash
git clone https://github.com/frogfromlake/A-R.git
cd AĒR
````

### 2\. Start the Infrastructure & Documentation

The project uses a `Makefile` to simplify Docker operations.

```bash
make up
```

This command starts:

  - **Documentation**: [http://localhost:8000](https://www.google.com/search?q=http://localhost:8000)
  - **MinIO (Data Lake)**: [http://localhost:9001](https://www.google.com/search?q=http://localhost:9001) (User: `aer_admin`, Pass: `aer_password_123`)
  - **Postgres**: `localhost:5432` (DB: `aer_metadata`)

### 3\. Service Development

#### Go Services

To run a service locally (e.g., the Ingestion API):

```bash
cd services/ingestion-api
go run main.go
```

#### Python Worker

```bash
cd services/analysis-worker
source venv/bin/activate
pip install -r requirements.txt
python main.py
```

## Architecture

This project follows the **arc42** framework. Detailed documentation can be found in the `/docs` folder or via the local documentation server started with `make docs`.
