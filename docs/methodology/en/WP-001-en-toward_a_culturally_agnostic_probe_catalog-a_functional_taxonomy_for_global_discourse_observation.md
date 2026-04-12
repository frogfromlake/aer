# WP-001: Toward a Culturally Agnostic Probe Catalog — A Functional Taxonomy for Global Discourse Observation

> **Series:** AĒR Scientific Methodology Working Papers
> **Status:** Draft v2 — substantially revised, open for interdisciplinary review
> **Date:** 2026-04-07 (v2), 2026-04-03 (v1)
> **Depends on:** Manifesto §III–§IV, Arc42 Chapter 13 (Scientific Foundations)
> **Architectural context:** Source Adapter Protocol (ADR-015), SilverMeta (§5.1.2), Crawler Ecosystem (§5.2)
> **Downstream:** WP-002 (Metric Validity), WP-003 (Platform Bias), WP-004 (Cross-Cultural Comparability), WP-005 (Temporal Granularity), WP-006 (Observer Effect)
> **License:** [CC BY-NC 4.0](https://creativecommons.org/licenses/by-nc/4.0/) — © 2026 Fabian Quist

---

## 1. Objective

This working paper addresses the first and most fundamental open research question from §13.6: **Which digital spaces constitute representative "probes" for observing societal discourse, and how do we select and weight them without imposing culturally biased categories?**

AĒR's Manifesto (§IV) establishes the **Probe Principle**: rather than attempting total data aggregation, the system selects strategic observation points within the global information space that serve as proxies for different societal realities. The selection, weighting, and interpretation of these probes constitute the core interdisciplinary challenge. But the Manifesto does not specify *how* probes should be selected. This working paper provides the methodological framework.

The central claim is that probe selection must be guided by **discursive function**, not by institutional form. A Supreme Court and a council of religious elders may look nothing alike institutionally, but both can serve the function of epistemic authority — defining what is true, legitimate, and expressible in a given society. A state news agency and an oligarch-funded media empire may have different ownership structures, but both can serve the function of power legitimation. By classifying probes according to their discursive function rather than their institutional category, AĒR achieves cross-cultural comparability without imposing Western institutional ontologies.

This paper grounds the functional taxonomy in comparative social science and anthropological theory, defines each discourse function with cross-cultural depth, specifies the Etic/Emic Dual Tagging System as the architectural mechanism for operationalization, establishes the probe selection process as a formalized scientific method, and identifies concrete research questions for interdisciplinary collaboration.

---

## 2. The Problem: Why Institutional Categories Fail

### 2.1 The Western Default

The most natural way to categorize information sources is by institutional type: "government," "media," "civil society," "academia," "social media." This categorization feels objective — it simply names what things are. But it is not objective. It is a product of Western political modernity, specifically of the liberal-democratic separation of powers and the Enlightenment distinction between state, market, and public sphere.

When applied globally, this categorization produces systematic distortion:

**The "media" fallacy.** In Western democracies, "media" typically refers to commercially or publicly funded news organizations operating under editorial independence norms (varying degrees, varying enforcement). In Russia, major media outlets operate under Kremlin editorial influence while maintaining the formal structure of independent journalism. In China, all media operate within the party-state framework; the concept of editorial independence from the state is not merely absent but is explicitly rejected as a Western liberal construct. In many Gulf states, media ownership is intertwined with ruling family interests. In India, media ownership concentrates among conglomerates with diverse business interests that shape editorial direction. Categorizing all these as "media" and treating them as functionally equivalent would be a category error — they serve different discursive functions despite occupying the same institutional label.

**The "government" fallacy.** "Government communication" in Germany means Bundesregierung press releases — institutional, bureaucratic, constitutionally constrained. "Government communication" in China means a complex apparatus spanning the State Council Information Office, Xinhua News Agency, People's Daily, CGTN, and an ecosystem of state-adjacent social media accounts. "Government communication" in the United States includes the White House press office but also a decentralized landscape of federal agency communications, Congressional communications, and state-level government speech. The institutional label "government" conceals more than it reveals.

**The "civil society" fallacy.** The concept of "civil society" — autonomous organizations operating between state and market — is a Western construct with limited applicability in many contexts. In societies where clan, tribal, or religious structures mediate between individual and state, the Western "civil society" category either forces local institutions into an ill-fitting frame or renders them invisible. In China, the concept of "civil society" (公民社会) is politically sensitive; organizations that might be labeled "civil society" in a Western context operate under fundamentally different constraints. In Russia, the "foreign agent" law has redefined the boundary between civil society and state control.

### 2.2 Epistemological Colonialism in Research Design

The imposition of Western institutional categories onto non-Western societies is a form of what Santos (2014) calls **epistemological colonialism** — the assumption that Western knowledge frameworks are universal and that other frameworks are deviations from or approximations of the Western standard.

In comparative politics, this problem is well-recognized. Collier and Mahon (1993) warned against "conceptual stretching" — applying concepts beyond the contexts in which they are valid. Sartori (1970) demonstrated that comparative analysis requires concepts at the right level of abstraction — too concrete (specific to one society) and comparison is impossible; too abstract (universal platitudes) and comparison is meaningless.

For AĒR, epistemological colonialism would manifest in probe selection: choosing data sources based on Western institutional categories (e.g., "we need to crawl the government news agency and the independent press in each country") and thereby imposing a structural assumption about how societies organize their discourse. In societies where discourse is organized through religious institutions, diaspora networks, clan-based oral traditions digitized through WhatsApp, or state-corporate media hybrids, the Western institutional map produces blind spots — and those blind spots are not random. They systematically exclude non-Western forms of discourse organization.

### 2.3 Functional Equivalence as the Solution

The methodological solution comes from comparative anthropology and political science: **functional equivalence** (Dogan & Pelassy, 1990; Peters, 1998). Rather than asking "what is this institution called?" we ask "what function does this institution serve in this society?"

Functional equivalence acknowledges that:

1. The same societal function (e.g., norm-setting, power legitimation, identity formation) can be performed by different institutional forms in different societies.
2. The same institutional form (e.g., "newspaper") can serve different functions in different societies.
3. Some functions are universally present across human societies (all societies have mechanisms for establishing norms, legitimating power, forming identity, and contesting hegemony) even though the institutional carriers of these functions vary.

This universality of function, combined with diversity of form, is what makes cross-cultural comparison possible without cultural erasure. AĒR does not need to find "the equivalent of the New York Times in Japan" — it needs to find the sources that serve the *function* of agenda-setting and epistemic authority in Japanese society, whatever institutional form they take.

---

## 3. The Four Discourse Functions: A Taxonomy

### 3.1 Theoretical Grounding

The functional taxonomy proposed here draws on multiple theoretical traditions:

- **Foucault's discourse theory** — discourse as a system of rules that determines what can be said, by whom, and under what conditions (Foucault, 1969). Every society maintains boundaries of the expressible; the first function captures the actors who set these boundaries.
- **Gramsci's hegemony** — the dominance of a ruling class not through coercion alone but through cultural and ideological leadership (Gramsci, 1971). The second function captures the channels through which hegemonic narratives are produced and maintained.
- **Anderson's imagined communities** — the construction of collective identity through shared narratives, symbols, and media (Anderson, 1991). The third function captures the sources that produce the "we" of collective identity.
- **Scott's hidden transcripts** — the resistance discourses that subordinate groups maintain outside the public sphere (Scott, 1990). The fourth function captures the spaces where counter-narratives emerge and hegemonic discourse is contested.

These four theoretical traditions map to four universal discourse functions that are present — in culturally specific forms — across all connected societies.

### 3.2 Function 1: Epistemic Authority (Norm & Truth Setting)

**Definition.** Actors or channels that define the boundaries of the expressible — establishing what counts as true, morally acceptable, legally binding, or spiritually pure within a given society. Epistemic authorities do not merely report facts; they *constitute* the framework within which facts are interpreted.

**Theoretical basis.** Foucault's concept of *episteme* — the unconscious structure of knowledge that determines what can be thought and said in a given epoch. AĒR's philosophical Episteme pillar (Chapter 1, §1.2) is directly derived from this concept.

**Cross-cultural instantiations:**

| Society | Institutional Form | Why It Functions as Epistemic Authority |
| :--- | :--- | :--- |
| Germany | Bundesverfassungsgericht (Federal Constitutional Court), Tagesschau (public broadcaster), Robert Koch Institut (public health) | The Constitutional Court's rulings define the legal boundaries of the expressible. Tagesschau sets the informational baseline. The RKI defined scientific consensus during COVID-19. |
| United States | Supreme Court, New York Times / Washington Post, CDC, university system | The Supreme Court defines constitutional boundaries. "Newspaper of record" status confers epistemic authority. University credentialing validates expertise. |
| Iran | Guardian Council (*Shora-ye Negahban*), senior Shia clergy (marjas), IRIB (state broadcasting) | Religious authority and state authority are constitutionally fused. The Guardian Council vets legislation against Islamic law. Marjas issue *fatwas* that function as epistemic rulings. |
| Japan | NHK, major national dailies (*Asahi*, *Yomiuri*, *Mainichi*), Ministry of Health | NHK's public broadcasting mandate confers epistemic status. The national press club (*kisha kurabu*) system restricts access and confers authority on institutionally recognized media. |
| Nigeria | Religious leaders (Christian bishops, Islamic clerics), traditional rulers (*obas*, *emirs*), state broadcasting (NTA) | In a society where religious affiliation structures social identity, religious leaders function as epistemic authorities alongside — and sometimes in tension with — state institutions. Traditional rulers maintain epistemic authority in their domains. |
| China | People's Daily, Xinhua, CCTV (state media trinity), Central Committee communiqués, Chinese Academy of Sciences | The party-state defines the boundaries of the expressible through explicit propaganda directives and implicit self-censorship norms. Official media is not merely reporting — it is performing the epistemic function of the party-state. |

**Key insight:** Epistemic authority is not inherently state-based. In many societies, religious institutions, tribal elders, or professional guilds hold epistemic authority that is independent of (or in tension with) state power. AĒR's probe selection must identify all salient epistemic authorities, not just those recognized by Western institutional categories.

**Observation challenge:** Epistemic authority is partly constituted by what it *excludes*. The boundaries of the expressible are defined as much by what is unsayable as by what is said. AĒR's text metrics (WP-002) capture what is expressed; they do not capture what is suppressed. This is a fundamental limitation documented in the Manifesto (§II: "Resonance over Truth").

### 3.3 Function 2: Resource & Power Legitimation

**Definition.** Channels utilized by dominant structures — state, military, oligarchic, corporate, or dynastic — to justify their power, operationalize their narratives, and maintain their position. These sources do not merely describe power; they *enact* it through communication.

**Theoretical basis.** Gramsci's hegemony — the process by which ruling groups secure consent through cultural leadership. Habermas' concept of *strategic communication* — communication oriented toward success (achieving goals) rather than understanding (reaching consensus).

**Cross-cultural instantiations:**

| Society | Institutional Form | Legitimation Mechanism |
| :--- | :--- | :--- |
| Germany | Bundesregierung press office, party communication, corporate PR (DAX companies) | Democratic legitimation through policy explanation. Government PR operates within constitutional transparency norms. Corporate PR operates within market communication norms. |
| Russia | RT, TASS, Kremlin.ru, state governors' communications, oligarch-adjacent media | Sovereign legitimation — the state's right to define the national interest. Media serves as a legitimation instrument rather than a watchdog. Oligarch-adjacent media maintains a facade of independence while reinforcing state narratives. |
| Saudi Arabia | Saudi Press Agency (SPA), Al Arabiya, Vision 2030 media apparatus | Monarchical legitimation through modernization narrative. Vision 2030 is simultaneously an economic plan and a communication strategy that reframes absolute monarchy as visionary leadership. |
| United States | White House communications, Pentagon press briefings, corporate media (business press) | Democratic and market legitimation. The Pentagon's communication apparatus is one of the world's largest PR operations. Corporate media naturalizes market logic as common sense. |
| India | Government-affiliated media (Doordarshan, PIB), ruling party's digital apparatus, business media (CNBC, Economic Times) | Democratic legitimation complicated by party-aligned media ecosystems. The ruling BJP's digital communication infrastructure blurs the line between government and party communication. |

**Key insight:** Power legitimation is not limited to state actors. In many societies, corporate, religious, military, or dynastic power structures maintain their own communication channels that serve the legitimation function. In some contexts (Gulf states, oligarchic systems, corporate-dominated media landscapes), the distinction between "state" and "private" power legitimation is analytically meaningless.

**Probe selection challenge:** Power legitimation sources are often the *easiest* to crawl (official RSS feeds, structured press releases, public APIs) because they are designed for dissemination. This creates a structural bias in AĒR's observable corpus — power-legitimating discourse is overrepresented because it is the most technically accessible (WP-003, §2.2).

### 3.4 Function 3: Cohesion & Identity Formation

**Definition.** Sources that generate a collective "we-feeling" — constructing shared identity through cultural narratives, myth-making, ritual, and the structural demarcation of the in-group from the "other." These sources produce the affective infrastructure of belonging.

**Theoretical basis.** Anderson's *imagined communities* — the argument that nations are not natural entities but are constructed through shared narratives and media. Durkheim's *collective consciousness* — the shared beliefs and sentiments that bind a society together. Turner's *communitas* — the intense feeling of social togetherness that emerges in ritual contexts.

**Cross-cultural instantiations:**

| Society | Institutional Form | Identity Formation Mechanism |
| :--- | :--- | :--- |
| Germany | Public broadcasters (entertainment), football culture (Bundesliga, DFB), Feuilleton (cultural pages), regional identity media | Cultural identity is constructed through the interplay of national broadcasting, regional identity (*Heimat*), and the cultural commentary tradition of the *Feuilleton*. Football functions as a national ritual that generates collective affect. |
| Japan | NHK (*taiga* historical dramas, *kohaku* music program), manga/anime ecosystem, local festival (*matsuri*) media | Cultural identity is reproduced through shared media rituals (NHK's year-end program is watched by ~40% of the population) and through the global export of manga/anime culture that paradoxically reinforces domestic identity through international recognition. |
| Brazil | Globo TV (telenovelas), samba/carnaval media ecosystem, football, evangelical church media | Telenovelas function as national narrative machines — they construct and reconstruct Brazilian identity weekly. Evangelical media increasingly competes with secular media as an identity formation source, particularly for lower-income populations. |
| Turkey | TRT (state broadcaster), pop culture (music, TV series exported across MENA), mosque communities, diaspora media | Turkish identity formation operates on multiple levels: state-curated national identity (Atatürk legacy), religious identity (Diyanet-mediated), cultural-imperial identity (Ottoman nostalgia in TV series), and diaspora identity (Turkish-German, Turkish-Dutch media). |
| United States | Hollywood, sports (NFL, NBA), social media influencers, talk radio, church communities | Identity formation is intensely pluralistic and fragmented. There is no single national identity narrative but competing identity formation machines: progressive cultural production (Hollywood, universities), conservative identity media (talk radio, Fox News), racial and ethnic identity media, religious community media. |

**Key insight:** Cohesion and identity formation operate through affect, not information. These sources do not primarily convey facts — they generate feelings of belonging, pride, nostalgia, and solidarity. AĒR's sentiment metrics (WP-002) capture the affective surface, but the deeper identity-constructing function requires narrative analysis (Tier 2/3 methods) and cultural expertise.

**Probe selection challenge:** Identity formation sources are often culturally opaque to outside observers. A German researcher may not recognize the identity-forming function of a Nigerian Nollywood film or a Korean variety show. Probe selection for this function *requires* area studies expertise — there is no shortcut through technical analysis.

### 3.5 Function 4: Subversion & Friction (Counter-Discourse)

**Definition.** Decentralized, activist, or hyper-viral spaces that challenge hegemonic narratives, expose power structures, and operate as accelerators for affective, radical, or transformative discourse. These sources exist in productive tension with Functions 1–3.

**Theoretical basis.** Scott's *hidden transcripts* — the critique that subordinate groups maintain off-stage, outside the hearing of power. Fraser's *subaltern counterpublics* — the parallel discursive arenas where members of subordinated social groups formulate oppositional interpretations of their identities, interests, and needs (Fraser, 1990). Deleuze and Guattari's *rhizome* — the non-hierarchical, multiply-connected network that resists arborescent (tree-like, hierarchical) structures (AĒR's Rhizome pillar).

**Cross-cultural instantiations:**

| Society | Institutional Form | Counter-Discourse Mechanism |
| :--- | :--- | :--- |
| Germany | Investigative journalism (*Der Spiegel*, *CORRECTIV*), protest movements (Fridays for Future, Letzte Generation), alternative media, migrant community media | Counter-discourse operates within democratic norms: investigative journalism as institutionalized subversion; protest movements as constitutionally protected expression; alternative media at the margins of the media system. |
| Iran | Diaspora satellite channels (BBC Persian, Iran International), VPN-mediated social media, underground music/art scene, reformist clergy | Counter-discourse is structurally dangerous — it operates from exile, through encrypted channels, and under constant threat of state repression. The 2022 *Mahsa Amini* protests demonstrated how counter-discourse can mobilize through social media despite state suppression. |
| China | WeChat/Weibo coded language, overseas Chinese-language media (Initium, The Reporter), academic code-switching, VPN-mediated access | Counter-discourse exists in the gaps of censorship: coded language (*hexie* 河蟹 = "river crab" = "harmony" = censorship), rapid screen-capture before deletion, academic publications that push boundaries through theoretical abstraction. The Great Firewall creates a distinct counter-discourse ecosystem for those who cross it. |
| Nigeria | Social media activism (#EndSARS), citizen journalism, diaspora commentary, satirical blogs | The #EndSARS movement (2020) demonstrated the power of social media-driven counter-discourse in a post-colonial context. Counter-discourse in Nigeria operates in the interplay between local languages, Nigerian Pidgin, and English, each register carrying different political connotations. |
| Russia | Telegram channels (post-2022 especially), samizdat tradition digitized, exile media (Meduza, Novaya Gazeta Europe), anonymous forums | Post-2022, counter-discourse in Russia has been forced into encrypted, anonymized, and exile-based channels. Telegram functions simultaneously as an information source and a counter-public sphere. |

**Key insight:** Counter-discourse is by definition the most difficult to observe. It operates in spaces designed to evade institutional surveillance — encrypted messaging, coded language, exile platforms, pseudonymous accounts. AĒR's commitment to observing only publicly accessible data (Manifesto, §13.8 Source Selection Criteria) means that the most politically significant counter-discourse may be the least observable. This is not a failure of the system — it is a structural limitation that must be documented for each probe context.

**The dynamic boundary.** The four functions are not static categories. Discourse actors move between functions over time. A counter-discourse movement that gains power may transition to power legitimation (e.g., the ANC from anti-apartheid movement to governing party). An epistemic authority may be delegitimized and pushed to the margins (e.g., traditional media losing authority to social media in certain contexts). AĒR's longitudinal observation (WP-005) should track these functional transitions as phenomena of interest, not as classification errors.

---

## 4. The Etic/Emic Dual Tagging System

### 4.1 Theoretical Foundation

The etic/emic distinction originates in linguistics (Pike, 1967) and was adopted by cultural anthropology (Harris, 1976):

- **Etic** (from *phonetic*): an observer-imposed, cross-culturally applicable analytical framework. The observer classifies phenomena according to a universal schema that enables comparison across contexts. The etic perspective sacrifices local meaning for global comparability.
- **Emic** (from *phonemic*): a participant-derived, culturally specific description of phenomena. The analyst describes phenomena using categories meaningful to the people within the culture. The emic perspective sacrifices comparability for local validity.

Neither perspective is sufficient alone. Pure etic analysis imposes foreign categories and misses local meaning. Pure emic analysis produces rich ethnographic descriptions that cannot be compared across contexts. AĒR needs both — simultaneously.

### 4.2 Operationalization in the Pipeline

The Dual Tagging System embeds the etic/emic distinction directly into AĒR's data architecture through the `SilverMeta` layer (ADR-015):

**The Etic Layer (Global Comparability).** Each probe receives an abstract functional classification:

```python
class ProbeEticTag(BaseModel):
    """Etic layer: observer-imposed functional classification."""
    discourse_function: Literal[
        "epistemic_authority",
        "power_legitimation",
        "cohesion_identity",
        "subversion_friction"
    ]
    function_confidence: float  # 0.0–1.0, researcher's confidence in classification
    secondary_function: str | None = None  # many sources serve multiple functions
    classification_date: str
    classified_by: str  # researcher identifier for provenance
```

This classification enables ClickHouse to aggregate metrics across culturally equivalent sources — e.g., comparing sentiment trends across all "epistemic authority" sources worldwide, regardless of their institutional form.

**The Emic Layer (Local Reality).** Each probe also receives untranslated, context-specific metadata that preserves its cultural specificity:

```python
class ProbeEmicTag(BaseModel):
    """Emic layer: participant-derived, culturally specific context."""
    local_designation: str        # e.g., "kisha kurabu", "ulama", "Feuilleton"
    cultural_context: str         # free-text description of local significance
    local_language: str           # ISO 639-1 code
    societal_role_description: str  # what this source means to its community
    emic_categories: list[str]    # locally meaningful categories, untranslated
```

The emic layer is *never* used for cross-cultural aggregation. It is preserved for Progressive Disclosure — enabling an analyst who drills down from a global aggregate to understand the local context of a specific source.

### 4.3 Multi-Functionality and Dynamic Classification

Most real-world sources serve multiple discourse functions simultaneously. Tagesschau is both an epistemic authority (norm-setting through informational baseline) and a power legitimation channel (as a state-funded broadcaster whose editorial independence is structurally influenced by inter-party proportional governance of ARD). The New York Times is both an epistemic authority and, at times, a power legitimation channel (its editorial page has historically reflected establishment positions) and a counter-discourse vehicle (its investigative journalism has exposed government misconduct).

The Etic tag must account for multi-functionality through:

1. **Primary and secondary function tags.** Each source has a primary discourse function and optionally one or more secondary functions.
2. **Function weights.** For sources with strong multi-functionality, the etic tag should include confidence-weighted function assignments (e.g., `epistemic_authority: 0.6, power_legitimation: 0.3, cohesion_identity: 0.1`).
3. **Temporal function shifts.** The function classification is not permanent. Sources that change their discursive role over time (e.g., a previously independent media outlet that becomes state-captured) should receive updated etic tags with temporal markers.

### 4.4 The Classification Process

Assigning etic/emic tags is not a technical task — it is an interdisciplinary research act. The proposed classification process:

**Step 1: Area expert nomination.** For each cultural region, an area studies scholar identifies candidate sources for each discourse function. The scholar provides an initial emic description (what this source means in its cultural context) and a proposed etic classification (which discourse function it serves).

**Step 2: Peer review.** A second expert — ideally from the same cultural region but a different disciplinary background (e.g., political science if the first expert is an anthropologist) — reviews the proposed classification. Disagreements are documented, not resolved by fiat.

**Step 3: Technical feasibility assessment.** The engineering team assesses whether the source is technically accessible for AĒR's crawler ecosystem: public endpoints, API availability, Terms of Service compatibility, data format (WP-003 §2.2 accessibility dimensions).

**Step 4: Ethical review.** Following WP-006 (§5.2), each new probe undergoes ethical review: does observation risk harm to the source community? Are there vulnerable populations whose discourse would be exposed?

**Step 5: Registration.** The source is registered in PostgreSQL's `sources` table, a Source Adapter is implemented or configured, the etic/emic tags are stored in the source metadata, and the classification rationale is documented in a Probe Registration Record.

---

## 5. The Probe Constellation: From Individual Sources to Systemic Observation

### 5.1 The Minimum Viable Probe Set

A single source cannot represent a society's discourse. A government RSS feed captures the power legitimation function; it says nothing about counter-discourse, identity formation, or even epistemic authority (which may reside outside the state). AĒR's analytical power emerges not from individual probes but from **probe constellations** — sets of sources that, taken together, cover the four discourse functions for a given society.

The **Minimum Viable Probe Set (MVPS)** for a given society is the smallest set of sources that covers all four discourse functions with at least one probe each. An MVPS for Germany might consist of:

| Function | Probe | Platform Type |
| :--- | :--- | :--- |
| Epistemic Authority | Tagesschau RSS | RSS |
| Power Legitimation | Bundesregierung RSS | RSS |
| Cohesion & Identity | (to be determined — cultural media) | TBD |
| Subversion & Friction | (to be determined — investigative/activist) | TBD |

AĒR's Probe 0 (§13.10) covers only the first two rows — both from the same platform type (RSS) and both from the institutional sphere. This is explicitly acknowledged as a calibration probe, not a scientifically representative probe set.

### 5.2 Weighting and Representativeness

Within a probe constellation, sources carry different weight in the society's discourse. Tagesschau reaches millions daily; a niche investigative blog reaches thousands. Both are necessary for discourse observation, but they contribute differently to the aggregate picture.

Weighting raises difficult questions:

**Volume-based weighting** (weight sources by their publication volume or audience reach) amplifies the voices that are already loudest. This reproduces the very power asymmetries AĒR aims to observe.

**Equal weighting** (treat all sources equally regardless of reach) overrepresents marginal voices relative to their societal impact. This distorts the macroscopic picture.

**Function-based weighting** (weight sources by the salience of their discourse function) requires a theoretical judgment about which functions are more "important" — itself a culturally biased assessment.

The proposed approach: **AĒR should not weight probes into a single aggregate but should maintain function-stratified views.** The dashboard shows four discourse-function lanes, each with its own metrics. The analyst can see the epistemic authority lane, the power legitimation lane, the identity formation lane, and the counter-discourse lane separately — or overlay them to observe inter-functional dynamics (e.g., how counter-discourse responds to power legitimation narratives). This avoids the weighting problem by refusing to collapse the four functions into a single number.

### 5.3 The Probe Coverage Map

AĒR should maintain a **Probe Coverage Map** — a visualization that shows, for each cultural region, which discourse functions are covered by active probes and which remain unobserved. This map is the macroscope's equivalent of a telescope's field of view indicator — it shows the analyst not only what AĒR sees but what it is blind to.

The Coverage Map directly addresses WP-006's call for "negative space visualization" (§7.3, Q6) and the Manifesto's Digital Divide acknowledgment (§II).

---

## 6. Probe 0 Revisited: Classification Under the Taxonomy

AĒR's operational Probe 0 (§13.10) can now be formally classified under the functional taxonomy:

| Source | Primary Function | Secondary Function | Etic Confidence | Emic Context |
| :--- | :--- | :--- | :--- | :--- |
| bundesregierung.de RSS | Power Legitimation | Epistemic Authority (policy as fact-setting) | 0.85 | *Bundesregierung Pressemitteilungen* — official government communication in *Beamtendeutsch* register, constitutionally mandated transparency |
| tagesschau.de RSS | Epistemic Authority | Power Legitimation (state-funded, proportional governance) | 0.75 | *Öffentlich-rechtlicher Rundfunk* — public broadcasting under inter-party governance (*Rundfunkräte*), institutional register, perceived by German public as authoritative baseline |

**Gaps in Probe 0.** Functions 3 (Cohesion & Identity) and 4 (Subversion & Friction) are entirely unrepresented. Both represented sources are institutional, editorial, and German-language. This is not a defect of Probe 0 — it was designed as engineering calibration (§13.10). But it means that any sociological interpretation of Probe 0 data must be limited to the institutional sphere of German discourse.

---

## 7. Architectural Implications

### 7.1 Source Registration Schema

The PostgreSQL `sources` table (Migration 001) currently stores basic source metadata. The functional taxonomy requires extending this schema — or storing the classification in a related table:

```sql
CREATE TABLE IF NOT EXISTS source_classifications (
    source_id          INTEGER REFERENCES sources(id),
    primary_function   VARCHAR(30) NOT NULL,  -- etic tag
    secondary_function VARCHAR(30),
    function_weights   JSONB,                 -- e.g., {"epistemic_authority": 0.6, "power_legitimation": 0.3}
    emic_designation   TEXT NOT NULL,          -- local name, untranslated
    emic_context       TEXT NOT NULL,          -- cultural context description
    emic_language      VARCHAR(10),
    classified_by      VARCHAR(100) NOT NULL,
    classification_date DATE NOT NULL,
    review_status      VARCHAR(20) DEFAULT 'pending',  -- pending, reviewed, contested
    PRIMARY KEY (source_id, classification_date)
);
```

This table is additive — it does not modify existing `sources` entries. Multiple classification records per source enable temporal tracking of functional transitions.

### 7.2 SilverMeta Extension

The `SilverMeta` layer should propagate the etic/emic classification to each document, enabling ClickHouse aggregation by discourse function:

```python
class DiscourseContext(BaseModel):
    """Discourse function context, propagated from source classification."""
    primary_function: str
    secondary_function: str | None = None
    emic_designation: str
```

This field is populated by the Source Adapter during harmonization, reading from the `source_classifications` table. It is stored in `SilverMeta`, not `SilverCore` — discourse function classification is source-level context, not a property of the individual document.

### 7.3 ClickHouse Aggregation by Discourse Function

With the discourse function available in `SilverMeta` (and propagated to Gold metrics via the extractor pipeline), ClickHouse can aggregate metrics by function:

```sql
SELECT
    toStartOfDay(timestamp) AS ts,
    discourse_function,
    avg(value) AS avg_sentiment
FROM aer_gold.metrics_with_context  -- view joining metrics + discourse function
WHERE metric_name = 'sentiment_score'
GROUP BY ts, discourse_function
ORDER BY ts;
```

This enables the BFF API to serve function-stratified views, and the dashboard to render the four-lane visualization described in §5.2.

### 7.4 BFF API Extension

A new query parameter on `/api/v1/metrics`:

- `?discourseFunction=epistemic_authority` — filter metrics by discourse function
- `?discourseFunction=power_legitimation,subversion_friction` — comma-separated for multi-function views

And a new endpoint:

- `GET /api/v1/probes/coverage` — returns the Probe Coverage Map: for each registered cultural region, which discourse functions have active probes and which are unobserved.

---

## 8. Open Questions for Interdisciplinary Collaborators

### 8.1 For Cultural Anthropologists and Area Studies Scholars

**Q1: For each major cultural region, which sources serve each of the four discourse functions?**

- This is the foundational question. AĒR needs area experts to map the discourse landscape of their region of expertise onto the four-function taxonomy.
- Deliverable: Regional Probe Nomination Reports (3–5 pages each) identifying candidate sources for each discourse function, with emic descriptions and etic classifications, for at minimum: Germany, France, United Kingdom, United States, Brazil, Russia, China, Japan, India, Nigeria, Iran, Saudi Arabia, Indonesia, South Africa, Mexico.

**Q2: Are four discourse functions sufficient, or does the taxonomy need refinement?**

- Do some societies have discourse functions that do not map cleanly to the four categories? Is there a fifth function (e.g., "mediation" — actors that bridge between functions)? Should any function be split (e.g., distinguishing religious epistemic authority from scientific epistemic authority)?
- Deliverable: A critical review of the taxonomy based on empirical analysis of non-Western discourse landscapes.

**Q3: How should AĒR handle sources that shift discourse function over time?**

- When a formerly independent media outlet is captured by the state, it transitions from epistemic authority (or counter-discourse) to power legitimation. How should this transition be documented and when should the etic tag be updated?
- Deliverable: A protocol for detecting and documenting functional transitions, with criteria for re-classification.

### 8.2 For Comparative Political Scientists

**Q4: How should AĒR weight probes within a probe constellation?**

- The function-stratified approach (§5.2) avoids the weighting problem by refusing to aggregate across functions. But within a function, multiple probes may serve the same function with different reach. How should intra-function weighting be handled?
- Deliverable: A weighting methodology for intra-function probe aggregation, drawing on survey sampling theory and media system analysis.

**Q5: How does AĒR's functional taxonomy relate to existing media system typologies?**

- Hallin and Mancini's (2004) three models of media and politics (Liberal, Democratic Corporatist, Polarized Pluralist) is the dominant typology in comparative media studies. How does the functional taxonomy relate to, extend, or challenge these models?
- Deliverable: A mapping between Hallin-Mancini media system types and AĒR's discourse function taxonomy, identifying where the taxonomies align and where they diverge.

### 8.3 For Computational Social Scientists

**Q6: Can discourse function be detected computationally, or must it be assigned by human experts?**

- Is there a text-level signal that distinguishes epistemic authority discourse from power legitimation discourse? Could a trained classifier assign discourse function tags automatically, reducing the dependence on area experts?
- Deliverable: A feasibility study on automated discourse function classification, using a labeled corpus of sources with known function assignments.

**Q7: How should AĒR measure inter-functional dynamics?**

- The interaction between discourse functions — how counter-discourse responds to power legitimation, how epistemic authority mediates between competing identity narratives — is arguably the most analytically interesting phenomenon AĒR can observe. What metrics capture these dynamics?
- Deliverable: A set of inter-functional metrics (e.g., response latency between functions, topic convergence/divergence across functions, entity co-occurrence across functions), with mathematical definitions and computational feasibility assessment.

---

## 9. References

- Anderson, B. (1991). *Imagined Communities: Reflections on the Origin and Spread of Nationalism* (revised ed.). Verso.
- Collier, D. & Mahon, J. E. (1993). "Conceptual 'Stretching' Revisited: Adapting Categories in Comparative Analysis." *American Political Science Review*, 87(4), 845–855.
- Dogan, M. & Pelassy, D. (1990). *How to Compare Nations: Strategies in Comparative Politics* (2nd ed.). Chatham House.
- Foucault, M. (1969). *L'Archéologie du savoir* (The Archaeology of Knowledge). Gallimard.
- Fraser, N. (1990). "Rethinking the Public Sphere: A Contribution to the Critique of Actually Existing Democracy." *Social Text*, 25/26, 56–80.
- Gramsci, A. (1971). *Selections from the Prison Notebooks* (ed. & trans. Hoare, Q. & Nowell-Smith, G.). International Publishers.
- Hallin, D. C. & Mancini, P. (2004). *Comparing Media Systems: Three Models of Media and Politics*. Cambridge University Press.
- Harris, M. (1976). "History and Significance of the Emic/Etic Distinction." *Annual Review of Anthropology*, 5, 329–350.
- Peters, B. G. (1998). *Comparative Politics: Theory and Methods*. NYU Press.
- Pike, K. L. (1967). *Language in Relation to a Unified Theory of the Structure of Human Behavior* (2nd ed.). Mouton.
- Santos, B. de S. (2014). *Epistemologies of the South: Justice Against Epistemicide*. Routledge.
- Sartori, G. (1970). "Concept Misformation in Comparative Politics." *American Political Science Review*, 64(4), 1033–1053.
- Scott, J. C. (1990). *Domination and the Arts of Resistance: Hidden Transcripts*. Yale University Press.

---

## Appendix A: Mapping to AĒR Open Research Questions (§13.6)

| §13.6 Question | WP-001 Section | Status |
| :--- | :--- | :--- |
| 1. Probe Selection | §2–§6 (full treatment) | Addressed — functional taxonomy, dual tagging, probe constellation methodology |
| 2. Bias Calibration | §3.3 (power legitimation bias), §3.5 (counter-discourse observability) | Partially addressed — deferred to WP-003 |
| 4. Cross-Cultural Comparability | §4 (etic/emic as comparability mechanism) | Foundation laid — extended in WP-004 |
| 6. Observer Effect | §5.3 (probe coverage as transparency instrument) | Previewed — deferred to WP-006 |

## Appendix B: Probe Registration Record Template

For each new probe added to the AĒR system, the following record should be completed and stored alongside the `sources` entry:

```yaml
probe_registration:
  source_name: ""
  source_url: ""
  cultural_region: ""
  language: ""

  etic_classification:
    primary_function: ""           # epistemic_authority | power_legitimation | cohesion_identity | subversion_friction
    secondary_function: ""
    function_confidence: 0.0
    classified_by: ""
    classification_date: ""

  emic_context:
    local_designation: ""          # untranslated local name
    societal_role: ""              # what this source means to its community
    cultural_notes: ""             # relevant cultural context

  technical_assessment:
    platform_type: ""              # rss, api, web_scrape, etc.
    access_method: ""
    data_format: ""
    tos_compliant: true
    publication_volume: ""         # estimated documents per day

  ethical_review:
    reviewer: ""
    review_date: ""
    risk_assessment: ""            # low, medium, high
    vulnerable_populations: false
    harm_potential: ""

  bias_documentation:
    known_demographic_skew: ""
    editorial_constraints: ""
    platform_effects: ""           # from WP-003
    what_is_not_observed: ""       # explicit gap documentation
```