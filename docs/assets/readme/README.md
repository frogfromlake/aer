# README screenshots

These four images are referenced by the top-level `README.md`
"Screenshots" section. Keep the **exact** filenames — the README embeds them directly.

| Filename | Surface | What it shows |
| :--- | :--- | :--- |
| `atmosphere.png` | Atmosphère | The 3D globe with the SideRail chrome (`/`). |
| `workbench-aleph.png` | Workbench · Aleph | A synchronic distribution histogram over Probe 0 (the "weather now"). |
| `workbench-episteme.png` | Workbench · Episteme | A diachronic time-series cell (metric over time). |
| `workbench-rhizome.png` | Workbench · Rhizome | A relational entity co-occurrence network. |
| `reflection.png` | Reflexion | A rendered Working Paper (WP-001). |

Captured at a 1600×900 viewport, `deviceScaleFactor: 1.5`, dark theme. Re-capture
when the UI changes; keep each PNG well under ~500 KB.

## Reproducible capture recipe

The dashboard is auth-gated ([ADR-040](../../arc42/09_architecture_decisions.md)) and
needs real data. Against a running stack (`make up` + some crawled data):

**1. Mint a throwaway login (non-destructive — does not touch your real admin):**

```bash
# create an invited throwaway admin, capture its invite token
ADMIN_BOOTSTRAP_EMAIL=shotbot@aer.local go run ./services/bff-api/cmd/bootstrap-admin
TOKEN=...                                  # the token= value from the printed link
# activate it (sets password + consent, auto-login)
curl -sk -X POST https://localhost/api/v1/auth/accept-invite -H 'Content-Type: application/json' \
  -d "{\"token\":\"$TOKEN\",\"password\":\"ScreenshotPass123!\",\"acceptResponsibleUse\":true}"
```

**2. Drive the four surfaces with Playwright** (run from `services/dashboard/`, which
has Playwright + Chromium installed). The Workbench URLs carry a base64url *pillar
state* seeding one Probe-0 panel with the cell controls collapsed (`cc:1`) so the
chart shows instead of the config strip:

- Atmosphère — `https://localhost/`
- Workbench · Aleph (distribution) — `https://localhost/workbench?activePillar=aleph&aleph=eyJ3IjpbeyJwIjpbeyJzIjpbeyJwaSI6WyJwcm9iZS0wLWRlLWluc3RpdHV0aW9uYWwtd2ViIl0sInNpIjpbXX1dLCJjIjoibSIsInYiOiJkaXN0cmlidXRpb24iLCJtIjoic2VudGltZW50X3Njb3JlX3NlbnRpd3MiLCJsIjoiZyIsImNjIjoxfV0sImZpIjowfV0sImF3IjowfQ`
- Workbench · Episteme (time_series) — `https://localhost/workbench?activePillar=episteme&episteme=eyJ3IjpbeyJwIjpbeyJzIjpbeyJwaSI6WyJwcm9iZS0wLWRlLWluc3RpdHV0aW9uYWwtd2ViIl0sInNpIjpbXX1dLCJjIjoibSIsInYiOiJ0aW1lX3NlcmllcyIsIm0iOiJzZW50aW1lbnRfc2NvcmVfc2VudGl3cyIsImwiOiJnIiwiY2MiOjF9XSwiZmkiOjB9XSwiYXciOjB9`
- Reflexion — `https://localhost/reflection/wp/wp-001`

Log in at `/login` (`#email` / `#password`), `goto` each URL, let it settle a few
seconds (the globe needs ~6 s), then `page.screenshot(...)`. Launch Chromium with
`--use-gl=angle --use-angle=swiftshader --ignore-certificate-errors` and
`ignoreHTTPSErrors: true` for the WebGL globe + self-signed TLS.

The seeds were generated with `encodePillarState`; to change probe/metric/view,
re-encode `{w:[{p:[{s:[{pi:[<probeId>],si:[]}],c:'m',v:<view>,m:<metric>,l:'g',cc:1}],fi:0}],aw:0}`
as base64url (no padding).
