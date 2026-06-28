# Production Deploy Runbook

> The step-by-step choreography for deploying AĒR to production, as actually
> executed — the Phase 149 first deploy plus the Phase 150 CD pipeline. It covers
> **one-time provisioning**, **routine releases** (tag-driven CD), **manual deploy
> / rollback**, and the **fresh-machine build findings** every new box or fork hits.
>
> Companion runbooks: [Backup & Restore](backup_restore.md),
> [Monitoring & Alerting](monitoring.md),
> [Scheduled Work](scheduled_work.md),
> [Operations Playbook](operations_playbook.md).

## 0. Architecture in one breath

- The production box **pulls pre-built images from GHCR** — it builds nothing.
- **CD:** a `vX.Y.Z` git tag → GitHub Actions builds + pushes the four service
  images → SSHes into the box → checks out the tag → `docker compose pull` +
  `up -d --wait` (health-gated). One `git tag` ships a release.
- **Images are private** (`ghcr.io/<owner>/aer-{ingestion-api,bff-api,analysis-worker,dashboard}`);
  the box authenticates pulls with a read-only `read:packages` token.
- **Secrets are off-box (Phase 155 / ADR-046).** They live in **GitHub Actions
  Environment Secrets**; the deploy job injects them into the box's tmpfs
  (`/run/aer-secrets`, RAM-only) as Docker secrets at deploy time — never written
  to the persistent disk. The box `.env` holds only **non-secret** config. See
  **Part F** for the one-time setup, the deploy flow, and reboot recovery.
- Single box, prod overlay (`compose.prod.yaml`), Traefik TLS, the dashboard is
  profile-gated (`--profile dashboard`).

## 1. Which part do I need?

| Situation | Go to |
|---|---|
| Fresh box, never deployed | **Part A** (provisioning) → then **Part B** |
| Shipping a code change to a live box | **Part B** (routine release) |
| A deploy broke something | **Part C** (rollback) |
| A build fails on a fresh box / fork | **Part D** (known findings) |
| Rotating tokens / keys | **Part E** (maintenance) |

---

## Part A — First-time provisioning (one-time)

### A.1 Host prerequisites

A clean Linux VPS (the reference target is a Hetzner CX-class box, x86, ≥16 GB).

```bash
# Docker Engine + compose plugin, make, git, restic, curl
apt-get update && apt-get install -y docker.io docker-compose-plugin make git restic curl
systemctl enable --now docker
```

- **Firewall:** allow **22/80/443 only** (Hetzner Cloud Firewall or `ufw`). Backend
  ports (Postgres/ClickHouse/NATS/MinIO/Grafana/OTel) stay internal — reach them by
  SSH tunnel (see [monitoring.md](monitoring.md)).
- **DNS:** an **A** record for `DASHBOARD_HOST` → the box IPv4. If the box is
  dual-stack, reconcile the **AAAA** record too (point it at the box IPv6 or remove
  it — never leave it dangling; ROADMAP Phase 151).

### A.2 Repository + environment

```bash
git clone https://github.com/<owner>/aer.git /opt/aer
cd /opt/aer
cp .env.example .env
chmod 600 .env            # root-owned, 0600 — non-secret config (see below)
```

> **Phase 155 / ADR-046 — secrets are NOT in `.env`.** The box `.env` holds only
> **non-secret** config (hosts, `APP_ENV`, `COMPOSE_FILE`, `IMAGE_TAG`, ACME email,
> feature flags). Every credential is a **GitHub Actions Environment Secret**,
> injected to the box's tmpfs at deploy. Do **Part F** (one-time) before the first
> tag, and leave the `REPLACE-ME` secret lines out of the box `.env`.

Fill in **`.env`** with real **non-secret** values (every non-secret `REPLACE-ME`).
The load-bearing production keys (see `.env.example` for the full annotated list):

- `APP_ENV=production`
- `COMPOSE_FILE=compose.yaml:compose.prod.yaml` — activates the prod overlay
- `DASHBOARD_HOST=<your-apex>` · `BFF_PUBLIC_BASE_URL=https://<your-apex>`
- `BFF_WEBAUTHN_RP_ID=<your-apex>` · `BFF_WEBAUTHN_RP_ORIGINS=https://<your-apex>`
- `ACME_EMAIL=<you>` · `ADMIN_BOOTSTRAP_EMAIL=<your-admin-inbox>`
- `SMTP_HOST/PORT/USERNAME/PASSWORD` + `SMTP_FROM_ADDRESS` (transactional email)
- `GHCR_OWNER=<owner>` · `IMAGE_TAG=latest` (the CD deploy pins the released tag)
- `RESTIC_REPOSITORY` (path **under `/home`**) · `RESTIC_PASSWORD` (escrow off-box!)
- all datastore + MinIO + session secrets

### A.3 GHCR access (private images)

The four service images are private, so the box authenticates pulls with a
**read-only** token (least privilege — never the write token):

1. Create a classic PAT with **only `read:packages`** at
   `https://github.com/settings/tokens` (name e.g. `aer-box-ghcr-pull`).
2. On the box:
   ```bash
   docker login ghcr.io -u <owner>
   # paste the token at the Password: prompt → "Login Succeeded"
   ```
   The credential persists in `/root/.docker/config.json` (root-only). Rotate per Part E.

### A.4 Preflight gate

```bash
make preflight
```

`make preflight` (SEC-045) refuses to proceed on empty/placeholder secrets or
incoherent host vars. It is the authoritative pre-deploy check — do not skip it.

### A.5 First bring-up

There are two paths. Once the CD pipeline exists, **path (a)** is preferred.

**(a) Via the CD pipeline (recommended).** Finish the CD prerequisites in
**Part B → "One-time CD setup"** (deploy key + three GitHub secrets), then cut the
first tag (Part B). CI builds the images and the deploy job provisions the running
stack on the box. Skip to A.6 once the deploy job is green.

**(b) Bootstrap by building locally (the Phase-149 path, no images yet).** Useful
if you need the stack up before the CD pipeline is wired:

```bash
make openapi-bundle      # generates the gitignored OpenAPI bundle the images COPY
# First run: keep ACME on the Let's Encrypt STAGING CA to avoid burning the prod
# rate limit while DNS/first-boot settle (set ACME_CA_SERVER in .env), then flip:
docker compose --profile dashboard up -d --build
```

**ACME staging → production flip** (after a clean staging issue):

```bash
# .env: clear ACME_CA_SERVER= (empty → Let's Encrypt PRODUCTION)
docker compose run --rm traefik rm -f /letsencrypt/acme.json   # clear staging cert
docker compose --profile dashboard up -d                       # re-issue a trusted cert
```

### A.6 Verify

```bash
docker compose --profile dashboard ps                  # all Up / healthy
curl -sf https://<your-apex>/api/v1/probes >/dev/null && echo "BFF reachable over TLS"
```

Open `https://<your-apex>` in a browser — the globe loads, TLS is browser-trusted.

### A.7 First admin

```bash
docker compose exec bff-api /app/bootstrap-admin
# prints an accept-invite link for ADMIN_BOOTSTRAP_EMAIL — open it, set a password
```

### A.8 Backups + a tested restore (mandatory before go-live)

Follow [backup_restore.md](backup_restore.md) §3 for the Storage-Box prerequisites
(SSH key on **port 23**, repo path **under `/home`**), then:

```bash
make backup                                   # first run inits the restic repo
restic snapshots --tag aer                    # confirm a snapshot landed off-box
RESTORE_CONFIRM=yes ./scripts/operations/restore.sh   # tested restore (drill)
```

Do **not** declare go-live without a tested restore that round-trips the data.

### A.9 Scheduled work (systemd timers)

```bash
cp infra/systemd/aer-crawl.{service,timer} infra/systemd/aer-backup.{service,timer} /etc/systemd/system/
# edit WorkingDirectory=/opt/aer if your path differs
systemctl daemon-reload
systemctl enable --now aer-crawl.timer aer-backup.timer
systemctl list-timers 'aer-*'
```

See [scheduled_work.md](scheduled_work.md#production-runtime-scheduling-host-systemd-timers-phase-149-sec-041).

### A.10 Alerting

Confirm Grafana provisioned the contact point + rules, then send a test mail
(SSH-tunnel to the Grafana container — it has no public router):

```bash
docker compose logs grafana | grep -i provision
ssh -L 3000:<grafana-container-ip>:3000 root@<box>      # then Alerting → Contact points → Test
```

See [monitoring.md](monitoring.md).

### A.11 Go / No-Go

- [ ] App reachable over **browser-trusted TLS**, auth-gated, admin can log in
- [ ] First crawl drains clean (`make crawl`) — Gold rows present, 0 quarantine
- [ ] **Tested restore** round-trips the data
- [ ] systemd timers installed + a backup has run under systemd
- [ ] An alert reaches the operator inbox
- [ ] Transactional email works end-to-end (invite → accept → activate; reset)

---

## Part B — Routine release (tag-driven CD) — the everyday path

Once set up, a release is **one command**.

```bash
# on main, with CI green:
git tag v1.2.3
git push origin v1.2.3
```

Then watch the **Actions** tab → workflow *"Release — Build & Push Service Images
to GHCR"*:

1. **build-and-push** (matrix) — builds + pushes the 4 images to GHCR, tagged
   `v1.2.3`, `1.2`, `sha-…`, `latest`.
2. **deploy** — SSHes to the box, `git checkout tags/v1.2.3`, pins `IMAGE_TAG`,
   `docker compose pull`, `up -d --wait`. The `--wait` **fails the job** if any
   service does not become healthy, so a broken deploy surfaces immediately.

The box ends on a **detached HEAD at the tag** — normal for a deploy target (it
tracks releases, not a branch).

> **First-build note:** the `analysis-worker` image bakes ~3–5 GB of models. The
> first build is slow (~50 min, on GitHub's runners — not your box); the registry
> cache makes later builds that don't touch the worker a fast cache hit. The
> box-side pull + recreate is always **minutes** (it builds nothing).

> **Observability config note:** Prometheus alert rules and Grafana provisioning
> (`infra/observability/`) are **bind-mounted, not baked into images**, so a plain
> `up --wait` does *not* recreate prometheus/grafana when only those files change —
> a tweaked alert rule would silently never load. `deploy_pull.sh` handles this:
> after the stack is healthy it diffs `infra/observability/` against the
> last-deployed commit (recorded in the untracked `.aer-deployed-sha`) and, if it
> changed, runs `up -d --no-deps --force-recreate prometheus grafana` to load the
> new config. So an alerting/dashboard change ships with a normal release tag — no
> manual restart. (For a *config-only* change you don't want to cut a release for,
> the lightweight path is on the box: `git checkout origin/main -- infra/observability/`
> then `docker compose up -d --force-recreate prometheus grafana`.)

### Manual fallback (CD unavailable / out-of-band deploy)

```bash
ssh root@<box>
cd /opt/aer
git fetch --tags --prune origin
git checkout tags/v1.2.3
bash scripts/operations/deploy_pull.sh v1.2.3      # pin + pull + up --wait + health gate
```

### One-time CD setup (deploy key + three secrets)

GitHub Actions deploys over SSH with a **dedicated** key (never your personal key):

```bash
# on your local machine:
ssh-keygen -t ed25519 -f ~/.ssh/aer_deploy -N "" -C "aer-github-deploy"
ssh-copy-id -i ~/.ssh/aer_deploy.pub root@<box>        # authorize it on the box
ssh-keyscan <box-host>                                  # capture the host key
```

Then create three **repository secrets** (Settings → Secrets and variables →
Actions → *Secrets*):

| Secret | Value |
|---|---|
| `DEPLOY_SSH_KEY` | the **private** key (`cat ~/.ssh/aer_deploy`, incl. BEGIN/END) |
| `DEPLOY_HOST` | the box host (e.g. your apex domain) — must match the keyscan target |
| `DEPLOY_KNOWN_HOSTS` | the `ssh-keyscan` output (pins the host key, no blind TOFU) |

The deploy job is gated `if: startsWith(github.ref, 'refs/tags/v')` — a manual
`workflow_dispatch` builds images but **never** auto-deploys prod.

---

## Part C — Rollback

Database migrations are **forward-only**; an image-tag downgrade against an
already-migrated volume is only safe when the rolled-back range changed **no
schema**.

- **No schema change in the bad release** → redeploy the previous good tag:
  ```bash
  ssh root@<box>; cd /opt/aer
  bash scripts/operations/deploy_pull.sh v1.2.2     # pull + recreate the older images
  ```
- **A schema migration was involved** → rollback = **restore from the
  pre-upgrade backup** (not a tag downgrade). This is why a backup is a mandatory
  pre-upgrade gate. See [backup_restore.md](backup_restore.md) §7.

> **Always `make backup` + confirm `restic snapshots` before a schema-changing
> upgrade.**

---

## Part D — Fresh-machine build findings (the nine, fixed in Phase 149)

These were found + fixed during the first deploy; they are baked into the repo now,
but knowing them helps when a fresh box or fork misbehaves.

1. **OpenAPI bundle** — `services/{bff-api,ingestion-api}/api/openapi.bundle.yaml`
   is gitignored + COPYed by the images. `make up` and the CI build run
   `make openapi-bundle` first (needs Python 3 + PyYAML).
2. **pnpm workspace** — the dashboard image must COPY `pnpm-workspace.yaml` +
   `packages/engine-3d/package.json` before `pnpm install --frozen-lockfile`
   (else `ERR_PNPM_LOCKFILE_CONFIG_MISMATCH`).
3. **GHCR access** — `aer-wikidata-index` and the four service images are private;
   the box needs `docker login ghcr.io` with a `read:packages` token (Part A.3).
4. **SentiWS lexicon** — no longer fetched at build. The SentiWS zip is vendored
   into the repo (`services/analysis-worker/vendor/`) and COPYed by the worker
   Dockerfile, because both upstream Leipzig hosts (old mirror + new download page)
   went offline. No `SENTIWS_URL` override is needed anymore; the SHA256 is still
   pinned and verified in the Dockerfile.
5. **Postgres 18 `PGDATA`** — `postgres:18` hard-refuses the legacy
   `/var/lib/postgresql/data` mount on a fresh volume unless `PGDATA` is set back to
   it (pinned in `compose.yaml`).
6. **minio-init** — the `minio/mc` image has no `grep`/`tr`/`awk`, so the ILM
   read-back assertion uses the POSIX `case` builtin; and `mc event add` is not
   idempotent on re-run ("overlapping") — tolerated in `mc_idempotent`.
7. **restic / Storage Box** — SSH/SFTP on **port 23**, the session lands in
   **`/home`** with no shell, so `RESTIC_REPOSITORY` must point **under `/home`**
   (an absolute `:/aer-backups` fails `restic init` with `SSH_FX_FAILURE`).
8. **restore drops first** — `restore.sh` runs `DROP DATABASE IF EXISTS` before the
   ClickHouse `RESTORE` (else "table already exists" against a populated target).
9. **backup retention grouping** — `restic forget --group-by host` (each run stages
   into a fresh `mktemp` path, so the default host,paths grouping never prunes →
   unbounded growth + a broken DSGVO-erasure bound).

---

## Part E — Maintenance

- **GHCR pull token** — rotate the box's `read:packages` token before expiry
  (re-run `docker login ghcr.io`); revoke the old one.
- **Deploy SSH key** — rotate `aer_deploy` periodically; update `DEPLOY_SSH_KEY` +
  the box `authorized_keys`.
- **Secrets escrow** — the `.env` secrets (esp. `RESTIC_PASSWORD`) live in a
  password manager off-box; losing `RESTIC_PASSWORD` makes every backup
  unrecoverable, by design.
- **Secrets off-box (Phase 155)** — the standing `.env` is the interim posture; the
  deploy-time-injection + Docker-secrets rearchitecture is tracked in ROADMAP
  Phase 155.

---

## Part F — Secrets off-box (Phase 155 / ADR-046)

Secrets are **not** stored on the box. They live in **GitHub Actions Environment
Secrets**; the deploy job streams them over SSH into the box's tmpfs
(`/run/aer-secrets`, RAM-only) as Docker secrets, mounted at `/run/secrets/*` and
read via the `<VAR>_FILE` convention. Nothing plaintext touches the persistent
disk (`docker inspect` shows only `*_FILE` paths). `infra/secrets/secrets.manifest`
is the canonical secret list.

### F.1 One-time setup (before the first Phase-155 deploy)

1. **Create a GitHub `production` Environment** (repo → Settings → Environments)
   and add **one Environment Secret per name** in `infra/secrets/secrets.manifest`
   (all except the `(derived)` `DB_URL`). The required set:
   `MINIO_ROOT_USER`, `MINIO_ROOT_PASSWORD`,
   `INGESTION_MINIO_ACCESS_KEY`/`INGESTION_MINIO_SECRET_KEY`,
   `WORKER_MINIO_ACCESS_KEY`/`WORKER_MINIO_SECRET_KEY`,
   `BFF_MINIO_ACCESS_KEY`/`BFF_MINIO_SECRET_KEY`,
   `POSTGRES_PASSWORD`, `BFF_DB_PASSWORD`, `BFF_AUTH_DB_PASSWORD`,
   `CLICKHOUSE_PASSWORD`, `INGESTION_API_KEY`, `BFF_API_KEY`,
   `GF_SECURITY_ADMIN_PASSWORD`, `RESTIC_PASSWORD`. Optional (leave unset if
   unused): `SMTP_PASSWORD`, `GF_SMTP_PASSWORD`. Use the **same values** currently
   in the box `.env`.
2. **Set the repo variable `STAGE_SECRETS=true`** (repo → Settings → Variables).
   The deploy job's staging step is a no-op until this is set, so the change is
   safe to merge ahead of time. (Optionally set repo *variables* `POSTGRES_USER`
   / `POSTGRES_DB` if they differ from `aer_admin` / `aer_metadata`.)
3. **Remove the secret lines from the box `/opt/aer/.env`.** Keep the non-secret
   config (`APP_ENV`, `COMPOSE_FILE`, `IMAGE_TAG`, `GHCR_OWNER`, the `*_HOST`s,
   ACME email, feature flags). Keep the file root-owned `chmod 600`.
4. Keep **`RESTIC_PASSWORD`** escrowed off-box in a password manager — losing it
   makes the encrypted backups unrecoverable, by design.
5. **Disable unattended reboots** on the box (e.g. `sudo systemctl disable --now
   unattended-upgrades` or set `Unattended-Upgrade::Automatic-Reboot "false";`) so
   a reboot only happens when you trigger it — see F.4.

### F.2 Deploying (unchanged for you)

Cut a tag exactly as before — `git tag vX.Y.Z && git push origin vX.Y.Z`. The CD
job now does **checkout → stage secrets to tmpfs → `deploy_pull.sh`** in one run:
it stages the GitHub secrets onto the box *before* `compose up`, regenerates the
non-secret `.env.runtime` (D9), brings the stack up health-gated, then runs the
**config-audit drift gate** (a drift fails the deploy). No secret ever lands on
the disk or in a log.

### F.3 Verifying

```bash
ssh root@<box> 'ls -l /run/aer-secrets | head'          # 0444 files, RAM-backed
ssh root@<box> 'cd /opt/aer && make config-audit'        # no drift on the live box
# Fortress check — only *_FILE paths, never a value:
ssh root@<box> 'docker inspect aer_postgres --format "{{range .Config.Env}}{{println .}}{{end}}" | grep -i password'
```

After the first Phase-155 deploy, **re-run the tested-restore drill** (Part A.8 /
backup_restore.md) once — backup/restore now read their credentials from tmpfs.

### F.4 Reboot recovery

A reboot clears the tmpfs, so the stack stays down until you **re-stage** — this
is the deliberate "no standing secrets" fortress trade-off. To recover, **re-run
the deploy** (re-run the last `release-images.yml` run, or push the current tag),
which re-stages the secrets and brings the stack back up. Because unattended
reboots are disabled (F.1.5), a reboot is operator-initiated, so re-staging is
expected. For the blind spot where the box reboots unattended *and* Grafana (which
itself needs a secret) cannot deliver the in-stack `TargetDown` alert, add an
**external** uptime monitor against `https://<your-apex>/api/v1/healthz`.

### F.5 Local development

`make up` auto-stages: `stage_secrets_local.sh` writes your local `.env` secrets to
`./.aer-secrets` (gitignored) and `gen_runtime_env.sh` writes `.env.runtime`, so
the local compose uses the identical Docker-secrets + env_file wiring as prod. No
manual step.

---

## References

- [Backup & Restore Runbook](backup_restore.md) — the four stores, the corruption-safe restore order, DSGVO erasure
- [Monitoring & Alerting](monitoring.md) — surfaces, SSH tunnels, alert → action
- [Scheduled Work](scheduled_work.md) — the systemd timers
- [Operations Playbook](operations_playbook.md) — per-component debugging + recovery
- Arc42 §7 Deployment View — the topology + network posture
