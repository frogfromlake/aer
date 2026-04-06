# Welcome to AĒR

**AĒR** (ancient Greek ἀήρ: the lower atmosphere, the surrounding climate) is a modular system for the real-time analysis and long-term observation of societal discourses. 

Functioning as a digital **"macroscope"**, AĒR is designed to capture large-scale patterns in the hopes, fears, conflicts, and aspirations of connected civilizations—prioritizing deterministic transparency and the principle of Ockham's Razor over opaque AI black boxes.

---

## Documentation Structure

This documentation portal bridges the gap between software engineering and the humanities. It is divided into three main pillars, accessible via the **top navigation tabs**:

### Architecture (arc42)
The complete technical blueprint of the AĒR pipeline. Following the established [arc42 framework](arc42/01_introduction_and_goals.md), this section details the system's modular microservice architecture, the strict Medallion data lake constraints (Bronze, Silver, Gold layers), and core engineering decisions (ADRs).

### Scientific Methodology
The epistemological lens of the system. This section documents the anthropological, sociological, and linguistic frameworks used to interpret the data. It defines how we select representative observation points and measure cultural resonance without falling into the trap of Eurocentric bias.
* **Read the latest working paper:** [WP-001: Functional Probe Taxonomy](methodology/WP-001-functional-probe-taxonomy.md)

### Operations Playbook
The practical guide for system operators. It contains [runbooks](operations_playbook.md), infrastructure commands, and observability guidelines (Grafana, Tempo, Prometheus) for deploying and maintaining the stack.

---

!!! info "Docs-as-Code"
    This documentation follows the **Docs-as-Code** principle. It is maintained directly alongside the source code in the repository to ensure that architectural design, scientific theory, and technical implementation remain perfectly synchronized.