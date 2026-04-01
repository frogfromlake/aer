# 2. Architecture Constraints

## 2.1 Organizational Constraints

| Constraint | Description |
| :--- | :--- |
| **Project Language** | The official project language is **English**. This applies strictly to all source code (variable names, comments), documentation (arc42, inline docs), API contracts (OpenAPI/Swagger), and commit messages. |
| **Monorepo** | The project is structured as a Monorepo to keep services, documentation, and API contracts tightly synchronized and easily accessible for local development. |
| **Docs-as-Code** | Architectural documentation is written in Markdown using the arc42 template and rendered via MkDocs. It must reside in the same repository as the codebase. |

## 2.2 Technical Constraints

| Constraint | Description |
| :--- | :--- |
| **Containerization** | All services and development environments must be fully containerized using Docker. No local installations of databases or runtimes (other than standard build tools) are permitted on the host OS. |
| **Polyglot Stack** | Go (Golang) is restricted to data ingestion, networking, and the API layer (BFF). Python is strictly reserved for data processing and deterministic analysis. |
| **Runtime Versions** | Go **1.26.1** or higher and Python **3.12** or higher are required. These versions are enforced in `go.work`, all `go.mod` files, the CI pipeline (`ci.yml`), and the Dockerfiles. |
| **No Direct Inter-Service HTTP** | Microservices must never call each other directly via synchronous HTTP. All inter-service communication is mediated exclusively through shared storage (MinIO) and the NATS JetStream message broker. |

## 2.3 Operational Constraints

These constraints govern how the infrastructure is configured, tested, and maintained. They are enforced through code, CI checks, and Git hooks — not by convention alone.

| Constraint | Description | Enforcement |
| :--- | :--- | :--- |
| **Hard-Pinned Image Tags** | All Docker images in `compose.yaml` must use immutable, patch-level version tags. The use of `latest`, release-candidate, or alpha tags is strictly prohibited. Image versions are upgraded manually after changelog review and local validation. See ADR-009. | Code review. CI Testcontainers will fail if a tag resolves to an unexpected image. |
| **Compose as SSoT for Tags** | `compose.yaml` is the Single Source of Truth for all container image versions. Testcontainers in both Go (`pkg/testutils/compose.go`) and Python (`get_compose_image()`) dynamically parse image tags from this file at test time. No image tag may be hardcoded in test files. See ADR-009. | SSoT parsers in `pkg/testutils/compose.go` and `test_storage.py`. Tests break if compose is not parseable. |
| **HTTP-Based Healthchecks** | Docker healthchecks and Testcontainers readiness probes must use HTTP endpoints (e.g., `GET /minio/health/live`, `GET /ping`) or native readiness commands (e.g., `pg_isready`). Log-parsing-based health detection is strictly prohibited — it is brittle and non-deterministic. | Code review. Enforced in `compose.yaml` healthcheck definitions and Testcontainers fixture setup. |
| **Pre-Commit: Linting** | The `scripts/hooks/pre-commit` Git hook runs `make lint` (`golangci-lint` for Go, `ruff` for Python) before every commit. Commits are blocked if linting fails. | Git hook in `scripts/hooks/pre-commit`. |
| **Pre-Push: Linting + Testing** | The `scripts/hooks/pre-push` Git hook runs `make lint` followed by `make test` (full Go integration tests + Python unit tests) before every push. Pushes are blocked if either step fails. | Git hook in `scripts/hooks/pre-push`. |
| **Contract-First Codegen** | The BFF API's OpenAPI specification (`services/bff-api/api/openapi.yaml`) is the SSoT for the API shape. Go server stubs and types are generated via `oapi-codegen` (`make codegen`). Generated files (`generated.go`) must never be manually edited. The CI pipeline enforces sync by re-running codegen and failing on `git diff`. | CI job: `make codegen && git diff --exit-code`. |
| **IaC-Only Provisioning** | Application services must never create infrastructure (buckets, tables, streams) at startup. All provisioning is handled by dedicated init containers (`nats-init`, `minio-init`) or database init scripts (`init.sql`). See Chapter 8.4. | Architectural convention. Init containers in `compose.yaml`. |