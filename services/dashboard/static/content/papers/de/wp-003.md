# WP-003: Plattform-Bias, algorithmische Verstärkung und die Erkennung nicht-menschlicher Akteure

> **Reihe:** AĒR Wissenschaftliche Methodik — Arbeitspapiere
> **Status:** Entwurf — offen für interdisziplinäre Begutachtung
> **Datum:** 07.04.2026
> **Abhängig von:** WP-001 (Funktionale Sondentaxonomie), WP-002 (Metrik-Validität)
> **Architekturkontext:** Crawler-Ökosystem (§5.2), Source Adapter Protocol (ADR-015), Manifest §III
> **Lizenz:** [CC BY-NC 4.0](https://creativecommons.org/licenses/by-nc/4.0/) — © 2026 Fabian Quist

---

## 1. Zielsetzung

Dieses Arbeitspapier adressiert die zweite offene Forschungsfrage aus §13.6: **Wie messen und korrigieren wir plattformspezifische algorithmische Verstärkung, Bot-Aktivität und demografische Verzerrung in gecrawlten Daten?**

AĒR strebt an, den „Digitalen Puls" der vernetzten Zivilisation zu beobachten (Manifest §I). Doch der Puls, den AĒR misst, ist kein unvermitteltes menschliches Signal — es ist ein Signal, das von den Plattformen, die es beherbergen, gefiltert, verstärkt, unterdrückt und umstrukturiert wurde. Jede digitale Plattform ist ein soziotechnisches System mit eigenen Affordanzen, Content-Moderationsrichtlinien, Empfehlungsalgorithmen, ökonomischen Anreizen und Governance-Strukturen. Diese Systeme übertragen menschlichen Diskurs nicht passiv; sie formen ihn aktiv.

Darüber hinaus wird ein wachsender Anteil von Online-Inhalten nicht von Menschen, sondern von automatisierten Accounts (Bots), koordinierten Einflussoperationen und generativen KI-Systemen produziert. Die Verpflichtung des Manifests, „die zugrundeliegenden menschlichen Impulse" zu beobachten (§III), erfordert von AĒR, zwischen organischem menschlichem Diskurs und synthetischem oder automatisiertem Inhalt zu unterscheiden — oder dies zumindest zu versuchen.

Dieses Papier kartiert die Landschaft plattforminduzierter Verzerrungen, untersucht die globale Vielfalt der Plattform-Ökosysteme, analysiert den aktuellen Stand der Forschung zu Bot- und synthetischer Inhaltserkennung und formuliert konkrete Forschungsfragen für die interdisziplinäre Zusammenarbeit. Es befasst sich mit Verzerrungen, die *stromaufwärts* von AĒRs Pipeline auftreten — bevor ein Dokument die Bronze-Schicht erreicht — im Gegensatz zu den Messverzerrungen, die in WP-002 adressiert werden und *innerhalb* der Pipeline während der Metrikextraktion auftreten.

---

## 2. Die Plattform als vermittelnde Infrastruktur

### 2.1 Plattformaffordanzen und Diskursstruktur

Plattformen sind keine neutralen Kanäle. Jede Plattform zwingt strukturelle Einschränkungen auf — Affordanzen — die Form und Inhalt des Diskurses prägen (Bucher & Helmond, 2018). Diese Affordanzen sind nicht lediglich technische Merkmale; sie sind Designentscheidungen mit soziologischen Konsequenzen.

**Zeichenlimits** formen die rhetorische Strategie. X/Twitters historisches 140-Zeichen-Limit (jetzt 280, oder für Premium-Nutzer effektiv unbegrenzt) incentivierte komprimierte, affektive und polarisierte Ausdrucksweise. Facebooks Abwesenheit von Zeichenlimits bei gleichzeitiger algorithmischer Priorisierung von Engagement incentiviert emotionale und spaltende Inhalte. Reddits verschachtelte Kommentarstruktur ermöglicht deliberative Argumentation, die auf X strukturell unmöglich ist. Telegrams Kanalmodell erzeugt Eins-zu-viele-Sendedynamiken, die sich sowohl von sozialen Medien als auch von traditionellen Medien unterscheiden. Jede Affordanz produziert eine andere *Art* von Text mit unterschiedlichen Sentimentprofilen, rhetorischen Strukturen und Informationsdichte.

**Sichtbarkeitsmechanismen** bestimmen, welche Inhalte beobachtet werden. Auf algorithmisch kuratierten Plattformen (Facebook, YouTube, TikTok, Instagram) hängt der Inhalt, auf den ein Crawler zugreifen kann, davon ab, was der Algorithmus an die Oberfläche bringt. Selbst beim Zugriff auf öffentliche Feeds oder APIs wird die Auswahl *welcher* Inhalt erscheint von engagement-optimierenden Algorithmen geprägt, deren Parameter proprietär, opak und ohne Vorankündigung änderbar sind. Auf chronologisch geordneten Plattformen (RSS, Mastodons Standard, viele Foren) ist der Selektionsbias anders, aber dennoch vorhanden: Aktualitätsbias, Publikationsfrequenz-Asymmetrien und redaktionelles Gatekeeping ersetzen algorithmische Kuration.

**Moderationsregime** filtern Inhalte vor der Beobachtung. Jede Plattform entfernt Inhalte gemäß ihrer Gemeinschaftsstandards — doch diese Standards variieren dramatisch über Plattformen, Jurisdiktionen und kulturelle Kontexte hinweg. Inhalte, die auf Telegram zulässig sind, können auf Facebook entfernt werden. Inhalte, die in einer Jurisdiktion moderiert werden (Deutschlands NetzDG), bleiben in einer anderen sichtbar. AĒR beobachtet den Diskurs nicht, den Plattformen unterdrücken, und diese Unterdrückung ist selbst eine Form von Bias: die *Abwesenheit* bestimmter Sprachmuster in gecrawlten Daten kann Plattformpolitik widerspiegeln statt gesellschaftlicher Einstellungen.

**API-Zugang als Forschungsengpass.** Seit 2023 haben große Plattformen den Forschungs-API-Zugang systematisch eingeschränkt oder verteuert (X's API-Preisumstrukturierung, Reddits API-Änderungen, Metas CrowdTangle-Abschaffung). AĒRs architektonische Entscheidung, eigenständige Crawler pro Quelle zu verwenden (§5.2), bedeutet, dass die Zugänglichkeit jeder Datenquelle eine unabhängige Einschränkung ist. RSS-Feeds — AĒRs Sonde-0-Datenquelle — umgehen dieses Problem vollständig, weil sie für den öffentlichen Konsum konzipiert sind. Aber RSS repräsentiert nur einen schmalen Ausschnitt der digitalen Diskurslandschaft (institutionell, redaktionell, unidirektional). Die Expansion zu reichhaltigeren Diskursräumen wird unweigerlich auf API-Restriktionen stoßen.

### 2.2 Die Asymmetrie beobachtbaren Diskurses

Nicht aller digitaler Diskurs ist gleichermaßen beobachtbar. Die Beobachtbarkeit eines Diskursraums hängt von drei orthogonalen Dimensionen ab:

**Technische Zugänglichkeit.** Bietet die Plattform öffentliche Endpunkte (RSS, öffentliche APIs, scrapbare Webseiten), oder sind Inhalte hinter Authentifizierung, Nur-App-Zugang oder Ende-zu-Ende-Verschlüsselung gesperrt? WhatsApp, Signal und private Telegram-Gruppen — bedeutende Diskursmedien in vielen Gesellschaften — sind ohne Teilnehmerzugang technisch unbeobachtbar, was ethische Bedenken aufwirft, die AĒR explizit vermeidet (keine nutzerbezogenen Daten, keine verdeckte Beobachtung).

**Rechtliche Zugänglichkeit.** Ist Crawling unter den Nutzungsbedingungen der Plattform, unter dem anwendbaren Datenschutzregime (DSGVO, CCPA, LGPD, PIPA) und unter den Gesetzen der relevanten Jurisdiktion zum Computerzugang gestattet? Rechtliche Einschränkungen sind nicht einheitlich — sie variieren nach Land, Plattform und Art der gesammelten Daten. AĒRs Sonde-0-Quellenauswahlkriterien (§13.8) priorisieren explizit Quellen ohne Nutzungsbedingungsbeschränkungen, doch dies begrenzt den beobachtbaren Raum auf institutionelle, staatliche und öffentliche Medienquellen.

**Ethische Zugänglichkeit.** Selbst wo technisch und rechtlich möglich, sollten einige Diskursräume nicht ohne informierte Zustimmung beobachtet werden. Private Foren, Selbsthilfegruppen, Minderheiten-Community-Räume und Plattformen, die von verwundbaren Bevölkerungsgruppen genutzt werden, erfordern ethische Prüfung, die über rechtliche Compliance hinausgeht. AĒRs Manifest verpflichtet sich zur Beobachtung statt Überwachung — doch die Grenze zwischen beiden ist kontextuell definiert und erfordert kontinuierliches ethisches Urteil.

Die Schnittmenge dieser drei Dimensionen erzeugt eine **Bias-Topologie**: Der Diskurs, den AĒR beobachten *kann*, ist eine nicht-zufällige Teilmenge des Diskurses, der *existiert*. Diese Teilmenge überrepräsentiert systematisch institutionellen, öffentlichen, englischsprachigen, auf westlichen Plattformen gehosteten Inhalt und unterrepräsentiert privaten, verschlüsselten, nicht-englischsprachigen und plattformgesperrten Diskurs. Dies ist kein Bug — es ist eine strukturelle Eigenschaft jedes digitalen Diskursobservatoriums. Die wissenschaftliche Verpflichtung besteht darin, diese Topologie explizit zu dokumentieren, nicht so zu tun, als existiere sie nicht.

---

## 3. Die globale Plattformlandschaft: Jenseits des westlichen Stacks

### 3.1 Plattformhegemonie und digitale Geopolitik

Das mentale Standardmodell „des Internets" in der westlichen Forschung ist eine Landschaft, die von einer Handvoll amerikanischer Plattformen dominiert wird: Google, Meta (Facebook/Instagram/WhatsApp), X/Twitter, Reddit, YouTube, TikTok (in chinesischem Besitz, aber global verbreitet). Dieses Modell ist nicht lediglich unvollständig — es ist aktiv irreführend für ein Projekt, das globalen Diskurs zu beobachten beabsichtigt.

Die globale Plattformlandschaft ist entlang sprachlicher, kultureller, politischer und regulatorischer Linien fragmentiert. AĒRs Expansion über Sonde 0 hinaus muss diese Fragmentierung berücksichtigen. Die folgende Kartierung ist illustrativ, nicht erschöpfend:

**Ostasien.** Chinas Internet ist ein eigenständiges Ökosystem hinter der Großen Firewall: WeChat (微信) dient als Super-App, die Messaging, soziale Medien, Zahlungsverkehr und öffentliche Informationen kombiniert; Weibo (微博) fungiert als Microblogging-Plattform mit eigener Zensurdynamik; Douyin (抖音, das chinesische TikTok) prägt visuellen Diskurs; Baidu Tieba (百度贴吧) bietet forenbasierte Diskussionen. Japans digitale Diskurslandschaft umfasst LINE (Messaging mit öffentlichen Accounts), Yahoo! Japan News-Kommentare, 2channel/5channel (anonyme Foren mit tiefgreifendem kulturellem Einfluss) und Hatena (Blogging). Südkoreas Landschaft zentriert sich auf Naver (Portal und Nachrichten), KakaoTalk (Messaging) und Community-Plattformen wie DC Inside und theqoo. Jede dieser Plattformen hat eigene Affordanzen, Moderationsregime und Nutzerdemografien, die strukturell unterschiedlichen Diskurs produzieren.

**Russland und der postsowjetische Raum.** VKontakte (VK) und Odnoklassniki bleiben dominante soziale Plattformen. Telegram erfüllt eine Doppelfunktion als Messaging-Plattform und de facto öffentliche Medieninfrastruktur, insbesondere seit 2022. Die RuNet-Governance beinhaltet staatlich gelenkte Inhaltsregulierung (Roskomnadzor), die beobachtbaren Diskurs auf fundamental andere Weise formt als westliche Content-Moderation.

**Süd- und Südostasien.** Indiens digitaler Diskurs wird von WhatsApp (Ende-zu-Ende-verschlüsselt, weitgehend unbeobachtbar), ShareChat (Hindi-sprachige soziale Plattform), Koo (indisches Microblogging, seit 2024 eingestellt — ein Beispiel für Plattform-Vergänglichkeit) und einer lebhaften vernakularsprachigen Blogosphäre über Dutzende von Sprachen geprägt. Indonesiens Diskurslandschaft umfasst eigenständige Nutzungen von Facebook, X und TikTok neben lokalen Foren wie Kaskus. Der politische Diskurs der Philippinen wird stark von Facebook geprägt, das für viele Nutzer als de facto Internet fungiert (Metas Free-Basics-Programm).

**Naher Osten und Nordafrika.** Plattformnutzungsmuster werden von Bedenken bezüglich staatlicher Überwachung geformt, was zu hoher Adoption verschlüsselter Nachrichtendienste (Telegram, Signal) neben öffentlichen Plattformen führt (X/Twitter bleibt bedeutsam für politischen Diskurs in den Golfstaaten und Iran). Al Jazeera, BBC Arabic und nationale Medien-RSS-Feeds repräsentieren die institutionelle Schicht, während Telegram-Kanäle und X-Accounts die Gegendiskursschicht repräsentieren. Die arabischsprachige Welt ist nicht monolithisch — Golfstaaten, die Levante, der Maghreb und Ägypten haben jeweils eigenständige digitale Kulturen.

**Subsahara-Afrika.** Mobile-first-Internetzugang prägt die Plattformadoption: WhatsApp und Facebook dominieren in vielen Märkten. Nigeria, Kenia und Südafrika haben eigenständige digitale Öffentlichkeiten. Facebook-Seiten lokaler Radiosender in Landessprachen fungieren als gemeinschaftliche Diskursplattformen. Das Fehlen dominanter lokaler Plattformen bedeutet, dass Diskurs auf globalen Plattformen stattfindet, aber von lokalen Kommunikationsnormen geformt wird — eine Form digitalen Code-Switchings, die die Analyse verkompliziert.

**Lateinamerika.** WhatsApp ist die dominante Kommunikationsplattform in der gesamten Region. X/Twitter erfüllt politische Diskursfunktionen in Brasilien, Argentinien und Mexiko. Die Telegram-Nutzung ist gewachsen, insbesondere in Brasilien. Regionale Medienökosysteme (Folha de São Paulo, El País América, Clarín) bieten RSS-zugänglichen institutionellen Diskurs, doch der Großteil des Diskurses findet auf globalen Plattformen statt, die auf lokal spezifische Weise genutzt werden.

### 3.2 Implikationen für AĒRs Crawler-Architektur

AĒRs Architekturmuster — eigenständige Crawler pro Quellentyp, die an eine einheitliche Ingestion API liefern (§5.2) — ist gut für diese heterogene Landschaft geeignet. Jede Plattform erfordert einen eigenen Crawler mit plattformspezifischer Logik für Authentifizierung, Paginierung, Rate Limiting und Datenextraktion. Das Source Adapter Protocol (ADR-015) stellt sicher, dass plattformspezifische Daten in `SilverCore` harmonisiert werden, ohne das Kernschema zu kontaminieren.

Die *Auswahl*, welche Plattformen gecrawlt werden, ist jedoch keine technische Entscheidung — sie ist eine wissenschaftliche, gesteuert durch WP-001s Funktionale Sondentaxonomie. Ein Telegram-Kanal, betrieben von einer iranischen Dissidentengruppe, und ein Bundesregierungs-RSS-Feed mögen beide die Diskursfunktionen „Subversion & Friktion" bzw. „Ressourcen- & Machtlegitimation" erfüllen, erfordern aber grundlegend verschiedene Crawler, unterschiedliche ethische Prüfung und unterschiedliche Bias-Dokumentation.

Die `SilverMeta`-Schicht muss plattformspezifische Metadaten erfassen, die Bias-Analyse ermöglichen:

- `platform_type`: Die Hosting-Plattform (z. B. `rss`, `twitter`, `telegram`, `weibo`)
- `access_method`: Wie die Daten gewonnen wurden (z. B. `public_api`, `rss_feed`, `web_scrape`, `academic_api`)
- `visibility_mechanism`: Wie die Plattform diesen Inhalt sichtbar machte (z. B. `chronological`, `algorithmic`, `editorial_curation`)
- `moderation_regime`: Bekannter Moderationskontext (z. B. `netzdg_applicable`, `great_firewall`, `unmoderated`)

Diese Felder beanspruchen nicht, Plattform-Bias zu *korrigieren* — sie dokumentieren die Bedingungen, unter denen die Daten produziert wurden, und ermöglichen nachgelagerter Analyse, diese zu berücksichtigen.

---

## 4. Algorithmische Verstärkung: Die unsichtbare Hand in den Daten

### 4.1 Was algorithmische Verstärkung für AĒR bedeutet

Wenn AĒR Diskurs von einer algorithmisch kuratierten Plattform beobachtet, beobachtet es nicht „was die Menschen sagen" — es beobachtet „was der Algorithmus aus dem, was die Menschen sagen, an die Oberfläche zu bringen gewählt hat." Diese Unterscheidung ist kritisch.

Empfehlungsalgorithmen optimieren auf Engagement-Metriken (Klicks, Shares, Kommentare, Wiedergabezeit). Ein substanzielles Forschungskorpus zeigt, dass engagement-optimierende Algorithmen systematisch Inhalte verstärken, die emotional erregend, polarisierend, empörungsauslösend oder neuheitssuchend sind (Brady et al., 2017; Bail et al., 2018; Huszár et al., 2022). Dies bedeutet, dass algorithmisch kuratierte Datenquellen einen systematischen **Negativitäts- und Extremitätsbias** tragen — nicht weil die Bevölkerung negativ oder extrem ist, sondern weil der Algorithmus auf Inhalte selektiert, die Reaktion provozieren.

Für AĒR erzeugt dies ein konkretes Messproblem: Wenn das System steigende Negativität im Diskurs einer algorithmisch kuratierten Quelle misst, kann es nicht bestimmen, ob:

1. Die Bevölkerung tatsächlich negativeres Sentiment äußert (gesellschaftliches Signal)
2. Der Algorithmus aktualisiert wurde, um negativere Inhalte zu verstärken (Plattformsignal)
3. Die Engagement-Dynamiken sich so verschoben haben, dass negativer Inhalt mehr Interaktion bekommt und daher mehr Sichtbarkeit (verhaltensbedingte Feedback-Schleife)

Diese drei Erklärungen produzieren identische Daten in der Gold-Schicht. Ihre Unterscheidung erfordert entweder plattforminterne Daten (über die AĒR nicht verfügt) oder sorgfältig entworfene kontrafaktische Vergleichsstrategien.

### 4.2 Kontrafaktische Strategien

**Plattformübergreifende Triangulation.** Wenn dasselbe Thema unterschiedliche Sentimentverteilungen auf algorithmisch kuratierten Plattformen (Facebook, YouTube) versus chronologisch geordneten Plattformen (RSS, Mastodon, Foren) produziert, kann die Divergenz auf algorithmische Verstärkung hindeuten. Dies erfordert, dass AĒR parallele Sonden über Plattformtypen hinweg für dieselbe Diskursfunktion und denselben kulturellen Kontext unterhält — eine direkte Anwendung von WP-001s Taxonomie.

**Temporale Diskontinuitätserkennung.** Algorithmische Änderungen werden oft abrupt ausgerollt. Wenn Sentiment- oder Themenverteilungen sich plötzlich ohne ein korrespondierendes reales Ereignis verschieben, kann die Verschiebung eine Algorithmusänderung der Plattform widerspiegeln statt eine Diskursveränderung. Die temporalen Metriken der Gold-Schicht (Publikationsfrequenz, temporale Verteilung) können als Anomaliedetektoren für diesen Zweck dienen.

**Volumen-Sentiment-Entkopplung.** Auf algorithmisch kuratierten Plattformen ist hochvolumiger Inhalt keine Zufallsstichprobe — er ist die Selektion des Algorithmus. Wenn Sentiment und Volumen korreliert sind (negativer Inhalt hat höheres Volumen), kann diese Korrelation algorithmisch statt organisch sein. Die Dekomposition von Trends in volumengewichtetes und ungewichtetes Sentiment kann diesen Effekt aufdecken.

**Quellendiversitätsmonitoring.** Wenn eine kleine Anzahl von Accounts oder Quellen den beobachteten Korpus einer Plattform dominiert, kann dies auf algorithmische Konzentration hindeuten statt auf Diskurskonsens. Das Tracking von Quellendiversitätsmetriken (Anzahl einzigartiger Autoren, Gini-Koeffizient des Publikationsvolumens pro Quelle) in der Gold-Schicht kann dieses Muster flaggen.

### 4.3 Der Sonderfall RSS und redaktionelle Plattformen

AĒRs Sonde 0 — deutsches institutionelles RSS — ist bemerkenswert frei von algorithmischem Verstärkungs-Bias. RSS-Feeds sind chronologisch geordnet, redaktionell kuratiert (durch menschliche Journalisten, nicht durch Algorithmen) und enthalten keine Engagement-Metriken. Dies macht RSS zu einer exzellenten Kalibrierungsbaseline, gerade weil es die auf sozialen Medien vorhandenen Verstärkungseffekte nicht aufweist.

Allerdings ist redaktionelle Kuration selbst ein Bias: Die redaktionellen Entscheidungen von Tagesschau-Journalisten darüber, welche Geschichten behandelt werden, wie sie gerahmt werden und welche Stimmen einbezogen werden, spiegeln institutionelle Prioritäten, journalistische Normen und das kulturelle Milieu des deutschen öffentlich-rechtlichen Rundfunks wider. Dies ist WP-001s „Ressourcen- & Machtlegitimation"-Funktion — eine spezifische Diskursfunktion, keine neutrale Baseline.

---

## 5. Nicht-menschliche Akteure: Bots, koordinierte Operationen und synthetischer Inhalt

### 5.1 Das Ausmaß des Problems

Die Aspiration des Manifests, „die zugrundeliegenden menschlichen Impulse" zu beobachten (§III), setzt voraus, dass der beobachtete Inhalt von Menschen produziert wurde. Diese Annahme ist zunehmend unzuverlässig.

**Social Bots.** Automatisierte Accounts, die menschliches Verhalten nachahmen, wurden auf jeder großen sozialen Plattform dokumentiert. Schätzungen der Bot-Prävalenz variieren stark — Varol et al. (2017) schätzten 9–15 % der aktiven Twitter-Accounts als Bots; neuere Schätzungen für X in 2024–2025 liegen höher. Bot-Prävalenz variiert nach Plattform, Sprache und Thema: Politischer Diskurs, Kryptowährungen und Gesundheitsdesinformation ziehen überproportionale Bot-Aktivität an. Bots produzieren Inhalte, die ununterscheidbar von menschlich verfassten Inhalten in AĒRs Pipeline eintreten, wenn der Crawler keine Bot-Erkennung upstream durchführt.

**Koordiniertes inauthentisches Verhalten (CIB).** Staatlich gesponserte und kommerziell motivierte Einflussoperationen nutzen Netzwerke von Accounts (nicht alle automatisiert), um spezifische Narrative zu verstärken. Das Stanford Internet Observatory, Graphika und das DFRLab haben Hunderte von CIB-Kampagnen über Plattformen hinweg dokumentiert. CIB ist für AĒR besonders herausfordernd, weil der Inhalt selbst linguistisch ununterscheidbar von organischem menschlichem Diskurs sein kann — die Inauthentizität liegt im Koordinationsmuster, nicht im Text.

**KI-generierter Inhalt.** Seit 2023 haben große Sprachmodelle die Produktion synthetischen Texts in beispiellosem Umfang und beispielloser Qualität ermöglicht. KI-generierte Nachrichtenartikel, Blogposts, Social-Media-Inhalte und Forenkommentare sind mit oberflächlichen linguistischen Merkmalen allein zunehmend schwer von menschlich verfasstem Text zu unterscheiden. Für AĒR stellt dies eine existenzielle Herausforderung dar: Wenn ein wachsender Anteil des beobachteten „Diskurses" maschinengeneriert ist, wird der Anspruch des Systems, menschliche gesellschaftliche Muster zu beobachten, untergraben. Die Rate der KI-Content-Proliferation variiert nach Plattform und Sprache — englischsprachiger Inhalt auf Plattformen mit niedrigen Moderationsbarrieren ist wahrscheinlich am stärksten betroffen.

### 5.2 Erkennungsansätze

Bot- und synthetische Inhaltserkennung ist ein aktives Forschungsfeld ohne etablierte Lösungen. Die Ansätze können auf AĒRs architektonische Constraints abgebildet werden:

**Verhaltensmerkmale auf Account-Ebene.** Traditionelle Bot-Erkennung stützt sich auf Metadaten über den postenden Account: Account-Alter, Postingfrequenz, Follower/Following-Verhältnisse, Profilvollständigkeit (Cresci, 2020). Diese Merkmale stehen AĒR nicht in allen Kontexten zur Verfügung — RSS-Feeds haben kein Account-Konzept; API-eingeschränkte Plattformen exponieren möglicherweise keine Account-Metadaten. Wo verfügbar, gehören Account-Metadaten in `SilverMeta` als quellenspezifischer Kontext, nicht in `SilverCore`.

**Koordinationserkennung auf Netzwerkebene.** CIB-Erkennung stützt sich auf die Identifikation koordinierter Posting-Muster: synchronisiertes Timing, geteilte Inhalte, Netzwerkstrukturanalyse (Pacheco et al., 2021). Dies erfordert Korpus-Level-Analyse (der `CorpusExtractor`-Pfad, antizipiert in §13.3) und dokumentübergreifende Metadaten — insbesondere temporale Ko-Okkurrenz von Inhalten verschiedener Quellen. AĒRs Gold-Schicht speichert bereits temporale Verteilungsmetriken; die Erweiterung auf Koordinationserkennung ist architektonisch machbar, aber methodologisch anspruchsvoll.

**Linguistische Merkmale synthetischen Texts.** KI-generierter Text weist statistische Regelmäßigkeiten auf, die sich von menschlichem Text unterscheiden: niedrigere Perplexität, gleichmäßigere Token-Verteilungen, charakteristische Phrasenmuster und reduzierte lexikalische Diversität (Gehrmann et al., 2019; Mitchell et al., 2023). Allerdings werden diese Merkmale mit der Verbesserung von Sprachmodellen rasch unzuverlässiger. Erkennungsmethoden, die gegen GPT-3 funktionieren, können gegen GPT-5 oder Claude scheitern. Dies ist ein Wettrüsten ohne stabiles Gleichgewicht.

**Wasserzeichen und Provenienz.** Einige KI-Anbieter betten statistische Wasserzeichen in generierten Text ein (Kirchenbauer et al., 2023). Diese sind nicht universell eingesetzt, können durch Paraphrasierung entfernt werden und sind nur detektierbar, wenn das Wasserzeichenschema bekannt ist. AĒR kann sich nicht auf Wasserzeichen als primäre Erkennungsstrategie stützen, sollte aber die Entwicklung von Provenienzstandards (C2PA, Content Credentials) beobachten, die in Zukunft Authentizitätssignale auf Metadatenebene liefern könnten.

### 5.3 AĒRs Position: Dokumentieren, nicht Filtern

Eine kritische Designentscheidung für AĒR ist, ob mutmaßlicher Bot- oder synthetischer Inhalt *herausgefiltert* oder als Metadaten *gekennzeichnet* werden soll. Die Verpflichtung des Manifests zur unverfälschten Beobachtung („unverfälschter Spiegel der Menschheit") legt Letzteres nahe: AĒR sollte Daten nicht auf Basis abgeleiteter Authentizität unterdrücken, weil die Inferenz selbst falsch sein kann (false positives), und weil Präsenz, Prävalenz und Dynamiken nicht-menschlicher Akteure selbst beobachtenswerte Phänomene sind.

Der vorgeschlagene Ansatz:

1. **Authentizitätsindikatoren als Gold-Metriken berechnen.** Per-Dokument-Extractors implementieren, die probabilistische Authentizitätsscores produzieren (z. B. `human_authorship_confidence: 0.87`). Diese sind Tier-2- oder Tier-3-Metriken — reproduzierbar mit fixierter Modellversion, aber nicht deterministisch über Modellversionen hinweg.

2. **Koordinationsindikatoren als Korpus-Level-Metriken berechnen.** `CorpusExtractor`-Instanzen implementieren, die temporale und inhaltsbasierte Koordinationsmuster über Dokumente innerhalb eines Zeitfensters erkennen. Koordinierte Cluster flaggen statt einzelner Dokumente.

3. **Indikatoren über Progressive Disclosure exponieren.** Das Dashboard zeigt Aggregatmetriken mit und ohne mutmaßlich nicht-menschlichen Inhalt. Der Analyst kann in geflaggte Dokumente hineinbohren und sein eigenes Urteil bilden. Das System trifft nicht die Filterentscheidung — der Mensch tut es.

4. **Das Wettrüsten dokumentieren.** Die Erkennungsmodelle werden sich über die Zeit verschlechtern, wenn synthetischer Inhalt sich verbessert. AĒR muss Erkennungsmodelle versionsfixieren und ihre bekannten Limitierungen dokumentieren, genau wie WP-002 es für Sentiment- und NER-Modelle verlangt.

---

## 6. Demografische Verzerrung und der Digital Divide

### 6.1 Wer fehlt in den Daten?

Das Manifest erkennt den Digital Divide als „einen definierenden Parameter der Beobachtung" an (§II). Dieser Abschnitt macht die Kluft konkret, indem er kartiert, wer im digitalen Diskurs, den AĒR beobachten kann, systematisch unterrepräsentiert ist.

**Alter.** Internetnutzung variiert dramatisch nach Alterskohorte, doch das Muster ist nicht über Kulturen hinweg einheitlich. In vielen westlichen Gesellschaften sind ältere Erwachsene (65+) in sozialen Medien unterrepräsentiert, aber in Nachrichtenkommentarsektionen und Foren präsent. In Subsahara-Afrika, wo das Medianalter bei 19 liegt, liegt die Schwelle der „ältere Generation"-Unterrepräsentation deutlich niedriger. In Japan, einer alternden Gesellschaft mit hoher digitaler Kompetenz, sind ältere Erwachsene auf LINE und Yahoo! News-Kommentaren präsent, aber auf TikTok und Instagram abwesend.

**Sozioökonomischer Status.** Internetzugang korreliert mit Einkommen, Bildung und Urbanisierung. Ländliche Bevölkerungen in Indien, Indonesien und Subsahara-Afrika haben andere Internetzugangsmuster (nur mobil, intermittierende Konnektivität, datenkostenbeschränkte Nutzung) als urbane Bevölkerungen. Diese Zugangsmuster bestimmen, welche Plattformen wie genutzt werden — datensparsame Anwendungen (WhatsApp, Telegram) können gegenüber datenintensiven (YouTube, TikTok) bevorzugt werden.

**Geschlecht.** Gender-Gaps beim Internetzugang bestehen in vielen Regionen fort. Die GSMA berichtet, dass Frauen in Ländern niedrigen und mittleren Einkommens 16 % weniger wahrscheinlich mobiles Internet nutzen als Männer (2023). Selbst wo der Zugang gleich ist, können Plattformwahl und Nutzungsmuster geschlechtsspezifisch in kulturell spezifischer Weise variieren. Anonyme Plattformen können in manchen Kulturen männliche Partizipation überrepräsentieren, während sie in anderen schützende Räume für Frauenstimmen bieten.

**Sprache.** Das Internet wird von englischsprachigem Inhalt dominiert (nach den meisten Schätzungen ca. 55–60 % des Webinhalts), gefolgt von Spanisch, Chinesisch, Arabisch und Portugiesisch. Sprecher weniger digitalisierter Sprachen — Tausende von Sprachen mit aktiven Sprechergemeinschaften, aber minimaler digitaler Präsenz — sind für jedes textbasierte Beobachtungssystem effektiv unsichtbar. AĒRs Ockham's-Razor-Prinzip erfordert, diese Grenze anzuerkennen, statt universelle Abdeckung vorzutäuschen.

**Politisches Umfeld.** Bürger autoritärer Regime können sich auf beobachtbaren Plattformen selbst zensieren und echten politischen Diskurs für verschlüsselte oder Offline-Kanäle reservieren. Der öffentliche Diskurs, den AĒR in solchen Kontexten beobachtet, kann das *Erlaubte* statt des *Authentischen* repräsentieren — eine Unterscheidung mit tiefgreifenden Implikationen für die Interpretation von Sentiment- und Haltungsmetriken.

### 6.2 Vom Anerkennen zum Dokumentieren

AĒR kann den Digital Divide nicht lösen. Es kann ihn jedoch für jede Sonde systematisch dokumentieren. Der vorgeschlagene Ansatz:

**Demografisches Profil pro Sonde.** Für jede in der PostgreSQL-Tabelle `sources` registrierte Datenquelle die bekannte demografische Verzerrung dokumentieren: welche Bevölkerungsgruppen überrepräsentiert sind, welche unterrepräsentiert, und welche Evidenz diese Bewertungen stützt. Diese Dokumentation gehört in `SilverMeta` als Metadaten auf Quellenebene, nicht als Pro-Dokument-Metadaten.

**Abdeckungslücken-Visualisierung.** Das Dashboard sollte nicht nur visualisieren, was AĒR beobachtet, sondern auch, was es *nicht beobachten kann* — eine Karte bekannter blinder Flecken. Dies ist das Makroskop-Äquivalent zur Lichtverschmutzungskarte eines Teleskops: eine explizite Anerkennung der Instrumentenlimitierungen als Merkmal des Instruments selbst.

**Gewichtungsstrategien (Forschungsfrage).** Kann demografische Gewichtung — eine Standardtechnik in der Surveymethodologie — auf digitale Diskursdaten angewendet werden? Wenn AĒR weiß, dass eine Plattform urbane, gebildete, 25–35-jährige Männer überrepräsentiert, kann es die Daten gewichten, um Populationsrepräsentativität anzunähern? Dies ist eine offene Forschungsfrage mit erheblichen methodologischen Herausforderungen: Die demografische Zusammensetzung der Plattformnutzer ist selbst unsicher, und die Beziehung zwischen Demografie und Diskursmustern ist komplex und kulturell variabel.

---

## 7. Offene Fragen für interdisziplinäre Kooperationspartner

### 7.1 Für Plattform-Governance- und Internetstudien-Wissenschaftler

**F1: Wie sollte AĒR algorithmische Kurationseffekte dokumentieren und berücksichtigen, wenn Diskurs von engagement-optimierten Plattformen beobachtet wird?**

- AĒRs RSS-basierte Sonde 0 vermeidet dieses Problem vollständig. Bei der Expansion zu Social-Media-Plattformen: welche Metadaten und kontrafaktischen Strategien sind erforderlich, um algorithmisches Signal von gesellschaftlichem Signal zu unterscheiden?
- Deliverable: Ein Framework zur Dokumentation algorithmischer Kurationseffekte pro Plattform, geeignet für die Einbindung in `SilverMeta`.

**F2: Wie sollte AĒR die Post-2023-Forschungs-API-Landschaft navigieren?**

- Da X, Reddit und Meta den akademischen API-Zugang einschränken, welche alternativen Datenerhebungsstrategien sind verfügbar, ethisch und rechtlich vertretbar?
- Welche Plattformen bieten derzeit robuste akademische Forschungs-APIs, und was sind ihre bekannten Limitierungen?
- Deliverable: Eine Plattform-für-Plattform-Zugangsbewertung mit rechtlichen und ethischen Annotationen, jährlich aktualisiert.

### 7.2 Für Computational Social Scientists und Bot-Forscher

**F3: Welche Bot-Erkennungsmethoden sind auf AĒRs Architektur anwendbar, angesichts der Tatsache, dass AĒR Text und Metadaten verarbeitet, aber oft keine vollständigen Verhaltensdaten auf Account-Ebene hat?**

- Rein textbasierte Bot-Erkennung ist weniger genau als Account-Level-Erkennung. Welche Konfidenzschwellen sind für ein System angemessen, das flaggt statt filtert?
- Deliverable: Eine Evaluation von Text- und Metadaten-Level-Bot-Erkennungsmethoden auf einem mehrsprachigen Korpus, mit Falsch-positiv-/Falsch-negativ-Raten pro Methode.

**F4: Wie sollte AĒR KI-generierten Inhalt in seiner Pipeline erkennen und flaggen?**

- Aktuelle Erkennungsmethoden degradieren schnell. Gibt es eine nachhaltige Erkennungsstrategie, oder sollte AĒR KI-generierten Inhalt als beobachtbares Phänomen akzeptieren und sich auf die *Messung seiner Prävalenz* konzentrieren statt auf das *Herausfiltern*?
- Deliverable: Ein Positionspapier zum epistemologischen Status KI-generierten Inhalts in Diskursbeobachtungssystemen, mit praktischen Empfehlungen für AĒRs Extractor-Pipeline.

**F5: Wie kann AĒR koordiniertes inauthentisches Verhalten mittels Korpus-Level-Analyse erkennen?**

- CIB-Erkennung erfordert typischerweise Netzwerkdaten (Follower-Graphen, Repost-Ketten). Können temporale Ko-Okkurrenz und Inhaltsähnlichkeit in AĒRs Gold-Schicht als Stellvertreter dienen?
- Deliverable: Eine Methode zur CIB-Erkennung unter ausschließlicher Nutzung der in `SilverCore` und `SilverMeta` verfügbaren Features (Zeitstempel, Quellidentifikatoren, bereinigter Text), evaluiert gegen bekannte CIB-Kampagnen.

### 7.3 Für Area-Studies-Wissenschaftler und digitale Anthropologen

**F6: Welche Plattformen bilden für jede große Kulturregion das Minimum Viable Probe Set für Diskursbeobachtung?**

- WP-001s Taxonomie definiert vier Diskursfunktionen. Für eine gegebene Gesellschaft (z. B. Brasilien, Japan, Nigeria, Iran), welche Plattformen bilden sich auf welche Funktionen ab? Welche Plattformen sind technisch, rechtlich und ethisch zugänglich?
- Deliverable: Regionale Plattformkarten (1–2 Seiten jeweils) für mindestens: Ostasien, Südasien, MENA, Subsahara-Afrika, Lateinamerika, Russland/postsowjetischer Raum, Europa, Nordamerika. Jede Karte sollte Plattformen, ihre Diskursfunktion, Zugänglichkeit und bekannte demografische Verzerrung identifizieren.

**F7: Wie beeinflussen plattformspezifische Kommunikationsnormen die Interpretierbarkeit von Textmetriken über Plattformen hinweg?**

- Ein Twitter/X-Thread, ein Reddit-Kommentar, ein Telegram-Kanalpost und ein RSS-Artikel über dasselbe Ereignis produzieren strukturell unterschiedlichen Text. Wie sollte AĒR plattformspezifische Genreeffekte beim Vergleich von Metriken über Quellen hinweg berücksichtigen?
- Deliverable: Eine Plattform-Genre-Taxonomie, die die strukturellen, rhetorischen und pragmatischen Unterschiede zwischen auf verschiedenen Plattformen produziertem Text dokumentiert.

### 7.4 Für Survey-Methodologen und Statistiker

**F8: Können demografische Gewichtungstechniken aus der Surveymethodologie für digitale Diskursdaten adaptiert werden?**

- Surveygewichtung erfordert bekannte Populationsparameter und bekannte Selektionswahrscheinlichkeiten. Keines von beiden ist für digitale Plattformen verfügbar. Unter welchen Bedingungen, falls überhaupt, kann Gewichtung demografische Verzerrung in gecrawlten Daten reduzieren?
- Deliverable: Eine methodologische Bewertung von Gewichtungsstrategien für nicht-probabilistische digitale Stichproben, mit Empfehlungen für AĒRs Aggregationspipeline.

**F9: Wie sollte AĒR die durch Plattform-Bias eingeführte Unsicherheit modellieren und berichten?**

- Wenn alle Metriken plattforminduzierte Unsicherheit tragen, wie sollte diese Unsicherheit durch die Aggregationspipeline propagiert und im Dashboard visualisiert werden?
- Deliverable: Ein Unsicherheitsquantifizierungs-Framework für plattformvermittelte Diskursmetriken, kompatibel mit AĒRs ClickHouse-Gold-Schema.

---

## 8. Architektonische Implikationen

### 8.1 SilverMeta-Erweiterung für Bias-Dokumentation

Das Source Adapter Protocol (ADR-015) unterstützt bereits quellenspezifische `SilverMeta`-Modelle. WP-003 schlägt die Standardisierung eines Sets bias-relevanter Metadatenfelder über alle zukünftigen Source Adapters vor:

```python
class BiasContext(BaseModel):
    """Standardisierte Bias-Dokumentationsfelder für SilverMeta."""
    platform_type: str           # z. B. "rss", "microblog", "forum", "messenger_channel"
    access_method: str           # z. B. "public_rss", "academic_api", "public_web"
    visibility_mechanism: str    # z. B. "chronological", "algorithmic", "editorial"
    moderation_context: str      # z. B. "netzdg", "unmoderated", "state_censored"
    engagement_data_available: bool  # ob Likes/Shares/Kommentare erfasst werden
    account_metadata_available: bool # ob Autoren-/Account-Features erfasst werden
```

Diese Felder ermöglichen nachgelagerter Analyse die Stratifizierung von Metriken nach Plattformcharakteristiken — z. B. den Vergleich von Sentimentverteilungen über algorithmisch kuratierte vs. chronologisch geordnete Quellen.

### 8.2 Neue Extractor-Typen

WP-003 antizipiert zwei neue Extractor-Kategorien über WP-002s Metrik-Extractors hinaus:

**Authentizitäts-Extractors (pro Dokument, Tier 2/3).** Berechnen probabilistische Scores für menschliche Autorenschaft, KI-Generierungswahrscheinlichkeit und Template-basierte Inhaltserkennung. Diese würden das bestehende `MetricExtractor`-Protokoll implementieren und Metriken wie `human_authorship_confidence` und `template_match_score` produzieren.

**Koordinations-Extractors (Korpus-Level, Tier 2).** Erkennen temporale und inhaltsbasierte Koordinationsmuster über Dokumente innerhalb eines Zeitfensters. Diese erfordern das `CorpusExtractor`-Protokoll (antizipiert in §13.3, blockiert durch R-9) und produzieren Korpus-Level-Metriken wie `coordination_score` für Dokumentcluster.

### 8.3 Auswirkung auf die Gold-Schicht

Keine Schemaänderungen an `aer_gold.metrics` sind erforderlich — Authentizitäts- und Koordinationsscores sind numerische Metriken, die in das bestehende `(timestamp, value, source, metric_name)`-Schema passen. Allerdings kann Koordinationserkennung auf Korpus-Level *Cluster-Level-Outputs* produzieren (Gruppen von als koordiniert geflaggten Dokumenten), die sich nicht sauber auf das Pro-Dokument-Metrikmodell abbilden lassen. Dies kann eine neue Gold-Tabelle erfordern:

```sql
CREATE TABLE aer_gold.coordination_clusters (
    cluster_id       String,
    detection_date   DateTime,
    detection_method String,
    method_version   String,
    document_ids     Array(String),
    coordination_type String,    -- z. B. "temporal_sync", "content_duplicate", "cross_platform"
    confidence       Float32
) ENGINE = MergeTree()
ORDER BY (detection_date, cluster_id)
TTL detection_date + INTERVAL 365 DAY;
```

Diese Tabelle würde von der BFF API abgefragt, um einzelne Dokumente mit ihrer Koordinationscluster-Zugehörigkeit zu annotieren — und Progressive Disclosure von Aggregattrends zu einzelnen geflaggten Clustern zu ermöglichen.

---

## 9. Die Verantwortung des Beobachters

WP-003 schneidet eine tiefere ethische Frage an, die vollständig in WP-006 (Beobachtereffekt) adressiert wird: AĒRs Akt der Diskursbeobachtung ist nicht neutral. Indem bestimmte Plattformen ausgewählt, bestimmte Erkennungsmethoden angewendet und bestimmte Inhalte als „inauthentisch" geflaggt werden, trifft AĒR Urteile, die politisches Gewicht tragen. Einen Telegram-Kanal als „bot-getrieben" oder eine Nachrichtenquelle als „staatskontrolliert" zu labeln, ist ein analytischer Akt mit Konsequenzen dafür, wie die Daten interpretiert werden.

AĒRs architektonische Verpflichtung zu Progressive Disclosure — dem Analysten die Rohdaten neben den berechneten Metriken zu zeigen — ist die primäre Schutzmaßnahme gegen Blackbox-Bias-Urteile. Doch das Design des Dashboards, die Standardansichten, die Wahl, welche Metriken hervorgehoben und welche verborgen werden — dies sind alles Designentscheidungen, die epistemologisches Gewicht tragen. Sie sollten transparent getroffen werden, mit Input der interdisziplinären Kooperationspartner, die dieses Papier zu engagieren sucht.

---

## 10. Referenzen

- Bail, C. A., Argyle, L. P., Brown, T. W., et al. (2018). „Exposure to Opposing Views on Social Media Can Increase Political Polarization." *Proceedings of the National Academy of Sciences*, 115(37), 9216–9221.
- Brady, W. J., Wills, J. A., Jost, J. T., Tucker, J. A. & Van Bavel, J. J. (2017). „Emotion Shapes the Diffusion of Moralized Content in Social Networks." *Proceedings of the National Academy of Sciences*, 114(28), 7313–7318.
- Bucher, T. & Helmond, A. (2018). „The Affordances of Social Media Platforms." In Burgess, J., Marwick, A. & Poell, T. (Hrsg.), *The SAGE Handbook of Social Media*, 233–253. SAGE.
- Cresci, S. (2020). „A Decade of Social Bot Detection." *Communications of the ACM*, 63(10), 72–83.
- Gehrmann, S., Strobelt, H. & Rush, A. M. (2019). „GLTR: Statistical Detection and Visualization of Generated Text." *Proceedings of ACL 2019 — System Demonstrations*.
- GSMA (2023). *The Mobile Gender Gap Report 2023*. GSMA Connected Women.
- Huszár, F., Ktena, S. I., O'Brien, C., et al. (2022). „Algorithmic Amplification of Politics on Twitter." *Proceedings of the National Academy of Sciences*, 119(1), e2025334119.
- Kirchenbauer, J., Geiping, J., Wen, Y., et al. (2023). „A Watermark for Large Language Models." *Proceedings of ICML 2023*.
- Mitchell, E., Lee, Y., Khazatsky, A., Manning, C. D. & Finn, C. (2023). „DetectGPT: Zero-Shot Machine-Generated Text Detection using Probability Curvature." *Proceedings of ICML 2023*.
- Pacheco, D., Hui, P.-M., Torres-Lugo, C., et al. (2021). „Uncovering Coordinated Networks on Social Media: Methods and Case Studies." *Proceedings of ICWSM 2021*.
- Varol, O., Ferrara, E., Davis, C. A., Menczer, F. & Flammini, A. (2017). „Online Human-Bot Interactions: Detection, Estimation, and Characterization." *Proceedings of ICWSM 2017*.

---

## Anhang A: Zuordnung zu AĒR Offene Forschungsfragen (§13.6)

| §13.6 Frage | WP-003 Abschnitt | Status |
| :--- | :--- | :--- |
| 2. Bias-Kalibrierung | §2–§6 | Adressiert — Framework und Forschungsfragen vorgeschlagen |
| 1. Sondenauswahl | §3 (Plattformlandschaft), §6 (demografische Verzerrung) | Teilweise adressiert — ergänzt WP-001 |
| 3. Metrik-Validität | §4.1 (Verstärkung beeinflusst Metrikinterpretation) | Querverweis zu WP-002 |
| 6. Beobachtereffekt | §9 | Vorweggenommen — WP-006 gewidmet |

## Anhang B: Zuordnung zu WP-001 Funktionale Taxonomie

| WP-001 Diskursfunktion | Plattform-Bias-Erwägungen |
| :--- | :--- |
| Epistemische Autorität | Institutionelle RSS/Websites — niedriger algorithmischer Bias, hoher redaktioneller Bias. Regierungsplattformen können staatliche Zensur beinhalten. |
| Ressourcen- & Machtlegitimation | Offizielle Kanäle — strukturell kontrollierte Botschaften. Bot-Verstärkung ist bei Staatsnarrativen verbreitet. |
| Kohäsion & Identitätsbildung | Soziale Medien, Influencer-Plattformen — hohe algorithmische Verstärkung, engagement-optimiert, signifikante Bot-/CIB-Präsenz. |
| Subversion & Friktion | Verschlüsselte Nachrichtendienste, anonyme Foren, alternative Plattformen — geringe Beobachtbarkeit, hohe Authentizitätsunsicherheit, Plattform-Vergänglichkeitsrisiko. |