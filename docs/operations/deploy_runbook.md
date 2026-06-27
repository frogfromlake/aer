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
- **Secrets** live in a root-owned `chmod 600` `/opt/aer/.env`, escrowed off-box in
  a password manager. Removing the standing `.env` (deploy-time injection + Docker
  secrets) is the deliberate end-state tracked in **ROADMAP Phase 155**.
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
chmod 600 .env            # root-owned, 0600 — the only standing secret file
```

Fill in **`.env`** with real values (every `REPLACE-ME` placeholder). The
load-bearing production keys (see `.env.example` for the full annotated list):

- `APP_ENV=production`
- `COMPOSE_FILE=compose.yaml:compose.prod.yaml` — activates the prod overlay
- `DASHBOARD_HOST=<your-apex>` · `BFF_PUBLIC_BASE_URL=https://<your-apex>`
- `BFF_WEBAUTHN_RP_ID=<your-apex>` · `BFF_WEBAUTHN_RP_ORIGINS=https://<your-apex>`
- `ACME_EMAIL=<you>` · `ADMIN_BOOTSTRAP_EMAIL=<your-admin-inbox>`
- `SMTP_HOST/PORT/USERNAME/PASSWORD` + `SMTP_FROM_ADDRESS` (transactional email)
- `GHCR_OWNER=<owner>` · `IMAGE_TAG=latest` (the CD deploy pins the released tag)
- `RESTIC_REPOSITORY` (path **under `/home`**) · `RESTIC_PASSWORD` (escrow off-box!)
- `SENTIWS_URL=` — only if the Leipzig mirror is down at build time (see Part D)
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
4. **SentiWS mirror** — the worker fetches SentiWS at build; the Leipzig mirror was
   down. Override `SENTIWS_URL` (build arg / repo variable) with a SHA-identical
   fallback (e.g. an Internet-Archive snapshot). The hash is pinned in the Dockerfile.
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

## References

- [Backup & Restore Runbook](backup_restore.md) — the four stores, the corruption-safe restore order, DSGVO erasure
- [Monitoring & Alerting](monitoring.md) — surfaces, SSH tunnels, alert → action
- [Scheduled Work](scheduled_work.md) — the systemd timers
- [Operations Playbook](operations_playbook.md) — per-component debugging + recovery
- Arc42 §7 Deployment View — the topology + network posture
