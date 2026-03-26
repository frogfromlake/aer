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