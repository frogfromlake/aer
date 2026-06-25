# WP-007: Erhebungsvollständigkeit und quellenübergreifende Fairness

> **Reihe:** AĒR Wissenschaftliche Methodik — Arbeitspapiere
> **Status:** Entwurf — offen für interdisziplinäre Begutachtung
> **Datum:** 22.06.2026
> **Abhängig von:** WP-001 (funktionaler Sondenkatalog), WP-004 (interkulturelle Vergleichbarkeit), WP-006 (Reflexivität & Ethik)
> **Architekturkontext:** ADR-028 (einzelner konfigurierbarer Web-Crawler), ADR-031 (DiscoveryProtocol für mehrkanalige Quellenerkennung), ADR-039 (Negativraum), Manifest §„unverstellter Spiegel", Qualitätsziel 1 (Wissenschaftliche Integrität)
> **Lizenz:** [CC BY-NC 4.0](https://creativecommons.org/licenses/by-nc/4.0/) — © 2026 Fabian Quist

---

## 1. Zielsetzung

AĒR vergleicht Diskurse *über* Quellen, Sprachen und Kulturen hinweg. Jeder solche Vergleich beruht auf einer stillschweigenden Voraussetzung: dass das Korpus, das AĒR für eine Quelle vorhält, eine **getreue und vergleichbare Stichprobe dessen ist, was diese Quelle tatsächlich veröffentlicht hat**. Dieses Arbeitspapier macht diese Voraussetzung explizit, zeigt, warum sie fragil ist, und definiert das Modell — Vollständigkeitsmessung, Fairness-Prüfung und Offenlegung —, das sie ehrlich hält, während das System von zwei Sonden auf Hunderte skaliert.

Die motivierende Beobachtung ist konkret. In einem kontrollierten Einzelfenster-Crawl (21.06.2026) lieferte der französische öffentlich-rechtliche Sender `franceinfo` etwa das 1,35-Fache des Dokumentenvolumens des deutschen Flaggschiffs `tagesschau`. Dieses Verhältnis ist entweder eine **reale Tatsache über die Diskurslandschaft** (eine große nationale Multimedia-Redaktion veröffentlicht tatsächlich mehr als eine fokussierte Flaggschiff-Marke) oder ein **Artefakt der Art, wie AĒR crawlt** (wir erfassen eine Quelle vollständiger als die andere oder übererfassen Nicht-Artikel aus einer von ihnen). Allein aus der Zahl sind die beiden nicht zu unterscheiden — und sie haben gegensätzlichen epistemischen Status. Das Erste ist ein Signal, das es zu zeigen lohnt; das Zweite ist eine Verzerrung, die jeden nachgelagerten Vergleich korrumpiert, während es *wie* ein Signal aussieht.

Die Frage, die dieses Papier beantwortet, lautet daher nicht „wie crawlen wir alles?", sondern: **wie weist AĒR nach, dass ein gemessener Unterschied zwischen Quellen die Welt ist und nicht das Instrument?**

---

## 2. Das Validitätsproblem: Durchsatz als Signal oder Artefakt

Ein Korpusgrößen- oder Durchsatzunterschied zwischen zwei Quellen kann aus vier verschiedenen Ursachen entstehen, von denen nur eine ein valides Diskurssignal ist:

1. **Echte Publikationsrate** — die Quelle veröffentlicht wirklich mehr. *Valides Signal.*
2. **Untererfassung der stilleren Quelle** — ein defekter oder nicht deklarierter Discovery-Kanal, ein Paginierungs-Abbruch, ein zu aggressiver URL-/Inhaltsfilter, ein Deduplizierungs-Falschtreffer oder Robots-/Throttle-Verluste lassen eine Quelle kleiner erscheinen, als sie ist. *Artefakt.*
3. **Übererfassung der lauteren Quelle** — ein zu schwacher Filter, Nicht-Artikel-Seiten (Live-Blogs, Fotogalerien, Medien-Stubs, Listenseiten), Locale-Spiegel-Leckage oder Doppelzählung blähen eine Quelle auf. *Artefakt.*
4. **Asymmetrische Erhebungsmethode** — eine Quelle stellt einen tiefen Archiv-Walker bereit, eine andere nur einen flachen News-Feed; der Unterschied ist real, spiegelt aber die *Publisher-Infrastruktur* wider, nicht das Diskursvolumen. *Teilweise valide, muss als Parameter offengelegt werden.*

Die Ursachen 2 und 3 sind das gefährliche Paar: Sie sind **still** (nichts wirft einen Fehler), sie **tarnen sich als Signal** (ein kleineres Korpus liest sich als stillerer Diskurs) und sie **akkumulieren mit der Skalierung** (jede ohne Vollständigkeitsprüfung hinzugefügte Sonde fügt eine weitere potenzielle Asymmetrie hinzu). Sie sind das erhebungsseitige Pendant zu der Selektionsverzerrung, vor der WP-006 §3 und WP-001 warnen, wirken aber eine Ebene früher — nicht „welche Quellen haben wir ausgewählt?", sondern „haben wir die ausgewählten Quellen getreu erfasst?".

Der Fall franceinfo/tagesschau ist gerade deshalb lehrreich, weil sich die Lücke unter kontrollierten Bedingungen *verkleinerte* (ein früherer Backfill legte ≈ 2,3:1 nahe; ein fairer Einzelfenster-Crawl ergab ≈ 1,35:1). Der Unterschied zwischen diesen beiden Verhältnissen war kein Diskurs — er war Akkumulationsasymmetrie im Erhebungsprozess. Ohne ein Vollständigkeitsmodell wäre dieses artefaktische 0,95-Fache als Signal gelesen worden.

---

## 3. Die Unmöglichkeit wörtlicher Vollständigkeit — und was an ihre Stelle tritt

Es ist verlockend, das Ziel als „100 % jeder Quelle crawlen" zu setzen. Dieses Ziel ist **inkohärent**, und es direkt zu verfolgen erzeugt aus drei Gründen schlechtere Wissenschaft:

1. **Es existiert keine Ground Truth.** Ein Publisher legt keine überprüfbare Zählung „jedes jemals von uns veröffentlichten Artikels" offen. Sitemaps sind partiell und hinken hinterher; RSS-Feeds sind fensterbegrenzt; Archive paginieren inkonsistent. Es gibt keinen Nenner, gegen den 100 % nachgewiesen werden könnten.
2. **Vollständigkeit ist zeit- und kanalgebunden.** „Vollständig" hat nur Bedeutung relativ zu einem Zeitraum (dem gleitenden Fenster) und den Kanälen, die ein Publisher tatsächlich offenlegt. Eine Quelle, die nur über einen für uns unerreichbaren Feed publiziert, ist nicht „unvollständig gecrawlt" — sie ist *strukturell unbeobachtbar*, was eine andere (und offenlegbare) Tatsache ist.
3. **Die Jagd nach 100 % lädt zu adversarialem Crawling ein.** Bot-Wälle zu umgehen, robots.txt zu ignorieren oder gerenderte SPAs auszulesen, um „alles zu bekommen", verletzt die im Manifest verankerte nicht-adversariale, standardmäßig höfliche Haltung (ADR-028). Vollständigkeit darf niemals mit Höflichkeit erkauft werden.

AĒR ersetzt daher *wörtliche* Vollständigkeit durch **gemessene, abgeglichene, offengelegte Vollständigkeit**:

> Eine Vollständigkeitsaussage ist ein **Verhältnis mit einem Nenner**, auf **Drift** überwacht und **offengelegt**, wo immer die Daten gezeigt werden. Eine Quelle wird nie als „vollständig gecrawlt" behauptet; sie wird als „in diesem Zeitraum zu N % vollständig gegenüber ihren deklarierten Kanälen" ausgewiesen, wobei der nicht-erfasste Rest als Negativraum benannt wird.

Diese Umrahmung verwandelt ein unbeweisbares Absolutum in eine messbare, falsifizierbare, ehrliche Größe — genau der Schritt, den WP-002 für die Metrikvalidität und WP-006 für die reflexive Provenienz vollzieht.

---

## 4. Ein Vier-Ebenen-Modell

Vollständigkeit und Fairness sind nicht eine Frage, sondern vier, die auf verschiedenen Ebenen wirken. Sie zu vermengen ist die Quelle der meisten Verwirrung (z. B. die falsche Intuition, „fair" bedeute „gleiche Korpusgröße").

### 4.1 Ebene 1 — Vollständigkeit (vertikal: haben wir die gesamte Ausgabe *dieser* Quelle erfasst?)

Die vertikale Frage, pro Quelle, pro Kanal, pro Zeitraum: `Vollständigkeit = erfasst / deklariert`.

Der **deklarierte Nenner** ist die eigene Angabe des Publishers darüber, was er im Zeitraum angeboten hat — gezählt *vor* den Filtern von AĒR: die Zahl der Sitemap-Einträge, deren `<lastmod>` ins Fenster fällt, die Zahl der RSS-Einträge im Fenster, die Paginierungstiefe eines Archivindex. Das ist verschieden von dem, was AĒR derzeit erfasst. Heute protokolliert der Crawler `urls_discovered` (die Zahl *nach* Anwendung des Zeitfensterfilters innerhalb jedes Kanals) und `urls_after_dedup` und vergleicht sie gegen eine **handgesetzte `expected_floor_per_run`**-Heuristik (ADR-031, `crawler_discovery_runs` / `crawler_discovery_alerts`). Diese Heuristik fängt grobe Unterschreitungen ab, kann aber kein Vollständigkeits-*verhältnis* ausdrücken, weil sie keinen gemessenen Nenner hat — nur eine Schätzung.

Die Abhilfe besteht darin, den **deklarierten Nenner zu messen** — als erstklassiges Telemetriefeld, das den bestehenden Alarm bei zwei aufeinanderfolgenden Unterschreitungen in einen Drift-Detektor gegen eine reale Zahl statt gegen eine Schätzung verwandelt.

### 4.2 Ebene 2 — Fairness (horizontal: sind die *Regeln* über Quellen hinweg symmetrisch?)

Fairness ist die am meisten missverstandene Ebene. **Fairness ist nicht gleiche Korpusgröße.** Zwei Quellen auf gleiches Volumen zu zwingen würde *die Realität verfälschen* — es würde das echte Publikationsraten-Signal (Ursache 1) löschen, das zu erfassen AĒR existiert. Das ist das Gegenteil eines unverstellten Spiegels.

Fairness ist **Symmetrie der Methode**: Jede Quelle unterliegt denselben *Klassen* von Aufnahmeregeln (derselben rein-technischen Filterung gemäß WP-006 §3, demselben Vollständigkeitsstandard, derselben Offenlegungspflicht), und jede **Asymmetrie der Erhebungsinfrastruktur** (Ebene-1-Ursache 4) wird dokumentiert und auf ihre Verzerrungsrichtung hin bewertet, statt versteckt zu werden. Der datumsindizierte Archiv-Walker der tagesschau, die News-Sitemap von franceinfo und die RSS-only-Oberfläche des Élysée sind keine Fairness-Verletzung — aber die Asymmetrie ist ein *erfasster struktureller Verzerrungsparameter* (bereits teilweise in der `bias_assessment.md` von Sonde 0, Struktureller Bias #8, festgehalten), kein unsichtbarer. Ein Fairness-Audit stellt eine Frage: **erhält irgendeine Quelle stillschweigend eine strengere oder laxere Regel als ihre Peers?**

### 4.3 Ebene 3 — Übererfassung (Reinheit: ist jedes erfasste Dokument ein echter Artikel?)

Das Spiegelbild von Ebene 1. Eine Quelle, deren Volumen durch Nicht-Artikel aufgebläht ist — Live-Blog-Stubs, Foto-/Video-Seiten, Listen-/Indexseiten, Locale-Spiegel-Duplikate —, ist genauso verzerrt wie eine mit Lücken, nur in die entgegengesetzte Richtung. Das Signal ist die quellenspezifische **Extraktions-Erfolgsrate** und **Nicht-Artikel-Rate**. (Konkret: Die Validierung vom 21.06.2026 fand tagesschau-Seiten, die mit einer Body-`word_count` von 4–40 bis ins Gold gelangten, weil der `min_word_count`-Filter des Crawlers auf der *Roh-HTML*-Wortzahl operiert — als 404-/Stub-Schutz —, während an der Silver-Grenze keine Mindestlänge für den Artikeltext existiert.)

Die maßgebliche Invariante ist **DISCLOSE-NEVER-COERCE** (ADR-039): dünner oder Nicht-Artikel-Inhalt wird entweder *mit* einer offengelegten Regel gefiltert oder als Negativraum ausgewiesen — er wird **nie still verworfen** (was eine Erhebungslücke verbergen würde) und **nie still gezählt** (was das Volumen aufblähen würde).

### 4.4 Ebene 4 — Robustheit der Vergleichsebene (bereits gebaut — hier der Vollständigkeit halber benannt)

Selbst ein perfekt gemessener, echt realer Größenunterschied darf nie naiv verglichen werden. AĒR erzwingt dies bereits auf der Analyseebene: z-Score-/Perzentil-Normalisierung, die interkulturelle Äquivalenzschranke (WP-004), die Merged-vs-Split-Kompositionsverweigerung für skalierte/intensive Metriken und k-Anonymität. Die Ebenen 1–3 *speisen* Ebene 4: Ein Vollständigkeitsverhältnis sagt der Vergleichsebene, **wie sehr sie jedem Korpus trauen darf**. Eine quellenübergreifende Überlagerung, gezeichnet über zwei Korpora mit 60 % und 98 % Vollständigkeit, erhebt einen Anspruch, den ihre Daten nicht tragen können — und die Vollständigkeitszahl ist es, die eine Leserin (oder eine künftige Schranke) das sehen lässt.

---

## 5. Der Abgleichmechanismus

Der operative Kern ist ein **Trichter** pro Quelle, pro Kanal, pro Lauf, dessen jede Stufe gezählt und zuordenbar ist:

```
declared (Publisher-Inventar im Fenster)
   └─▶ discovered (Kanal hat es im Fenster aufgetan)
          └─▶ after_dedup (eindeutig über Kanäle hinweg)
                 └─▶ passed_filters (url_filter + content_filter)
                        └─▶ fetched (HTTP-Erfolg, kein 304-unverändert)
                               └─▶ extracted (trafilatura-Body ≥ Schwelle)
                                      └─▶ Gold (im analytischen Speicher angekommen)
```

`Vollständigkeit = erfasst / deklariert` ist das Verhältnis von oben nach unten; der *Abfall an jeder Stufe* ist die Diagnose. Die Trichter-Asymmetrie vom 21.06.2026 — franceinfo `discovered 526 → erfasst 395` (75 %) gegenüber tagesschau `284 → 282` (99 %) — ist genau die Art Signal, die das lesbar macht: Der franceinfo-Abfall ist *mutmaßlich* legitime Medienfilterung (`/replay-radio/`, `/replay-jt/`), **muss aber Kanal für Kanal bestätigt werden**, denn eine unerklärte Konversionslücke von 24 Punkten zwischen zwei gleichrangigen EA-Quellen ist von einer Filterverzerrung nicht zu unterscheiden, solange sie nicht zugeordnet ist.

Zwei Entwurfsbeschränkungen folgen aus der übrigen AĒR-Architektur:

- **Der Nenner wird höflich gemessen oder gar nicht.** Das Zählen des deklarierten Inventars nutzt die Kanäle, die der Publisher ohnehin offenlegt (Sitemap-Eintragszahlen, Feed-Längen) — niemals zusätzliches adversariales Sondieren. Wo ein wahrer Nenner nicht erlangbar ist, wird die Vollständigkeit als *unbestimmt* (Negativraum) ausgewiesen, nicht als 100 % angenommen.
- **Drift, nicht absolute Schwellen, löst Alarme aus.** Die Vollständigkeit einer Quelle wird gegen ihre eigene gleitende Baseline verglichen (die bestehende Zwei-aufeinanderfolgende-Läufe-Semantik in `crawler_discovery_alerts`), sodass eine echte ruhige Woche eines Publishers keinen Fehlalarm auslöst, ein gebrochener Kanal aber schon.

---

## 6. Der Onboarding-Vertrag (Erweiterbarkeit)

Das größte Einzelrisiko sind nicht die zwei aktuellen Sonden — es ist die *hundertste* Quelle, die ein künftiger Operator hinzufügt, der nicht das gesamte Verzerrungsmodell im Kopf hält. Vollständigkeit und Fairness müssen daher **strukturelle Eigenschaften des Akts des Hinzufügens einer Quelle** sein, kein einmal durchgeführtes und vergessenes manuelles Audit. Das ist die tragende Anforderung dieses Papiers.

Jede neue Quelle und Sonde trägt von Tag eins an einen **Vollständigkeitsvertrag**:

1. **Das Audit erfasst den Nenner.** `aer-audit-source` (ADR-031) inventarisiert bereits die Discovery-Kanäle eines Publishers; es wird erweitert, um auch die **deklarierte-Inventar-Baseline** pro Kanal aufzuzeichnen, sodass der quellenspezifische Floor ein *gemessener* Startpunkt ist statt einer handgetippten `expected_floor_per_run`-Schätzung.
2. **`add-a-source.md` erhält einen Vollständigkeitsschritt.** Die Onboarding-Checkliste verlangt: die Kanäle deklarieren, die der Publisher offenlegt; die deklarierte-Inventar-Baseline erfassen; die Drift-Schwelle setzen; und **jede Erhebungsmethoden-Asymmetrie gegenüber den bestehenden Peer-Quellen der Sonde dokumentieren** (Ebene 2) im Sonden-Dossier.
3. **Der Ehrlichkeitsdurchgang von `add-a-probe.md` prüft Symmetrie.** Schritt G („Honesty pass") erhält eine explizite Vollständigkeits-/Fairness-Prüfung: Werden die Quellen dieser Sonde unter symmetrischen Regeln erhoben, und sind ihre Asymmetrien dokumentiert und bewertet?

Die Kostendisziplin von WP-006 §3 gilt weiterhin: Der Vertrag fügt *Messung und Offenlegung* hinzu, niemals redaktionelle Selektion. Er sagt einem Operator nicht, welche Quellen zu wählen sind — er stellt sicher, dass das, was auch immer gewählt wird, getreu und vergleichbar erhoben wird.

---

## 7. Offenlegung als Negativraum

Vollständigkeit ist nur dann bedeutungsvoll, wenn sie *mit* den Daten in jede Ansicht wandert (ADR-039: „was AĒR nicht sieht, ist eine erstklassige Oberfläche"). Die Offenlegung erweitert bestehende Oberflächen, statt neue zu erfinden:

- **Das quellenspezifische Coverage-Panel** (`DiscoveryCoveragePanel`, das bereits `GET /api/v1/sources/{id}/discovery-coverage` konsumiert) erhält das Vollständigkeitsverhältnis und die Trichter-Aufschlüsselung neben der heutigen Beobachtet-vs-Floor-Ansicht.
- **Eine neue Negativraum-Klasse — `collection-completeness`** — tritt zu den sechs bestehenden Klassen der Taxonomie hinzu, ausgewiesen durch das einzige `NegativeSpaceBadge`-Token: „das Korpus dieser Quelle ist für den gewählten Zeitraum zu N % vollständig". Weil das Badge mit den Daten mitreist, kann eine Leserin nicht zwei Korpora nebeneinanderstellen, ohne die Vollständigkeit jedes von ihnen zu sehen — was der ganze Sinn ist.

Das schließt die reflexive Schleife, die WP-006 fordert: AĒR *versucht* nicht nur, vollständig zu sein; es **misst und zeigt, wie vollständig es tatsächlich ist**, und benennt den Rest, den es nicht sehen kann.

---

## 8. Nicht-Ziele (was dieses Papier ausdrücklich ablehnt)

- **Korpusgrößen anzugleichen.** Abgelehnt — es verfälscht das echte Publikationsraten-Signal (§4.2).
- **Wörtliche 100-%-Vollständigkeit zu verfolgen.** Abgelehnt als inkohärent und adversarial-einladend (§3).
- **Redaktionelle Steuerung, um Quellen zu „balancieren".** Abgelehnt gemäß WP-006 §3 — Vollständigkeit fügt Messung hinzu, nie Selektion.
- **Dünnen/Nicht-Artikel-Inhalt still zu verwerfen.** Abgelehnt gemäß DISCLOSE-NEVER-COERCE — er muss offengelegt oder ausgewiesen werden (§4.3).
- **Eine strukturell-unbeobachtbare Quelle als „0 % gecrawlt" zu behandeln.** Abgelehnt — Unbeobachtbarkeit (z. B. ein bot-verwalltes Portal, ADR-028) ist eine eigene, offengelegte Tatsache, kein Vollständigkeitsversagen.

---

## 9. Offene Fragen für interdisziplinäre Mitwirkende

1. **Für Umfragemethodiker und Statistikerinnen.** Gibt es einen prinzipiengeleiteten Weg, die eigene Unvollständigkeit eines deklarierten Nenners zu schätzen (die Sitemap des Publishers ist selbst ein partieller Rahmen)? Was ist die richtige Konfidenzdarstellung für ein Vollständigkeitsverhältnis, dessen Nenner unsicher ist?
2. **Für Computational Social Scientists.** Bei welcher Vollständigkeitsasymmetrie wird ein quellenübergreifender Vergleich ungültig statt bloß vorbehaltsbedürftig? Gibt es eine vertretbare Schwelle, die einen Vergleich *blockieren* (nicht nur annotieren) sollte?
3. **Für STS-Forschende (Fortsetzung von WP-006).** Erzeugt die Veröffentlichung quellenspezifischer Vollständigkeit eine neue Reaktivität — könnte ein Publisher seinen „Vollständigkeits-Score" durch Manipulation seiner Sitemap spielen? Ist Vollständigkeitsoffenlegung selbst performativ?
4. **Für Informationsdesign-Forschende.** Wie wird eine quellenspezifische Vollständigkeitszahl so gezeigt, dass sie die Interpretation informiert, ohne zu einer trügerischen „Datenqualitäts-Rangliste" zu werden, die die Ranking-Verzerrung wieder einführt, die AĒR andernorts vermeidet?

---

## 10. Umsetzung

Die Ingenieurarbeit, die dieses Papier operationalisiert — die Erfassung des deklarierten Nenners im Crawler und der Abgleich, das Vollständigkeitsfeld des BFF, das Dashboard-Panel und die Negativraum-Klasse `collection-completeness` sowie die Onboarding-Vertrag-Änderungen an `add-a-source.md` / `add-a-probe.md` / `aer-audit-source` — wird als eigene ROADMAP-Phase geführt. Das meiste davon ist **Vor-Deployment**-Arbeit: Es ist ein Datenqualitäts-Fundament, und weil es prägt, wie jede künftige Quelle hinzugefügt wird, muss es existieren, *bevor* der Sondenkatalog skaliert, nicht danach.

---

*WP-007 erweitert die AĒR-Methodikreihe von „wie sollen wir beobachten" (WP-001–005) und „was geschieht, weil wir beobachten" (WP-006) um eine dritte reflexive Frage: **wie getreu haben wir tatsächlich erfasst, was wir zu beobachten ausgezogen sind — und wie weisen wir es nach?***
