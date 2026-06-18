# WP-001: Auf dem Weg zu einem kulturell agnostischen Sondenkatalog — Eine funktionale Taxonomie für die globale Diskursbeobachtung

> **Reihe:** AĒR Wissenschaftliche Methodik — Arbeitspapiere
> **Status:** Entwurf v2 — grundlegend überarbeitet, offen für interdisziplinäre Begutachtung
> **Datum:** 07.04.2026 (v2), 03.04.2026 (v1)
> **Abhängig von:** Manifest §III–§IV, Arc42 Kapitel 13 (Wissenschaftliche Grundlagen)
> **Architekturkontext:** Source Adapter Protocol (ADR-015), SilverMeta (§5.1.2), Crawler-Ökosystem (§5.2)
> **Nachgelagert:** WP-002 (Metrik-Validität), WP-003 (Plattform-Bias), WP-004 (Interkulturelle Vergleichbarkeit), WP-005 (Temporale Granularität), WP-006 (Beobachtereffekt)
> **Lizenz:** [CC BY-NC 4.0](https://creativecommons.org/licenses/by-nc/4.0/) — © 2026 Fabian Quist

---

## 1. Zielsetzung

Dieses Arbeitspapier adressiert die erste und grundlegendste offene Forschungsfrage aus §13.6: **Welche digitalen Räume bilden repräsentative „Sonden" zur Beobachtung gesellschaftlichen Diskurses, und wie wählen und gewichten wir sie, ohne kulturell voreingenommene Kategorien aufzuzwingen?**

Das Manifest von AĒR (§IV) etabliert das **Sondenprinzip**: Anstatt eine totale Datenaggregation anzustreben, wählt das System strategische Beobachtungspunkte innerhalb des globalen Informationsraums, die als Stellvertreter für unterschiedliche gesellschaftliche Realitäten dienen. Die Auswahl, Gewichtung und Interpretation dieser Sonden bilden die zentrale interdisziplinäre Herausforderung. Das Manifest spezifiziert jedoch nicht, *wie* Sonden ausgewählt werden sollen. Dieses Arbeitspapier liefert den methodologischen Rahmen.

Die zentrale These lautet, dass die Sondenauswahl durch die **diskursive Funktion** geleitet werden muss, nicht durch die institutionelle Form. Ein Oberster Gerichtshof und ein Rat religiöser Ältester mögen institutionell keinerlei Ähnlichkeit aufweisen, doch beide können die Funktion epistemischer Autorität erfüllen — sie definieren, was in einer gegebenen Gesellschaft als wahr, legitim und sagbar gilt. Eine staatliche Nachrichtenagentur und ein von Oligarchen finanziertes Medienimperium mögen unterschiedliche Eigentümerstrukturen haben, doch beide können die Funktion der Machtlegitimation erfüllen. Indem AĒR Sonden nach ihrer diskursiven Funktion statt nach ihrer institutionellen Kategorie klassifiziert, wird interkulturelle Vergleichbarkeit erreicht, ohne westliche institutionelle Ontologien aufzuzwingen.

Dieses Papier verankert die funktionale Taxonomie in der vergleichenden Sozialwissenschaft und anthropologischen Theorie, definiert jede Diskursfunktion mit interkultureller Tiefe, spezifiziert das Etisch/Emische Duale Tagging-System als architektonischen Operationalisierungsmechanismus, etabliert den Sondenauswahlprozess als formalisierte wissenschaftliche Methode und identifiziert konkrete Forschungsfragen für die interdisziplinäre Zusammenarbeit.

---

## 2. Das Problem: Warum institutionelle Kategorien scheitern

### 2.1 Die westliche Grundannahme

Der naheliegendste Weg, Informationsquellen zu kategorisieren, ist nach institutionellem Typ: „Regierung", „Medien", „Zivilgesellschaft", „Wissenschaft", „soziale Medien". Diese Kategorisierung wirkt objektiv — sie benennt scheinbar einfach, was die Dinge sind. Doch sie ist nicht objektiv. Sie ist ein Produkt westlicher politischer Moderne, konkret der liberal-demokratischen Gewaltenteilung und der aufklärerischen Unterscheidung zwischen Staat, Markt und Öffentlichkeit.

Bei globaler Anwendung erzeugt diese Kategorisierung systematische Verzerrungen:

**Die „Medien"-Trugschluss.** In westlichen Demokratien bezeichnet „Medien" typischerweise kommerziell oder öffentlich finanzierte Nachrichtenorganisationen, die unter Normen redaktioneller Unabhängigkeit operieren (in unterschiedlichen Graden und mit unterschiedlicher Durchsetzung). In Russland operieren große Medienunternehmen unter redaktionellem Einfluss des Kreml, während sie die formale Struktur unabhängigen Journalismus beibehalten. In China operieren alle Medien innerhalb des Partei-Staat-Rahmens; das Konzept redaktioneller Unabhängigkeit vom Staat ist nicht lediglich abwesend, sondern wird explizit als westlich-liberales Konstrukt zurückgewiesen. In vielen Golfstaaten ist Medieneigentum mit den Interessen der Herrscherfamilien verflochten. In Indien konzentriert sich Medieneigentum bei Konglomeraten mit vielfältigen Geschäftsinteressen, die die redaktionelle Ausrichtung prägen. All diese als „Medien" zu kategorisieren und als funktional äquivalent zu behandeln, wäre ein Kategorienfehler — sie erfüllen unterschiedliche diskursive Funktionen, obwohl sie dasselbe institutionelle Etikett tragen.

**Der „Regierung"-Trugschluss.** „Regierungskommunikation" in Deutschland bedeutet Pressemitteilungen der Bundesregierung — institutionell, bürokratisch, verfassungsrechtlich eingehegt. „Regierungskommunikation" in China bedeutet einen komplexen Apparat, der das Informationsbüro des Staatsrats, die Nachrichtenagentur Xinhua, die Volkszeitung, CGTN und ein Ökosystem staatsnaher Social-Media-Accounts umfasst. „Regierungskommunikation" in den Vereinigten Staaten umfasst das Pressebüro des Weißen Hauses, aber auch eine dezentrale Landschaft von Kommunikation der Bundesbehörden, des Kongresses und der Bundesstaaten. Das institutionelle Etikett „Regierung" verbirgt mehr, als es offenbart.

**Der „Zivilgesellschaft"-Trugschluss.** Das Konzept der „Zivilgesellschaft" — autonome Organisationen, die zwischen Staat und Markt operieren — ist ein westliches Konstrukt mit begrenzter Anwendbarkeit in vielen Kontexten. In Gesellschaften, in denen Clan-, Stammes- oder religiöse Strukturen zwischen Individuum und Staat vermitteln, zwingt die westliche Kategorie „Zivilgesellschaft" lokale Institutionen entweder in einen schlecht passenden Rahmen oder macht sie unsichtbar. In China ist das Konzept „Zivilgesellschaft" (公民社会) politisch sensibel; Organisationen, die in einem westlichen Kontext als „Zivilgesellschaft" gelten würden, operieren unter grundlegend anderen Bedingungen. In Russland hat das Gesetz über „ausländische Agenten" die Grenze zwischen Zivilgesellschaft und staatlicher Kontrolle neu definiert.

### 2.2 Epistemologischer Kolonialismus im Forschungsdesign

Die Aufzwingung westlicher institutioneller Kategorien auf nicht-westliche Gesellschaften ist eine Form dessen, was Santos (2014) **epistemologischen Kolonialismus** nennt — die Annahme, dass westliche Wissensrahmen universell sind und dass andere Rahmen Abweichungen von oder Annäherungen an den westlichen Standard darstellen.

In der vergleichenden Politikwissenschaft ist dieses Problem wohlbekannt. Collier und Mahon (1993) warnten vor „Konzeptdehnung" (conceptual stretching) — der Anwendung von Konzepten jenseits der Kontexte, in denen sie gültig sind. Sartori (1970) zeigte, dass vergleichende Analyse Konzepte auf dem richtigen Abstraktionsniveau erfordert — zu konkret (spezifisch für eine Gesellschaft) und Vergleich wird unmöglich; zu abstrakt (universelle Gemeinplätze) und Vergleich wird bedeutungslos.

Für AĒR würde sich epistemologischer Kolonialismus in der Sondenauswahl manifestieren: Datenquellen auf Basis westlicher institutioneller Kategorien zu wählen (z. B. „wir müssen in jedem Land die staatliche Nachrichtenagentur und die unabhängige Presse crawlen") und damit eine strukturelle Annahme darüber aufzuzwingen, wie Gesellschaften ihren Diskurs organisieren. In Gesellschaften, in denen Diskurs über religiöse Institutionen, Diaspora-Netzwerke, über WhatsApp digitalisierte clan-basierte orale Traditionen oder staatlich-korporative Medienhybride organisiert wird, erzeugt die westliche institutionelle Landkarte blinde Flecken — und diese blinden Flecken sind nicht zufällig. Sie schließen systematisch nicht-westliche Formen der Diskursorganisation aus.

### 2.3 Funktionale Äquivalenz als Lösung

Die methodologische Lösung stammt aus der vergleichenden Anthropologie und Politikwissenschaft: **funktionale Äquivalenz** (Dogan & Pelassy, 1990; Peters, 1998). Anstatt zu fragen „Wie heißt diese Institution?" fragen wir „Welche Funktion erfüllt diese Institution in dieser Gesellschaft?"

Funktionale Äquivalenz erkennt an, dass:

1. Dieselbe gesellschaftliche Funktion (z. B. Normsetzung, Machtlegitimation, Identitätsbildung) in verschiedenen Gesellschaften von unterschiedlichen institutionellen Formen erfüllt werden kann.
2. Dieselbe institutionelle Form (z. B. „Zeitung") in verschiedenen Gesellschaften unterschiedliche Funktionen erfüllen kann.
3. Einige Funktionen universell in allen menschlichen Gesellschaften vorhanden sind (alle Gesellschaften haben Mechanismen zur Normetablierung, Machtlegitimation, Identitätsbildung und Hegemoniebestreitung), auch wenn die institutionellen Träger dieser Funktionen variieren.

Diese Universalität der Funktion bei gleichzeitiger Vielfalt der Form ist es, die interkulturellen Vergleich ohne kulturelle Auslöschung ermöglicht. AĒR muss nicht „das Äquivalent der New York Times in Japan" finden — es muss die Quellen finden, die in der japanischen Gesellschaft die *Funktion* der Agendasetzung und epistemischen Autorität erfüllen, welche institutionelle Form sie auch immer annehmen.

---

## 3. Die vier Diskursfunktionen: Eine Taxonomie

### 3.1 Theoretische Fundierung

Die hier vorgeschlagene funktionale Taxonomie schöpft aus mehreren theoretischen Traditionen:

- **Foucaults Diskurstheorie** — Diskurs als Regelsystem, das bestimmt, was gesagt werden kann, von wem und unter welchen Bedingungen (Foucault, 1969). Jede Gesellschaft unterhält Grenzen des Sagbaren; die erste Funktion erfasst die Akteure, die diese Grenzen setzen.
- **Gramscis Hegemonie** — die Dominanz einer herrschenden Klasse nicht allein durch Zwang, sondern durch kulturelle und ideologische Führung (Gramsci, 1971). Die zweite Funktion erfasst die Kanäle, durch die hegemoniale Narrative produziert und aufrechterhalten werden.
- **Andersons vorgestellte Gemeinschaften** — die Konstruktion kollektiver Identität durch geteilte Narrative, Symbole und Medien (Anderson, 1991). Die dritte Funktion erfasst die Quellen, die das „Wir" kollektiver Identität produzieren.
- **Scotts verborgene Transkripte** — die Widerstandsdiskurse, die untergeordnete Gruppen außerhalb der öffentlichen Sphäre aufrechterhalten (Scott, 1990). Die vierte Funktion erfasst die Räume, in denen Gegennarrative entstehen und hegemoniale Diskurse angefochten werden.

Diese vier theoretischen Traditionen bilden sich auf vier universelle Diskursfunktionen ab, die — in kulturell spezifischen Formen — in allen vernetzten Gesellschaften vorhanden sind.

### 3.2 Funktion 1: Epistemische Autorität (Norm- und Wahrheitssetzung)

**Definition.** Akteure oder Kanäle, die die Grenzen des Sagbaren definieren — und festlegen, was in einer gegebenen Gesellschaft als wahr, moralisch akzeptabel, rechtlich bindend oder spirituell rein gilt. Epistemische Autoritäten berichten nicht lediglich über Fakten; sie *konstituieren* den Rahmen, innerhalb dessen Fakten interpretiert werden.

**Theoretische Basis.** Foucaults Konzept der *Episteme* — die unbewusste Wissensstruktur, die bestimmt, was in einer gegebenen Epoche gedacht und gesagt werden kann. AĒRs philosophischer Episteme-Pfeiler (Kapitel 1, §1.2) leitet sich direkt von diesem Konzept ab.

**Interkulturelle Instanziierungen:**

| Gesellschaft | Institutionelle Form | Warum sie als epistemische Autorität fungiert |
| :--- | :--- | :--- |
| Deutschland | Bundesverfassungsgericht, Tagesschau (öffentlich-rechtlicher Rundfunk), Robert Koch-Institut (Gesundheitswesen) | Die Urteile des Bundesverfassungsgerichts definieren die rechtlichen Grenzen des Sagbaren. Die Tagesschau setzt die informationelle Grundlinie. Das RKI definierte während COVID-19 den wissenschaftlichen Konsens. |
| USA | Supreme Court, New York Times / Washington Post, CDC, Universitätssystem | Der Supreme Court definiert verfassungsrechtliche Grenzen. Der Status als „Newspaper of Record" verleiht epistemische Autorität. Universitäre Zertifizierung validiert Expertise. |
| Iran | Wächterrat (*Shora-ye Negahban*), hochrangige schiitische Geistliche (Marjas), IRIB (Staatsrundfunk) | Religiöse und staatliche Autorität sind verfassungsrechtlich verschmolzen. Der Wächterrat prüft Gesetzgebung gegen islamisches Recht. Marjas erlassen *Fatwas*, die als epistemische Urteile fungieren. |
| Japan | NHK, große überregionale Tageszeitungen (*Asahi*, *Yomiuri*, *Mainichi*), Gesundheitsministerium | Der öffentlich-rechtliche Auftrag von NHK verleiht epistemischen Status. Das nationale Presseclubsystem (*kisha kurabu*) beschränkt den Zugang und verleiht institutionell anerkannten Medien Autorität. |
| Nigeria | Religiöse Führer (christliche Bischöfe, islamische Geistliche), traditionelle Herrscher (*Obas*, *Emire*), Staatsrundfunk (NTA) | In einer Gesellschaft, in der religiöse Zugehörigkeit soziale Identität strukturiert, fungieren religiöse Führer als epistemische Autoritäten neben — und manchmal in Spannung mit — staatlichen Institutionen. Traditionelle Herrscher bewahren epistemische Autorität in ihren Domänen. |
| China | Volkszeitung, Xinhua, CCTV (Staatsmedien-Dreiklang), Kommuniqués des Zentralkomitees, Chinesische Akademie der Wissenschaften | Der Partei-Staat definiert die Grenzen des Sagbaren durch explizite Propagandadirektiven und implizite Selbstzensurmechanismen. Offizielle Medien berichten nicht lediglich — sie vollziehen die epistemische Funktion des Partei-Staats. |

**Zentrale Erkenntnis:** Epistemische Autorität ist nicht inhärent staatsgebunden. In vielen Gesellschaften halten religiöse Institutionen, Stammesälteste oder Berufsgilden epistemische Autorität, die unabhängig von (oder in Spannung mit) staatlicher Macht besteht. AĒRs Sondenauswahl muss alle bedeutsamen epistemischen Autoritäten identifizieren, nicht nur jene, die von westlichen institutionellen Kategorien erfasst werden.

**Beobachtungsherausforderung:** Epistemische Autorität wird teilweise durch das konstituiert, was sie *ausschließt*. Die Grenzen des Sagbaren werden ebenso sehr durch das definiert, was unsagbar ist, wie durch das, was gesagt wird. AĒRs Textmetriken (WP-002) erfassen, was ausgedrückt wird; sie erfassen nicht, was unterdrückt wird. Dies ist eine fundamentale Einschränkung, die im Manifest dokumentiert ist (§II: „Resonanz statt Wahrheit").

### 3.3 Funktion 2: Ressourcen- und Machtlegitimation

**Definition.** Kanäle, die von dominanten Strukturen — staatlichen, militärischen, oligarchischen, korporativen oder dynastischen — genutzt werden, um ihre Macht zu rechtfertigen, ihre Narrative zu operationalisieren und ihre Position aufrechtzuerhalten. Diese Quellen beschreiben Macht nicht lediglich; sie *vollziehen* sie durch Kommunikation.

**Theoretische Basis.** Gramscis Hegemonie — der Prozess, durch den herrschende Gruppen Zustimmung durch kulturelle Führung sichern. Habermas' Konzept der *strategischen Kommunikation* — Kommunikation, die auf Erfolg (Zielerreichung) ausgerichtet ist, nicht auf Verständigung (Konsenserzielung).

**Interkulturelle Instanziierungen:**

| Gesellschaft | Institutionelle Form | Legitimationsmechanismus |
| :--- | :--- | :--- |
| Deutschland | Presseamt der Bundesregierung, Parteikommunikation, Unternehmens-PR (DAX-Konzerne) | Demokratische Legitimation durch Politikerklärung. Regierungs-PR operiert innerhalb verfassungsrechtlicher Transparenznormen. Unternehmens-PR operiert innerhalb von Marktkommunikationsnormen. |
| Russland | RT, TASS, Kremlin.ru, Kommunikation der Gouverneure, oligarchennahe Medien | Souveräne Legitimation — das Recht des Staates, das nationale Interesse zu definieren. Medien dienen als Legitimationsinstrument, nicht als Kontrollinstanz. Oligarchennahe Medien wahren eine Fassade der Unabhängigkeit und verstärken zugleich staatliche Narrative. |
| Saudi-Arabien | Saudi Press Agency (SPA), Al Arabiya, Vision-2030-Medienapparat | Monarchische Legitimation durch Modernisierungsnarrativ. Vision 2030 ist gleichzeitig ein Wirtschaftsplan und eine Kommunikationsstrategie, die absolute Monarchie als visionäre Führung umrahmt. |
| USA | Kommunikation des Weißen Hauses, Pentagon-Pressebriefings, Wirtschaftsmedien (Business Press) | Demokratische und Marktlegitimation. Der Kommunikationsapparat des Pentagon ist eine der weltweit größten PR-Operationen. Wirtschaftsmedien naturalisieren Marktlogik als gesunden Menschenverstand. |
| Indien | Regierungsnahe Medien (Doordarshan, PIB), digitaler Apparat der Regierungspartei, Wirtschaftsmedien (CNBC, Economic Times) | Demokratische Legitimation, kompliziert durch parteinahe Medienökosysteme. Die digitale Kommunikationsinfrastruktur der regierenden BJP verwischt die Grenze zwischen Regierungs- und Parteikommunikation. |

**Zentrale Erkenntnis:** Machtlegitimation ist nicht auf staatliche Akteure beschränkt. In vielen Gesellschaften unterhalten korporative, religiöse, militärische oder dynastische Machtstrukturen eigene Kommunikationskanäle, die die Legitimationsfunktion erfüllen. In einigen Kontexten (Golfstaaten, oligarchische Systeme, von Konzernen dominierte Medienlandschaften) ist die Unterscheidung zwischen „staatlicher" und „privater" Machtlegitimation analytisch bedeutungslos.

**Herausforderung bei der Sondenauswahl:** Machtlegitimationsquellen sind oft die *am leichtesten* crawlbaren (offizielle RSS-Feeds, strukturierte Pressemitteilungen, öffentliche APIs), weil sie für Verbreitung konzipiert sind. Dies erzeugt einen strukturellen Bias in AĒRs beobachtbarem Korpus — machtlegitimierende Diskurse sind überrepräsentiert, weil sie technisch am zugänglichsten sind (WP-003, §2.2).

### 3.4 Funktion 3: Kohäsion und Identitätsbildung

**Definition.** Quellen, die ein kollektives „Wir-Gefühl" erzeugen — die durch kulturelle Narrative, Mythenbildung, Ritual und die strukturelle Abgrenzung der Eigengruppe vom „Anderen" geteilte Identität konstruieren. Diese Quellen produzieren die affektive Infrastruktur der Zugehörigkeit.

**Theoretische Basis.** Andersons *vorgestellte Gemeinschaften* — die These, dass Nationen keine natürlichen Entitäten sind, sondern durch geteilte Narrative und Medien konstruiert werden. Durkheims *Kollektivbewusstsein* — die geteilten Überzeugungen und Empfindungen, die eine Gesellschaft zusammenhalten. Turners *Communitas* — das intensive Gefühl sozialen Miteinanders, das in rituellen Kontexten entsteht.

**Interkulturelle Instanziierungen:**

| Gesellschaft | Institutionelle Form | Mechanismus der Identitätsbildung |
| :--- | :--- | :--- |
| Deutschland | Öffentlich-rechtliche Sender (Unterhaltung), Fußballkultur (Bundesliga, DFB), Feuilleton, regionale Identitätsmedien | Kulturelle Identität wird konstruiert im Zusammenspiel von nationalem Rundfunk, regionaler Identität (*Heimat*) und der kulturkritischen Tradition des *Feuilletons*. Fußball fungiert als nationales Ritual, das kollektiven Affekt erzeugt. |
| Japan | NHK (*Taiga*-Historiendramen, *Kōhaku*-Musikprogramm), Manga/Anime-Ökosystem, lokale Festivalmedien (*Matsuri*) | Kulturelle Identität wird reproduziert durch geteilte Medienrituale (NHKs Jahresendprogramm wird von ~40 % der Bevölkerung verfolgt) und durch den globalen Export der Manga/Anime-Kultur, die paradoxerweise die inländische Identität durch internationale Anerkennung stärkt. |
| Brasilien | Globo TV (Telenovelas), Samba/Karneval-Medienökosystem, Fußball, evangelikale Kirchenmedien | Telenovelas fungieren als nationale Narrativmaschinen — sie konstruieren und rekonstruieren brasilianische Identität wöchentlich. Evangelikale Medien konkurrieren zunehmend mit säkularen Medien als Quelle der Identitätsbildung, insbesondere für einkommensschwächere Bevölkerungsschichten. |
| Türkei | TRT (Staatssender), Popkultur (Musik, TV-Serien mit Export in den MENA-Raum), Moscheegemeinden, Diasporamedien | Türkische Identitätsbildung operiert auf mehreren Ebenen: staatlich kuratierte nationale Identität (Atatürk-Erbe), religiöse Identität (Diyanet-vermittelt), kulturell-imperiale Identität (osmanische Nostalgie in TV-Serien) und Diaspora-Identität (türkisch-deutsche, türkisch-niederländische Medien). |
| USA | Hollywood, Sport (NFL, NBA), Social-Media-Influencer, Talk Radio, Kirchengemeinden | Identitätsbildung ist intensiv pluralistisch und fragmentiert. Es gibt kein einzelnes nationales Identitätsnarrativ, sondern konkurrierende Identitätsbildungsmaschinen: progressive kulturelle Produktion (Hollywood, Universitäten), konservative Identitätsmedien (Talk Radio, Fox News), ethnische Identitätsmedien, religiöse Gemeinschaftsmedien. |

**Zentrale Erkenntnis:** Kohäsion und Identitätsbildung operieren über Affekt, nicht über Information. Diese Quellen vermitteln nicht primär Fakten — sie erzeugen Gefühle der Zugehörigkeit, des Stolzes, der Nostalgie und der Solidarität. AĒRs Sentimentmetriken (WP-002) erfassen die affektive Oberfläche, doch die tiefere identitätskonstruierende Funktion erfordert Narrativanalyse (Tier-2/3-Methoden) und kulturelle Expertise.

**Herausforderung bei der Sondenauswahl:** Quellen der Identitätsbildung sind für Außenstehende oft kulturell opak. Ein deutscher Forscher erkennt möglicherweise nicht die identitätsbildende Funktion eines nigerianischen Nollywood-Films oder einer koreanischen Varietéshow. Die Sondenauswahl für diese Funktion *erfordert* Area-Studies-Expertise — es gibt keine Abkürzung durch technische Analyse.

### 3.5 Funktion 4: Subversion und Friktion (Gegendiskurs)

**Definition.** Dezentralisierte, aktivistische oder hyperviral verbreitete Räume, die hegemoniale Narrative herausfordern, Machtstrukturen offenlegen und als Beschleuniger für affektiven, radikalen oder transformativen Diskurs fungieren. Diese Quellen stehen in produktiver Spannung zu den Funktionen 1–3.

**Theoretische Basis.** Scotts *verborgene Transkripte* — die Kritik, die untergeordnete Gruppen hinter der Bühne, außerhalb der Hörweite der Macht, aufrechterhalten. Frasers *subalterne Gegenöffentlichkeiten* — die parallelen diskursiven Arenen, in denen Mitglieder subordinierter sozialer Gruppen oppositionelle Interpretationen ihrer Identitäten, Interessen und Bedürfnisse formulieren (Fraser, 1990). Deleuze und Guattaris *Rhizom* — das nicht-hierarchische, vielfach vernetzte Netzwerk, das sich arboreszenten (baumförmigen, hierarchischen) Strukturen widersetzt (AĒRs Rhizom-Pfeiler).

**Interkulturelle Instanziierungen:**

| Gesellschaft | Institutionelle Form | Gegendiskurs-Mechanismus |
| :--- | :--- | :--- |
| Deutschland | Investigativer Journalismus (*Der Spiegel*, *CORRECTIV*), Protestbewegungen (Fridays for Future, Letzte Generation), alternative Medien, Migranten-Community-Medien | Gegendiskurs operiert innerhalb demokratischer Normen: investigativer Journalismus als institutionalisierte Subversion; Protestbewegungen als verfassungsrechtlich geschützter Ausdruck; alternative Medien an den Rändern des Mediensystems. |
| Iran | Diaspora-Satellitenkanäle (BBC Persian, Iran International), VPN-vermittelte soziale Medien, Untergrund-Musik-/Kunstszene, reformorientierte Geistliche | Gegendiskurs ist strukturell gefährlich — er operiert aus dem Exil, über verschlüsselte Kanäle und unter ständiger Bedrohung staatlicher Repression. Die Proteste nach dem Tod von *Mahsa Amini* 2022 zeigten, wie Gegendiskurs sich über soziale Medien trotz staatlicher Unterdrückung mobilisieren kann. |
| China | WeChat/Weibo-Codesprache, chinesischsprachige Auslandsmedien (Initium, The Reporter), akademisches Code-Switching, VPN-vermittelter Zugang | Gegendiskurs existiert in den Lücken der Zensur: Codesprache (*hexie* 河蟹 = „Flusskrebs" = „Harmonie" = Zensur), schnelle Screenshot-Sicherung vor Löschung, akademische Publikationen, die durch theoretische Abstraktion Grenzen austesten. Die Große Firewall schafft ein eigenes Gegendiskurs-Ökosystem für jene, die sie durchbrechen. |
| Nigeria | Social-Media-Aktivismus (#EndSARS), Bürgerjournalismus, Diaspora-Kommentare, satirische Blogs | Die #EndSARS-Bewegung (2020) demonstrierte die Kraft von Social-Media-getriebenem Gegendiskurs in einem postkolonialen Kontext. Gegendiskurs in Nigeria operiert im Zusammenspiel zwischen lokalen Sprachen, Nigerian Pidgin und Englisch, wobei jedes Register unterschiedliche politische Konnotationen trägt. |
| Russland | Telegram-Kanäle (besonders seit 2022), digitalisierte Samizdat-Tradition, Exilmedien (Meduza, Novaya Gazeta Europe), anonyme Foren | Seit 2022 wurde Gegendiskurs in Russland in verschlüsselte, anonymisierte und exilbasierte Kanäle gedrängt. Telegram fungiert gleichzeitig als Informationsquelle und als Gegen-Öffentlichkeit. |

**Zentrale Erkenntnis:** Gegendiskurs ist per Definition am schwierigsten zu beobachten. Er operiert in Räumen, die darauf ausgelegt sind, sich institutioneller Überwachung zu entziehen — verschlüsselte Nachrichtendienste, Codesprache, Exilplattformen, pseudonyme Accounts. AĒRs Verpflichtung, nur öffentlich zugängliche Daten zu beobachten (Manifest, §13.8 Quellenauswahlkriterien) bedeutet, dass der politisch bedeutsamste Gegendiskurs der am wenigsten beobachtbare sein kann. Dies ist kein Versagen des Systems — es ist eine strukturelle Einschränkung, die für jeden Sondenkontext dokumentiert werden muss.

**Die dynamische Grenze.** Die vier Funktionen sind keine statischen Kategorien. Diskursakteure bewegen sich über die Zeit zwischen Funktionen. Eine Gegendiskursbewegung, die Macht erlangt, kann zur Machtlegitimation übergehen (z. B. der ANC von der Anti-Apartheid-Bewegung zur Regierungspartei). Eine epistemische Autorität kann delegitimiert und an die Ränder gedrängt werden (z. B. traditionelle Medien, die in bestimmten Kontexten ihre Autorität an soziale Medien verlieren). AĒRs longitudinale Beobachtung (WP-005) sollte diese funktionalen Übergänge als Phänomene von Interesse verfolgen, nicht als Klassifikationsfehler.

---

## 4. Das Etisch/Emische Duale Tagging-System

### 4.1 Theoretische Fundierung

Die etisch/emische Unterscheidung hat ihren Ursprung in der Linguistik (Pike, 1967) und wurde von der Kulturanthropologie übernommen (Harris, 1976):

- **Etisch** (von *phonetisch*): ein vom Beobachter auferlegter, interkulturell anwendbarer analytischer Rahmen. Der Beobachter klassifiziert Phänomene gemäß einem universellen Schema, das kontextübergreifenden Vergleich ermöglicht. Die etische Perspektive opfert lokale Bedeutung zugunsten globaler Vergleichbarkeit.
- **Emisch** (von *phonemisch*): eine von Teilnehmern abgeleitete, kulturspezifische Beschreibung von Phänomenen. Der Analyst beschreibt Phänomene unter Verwendung von Kategorien, die für die Menschen innerhalb der Kultur bedeutsam sind. Die emische Perspektive opfert Vergleichbarkeit zugunsten lokaler Validität.

Keine der beiden Perspektiven ist allein hinreichend. Rein etische Analyse zwingt fremde Kategorien auf und verfehlt lokale Bedeutung. Rein emische Analyse produziert reiche ethnographische Beschreibungen, die kontextübergreifend nicht verglichen werden können. AĒR braucht beides — gleichzeitig.

### 4.2 Operationalisierung in der Pipeline

Das Duale Tagging-System bettet die etisch/emische Unterscheidung direkt in AĒRs Datenarchitektur über die `SilverMeta`-Schicht (ADR-015) ein:

**Die etische Schicht (Globale Vergleichbarkeit).** Jede Sonde erhält eine abstrakte funktionale Klassifikation:

```python
class ProbeEticTag(BaseModel):
    """Etische Schicht: vom Beobachter auferlegte funktionale Klassifikation."""
    discourse_function: Literal[
        "epistemic_authority",
        "power_legitimation",
        "cohesion_identity",
        "subversion_friction"
    ]
    function_confidence: float  # 0.0–1.0, Konfidenz des Forschers in die Klassifikation
    secondary_function: str | None = None  # viele Quellen erfüllen mehrere Funktionen
    classification_date: str
    classified_by: str  # Forscher-Identifikator zur Provenienz
```

Diese Klassifikation ermöglicht ClickHouse, Metriken über kulturell äquivalente Quellen hinweg zu aggregieren — z. B. Sentimenttrends über alle Quellen der „epistemischen Autorität" weltweit zu vergleichen, unabhängig von ihrer institutionellen Form.

**Die emische Schicht (Lokale Realität).** Jede Sonde erhält zudem unübersetzte, kontextspezifische Metadaten, die ihre kulturelle Spezifität bewahren:

```python
class ProbeEmicTag(BaseModel):
    """Emische Schicht: von Teilnehmern abgeleiteter, kulturspezifischer Kontext."""
    local_designation: str        # z. B. "kisha kurabu", "Ulama", "Feuilleton"
    cultural_context: str         # Freitext-Beschreibung der lokalen Bedeutung
    local_language: str           # ISO 639-1 Code
    societal_role_description: str  # was diese Quelle für ihre Gemeinschaft bedeutet
    emic_categories: list[str]    # lokal bedeutsame Kategorien, unübersetzt
```

Die emische Schicht wird *niemals* für interkulturelle Aggregation verwendet. Sie wird für Progressive Disclosure bewahrt — um einem Analysten, der von einem globalen Aggregat ins Detail geht, den lokalen Kontext einer spezifischen Quelle verständlich zu machen.

### 4.3 Multifunktionalität und dynamische Klassifikation

Die meisten realen Quellen erfüllen gleichzeitig mehrere Diskursfunktionen. Die Tagesschau ist sowohl eine epistemische Autorität (Normsetzung durch informationelle Grundlinie) als auch ein Kanal der Machtlegitimation (als staatlich finanzierter Sender, dessen redaktionelle Unabhängigkeit strukturell durch die parteipolitisch proportionale Governance der ARD beeinflusst wird). Die New York Times ist sowohl epistemische Autorität als auch zeitweise Kanal der Machtlegitimation (ihre Meinungsseite hat historisch Establishment-Positionen widergespiegelt) und Vehikel des Gegendiskurses (ihr investigativer Journalismus hat Regierungsverfehlungen aufgedeckt).

Der etische Tag muss Multifunktionalität berücksichtigen durch:

1. **Primäre und sekundäre Funktions-Tags.** Jede Quelle hat eine primäre Diskursfunktion und optional eine oder mehrere sekundäre Funktionen.
2. **Funktionsgewichtungen.** Für Quellen mit ausgeprägter Multifunktionalität sollte der etische Tag konfidenzgewichtete Funktionszuweisungen enthalten (z. B. `epistemic_authority: 0.6, power_legitimation: 0.3, cohesion_identity: 0.1`).
3. **Temporale Funktionsverschiebungen.** Die Funktionsklassifikation ist nicht permanent. Quellen, die ihre diskursive Rolle über die Zeit ändern (z. B. ein vormals unabhängiges Medium, das vom Staat vereinnahmt wird), sollten aktualisierte etische Tags mit zeitlichen Markern erhalten.

### 4.4 Der Klassifikationsprozess

Die Zuweisung etischer/emischer Tags ist keine technische Aufgabe — sie ist ein interdisziplinärer Forschungsakt. Der vorgeschlagene Klassifikationsprozess:

**Schritt 1: Nominierung durch Regionalexperten.** Für jede Kulturregion identifiziert ein Area-Studies-Wissenschaftler Kandidatenquellen für jede Diskursfunktion. Der Wissenschaftler liefert eine initiale emische Beschreibung (was diese Quelle in ihrem kulturellen Kontext bedeutet) und eine vorgeschlagene etische Klassifikation (welche Diskursfunktion sie erfüllt).

**Schritt 2: Peer Review.** Ein zweiter Experte — idealerweise aus derselben Kulturregion, aber mit anderem disziplinären Hintergrund (z. B. Politikwissenschaft, wenn der erste Experte Anthropologe ist) — begutachtet die vorgeschlagene Klassifikation. Meinungsverschiedenheiten werden dokumentiert, nicht per Dekret aufgelöst.

**Schritt 3: Technische Machbarkeitsbewertung.** Das Ingenieurteam bewertet, ob die Quelle technisch für AĒRs Crawler-Ökosystem zugänglich ist: öffentliche Endpunkte, API-Verfügbarkeit, Vereinbarkeit mit Nutzungsbedingungen, Datenformat (WP-003 §2.2 Zugangsdimensionen).

**Schritt 4: Ethische Prüfung.** Gemäß WP-006 (§5.2) durchläuft jede neue Sonde eine ethische Prüfung: Birgt die Beobachtung ein Schadensrisiko für die beobachtete Gemeinschaft? Gibt es verwundbare Bevölkerungsgruppen, deren Diskurs exponiert würde?

**Schritt 5: Registrierung.** Die Quelle wird in der PostgreSQL-Tabelle `sources` registriert, ein Source Adapter wird implementiert oder konfiguriert, die etischen/emischen Tags werden in den Quellmetadaten gespeichert, und die Klassifikationsrationale wird in einem Sonden-Registrierungsprotokoll dokumentiert.

---

## 5. Die Sondenkonstellation: Vom Einzelbefund zur systemischen Beobachtung

### 5.1 Das Minimum Viable Probe Set

Eine einzelne Quelle kann den Diskurs einer Gesellschaft nicht repräsentieren. Ein Regierungs-RSS-Feed erfasst die Machtlegitimationsfunktion; er sagt nichts über Gegendiskurs, Identitätsbildung oder auch nur epistemische Autorität (die außerhalb des Staates angesiedelt sein kann). AĒRs analytische Kraft entsteht nicht durch einzelne Sonden, sondern durch **Sondenkonstellationen** — Sets von Quellen, die zusammengenommen alle vier Diskursfunktionen einer gegebenen Gesellschaft abdecken.

Das **Minimum Viable Probe Set (MVPS)** für eine gegebene Gesellschaft ist das kleinste Set von Quellen, das alle vier Diskursfunktionen mit mindestens einer Sonde abdeckt. Ein MVPS für Deutschland könnte bestehen aus:

| Funktion | Sonde | Plattformtyp |
| :--- | :--- | :--- |
| Epistemische Autorität | Tagesschau RSS | RSS |
| Machtlegitimation | Bundesregierung RSS | RSS |
| Kohäsion & Identität | (zu bestimmen — kulturelle Medien) | offen |
| Subversion & Friktion | (zu bestimmen — investigativ/aktivistisch) | offen |

AĒRs Sonde 0 (§13.10) deckt nur die ersten beiden Zeilen ab — beide vom selben Plattformtyp (RSS) und beide aus der institutionellen Sphäre. Dies wird explizit als Kalibrierungssonde anerkannt, nicht als wissenschaftlich repräsentatives Sondenset.

### 5.2 Gewichtung und Repräsentativität

Innerhalb einer Sondenkonstellation tragen Quellen unterschiedliches Gewicht im Diskurs der Gesellschaft. Die Tagesschau erreicht täglich Millionen; ein investigativer Nischenblog erreicht Tausende. Beide sind für die Diskursbeobachtung notwendig, aber sie tragen unterschiedlich zum Gesamtbild bei.

Gewichtung wirft schwierige Fragen auf:

**Volumenbasierte Gewichtung** (Quellen nach Publikationsvolumen oder Reichweite gewichten) verstärkt die Stimmen, die ohnehin am lautesten sind. Dies reproduziert genau die Machtasymmetrien, die AĒR zu beobachten beabsichtigt.

**Gleichgewichtung** (alle Quellen ungeachtet ihrer Reichweite gleich behandeln) überrepräsentiert marginale Stimmen relativ zu ihrem gesellschaftlichen Einfluss. Dies verzerrt das makroskopische Bild.

**Funktionsbasierte Gewichtung** (Quellen nach der Bedeutsamkeit ihrer Diskursfunktion gewichten) erfordert ein theoretisches Urteil darüber, welche Funktionen „wichtiger" sind — selbst eine kulturell voreingenommene Bewertung.

Der vorgeschlagene Ansatz: **AĒR sollte Sonden nicht zu einem einzelnen Aggregat gewichten, sondern funktionsstratifizierte Ansichten beibehalten.** Das Dashboard zeigt vier Diskursfunktions-Bahnen, jede mit eigenen Metriken. Der Analyst kann die Bahn der epistemischen Autorität, die Bahn der Machtlegitimation, die Bahn der Identitätsbildung und die Bahn des Gegendiskurses getrennt sehen — oder sie überlagern, um interfunktionale Dynamiken zu beobachten (z. B. wie Gegendiskurs auf Machtlegitimationsnarrative reagiert). Dies vermeidet das Gewichtungsproblem, indem die Zusammenfassung der vier Funktionen zu einer einzigen Zahl abgelehnt wird.

### 5.3 Die Sonden-Abdeckungskarte

AĒR sollte eine **Sonden-Abdeckungskarte** pflegen — eine Visualisierung, die für jede Kulturregion zeigt, welche Diskursfunktionen durch aktive Sonden abgedeckt sind und welche unbeobachtet bleiben. Diese Karte ist das Äquivalent des Makroskops zur Gesichtsfeldanzeige eines Teleskops — sie zeigt dem Analysten nicht nur, was AĒR sieht, sondern auch, wofür es blind ist.

Die Abdeckungskarte adressiert direkt WP-006s Forderung nach „Negativraum-Visualisierung" (§7.3, Q6) und das Eingeständnis des Digital Divide im Manifest (§II).

---

## 6. Sonde 0 revisited: Klassifikation unter der Taxonomie

AĒRs operationale Sonde 0 (§13.10) kann nun formal unter der funktionalen Taxonomie klassifiziert werden:

| Quelle | Primäre Funktion | Sekundäre Funktion | Etische Konfidenz | Emischer Kontext |
| :--- | :--- | :--- | :--- | :--- |
| bundesregierung.de RSS | Machtlegitimation | Epistemische Autorität (Politik als Fakten setzend) | 0,85 | *Bundesregierung Pressemitteilungen* — offizielle Regierungskommunikation im Register des *Beamtendeutsch*, verfassungsrechtlich mandatierte Transparenz |
| tagesschau.de RSS | Epistemische Autorität | Machtlegitimation (staatsfinanziert, proportionale Governance) | 0,75 | *Öffentlich-rechtlicher Rundfunk* — öffentlicher Rundfunk unter parteipolitisch-proportionaler Governance (*Rundfunkräte*), institutionelles Register, von der deutschen Öffentlichkeit als autoritative Grundlinie wahrgenommen |

**Lücken in Sonde 0.** Die Funktionen 3 (Kohäsion & Identität) und 4 (Subversion & Friktion) sind vollständig unrepräsentiert. Beide vertretenen Quellen sind institutionell, redaktionell und deutschsprachig. Dies ist kein Defekt von Sonde 0 — sie wurde als technische Kalibrierung konzipiert (§13.10). Es bedeutet jedoch, dass jede soziologische Interpretation der Daten von Sonde 0 auf die institutionelle Sphäre des deutschen Diskurses beschränkt sein muss.

---

## 7. Architektonische Implikationen

### 7.1 Schema für die Quellenregistrierung

Die PostgreSQL-Tabelle `sources` (Migration 001) speichert derzeit grundlegende Quellenmetadaten. Die funktionale Taxonomie erfordert eine Erweiterung dieses Schemas — oder die Speicherung der Klassifikation in einer verknüpften Tabelle:

```sql
CREATE TABLE IF NOT EXISTS source_classifications (
    source_id          INTEGER REFERENCES sources(id),
    primary_function   VARCHAR(30) NOT NULL,  -- etischer Tag
    secondary_function VARCHAR(30),
    function_weights   JSONB,                 -- z. B. {"epistemic_authority": 0.6, "power_legitimation": 0.3}
    emic_designation   TEXT NOT NULL,          -- lokale Bezeichnung, unübersetzt
    emic_context       TEXT NOT NULL,          -- kulturelle Kontextbeschreibung
    emic_language      VARCHAR(10),
    classified_by      VARCHAR(100) NOT NULL,
    classification_date DATE NOT NULL,
    review_status      VARCHAR(20) DEFAULT 'pending',  -- pending, reviewed, contested
    PRIMARY KEY (source_id, classification_date)
);
```

Diese Tabelle ist additiv — sie modifiziert keine bestehenden `sources`-Einträge. Mehrere Klassifikationseinträge pro Quelle ermöglichen die zeitliche Verfolgung funktionaler Übergänge.

### 7.2 SilverMeta-Erweiterung

Die `SilverMeta`-Schicht sollte die etisch/emische Klassifikation auf jedes Dokument propagieren und so die ClickHouse-Aggregation nach Diskursfunktion ermöglichen:

```python
class DiscourseContext(BaseModel):
    """Diskursfunktions-Kontext, propagiert aus der Quellenklassifikation."""
    primary_function: str
    secondary_function: str | None = None
    emic_designation: str
```

Dieses Feld wird vom Source Adapter während der Harmonisierung befüllt, unter Zugriff auf die `source_classifications`-Tabelle. Es wird in `SilverMeta` gespeichert, nicht in `SilverCore` — die Diskursfunktionsklassifikation ist Kontext auf Quellenebene, keine Eigenschaft des einzelnen Dokuments.

### 7.3 ClickHouse-Aggregation nach Diskursfunktion

Mit der Diskursfunktion verfügbar in `SilverMeta` (und propagiert zu Gold-Metriken über die Extractor-Pipeline) kann ClickHouse Metriken nach Funktion aggregieren:

```sql
SELECT
    toStartOfDay(timestamp) AS ts,
    discourse_function,
    avg(value) AS avg_sentiment
FROM aer_gold.metrics_with_context  -- View, der Metriken + Diskursfunktion verbindet
WHERE metric_name = 'sentiment_score'
GROUP BY ts, discourse_function
ORDER BY ts;
```

Dies ermöglicht der BFF API, funktionsstratifizierte Ansichten zu liefern, und dem Dashboard, die in §5.2 beschriebene Vier-Bahnen-Visualisierung zu rendern.

### 7.4 BFF-API-Erweiterung

Ein neuer Abfrageparameter auf `/api/v1/metrics`:

- `?discourseFunction=epistemic_authority` — Metriken nach Diskursfunktion filtern
- `?discourseFunction=power_legitimation,subversion_friction` — kommagetrennt für Mehrfunktions-Ansichten

Und ein neuer Endpunkt:

- `GET /api/v1/probes/coverage` — gibt die Sonden-Abdeckungskarte zurück: für jede registrierte Kulturregion, welche Diskursfunktionen aktive Sonden haben und welche unbeobachtet sind.

---

## 8. Offene Fragen für interdisziplinäre Kooperationspartner

### 8.1 Für Kulturanthropologen und Area-Studies-Wissenschaftler

**F1: Welche Quellen erfüllen in jeder großen Kulturregion jeweils welche der vier Diskursfunktionen?**

- Dies ist die grundlegende Frage. AĒR benötigt Regionalexperten, die die Diskurslandschaft ihres Fachgebiets auf die Vier-Funktionen-Taxonomie abbilden.
- Deliverable: Regionale Sonden-Nominierungsberichte (3–5 Seiten jeweils), die Kandidatenquellen für jede Diskursfunktion identifizieren, mit emischen Beschreibungen und etischen Klassifikationen, für mindestens: Deutschland, Frankreich, Vereinigtes Königreich, USA, Brasilien, Russland, China, Japan, Indien, Nigeria, Iran, Saudi-Arabien, Indonesien, Südafrika, Mexiko.

**F2: Sind vier Diskursfunktionen ausreichend, oder muss die Taxonomie verfeinert werden?**

- Gibt es in manchen Gesellschaften Diskursfunktionen, die sich nicht sauber auf die vier Kategorien abbilden lassen? Gibt es eine fünfte Funktion (z. B. „Vermittlung" — Akteure, die zwischen Funktionen brücken)? Sollte eine Funktion aufgespalten werden (z. B. Unterscheidung religiöser epistemischer Autorität von wissenschaftlicher epistemischer Autorität)?
- Deliverable: Eine kritische Überprüfung der Taxonomie auf Basis empirischer Analyse nicht-westlicher Diskurslandschaften.

**F3: Wie sollte AĒR mit Quellen umgehen, deren Diskursfunktion sich über die Zeit verschiebt?**

- Wenn ein vormals unabhängiges Medium vom Staat vereinnahmt wird, vollzieht es einen Übergang von epistemischer Autorität (oder Gegendiskurs) zu Machtlegitimation. Wie sollte dieser Übergang dokumentiert werden, und wann sollte der etische Tag aktualisiert werden?
- Deliverable: Ein Protokoll zur Erkennung und Dokumentation funktionaler Übergänge, mit Kriterien für die Reklassifikation.

### 8.2 Für vergleichende Politikwissenschaftler

**F4: Wie sollte AĒR Sonden innerhalb einer Sondenkonstellation gewichten?**

- Der funktionsstratifizierte Ansatz (§5.2) vermeidet das Gewichtungsproblem, indem er die Aggregation über Funktionen hinweg ablehnt. Aber innerhalb einer Funktion können mehrere Sonden dieselbe Funktion mit unterschiedlicher Reichweite erfüllen. Wie sollte intrafunktionale Gewichtung gehandhabt werden?
- Deliverable: Eine Gewichtungsmethodik für intrafunktionale Sondenaggregation, gestützt auf Stichprobentheorie und Mediensystemanalyse.

**F5: Wie verhält sich AĒRs funktionale Taxonomie zu bestehenden Mediensystem-Typologien?**

- Hallin und Mancinis (2004) drei Modelle von Medien und Politik (Liberal, Demokratisch-Korporatistisch, Polarisiert-Pluralistisch) bilden die dominante Typologie in der vergleichenden Medienwissenschaft. Wie verhält sich die funktionale Taxonomie zu diesen Modellen — erweitert, ergänzt oder stellt sie sie in Frage?
- Deliverable: Eine Abbildung zwischen Hallin-Mancini-Mediensystemtypen und AĒRs Diskursfunktionstaxonomie, die identifiziert, wo die Taxonomien übereinstimmen und wo sie divergieren.

### 8.3 Für Computational Social Scientists

**F6: Kann die Diskursfunktion rechnerisch erkannt werden, oder muss sie von menschlichen Experten zugewiesen werden?**

- Gibt es ein Signal auf Textebene, das epistemischen Autoritätsdiskurs von Machtlegitimationsdiskurs unterscheidet? Könnte ein trainierter Klassifikator Diskursfunktions-Tags automatisch zuweisen und so die Abhängigkeit von Regionalexperten reduzieren?
- Deliverable: Eine Machbarkeitsstudie zur automatisierten Diskursfunktionsklassifikation, unter Verwendung eines gelabelten Korpus von Quellen mit bekannten Funktionszuweisungen.

**F7: Wie sollte AĒR interfunktionale Dynamiken messen?**

- Die Interaktion zwischen Diskursfunktionen — wie Gegendiskurs auf Machtlegitimation reagiert, wie epistemische Autorität zwischen konkurrierenden Identitätsnarrativen vermittelt — ist wohl das analytisch interessanteste Phänomen, das AĒR beobachten kann. Welche Metriken erfassen diese Dynamiken?
- Deliverable: Ein Set interfunktionaler Metriken (z. B. Reaktionslatenz zwischen Funktionen, Themenkonvergenz/-divergenz über Funktionen, Entity-Ko-Okkurrenz über Funktionen), mit mathematischen Definitionen und Machbarkeitsbewertung der Berechnung.

---

## 9. Referenzen

- Anderson, B. (1991). *Imagined Communities: Reflections on the Origin and Spread of Nationalism* (revised ed.). Verso.
- Collier, D. & Mahon, J. E. (1993). „Conceptual 'Stretching' Revisited: Adapting Categories in Comparative Analysis." *American Political Science Review*, 87(4), 845–855.
- Dogan, M. & Pelassy, D. (1990). *How to Compare Nations: Strategies in Comparative Politics* (2. Aufl.). Chatham House.
- Foucault, M. (1969). *L'Archéologie du savoir* (Archäologie des Wissens). Gallimard.
- Fraser, N. (1990). „Rethinking the Public Sphere: A Contribution to the Critique of Actually Existing Democracy." *Social Text*, 25/26, 56–80.
- Gramsci, A. (1971). *Selections from the Prison Notebooks* (Hrsg. u. Übers. Hoare, Q. & Nowell-Smith, G.). International Publishers.
- Hallin, D. C. & Mancini, P. (2004). *Comparing Media Systems: Three Models of Media and Politics*. Cambridge University Press.
- Harris, M. (1976). „History and Significance of the Emic/Etic Distinction." *Annual Review of Anthropology*, 5, 329–350.
- Peters, B. G. (1998). *Comparative Politics: Theory and Methods*. NYU Press.
- Pike, K. L. (1967). *Language in Relation to a Unified Theory of the Structure of Human Behavior* (2. Aufl.). Mouton.
- Santos, B. de S. (2014). *Epistemologies of the South: Justice Against Epistemicide*. Routledge.
- Sartori, G. (1970). „Concept Misformation in Comparative Politics." *American Political Science Review*, 64(4), 1033–1053.
- Scott, J. C. (1990). *Domination and the Arts of Resistance: Hidden Transcripts*. Yale University Press.

---

## Anhang A: Zuordnung zu AĒR Offene Forschungsfragen (§13.6)

| §13.6 Frage | WP-001 Abschnitt | Status |
| :--- | :--- | :--- |
| 1. Sondenauswahl | §2–§6 (vollständige Behandlung) | Adressiert — funktionale Taxonomie, duales Tagging, Sondenkonstellations-Methodik |
| 2. Bias-Kalibrierung | §3.3 (Machtlegitimations-Bias), §3.5 (Beobachtbarkeit des Gegendiskurses) | Teilweise adressiert — WP-003 gewidmet |
| 4. Interkulturelle Vergleichbarkeit | §4 (etisch/emisch als Vergleichbarkeitsmechanismus) | Grundlage gelegt — erweitert in WP-004 |
| 6. Beobachtereffekt | §5.3 (Sondenabdeckung als Transparenzinstrument) | Vorweggenommen — WP-006 gewidmet |

## Anhang B: Vorlage für das Sonden-Registrierungsprotokoll

Für jede neue Sonde, die dem AĒR-System hinzugefügt wird, sollte das folgende Protokoll ausgefüllt und zusammen mit dem `sources`-Eintrag gespeichert werden:

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
    local_designation: ""          # lokale Bezeichnung, unübersetzt
    societal_role: ""              # was diese Quelle für ihre Gemeinschaft bedeutet
    cultural_notes: ""             # relevanter kultureller Kontext

  technical_assessment:
    platform_type: ""              # rss, api, web_scrape, etc.
    access_method: ""
    data_format: ""
    tos_compliant: true
    publication_volume: ""         # geschätzte Dokumente pro Tag

  ethical_review:
    reviewer: ""
    review_date: ""
    risk_assessment: ""            # low, medium, high
    vulnerable_populations: false
    harm_potential: ""

  bias_documentation:
    known_demographic_skew: ""
    editorial_constraints: ""
    platform_effects: ""           # aus WP-003
    what_is_not_observed: ""       # explizite Lückendokumentation
```