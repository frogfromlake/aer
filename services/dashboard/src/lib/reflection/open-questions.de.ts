// Open Research Questions — German (DE) prose catalog (Phase 144c / ADR-042).
//
// Locale-INDEPENDENT structure (id, sourceWp, sourceSection, subsection) stays
// in `open-questions.ts`; only the five user-visible prose fields localize here,
// keyed by the shared question `id`. The catalog accessors overlay these onto
// the English structure when the UI locale is `de` — a per-locale STATIC source
// (no BFF round-trip), like the per-locale Working-Paper bodies, resolved via
// paraglide's `getLocale()` so a switch re-renders with no reload and node tests
// fall back to the base locale (English). Faithfully transcribed from the
// approved German WP translations (`docs/methodology/de/WP-00X-de-*.md`, §7/§8);
// code identifiers and emic proper nouns stay verbatim. EN⇔DE id parity is
// enforced by `tests/unit/open-questions-parity.test.ts`.

import type { QuestionProse } from './open-questions';

export const OPEN_QUESTIONS_DE_PROSE: Record<string, QuestionProse> = {
  'wp-001-q1': {
    disciplinaryScope: 'Für Kulturanthropologen und Area-Studies-Wissenschaftler',
    shortLabel: 'Regionale Sondenzuordnung: welche Quellen erfüllen welche Diskursfunktion?',
    question:
      'Welche Quellen erfüllen in jeder großen Kulturregion jeweils welche der vier Diskursfunktionen? Dies ist die grundlegende Frage. AĒR benötigt Regionalexperten, die die Diskurslandschaft ihres Fachgebiets auf die Vier-Funktionen-Taxonomie abbilden.',
    deliverable:
      'Regionale Sonden-Nominierungsberichte (3–5 Seiten jeweils), die Kandidatenquellen für jede Diskursfunktion identifizieren, mit emischen Beschreibungen und etischen Klassifikationen, für mindestens: Deutschland, Frankreich, Vereinigtes Königreich, USA, Brasilien, Russland, China, Japan, Indien, Nigeria, Iran, Saudi-Arabien, Indonesien, Südafrika, Mexiko.',
    pipelineHook:
      'Sonden-Dossier-Verzeichnis; Source Adapter Protocol (ADR-015); emische SilverMeta-Felder'
  },
  'wp-001-q2': {
    disciplinaryScope: 'Für Kulturanthropologen und Area-Studies-Wissenschaftler',
    shortLabel:
      'Sind vier Diskursfunktionen ausreichend, oder muss die Taxonomie verfeinert werden?',
    question:
      'Gibt es in manchen Gesellschaften Diskursfunktionen, die sich nicht sauber auf die vier Kategorien abbilden lassen? Gibt es eine fünfte Funktion (z. B. „Vermittlung" — Akteure, die zwischen Funktionen brücken)? Sollte eine Funktion aufgespalten werden (z. B. Unterscheidung religiöser epistemischer Autorität von wissenschaftlicher epistemischer Autorität)?',
    deliverable:
      'Eine kritische Überprüfung der Taxonomie auf Basis empirischer Analyse nicht-westlicher Diskurslandschaften.',
    pipelineHook:
      'WP-001 §3 Taxonomie; die Vier-Bahnen-Struktur auf Oberfläche II; discourse_function-Feld in aer_gold.metrics'
  },
  'wp-001-q3': {
    disciplinaryScope: 'Für Kulturanthropologen und Area-Studies-Wissenschaftler',
    shortLabel:
      'Wie sollte AĒR mit Quellen umgehen, die ihre Diskursfunktion über die Zeit verschieben?',
    question:
      'Wenn ein vormals unabhängiges Medium vom Staat vereinnahmt wird, vollzieht es einen Übergang von epistemischer Autorität (oder Gegendiskurs) zu Machtlegitimation. Wie sollte dieser Übergang dokumentiert werden, und wann sollte der etische Tag aktualisiert werden?',
    deliverable:
      'Ein Protokoll zur Erkennung und Dokumentation funktionaler Übergänge, mit Kriterien für die Reklassifikation.',
    pipelineHook: 'Etische Tag-Felder in SilverMeta; Revisionsverlauf im Sonden-Dossier'
  },
  'wp-001-q4': {
    disciplinaryScope: 'Für vergleichende Politikwissenschaftler',
    shortLabel: 'Wie sollte AĒR Sonden innerhalb einer Sondenkonstellation gewichten?',
    question:
      'Der funktionsstratifizierte Ansatz (§5.2) vermeidet das Gewichtungsproblem, indem er die Aggregation über Funktionen hinweg ablehnt. Aber innerhalb einer Funktion können mehrere Sonden dieselbe Funktion mit unterschiedlicher Reichweite erfüllen. Wie sollte intrafunktionale Gewichtung gehandhabt werden?',
    deliverable:
      'Eine Gewichtungsmethodik für intrafunktionale Sondenaggregation, gestützt auf Stichprobentheorie und Mediensystemanalyse.',
    pipelineHook:
      'Architektur der Sondenkonstellation; Funktionsabdeckungs-Indikator im Sonden-Dossier'
  },
  'wp-001-q5': {
    disciplinaryScope: 'Für vergleichende Politikwissenschaftler',
    shortLabel:
      'Wie verhält sich AĒRs funktionale Taxonomie zu bestehenden Mediensystem-Typologien?',
    question:
      'Hallin und Mancinis (2004) drei Modelle von Medien und Politik (Liberal, Demokratisch-Korporatistisch, Polarisiert-Pluralistisch) bilden die dominante Typologie in der vergleichenden Medienwissenschaft. Wie verhält sich die funktionale Taxonomie zu diesen Modellen — erweitert, ergänzt oder stellt sie sie in Frage?',
    deliverable:
      'Eine Abbildung zwischen Hallin-Mancini-Mediensystemtypen und AĒRs Diskursfunktionstaxonomie, die identifiziert, wo die Taxonomien übereinstimmen und wo sie divergieren.',
    pipelineHook: 'Sondenregistrierung; etische Klassifikation in SilverMeta'
  },
  'wp-001-q6': {
    disciplinaryScope: 'Für Computational Social Scientists',
    shortLabel: 'Kann die Diskursfunktion rechnerisch erkannt werden?',
    question:
      'Gibt es ein Signal auf Textebene, das epistemischen Autoritätsdiskurs von Machtlegitimationsdiskurs unterscheidet? Könnte ein trainierter Klassifikator Diskursfunktions-Tags automatisch zuweisen und so die Abhängigkeit von Regionalexperten reduzieren?',
    deliverable:
      'Eine Machbarkeitsstudie zur automatisierten Diskursfunktionsklassifikation, unter Verwendung eines gelabelten Korpus von Quellen mit bekannten Funktionszuweisungen.',
    pipelineHook: 'MetricExtractor-Protokoll; discourse_function-Ableitung in processor.py'
  },
  'wp-001-q7': {
    disciplinaryScope: 'Für Computational Social Scientists',
    shortLabel: 'Wie sollte AĒR interfunktionale Dynamiken messen?',
    question:
      'Die Interaktion zwischen Diskursfunktionen — wie Gegendiskurs auf Machtlegitimation reagiert, wie epistemische Autorität zwischen konkurrierenden Identitätsnarrativen vermittelt — ist wohl das analytisch interessanteste Phänomen, das AĒR beobachten kann. Welche Metriken erfassen diese Dynamiken?',
    deliverable:
      'Ein Set interfunktionaler Metriken (z. B. Reaktionslatenz zwischen Funktionen, Themenkonvergenz/-divergenz über Funktionen, Entity-Ko-Okkurrenz über Funktionen), mit mathematischen Definitionen und Machbarkeitsbewertung der Berechnung.',
    pipelineHook:
      'EntityCoOccurrenceExtractor; aer_gold.entity_cooccurrences; Network-Science-View-Mode-Zellen'
  },
  'wp-002-q1': {
    disciplinaryScope: 'Für Computational Social Scientists',
    shortLabel: 'Welche Sentimentmethode ist für AĒRs Constraints geeignet?',
    question:
      'Welche Sentimentmethode(n) sind für deutschen redaktionellen Text aus institutionellen RSS-Feeds geeignet, unter Berücksichtigung von AĒRs Constraints für Determinismus und Transparenz? AĒRs aktuelle Methode (SentiWS-Lexikon, mittlere Polarität) ist eine Tier-1-Baseline. Gibt es eine validierte, deterministische Alternative, die Negation und Kompositionalität handhabt und dabei auditierbar bleibt? Falls die Antwort „keine deterministische Methode ist adäquat" lautet, wie sollte AĒR eine Tier-2-Methode (z. B. einen feinabgestimmten Klassifikator mit fixierter Modellversion) integrieren und dabei Nachvollziehbarkeit bewahren?',
    deliverable: 'Eine Empfehlung mit Validierungsevidenz auf einem vergleichbaren Korpus.',
    pipelineHook: 'SentimentExtractor; metric_provenance.yaml; Tier-1/2-Klassifikation'
  },
  'wp-002-q2': {
    disciplinaryScope: 'Für Computational Social Scientists',
    shortLabel: 'Welches Annotationsschema sollte AĒR für die Sentiment-Validierung verwenden?',
    question:
      'Ist eine eindimensionale Positiv-Negativ-Skala angemessen, oder benötigt AĒR ein mehrdimensionales Affektmodell (Valenz + Erregung, oder diskrete Emotionen)? Wie sollten Annotatoren mit institutionellem Register umgehen (Pressemitteilungen, die bewusst neutral sind)? Ist „neutral" die Abwesenheit von Sentiment, oder ist es eine eigenständige kommunikative Haltung?',
    deliverable: 'Ein Annotationscodebuch mit Arbeitsbeispielen aus deutschen RSS-Beschreibungen.',
    pipelineHook:
      'SentimentExtractor; Tabelle aer_gold.metric_validity; Validierungsrahmenwerk in §6'
  },
  'wp-002-q3': {
    disciplinaryScope: 'Für Computational Social Scientists',
    shortLabel: 'Wie sollte AĒR Sentimentscores für quellenübergreifende Vergleiche normalisieren?',
    question:
      'Pressemitteilungen der Bundesregierung und Tagesschau-Artikel haben strukturell unterschiedliche Sentiment-Baselines. Wie vergleichen wir Sentiment-Veränderung über Quellen hinweg, wenn die absoluten Niveaus nicht vergleichbar sind?',
    deliverable:
      'Eine Normalisierungsstrategie (z-score pro Quelle, Perzentil-Ranking, Differenz-in-Differenzen) mit statistischer Begründung.',
    pipelineHook:
      'Parameter normalization=zscore in GET /api/v1/metrics; Äquivalenzregister (WP-004 §5.2); Refusal-Typ normalization_equivalence_missing'
  },
  'wp-002-q4': {
    disciplinaryScope: 'Für Computerlinguisten / NLP-Forscher',
    shortLabel: 'Wie sollte AĒR deutsche Komposita in der Sentimentanalyse behandeln?',
    question:
      'Sollten Komposita vor dem Lexikon-Lookup dekomponiert werden? Wenn ja, welche Dekompositionsstrategie ist angemessen (frequenzbasiert, regelbasiert, neural)? Wie erstreckt sich dies auf andere agglutinative Sprachen (Türkisch, Finnisch, Koreanisch), wenn AĒR über das Deutsche hinaus expandiert?',
    deliverable:
      'Eine Evaluation von Komposita-Dekompositionsstrategien auf einem deutschen Nachrichtenkorpus mit Auswirkung auf die Sentiment-Scoring-Genauigkeit.',
    pipelineHook: 'SentimentExtractor; Integration des SentiWS v2.0-Lexikons'
  },
  'wp-002-q5': {
    disciplinaryScope: 'Für Computerlinguisten / NLP-Forscher',
    shortLabel: 'Welche NER-Modelle und Entity-Linking-Strategien sind für AĒR geeignet?',
    question:
      'Das aktuelle spaCy de_core_news_lg-Modell ist einsprachig. AĒR wird Arabisch, Mandarin, Hindi, Portugiesisch, Spanisch, Französisch und weitere Sprachen verarbeiten müssen. Ist ein einzelnes mehrsprachiges Modell (XLM-RoBERTa) pro-Sprache-Modellen vorzuziehen? Wie sollte Entity Linking mit Entitäten umgehen, die in nicht-westlichen Kontexten hochrelevant, aber in Wikidata unterrepräsentiert sind?',
    deliverable:
      'Ein Benchmark-Vergleich von NER-/Entity-Linking-Ansätzen auf einem mehrsprachigen Nachrichtenkorpus, evaluiert pro Sprache und Entity-Typ.',
    pipelineHook: 'NamedEntityExtractor; spaCy-Modellauswahl; aer_gold.entities'
  },
  'wp-002-q6': {
    disciplinaryScope: 'Für Computerlinguisten / NLP-Forscher',
    shortLabel: 'Wie sollte AĒR code-geswitchte Texte erkennen und behandeln?',
    question:
      'In mehrsprachigen digitalen Räumen mischen Dokumente häufig Sprachen. Wie sollten Spracherkennung, Sentimentbewertung und Entity-Extraktion auf code-geswitchtem Text operieren?',
    deliverable:
      'Ein Survey über Code-Switching-Erkennungsmethoden mit einer Empfehlung für AĒRs pro-Dokument-Verarbeitungsmodell.',
    pipelineHook:
      'LanguageDetectionExtractor; langdetect-Seed; aer_gold.language_detections; Feld SilverCore.language'
  },
  'wp-002-q7': {
    disciplinaryScope: 'Für Kulturanthropologen und Area-Studies-Wissenschaftler',
    shortLabel: 'Wie sollte AĒR Sentiment-Baselines über Kulturen hinweg kalibrieren?',
    question:
      'Wenn japanische redaktionelle Prosa systematisch zurückhaltender ist als amerikanischer Journalismus, erfordert ein interkultureller Sentimentvergleich eine Baseline-Normalisierung. Aber die Baseline ist nicht lediglich ein statistisches Artefakt — sie ist das kulturelle Phänomen, das AĒR zu beobachten beabsichtigt. Ist Baseline-Normalisierung ein Akt kultureller Auslöschung? Sollte AĒR Roh-Baselines bewahren und die Differenz als Befund visualisieren?',
    deliverable:
      'Ein Positionspapier zur epistemologischen Spannung zwischen Normalisierung und kultureller Beobachtung.',
    pipelineHook:
      'z-score-Normalisierungs-Gate; Metric Equivalence Registry (WP-004 §5.2); Prinzip der nicht-präskriptiven Visualisierung (Design Brief §7.5)'
  },
  'wp-002-q8': {
    disciplinaryScope: 'Für Kulturanthropologen und Area-Studies-Wissenschaftler',
    shortLabel: 'Wie sollte AĒRs Entity-Ontologie kulturspezifische Organisationsformen behandeln?',
    question:
      'WP-001 definiert ein Duales Tagging-System (etische/emische Schichten). Wie sollte dies auf die Entity-Klassifikation erweitert werden? Sollte ein japanisches keiretsu in der etischen Schicht als ORG getaggt werden und dabei seine emische Spezifität bewahren? Welche kulturspezifischen Entity-Typen werden von westlichen NER-Modellen systematisch übersehen?',
    deliverable:
      'Eine interkulturelle Entity-Taxonomie, die spaCys Standardlabels um kulturell informierte Kategorien erweitert, abgebildet auf WP-001s emische Schicht.',
    pipelineHook:
      'NamedEntityExtractor; Feld entity_label in aer_gold.entities; emische SilverMeta-Tags'
  },
  'wp-002-q9': {
    disciplinaryScope: 'Für Methodologen und Statistiker',
    shortLabel:
      'Wie sollte AĒR das ökologische Inferenzproblem in der Aggregation berücksichtigen?',
    question:
      'Wenn Sentimentmetriken über Dokumente, Quellen und Zeitfenster in der Gold-Schicht aggregiert werden, wie kann AĒR erkennen, ob beobachtete Trends genuine Diskursverschiebungen widerspiegeln vs. kompositorische Veränderungen im Korpus?',
    deliverable:
      'Ein statistisches Framework zur Dekomposition aggregierter Metrikänderungen in innerhalb-der-Quelle- und zwischen-Quellen-Komponenten.',
    pipelineHook:
      'GET /api/v1/metrics mit Quellenfilter; Progressive Disclosure (L5 Evidence Reader); CorpusExtractor-Protokoll'
  },
  'wp-002-q10': {
    disciplinaryScope: 'Für Methodologen und Statistiker',
    shortLabel:
      'Welches temporale Aggregationsfenster ist für verschiedene Metriktypen angemessen?',
    question:
      'Sentiment kann bei täglicher Auflösung für Eilmeldungen bedeutsam sein, aber nur bei wöchentlicher oder monatlicher Auflösung für kulturellen Drift. Themenprävalenz kann andere temporale Fenster erfordern als Entity-Ko-Okkurrenz-Netzwerke.',
    deliverable:
      'Eine empirische Analyse der Metrikstabilität über temporale Auflösungen auf einem Pilotkorpus.',
    pipelineHook:
      'TemporalDistributionExtractor; 5-Minuten-Downsampling in GET /api/v1/metrics; die temporalen Aleph-/Episteme-Modi'
  },
  'wp-003-q1': {
    disciplinaryScope: 'Für Plattform-Governance- und Internetstudien-Wissenschaftler',
    shortLabel: 'Wie sollte AĒR algorithmische Kurationseffekte dokumentieren und berücksichtigen?',
    question:
      'AĒRs RSS-basierte Sonde 0 vermeidet algorithmische Kuration nahezu vollständig. Bei der Expansion zu Social-Media-Plattformen: welche Metadaten und kontrafaktischen Strategien sind erforderlich, um algorithmisches Signal von gesellschaftlichem Signal zu unterscheiden?',
    deliverable:
      'Ein Framework zur Dokumentation algorithmischer Kurationseffekte pro Plattform, geeignet für die Einbindung in SilverMeta.',
    pipelineHook:
      'BiasContext-Felder in SilverMeta (WP-003 §8.1); Source Adapter Protocol (ADR-015)'
  },
  'wp-003-q2': {
    disciplinaryScope: 'Für Plattform-Governance- und Internetstudien-Wissenschaftler',
    shortLabel: 'Wie sollte AĒR die Post-2023-Forschungs-API-Landschaft navigieren?',
    question:
      'Da X, Reddit und Meta den akademischen API-Zugang einschränken, welche alternativen Datenerhebungsstrategien sind verfügbar, ethisch und rechtlich vertretbar? Welche Plattformen bieten derzeit robuste akademische Forschungs-APIs, und was sind ihre bekannten Limitierungen?',
    deliverable:
      'Eine Plattform-für-Plattform-Zugangsbewertung mit rechtlichen und ethischen Annotationen, jährlich aktualisiert.',
    pipelineHook: 'RSS Crawler (aktuell); zukünftige Crawler-Architektur in crawlers/'
  },
  'wp-003-q3': {
    disciplinaryScope: 'Für Computational Social Scientists und Bot-Forscher',
    shortLabel: 'Welche Bot-Erkennungsmethoden sind auf rein textbasierte Verarbeitung anwendbar?',
    question:
      'Rein textbasierte Bot-Erkennung ist weniger genau als Account-Level-Erkennung. Welche Methoden sind auf AĒRs Architektur anwendbar, angesichts der Tatsache, dass AĒR Text und Metadaten verarbeitet, aber oft keine vollständigen Verhaltensdaten auf Account-Ebene hat? Welche Konfidenzschwellen sind für ein System angemessen, das flaggt statt filtert?',
    deliverable:
      'Eine Evaluation von Text- und Metadaten-Level-Bot-Erkennungsmethoden auf einem mehrsprachigen Korpus, mit Falsch-positiv-/Falsch-negativ-Raten pro Methode.',
    pipelineHook:
      'Zukünftige Authentizitäts-Extraktoren (MetricExtractor-Protokoll); human_authorship_confidence-Metrik'
  },
  'wp-003-q4': {
    disciplinaryScope: 'Für Computational Social Scientists und Bot-Forscher',
    shortLabel: 'Wie sollte AĒR KI-generierten Inhalt erkennen und flaggen?',
    question:
      'Aktuelle Erkennungsmethoden degradieren schnell. Gibt es eine nachhaltige Erkennungsstrategie, oder sollte AĒR KI-generierten Inhalt als beobachtbares Phänomen akzeptieren und sich auf die Messung seiner Prävalenz konzentrieren statt auf das Herausfiltern?',
    deliverable:
      'Ein Positionspapier zum epistemologischen Status KI-generierten Inhalts in Diskursbeobachtungssystemen, mit praktischen Empfehlungen.',
    pipelineHook:
      'Zukünftige Tier-2/3-Authentizitäts-Extraktoren; template_match_score-Metrik; MetricExtractor-Protokoll'
  },
  'wp-003-q5': {
    disciplinaryScope: 'Für Computational Social Scientists und Bot-Forscher',
    shortLabel:
      'Wie kann AĒR koordiniertes inauthentisches Verhalten mittels Korpus-Level-Analyse erkennen?',
    question:
      'CIB-Erkennung erfordert typischerweise Netzwerkdaten (Follower-Graphen, Repost-Ketten). Können temporale Ko-Okkurrenz und Inhaltsähnlichkeit in AĒRs Gold-Schicht als Stellvertreter dienen?',
    deliverable:
      'Eine Methode zur CIB-Erkennung unter ausschließlicher Nutzung der in SilverCore und SilverMeta verfügbaren Features (Zeitstempel, Quellidentifikatoren, bereinigter Text), evaluiert gegen bekannte CIB-Kampagnen.',
    pipelineHook:
      'CorpusExtractor-Protokoll; aer_gold.entity_cooccurrences; EntityCoOccurrenceExtractor'
  },
  'wp-003-q6': {
    disciplinaryScope: 'Für Area-Studies-Wissenschaftler und digitale Anthropologen',
    shortLabel: 'Welche Plattformen bilden für jede Kulturregion das Minimum Viable Probe Set?',
    question:
      'AĒRs WP-001-Taxonomie definiert vier Diskursfunktionen. Für eine gegebene Gesellschaft (z. B. Brasilien, Japan, Nigeria, Iran), welche Plattformen bilden sich auf welche Funktionen ab? Welche Plattformen sind technisch, rechtlich und ethisch zugänglich?',
    deliverable:
      'Regionale Plattformkarten (1–2 Seiten jeweils) für mindestens: Ostasien, Südasien, MENA, Subsahara-Afrika, Lateinamerika, Russland/postsowjetischer Raum, Europa, Nordamerika.',
    pipelineHook: 'Sondenregistrierung; Crawler-Architektur; Source Adapter Protocol'
  },
  'wp-003-q7': {
    disciplinaryScope: 'Für Area-Studies-Wissenschaftler und digitale Anthropologen',
    shortLabel:
      'Wie beeinflussen plattformspezifische Kommunikationsnormen die Interpretierbarkeit von Metriken?',
    question:
      'Ein Twitter/X-Thread, ein Reddit-Kommentar, ein Telegram-Kanalpost und ein RSS-Artikel über dasselbe Ereignis produzieren strukturell unterschiedlichen Text. Wie sollte AĒR plattformspezifische Genreeffekte beim Vergleich von Metriken über Quellen hinweg berücksichtigen?',
    deliverable:
      'Eine Plattform-Genre-Taxonomie, die die strukturellen, rhetorischen und pragmatischen Unterschiede zwischen auf verschiedenen Plattformen produziertem Text dokumentiert.',
    pipelineHook:
      'SilverMeta platform_type-Feld; source_type in SilverCore; Diskursfunktions-Ableitung'
  },
  'wp-003-q8': {
    disciplinaryScope: 'Für Survey-Methodologen und Statistiker',
    shortLabel: 'Können Surveygewichtungstechniken für digitale Diskursdaten adaptiert werden?',
    question:
      'Surveygewichtung erfordert bekannte Populationsparameter und bekannte Selektionswahrscheinlichkeiten. Keines von beiden ist für digitale Plattformen verfügbar. Unter welchen Bedingungen, falls überhaupt, kann Gewichtung demografische Verzerrung in gecrawlten Daten reduzieren?',
    deliverable:
      'Eine methodologische Bewertung von Gewichtungsstrategien für nicht-probabilistische digitale Stichproben, mit Empfehlungen für AĒRs Aggregationspipeline.',
    pipelineHook:
      'Gold-Schicht-Aggregation; Negativraum-Overlay (Phase 113); demografische Verzerrung in WP-003 §6'
  },
  'wp-003-q9': {
    disciplinaryScope: 'Für Survey-Methodologen und Statistiker',
    shortLabel:
      'Wie sollte AĒR die durch Plattform-Bias eingeführte Unsicherheit modellieren und berichten?',
    question:
      'Wenn alle Metriken plattforminduzierte Unsicherheit tragen, wie sollte diese Unsicherheit durch die Aggregationspipeline propagiert und im Dashboard visualisiert werden?',
    deliverable:
      'Ein Unsicherheitsquantifizierungs-Framework für plattformvermittelte Diskursmetriken, kompatibel mit AĒRs ClickHouse-Gold-Schema.',
    pipelineHook:
      'aer_gold.metrics-Schema; known_limitations in metric_provenance.yaml; Unsicherheitsvisualisierung in Surface-II-Diagrammen'
  },
  'wp-004-q1': {
    disciplinaryScope: 'Für vergleichende Sozialwissenschaftler und Methodologen',
    shortLabel: 'Welche von AĒRs Metriken sind Kandidaten für interkulturelle skalare Äquivalenz?',
    question:
      'Sentimentpolarität, Themenprävalenz, Entity-Salienz, Narrativrahmen — für jede: Gibt es einen realistischen Weg zur Etablierung skalarer Äquivalenz über Sprachen hinweg, oder sollte AĒR akzeptieren, dass diese Metriken nur intrakulturell sinnvoll sind?',
    deliverable:
      'Eine Metrik-für-Metrik-Bewertung des Vergleichbarkeitspotenzials, mit empfohlenen Vergleichsebenen (temporal, Abweichung, absolut) für jede.',
    pipelineHook:
      'Tabelle aer_gold.metric_equivalence; normalization=zscore-Gate; Verweigerungstyp normalization_equivalence_missing'
  },
  'wp-004-q2': {
    disciplinaryScope: 'Für vergleichende Sozialwissenschaftler und Methodologen',
    shortLabel: 'Welche Validierungsmethodik etabliert interkulturelle Metrikäquivalenz?',
    question:
      'In der Surveymethodik wird Messinvarianz über Multi-Gruppen-Konfirmatorische Faktorenanalyse (MGCFA) getestet. Gibt es eine analoge Methodik für computationale Textmetriken?',
    deliverable:
      'Ein Validierungsprotokoll für interkulturelle Metrikäquivalenz, adaptiert für computationale Diskursanalyse.',
    pipelineHook: 'aer_gold.metric_equivalence; Validierungs-Framework in WP-002 §6'
  },
  'wp-004-q3': {
    disciplinaryScope: 'Für vergleichende Sozialwissenschaftler und Methodologen',
    shortLabel:
      'Wie sollte AĒR mit Metriken umgehen, die intrakulturell valide, aber interkulturell inkommensurabel sind?',
    question:
      'Sollten solche Metriken nebeneinander mit expliziten Nicht-Äquivalenz-Warnungen angezeigt werden? Sollte das Dashboard kulturell kontextualisierte interpretive Rahmen anbieten? Sollten inkommensurable Metriken aus interkulturellen Ansichten gänzlich ausgeschlossen werden?',
    deliverable:
      'Dashboard-Designrichtlinien für die Präsentation nicht-äquivalenter, aber ko-dargestellter Metriken.',
    pipelineHook:
      'Verweigerungsoberfläche für normalization_equivalence_missing; Progressive Disclosure; Notizen zum kulturellen Kontext in der Metrik-Provenienz'
  },
  'wp-004-q4': {
    disciplinaryScope: 'Für Computerlinguisten',
    shortLabel:
      'Welche Tokenisierungsstrategie ermöglicht die sinnvollsten sprachübergreifenden Wort-Level-Metriken?',
    question:
      'Sollte AĒR sprachspezifische Tokenisierer verwenden (spaCy pro Sprache, jieba für Chinesisch, MeCab für Japanisch), einen universellen Subwort-Tokenisierer (SentencePiece, BPE) oder Zeichen-Level-Features?',
    deliverable:
      'Eine vergleichende Evaluation von Tokenisierungsstrategien auf einem mehrsprachigen Nachrichtenkorpus, mit Auswirkung auf nachgelagerte Metrikvergleichbarkeit.',
    pipelineHook:
      'Extractor-Pipeline des Analysis Workers; künftige Unterstützung mehrsprachiger Extraktoren'
  },
  'wp-004-q5': {
    disciplinaryScope: 'Für Computerlinguisten',
    shortLabel: 'Können mehrsprachige Satz-Embeddings als interkultureller Merkmalsraum dienen?',
    question:
      'Modelle wie XLM-RoBERTa und LaBSE produzieren sprachunabhängige Embeddings. Sind diese Embeddings hinreichend kulturell neutral für interkulturelles Topic-Alignment, oder betten sie englischzentrische semantische Strukturen ein?',
    deliverable:
      'Eine Evaluation mehrsprachiger Embedding-Räume für interkulturelles Topic-Alignment, mit besonderer Aufmerksamkeit auf nicht-europäische Sprachen.',
    pipelineHook: 'Künftige Tier-2/3-Topic-Extraktoren; geplanter LDA/BERTopic-CorpusExtractor'
  },
  'wp-004-q6': {
    disciplinaryScope: 'Für Kulturanthropologen und Area-Studies-Wissenschaftler',
    shortLabel:
      'Welche expressiven Normen beeinflussen Textmetrik-Baselines über Kulturregionen hinweg?',
    question:
      'AĒR benötigt Dokumentation expressiver Konventionen, institutioneller Register und Kommunikationsnormen, die beeinflussen, wie Textmetriken in jedem kulturellen Kontext interpretiert werden sollten.',
    deliverable:
      'Kulturelle Kalibrierungsprofile (2–3 Seiten jeweils) für dieselben Regionen, die in WP-003 Q6 identifiziert wurden, fokussiert auf textebene Kommunikationsnormen statt Plattformwahl.',
    pipelineHook:
      'Notizen zum kulturellen Kontext in metric_provenance.yaml; Abschnitt zum kulturellen Kontext im Methodik-Tray'
  },
  'wp-004-q7': {
    disciplinaryScope: 'Für Kulturanthropologen und Area-Studies-Wissenschaftler',
    shortLabel: 'Welche Konzepte und Themen sollten nicht interkulturell aligniert werden?',
    question:
      'Manche Diskursthemen sind so tief in den lokalen kulturellen, historischen und politischen Kontext eingebettet, dass interkulturelles Alignment irreführend wäre. Die Identifikation dieser „nicht-vergleichbaren" Themen ist ebenso wichtig wie die Identifikation vergleichbarer.',
    deliverable:
      'Eine Liste kulturell gebundener Diskurskonzepte pro Region, mit Erklärungen, warum interkulturelles Alignment unangemessen ist.',
    pipelineHook: 'Content catalog refusal entries; Nicht-Äquivalenz-Warnungen im Dashboard'
  },
  'wp-004-q8': {
    disciplinaryScope: 'Für Statistiker und Datenwissenschaftler',
    shortLabel:
      'Welche Baseline-Kalibrierungsperiode ist für stabile Z-Score-Berechnung erforderlich?',
    question:
      'Wie viele Dokumente, über wie viele Tage, werden benötigt, um eine zuverlässige Baseline zu etablieren? Unterscheidet sich die erforderliche Kalibrierungsperiode nach Quellentyp (hochvolumiger Nachrichten-Feed vs. niedrigvolumige Regierungspressemitteilungen)?',
    deliverable:
      'Eine statistische Analyse der Baseline-Stabilität als Funktion von Korpusgröße und temporalem Fenster, unter Verwendung von AĒRs Sonde-0-Daten als Pilot.',
    pipelineHook:
      'normalization=zscore in GET /api/v1/metrics; aer_gold.metric_equivalence; Baseline-Kalibrierungs-Metadaten'
  },
  'wp-004-q9': {
    disciplinaryScope: 'Für Statistiker und Datenwissenschaftler',
    shortLabel:
      'Wie sollte AĒR Normalisierungsunsicherheit durch die Aggregationspipeline propagieren?',
    question:
      'Wenn Z-Scores aus unsicheren Baselines berechnet und über Quellen hinweg aggregiert werden, wie sollte die resultierende Unsicherheit quantifiziert und dargestellt werden?',
    deliverable:
      'Ein Framework zur Unsicherheitspropagation für normalisierte Diskursmetriken, kompatibel mit ClickHouse-Aggregationsfunktionen.',
    pipelineHook:
      'aer_gold.metrics-Schema; Verteilungs-Endpunkt GET /api/v1/metrics; Unsicherheitsvisualisierung in Oberfläche II'
  },
  'wp-005-q1': {
    disciplinaryScope: 'Für Zeitreihenanalysten und Statistiker',
    shortLabel:
      'Welche temporale Dekompositionsmethode ist für AĒRs Diskurszeitreihen am besten geeignet?',
    question:
      'Klassische Dekomposition, STL, Wavelet-Analyse oder eine Kombination? AĒRs Zeitreihen haben irreguläre Abtastung (RSS-Feeds publizieren nicht in festen Intervallen), potenzielle Strukturbrüche (Algorithmusänderungen, neue Quellen hinzugefügt) und mehrere überlappende Periodizitäten (täglich, wöchentlich, saisonal).',
    deliverable:
      'Eine vergleichende Evaluation von Dekompositionsmethoden auf AĒRs Sonde-0-Daten, mit Empfehlungen für jede temporale Skala.',
    pipelineHook:
      'GET /api/v1/metrics mit resolution-Parameter; TemporalDistributionExtractor; die drei temporalen Säulen-Modi'
  },
  'wp-005-q2': {
    disciplinaryScope: 'Für Zeitreihenanalysten und Statistiker',
    shortLabel: 'Wie sollte AĒR Change Points in Diskursmetriken berechnen?',
    question:
      'Welcher Change-Point-Erkennungsalgorithmus (CUSUM, PELT, Bayesianisch) ist für AĒRs Datencharakteristiken (verrauscht, irregulär abgetastet, potenziell nicht-stationär) geeignet? Wie sollte die Signifikanz von Change Points bewertet werden — frequentistisch (p-Werte) oder Bayesianisch (Posterior-Wahrscheinlichkeit)?',
    deliverable:
      'Eine Change-Point-Erkennungspipeline, evaluiert auf Sonde-0-Daten mit bekannten Ereignissen als Grundwahrheit.',
    pipelineHook:
      'Zukünftiger CorpusExtractor für Change-Point-Erkennung; temporale Ansicht der Episteme-Säule'
  },
  'wp-005-q3': {
    disciplinaryScope: 'Für Zeitreihenanalysten und Statistiker',
    shortLabel:
      'Was ist das minimale bedeutsame Aggregationsfenster für jedes Metrik-Quellen-Paar?',
    question:
      'Unter Verwendung von Sonde-0-Daten (Tagesschau und Bundesregierung RSS) das minimale Aggregationsfenster berechnen, bei dem Sentiment-, Entity-Count- und Wortzahl-Metriken stabilisieren (Varianz des Mittelwerts unter eine Schwelle fällt).',
    deliverable:
      'Empirische Minimalfenster für jede aktuelle Metrik, mit dokumentierter statistischer Methode zur Replikation auf zukünftigen Sonden.',
    pipelineHook:
      '5-Minuten-Downsampling in GET /api/v1/metrics; resolution-Parameter; TemporalDistributionExtractor'
  },
  'wp-005-q4': {
    disciplinaryScope: 'Für Kommunikationswissenschaftler und Medienwissenschaftler',
    shortLabel: 'Wie unterscheiden sich Nachrichtenzyklen über Kulturen hinweg?',
    question:
      'Der 24-Stunden-Nachrichtenzyklus ist nicht universell. Wie unterscheiden sich Nachrichtenpublikationsrhythmen zwischen den Medienkulturen, die AĒR beobachten wird? Was sind die zentralen temporalen Strukturen (Ausgabenzyklen, Nachrichtensaisons, Aufmerksamkeits-Abklingmuster)?',
    deliverable:
      'Ein vergleichendes Medientemporalitätsprofil für die in WP-003 Q6 identifizierten Kulturregionen.',
    pipelineHook:
      'publication_hour- und publication_weekday-Metriken; TemporalDistributionExtractor; Echtzeit-Modus der Aleph-Säule'
  },
  'wp-005-q5': {
    disciplinaryScope: 'Für Kommunikationswissenschaftler und Medienwissenschaftler',
    shortLabel:
      'Wie sollte AĒR genuine Diskursverschiebungen von saisonalen und zyklischen Effekten unterscheiden?',
    question:
      'Ein Sentimenteinbruch im August kann das deutsche Sommerloch widerspiegeln, keine genuine Diskursverschiebung. Wie sollte das System zyklische Effekte von strukturellen Veränderungen unterscheiden?',
    deliverable:
      'Ein Katalog kulturspezifischer temporaler Muster (Feiertage, Mediensaisons, politische Zyklen) pro Region, formatiert als maschinenlesbare Kalendermetadaten zur Integration in das Dashboard.',
    pipelineHook:
      'TemporalDistributionExtractor; Langzeitansicht der Episteme-Säule; zukünftige Kalendermetadaten in SilverMeta'
  },
  'wp-005-q6': {
    disciplinaryScope: 'Für Computational Social Scientists',
    shortLabel: 'Bei welcher temporalen Auflösung werden Topic-Model-Outputs stabil?',
    question:
      'LDA und BERTopic sind empfindlich gegenüber der Korpusgröße. Wenn AĒR Topic Models auf täglichen, wöchentlichen und monatlichen Korpora aus denselben Daten ausführt, wie unterscheiden sich die entdeckten Topics? Bei welcher Korpusgröße stabilisieren sich Topics?',
    deliverable:
      'Eine Korpusgrößen-Sensitivitätsanalyse für LDA und BERTopic auf deutschem Nachrichtentext, mit Empfehlungen für minimale Korpusgröße pro temporalem Fenster.',
    pipelineHook:
      'CorpusExtractor-Protokoll (vorgesehen); zukünftige Korpus-Batch-Schleife der Topic-Extraktion'
  },
  'wp-005-q7': {
    disciplinaryScope: 'Für Computational Social Scientists',
    shortLabel: 'Wie sollte AĒR quellenübergreifende Propagationsdynamiken modellieren?',
    question:
      'Granger-Kausalität, Transferentropie und Kreuzkorrelation sind Standardmethoden zur Erkennung temporaler Lead-Lag-Beziehungen zwischen Zeitreihen. Welche Methode ist für Diskurspropagationsanalyse geeignet, angesichts AĒRs Datencharakteristiken?',
    deliverable:
      'Eine Methodenevaluation für quellenübergreifende Propagationserkennung, getestet auf bekannten Propagationsereignissen in Sonde-0-Daten.',
    pipelineHook:
      'Rhizome-Säule; Relational Networks view-mode domain; zukünftige quellenübergreifende Propagationsmetriken'
  },
  'wp-005-q8': {
    disciplinaryScope: 'Für Digital-Humanities-Wissenschaftler',
    shortLabel:
      'Wie sollte AĒR Foucaults epistemische Verschiebungen in temporalen Metriken operationalisieren?',
    question:
      'Die Episteme-Säule strebt an, Verschiebungen in den „Grenzen des Sagbaren" zu messen. Dies ist ein langfristiges, qualitatives Konzept. Kann es als quantitative temporale Metrik operationalisiert werden (Vokabularemergenzrate, semantische Driftgeschwindigkeit, Overton-Window-Breite), oder widersteht es fundamental der Quantifizierung?',
    deliverable:
      'Ein Positionspapier zur Operationalisierbarkeit epistemischen Wandels, mit spezifischen Vorschlägen für temporale Metriken, die das Konzept annähern, ohne es zu reduzieren.',
    pipelineHook:
      'Modus der Episteme-Säule; Langzeit-Zeitraumansicht; zukünftiger CorpusExtractor für Vokabularemergenz'
  },
  'wp-006-q1': {
    disciplinaryScope: 'Für STS-Wissenschaftler und Wissenschaftssoziologen',
    shortLabel: 'Wie sollte AĒRs Beobachtereffekt empirisch untersucht werden?',
    question:
      'Können wir ein natürliches Experiment entwerfen, das misst, ob AĒRs Veröffentlichung von Diskursmetriken die nachfolgende Diskursproduktion verändert? Zum Beispiel: Diskursmuster in Quellen messen, die sich bewusst sind, von AĒR beobachtet zu werden, vs. Quellen, die es nicht sind.',
    deliverable:
      'Ein Forschungsdesign zur empirischen Untersuchung von AĒRs Beobachtereffekt, mit Überlegungen zur ethischen Prüfung.',
    pipelineHook:
      'Reflexive Architecture (ADR-017); Oberfläche III Reflexion; bekannte Limitationen im Methodik-Tray'
  },
  'wp-006-q2': {
    disciplinaryScope: 'Für STS-Wissenschaftler und Wissenschaftssoziologen',
    shortLabel:
      'Welche Governance-Strukturen sind für ein Open-Source-Diskursmakroskop angemessen?',
    question:
      'Wie sollte AĒR Offenheit (wissenschaftliche Integrität, Reproduzierbarkeit) mit Verantwortung (Verhinderung von Instrumentalisierung, Schutz verwundbarer Gemeinschaften) ausbalancieren?',
    deliverable:
      'Ein Governance-Modell-Vorschlag, gestützt auf Präzedenzfälle anderer Dual-Use-Forschungsinstrumente (Genomdatenbanken, Klimamodelle, Wahlbeobachtungssysteme).',
    pipelineHook:
      'Silver-Layer-Eignung-Prüfprozess (WP-006 §5.2); k-Anonymitäts-Gate auf L5; Richtlinie zur verantwortungsvollen Offenlegung'
  },
  'wp-006-q3': {
    disciplinaryScope: 'Für Ethiker und politische Theoretiker',
    shortLabel: 'Unter welchen Bedingungen ist aggregierte Diskursbeobachtung ethisch zulässig?',
    question:
      'AĒR verarbeitet öffentliche Daten und identifiziert keine Individuen. Aber aggregierte Beobachtung von Gemeinschaften, Kulturen und Gesellschaften wirft ethische Fragen auf kollektiver Ebene auf, die individuumsbezogene Rahmenwerke (DSGVO, informierte Zustimmung) nicht adressieren. Welches ethische Rahmenwerk ist angemessen?',
    deliverable:
      'Ein ethisches Bewertungsrahmenwerk für kollektive Diskursbeobachtung, das die in §5.2 identifizierten Fälle adressiert (indigene Gemeinschaften, verwundbare Bevölkerungsgruppen, autoritäre Kontexte).',
    pipelineHook:
      'Silver-Layer-Eignung-Prüfung (WP-006 §5.2); ethische Prüfung der Sondenregistrierung; k-Anonymitäts-Gate'
  },
  'wp-006-q4': {
    disciplinaryScope: 'Für Ethiker und politische Theoretiker',
    shortLabel:
      'Wie sollte AĒR mit Befunden umgehen, die zur Unterdrückung von Dissens genutzt werden könnten?',
    question:
      'Wenn AĒRs Metriken offenbaren, dass ein oppositionelles Narrativ in einem bestimmten Land an Zugkraft gewinnt, könnte die Veröffentlichung dieses Befunds die Menschen hinter dem Narrativ gefährden. Sollte AĒR die Veröffentlichung verzögern? Den Zugang einschränken? Veröffentlichen, aber kontextualisieren?',
    deliverable:
      'Eine Richtlinie zur verantwortungsvollen Offenlegung politisch sensibler Diskursbefunde.',
    pipelineHook:
      'Silver-Layer-Eignung-Gate; k-Anonymitäts-Schwelle auf L5; refusal-as-feature-Architektur (ADR-017)'
  },
  'wp-006-q5': {
    disciplinaryScope: 'Für Informationsdesign- und Visualisierungsforscher',
    shortLabel: 'Wie sollte AĒR Reifikation minimieren und kritisches Engagement maximieren?',
    question:
      'Welche Visualisierungsstrategien ermutigen Nutzer, die Daten zu hinterfragen statt sie unkritisch zu akzeptieren? Wie können Unsicherheit, Provisorietät und kultureller Kontext visuell kommuniziert werden, ohne den Nutzer zu überfordern?',
    deliverable:
      'Dashboard-Designprinzipien und Prototyp-Wireframes, die das Prinzip der nicht-präskriptiven Visualisierung (§6.2) implementieren, evaluiert mit Nutzerstudien.',
    pipelineHook:
      'Prinzip der nicht-präskriptiven Visualisierung (Design Brief §7.5); epistemic weight (§7.8); Progressive Semantics'
  },
  'wp-006-q6': {
    disciplinaryScope: 'Für Informationsdesign- und Visualisierungsforscher',
    shortLabel: 'Wie sollte AĒR visuell darstellen, was es nicht beobachten kann?',
    question:
      'Der Digital Divide (Manifest §II), die Plattform-Unzugänglichkeit (WP-003) und die demografische Verzerrung sind alles Formen systematischer Abwesenheit. Wie sollte ein Dashboard Abwesenheit sichtbar machen — nicht nur zeigen, was das Makroskop sieht, sondern auch, wofür es blind ist?',
    deliverable:
      'Visualisierungskonzepte für „Negativraum“ — die Darstellung von Beobachtungslimitierungen als integraler Bestandteil des Dashboards.',
    pipelineHook:
      'Negative Space overlay (Phase 113); Abwesenheitsregionen auf dem Oberfläche-I-Globus; Negative Space mode im Methodik-Tray'
  },
  'wp-006-q7': {
    disciplinaryScope: 'Für digitale Anthropologen und Area-Studies-Wissenschaftler',
    shortLabel:
      'Welche Beobachtereffekte sind für spezifische kulturelle Kontexte beim Veröffentlichen von AĒRs Metriken wahrscheinlich?',
    question:
      'In jeder Kulturregion (WP-003 Q6, WP-004 Q6): Wie würde die öffentliche Sichtbarkeit von Diskursmetriken die Diskursproduktion beeinflussen? Gibt es Kontexte, in denen Veröffentlichung vorteilhaft wäre (Transparenz, demokratische Rechenschaftspflicht) und Kontexte, in denen sie schädlich wäre (Ermöglichung von Repression, Verstärkung von Manipulation)?',
    deliverable:
      'Pro-Region-Beobachtereffekt-Bewertungen (1–2 Seiten jeweils), die AĒRs ethischen Prüfprozess für neue Sonden informieren.',
    pipelineHook:
      'ethische Prüfung der Sondenregistrierung; Silver-Layer-Eignung (WP-006 §5.2); Richtlinie zur verantwortungsvollen Offenlegung'
  }
};
