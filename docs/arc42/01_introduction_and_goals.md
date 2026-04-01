# 1. Introduction and Goals

## 1.1 Task Description
**AĒR** (ancient Greek ἀήρ: the lower atmosphere, the surrounding climate) is a modular system for the real-time analysis and long-term observation of societal discourses. It acts as a "macroscope" to understand international and intercultural behavioral patterns, mindsets, habitus, and discourses over time.

The goal of AĒR is not individualized surveillance, but rather the intelligent aggregation and synthesis of meaningful metadata. Utilizing modern technical capabilities, it aims to visualize hidden societal currents — whether in continuous cultural shifts or in reactions to specific economic, social, or political events.

AĒR functions as an **unaltered mirror of humanity**. Following the principle of **Ockham's Razor**, the system deliberately avoids opaque AI black boxes or overly complex models that might interpret and potentially distort raw data. The methodology remains as simple, genuine, and transparent as possible. The system strictly separates the unstructured acquisition of raw data from global sources, the deterministic normalization of this data, and the subsequent transparent sociological and linguistic analysis.

## 1.2 Philosophical Foundation (The AĒR DNA)
The project and its analytical evaluations are based on three central cultural and structural-scientific pillars, which also form its name:

* **A - Aleph (after Jorge Luis Borges):** The single point in space that contains all other points. AĒR aggregates global, fragmented data streams into a single, coherent dashboard to make the "big picture" of human interaction and culture observable simultaneously.
* **E - Episteme (after Michel Foucault):** The unconscious, underlying set of rules of an epoch that defines what can be thought and said. AĒR analyzes continuous discourse shifts to measure how the boundaries of the expressible (framing, narratives) form and change within shifting cultures.
* **R - Rhizome (after Deleuze/Guattari):** A decentralized, proliferating root network. AĒR utilizes this model to understand how information, cultural patterns, mindsets, and discourses spread non-linearly through global networks and social strata.

## 1.3 Quality Goals
The architecture of AĒR is driven by the following primary quality goals (detailed scenarios in Chapter 10):

| Priority | Quality Goal | Description |
| :--- | :--- | :--- |
| **1** | **Scientific Integrity & Transparency** | The analysis must not distort raw data through cascading AI models. Algorithms must be deterministic, simple, and traceable (Ockham's Razor). Raw data (metadata) must remain intact as direct evidence for the UI (Progressive Disclosure) to allow for valid sociological conclusions. |
| **2** | **Extensibility (Modularity)** | New data sources must be addable as standalone external crawlers without affecting the core system. New sociological metrics must be implementable as isolated processing steps in the Python analysis worker. |
| **3** | **Scalability & Performance** | The system must be able to process massive amounts of text and metadata in parallel. The BFF API must prevent OOM through server-side downsampling and hard query limits. |
| **4** | **Maintainability** | The separation of data collection (Ingestion) and analysis must be absolute — no direct HTTP calls between services. Inter-service communication is mediated exclusively through NATS JetStream and shared storage (MinIO). API contracts between the BFF and consumers are enforced via OpenAPI code generation with CI sync checks. |
| **5** | **Security** | The internet-facing BFF API must require authentication. Databases and internal services must be unreachable from the public network via Docker network segmentation. Supply chain security must be enforced via automated dependency auditing and container image scanning in CI. |

## 1.4 Stakeholders

| Role | Expectation |
| :--- | :--- |
| **System Architect / Lead Dev** | Stable microservice architecture with clear separation of concerns. Performant polyglot tech stack (Go for I/O, Python for analysis). Strict operational constraints: hard-pinned image tags, SSoT enforcement via `compose.yaml`, automated CI/CD with contract checks, security scanning, and Git hook–enforced code quality gates. No unnecessary complexity (Ockham's Razor). |
| **Sociologists / Analysts** | Valid, undistorted metrics produced by transparent, deterministic algorithms (no black boxes). Deep filtering capabilities by culture, demographics, and timeframes. Ability to drill down from aggregated trends (Gold) to original raw source data (Bronze) via Progressive Disclosure. |
| **End User (Dashboard)** | Intuitive, interactive visualization that makes a genuine, traceable "mirror of society" tangible. *(Note: The frontend dashboard is not yet implemented — see Chapter 8, roadmap Phase 4.)* |
| **System Operator** | Full observability via Grafana dashboards, Prometheus alerting, and distributed tracing (Tempo). Automated data lifecycle management (ILM/TTL). Granular `make` targets for infrastructure and service control. Predictable storage costs through automated retention policies. |