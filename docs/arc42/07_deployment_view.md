# 7. Deployment View (Verteilungssicht)

## 7.1 Local Development Environment (DevEx)
To ensure a consistent and isolated development environment across all machines, **AĒR** relies strictly on containerization for local development. No services (databases, parsers, documentation) are installed directly on the host machine.

The local stack is orchestrated via modern `docker compose` and abstracted through a `Makefile` to simplify daily developer operations.

### 7.1.1 Documentation Stack
The architecture and system documentation is written in Markdown (Docs-as-Code) following the arc42 framework. 

* **Renderer:** MkDocs (Material Theme)
* **Container Image:** `squidfunk/mkdocs-material:latest`
* **Orchestration:** Started via `make docs` (maps host port 8000 to container port 8000).
* **Volume Mount:** The root directory is mounted into the container at `/docs` to enable real-time hot-reloading upon saving `.md` files in the editor.

*(Note: As the project grows, backend services like the Go Ingestion-Service, Python Analysis-Service, and the ClickHouse database will be added to this local compose stack.)*