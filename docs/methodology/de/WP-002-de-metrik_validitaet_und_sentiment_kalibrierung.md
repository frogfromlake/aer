# WP-002: Metrik-Validität und Sentiment-Kalibrierung

> **Reihe:** AĒR Wissenschaftliche Methodik — Arbeitspapiere
> **Status:** Entwurf — offen für interdisziplinäre Begutachtung
> **Datum:** 07.04.2026
> **Abhängig von:** WP-001 (Funktionale Sondentaxonomie), Arc42 Kapitel 13 (Wissenschaftliche Grundlagen)
> **Architekturkontext:** Analysis Worker Extractor Pipeline (§8.10), Tier-1–3-Metrikklassifikation (§13.3)
> **Lizenz:** [CC BY 4.0](https://creativecommons.org/licenses/by/4.0/) — © 2026 Fabian Quist

---

## 1. Zielsetzung

Dieses Arbeitspapier adressiert eine grundlegende Frage für das AĒR-Projekt: **Unter welchen Bedingungen können computationale Textmetriken als valide Stellvertreter für gesellschaftliche Einstellungen dienen, und welche Kalibrierung ist erforderlich, bevor sie über Sprachen, Kulturen und Medientypen hinweg sinnvoll verglichen werden können?**

AĒRs Engineering-Pipeline (Phasen 41–46) hat eine funktionsfähige Extractor-Architektur mit provisorischen Proof-of-Concept-Implementierungen für Sentimentbewertung, Named Entity Recognition, Spracherkennung und temporale Verteilung etabliert. Diese Extractors validieren die technische Korrektheit der Pipeline — sie validieren nicht die *wissenschaftliche Bedeutung* der Zahlen, die sie produzieren. Ein Sentimentwert von `0.34`, extrahiert aus einer Tagesschau-RSS-Beschreibung mittels SentiWS, sagt uns, dass bestimmte Wörter im Text positive Polaritätsgewichte in einem Lexikon der Universität Leipzig tragen. Er sagt uns nicht, ob der Artikel eine positive Einstellung ausdrückt, ob der Journalist Optimismus vermitteln wollte, oder ob ein Leser in München, Nairobi oder São Paulo den Text als positiv wahrnehmen würde.

Dieses Papier kartiert die Lücke zwischen computationalem Output und soziologischer Interpretation, identifiziert die spezifischen wissenschaftlichen Fragen, die durch interdisziplinäre Zusammenarbeit beantwortet werden müssen, und schlägt ein Validierungsrahmenwerk vor, das mit AĒRs architektonischen Constraints kompatibel ist: Determinismus, Transparenz und Ockhams Rasiermesser.

---

## 2. Das Validitätsproblem: Von Zahlen zu Bedeutung

### 2.1 Konstruktvalidität in der Computational Social Science

Die zentrale epistemologische Herausforderung ist eine der **Konstruktvalidität**: Misst die computationale Metrik das theoretische Konstrukt, das sie zu repräsentieren beansprucht? Dieses Problem ist in der Surveymethodologie wohlbekannt (Cronbach & Meehl, 1955), nimmt aber in der computationalen Textanalyse eine eigenständige Form an.

In der traditionellen Sozialwissenschaft entwirft ein Forscher ein Erhebungsinstrument, wendet es auf eine Stichprobenpopulation an und validiert das Instrument gegen externe Kriterien. Der Forscher kontrolliert den Messprozess. In der computationalen Diskursanalyse ist das „Instrument" ein Algorithmus, der auf Text angewendet wird, der zu Zwecken produziert wurde, die mit der Messung in keinerlei Zusammenhang stehen. Der Text ist keine Antwort auf einen strukturierten Prompt — er ist ein kulturelles Artefakt, eingebettet in einen spezifischen kommunikativen Kontext (Redaktionspolitik, Plattformaffordanzen, Publikumserwartungen, Genrekonventionen).

Grimmer, Roberts & Stewart (2022) formalisieren diese Spannung: Computationale Textanalysemethoden sind mächtige Werkzeuge für *Entdeckung* und *Messung*, aber sie erfordern explizite Validierung gegen menschliches Urteil für jede neue Anwendungsdomäne. Es gibt keinen universellen Sentimentalgorithmus, der über alle Texttypen, Sprachen und kulturellen Kontexte hinweg „funktioniert". Validität ist stets *kontextuell*.

**Implikation für AĒR:** Jeder Metrik-Extractor muss nicht einmalig validiert werden, sondern für jede Kombination aus:

- **Sprache** (deutsche Leitartikelprosa ≠ arabischer Twitter-Diskurs ≠ japanische Blogkultur)
- **Quellentyp** (RSS-Feed-Beschreibungen ≠ vollständige Artikel ≠ Social-Media-Posts ≠ Forenthreads)
- **Diskursfunktion** (WP-001-Taxonomie: epistemische Autorität ≠ Gegendiskurs ≠ Identitätsbildung)
- **Temporaler Kontext** (Krisenzeiträume können Sentiment-Baseline-Verteilungen verschieben)

### 2.2 Der Aggregationstrugschluss

AĒRs Gold-Schicht speichert Metriken als Zeitreihen-Datenpunkte, aggregiert über Dokumente und Quellen. Diese Aggregation ist das Kernwertversprechen des Systems — sie ermöglicht makroskopische Beobachtung. Doch Aggregation führt ein kritisches Risiko ein: den **ökologischen Inferenzfehlschluss** (Robinson, 1950). Muster, die auf der Aggregatebene beobachtet werden (z. B. „das durchschnittliche Sentiment gegenüber Immigration sank in Q3 2026"), spiegeln möglicherweise nicht die Muster auf Einzeldokumentebene wider. Sie können stattdessen Veränderungen in der *Zusammensetzung* des Korpus widerspiegeln (z. B. eine neue Quelle mit systematisch niedrigerem Sentiment wurde hinzugefügt, oder eine hochvolumige Quelle erhöhte ihre Publikationsfrequenz).

Dies ist nicht lediglich ein statistisches Problem — es ist ein soziologisches. Wenn AĒRs Dashboard einen Rückgang des „positiven Diskurses" in einer bestimmten Region zeigt, liegt das daran, dass:

1. Einzelne Autoren negativere Ansichten äußern? (Genuine Einstellungsverschiebung)
2. Der Empfehlungsalgorithmus der Plattform negativere Inhalte verstärkt? (Algorithmisches Artefakt)
3. Eine neue, strukturell negative Quelle (z. B. investigativer Journalismus) zum Sondenset hinzugefügt wurde? (Stichprobenartefakt)
4. Das Sentimentlexikon die in dieser Region verwendete Sprachvarietät systematisch zu niedrig bewertet? (Messartefakt)

Ohne Validierung sind diese Erklärungen in der aggregierten Metrik ununterscheidbar. AĒRs Progressive-Disclosure-Prinzip (ADR-003) — die Fähigkeit, von Gold-Aggregaten bis zu einzelnen Bronze-Dokumenten hinunterzugehen — ist die architektonische Schutzmaßnahme. Doch Progressive Disclosure ermöglicht *menschliche* Disambiguierung; es *automatisiert* sie nicht. Die wissenschaftliche Frage ist, wie Metriken und Metadaten so gestaltet werden, dass diese Disambiguierung handhabbar wird.

---

## 3. Sentimentanalyse: Stand der Forschung und Limitierungen

### 3.1 AĒRs aktuelle Implementierung (Provisorischer PoC)

Der `SentimentExtractor` (Phase 42) verwendet SentiWS v2.0, ein deutschsprachiges Wort-Polaritäts-Lexikon, veröffentlicht von der Universität Leipzig (Remus, Quasthoff & Heyer, 2010). Der Algorithmus ist bewusst minimal:

1. Tokenisiere `cleaned_text` nach Whitespace.
2. Kleinschreibung jedes Tokens und Entfernung von Interpunktion an den Grenzen.
3. Nachschlagen jedes Tokens im SentiWS-Lexikon (~3.500 Grundformen + Flexionen).
4. Score = arithmetisches Mittel der übereinstimmenden Token-Polaritäten.
5. Begrenzung auf [-1,0; 1,0].

Dieser Ansatz wurde gewählt, weil er vollständig deterministisch, auditierbar (jeder Score ist auf einzelne Wortübereinstimmungen zurückverfolgbar) und ohne externe API-Aufrufe oder Modellinferenz auskommt. Er erfüllt AĒRs Tier-1-Kriterien (§13.3.1). Er ist gleichzeitig, gemessen an jedem Standard moderner NLP, stark limitiert.

### 3.2 Bekannte Limitierungen lexikonbasierter Sentimentanalyse

Die folgenden Limitierungen sind im Quellcode des Extractors und in §13.3.1 dokumentiert. Sie werden hier im wissenschaftlichen Kontext rekapituliert:

**Negationsblindheit.** Der Satz *„Das ist nicht gut"* erhält einen positiven Score, weil *„gut"* ein positives Polaritätsgewicht trägt und die Negationspartikel *„nicht"* nicht im Lexikon enthalten ist. Negationsbehandlung ist ein gelöstes Problem in regelbasierter NLP (z. B. NegEx, Wiegand et al., 2010), aber jede regelbasierte Negationsskopdetektion muss pro Sprache validiert werden. Die deutsche Satzstruktur mit ihren verbfinalen Nebensätzen erzeugt Negationsskope, die sich fundamental vom Englischen unterscheiden.

**Kompositionalität und Komposita.** Deutsch ist eine agglutinative Sprache. Das Kompositum *„Klimaschutzpaket"* ist ein einzelnes orthographisches Token, das latentes Sentiment durch seine Bestandteile trägt — aber das Kompositum selbst ist in keinem Sentimentlexikon enthalten. Dekompositierungsstrategien (empirisches Kompositasplitting, z. B. Koehn & Knight, 2003) führen eigene Fehlerquellen ein. Andere agglutinative Sprachen — Türkisch, Finnisch, Ungarisch, Koreanisch, Japanisch — stellen analoge Herausforderungen mit je eigenen morphologischen Regeln dar.

**Ironie und Sarkasmus.** Lexikonbasierte Methoden sind strukturell unfähig, Ironie zu erkennen, bei der die Oberflächenpolarität invertiert ist. Ironieerkennung bleibt ein offenes Forschungsproblem selbst für transformerbasierte Modelle (Van Hee et al., 2018), und Ironiekonventionen sind tief kulturspezifisch. Was sich im britischen Englisch als beißender Sarkasmus liest, kann in vielen ostasiatischen Kommunikationskontexten wörtlich interpretiert werden, wo indirekte Kommunikation anderen pragmatischen Konventionen folgt (Hall, 1976; Hofstede, 2001).

**Domänenabhängigkeit.** Das Wort *„Krise"* trägt in SentiWS ein negatives Polaritätsgewicht. Im Wirtschaftsjournalismus ist *„Krise"* ein deskriptiver Begriff ohne inhärente evaluative Absicht — er beschreibt einen Zustand. In politischen Kommentaren kann dasselbe Wort starken negativen Affekt tragen. Lexikonscores sind kontextblind; dasselbe Wort erhält dasselbe Gewicht ungeachtet von Genre, Register oder kommunikativer Absicht.

**Lexikonabdeckung und Aktualität.** SentiWS v2.0 wurde 2010 veröffentlicht. Die deutsche Sprache hat sich weiterentwickelt — neue evaluative Begriffe sind in den Diskurs eingetreten (*„Wutbürger"*, *„Querdenker"*, *„toxisch"* im metaphorischen sozialen Sinne), und bestehende Begriffe haben ihre Polarität verschoben (*„liberal"* trägt im deutschen vs. amerikanisch-englischen Diskurs unterschiedliche Konnotationen). Ein fixiertes Lexikon ist eine Momentaufnahme evaluativer Sprache zu einem Zeitpunkt, kein lebendes Instrument.

### 3.3 Jenseits von Lexika: Das Spektrum der Sentimentmethoden

Die Forschungslandschaft bietet ein Spektrum von Ansätzen mit unterschiedlichen Abwägungen gegenüber AĒRs architektonischen Prinzipien:

**Regelbasierte Systeme (VADER, TextBlob, SentiStrength).** Diese erweitern lexikonbasierte Ansätze um heuristische Regeln für Negation, Intensivierung und Interpunktion. VADER (Hutto & Gilbert, 2014) wurde auf englischsprachigen Social-Media-Texten kalibriert und funktioniert schlecht bei formaler Leitartikelprosa und nicht-englischen Sprachen. SentiStrength (Thelwall et al., 2010) unterstützt mehrere Sprachen, wurde aber für kurze informelle Texte entwickelt. Keines dieser Systeme wurde systematisch an deutschen institutionellen RSS-Inhalten validiert.

**Überwachtes maschinelles Lernen (SVM, Naive Bayes mit TF-IDF-Features).** Diese erfordern gelabelte Trainingsdaten — einen Korpus von Texten, die von menschlichen Kodierern bezüglich Sentiment annotiert wurden. Der Annotationsprozess selbst ist kulturell situiert: Was als „positiv" oder „negativ" gilt, hängt vom kulturellen Hintergrund, der sprachlichen Kompetenz und dem Interpretationsrahmen der Annotatoren ab. Die Intercoder-Übereinstimmung bei Sentimentbewertung ist für nuancierte Texte notorisch niedrig (Rosenthal et al., 2017). Für AĒR führt dieser Ansatz eine zirkuläre Abhängigkeit ein: Wir benötigen validierte Sentimentlabels zum Trainieren des Modells, aber die Generierung dieser Labels *ist* das Validierungsproblem.

**Vortrainierte Transformer-Modelle (BERT, XLM-RoBERTa, sprachspezifische Varianten).** Deutschspezifische Modelle wie *german-sentiment-bert* (Guhr et al., 2020) erreichen State-of-the-Art-Performance auf Benchmark-Datensätzen. Allerdings verletzen sie AĒRs Tier-1-Determinismusanforderung — Transformer-Inferenz ist empfindlich gegenüber Gleitkomma-Präzision, Hardware und Bibliotheksversionen. Sie sind opak (die interne Logik des Modells ist nicht auditierbar), und sie betten Trainingsbiases ein, die schwer zu charakterisieren sind. Unter AĒRs Tiersystem würde transformerbasiertes Sentiment als Tier 2 (reproduzierbar mit fixierter Modellversion und fixiertem Seed) oder Tier 3 (nicht-deterministisch) klassifiziert.

**LLM-basiertes Sentiment (GPT-4, Claude, Open-Source-LLMs).** Die leistungsfähigste, aber am wenigsten transparente Option. LLM-Outputs sind nicht-deterministisch, nicht-reproduzierbar und in großem Maßstab wirtschaftlich kostspielig. Unter AĒRs Rahmenwerk ist LLM-abgeleitetes Sentiment strikt Tier 3 — nur als explorative Ergänzung zulässig, niemals als primäre Metrik (§13.3.3).

### 3.4 Die interkulturelle Sentiment-Herausforderung

Sentiment ist kein universelles Konstrukt. Die Annahme, dass „positiv" und „negativ" kulturell invariante emotionale Pole sind, ist ein westliches psychologisches Framework (Russell, 1980), das sich nicht sauber auf alle menschlichen Affektsysteme abbilden lässt.

**Linguistische Relativität des Affekts.** Sprachen kodieren Affekt unterschiedlich. Das japanische Konzept *amae* (甘え) — ein angenehmes Gefühl der Abhängigkeit von einer anderen Person — hat kein englisches Äquivalent und keine Polaritätszuordnung in irgendeinem westlichen Sentimentlexikon. Die deutsche *Schadenfreude* ist lexikalisch negativ (sie enthält *Schaden*), aber psychologisch komplex. Das arabische emotionale Vokabular unterscheidet Zustände des Herzens (*qalb*), die sich nicht auf die Positiv-negativ-Achse abbilden lassen. Ein Sentimentsystem, das allen Affekt auf einen einzigen Skalarwert [-1,0; 1,0] reduziert, zwingt ein dimensionales Emotionsmodell auf, das kulturspezifisch ist.

**Pragmatische Konventionen.** Dieselbe Äußerung trägt je nach Kommunikationsnormen unterschiedliches Sentiment. In vielen ostasiatischen Medienkulturen sind Zurückhaltung und Untertreibung das Standardregister — eine milde positive Aussage kann starke Zustimmung repräsentieren. In der amerikanischen Medienkultur ist hyperbolische Positivität die Grundlinie — „amazing", „incredible", „game-changing" sind registerneutral im Technologiejournalismus. Ein interkultureller Sentimentvergleich muss diese **Baseline-Kalibrierungsunterschiede** in expressiven Konventionen berücksichtigen.

**Politisches und institutionelles Register.** Regierungspressemitteilungen — das Kernmaterial von AĒRs Sonde 0 — verwenden über Kulturen hinweg ein bewusst neutrales institutionelles Register. Die *Bundesregierung* kommuniziert im Beamtendeutsch; das Weiße Haus verwendet diplomatische Prosa; der chinesische Staatsrat verwendet ein eigenes offizielles Register (官方语言). Diese Register sind darauf ausgelegt, offenes Sentiment zu unterdrücken. Ein lexikonbasierter Score von `0,0` bei einer Regierungspressemitteilung bedeutet nicht „neutrale Einstellung" — er bedeutet „institutionelles Register funktioniert wie vorgesehen". Diese Unterscheidung ist kritisch für den interkulturellen Vergleich.

**Code-Switching und mehrsprachiger Diskurs.** In vielen digitalen Räumen — insbesondere in postkolonialen Kontexten (Indien, Philippinen, Nigeria, Nordafrika) — wechseln Nutzer routinemäßig innerhalb eines einzelnen Textes zwischen Sprachen. Ein Tweet, der Hindi und Englisch mischt (*Hinglish*), oder ein Facebook-Post, der Französisch und Wolof mischt, kann von keinem einsprachigen Sentimentsystem sinnvoll bewertet werden. AĒRs aktueller Spracherkennungs-Extractor weist jedem Dokument eine einzelne Primärsprache zu — dies ist für code-geswitchte Texte unzureichend.

---

## 4. Named Entity Recognition: Über die Extraktion hinaus

### 4.1 AĒRs aktuelle Implementierung

Der `NamedEntityExtractor` (Phase 42) verwendet spaCys `de_core_news_lg`-Modell (v3.8.0) mit allen Pipeline-Komponenten deaktiviert außer NER. Er extrahiert Entity-Spans, klassifiziert als PER (Person), ORG (Organisation), LOC (Ort) und MISC (Sonstiges). Rohe Spans werden in `aer_gold.entities` gespeichert; ein aggregierter `entity_count` wird als Metrik gespeichert.

### 4.2 Das Entity-Linking-Problem

Rohe Entity-Extraktion ohne **Entity Linking** (auch Entity-Disambiguierung oder Entity-Resolution genannt) produziert Daten, die schwer sinnvoll zu aggregieren sind. Die Zeichenkette *„Merkel"* in einem Dokument und *„Angela Merkel"* in einem anderen werden als separate Entitäten gespeichert. *„CDU"*, *„die Union"* und *„Christdemokraten"* beziehen sich auf dieselbe politische Entität, sind aber nicht verknüpft. Diese Fragmentierung wird bei Skalierung katastrophal: Ein Dashboard, das „Top-Entitäten" zeigt, würde dieselbe reale Entität mehrfach unter verschiedenen Oberflächenformen auflisten.

Entity Linking erfordert eine **Wissensbasis** (z. B. Wikidata, DBpedia), die Oberflächenformen auf kanonische Identifikatoren abbildet. Die Wahl der Wissensbasis ist selbst kulturell signifikant: Wikidatas Ontologie spiegelt die redaktionellen Entscheidungen ihrer Beitragenden-Community wider, die in Richtung westlicher, englischsprachiger Perspektiven verzerrt ist. Eine Entität, die im brasilianischen politischen Diskurs hochrelevant ist, kann einen spärlichen oder fehlenden Wikidata-Eintrag haben. Die Wissensbasis ist kein neutraler Boden — sie ist ein kulturelles Artefakt.

### 4.3 Interkulturelle Entity-Herausforderungen

**Namenskonventionen.** NER-Modelle, die auf westeuropäischem Text trainiert wurden, erwarten die Reihenfolge Vorname-Nachname. Ostasiatische Namenskonventionen (Nachname zuerst), patronymische Systeme (isländisch, arabisch, russisch), mononymische Namen (verbreitet in der indonesischen Kultur) und in Honorifika eingebettete Namen (japanisches Keigo) verletzen alle diese Erwartungen. Ein spaCy-Modell, das auf deutschen Nachrichtentexten trainiert wurde, wird *„習近平"* oder *„محمد بن سلمان"* nicht korrekt parsen.

**Organisationsgrenzen.** Das Konzept der „Organisation" variiert kulturell. Im japanischen *Keiretsu*-System oder der koreanischen *Chaebol*-Struktur ist die Grenze zwischen eigenständigen Entitäten verwischt. In vielen afrikanischen Kontexten fungieren Stammes- oder Clanstrukturen als organisationale Entitäten, werden aber von NER-Modellen nicht erkannt, die auf westlichen Unternehmens- und Regierungsontologien trainiert wurden. WP-001s Funktionale Sondentaxonomie adressiert dies explizit über die emische Schicht — aber NER-Modelle operieren auf der etischen Schicht und müssen entsprechend ausgewählt oder trainiert werden.

**Geopolitische Sensibilität.** Die Extraktion von Ortsentitäten beinhaltet implizite geopolitische Annahmen. Ob *„Taiwan"* als Land oder Region getaggt wird, ob *„Palästina"* ein LOC- oder GPE-Label erhält, ob *„Kaschmir"* Indien oder Pakistan zugeordnet wird — dies sind keine technischen Entscheidungen. Es sind politische Positionen, eingebettet in die Trainingsdaten des NER-Modells. AĒR muss diese eingebetteten Annahmen dokumentieren und, wo möglich, geopolitische Klassifikation an die Analystenebene (Progressive Disclosure) delegieren, statt sie in den Extractor einzubauen.

---

## 5. Spracherkennung: Die Illusion der Einsprachigkeit

### 5.1 AĒRs aktuelle Implementierung

Der `LanguageDetectionExtractor` (Phase 42, erweitert in Phase 45) verwendet `langdetect` mit einem fixierten Random Seed. Er weist jedem Dokument einen primären Sprachcode und einen Konfidenzwert (0,0–1,0) zu. Rangierte Kandidaten werden in `aer_gold.language_detections` persistiert.

### 5.2 Limitierungen in einer mehrsprachigen Welt

**Kurztext-Degradation.** RSS-Feed-Beschreibungen umfassen typischerweise 50–200 Zeichen. Die Genauigkeit der Spracherkennung degradiert unterhalb von 100 Zeichen signifikant (Jauhiainen et al., 2017). Eine deutsche Überschrift mit einem englischen Lehnwort (*„Homeoffice-Pflicht"*) oder einem französischen Begriff (*„Ménage-à-trois der Parteien"*) kann eine mehrdeutige oder falsche Sprachklassifikation produzieren.

**Dialekt- und Varietätenerkennung.** `langdetect` unterscheidet zwischen Sprachcodes (z. B. `de`, `en`, `fr`), aber nicht zwischen Sprachvarietäten. Formales Standarddeutsch (*Hochdeutsch*), Schweizerdeutsch und österreichisches Deutsch werden identisch klassifiziert. Dies kollabiert bedeutsame soziolinguistische Variation. Für ein System, das globalen Diskurs zu beobachten beabsichtigt, ist die Unterscheidung zwischen vereinfachtem und traditionellem Chinesisch, zwischen brasilianischem und europäischem Portugiesisch, zwischen lateinamerikanischem und kastilischem Spanisch analytisch relevant.

**Schrifterkennung vs. Spracherkennung.** Sprachen, die eine Schrift teilen (z. B. Serbisch in lateinischer vs. kyrillischer Schrift, Bosnisch/Kroatisch/Serbisch als plurizentrische Sprache) stellen Erkennungsherausforderungen dar, die nicht auf statistische Zeichen-N-Gramm-Modelle reduzierbar sind. Die Wahl des Schriftsystems ist selbst eine kulturelle und politische Aussage.

---

## 6. Auf dem Weg zu einem Validierungsrahmenwerk

### 6.1 Designprinzipien

Jedes Validierungsrahmenwerk für AĒRs Metriken muss die architektonischen Constraints des Projekts respektieren:

1. **Transparenz vor Performance.** Eine validierte Metrik mit bekannten Limitierungen und dokumentierten Fehlermodi ist einer hochperformanten Metrik vorzuziehen, deren Fehlercharakteristiken opak sind. AĒR ist ein Makroskop, kein Mikroskop — es opfert Einzeldokument-Präzision zugunsten von Beobachtbarkeit auf Aggregatebene, aber es muss *wissen*, was es opfert.

2. **Provenienz als erstklassiges Datum.** Jede Metrik muss ihre eigene Provenienz tragen: die Algorithmusversion, den Lexikon-/Modell-Hash, die verwendeten Parameter und einen Konfidenzqualifikator. Dies ist bereits architektonisch antizipiert im `SilverEnvelope.extraction_provenance`-Feld (Phase 46) und der `version_hash`-Eigenschaft jedes Extractors.

3. **Validierung ist kontextgebunden, nicht universell.** Eine für deutsches redaktionelles RSS validierte Sentimentmethode kann nicht als valide für arabische soziale Medien oder japanische Blogposts angenommen werden. Validierungsergebnisse sind an die spezifische Kombination aus Sprache, Quellentyp und Diskursfunktion (WP-001-Taxonomie) gebunden.

4. **Menschliches Urteil als Grundwahrheit.** Für subjektive Konstrukte wie Sentiment und Haltung gibt es keine externe Grundwahrheit — nur menschliches Urteil. Validierung erfordert Annotationsstudien mit kulturell kompetenten Kodierern, gemäß etablierten Intercoder-Reliabilitätsprotokollen (Krippendorffs Alpha ≥ 0,667 als Minimalschwelle, Krippendorff, 2004).

### 6.2 Vorgeschlagenes Validierungsprotokoll

Für jedes Metrik-Kontext-Paar (z. B. „SentiWS-Sentiment auf deutschem institutionellem RSS") wird das folgende Protokoll vorgeschlagen:

**Schritt 1 — Annotationsstudie.** Stichprobenziehung von *n* Dokumenten aus der Sonde (stratifiziert nach Quelle, temporaler Verteilung und Thema). Rekrutierung von ≥ 3 Annotatoren mit muttersprachlicher Kompetenz und Domänenwissen. Definition eines klaren Annotationsschemas (z. B. 5-Punkte-Likert-Skala für Sentiment oder kategoriale Labels). Berechnung der Intercoder-Reliabilität (Krippendorffs Alpha). Bei Alpha < 0,667 ist das Annotationsschema zu mehrdeutig — Überarbeitung vor dem Fortfahren.

**Schritt 2 — Baseline-Vergleich.** Anwendung der computationalen Metrik auf die annotierte Stichprobe. Berechnung der Korrelation (Pearson oder Spearman) zwischen algorithmischen Scores und gemittelten menschlichen Urteilen. Bericht von Precision, Recall und F1 für kategoriale Klassifikationen. Dokumentation systematischer Fehlermuster (z. B. „SentiWS überbewertet negationshaltige Sätze im Durchschnitt um 0,3").

**Schritt 3 — Fehlertaxonomie.** Klassifikation von Abweichungen zwischen menschlichem und algorithmischem Urteil in eine strukturierte Taxonomie: Negationsfehler, Ironiefehler, domänenspezifischer Begriffsfehler, Kompositionalitätsfehler, kultureller Registerfehler. Diese Taxonomie wird Teil der Metrikdokumentation und informiert das Design verbesserter Extractors.

**Schritt 4 — Kontexttransfertest.** Anwendung der Metrik auf einen strukturell anderen Kontext (z. B. von deutschem RSS zu deutschem Twitter, oder von deutschem RSS zu französischem RSS). Messung der Performance-Degradation. Dokumentation der *Transfergrenze* — des Punktes, an dem die Validität der Metrik zusammenbricht.

**Schritt 5 — Longitudinaler Stabilitätstest.** Anwendung der Metrik auf eine Zeitreihenstichprobe über ≥ 6 Monate. Prüfung auf temporale Drift: Verändert sich die Beziehung der Metrik zum menschlichen Urteil über die Zeit? Wenn ja, wird die Drift durch Sprachwandel (neue Begriffe im Lexikon), Quellenverhalten (Redaktionspolitikänderungen) oder Metrikverfall (veralternde Trainingsdaten des Modells) verursacht?

### 6.3 Architektonische Integration

Validierungsergebnisse sollten als **Metrik-Metadaten** in der Gold-Schicht gespeichert werden, die der BFF API ermöglichen, Konfidenzqualifikatoren neben rohen Metrikwerten zu exponieren. Eine vorgeschlagene Schemaerweiterung:

```
aer_gold.metric_validity (
    metric_name       String,
    context_key       String,    -- z. B. "de:rss:epistemic_authority"
    validation_date   DateTime,
    alpha_score       Float32,   -- Intercoder-Reliabilität
    correlation       Float32,   -- Mensch-Algorithmus-Korrelation
    n_annotated       UInt32,
    error_taxonomy    String,    -- JSON-Blob: Fehlertyp → Häufigkeit
    valid_until       DateTime   -- Verfallsdatum des Validitätsanspruchs
)
```

Diese Tabelle ermöglicht nachgelagerten Konsumenten (Dashboards, Forschungs-APIs) die Unterscheidung zwischen validierten und nicht-validierten Metriken — eine kritische Transparenzanforderung für wissenschaftliche Nutzung.

---

## 7. Offene Fragen für interdisziplinäre Kooperationspartner

Die folgenden Fragen sind als konkrete Forschungsprobleme formuliert, die Expertise jenseits des Software Engineering erfordern. Jede Frage identifiziert die relevante Disziplin, die spezifische Lücke in AĒRs aktueller Implementierung und die wertvollste Form des Beitrags.

### 7.1 Für Computational Social Scientists

**F1: Welche Sentimentmethode(n) sind für deutschen redaktionellen Text aus institutionellen RSS-Feeds geeignet, unter Berücksichtigung von AĒRs Constraints für Determinismus und Transparenz?**

- AĒRs aktuelle Methode (SentiWS-Lexikon, mittlere Polarität) ist eine Tier-1-Baseline. Gibt es eine validierte, deterministische Alternative, die Negation und Kompositionalität handhabt und dabei auditierbar bleibt?
- Falls die Antwort „keine deterministische Methode ist adäquat" lautet, wie sollte AĒR eine Tier-2-Methode (z. B. einen feinabgestimmten Klassifikator mit fixierter Modellversion) integrieren und dabei Nachvollziehbarkeit bewahren?
- Deliverable: Eine Empfehlung mit Validierungsevidenz auf einem vergleichbaren Korpus.

**F2: Welches Annotationsschema sollte AĒR für Sentiment-Validierungsstudien verwenden?**

- Ist eine eindimensionale Positiv-Negativ-Skala angemessen, oder benötigt AĒR ein mehrdimensionales Affektmodell (Valenz + Erregung, oder diskrete Emotionen)?
- Wie sollten Annotatoren mit institutionellem Register umgehen (Pressemitteilungen, die bewusst neutral sind)? Ist „neutral" die Abwesenheit von Sentiment, oder ist es eine eigenständige kommunikative Haltung?
- Deliverable: Ein Annotationscodebuch mit Arbeitsbeispielen aus deutschen RSS-Beschreibungen.

**F3: Wie sollte AĒR Sentimentscores für quellenübergreifende Vergleiche normalisieren?**

- Pressemitteilungen der Bundesregierung und Tagesschau-Artikel haben strukturell unterschiedliche Sentiment-Baselines. Wie vergleichen wir Sentiment-*Veränderung* über Quellen hinweg, wenn die absoluten Niveaus nicht vergleichbar sind?
- Deliverable: Eine Normalisierungsstrategie (Z-Score pro Quelle, Perzentil-Ranking, Differenz-in-Differenzen) mit statistischer Begründung.

### 7.2 Für Computerlinguisten / NLP-Forscher

**F4: Wie sollte AĒR deutsche Komposita in der lexikonbasierten Sentimentanalyse behandeln?**

- Sollten Komposita vor dem Lexikon-Lookup dekomponiert werden? Wenn ja, welche Dekompositionsstrategie ist angemessen (frequenzbasiert, regelbasiert, neural)?
- Wie erstreckt sich dies auf andere agglutinative Sprachen (Türkisch, Finnisch, Koreanisch), wenn AĒR über das Deutsche hinaus expandiert?
- Deliverable: Eine Evaluation von Komposita-Dekompositionsstrategien auf einem deutschen Nachrichtenkorpus mit Auswirkung auf die Sentiment-Scoring-Genauigkeit.

**F5: Welche NER-Modelle und Entity-Linking-Strategien sind für AĒRs mehrsprachigen, multisource Kontext geeignet?**

- Das aktuelle spaCy `de_core_news_lg`-Modell ist einsprachig. AĒR wird Arabisch, Mandarin, Hindi, Portugiesisch, Spanisch, Französisch und weitere Sprachen verarbeiten müssen. Ist ein einzelnes mehrsprachiges Modell (XLM-RoBERTa) pro-Sprache-Modellen vorzuziehen?
- Wie sollte Entity Linking mit Entitäten umgehen, die in nicht-westlichen Kontexten hochrelevant, aber in Wikidata unterrepräsentiert sind?
- Deliverable: Ein Benchmark-Vergleich von NER-/Entity-Linking-Ansätzen auf einem mehrsprachigen Nachrichtenkorpus, evaluiert pro Sprache und Entity-Typ.

**F6: Wie sollte AĒR code-geswitchte Texte erkennen und behandeln?**

- In mehrsprachigen digitalen Räumen mischen Dokumente häufig Sprachen. Wie sollten Spracherkennung, Sentimentbewertung und Entity-Extraktion auf code-geswitchtem Text operieren?
- Deliverable: Ein Survey über Code-Switching-Erkennungsmethoden mit einer Empfehlung für AĒRs pro-Dokument-Verarbeitungsmodell.

### 7.3 Für Kulturanthropologen und Area-Studies-Wissenschaftler

**F7: Wie sollte AĒR Sentiment-Baselines über Kulturen mit unterschiedlichen expressiven Normen kalibrieren?**

- Wenn japanische redaktionelle Prosa systematisch zurückhaltender ist als amerikanischer Journalismus, erfordert ein interkultureller Sentimentvergleich eine Baseline-Normalisierung. Aber die Baseline ist nicht lediglich ein statistisches Artefakt — sie *ist* das kulturelle Phänomen, das AĒR zu beobachten beabsichtigt (epistemische Grenzen, gemäß dem Episteme-Pfeiler des Manifests).
- Ist Baseline-Normalisierung ein Akt kultureller Auslöschung? Sollte AĒR Roh-Baselines bewahren und die *Differenz* als Befund visualisieren?
- Deliverable: Ein Positionspapier zur epistemologischen Spannung zwischen Normalisierung und kultureller Beobachtung.

**F8: Wie sollte AĒRs Entity-Ontologie kulturspezifische Organisationsformen behandeln?**

- WP-001 definiert ein Duales Tagging-System (etische/emische Schichten). Wie sollte dies auf die Entity-Klassifikation erweitert werden? Sollte ein japanisches *Keiretsu* in der etischen Schicht als ORG getaggt werden und dabei seine emische Spezifität bewahren?
- Welche kulturspezifischen Entity-Typen werden von westlichen NER-Modellen systematisch übersehen, und wie sollte AĒRs Entity-Taxonomie diese berücksichtigen?
- Deliverable: Eine interkulturelle Entity-Taxonomie, die spaCys Standardlabels um kulturell informierte Kategorien erweitert, abgebildet auf WP-001s emische Schicht.

### 7.4 Für Methodologen und Statistiker

**F9: Wie sollte AĒRs Aggregationspipeline das ökologische Inferenzproblem berücksichtigen?**

- Wenn Sentimentmetriken über Dokumente, Quellen und Zeitfenster in der Gold-Schicht aggregiert werden, wie kann AĒR erkennen, ob beobachtete Trends genuine Diskursverschiebungen widerspiegeln vs. kompositorische Veränderungen im Korpus?
- Deliverable: Ein statistisches Framework zur Dekomposition aggregierter Metrikänderungen in innerhalb-der-Quelle- und zwischen-Quellen-Komponenten.

**F10: Welches temporale Aggregationsfenster ist für verschiedene Metriktypen angemessen?**

- Sentiment kann bei täglicher Auflösung für Eilmeldungen bedeutsam sein, aber nur bei wöchentlicher oder monatlicher Auflösung für kulturellen Drift. Themenprävalenz kann andere temporale Fenster erfordern als Entity-Ko-Okkurrenz-Netzwerke.
- Deliverable: Eine empirische Analyse der Metrikstabilität über temporale Auflösungen auf einem Pilotkorpus.

---

## 8. Einordnung in AĒRs architektonische Constraints

### 8.1 Tier-Klassifikation revisited

Das in §6 vorgeschlagene Validierungsrahmenwerk kann offenbaren, dass keine Tier-1-Methode (vollständig deterministisch) für AĒRs analytische Ziele adäquat ist. Dies würde eine strategische Entscheidung erzwingen:

- **Option A: Tier-1-Limitierungen akzeptieren.** Lexikonbasiertes Sentiment als transparentes, aber grobes Signal verwenden. Im Dashboard anerkennen, dass Sentimentscores *lexikalische Polaritätsindikatoren* sind, keine Messungen evaluativer Einstellung. Dies ist ehrlich und architektonisch einfach, aber analytisch schwach.

- **Option B: Validierte Tier-2-Methoden einführen.** Einen überwacht trainierten Klassifikator oder feinabgestimmten Transformer mit fixierter Modellversion und fixiertem Seed adoptieren. Höhere Genauigkeit bei bewahrter Reproduzierbarkeit. Dies erfordert Validierungsinfrastruktur (Annotationsstudien, Benchmarking-Pipelines) und führt Modellabhängigkeitsrisiken ein (R-10).

- **Option C: Hybridarchitektur.** Tier 1 als unveränderliche Baseline verwenden. Tier-2/3-Anreicherungen darüber schichten, explizit gekennzeichnet. Das Dashboard zeigt stets den Tier-1-Score; Tier-2/3-Scores sind über Progressive Disclosure verfügbar. Dies bewahrt AĒRs Transparenzgarantie bei gleichzeitiger analytischer Tiefe.

Option C passt am besten zu AĒRs bestehender Tier-Architektur (§13.3) und dem Ockham's-Razor-Prinzip — sie fügt Komplexität nur dort hinzu, wo die einfachere Methode *nachweislich* unzureichend ist, und verbirgt die einfache Methode nie hinter der komplexen.

### 8.2 Auswirkung auf die Extractor-Pipeline

Das Validierungsrahmenwerk erfordert keine Änderungen an der technischen Architektur der Extractor-Pipeline. Das `MetricExtractor`-Protokoll und das Dependency-Injection-Pattern (§8.10) unterstützen bereits mehrere Extractors, die Metriken mit derselben semantischen Absicht, aber unterschiedlichen Methoden produzieren. Ein validierter Tier-2-Sentiment-Extractor würde neben (nicht anstelle von) dem bestehenden SentiWS-Extractor registriert und einen separaten `metric_name` produzieren (z. B. `sentiment_score_bert` vs. `sentiment_score_sentiws`). Der `/metrics/available`-Endpunkt der BFF API würde beide exponieren; das Dashboard würde basierend auf dem Validierungsstatus der Metrik auswählen.

### 8.3 Auswirkung auf den Datenvertrag

Der `SilverCore`-Record bleibt unverändert — Extractors empfangen unveränderliche Silver-Daten und produzieren Gold-Metriken. Die vorgeschlagene `aer_gold.metric_validity`-Tabelle (§6.3) ist ein neues Gold-Schicht-Konstrukt, das den Silver Contract nicht beeinflusst (ADR-002, ADR-015).

---

## 9. Referenzen

- Cronbach, L. J. & Meehl, P. E. (1955). „Construct Validity in Psychological Tests." *Psychological Bulletin*, 52(4), 281–302.
- Grimmer, J., Roberts, M. E. & Stewart, B. M. (2022). *Text as Data: A New Framework for Machine Learning and the Social Sciences*. Princeton University Press.
- Guhr, O., Schumann, A.-K., Baber, F. & Buettner, A. (2020). „Training a Broad-Coverage German Sentiment Classification Model for Dialog Systems." *Proceedings of LREC 2020*.
- Hall, E. T. (1976). *Beyond Culture*. Anchor Books.
- Hofstede, G. (2001). *Culture's Consequences: Comparing Values, Behaviors, Institutions and Organizations Across Nations*. Sage.
- Hutto, C. J. & Gilbert, E. (2014). „VADER: A Parsimonious Rule-based Model for Sentiment Analysis of Social Media Text." *Proceedings of ICWSM 2014*.
- Jauhiainen, T., Lui, M., Zampieri, M., Baldwin, T. & Lindén, K. (2017). „Automatic Language Identification in Texts: A Survey." *Journal of Artificial Intelligence Research*, 65, 675–782.
- Koehn, P. & Knight, K. (2003). „Empirical Methods for Compound Splitting." *Proceedings of EACL 2003*.
- Krippendorff, K. (2004). *Content Analysis: An Introduction to Its Methodology* (2. Aufl.). Sage.
- Remus, R., Quasthoff, U. & Heyer, G. (2010). „SentiWS — A Publicly Available German-Language Resource for Sentiment Analysis." *Proceedings of LREC 2010*.
- Robinson, W. S. (1950). „Ecological Correlations and the Behavior of Individuals." *American Sociological Review*, 15(3), 351–357.
- Rosenthal, S., Farra, N. & Nakov, P. (2017). „SemEval-2017 Task 4: Sentiment Analysis in Twitter." *Proceedings of SemEval-2017*.
- Russell, J. A. (1980). „A Circumplex Model of Affect." *Journal of Personality and Social Psychology*, 39(6), 1161–1178.
- Thelwall, M., Buckley, K., Paltoglou, G., Cai, D. & Kappas, A. (2010). „Sentiment Strength Detection in Short Informal Text." *Journal of the American Society for Information Science and Technology*, 61(12), 2544–2558.
- Van Hee, C., Lefever, E. & Hoste, V. (2018). „SemEval-2018 Task 3: Irony Detection in English Tweets." *Proceedings of SemEval-2018*.
- Wiegand, M., Balahur, A., Roth, B., Klakow, D. & Montoyo, A. (2010). „A Survey on the Role of Negation in Sentiment Analysis." *Proceedings of the Workshop on Negation and Speculation in Natural Language Processing*.

---

## Anhang A: Zuordnung zu AĒR Offene Forschungsfragen (§13.6)

| §13.6 Frage | WP-002 Abschnitt | Status |
| :--- | :--- | :--- |
| 3. Metrik-Validität | §2–§3, §6 | Adressiert — Validierungsrahmenwerk vorgeschlagen |
| 4. Interkulturelle Vergleichbarkeit | §3.4, §4.3, §7.3 | Adressiert — Forschungsfragen formuliert |
| 5. Temporale Granularität | §7.4 (F10) | Teilweise adressiert — WP-005 gewidmet |
| 1. Sondenauswahl | — | Adressiert in WP-001 |
| 2. Bias-Kalibrierung | — | WP-003 gewidmet |
| 6. Beobachtereffekt | — | WP-006 gewidmet |