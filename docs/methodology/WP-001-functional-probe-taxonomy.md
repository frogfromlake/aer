### Toward a Culturally Agnostic Probe Catalog: A Functional Taxonomy

**Objective:** To establish a rigorous, culturally agnostic framework for selecting observation points ("Probes") within the global digital discourse. The framework must actively resist epistemological colonialism—the tendency to impose Western or Eurocentric institutional categories (e.g., "State," "Media," "Civil Society") onto diverse global networks. 

**Methodological Approach: Functional Equivalence** Rather than mapping the global information space through rigid institutional definitions, AĒR categorizes Probes based on their *discursive function* within their specific cultural rhizome. This anthropological approach acknowledges that a functional role (e.g., setting moral norms) might be fulfilled by a Supreme Court in one society, but by a religious authority, a digital diaspora network, or a decentralized forum in another.

To achieve international comparability without sacrificing local context, the selection of any Probe must be guided by the following functional taxonomy:

1. **Epistemic Authority (Norm & Truth Setting):** Actors or channels that define the boundaries of the expressible, establishing societal consensus on truth, morality, or purity.
2. **Resource & Power Legitimation:** Channels utilized by dominant structures (state, military, oligarchic, or corporate) to justify their power and operationalize their narratives.
3. **Cohesion & Identity Formation:** Sources that generate a collective "we-feeling" through cultural narratives, myth-making, and structural demarcation from the "other" (e.g., pop culture, preachers, nationalist influencers).
4. **Subversion & Friction (Counter-Discourse):** Decentralized, activist, or hyper-viral spaces that challenge hegemonic narratives and operate as accelerators for affective or radicalized discourse.

**Architectural Implication: The Dual Tagging System** To operationalize this taxonomy within the AĒR pipeline, the `SilverMeta` layer must support a dual-ontological tagging schema:
* **The Etic Layer (Global Comparability):** An abstract, functional classification (e.g., `discourse_function: epistemic_authority`) allowing ClickHouse to aggregate cross-cultural metrics on equivalent societal forces.
* **The Emic Layer (Local Reality):** Untranslated, context-specific metadata (e.g., `local_context: zaibatsu` or `local_context: ulama`) preserving the original anthropological reality for Progressive Disclosure in the UI.

The definition and integration of a new source into the AĒR crawler ecosystem is therefore not merely a technical configuration, but a deliberate anthropological act requiring qualitative validation.
