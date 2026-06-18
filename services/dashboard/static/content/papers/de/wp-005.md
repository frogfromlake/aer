# WP-005: Temporale Granularität von Diskursverschiebungen

> **Reihe:** AĒR Wissenschaftliche Methodik — Arbeitspapiere
> **Status:** Entwurf — offen für interdisziplinäre Begutachtung
> **Datum:** 07.04.2026
> **Abhängig von:** WP-002 (Metrik-Validität), WP-004 (Interkulturelle Vergleichbarkeit)
> **Architekturkontext:** ClickHouse Gold Layer (§5.1.4), Multi-Resolution Temporal Framework (§8.13), Data Lifecycle Management (§8.8), Temporal Extractor (Phase 42)
> **Lizenz:** [CC BY-NC 4.0](https://creativecommons.org/licenses/by-nc/4.0/) — © 2026 Fabian Quist

---

## 1. Zielsetzung

Dieses Arbeitspapier adressiert die fünfte offene Forschungsfrage aus §13.6: **Bei welcher temporalen Auflösung werden Diskursverschiebungen bedeutsam? Stunden (Eilmeldungen), Tage (Nachrichtenzyklen), Wochen (Politikdebatten), Monate (kulturelle Verschiebungen)?**

AĒRs Gold-Schicht speichert jede Metrik mit einem Pro-Dokument-Zeitstempel. Die BFF API aggregiert diese in 5-Minuten-Intervalle für das Dashboard (Phase 13 Downsampling). Doch die rohe temporale Auflösung der Daten sagt nichts über die temporale Auflösung von *Bedeutung*. Ein Sentimentwert, alle 5 Minuten auf einem Tagesschau-RSS-Feed berechnet, produziert eine zackige Zeitreihe, deren hochfrequente Oszillationen fast sicher Rauschen sind — das Ergebnis einzelner Artikel, die in das Beobachtungsfenster ein- und austreten — während ihre niederfrequenten Trends genuine Signale sich verschiebenden Diskurses enthalten können.

Die Frage der temporalen Granularität ist nicht rein statistisch. Sie ist tief verflochten mit der *Art* des beobachteten Diskursphänomens, dem *Typ* der Datenquelle und dem *kulturellen Kontext* des Diskurses. Eilmeldungen entfalten sich in Minuten; Nachrichtenzyklen operieren in Stunden; Politikdebatten entwickeln sich über Tage und Wochen; kulturelle Verschiebungen — die Episteme-Dimension von AĒRs DNA — manifestieren sich über Monate und Jahre. Eine einzelne temporale Auflösung kann nicht allen diesen Phänomenen dienen.

Dieses Papier kartiert die Beziehung zwischen temporaler Granularität und Diskursphänomenen, schlägt ein multiskalares temporales Rahmenwerk vor und identifiziert die architektonischen und methodologischen Anforderungen für dessen Implementierung innerhalb von AĒRs Pipeline.

---

## 2. Diskursphänomene und ihre temporalen Signaturen

### 2.1 Eine Taxonomie temporaler Skalen

Diskursphänomene operieren über mindestens fünf distinkte temporale Skalen. Jede Skala korrespondiert mit verschiedenen gesellschaftlichen Prozessen, erfordert verschiedene Metriken und verlangt verschiedene analytische Methoden.

**Skala 1: Ereignisreaktion (Minuten bis Stunden).** Wenn ein großes Ereignis eintritt — ein Terroranschlag, eine Naturkatastrophe, ein politischer Rücktritt, eine Zentralbankentscheidung — verschiebt sich die digitale Diskurslandschaft schnell. Publikationsfrequenz steigt sprunghaft an; Sentiment polarisiert sich; neue Entitäten treten in den Diskurs ein. Die temporale Signatur der Ereignisreaktion ist ein **scharfer Impuls** gefolgt von exponentiellem Abklingen. Auf Social-Media-Plattformen (die AĒR in zukünftigen Sonden beobachten kann) kann dieser Impuls auf Minutenebene gemessen werden. Auf RSS-Feeds (Sonde 0) ist die Auflösung gröber — institutionelle Medien reagieren innerhalb von Stunden, nicht Minuten.

Die analytische Frage auf dieser Skala ist *Reaktivität*: Wie schnell reagiert der Diskurs? Wie variiert die Reaktionslatenz nach Quellentyp, nach Plattform, nach Kultur? WP-003 bemerkte, dass verschiedene Plattformen verschiedene temporale Affordanzen haben (Minutenebene auf X/Twitter, Stundenebene auf RSS, Halbtagsebene bei traditionellen Printmedien). Ereignisreaktionsanalyse muss diese plattformspezifischen Latenzen berücksichtigen.

**Skala 2: Nachrichtenzyklus (Stunden bis Tage).** Der Nachrichtenzyklus ist eine kulturell konstruierte temporale Einheit. In westlichen 24-Stunden-Nachrichtenkulturen steigt eine Geschichte auf, erreicht ihren Höhepunkt und verblasst innerhalb von 24–48 Stunden. In Medienkulturen mit Morgen-/Abendausgabe-Rhythmen (Japan, Teile Europas) ist der Zyklus anders strukturiert. Der Nachrichtenzyklus bestimmt, wie lange ein Thema salient bleibt — und Salienz ist, was AĒRs Themenmetriken zu messen versuchen.

Auf dieser Skala sind die relevanten Metriken **Themenprävalenz** (welcher Anteil des Diskurses ist Thema X gewidmet?), **Entity-Salienz** (welche Akteure werden erwähnt?) und **Framing-Verschiebungen** (verändert sich das Framing eines Themas im Verlauf des Zyklus?). Stündliche oder halbtägliche Aggregation ist angemessen; substündliche Daten sind Rauschen.

**Skala 3: Agendadynamiken (Tage bis Wochen).** Medienagenden und öffentliche Aufmerksamkeit werden nicht allein von Ereignissen getrieben — sie werden durch institutionelle Rhythmen (Parlamentssitzungen, Quartalsberichte, Wahlkalender), durch redaktionelle Entscheidungen (investigative Serien, Themenwochen) und durch intermediale Agendasetzung (McCombs, 2004) geformt. Auf dieser Skala verschiebt sich die Frage von „Was ist passiert?" zu „Worauf achtet der Diskurs?"

Relevante Metriken sind **Themenpersistenz** (wie viele Tage bleibt ein Thema oberhalb einer Salienzschwelle?), **Agendadiversität** (wie konzentriert oder verteilt ist Aufmerksamkeit über Themen hinweg?) und **quellenübergreifende Konvergenz** (konvergieren verschiedene Quellen auf dieselbe Agenda?). Tägliche oder mehrtägige Aggregation ist angemessen.

**Skala 4: Politik- und öffentlicher Diskurs (Wochen bis Monate).** Politikdebatten, soziale Bewegungen und öffentliche Kontroversen entfalten sich über Wochen und Monate. Die deutsche *Energiewende*-Debatte, der amerikanische Gesundheitsreform-Diskurs, die französischen Rentenreform-Proteste — dies sind anhaltende Diskursphänomene mit komplexer interner Dynamik: Phasen der Argumentation, Gegenargumentation, Framing-Wettbewerbe und Auflösung oder Erschöpfung.

Auf dieser Skala sind die relevanten Metriken **Sentimenttrends** (wird die Bewertung einer Politik über Wochen positiver oder negativer?), **Narrativevolution** (wie verschieben sich die dominanten Rahmen?) und **Polarisierungsdynamiken** (konvergiert oder divergiert der Diskurs?). Wöchentliche Aggregation mit gleitenden Durchschnitten ist angemessen; tägliche Fluktuationen sind auf dieser Skala typischerweise Rauschen.

**Skala 5: Kultureller Drift (Monate bis Jahre).** Die Episteme-Dimension — die Grenzen dessen, was gedacht und gesagt werden kann — verschiebt sich langsam. Die Normalisierung bestimmter Vokabulare, das Aufkommen neuer politischer Kategorien, die graduelle Umrahmung sozialer Themen — dies sind Mehrjahresphänomene, die longitudinale Beobachtung erfordern. Foucaults epistemische Verschiebungen operieren auf generationalen Zeitskalen; Shillers Narrativepidemien operieren auf Mehrjahreszyklen.

Auf dieser Skala sind die relevanten Metriken **Vokabularevolution** (welche Begriffe treten in das aktive Lexikon ein und aus?), **semantischer Drift** (verändern bestehende Begriffe ihre konnotative Bedeutung über die Zeit?), **Baseline-Verschiebungen** (verändert sich das „normale" Sentimentniveau?) und **strukturelle Veränderungen** (entstehen neue Themencluster oder lösen sich alte auf?). Monatliche oder quartalsweise Aggregation ist angemessen; jede höhere Auflösung wird von zyklischem Rauschen überlagert (Wochentagsmuster, saisonale Effekte, Feiertagseinbrüche).

### 2.2 Die Multiskalen-Hypothese

Die zentrale Hypothese dieses Papiers ist, dass **keine einzelne temporale Auflösung für alle Diskursmetriken angemessen ist und dass die optimale Auflösung von der Interaktion zwischen Metriktyp, Quellentyp und dem unter Beobachtung stehenden Diskursphänomen abhängt.**

Dies ist nicht lediglich ein Data-Engineering-Problem (wie man in ClickHouse effizient aggregiert) — es ist ein wissenschaftliches Problem (bei welcher Auflösung tritt Signal aus dem Rauschen hervor?). Hochfrequente Daten für ein langsames Phänomen zu präsentieren, erzeugt die Illusion von Volatilität, wo Stabilität herrscht. Niederfrequente Daten für ein schnelles Phänomen zu präsentieren, maskiert die Dynamiken, die das Phänomen ausmachen.

---

## 3. Signal, Rauschen und das Auflösungsproblem

### 3.1 Rauschquellen in AĒRs Zeitreihen

AĒRs Gold-Schicht-Zeitreihen enthalten Rauschen auf mehreren Frequenzen:

**Dokumentebenes Rauschen.** Einzelne Artikel haben idiosynkratische Sentimentscores, Entity-Kompositionen und Wortzahlen. Wenn das Beobachtungsfenster nur wenige Dokumente enthält (wie bei niedrigvolumigen RSS-Feeds), kann ein einzelner Artikel die Aggregatmetrik dominieren. Dies ist ein Kleinstichprobenproblem — das Signal-Rausch-Verhältnis verbessert sich mit dem Dokumentvolumen.

**Publikationsplan-Rauschen.** RSS-Feeds publizieren in unregelmäßigen Intervallen, die durch redaktionelle Rhythmen bestimmt werden, nicht durch Diskursdynamiken. Eine Lücke zwischen Publikationen ist keine Lücke im Diskurs — sie ist eine Lücke in der Beobachtung. Der temporale Verteilungs-Extractor (Phase 42) erfasst Publikationsmuster, aber die *Abwesenheit* von Daten zu einem gegebenen Zeitpunkt ist mehrdeutig: Sie kann „nichts ist passiert" oder „nichts wurde publiziert" bedeuten.

**Metrikextraktions-Rauschen.** Provisorische Extractors (WP-002) führen Messrauschen ein, das genuinem Signal überlagert ist. SentiWS-Sentimentscores haben unbekannte Fehlerverteilungen; spaCy NER hat variable Präzision pro Entity-Typ. Dieses Rauschen ist auf allen temporalen Skalen konstant, wird aber proportional größer, wenn Aggregationsfenster schrumpfen (weniger Dokumente zum Mitteln).

**Zyklisches Rauschen.** Wochentag-/Wochenend-Muster, Tageszeit-Muster und saisonale Muster (Ferienzeiten, Parlamentspausen, Sommerlöcher) erzeugen periodische Fluktuationen, die vorhersagbar sind, aber vor der Trendanalyse gefiltert oder modelliert werden müssen. AĒRs temporaler Extractor produziert `publication_hour` und `publication_weekday` — diese können als Features für zyklische Dekomposition dienen.

### 3.2 Temporale Dekomposition

Die Zeitreihenanalyse bietet etablierte Methoden zur Trennung von Signal und Rauschen auf verschiedenen Skalen. Die relevantesten für AĒR:

**Klassische Dekomposition.** Die Zeitreihe in Trend- (T), saisonale (S) und Restkomponenten (R) zerlegen: Y = T + S + R. Der Trend erfasst Skala-4–5-Phänomene; die saisonale Komponente erfasst zyklische Muster (Skala 2–3); der Rest erfasst Ereignisreaktionen (Skala 1) und Rauschen.

**STL-Dekomposition (Seasonal and Trend decomposition using Loess).** Eine robuste nichtparametrische Methode (Cleveland et al., 1990), die irreguläre Saisonalität handhabt und gegenüber Ausreißern resistent ist. Gut geeignet für AĒRs Daten, die irreguläre Publikationspläne und ereignisgetriebene Spitzen aufweisen können.

**Wavelet-Dekomposition.** Multi-Auflösungsanalyse, die eine Zeitreihe gleichzeitig in Komponenten verschiedener Frequenzbänder zerlegt. Dies ist theoretisch am besten mit der Multiskalen-Hypothese vereinbar — sie produziert eine *Auflösungspyramide*, bei der jede Ebene Phänomene auf einer anderen temporalen Skala erfasst.

**Change-Point-Erkennung.** Statistische Methoden (CUSUM, PELT, Bayesianische Change-Point-Erkennung), die Momente identifizieren, in denen sich die statistischen Eigenschaften der Zeitreihe abrupt ändern. Diese sind direkt relevant für die Erkennung von Diskursverschiebungen — Momente, in denen der Trend seine Steigung ändert, die Varianz sich ändert oder der Mittelwert verschiebt. Change Points auf verschiedenen Skalen korrespondieren mit verschiedenen Typen von Diskursphänomenen.

### 3.3 Das minimale bedeutsame Aggregationsfenster

Für jedes Metrik-Quellen-Paar existiert ein **minimales bedeutsames Aggregationsfenster** — das kleinste Zeitfenster, für das die Aggregatmetrik statistisch vom Rauschen unterscheidbar ist. Unterhalb dieses Fensters wird das Aggregat von dokumentebenem Variationsrauschen und Publikationsplaneffekten dominiert.

Das minimale Fenster hängt ab von:

- **Dokumentvolumen pro Fenster.** Eine Quelle, die 20 Artikel pro Tag publiziert, kann für die meisten Metriken tägliche Aggregation unterstützen. Eine Quelle, die 5 Artikel pro Tag publiziert, braucht mindestens 2–3 Tage für stabile Sentimentdurchschnitte. Die Beziehung ist ungefähr: minimales Fenster ≈ k / Publikationsrate, wobei k die Anzahl der Dokumente ist, die für eine stabile Schätzung benötigt werden (k ≈ 30 für Mittelwerte, gemäß der Heuristik des zentralen Grenzwertsatzes).

- **Metrikvarianz.** Hochvariante Metriken (Sentiment, wo einzelne Dokumente stark variieren) brauchen größere Fenster als niedrigvariante Metriken (Spracherkennungskonfidenz, die pro Quelle stabil ist).

- **Interessierende Effektgröße.** Wenn der Analyst nach großen Verschiebungen sucht (Krisenreaktion), genügen kleinere Fenster, weil das Signal stark ist. Wenn der Analyst nach subtilen Drifts sucht (Episteme-Level-Veränderungen), sind größere Fenster erforderlich, weil das Signal relativ zum Rauschen schwach ist.

AĒR sollte minimale bedeutsame Aggregationsfenster als Metrik-Metadaten berechnen und exponieren, um dem Dashboard zu ermöglichen, Analysten zu warnen, wenn sie eine temporale Auflösung wählen, die unterhalb der bedeutsamen Schwelle für ein gegebenes Metrik-Quellen-Paar liegt.

---

## 4. Kulturelle Temporalitäten

### 4.1 Zeit ist nicht kulturell neutral

WP-004 (§4.3) führte das Konzept kultureller temporaler Rhythmen ein. WP-005 vertieft diese Analyse.

Der gregorianische Kalender und die 24-Stunden-UTC-Uhr, die AĒR als temporalen Bezugsrahmen verwendet, sind nicht universell — sie sind westliche Instrumente, die eine spezifische temporale Ontologie aufzwingen. Andere temporale Rahmenwerke koexistieren:

**Religiöse Kalender.** Der islamische Kalender (lunar, 354–355 Tage) strukturiert soziale Rhythmen in muslimisch geprägten Gesellschaften. Der Ramadan verschiebt sich jährlich um ca. 10 Tage relativ zum gregorianischen Kalender und erzeugt damit ein wanderndes temporales Muster in Publikationsfrequenz und Diskursthemen. Der hebräische Kalender, der hinduistische Kalender (mit seinen regionalen Varianten), der chinesische landwirtschaftliche Kalender und der buddhistische Kalender strukturieren soziale Temporalität jeweils unterschiedlich.

**Politische Kalender.** Wahlzyklen, Legislaturperioden, Geschäftsjahre und nationale Gedenktage erzeugen temporale Strukturen, die nach Land variieren. Der amerikanische politische Kalender (Zwischenwahlen alle 2 Jahre, Präsidentschaftswahlen alle 4) erzeugt Diskursrhythmen, die sich vom deutschen Bundestagszyklus (4 Jahre, keine Zwischenwahlen) oder dem Sitzungsplan des japanischen Parlaments (ordentliche Sitzung Januar–Juni, außerordentliche Sitzungen nach Bedarf) unterscheiden.

**Medienkalender.** Publikationsrhythmen variieren nach Medienkultur. Manche Medienkulturen haben distinkte „Nachrichtensaisons" (das deutsche *Sommerloch* — wenn reduzierte politische Aktivität zu leichten Nachrichten führt). Manche Kulturen haben mediale Sperrfristen um Wahlen (Frankreichs 24-Stunden-Vorwahlschweigen). Manche Kulturen haben zensurgetriebene temporale Muster (verstärkte Inhaltsentfernung vor politisch sensiblen Jahrestagen in China).

### 4.2 Ereignisreaktionsdynamiken über Kulturen hinweg

Wie schnell und wie intensiv verschiedene Kulturen auf Ereignisse reagieren, ist selbst eine kulturelle Variable. Vergleichende Ereignisreaktionsanalyse erfordert die Kontrolle für:

**Plattformvermittelte Reaktionslatenz.** Soziale Medien ermöglichen sofortige Reaktion; RSS reflektiert redaktionelle Verarbeitungszeit. Den Vergleich von Reaktionslatenz über Kulturen hinweg erfordert den Vergleich von gleich-für-gleich-Plattformtypen (WP-003).

**Institutionelle Reaktionsnormen.** Deutsche institutionelle Quellen tendieren zu gemessener, verzögerter Reaktion (offizielle Stellungnahmen brauchen Zeit). Amerikanische institutionelle Quellen reagieren oft schnell mit vorläufigen Stellungnahmen. Dieser Unterschied reflektiert institutionelle Kultur, nicht die Geschwindigkeit öffentlicher Reaktion.

**Aufmerksamkeitsdauer.** Wie lange hält eine Gesellschaft ihre Aufmerksamkeit auf ein Ereignis? Das Konzept des *nichibu* (日没, „Sonnenuntergang" — ein Thema, das aus der Aufmerksamkeit verschwindet) in der japanischen Medienwissenschaft legt kulturspezifische Aufmerksamkeits-Abklingkurven nahe. Amerikanische Medienstudien dokumentieren zunehmend kürzere Aufmerksamkeitszyklen, getrieben durch den Wettbewerb des 24-Stunden-Kabelnachrichtenfernsehens. Ob diese Unterschiede kulturell oder plattformgetrieben (oder beides) sind, ist eine offene Forschungsfrage.

### 4.3 Implikationen für AĒRs temporales Rahmenwerk

AĒRs temporales Rahmenwerk muss:

1. **Multiskalare Standardkonfiguration bieten.** Das Dashboard sollte niemals eine einzelne temporale Auflösung erzwingen. Stattdessen sollte es einen Auflösungswähler anbieten, der der Diskursphänomen-Taxonomie (§2.1) entspricht: Ereignisreaktion (Stunden), Nachrichtenzyklus (Tage), Agendadynamiken (Wochen), Politikdiskurs (Monate), kultureller Drift (Quartale/Jahre).

2. **Kulturell annotiert sein.** Temporale Anomalien (Einbrüche der Publikationsfrequenz, Sentimentverschiebungen) sollten vor kulturellen Kalendern interpretierbar sein. Dies erfordert einen kulturellen Kalender-Metadatendienst oder eine Annotationsschicht — eine Nachschlagetabelle, die Daten kulturell bedeutsamen Ereignissen pro Region zuordnet.

3. **Quellenadaptiv sein.** Das minimale bedeutsame Aggregationsfenster (§3.3) sollte pro Quelle berechnet und dem Dashboard exponiert werden, um zu verhindern, dass Analysten hochfrequentes Rauschen überinterpretieren.

---

## 5. Architektonische Implikationen

### 5.1 ClickHouse Multi-Auflösungs-Aggregation

AĒRs aktuelle BFF API verwendet eine einzelne Aggregationsstrategie: `toStartOfFiveMinute()` mit `avg()` (Phase 13). Dies ist für Echtzeit-Monitoring angemessen, aber unzureichend für multiskalare Analyse.

Die vorgeschlagene Erweiterung führt auflösungsspezifische Aggregation ein:

```sql
-- Skala 1: Ereignisreaktion (stündlich)
SELECT toStartOfHour(timestamp) AS ts,
       avg(value) AS value, source, metric_name
FROM aer_gold.metrics
WHERE timestamp BETWEEN {start} AND {end}
GROUP BY ts, source, metric_name
ORDER BY ts;

-- Skala 3: Agendadynamiken (täglich)
SELECT toStartOfDay(timestamp) AS ts,
       avg(value) AS value, source, metric_name
FROM aer_gold.metrics
WHERE timestamp BETWEEN {start} AND {end}
GROUP BY ts, source, metric_name
ORDER BY ts;

-- Skala 4: Politikdiskurs (wöchentlich)
SELECT toStartOfWeek(timestamp) AS ts,
       avg(value) AS value, source, metric_name
FROM aer_gold.metrics
WHERE timestamp BETWEEN {start} AND {end}
GROUP BY ts, source, metric_name
ORDER BY ts;

-- Skala 5: Kultureller Drift (monatlich)
SELECT toStartOfMonth(timestamp) AS ts,
       avg(value) AS value, source, metric_name
FROM aer_gold.metrics
WHERE timestamp BETWEEN {start} AND {end}
GROUP BY ts, source, metric_name
ORDER BY ts;
```

ClickHouses spaltenbasierte Speicherung und vektorisierte Ausführung machen diese Aggregationen selbst auf großen Datensätzen effizient. Für die initiale Implementierung sind keine Materialized Views erforderlich — Abfragezeitaggregation genügt. Materialized Views können später zur Performanceoptimierung eingeführt werden, falls Abfragelatenz zum Problem wird.

### 5.2 BFF-API-Erweiterung

Der `/api/v1/metrics`-Endpunkt der BFF API sollte einen `resolution`-Abfrageparameter akzeptieren:

- `?resolution=5min` — aktuelles Verhalten (Standard für Echtzeit-Monitoring)
- `?resolution=hourly` — Skala-1-Aggregation
- `?resolution=daily` — Skala-2–3-Aggregation
- `?resolution=weekly` — Skala-4-Aggregation
- `?resolution=monthly` — Skala-5-Aggregation

Der `rowLimit` OOM-Schutz (Phase 13) sollte pro Auflösung angepasst werden: breitere Aggregationsfenster produzieren weniger Zeilen, sodass das Limit für niedrigere Auflösungen gelockert werden kann.

### 5.3 Trend- und Anomalie-Metadaten

Um multiskalare Analyse im Dashboard zu unterstützen, sollte die BFF API berechnete Trend- und Anomalie-Metadaten neben Rohmetriken exponieren:

**Gleitende Durchschnitte.** Exponentiell gewichtete gleitende Durchschnitte (EWMA) bei mehreren Fenstergrößen berechnen. ClickHouse unterstützt Window-Funktionen, die diese effizient berechnen können.

**Change-Point-Indikatoren.** Change Points serverseitig mittels CUSUM oder ähnlichen Methoden vorberechnen und als Metadaten neben der Zeitreihe zurückgeben. Dies vermeidet, komplexe statistische Berechnungen an das Frontend zu schieben.

**Minimales bedeutsames Fenster.** Das berechnete minimale bedeutsame Aggregationsfenster pro Metrik-Quellen-Paar als Teil der `/api/v1/metrics/available`-Antwort zurückgeben, um dem Frontend zu ermöglichen, temporale Auflösungen unterhalb der Schwelle zu deaktivieren.

### 5.4 Speicherung und Auflösungsstufen

AĒRs aktuelle TTL-Richtlinie beträgt 365 Tage bei voller Auflösung. Für langfristige Analyse des kulturellen Drifts (Skala 5) müssen Daten über Jahre aufbewahrt werden — aber Per-Dokument-Metriken bei voller Auflösung über Jahre zu speichern, ist weder notwendig noch wirtschaftlich.

Eine gestufte Speicherungsstrategie:

| Alter | Auflösung | Speicherung |
| :--- | :--- | :--- |
| 0–30 Tage | Voll (pro Dokument) | `aer_gold.metrics` (aktuell) |
| 30–365 Tage | Stündliche Aggregate | `aer_gold.metrics_hourly` (Materialized View) |
| 1–5 Jahre | Tägliche Aggregate | `aer_gold.metrics_daily` (Materialized View) |
| 5+ Jahre | Monatliche Aggregate | `aer_gold.metrics_monthly` (Materialized View) |

ClickHouse Materialized Views können dieses Tiering automatisieren. Die BFF API wählt die angemessene Stufe basierend auf dem angeforderten Zeitbereich und der Auflösung und liefert transparent Daten aus der höchstauflösenden Tabelle, die den angeforderten Zeitraum abdeckt.

---

## 6. Die drei Pfeiler und ihre temporalen Signaturen

Jeder von AĒRs philosophischen Pfeilern (Kapitel 1, §1.2) korrespondiert mit einem distinkten temporalen Analysemodus:

### 6.1 Aleph: Synchrone Aggregation

Das Aleph-Prinzip — der einzelne Punkt, der alle anderen Punkte enthält — ist fundamental **synchron**: Es fragt, wie die Welt *gerade jetzt* aussieht. Das Aleph-Dashboard ist ein Echtzeit-Querschnitts-Schnappschuss: Worüber handelt der Diskurs heute? Welche Entitäten sind salient? Was ist der emotionale Ton?

Temporale Anforderung: nahezu Echtzeit bis stündliche Aggregation. Die Aleph-Ansicht ist der „Wetterbericht" des globalen Diskurses — sie zeigt aktuelle Bedingungen, nicht historische Trends.

### 6.2 Episteme: Diachrone Trendanalyse

Das Episteme-Prinzip — die Grenzen des Sagbaren — ist fundamental **diachron**: Es fragt, wie sich der Diskurs *über die Zeit verändert*. Die Erkennung von Verschiebungen dessen, was gedacht und gesagt werden kann, erfordert langfristige temporale Analyse — Wochen, Monate, Jahre.

Temporale Anforderung: wöchentliche bis monatliche Aggregation mit Trenderkennung. Die Episteme-Ansicht ist die „Klimaaufzeichnung" des globalen Diskurses — sie zeigt langsame strukturelle Veränderungen, die bei täglicher Auflösung unsichtbar sind.

Spezifische Episteme-relevante Analysen:

- **Vokabularemergenz.** Die Ersterscheinung und Adoptionskurve neuer Begriffe verfolgen (z. B. *„Wutbürger"* im deutschen politischen Vokabular, *„incel"* im englischen Diskurs, *„内卷"* (Involution) im chinesischen sozialen Vokabular).
- **Semantischer Drift.** Veränderungen im Ko-Okkurrenz-Kontext bestehender Begriffe über Monate/Jahre mittels temporaler Wort-Embeddings oder diachroner distributioneller Semantik verfolgen (Hamilton et al., 2016).
- **Overton-Window-Dynamiken.** Verfolgen, welche Themen und Positionen sich über Mehrmonate-Zeiträume vom Rand zum Mainstream des Diskurses bewegen (oder umgekehrt).

### 6.3 Rhizom: Propagationsdynamiken

Das Rhizom-Prinzip — nichtlineare, dezentrale Informationsverbreitung — ist fundamental **relational-temporal**: Es fragt, wie Muster *durch* Netzwerke *über* die Zeit propagieren. Dies ist der temporalkomplexeste Analysemodus, weil er die Verfolgung der Bewegung von Diskurselementen (Themen, Rahmen, Entitäten) über Quellen, Plattformen und Kulturen hinweg erfordert.

Temporale Anforderung: hochauflösend (stündlich) innerhalb von Ereignisreaktionsfenstern, kombiniert mit quellenübergreifender Lag-Analyse. Die Rhizom-Ansicht ist die „Epidemiologie" des Diskurses — sie verfolgt, wie Narrative sich von Quelle zu Quelle ausbreiten, die temporale Verzögerung zwischen Ersterscheinung und breiter Adoption, und die strukturellen Pfade des Informationsflusses.

Spezifische Rhizom-relevante Analysen:

- **Quellenübergreifende Propagation.** Wenn ein Thema erstmals in Quelle A zum Zeitpunkt t₁ und in Quelle B zum Zeitpunkt t₂ erscheint, tragen die Verzögerung (t₂ - t₁) und die Richtung der Propagation Information über die Struktur des Informationsökosystems. Institutionelle Quellen (RSS) können sozialen Medien hinterherhinken; aber in manchen Fällen brechen institutionelle Quellen Geschichten, die sich zu sozialen Medien propagieren. Richtung und Latenz der Propagation sind selbst Befunde.
- **Narrative Ansteckungskurven.** Shillers Rahmenwerk der Narrativökonomik verfolgt die Adoption von Narrativen als epidemische Kurven (S-förmige Adoption, exponentielles Abklingen). Die Anpassung dieser Kurven erfordert hochauflösende temporale Daten während der Wachstumsphase und niederauflösende Daten für die stationäre Phase und die Abklingphase.

---

## 7. Offene Fragen für interdisziplinäre Kooperationspartner

### 7.1 Für Zeitreihenanalysten und Statistiker

**F1: Welche temporale Dekompositionsmethode ist für AĒRs Diskurszeitreihen am besten geeignet?**

- Klassische Dekomposition, STL, Wavelet-Analyse oder eine Kombination? AĒRs Zeitreihen haben irreguläre Abtastung (RSS-Feeds publizieren nicht in festen Intervallen), potenzielle Strukturbrüche (Algorithmusänderungen, neue Quellen hinzugefügt) und mehrere überlappende Periodizitäten (täglich, wöchentlich, saisonal).
- Deliverable: Eine vergleichende Evaluation von Dekompositionsmethoden auf AĒRs Sonde-0-Daten, mit Empfehlungen für jede temporale Skala.

**F2: Wie sollte AĒR Change Points in Diskursmetriken berechnen?**

- Welcher Change-Point-Erkennungsalgorithmus (CUSUM, PELT, Bayesianisch) ist für AĒRs Datencharakteristiken (verrauscht, irregulär abgetastet, potenziell nicht-stationär) geeignet? Wie sollte die Signifikanz von Change Points bewertet werden — frequentistisch (p-Werte) oder Bayesianisch (Posterior-Wahrscheinlichkeit)?
- Deliverable: Eine Change-Point-Erkennungspipeline, evaluiert auf Sonde-0-Daten mit bekannten Ereignissen als Grundwahrheit.

**F3: Was ist das minimale bedeutsame Aggregationsfenster für jedes Metrik-Quellen-Paar in AĒRs aktueller Pipeline?**

- Unter Verwendung von Sonde-0-Daten (Tagesschau und Bundesregierung RSS) das minimale Aggregationsfenster berechnen, bei dem Sentiment-, Entity-Count- und Wortzahl-Metriken stabilisieren (Varianz des Mittelwerts unter eine Schwelle fällt).
- Deliverable: Empirische Minimalfenster für jede aktuelle Metrik, mit dokumentierter statistischer Methode zur Replikation auf zukünftigen Sonden.

### 7.2 Für Kommunikationswissenschaftler und Medienwissenschaftler

**F4: Wie unterscheiden sich Nachrichtenzyklen über Kulturen hinweg, und wie sollte AĒR diese Unterschiede berücksichtigen?**

- Der 24-Stunden-Nachrichtenzyklus ist nicht universell. Wie unterscheiden sich Nachrichtenpublikationsrhythmen zwischen den Medienkulturen, die AĒR beobachten wird? Was sind die zentralen temporalen Strukturen (Ausgabenzyklen, Nachrichtensaisons, Aufmerksamkeits-Abklingmuster)?
- Deliverable: Ein vergleichendes Medientemporalitätsprofil für die in WP-003 F6 identifizierten Kulturregionen.

**F5: Wie sollte AĒR genuine Diskursverschiebungen von saisonalen und zyklischen Effekten erkennen und unterscheiden?**

- Ein Sentimenteinbruch im August kann das deutsche *Sommerloch* widerspiegeln, keine genuine Diskursverschiebung. Wie sollte das System zyklische Effekte von strukturellen Veränderungen unterscheiden?
- Deliverable: Ein Katalog kulturspezifischer temporaler Muster (Feiertage, Mediensaisons, politische Zyklen) pro Region, formatiert als maschinenlesbare Kalendermetadaten zur Integration in das Dashboard.

### 7.3 Für Computational Social Scientists

**F6: Bei welcher temporalen Auflösung werden Topic-Model-Outputs stabil?**

- LDA und BERTopic sind empfindlich gegenüber der Korpusgröße. Wenn AĒR Topic Models auf täglichen, wöchentlichen und monatlichen Korpora aus denselben Daten ausführt, wie unterscheiden sich die entdeckten Topics? Bei welcher Korpusgröße stabilisieren sich Topics?
- Deliverable: Eine Korpusgrößen-Sensitivitätsanalyse für LDA und BERTopic auf deutschem Nachrichtentext, mit Empfehlungen für minimale Korpusgröße pro temporalem Fenster.

**F7: Wie sollte AĒR quellenübergreifende Propagationsdynamiken modellieren?**

- Granger-Kausalität, Transfer-Entropie und Kreuzkorrelation sind Standardmethoden zur Erkennung temporaler Lead-Lag-Beziehungen zwischen Zeitreihen. Welche Methode ist für Diskurspropagationsanalyse geeignet, angesichts AĒRs Datencharakteristiken?
- Deliverable: Eine Methodenevaluation für quellenübergreifende Propagationserkennung, getestet auf bekannten Propagationsereignissen in Sonde-0-Daten.

### 7.4 Für Digital-Humanities-Wissenschaftler

**F8: Wie sollte AĒR Foucaults epistemische Verschiebungen in temporalen Metriken operationalisieren?**

- Der Episteme-Pfeiler strebt an, Verschiebungen in den „Grenzen des Sagbaren" zu messen. Dies ist ein langfristiges, qualitatives Konzept. Kann es als quantitative temporale Metrik operationalisiert werden (Vokabularemergenzrate, semantische Driftgeschwindigkeit, Overton-Window-Breite), oder widersteht es fundamental der Quantifizierung?
- Deliverable: Ein Positionspapier zur Operationalisierbarkeit epistemischen Wandels, mit spezifischen Vorschlägen für temporale Metriken, die das Konzept annähern, ohne es zu reduzieren.

---

## 8. Referenzen

- Cleveland, R. B., Cleveland, W. S., McRae, J. E. & Terpenning, I. (1990). „STL: A Seasonal-Trend Decomposition Procedure Based on Loess." *Journal of Official Statistics*, 6(1), 3–73.
- Hamilton, W. L., Leskovec, J. & Jurafsky, D. (2016). „Diachronic Word Embeddings Reveal Statistical Laws of Semantic Change." *Proceedings of ACL 2016*.
- McCombs, M. (2004). *Setting the Agenda: The Mass Media and Public Opinion*. Polity.
- Shiller, R. J. (2020). *Narrative Economics: How Stories Go Viral and Drive Major Economic Events*. Princeton University Press.

---

## Anhang A: Zuordnung zu AĒR Offene Forschungsfragen (§13.6)

| §13.6 Frage | WP-005 Abschnitt | Status |
| :--- | :--- | :--- |
| 5. Temporale Granularität | §2–§6 (vollständige Behandlung) | Adressiert — multiskalares Rahmenwerk vorgeschlagen |
| 4. Interkulturelle Vergleichbarkeit | §4 (kulturelle Temporalitäten) | Querverweis zu WP-004 |
| 3. Metrik-Validität | §3 (Signal- und Rauschtrennung) | Querverweis zu WP-002 |

## Anhang B: Temporale Skalen-Referenz

| Skala | Phänomene | Auflösung | AĒR-Pfeiler | Beispielmetrik |
| :--- | :--- | :--- | :--- | :--- |
| 1 (Min–Stunden) | Ereignisreaktion | 5min–stündlich | Rhizom | Publikationsfrequenzspitze, Entity-Emergenz |
| 2 (Stunden–Tage) | Nachrichtenzyklus | Stündlich–täglich | Aleph | Themenprävalenz, Entity-Salienz |
| 3 (Tage–Wochen) | Agendadynamiken | Täglich–wöchentlich | Aleph / Episteme | Themenpersistenz, Agendadiversität |
| 4 (Wochen–Monate) | Politikdiskurs | Wöchentlich–monatlich | Episteme | Sentimenttrends, Narrativrahmen-Evolution |
| 5 (Monate–Jahre) | Kultureller Drift | Monatlich–quartalsweise | Episteme | Vokabularemergenz, semantischer Drift, Baseline-Verschiebung |
| Skalenübergreifend | Propagation | Stündlich (innerhalb Ereignis) + wöchentlich (strukturell) | Rhizom | Quellenübergreifende Verzögerung, Narrativ-Ansteckungskurven |