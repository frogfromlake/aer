"""Pre-download Tier-2 BERT sentiment models into the build-time HF cache.

Invoked from the analysis-worker Dockerfile builder stage so the resulting
image carries the model weights on disk and the deployed worker can run
with `TRANSFORMERS_OFFLINE=1`. Revisions are read from the Language
Capability Manifest (`configs/language_capabilities.yaml`) — the same
single source of truth the extractors consume at construction time, per
ADR-024.

Usage::

    python prefetch_bert_models.py <manifest_path> <hf_home_dir>

Exit codes: 0 on success, non-zero on unrecoverable failure (the build
must fail loudly rather than ship an image with missing weights).
"""

from __future__ import annotations

import sys
import time
from pathlib import Path

import yaml
from huggingface_hub import snapshot_download
from huggingface_hub.errors import HfHubHTTPError

_MAX_ATTEMPTS = 5
_BACKOFF_SECONDS = (5, 15, 30, 60, 120)


def _collect_targets(manifest: dict) -> list[tuple[str, str]]:
    targets: list[tuple[str, str]] = []

    shared = manifest.get("shared", {}) or {}
    multilingual = shared.get("multilingual_bert")
    if multilingual:
        targets.append((multilingual["model"], multilingual["model_revision"]))

    for lang_block in (manifest.get("languages") or {}).values():
        refinement = lang_block.get("sentiment_tier2_refinement")
        if refinement and refinement.get("method") == "news_domain_bert":
            targets.append((refinement["model"], refinement["model_revision"]))

    return targets


def _download_with_retry(repo_id: str, revision: str, cache_dir: Path) -> None:
    last_exc: Exception | None = None
    for attempt in range(_MAX_ATTEMPTS):
        try:
            print(
                f"[prefetch] {repo_id}@{revision[:12]} "
                f"(attempt {attempt + 1}/{_MAX_ATTEMPTS})",
                flush=True,
            )
            snapshot_download(
                repo_id=repo_id,
                revision=revision,
                cache_dir=str(cache_dir / "hub"),
                local_files_only=False,
            )
            print(f"[prefetch] OK {repo_id}@{revision[:12]}", flush=True)
            return
        except (HfHubHTTPError, OSError, TimeoutError) as exc:
            last_exc = exc
            backoff = _BACKOFF_SECONDS[min(attempt, len(_BACKOFF_SECONDS) - 1)]
            print(
                f"[prefetch] FAIL {repo_id}@{revision[:12]} "
                f"({type(exc).__name__}: {exc}); retrying in {backoff}s",
                flush=True,
            )
            time.sleep(backoff)
    raise RuntimeError(
        f"prefetch failed after {_MAX_ATTEMPTS} attempts: {repo_id}@{revision}"
    ) from last_exc


def main(manifest_path: str, hf_home: str) -> int:
    manifest = yaml.safe_load(Path(manifest_path).read_text(encoding="utf-8"))
    targets = _collect_targets(manifest)
    if not targets:
        print("[prefetch] no Tier-2 models declared in manifest", flush=True)
        return 0

    cache_dir = Path(hf_home)
    cache_dir.mkdir(parents=True, exist_ok=True)

    for repo_id, revision in targets:
        _download_with_retry(repo_id, revision, cache_dir)

    return 0


if __name__ == "__main__":
    if len(sys.argv) != 3:
        print(__doc__, file=sys.stderr)
        sys.exit(2)
    sys.exit(main(sys.argv[1], sys.argv[2]))
