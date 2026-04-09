# WP-004: Interkulturelle Vergleichbarkeit von Diskursmetriken

> **Reihe:** AĒR Wissenschaftliche Methodik — Arbeitspapiere
> **Status:** Entwurf — offen für interdisziplinäre Begutachtung
> **Datum:** 07.04.2026
> **Abhängig von:** WP-001 (Funktionale Sondentaxonomie), WP-002 (Metrik-Validität), WP-003 (Plattform-Bias)
> **Architekturkontext:** Gold-Layer-Aggregation (§5.1.4), ClickHouse OLAP (§8.6), BFF API (§5.1.3), Manifest §II–§IV
> **Lizenz:** [CC BY-NC 4.0](https://creativecommons.org/licenses/by-nc/4.0/) — © 2026 Fabian Quist

---

## 1. Zielsetzung

Dieses Arbeitspapier adressiert die vierte offene Forschungsfrage aus §13.6: **Kann dieselbe Metrik über Sprachen und kulturelle Kontexte hinweg sinnvoll verglichen werden? Welche Normalisierung ist erforderlich?**

AĒRs Aleph-Prinzip — die Aggregation fragmentierter globaler Datenströme in eine einzige kohärente Ansicht — setzt voraus, dass Daten aus verschiedenen kulturellen Kontexten *nebeneinander* in einem vereinheitlichten analytischen Raum platziert werden können. Doch Vergleich ist nicht Konkatenation. Einen deutschen Sentimentwert neben einen japanischen Sentimentwert in derselben ClickHouse-Spalte zu platzieren, macht sie nicht vergleichbar. Es macht sie *ko-lokalisiert*.

Vergleichbarkeit ist die zentrale methodologische Herausforderung jedes interkulturellen Forschungsinstruments. Die Geschichte der vergleichenden Sozialwissenschaft — von Durkheims vergleichender Methode über den World Values Survey bis zur heutigen Computational-Social-Science-Landschaft — ist eine Geschichte des Ringens mit einem fundamentalen Paradoxon: **Um zu vergleichen, braucht man einen gemeinsamen Bezugsrahmen; doch die Aufzwingung eines gemeinsamen Rahmens riskiert, genau jene Unterschiede auszulöschen, die man zu beobachten sucht.** Dies ist das Vergleichbarkeitsparadoxon, und es liegt im Kern von AĒRs wissenschaftlicher Agenda.

WP-001 führte das Etisch/Emische Duale Tagging-System als strukturelle Antwort auf dieses Paradoxon auf Sondenebene ein. WP-004 erweitert diese Logik auf die Metrikebene. Es fragt: Für jede computationale Metrik, die AĒR produziert, unter welchen Bedingungen ist interkultureller Vergleich sinnvoll, welche Normalisierung ist erforderlich, und wo wird Vergleich epistemologisch illegitim?

---

## 2. Das Vergleichbarkeitsparadoxon

### 2.1 Vergleichen, was nicht gleichgesetzt werden kann

Das Paradoxon ist in der vergleichenden Methodik altbekannt. Sartori (1970) formulierte es als die „Leiter der Abstraktion": Je abstrakter ein Konzept (und damit je breiter anwendbar über Kontexte hinweg), desto weniger erfasst es die spezifische Realität eines einzelnen Kontexts. Je konkreter und kontextsensitiver ein Konzept, desto weniger lässt es sich über seinen ursprünglichen Rahmen hinaus anwenden.

Angewandt auf AĒRs Metriken:

- Eine **hochabstrakte Metrik** wie „Wortzahl" ist über Sprachen und Kulturen hinweg perfekt vergleichbar. Ein 200-Wort-Artikel auf Deutsch und ein 200-Wort-Artikel auf Japanisch haben dieselbe Wortzahl (unter der Annahme äquivalenter Tokenisierung — bereits eine nicht-triviale Annahme, wie in §3.1 diskutiert). Aber Wortzahl sagt uns fast nichts über Diskurs.

- Eine **moderat abstrakte Metrik** wie „Sentimentpolarität" beansprucht, etwas kulturell Universelles zu messen — evaluative Einstellung — tut dies aber mit kulturspezifischen Instrumenten (Lexika, auf kulturell situierten Daten trainierte Modelle). Derselbe Sentimentwert, produziert von verschiedenen Werkzeugen in verschiedenen Sprachen, misst möglicherweise nicht dasselbe Konstrukt (WP-002, §3.4).

- Eine **kulturspezifische Metrik** wie „Narrativrahmen" (z. B. „Versicherheitlichungsrahmen", „Menschenrechtsrahmen") kann präzise das Diskursphänomen von Interesse erfassen, aber ihre Anwendbarkeit ist durch den kulturellen und politischen Kontext begrenzt, in dem der Rahmen existiert. Der Versicherheitlichungsrahmen (Buzan et al., 1998) ist ein Produkt westlicher Theorie der internationalen Beziehungen; ihn auf den chinesischen außenpolitischen Diskurs anzuwenden, zwingt eine theoretische Linse auf, die möglicherweise nicht passt.

AĒR muss auf allen drei Ebenen gleichzeitig operieren. Die Aleph-Vision erfordert hohe Abstraktion (globale Aggregation); die Episteme-Vision erfordert kulturelle Sensibilität (messen, was *innerhalb* eines spezifischen epistemischen Rahmens gedacht und gesagt werden kann); die Rhizom-Vision erfordert relationale Analyse (wie Muster *über* Rahmen hinweg propagieren). Kein einzelnes Abstraktionsniveau dient allen dreien.

### 2.2 Äquivalenz als Forschungsproblem, nicht als Annahme

Vergleichende Methodik unterscheidet mehrere Typen von Äquivalenz (van de Vijver & Leung, 1997):

**Konstruktäquivalenz.** Misst die Metrik dasselbe theoretische Konstrukt über Kontexte hinweg? Sentimentpolarität ist möglicherweise nicht dasselbe Konstrukt in Kulturen mit unterschiedlichen Affektsystemen (WP-002, §3.4). Aus LDA auf einem deutschen Korpus abgeleitete Themenkategorien repräsentieren möglicherweise nicht dieselben thematischen Strukturen im arabischen Diskurs.

**Messäquivalenz.** Selbst wenn das Konstrukt dasselbe ist, operiert das Messinstrument identisch? Ein Sentimentlexikon, das auf deutschen Zeitungstexten kalibriert wurde, produziert systematisch andere Scores auf deutschen Parlamentsdebatten-Transkripten — und die Divergenz liegt *innerhalb* einer einzelnen Sprache und Kultur. Über Sprachen hinweg erfordert Messäquivalenz separate Validierung für jede Sprache (WP-002, §6.2).

**Skalare Äquivalenz.** Können Scores auf derselben Skala verglichen werden? Zeigt ein Sentimentwert von `0,3` auf Deutsch und `0,3` auf Japanisch denselben *Grad* positiver Bewertung an? Ohne skalare Äquivalenz sind nur Rangordnungsvergleiche gültig („positiver als"), nicht Magnitudenvergleiche („gleich positiv").

**Temporale Äquivalenz.** Behält eine Metrik dieselbe Bedeutung über die Zeit innerhalb eines einzelnen kulturellen Kontexts? Sprache entwickelt sich, Medienregister verschieben sich, politische Vokabulare transformieren sich. Ein 2020 validiertes Sentimentlexikon produziert möglicherweise keine äquivalenten Ergebnisse in 2026, selbst innerhalb derselben Sprache.

AĒRs wissenschaftliche Integrität erfordert, dass **Äquivalenz als Forschungsfrage behandelt wird, die für jedes Metrik-Kontext-Paar empirisch getestet werden muss, nicht als Annahme, die stillschweigend getroffen wird.** Dies hat direkte architektonische Konsequenzen: Die Gold-Schicht muss genügend Metadaten speichern, um zwischen validierten und nicht-validierten interkulturellen Vergleichen zu unterscheiden.

---

## 3. Linguistische Vergleichbarkeitsherausforderungen

### 3.1 Tokenisierung: Die erste unsichtbare Divergenz

Jede Textmetrik beginnt mit Tokenisierung — dem Aufteilen von kontinuierlichem Text in diskrete Einheiten. Dieser scheinbar triviale Vorverarbeitungsschritt führt die erste sprachübergreifende Unvergleichbarkeit ein.

**Leerzeichen-begrenzte Sprachen** (Englisch, Deutsch, Französisch, Spanisch, Portugiesisch, Russisch, Arabisch) verwenden Whitespace als Wortgrenzmarker. Tokenisierung ist relativ geradlinig, wobei Komplikationen durch Klitika (französisch *l'homme*), Kontraktionen (englisch *don't*) und Komposita (deutsch *Klimaschutzpaket*) entstehen.

**Nicht-leerzeichen-begrenzte Sprachen** (Chinesisch, Japanisch, Thailändisch, Khmer, Laotisch, Myanmar) erfordern algorithmische Wortsegmentierung. Chinesischer Text ist eine kontinuierliche Zeichenkette ohne Whitespace zwischen Wörtern; die korrekte Segmentierung hängt vom Kontext ab und ist selbst eine NLP-Aufgabe mit nicht-trivialen Fehlerquoten. Das Wort *„研究生命的起源"* kann segmentiert werden als *„研究/生命/的/起源"* (den Ursprung des Lebens erforschen) oder *„研究生/命/的/起源"* (das Schicksal eines Doktoranden). Verschiedene Segmentierer (jieba, pkuseg, LTP) produzieren unterschiedliche Token-Zahlen für denselben Text.

**Agglutinative Sprachen** (Türkisch, Finnisch, Ungarisch, Koreanisch, Swahili, Quechua) kodieren grammatische Beziehungen durch Affigierung. Ein einzelnes türkisches Wort wie *„evlerinizden"* (aus euren Häusern) kodiert Stamm, Plural, Possessiv und Ablativ — Informationen, die fünf englische Wörter erfordern. Token-Level-Metriken (Wortzahl, Type-Token-Ratio, Lexikonabdeckung) sind zwischen agglutinativen und analytischen Sprachen ohne morphologische Normalisierung strukturell unvergleichbar.

**Implikation für AĒR:** Die `word_count`-Metrik in `SilverCore` — derzeit berechnet durch `len(cleaned_text.split())` — ist sprachübergreifend nicht vergleichbar. Ein 200-Wort-Artikel auf Deutsch und ein 200-Wort-Artikel auf Japanisch (nach Segmentierung) repräsentieren nicht äquivalente Informationsmengen. AĒR muss entweder (a) sprachspezifische Tokenisierungsstrategien pro Source Adapter definieren, (b) sprachunabhängige Einheiten verwenden (Zeichen-N-Gramme, Byte-Pair-Encoding-Tokens) oder (c) anerkennen, dass Wort-Level-Metriken nur intralinguistisch gelten.

### 3.2 Sentiment über Sprachen hinweg: Verschiedene Instrumente, verschiedene Konstrukte

WP-002 (§3.4) dokumentierte die kulturellen Herausforderungen der Sentimentanalyse. Aus Vergleichbarkeitsperspektive ist das Problem strukturell:

**Es existiert kein universelles Sentimentlexikon.** SentiWS (Deutsch), AFINN (Englisch), NRC Emotion Lexicon (mehrsprachig, aber englisch-abgeleitet), SentiWordNet (englisch-abgeleitet über WordNet) — jedes Lexikon wurde von verschiedenen Teams entwickelt, unter Verwendung verschiedener Annotationsmethodologien, verschiedener Korpora und verschiedener theoretischer Rahmenwerke. Selbst wenn zwei Lexika beide beanspruchen, „Sentiment" zu messen, operationalisieren sie es unterschiedlich. SentiWS weist kontinuierliche Polaritätsgewichte zu; AFINN verwendet ganzzahlige Scores (-5 bis +5); NRC verwendet binäre Emotionskategorien. Ihre Outputs auf demselben Dashboard zu platzieren ist ohne explizite Kalibrierung methodologisch unhaltbar.

**Übersetzung bewahrt Sentiment nicht.** Die maschinelle Übersetzung von Text in eine gemeinsame Sprache (typischerweise Englisch) vor Anwendung eines einzigen Sentimentwerkzeugs ist eine verbreitete Abkürzung, die systematischen Bias einführt. Übersetzung neutralisiert kulturspezifische Konnotationen, verflacht Registerunterschiede und zwingt die pragmatischen Konventionen der Zielsprache auf. Das arabische Wort *„إن شاء الله"* (insha'Allah) wird in Kontexten von genuiner Frömmigkeit über höfliche Ablehnung bis hin zu resigniertem Fatalismus verwendet — es kann nicht ins Englische übersetzt und dann sentimentbewertet werden, ohne seine pragmatische Funktion zu verlieren.

**Vorgeschlagener Ansatz: Relativer statt absoluter Vergleich.** Anstatt rohe Sentimentscores sprachübergreifend zu vergleichen, sollte AĒR *kontextinterne Abweichungen* von einer etablierten Baseline vergleichen. Wenn die Baseline des Sentiments von Tagesschau-Artikeln bei `0,05` liegt und eine gegebene Woche `0,15` produziert, ist die Abweichung von `+0,10` ein bedeutsames Signal innerhalb des deutschen institutionellen RSS-Kontexts. Wenn die Baseline von NHK-Artikeln (Japan) bei `0,01` liegt und eine gegebene Woche `0,06` produziert, ist die Abweichung von `+0,05` ein vergleichbares Signal — nicht weil die Rohwerte äquivalent wären, sondern weil beide eine Verschiebung ähnlicher relativer Größenordnung von ihren jeweiligen Baselines repräsentieren.

Dieser Ansatz erfordert:

1. Etablierung von Pro-Quellen-Sentiment-Baselines über eine Kalibrierungsperiode
2. Berechnung von Abweichungen von der Baseline (Z-Scores, Perzentilränge oder Differenz-in-Differenzen)
3. Vergleich von Abweichungen, nicht Rohwerten, im Dashboard
4. Dokumentation der Baseline-Periode, Methode und des kulturellen Kontexts als Metrik-Metadaten

### 3.3 Named Entities über Ontologien hinweg

WP-002 (§4) dokumentierte NER-Herausforderungen. Interkulturelle Vergleichbarkeit führt ein tieferes Problem ein: **ontologische Inkommensurabilität**.

Die Entity-Kategorien westlicher NER-Modelle (PER, ORG, LOC, GPE, DATE, MONEY) spiegeln ein westliches ontologisches Rahmenwerk wider, das Personen von Organisationen, Orte von geopolitischen Entitäten und temporale von monetären Ausdrücken unterscheidet. Diese Unterscheidungen sind nicht universell salient:

- In vielen indigenen Kulturen ist die Unterscheidung zwischen PER (Person) und LOC (Ort) durch animistische Ontologien verwischt, in denen Flüsse, Berge und Wälder Personalität besitzen.
- Die ORG-Kategorie setzt ein westliches Modell formaler Organisation voraus, das sich nicht sauber auf Clanstrukturen, Kastennetzwerke oder informelle Patronagesysteme abbilden lässt.
- Die GPE-Kategorie (geopolitische Entität) kodiert umstrittene politische Grenzen als etablierte Fakten.

Für AĒRs Entity-Aggregation (`aer_gold.entities`) bedeutet dies, dass interkulturelle Entity-Zählungen und Entity-Ko-Okkurrenz-Netzwerke nur innerhalb des ontologischen Rahmenwerks des verwendeten NER-Modells vergleichbar sind. „Top-Entitäten im deutschen Diskurs" mit „Top-Entitäten im chinesischen Diskurs" unter Verwendung derselben Entity-Taxonomie zu vergleichen, produziert linguistisch valide, aber ontologisch fehlerhafte Ergebnisse.

WP-001s emische Schicht adressiert dies auf Sondenebene. Der analoge Mechanismus auf Metrikebene wäre ein **pro-Sprache-Entity-Ontologie-Mapping**, das lokale Entity-Kategorien neben den etischen NER-Labels bewahrt.

### 3.4 Topic Models über kulturelle Grenzen hinweg

Topic Modeling (LDA, BERTopic) entdeckt latente thematische Strukturen in Dokumentensammlungen. Diese Strukturen sind korpusabhängig — die in einem deutschen Korpus entdeckten Topics sind nicht dieselben wie die in einem japanischen Korpus, selbst wenn beide Korpora „die Nachrichten" abdecken.

**Sprachspezifische Themenräume.** Ein deutsches LDA-Modell könnte Topics wie „Energiewende", „Leitkultur" oder „Schuldenbremse" entdecken — Konzepte, die tief im deutschen politischen Diskurs verankert sind und in anderen Sprachen keine direkten Äquivalente haben. Ein japanisches Modell könnte „働き方改革" (Reform der Arbeitsweise) oder „少子高齢化" (sinkende Geburtenrate und alternde Gesellschaft) entdecken. Dies sind nicht verschiedene Labels für dieselben Topics — es sind verschiedene Topics, die verschiedene gesellschaftliche Präokkupationen widerspiegeln.

**Sprachübergreifendes Topic-Alignment.** Neuere Forschung zu mehrsprachigen Topic-Modellen (Hao & Paul, 2018; Bianchi et al., 2021) versucht, gemeinsame Themenräume über Sprachen hinweg mittels mehrsprachiger Embeddings zu entdecken. Diese Modelle können thematisch verwandte Cluster über Sprachen hinweg identifizieren — aber „verwandt" ist nicht „äquivalent". Das deutsche „Energiewende"-Topic und das japanische „原発再稼働" (Atomkraft-Neustart)-Topic können zusammen clustern, weil beide Energiepolitik betreffen, aber sie tragen fundamental verschiedene politische Valenzen, historische Kontexte und gesellschaftliche Bedeutungen.

**Vorgeschlagener Ansatz: Parallele Themenentdeckung mit menschlich validiertem Alignment.** Anstatt ein einzelnes sprachübergreifendes Topic-Modell aufzuzwingen, sollte AĒR:

1. Sprachspezifische Topic-Modelle pro linguistischem Korpus ausführen (intrakulturelle Themenentdeckung)
2. Mehrsprachige Embeddings verwenden, um sprachübergreifende Topic-Alignments vorzuschlagen
3. Menschliche Validierung durch Regionalexperten für jedes vorgeschlagene Alignment erfordern
4. Topic-Alignments als separate Mapping-Tabelle speichern, nicht als Eigenschaft der Topics selbst

Dies bewahrt die kulturelle Spezifität jedes Topics und ermöglicht gleichzeitig validierten interkulturellen Vergleich — eine direkte Anwendung des Etisch/Emisch-Prinzips aus WP-001.

---

## 4. Kulturelle Vergleichbarkeit: Jenseits der Sprache

### 4.1 Expressive Normen und Baseline-Kalibrierung

Interkulturelle Vergleichbarkeit ist nicht lediglich ein linguistisches Problem — es ist ein kulturelles. Selbst wenn zwei Sprachen identische Sentimentlexika hätten (was nicht der Fall ist), unterscheidet sich die *Baseline-Expressivität* des Diskurses über Kulturen hinweg in Weisen, die die Metrikinterpretation beeinflussen.

**High-Context- vs. Low-Context-Kommunikation.** Halls (1976) Unterscheidung zwischen High-Context-Kulturen (wo Bedeutung in Kontext, Beziehungen und nonverbalen Signalen eingebettet ist — Japan, China, arabische Kulturen, weite Teile Afrikas) und Low-Context-Kulturen (wo Bedeutung explizit im Text liegt — Deutschland, Skandinavien, die Vereinigten Staaten) hat direkte Implikationen für textbasierte Metriken. In High-Context-Kommunikation ist die wichtigste Information, was *nicht* gesagt wird. Eine Textmetrik, die nur expliziten Inhalt misst, unterrepräsentiert systematisch den kommunikativen Reichtum von High-Context-Kulturen.

**Emotionale Darstellungsregeln.** Kulturen unterscheiden sich in den Normen darüber, welche Emotionen öffentlich ausgedrückt werden können und mit welcher Intensität (Matsumoto, 1990). Japanischer Mediendiskurs folgt *honne/tatemae*-Normen (本音/建前), die private Meinung von öffentlichem Ausdruck unterscheiden. Arabischer Mediendiskurs operiert innerhalb von *wajh*-Normen (وجه, „Gesicht"), die den öffentlichen Ausdruck von Kritik und Dissens regeln. Amerikanischer Mediendiskurs folgt Normen affektiver Intensivierung, die „amazing", „incredible" und „devastating" zu registerneutralen Deskriptoren machen. Dies ist kein Rauschen — es sind kulturelle Daten, die AĒR beobachten sollte, nicht wegnormalisieren.

**Institutionelle Registerkonventionen.** Regierungspressemitteilungen — eine primäre Datenquelle für AĒRs frühe Sonden — folgen kulturspezifischen institutionellen Registern (WP-002, §3.4). Das *Beamtendeutsch* der Bundesregierung, die *guānfāng yǔyán* (官方语言) des chinesischen Staatsrats, der britische Civil-Service-Stil und der amerikanische Executive-Prose-Stil produzieren alle systematisch verschiedene Sentimentprofile, die institutionelle Kultur widerspiegeln, nicht öffentliches Sentiment. Sentiment über diese Register hinweg ohne Berücksichtigung registerspezifischer Baselines zu vergleichen, würde Artefakte produzieren, keine Befunde.

### 4.2 Das Normalisierungsdilemma

WP-002 (§7.3, F7) antizipierte eine fundamentale epistemologische Spannung: Wenn AĒR Metriken normalisiert, um interkulturelle Baseline-Unterschiede zu berücksichtigen, kann es genau die Phänomene auslöschen, die es zu beobachten sucht.

**Fallstudie: Zurückhaltung als kulturelles Datum.** Wenn japanischer institutioneller Diskurs systematisch zurückhaltender ist als amerikanischer institutioneller Diskurs (niedrigere absolute Sentimentscores, weniger Superlative, mehr Hedging), dann ist dieser Unterschied selbst ein Befund — er spiegelt verschiedene Kommunikationsnormen, verschiedene institutionelle Kulturen und verschiedene Beziehungen zwischen Staat und Öffentlichkeit wider. Beide auf dieselbe Baseline zu normalisieren, löscht diesen Befund aus.

**Fallstudie: Polarisierung als relatives Phänomen.** Wenn deutscher politischer Diskurs eine Sentiment-Spanne von [-0,3; +0,3] zeigt, während amerikanischer politischer Diskurs [-0,8; +0,8] zeigt, kann die breitere amerikanische Spanne höhere Polarisierung anzeigen — oder sie kann verschiedene expressive Normen widerspiegeln. Nur mit kultureller Expertise können wir diese Erklärungen unterscheiden.

**Das Normalisierungsspektrum:**

| Strategie | Was sie vergleicht | Was sie bewahrt | Was sie auslöscht |
| :--- | :--- | :--- | :--- |
| **Rohwerte** | Nichts (Werte sind inkommensurabel) | Alles | Nichts |
| **Z-Score pro Quelle** | Intra-Quellen-Abweichungen von der Baseline | Temporale Dynamiken innerhalb jeder Quelle | Absolute Niveauunterschiede zwischen Quellen |
| **Perzentilrang pro Sprache** | Relative Position innerhalb des linguistischen Korpus | Rangordnung innerhalb der Sprache | Skalenunterschiede zwischen Sprachen |
| **Abweichung von Diskursfunktions-Baseline** | Verschiebungen relativ zu gleich-funktionalen Quellen | Funktionale Äquivalenz (WP-001) | Intrafunktionale kulturelle Variation |
| **Universelle Normalisierung** | Rohe Positionen auf einer einzigen globalen Skala | Nichts Kulturspezifisches | Alles Kulturspezifische |

AĒR sollte **mehrere Normalisierungsansichten gleichzeitig unterstützen**, wählbar durch den Analysten. Das Dashboard sollte niemals einen einzigen „korrekten" Vergleich präsentieren — es sollte die Rohdaten, die Normalisierungsmethode und den kulturellen Kontext exponieren und dem Analysten ermöglichen, den für seine Forschungsfrage angemessenen Vergleich zu wählen.

### 4.3 Temporale Rhythmen über Kulturen hinweg

Temporale Metriken — Publikationsfrequenz, Tageszeit-Verteilung, Wochentag-Muster — gehören zu AĒRs robustesten und sprachunabhängigsten Metriken. Doch selbst diese tragen kulturelle Spezifität.

**Wochenrhythmen.** Das „Wochenende" ist kulturell definiert. In den meisten westlichen Ländern sind Samstag und Sonntag Ruhetage. In Israel ist das Wochenende Freitag–Samstag. In vielen muslimisch geprägten Ländern ist Freitag der primäre Ruhetag; manche beobachten Freitag–Samstag, andere Donnerstag–Freitag. AĒRs `publication_weekday`-Metrik (0=Montag, 6=Sonntag) kodiert dieses kulturelle Wissen nicht. Ein Einbruch der Publikationsfrequenz am Freitag in saudischen Quellen und am Sonntag in deutschen Quellen spiegelt dasselbe soziale Phänomen wider — den wöchentlichen Ruhezyklus — wird aber als verschiedene Datenpunkte kodiert.

**Nachrichtenzyklen.** Der Rhythmus der Nachrichtenpublikation variiert nach Medienkultur. Amerikanische Nachrichtenzyklen sind 24-stündig mit Spitzen durch die Geschäftszeiten an der Ostküste. Japanische Nachrichtenzyklen folgen *asa-kan/yū-kan*-Rhythmen (朝刊/夕刊, Morgen-/Abendausgabe), die vom Printjournalismus geerbt sind. Europäische Nachrichtenzyklen variieren nach Land. Eilmeldungsdynamiken unterscheiden sich nach Plattform: Twitter/X operiert auf Minutenebene; RSS auf Stundenebene; traditionelle Medien auf Halbtagsebene.

**Kalendereffekte.** Kulturelle und religiöse Kalender beeinflussen Publikationsmuster in Weisen, die vom gregorianischen Kalender nicht erfasst werden. Ramadan, Chinesisches Neujahr, Diwali, Obon und nationale Unabhängigkeitstage erzeugen alle temporale Muster im Diskurs, die kulturellen Kontext zur Interpretation erfordern. Ein Rückgang des Publikationsvolumens während des Chinesischen Neujahrs ist kein Rückgang des Diskurses — es ist ein kultureller Rhythmus.

---

## 5. Erweiterung des Etisch/Emischen Rahmenwerks auf Metriken

### 5.1 Von Sonden zu Metriken

WP-001 etablierte das Etisch/Emische Duale Tagging-System für die Sondenklassifikation:

- **Etische Schicht:** Abstrakte, funktionale Klassifikation, die interkulturelle Aggregation ermöglicht
- **Emische Schicht:** Lokale, kontextspezifische Metadaten, die die kulturelle Realität bewahren

Dieselbe Logik gilt für Metriken. Jede Metrik in der Gold-Schicht sollte beides tragen:

- **Etische Metrikidentität:** Welches abstrakte Konstrukt misst diese Metrik? (z. B. „evaluative Polarität", „Entity-Salienz", „thematischer Fokus")
- **Emischer Metrikkontext:** Welches kulturspezifische Instrument hat diese Messung produziert? (z. B. „SentiWS v2.0 auf deutschem Text", „ASARI auf japanischem Text", „BERT-base-arabic-sentiment auf arabischem Text")

Dies ermöglicht der BFF API, zwei Ansichten zu exponieren:

1. **Etische Aggregation:** „Zeige mir evaluative Polarität über alle Quellen" — aggregiert Metriken verschiedener Instrumente, die dasselbe Konstrukt messen, mit expliziten Äquivalenz-Metadaten
2. **Emisches Detail:** „Zeige mir SentiWS-Sentiment für Tagesschau" — zeigt den Rohoutput eines spezifischen Instruments auf einer spezifischen Quelle, ohne interkulturelle Ansprüche

### 5.2 Die Metrik-Äquivalenz-Registry

Um die etisch/emische Metrikklassifikation zu operationalisieren, benötigt AĒR eine **Metrik-Äquivalenz-Registry** — ein kuratiertes Mapping, das definiert, welche instrumentspezifischen Metriken für interkulturellen Vergleich als äquivalent betrachtet werden, unter welchen Bedingungen und mit welcher Konfidenz.

```sql
CREATE TABLE aer_gold.metric_equivalence (
    etic_construct     String,         -- z. B. "evaluative_polarity"
    metric_name        String,         -- z. B. "sentiment_score_sentiws"
    language           String,         -- z. B. "de"
    instrument_version String,         -- z. B. "SentiWS_v2.0"
    equivalence_type   String,         -- "construct", "measurement", "scalar"
    validation_status  String,         -- "validated", "assumed", "contested"
    validation_ref     Nullable(String), -- Referenz zur Validierungsstudie
    valid_from         DateTime,
    valid_until        Nullable(DateTime)
) ENGINE = MergeTree()
ORDER BY (etic_construct, language, metric_name);
```

Diese Tabelle wird *nicht* automatisch befüllt — sie wird von Forschern durch den in WP-002 (§6.2) beschriebenen Validierungsprozess kuratiert. Jeder Eintrag repräsentiert ein wissenschaftliches Urteil: „SentiWS-Sentiment auf deutschem RSS und ASARI-Sentiment auf japanischem RSS werden für das Konstrukt ‚evaluative Polarität' als messäquivalent betrachtet basierend auf [Validierungsstudie]." Ohne Eintrag in dieser Registry sollte interkulturelle Aggregation eines Metrikpaars im Dashboard als nicht-validiert geflaggt werden.

### 5.3 Drei Ebenen interkulturellen Vergleichs

Basierend auf dem Äquivalenz-Rahmenwerk kann AĒR drei Ebenen interkulturellen Vergleichs unterstützen, jede mit unterschiedlichen Anforderungen und Konfidenzniveaus:

**Ebene 1: Temporaler Mustervergleich (sprachunabhängig).** Vergleich, *wann* Diskurs über Kulturen hinweg stattfindet: Publikationsfrequenz, Tageszeit-Verteilungen, Wochenrhythmen, Ereignis-Reaktionslatenzen. Diese Metriken sind strukturell vergleichbar, weil sie Zeit messen, nicht Text. Kulturelles Kalenderwissen ist für die Interpretation erforderlich, aber nicht für die Berechnung. Konfidenz: Hoch.

**Ebene 2: Relativer Abweichungsvergleich (baseline-normalisiert).** Vergleich, *wie stark sich Diskurs* über Kulturen hinweg *verändert*: Z-Score-Abweichungen von etablierten Baselines, Intra-Quellen-Trendrichtungen, Volatilitätsmaße. Diese Vergleiche sind sinnvoll, weil sie Veränderung relativ zu einem kulturell kalibrierten Referenzpunkt messen. Sie beanspruchen nicht, dass die absoluten Werte äquivalent sind — nur dass Magnitude und Richtung der Veränderung vergleichbar sind. Konfidenz: Mittel, erfordert Pro-Quellen-Baseline-Kalibrierung.

**Ebene 3: Absolutwertvergleich (instrumentharmonisiert).** Vergleich dessen, *was Diskurs sagt*, über Kulturen hinweg: Sentimentniveaus, Themenprävalenzen, Entity-Salienzen. Dies ist die ambitionierteste und fragilste Vergleichsebene. Sie erfordert validierte skalare Äquivalenz zwischen Instrumenten, Normalisierung für expressive Normen und Expertenurteil über Konstruktäquivalenz. Konfidenz: Niedrig ohne umfangreiche Validierung; sollte niemals die Standard-Dashboard-Ansicht sein.

---

## 6. Architektonische Implikationen

### 6.1 Gold-Layer-Erweiterungen

Das aktuelle `aer_gold.metrics`-Schema speichert `(timestamp, value, source, metric_name)`. Zur Unterstützung interkultureller Vergleichbarkeit werden folgende Erweiterungen vorgeschlagen:

**Baseline-Tabelle.** Speichert berechnete Baselines pro Quelle und Metrik für Abweichungsberechnungen:

```sql
CREATE TABLE aer_gold.metric_baselines (
    metric_name    String,
    source         String,
    language       String,
    baseline_value Float64,
    baseline_std   Float64,
    window_start   DateTime,
    window_end     DateTime,
    n_documents    UInt32,
    compute_date   DateTime
) ENGINE = ReplacingMergeTree(compute_date)
ORDER BY (metric_name, source, language);
```

**Abweichungsansicht.** Eine ClickHouse Materialized View oder Abfragezeitberechnung, die Z-Scores produziert:

```sql
SELECT
    m.timestamp,
    m.source,
    m.metric_name,
    m.value AS raw_value,
    (m.value - b.baseline_value) / nullIf(b.baseline_std, 0) AS z_score
FROM aer_gold.metrics m
JOIN aer_gold.metric_baselines b
    ON m.metric_name = b.metric_name AND m.source = b.source
```

Dies ermöglicht der BFF API, sowohl Rohwerte als auch normalisierte Abweichungen zu liefern und dem Frontend die Wahl der angemessenen Vergleichsebene zu überlassen.

### 6.2 BFF-API-Erweiterungen

Die BFF API (`/api/v1/metrics`) sollte einen `normalization`-Abfrageparameter unterstützen:

- `?normalization=raw` — Roh-Metrikwerte zurückgeben (Standard, aktuelles Verhalten)
- `?normalization=zscore` — Z-Score-Abweichungen von Quellen-Baselines zurückgeben
- `?normalization=percentile` — Intra-Sprach-Perzentilränge zurückgeben

Der `/api/v1/metrics/available`-Endpunkt sollte Äquivalenz-Metadaten exponieren: welche Metriken für interkulturellen Vergleich validiert sind und auf welcher Äquivalenzebene.

### 6.3 Dashboard-Design-Implikationen

Das Dashboard darf niemals stillschweigend nicht-äquivalente Metriken auf demselben Chart platzieren. Bei der Anzeige interkultureller Vergleiche:

1. **Standard auf Ebene 1 (temporale Muster).** Publikationsfrequenz, temporale Verteilung und Ereignis-Reaktionszeiten über Kulturen hinweg zeigen. Dies sind sichere Vergleiche.

2. **Ebene 2 (Abweichungen) mit Beschriftung aktivieren.** Z-Score-Abweichungen mit klarer Beschriftung zeigen: „Dieses Chart zeigt Abweichungen von der Quellen-Baseline, nicht absolutes Sentiment." Die Baseline-Periode und Methode in Tooltips einschließen.

3. **Ebene 3 (absolute Werte) hinter Validierungsstatus sperren.** Absolute interkulturelle Metrikvergleiche nur für Metrikpaare anzeigen, die Einträge in der Metrik-Äquivalenz-Registry mit `validation_status = 'validated'` haben. Für nicht-validierte Paare eine Warnung anzeigen und die Abweichungsansicht als Alternative anbieten.

---

## 7. Offene Fragen für interdisziplinäre Kooperationspartner

### 7.1 Für vergleichende Sozialwissenschaftler und Methodologen

**F1: Welche von AĒRs geplanten Metriken sind Kandidaten für interkulturelle skalare Äquivalenz, und welche sind inhärent inkommensurabel?**

- Sentimentpolarität, Themenprävalenz, Entity-Salienz, Narrativrahmen — für jede: Gibt es einen realistischen Weg zur Etablierung skalarer Äquivalenz über Sprachen hinweg, oder sollte AĒR akzeptieren, dass diese Metriken nur intrakulturell sinnvoll sind?
- Deliverable: Eine Metrik-für-Metrik-Bewertung des Vergleichbarkeitspotenzials, mit empfohlenen Vergleichsebenen (temporal, Abweichung, absolut) für jede.

**F2: Welche Validierungsmethodik ist für die Etablierung interkultureller Metrikäquivalenz geeignet?**

- In der Surveymethodik wird Messinvarianz über Multi-Gruppen-Konfirmatorische Faktorenanalyse (MGCFA) getestet. Gibt es eine analoge Methodik für computationale Textmetriken?
- Deliverable: Ein Validierungsprotokoll für interkulturelle Metrikäquivalenz, adaptiert für computationale Diskursanalyse.

**F3: Wie sollte AĒR mit Metriken umgehen, die intrakulturell valide, aber interkulturell inkommensurabel sind?**

- Sollten solche Metriken nebeneinander mit expliziten Nicht-Äquivalenz-Warnungen angezeigt werden? Sollte das Dashboard kulturell kontextualisierte interpretive Rahmen anbieten? Sollten inkommensurable Metriken aus interkulturellen Ansichten gänzlich ausgeschlossen werden?
- Deliverable: Dashboard-Designrichtlinien für die Präsentation nicht-äquivalenter, aber ko-dargestellter Metriken.

### 7.2 Für Computerlinguisten

**F4: Welche Tokenisierungsstrategie ermöglicht die sinnvollsten sprachübergreifenden Wort-Level-Metriken?**

- Sollte AĒR sprachspezifische Tokenisierer verwenden (spaCy pro Sprache, jieba für Chinesisch, MeCab für Japanisch), einen universellen Subwort-Tokenisierer (SentencePiece, BPE) oder Zeichen-Level-Features?
- Deliverable: Eine vergleichende Evaluation von Tokenisierungsstrategien auf einem mehrsprachigen Nachrichtenkorpus, mit Auswirkung auf nachgelagerte Metrikvergleichbarkeit.

**F5: Können mehrsprachige Satz-Embeddings als sprachunabhängiger Merkmalsraum für interkulturellen Themenvergleich dienen?**

- Modelle wie XLM-RoBERTa und LaBSE produzieren sprachunabhängige Embeddings. Sind diese Embeddings hinreichend kulturell neutral für interkulturelles Topic-Alignment, oder betten sie englischzentrische semantische Strukturen ein?
- Deliverable: Eine Evaluation mehrsprachiger Embedding-Räume für interkulturelles Topic-Alignment, mit besonderer Aufmerksamkeit auf nicht-europäische Sprachen.

### 7.3 Für Kulturanthropologen und Area-Studies-Wissenschaftler

**F6: Welches sind für jede große Kulturregion die zentralen expressiven Normen, die Textmetrik-Baselines beeinflussen?**

- AĒR benötigt Dokumentation expressiver Konventionen, institutioneller Register und Kommunikationsnormen, die beeinflussen, wie Textmetriken in jedem kulturellen Kontext interpretiert werden sollten.
- Deliverable: Kulturelle Kalibrierungsprofile (2–3 Seiten jeweils) für dieselben Regionen, die in WP-003 F6 identifiziert wurden, fokussiert auf textebene Kommunikationsnormen statt Plattformwahl.

**F7: Welche Konzepte und Themen sind kulturspezifisch und sollten *nicht* interkulturell aligniert werden?**

- Manche Diskursthemen sind so tief in den lokalen kulturellen, historischen und politischen Kontext eingebettet, dass interkulturelles Alignment irreführend wäre. Die Identifikation dieser „nicht-vergleichbaren" Themen ist ebenso wichtig wie die Identifikation vergleichbarer.
- Deliverable: Eine Liste kulturell gebundener Diskurskonzepte pro Region, mit Erklärungen, warum interkulturelles Alignment unangemessen ist.

### 7.4 Für Statistiker und Datenwissenschaftler

**F8: Welche Baseline-Kalibrierungsperiode ist für stabile Z-Score-Berechnung pro Quelle erforderlich?**

- Wie viele Dokumente, über wie viele Tage, werden benötigt, um eine zuverlässige Baseline zu etablieren? Unterscheidet sich die erforderliche Kalibrierungsperiode nach Quellentyp (hochvolumiger Nachrichten-Feed vs. niedrigvolumige Regierungspressemitteilungen)?
- Deliverable: Eine statistische Analyse der Baseline-Stabilität als Funktion von Korpusgröße und temporalem Fenster, unter Verwendung von AĒRs Sonde-0-Daten als Pilot.

**F9: Wie sollte AĒR Normalisierungsunsicherheit durch die Aggregationspipeline propagieren?**

- Wenn Z-Scores aus unsicheren Baselines berechnet und über Quellen hinweg aggregiert werden, wie sollte die resultierende Unsicherheit quantifiziert und dargestellt werden?
- Deliverable: Ein Framework zur Unsicherheitspropagation für normalisierte Diskursmetriken, kompatibel mit ClickHouse-Aggregationsfunktionen.

---

## 8. Die ethische Dimension des Vergleichs

Interkultureller Vergleich ist nicht lediglich eine technische Herausforderung — er ist eine ethische. Die Geschichte der vergleichenden Sozialwissenschaft umfasst Episoden, in denen quantitative Vergleiche über Kulturen hinweg dazu dienten, Gesellschaften auf einer vermeintlichen Skala von „Entwicklung" oder „Modernität" zu ordnen — ein Erbe kolonialer Epistemologie, dem AĒR aktiv widerstehen muss.

**Die Ranking-Falle.** Ein Dashboard, das zeigt „Land A hat höheres positives Sentiment als Land B", lädt zum Ranking ein. Doch Ranking setzt ein normatives Rahmenwerk voraus, in dem höheres positives Sentiment „besser" ist. AĒRs Manifest verpflichtet sich zur Beobachtung statt Bewertung — zur Dokumentation von Resonanz, nicht zu deren Beurteilung. Das Dashboard-Design muss der Versuchung widerstehen, Vergleich als Ranking zu präsentieren.

**Die Universalismus-Falle.** Ein interkultureller Metrikvergleich behauptet implizit, dass das gemessene Konstrukt universell ist — dass „Sentiment" in allen Kulturen dasselbe Ding ist. Dieser Anspruch ist empirisch anfechtbar und muss als Hypothese behandelt werden, nicht als Annahme. Die Metrik-Äquivalenz-Registry (§5.2) ist der architektonische Mechanismus, um diesen Anspruch explizit und falsifizierbar zu machen.

**Die Defizit-Falle.** Der Vergleich von Metriken über Kulturen hinweg birgt das Risiko, Differenz als Defizit zu rahmen. Wenn der digitale Diskurs einer Kultur niedrigere „Diversität" zeigt (wie auch immer gemessen), könnte dies eine kohäsive Öffentlichkeit widerspiegeln, ein eingeschränktes Medienumfeld oder ein Messartefakt — diese Erklärungen tragen grundverschiedene normative Valenzen. AĒRs Progressive-Disclosure-Prinzip dient als Schutzmaßnahme: Der Analyst kann stets vom Aggregatvergleich in den kulturellen Kontext hineinbohren.

Die fundamentale ethische Verpflichtung ist: **AĒR vergleicht, um zu verstehen, nicht um zu ordnen.** Die Architektur muss diese Verpflichtung durch Design durchsetzen — indem Normalisierung explizit, Äquivalenzansprüche falsifizierbar und kultureller Kontext stets zugänglich gemacht werden.

---

## 9. Referenzen

- Bianchi, F., Terragni, S. & Hovy, D. (2021). „Cross-lingual Contextualized Topic Models with Zero-shot Learning." *Proceedings of EACL 2021*.
- Buzan, B., Wæver, O. & de Wilde, J. (1998). *Security: A New Framework for Analysis*. Lynne Rienner.
- Hall, E. T. (1976). *Beyond Culture*. Anchor Books.
- Hao, S. & Paul, M. J. (2018). „Lessons from the Bible on Modern Topics: Low-Resource Multilingual Topic Model Evaluation." *Proceedings of NAACL 2018*.
- Matsumoto, D. (1990). „Cultural Similarities and Differences in Display Rules." *Motivation and Emotion*, 14(3), 195–214.
- Sartori, G. (1970). „Concept Misformation in Comparative Politics." *American Political Science Review*, 64(4), 1033–1053.
- van de Vijver, F. J. R. & Leung, K. (1997). *Methods and Data Analysis for Cross-Cultural Research*. SAGE.

---

## Anhang A: Zuordnung zu AĒR Offene Forschungsfragen (§13.6)

| §13.6 Frage | WP-004 Abschnitt | Status |
| :--- | :--- | :--- |
| 4. Interkulturelle Vergleichbarkeit | §2–§6 (vollständige Behandlung) | Adressiert — Framework, Normalisierungsstrategien und Forschungsfragen vorgeschlagen |
| 3. Metrik-Validität | §3.2 (sprachübergreifendes Sentiment), §3.3 (NER-Ontologien) | Querverweis zu WP-002 |
| 5. Temporale Granularität | §4.3 (temporale Rhythmen über Kulturen) | Teilweise adressiert — WP-005 gewidmet |
| 1. Sondenauswahl | §5 (etisch/emische Metriken als Erweiterung von WP-001) | Querverweis zu WP-001 |

## Anhang B: Vergleichsebenen-Entscheidungsmatrix

| Forschungsfragentyp | Empfohlene Ebene | Normalisierung | Anforderungen |
| :--- | :--- | :--- | :--- |
| „Wann erreicht Diskurs seinen Höhepunkt über Kulturen hinweg?" | Ebene 1 (temporal) | Nicht erforderlich | Kulturelles Kalenderwissen |
| „Hat sich das Sentiment sowohl in DE als auch in JP nach Ereignis X verschoben?" | Ebene 2 (Abweichung) | Z-Score pro Quelle | Pro-Quellen-Baseline-Kalibrierung |
| „Ist deutscher Diskurs polarisierter als japanischer?" | Ebene 3 (absolut) | Instrumentharmonisierung | Validierte skalare Äquivalenz, Expertenbegutachtung |
| „Welche Themen dominieren in jeder Kultur?" | Nur intrakulturell | N/A | Pro-Sprache-Topic-Modelle, menschliches Alignment |
| „Tauchen dieselben Entitäten über Kulturen hinweg auf?" | Ebene 1 (Ko-Okkurrenz) | Entity Linking zu gemeinsamer KB | Mehrsprachiges Entity Linking, Ontologie-Alignment |