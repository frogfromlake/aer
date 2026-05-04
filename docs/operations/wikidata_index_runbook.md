# Wikidata Alias Index — Quarterly Runbook

**Purpose:** end-to-end procedure for refreshing the Phase-118 Wikidata alias index — production build, output validation, deployment to GHCR + compose stack, and rollback. Written so that future-self (or another operator) can execute it cold three months from now.

**Audience:** the operator running the quarterly refresh on a local 12-core / 32 GB workstation. The GitHub Actions workflow (`.github/workflows/wikidata_index_rebuild.yml`) follows the same logical steps with workflow-runner specifics; this runbook is the canonical local procedure and the workflow's reference.

**Architectural framing:** Phase 118 is *Disambiguation*, not Discovery — the alias index is a metadata sidecar over `aer_gold.entities`, not the canonical entity registry. See **ADR-027** for why this matters: index coverage is intentionally scoped to institutional editorial discourse, and missing entities are not bugs. Bucket curation has a defined ceiling; new domains do not require synchronous YAML expansion. This runbook assumes that frame.

**Build mechanism:** dump-streaming N-Triples parser over `latest-truthy.nt.bz2` via `pyoxigraph` (Phase 118b — superseded the original SPARQL pipeline after empirical evidence of public-endpoint timeouts on bucket-discovery queries).

---

## Table of Contents

- [Prerequisites](#prerequisites)
- [Part 1 — Production Build](#part-1--production-build)
- [Part 2 — Validation](#part-2--validation)
- [Part 3 — Deployment](#part-3--deployment)
- [Part 4 — Rollback](#part-4--rollback)
- [Part 5 — Build Registry](#part-5--build-registry)
- [Appendix — Common Failure Modes](#appendix--common-failure-modes)

---

## Prerequisites

| Requirement | Value |
|---|---|
| Working directory | `/home/nelix/projects/aer` |
| Python venv | `/tmp/wd_venv/` (PyYAML, pyoxigraph≥0.4.11, requests, tenacity) |
| Dump location | `/home/nelix/wikidata-build/latest-truthy.nt.bz2` (~43 GB) |
| Disk free | ≥ 10 GB for build artefacts + Pass-B/C caches |
| Memory | ≥ 8 GB free during build (peak ~6-8 GB Python RSS at full scope) |
| Tools | `lbzip2`, `bzip2`, `grep`, `jq`, `curl`, `docker`, `gh` (optional) |
| Network | Pass C SPARQL needs ~5-7 min uninterrupted; checkpoint absorbs blips |
| GHCR auth | PAT with `write:packages` scope (see Part 3 Step 0a) |

**Refresh the dump** before each quarterly build. Wikidata publishes new truthy dumps every Wednesday:

```bash
cd /home/nelix/wikidata-build
wget --continue --tries=3 --read-timeout=300 \
  https://dumps.wikimedia.your.org/wikidatawiki/entities/latest-truthy.nt.bz2
# ~10-15 min on a fast connection; resumes on partial download
```

The `your.org` mirror is a polite-mirror serving the same content as `dumps.wikimedia.org` — useful when the official endpoint is congested. Either works.

**Refresh the venv** if pyoxigraph or requests have moved:

```bash
/tmp/wd_venv/bin/pip install -U PyYAML pyoxigraph requests tenacity
```

The Python version requirement follows `services/analysis-worker` (3.14+ as of writing).

---

## Part 1 — Production Build

### 1.1 Pre-build cleanup

Remove stale outputs from previous builds before starting:

```bash
rm -f /home/nelix/wikidata-build/wikidata_aliases.db \
      /home/nelix/wikidata-build/wikidata_aliases.db.sha256 \
      /home/nelix/wikidata-build/wikidata_aliases.db.passb_cache.pickle \
      /home/nelix/wikidata-build/wikidata_aliases.db.passc_cache.pickle
```

Keep this strict — leftover Pass-B-cache from an earlier YAML or `--languages` setting is a silent-correctness hazard. The cache load checks `snapshot_date` and `languages` but does **not** revalidate the bucket DSL.

### 1.2 Launch the build

```bash
/home/nelix/projects/aer/scripts/wikidata_validate.sh prod
```

This wraps `build_wikidata_index.py` with the canonical CLI args (default dump path, default buckets file, default output path). For one-off runs against a non-default location, invoke the build script directly:

```bash
/tmp/wd_venv/bin/python /home/nelix/projects/aer/scripts/build_wikidata_index.py \
  --dump-path /home/nelix/wikidata-build/latest-truthy.nt.bz2 \
  --buckets-file /home/nelix/projects/aer/services/analysis-worker/data/wikidata_type_buckets.yaml \
  --output-path /home/nelix/wikidata-build/wikidata_aliases.db
```

Both forms produce identical output (determinism contract).

### 1.3 What to monitor

The script logs at three rhythms:

* **Triple progress** every 5,000,000 parsed triples (Pass B). Format: `scanned N triples (skipped K malformed)`.
* **Heartbeat** every 60 s wall-clock during Pass B with elapsed-time, throughput, and resident memory. Format: `PassB heartbeat elapsed=Ns triples=N rate=N/s candidates=N rss=NMB`.
* **Pass-C batch progress** every 10 batches. Format: `Pass C: batch N/M (K QIDs)`.

**Memory watermark.** RSS grows during Pass B as the candidate accumulator fills. For a `de,en,fr` build at full bucket scope, expect:

| Scan % | RSS (GB) |
|---|---|
| 1 | 0.1 |
| 25 | 3.2 |
| 50 | 6.4 |
| 75 | 7.5 (saturating) |
| 100 | 8-10 (final) |

If RSS exceeds 15 GB before 50% scan, **abort and investigate** — either the YAML grew dramatically or label-language filtering has regressed (label storage dominates per-candidate memory; see commit history of `_hydrate_candidates`).

**ETA.** At ~134k triples/sec, full-dump Pass B takes ~10-11 hours on a 12-core laptop. Pass C adds ~30-40 minutes for ~250k QIDs. Total: ~11-12 h end-to-end.

### 1.4 Resume after interruption

The build has two checkpoint layers:

* **Pass-B cache** at `<output>.passb_cache.pickle`. Written once, after Pass B completes. Re-running the script with the same dump + languages skips Pass B entirely and goes straight to Pass C.
* **Pass-C cache** at `<output>.passc_cache.pickle`. Written every 100 SPARQL batches and on the last batch. A mid-Pass-C network outage that exceeds the tenacity retry budget (8 retries × max 120 s) resumes from the last completed checkpoint rather than batch 0.

Resuming is automatic — no flag needed. Just re-run the same command. The cache validates `snapshot_date` and `languages` (Pass B) and a hash of the candidate-QID set (Pass C); a mismatch triggers a clean re-run rather than silent corruption.

If the script crashes mid-flight for any reason (network, signal, OOM, disk pressure), check the cache files exist at the expected paths, then re-run. Do not delete the caches.

---

## Part 2 — Validation

The build's exit-zero is necessary but not sufficient — it confirms the code ran cleanly, not that the output is semantically correct. Always run validation before deployment.

### 2.1 Layered validation pipeline

`scripts/wikidata_validate.sh` provides three validation stages of increasing scope. The Layer-1 and Layer-2 stages are smoke tests against subset samples; only the Production-build output (`/home/nelix/wikidata-build/wikidata_aliases.db`) goes to deployment.

```bash
# Optional — re-run before each release as a regression gate against the YAML.
# Each is independent; layer-1 + layer-2 take ~25 min total and gate the
# pipeline before the multi-hour prod build.
scripts/wikidata_validate.sh layer1   # ~5 min — canonical-7 + negatives + determinism
scripts/wikidata_validate.sh layer2   # ~20 min — 2 GB sample, sanity ceilings
```

These are recommended before YAML changes ship. After a Production build, the next subsection's spot-check is the operational gate.

### 2.2 Production-build spot-check

Run the validation snippet below. It checks:

1. All canonical entities are present in the correct bucket
2. Out-of-scope entities (TV programs, generic magazines, archives) are absent
3. Bucket distribution is in the expected order of magnitude
4. Aliases include multilingual coverage with altLabels (colloquial forms)

```bash
/tmp/wd_venv/bin/python <<'PYEOF'
import sqlite3
DB = '/home/nelix/wikidata-build/wikidata_aliases.db'
conn = sqlite3.connect(DB)

print("=" * 70)
print("BUILD METADATA")
print("=" * 70)
for k, v in conn.execute("SELECT key, value FROM build_metadata ORDER BY key"):
    print(f"  {k}: {v}")

print()
print("=" * 70)
print("BUCKET DISTRIBUTION")
print("=" * 70)
counts: dict[str, int] = {}
for tb, n in conn.execute(
    "SELECT type_buckets, COUNT(*) FROM entities GROUP BY type_buckets"
):
    for b in tb.split(","):
        counts[b] = counts.get(b, 0) + n
for bucket in sorted(counts, key=lambda b: -counts[b]):
    print(f"  {bucket:35s} {counts[bucket]:>8d}")
print(f"  {'(total entities)':35s} {conn.execute('SELECT COUNT(*) FROM entities').fetchone()[0]:>8d}")
print(f"  {'(total aliases)':35s} {conn.execute('SELECT COUNT(*) FROM aliases').fetchone()[0]:>8d}")

print()
print("=" * 70)
print("CANONICAL ENTITIES — POSITIVE TESTS")
print("=" * 70)
expected = {
    "Q64":     ("Berlin",                "cities_population_threshold"),
    "Q61053":  ("Olaf Scholz",           "politicians"),
    "Q566257": ("Friedrich Merz",        "politicians"),
    "Q313827": ("Bundesregierung DE",    "government_agencies"),
    "Q154797": ("German Bundestag",      "government_agencies"),
    "Q458":    ("European Union",        "eu_institutions"),
    "Q162222": ("Deutsche Bundesbank",   "central_banks"),
    "Q183":    ("Germany",               "sovereign_states"),
    "Q142":    ("France",                "sovereign_states"),
    "Q567":    ("Angela Merkel",         "politicians"),
    "Q1726":   ("Munich",                "cities_population_threshold"),
    "Q8889":   ("European Commission",   "eu_institutions"),
    "Q8896":   ("European Parliament",   "eu_institutions"),
    "Q8901":   ("European Central Bank", "eu_institutions"),
}
fail = 0
for qid, (name, want_bucket) in expected.items():
    row = conn.execute(
        "SELECT type_buckets, sitelink_count FROM entities WHERE wikidata_qid=?",
        (qid,),
    ).fetchone()
    if row is None:
        print(f"  FAIL {qid:8s} ({name}): NOT IN INDEX (expected {want_bucket})"); fail += 1
    elif want_bucket not in row[0].split(","):
        print(f"  FAIL {qid:8s} ({name}): buckets={row[0]} (expected {want_bucket})"); fail += 1
    else:
        print(f"  PASS {qid:8s} ({name:25s}) sitelinks={row[1]:>4d}  buckets={row[0]}")

print()
print("=" * 70)
print("OUT-OF-SCOPE / NEGATIVE CONTROLS")
print("=" * 70)
out_of_scope = [
    ("Q703907", "Tagesschau (TV program)"),
    ("Q131478", "Der Spiegel (magazine)"),
    ("Q565",    "Wikimedia Commons (archives)"),
    ("Q1",      "Universe"),
    ("Q5",      "Human (the class)"),
]
for qid, name in out_of_scope:
    row = conn.execute("SELECT type_buckets FROM entities WHERE wikidata_qid=?", (qid,)).fetchone()
    if row is None:
        print(f"  PASS {qid:8s} ({name:35s}) not in index — correct")
    else:
        print(f"  FAIL {qid:8s} ({name:35s}) leaked into buckets={row[0]}"); fail += 1

print()
print(f"validation result: {'PASSED' if fail == 0 else f'FAILED ({fail} assertions)'}")
PYEOF
```

**Expected order-of-magnitude bucket sizes** for the current YAML (de/en/fr scope):

| Bucket | Expected count | Notes |
|---|---|---|
| `politicians` | 200k–250k | Dominant — global P31=Q5 + politician P106 + sitelinks≥3 |
| `political_parties` | 10k–20k | |
| `government_agencies` | 5k–10k | |
| `broadcasters` | 5k–10k | |
| `cities_population_threshold` | 4k–8k | P1082 ≥ 50k filter |
| `news_organisations` | 3k–8k | |
| `international_organizations` | 500–1500 | |
| `subnational_entities` | 200–500 | |
| `central_banks` | 100–300 | |
| `sovereign_states` | ~200 | UN members + few non-UN |
| `eu_institutions` | ≤ 28 | Curated `qid_any` list — strict ceiling |
| **Total entities** | 250k–300k | |
| **Total aliases** | 800k–1.2M | |

If `eu_institutions > 28` or `sovereign_states > 250`, the bucket DSL has regressed. Bisect the YAML.

If `politicians < 100k`, Pass C sitelinks hydration likely failed mid-build (silent partial success). Check `Pass C complete: N/M QIDs got sitelinks` in the build log — `N` should be ≥ 99% of `M`.

### 2.3 Determinism check (optional, for major releases)

Re-run the build from cache and compare hashes:

```bash
# Backup the just-built DB
cp /home/nelix/wikidata-build/wikidata_aliases.db /tmp/wd_first.db
HASH1=$(sha256sum /tmp/wd_first.db | cut -d' ' -f1)

# Force cache reload (Pass B is the slow part; Pass C is ~30 min)
rm /home/nelix/wikidata-build/wikidata_aliases.db
scripts/wikidata_validate.sh prod  # ~30-40 min (Pass B from cache)

HASH2=$(sha256sum /home/nelix/wikidata-build/wikidata_aliases.db | cut -d' ' -f1)
[ "$HASH1" = "$HASH2" ] && echo "determinism OK" || echo "DETERMINISM BROKEN"
```

This is paranoia-level — the determinism contract is enforced by the build script's design (sorted-tuple insertion + `iterdump` canonicalisation round-trip) and is unit-tested at fixture scale. Verifying at production scale once per major release is a cheap insurance.

---

## Part 3 — Deployment

This is the steady-state deployment procedure. The validated DB at `/home/nelix/wikidata-build/wikidata_aliases.db` becomes a GHCR image, gets pinned in `.env`, and replaces the running volume contents.

### Schritt 0a — GHCR Personal Access Token erstellen

Du brauchst ein PAT mit `write:packages` scope für den Image-Push (Schritt 4).

1. Browser öffnen: <https://github.com/settings/tokens>
2. **Generate new token** → **Generate new token (classic)**
3. **Note:** z.B. `aer-ghcr-write` (für deine Übersicht)
4. **Expiration:** deine Wahl — empfehle 90 Tage (du baust quartalsweise neu, der Token überlebt einen Build-Zyklus)
5. **Select scopes:** nur `write:packages` aktivieren — schließt `read:packages` automatisch ein. Andere Scopes NICHT aktivieren (Principle of Least Privilege).
6. **Generate token** → Token kopieren (wird nur einmal angezeigt!)
7. Token als Environment-Variable für die laufende Shell setzen:

```bash
export GHCR_PAT="ghp_xxxxxxxxxxxxxxxxxxxx"
```

Persistieren über Shell-Restarts hinweg (optional): Token in `~/.ghcr_pat` legen, `chmod 600 ~/.ghcr_pat`, dann in `.bashrc`/`.zshrc`:

```bash
[ -f ~/.ghcr_pat ] && export GHCR_PAT=$(cat ~/.ghcr_pat)
```

Schritt 4 nutzt dann `echo "${GHCR_PAT}" | docker login …`.

### Schritt 0b — `.env` für Production konfigurieren

Öffne `/home/nelix/projects/aer/.env` (NICHT `.env.example`) und finde den Phase-118-Block. Default-Stand sieht so aus:

```bash
WIKIDATA_INDEX_PATH=/data/wikidata/wikidata_aliases.db
WIKIDATA_INDEX_SHA256=
WIKIDATA_INDEX_TAG=latest
```

Setze diese Werte für den neuen Production-Rollout:

```bash
WIKIDATA_INDEX_PATH=/data/wikidata/wikidata_aliases.db
WIKIDATA_INDEX_SHA256=<sha256 aus build_metadata, z.B. 93f59582…>
WIKIDATA_INDEX_TAG=<snapshot-date, z.B. 2026-05-03>
```

**Was die drei Variablen tun:**

| Variable | Bedeutung |
|---|---|
| `WIKIDATA_INDEX_PATH` | Read-only Mount-Pfad im Worker-Container. Unverändert lassen — entspricht dem `wikidata_data` Volume aus `compose.yaml`. |
| `WIKIDATA_INDEX_SHA256` | Worker macht beim Boot Hash-Verification (`entity_linking.py` `WikidataAliasIndex.__init__`). Mismatch → fail-fast Boot-Abort. **Load-bearing in Production.** Leer = keine Verification (nur Dev). |
| `WIKIDATA_INDEX_TAG` | Image-Tag den der `wikidata-index-init` Compose-Service zieht. **Pin auf Date-Tag** statt `latest` — sonst wechselt das Image beim nächsten Quartals-Build automatisch und bricht Determinismus (Image-Pinning-Policy). |

Die `.env`-Änderung wird erst in **Schritt 6** wirksam (Stack-Restart). Bis dahin laufen die Container mit dem alten Tag weiter — kein Disruption-Risiko durch das Speichern der Datei jetzt.

### Schritt 1 — Cache-Cleanup

Build-Caches werden retained bis explizit validiert. Nach erfolgreicher Validation in Part 2: cleanup.

```bash
/tmp/wd_venv/bin/python /home/nelix/projects/aer/scripts/build_wikidata_index.py \
  --validated --output-path /home/nelix/wikidata-build/wikidata_aliases.db
```

**Erwartete Ausgabe:** `Removed Pass B cache at …` und `No Pass C cache at … — nothing to remove.` (Pass-C-Cache wurde am Ende von Part 1 vom Skript selbst gelöscht.)

### Schritt 2 — Build-Context stagen

Der Dockerfile erwartet die Artefakte unter `infra/wikidata-index/data/`.

```bash
cd /home/nelix/projects/aer
mkdir -p infra/wikidata-index/data
cp /home/nelix/wikidata-build/wikidata_aliases.db        infra/wikidata-index/data/
cp /home/nelix/wikidata-build/wikidata_aliases.db.sha256 infra/wikidata-index/data/

# Sanity-Check: Hash muss matchen
cd infra/wikidata-index/data && sha256sum -c wikidata_aliases.db.sha256 && cd /home/nelix/projects/aer
```

**Pre-flight check:** Stelle sicher dass `infra/wikidata-index/data/` in `.gitignore` ist.

```bash
git check-ignore infra/wikidata-index/data/wikidata_aliases.db && echo "ignored OK" || echo "NOT IGNORED — fix .gitignore zuerst"
```

### Schritt 3 — Image bauen

Tag-Schema folgt dem CI-Workflow (`workflow_dispatch` würde `<date>` + `latest` + `hash-<sha>` taggen).

```bash
SNAPSHOT_DATE=<z.B. 2026-05-03>
DB_HASH=<sha256 aus build_metadata>
HASH_SHORT=${DB_HASH:0:7}
IMAGE=ghcr.io/frogfromlake/aer-wikidata-index

docker build \
  --tag ${IMAGE}:${SNAPSHOT_DATE} \
  --tag ${IMAGE}:latest \
  --tag ${IMAGE}:hash-${HASH_SHORT} \
  --label org.aer.wikidata.snapshot-date=${SNAPSHOT_DATE} \
  --label org.aer.wikidata.sha256=${DB_HASH} \
  --label org.aer.wikidata.build-method=dump-stream \
  ./infra/wikidata-index/

# Image-Self-Test (Dockerfile CMD = sha256sum -c)
docker run --rm ${IMAGE}:${SNAPSHOT_DATE}
# Expect: "OK"
```

Bei Hash-Mismatch im Self-Test → Build-Context (Schritt 2) nochmal stagen.

### Schritt 4 — Push zu GHCR

**Login mit dem PAT aus Schritt 0a:**

```bash
echo "${GHCR_PAT}" | docker login ghcr.io -u frogfromlake --password-stdin
```

Erwartete Ausgabe: `Login Succeeded`. Wird in `~/.docker/config.json` persistiert — bis zum PAT-Expiry musst du nicht erneut einloggen.

**Push** (alle drei Tags):

```bash
docker push ${IMAGE}:${SNAPSHOT_DATE}
docker push ${IMAGE}:latest
docker push ${IMAGE}:hash-${HASH_SHORT}
```

### Schritt 5 — `.env`-Werte verifizieren

Du hast die `.env`-Werte bereits in **Schritt 0b** gesetzt. Schneller Sanity-Check vor dem Stack-Restart:

```bash
grep '^WIKIDATA_INDEX_' /home/nelix/projects/aer/.env
```

Erwartete Ausgabe (exakt drei Zeilen):

```
WIKIDATA_INDEX_PATH=/data/wikidata/wikidata_aliases.db
WIKIDATA_INDEX_SHA256=<dein hash>
WIKIDATA_INDEX_TAG=<dein date>
```

Falls `WIKIDATA_INDEX_TAG` noch `latest` zeigt oder `WIKIDATA_INDEX_SHA256` leer ist → zurück zu Schritt 0b.

### Schritt 6 — Stack restart (Worker + Init-Container)

Volume muss neu befüllt werden — alte DB liegt sonst noch im Volume.

```bash
# Stop + remove init container, drop volume
docker compose stop analysis-worker wikidata-index-init
docker compose rm -f wikidata-index-init
docker volume rm aer_wikidata_data 2>/dev/null || true

# Pull neues Image, init starten
docker compose pull wikidata-index-init
docker compose up -d wikidata-index-init
docker compose logs --tail=20 wikidata-index-init
# Expect: "wikidata-index-init: copy + checksum verified" + Hash

# Worker starten
docker compose up -d analysis-worker
docker compose logs --tail=30 analysis-worker | grep -i "wikidata\|alias"
# Expect: "Wikidata alias index hash verified path=/data/wikidata/wikidata_aliases.db sha256=…"
```

**Bei Hash-Mismatch im Worker-Log:** `.env`-Hash stimmt nicht mit Image-Hash. Tag/SHA gegenchecken. Der Worker startet bewusst nicht — silent-drift guard.

### Schritt 7 — Smoke-Test im laufenden System

Crawler-Run gegen Probe-0 triggern.

```bash
make crawl
```

Nach 1-2 Minuten: `entity_links` rows checken.

```bash
# Counts
docker compose exec clickhouse clickhouse-client --query \
  "SELECT count(), uniq(wikidata_qid) FROM aer_gold.entity_links \
   WHERE timestamp >= now() - INTERVAL 10 MINUTE"

# Sample der höchsten-confidence Links
docker compose exec clickhouse clickhouse-client --query \
  "SELECT entity_text, wikidata_qid, link_confidence, link_method \
   FROM aer_gold.entity_links \
   WHERE timestamp >= now() - INTERVAL 10 MINUTE \
   ORDER BY link_confidence DESC LIMIT 20"
```

**Erwartet:** Politische Entitäten (Scholz, Merz, Bundestag, …) tauchen mit echten QIDs auf, `link_method=exact_match`, `link_confidence=1.0`.

---

## Part 4 — Rollback

Rollback ist verlustfrei: GHCR-Date-Tags sind immutable solange nicht manuell `docker manifest delete`-d, und das Pass-B-Cache der vorigen Build-Iteration kann optional zurückgespielt werden.

```bash
# .env auf alten Tag zurücksetzen
WIKIDATA_INDEX_TAG=<old-date>
WIKIDATA_INDEX_SHA256=<old-hash>

# Volume neu befüllen
docker compose stop analysis-worker wikidata-index-init
docker volume rm aer_wikidata_data
docker compose up -d wikidata-index-init analysis-worker

# Verify
docker compose logs --tail=30 analysis-worker | grep -i wikidata
```

Wenn der `<old-hash>` nicht mehr griffbereit ist: GHCR-Image-Labels lesen.

```bash
docker pull ghcr.io/frogfromlake/aer-wikidata-index:<old-date>
docker inspect ghcr.io/frogfromlake/aer-wikidata-index:<old-date> \
  --format '{{ index .Config.Labels "org.aer.wikidata.sha256" }}'
```

---

## Part 5 — Build Registry

Append a row after each successful production build. The registry exists so a future operator can answer "what was deployed when, and which dump was it built from" without spelunking through git history.

| Build Date | Snapshot | SHA256 | Size (MB) | Entities | Aliases | Languages | Notes |
|---|---|---|---|---|---|---|---|
| 2026-05-04 | 2026-05-03 | `93f59582ca304f36b42ee6379fced5c6e8ef9af0db8a961dc3c33c5385f1b5c1` | 134 | 259,869 | 989,382 | de,en,fr | First Phase-118b post-mortem-corrected build. Bucket DSL fixed (qid_any added, 12 wrong type-class QIDs replaced). ADR-027 codified. Test set switched to verified canonical-7 QIDs. |

---

## Appendix — Common Failure Modes

### Network outage during Pass C

**Symptoms:** stack trace ending in `requests.exceptions.ConnectionError` or `RemoteDisconnected`.

**Resolution:** the script's tenacity decorator now retries on transport-level errors (Patch A from 2026-05-04) with exponential backoff up to 8 attempts. If retries exhaust, the Pass-C cache (Patch B) preserves all batches up to the last completed checkpoint (every 100 batches). Re-running the same command resumes from the checkpoint.

If the cache is missing or corrupted: re-run loses up to 100 batches of Pass-C work (~2 min). Pass B is unaffected (separate cache).

### OOM during Pass B

**Symptoms:** kernel OOM-kills the python process; `dmesg | tail` shows the kill record.

**Cause:** candidate accumulator grew beyond available RAM. With the language-filter patch applied, peak RSS is ~8-10 GB at full scope — well within 32 GB. If you see RSS > 15 GB before 50% scan, the YAML has either grown the bucket scope dramatically OR the language filter has regressed.

**Resolution:** check the bucket YAML for additions; check `_hydrate_candidates` for the language filter at parse time (`obj.language in languages`). If neither has changed, file an issue.

### Cache version mismatch on resume

**Symptoms:** log line `Pass B cache has incompatible format version N (expected M); ignoring`.

**Cause:** the build script bumped `CACHE_FORMAT_VERSION` between the cache write and the re-run.

**Resolution:** delete the cache and re-run. The cache invalidation is by design — old caches with different schemas are never silently merged.

### `pyoxigraph` parse errors on `@es-419`

**Symptoms:** non-zero count in `Pass B complete: subjects=N candidates=K labels_with_lang=L elapsed=Ts` followed by `skipped X malformed triples`.

**Cause:** Wikidata occasionally produces N-Triple lines with language-tags pyoxigraph rejects (`@es-419` for Latin American Spanish, etc.) — they violate strict N-Triples but are valid Turtle.

**Resolution:** the per-line resilient parser (`_iter_triples`) skips them and continues. Skip rate is typically <0.005% of total triples — no operational concern. If the rate exceeds 0.1%, the dump may be corrupted; verify the SHA against `dumps.wikimedia.org/wikidatawiki/entities/dcatap.rdf`.

### `analysis-worker` won't start: hash mismatch

**Symptoms:** worker log line `Wikidata alias index hash mismatch — expected=X actual=Y. The index on the volume does not match the worker's expected build; refusing to start to prevent silent index drift.`

**Cause:** `.env`'s `WIKIDATA_INDEX_SHA256` doesn't match the hash baked into the image at the pinned tag.

**Resolution:** read the image label and update `.env`:

```bash
docker inspect ghcr.io/frogfromlake/aer-wikidata-index:${WIKIDATA_INDEX_TAG} \
  --format '{{ index .Config.Labels "org.aer.wikidata.sha256" }}'
```

This is the silent-drift guard working correctly — never disable it in production.

### `wikidata-index-init` exits non-zero

**Symptoms:** init container exits with non-zero code; logs show `sha256sum: WARNING: 1 computed checksum did NOT match`.

**Cause:** the image was built with a corrupted `data/wikidata_aliases.db` that doesn't match its sidecar.

**Resolution:** rebuild the image (Part 3 Step 3); the in-Dockerfile self-test (`CMD sha256sum -c`) would have caught this if you ran it (Step 3 final command).

---

## Cross-references

- **ADR-027** — Wikidata Entity Linking is Disambiguation, not Discovery (architectural framing)
- **`docs/operations/operations_playbook.md`** — Stack-level operations reference (smoke tests, observability, volume management)
- **`docs/operations/scheduled_work.md`** — Category C reference for `wikidata_index_rebuild.yml`
- **`scripts/build_wikidata_index.py`** — build script (canonical implementation)
- **`scripts/wikidata_validate.sh`** — validation pipeline (Layer-1, Layer-2, prod stages)
- **`services/analysis-worker/data/wikidata_type_buckets.yaml`** — bucket DSL (the index scope SoT)
- **`infra/wikidata-index/Dockerfile`** — image definition
- **`compose.yaml`** §3.1b — `wikidata-index-init` service
- **`.github/workflows/wikidata_index_rebuild.yml`** — CI equivalent of this runbook
