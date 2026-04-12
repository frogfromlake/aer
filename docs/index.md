# Welcome to AĒR

**AĒR** (ancient Greek ἀήρ: the lower atmosphere, the surrounding climate) is a modular system for the real-time analysis and long-term observation of societal discourses. 

Functioning as a digital **"macroscope"**, AĒR is designed to capture large-scale patterns in the hopes, fears, conflicts, and aspirations of connected civilizations—prioritizing deterministic transparency and the principle of Ockham's Razor over opaque AI black boxes.

---

## Documentation Structure

This documentation portal bridges the gap between software engineering and the humanities. It is divided into the following pillars, accessible via the **top navigation tabs**:

### Architecture (arc42)
The complete technical blueprint of the AĒR pipeline. Following the established [arc42 framework](arc42/01_introduction_and_goals.md), this section details the system's modular microservice architecture, the strict Medallion data lake constraints (Bronze, Silver, Gold layers), and core engineering decisions (ADRs).

### Scientific Methodology

The epistemological lens of the system. A series of six interdisciplinary working papers documents the anthropological, sociological, and linguistic frameworks used to interpret the data. The papers are available in **English** and **German** to support outreach to both international and German-speaking research institutions.

| Paper | Topic | EN | DE |
| :--- | :--- | :---: | :---: |
| **WP-001** | Functional Probe Taxonomy | [EN](methodology/en/WP-001-en-toward_a_culturally_agnostic_probe_catalog-a_functional_taxonomy_for_global_discourse_observation.md) | [DE](methodology/de/WP-001-de-auf_dem_weg_zu_einem_kulturell_agnostischen_sondenkatalog-eine_funktionale_taxonomie_fuer_die_globale_diskursbeobachtung.md) |
| **WP-002** | Metric Validity & Sentiment Calibration | [EN](methodology/en/WP-002-en-metric_validity_and_sentiment_calibration.md) | [DE](methodology/de/WP-002-de-metrik_validitaet_und_sentiment_kalibrierung.md) |
| **WP-003** | Platform Bias & Non-Human Actors | [EN](methodology/en/WP-003-en-platform_bias_algorithmic_amplification_and_the_detection_of_non-human_actors.md) | [DE](methodology/de/WP-003-de-plattform_bias_algorithmische_verstaerkung_und_die_erkennung_nicht-menschlicher_akteure.md) |
| **WP-004** | Cross-Cultural Comparability | [EN](methodology/en/WP-004-en-cross-cultural_comparability_of_discourse_metrics.md) | [DE](methodology/de/WP-004-de-interkulturelle_vergleichbarkeit_von_diskursmetriken.md) |
| **WP-005** | Temporal Granularity | [EN](methodology/en/WP-005-en-temporal_granularity_of_discourse_shifts.md) | [DE](methodology/de/WP-005-de-temporale_granularitaet_von_diskursverschiebungen.md) |
| **WP-006** | Observer Effect & Reflexivity | [EN](methodology/en/WP-006-en-observer_effect_reflexivity_and_the_ethics_of_discourse_measurement.md) | [DE](methodology/de/WP-006-de-beobachtereffekt_reflexivitaet_und_die_ethik_der_diskursmessung.md) |

### Operations Playbook
The practical guide for system operators. It contains [runbooks](operations_playbook.md), infrastructure commands, and observability guidelines (Grafana, Tempo, Prometheus) for deploying and maintaining the stack — the *what to type* reference.

### Scientific Operations Guide
The bridge document between developer operations and scientific methodology. It maps every point at which scientific judgment enters the AĒR pipeline (`source_classifications`, `metric_validity`, `metric_equivalence`, `metric_baselines`, `BiasContext`, the Cultural Calendar, and the Probe Dossier) to the responsible role, the Working Paper rationale, the Operations Playbook commands, and the resulting persisted artefact. Six end-to-end workflows are documented, each with a concrete walkthrough using Probe 0 (German Institutional RSS) and real values. See the [Scientific Operations Guide](scientific_operations_guide.md).

---

!!! info "Docs-as-Code"
    This documentation follows the **Docs-as-Code** principle. It is maintained directly alongside the source code in the repository to ensure that architectural design, scientific theory, and technical implementation remain perfectly synchronized.