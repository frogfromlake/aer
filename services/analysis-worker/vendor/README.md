# Vendored third-party data

## SentiWS_v2.0.zip — SentiWS German sentiment lexicon

A verbatim, unmodified copy of the SentiWS v2.0 German sentiment lexicon, committed
to the repository and consumed at build time by `services/analysis-worker/Dockerfile`
(`COPY` → `unzip` → `/app/data/sentiws`).

| | |
|---|---|
| **File** | `SentiWS_v2.0.zip` (97 748 bytes) |
| **SHA-256** | `4dd6ce99a44b5122c04fe0e7ca36db1f94d738201095edabb0a16cc98f160b91` |
| **Version** | SentiWS v2.0 |
| **License** | CC-BY-SA (Creative Commons Attribution-ShareAlike) — the authoritative license and version are the files bundled **inside** the zip |
| **Authors / attribution** | R. Remus, U. Quasthoff & G. Heyer (2010): *SentiWS — a Publicly Available German-language Resource for Sentiment Analysis.* Proceedings of LREC'10, pp. 1168–1171. Natural Language Processing Group, Leipzig University. |

### Why this is vendored (not downloaded at build time)

Both upstream sources are **offline** (observed 2026-06-28, down for several days):

- old mirror: `https://downloads.wortschatz-leipzig.de/etc/SentiWS/SentiWS_v2.0.zip`
- new download page: `https://wortschatz.uni-leipzig.de/de/download/#sentiws`

A build that fetches over the network is therefore unreliable everywhere (local
dev, CI, and the production box). Vendoring the file:

- removes the build-time network dependency entirely → the worker image builds
  fully reproducibly and offline-capably;
- is cheap — the lexicon is ~95 KB;
- keeps the supply-chain guarantee: the Dockerfile still runs `sha256sum -c`
  against the pinned `SENTIWS_SHA256`, so a corrupted or tampered vendored file
  fails the build.

The bytes here are byte-identical to the last good upstream release (the SHA-256
above matches an Internet Archive snapshot of the original Leipzig download).

### Updating to a newer SentiWS release

1. Replace `SentiWS_v2.0.zip` with the new file (rename the variable in the
   Dockerfile/this file if the version string changes).
2. Run `make deps-refresh` — Step 3 recomputes `SENTIWS_SHA256` from the vendored
   file and rewrites the Dockerfile `ARG`.
3. Update the attribution above if the license/version changed.
