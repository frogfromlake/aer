# SentiWS — SentimentWortschatz

**Source:** Leipzig University, Department of NLP
**License:** CC-BY-SA 4.0
**URL:** https://wortschatz.uni-leipzig.de/en/download
**Version:** v2.0 (pinned)

## Files

- `SentiWS_v2.0_Positive.txt` — Positive polarity words
- `SentiWS_v2.0_Negative.txt` — Negative polarity words

## Format

Each line: `word|POS\tweight\tinflection1,inflection2,...`

Example:
```
Glück|NN	0.5765	Glücks,Glückes,Glücke,Glücken
```

## Download

Download from https://wortschatz.uni-leipzig.de/en/download and place the two
`.txt` files in this directory. The files are not committed to the repository
due to licensing clarity — they must be downloaded separately.

The Dockerfile copies this directory into the container image. If the files
are missing, the SentimentExtractor will log a warning and produce no metrics.

## Provisional Status (Phase 42)

SentiWS is chosen because it is deterministic, auditable, and German-language —
not because it is the best sentiment method. It does not handle negation, irony,
domain-specific language, or compositionality. The lexicon, scoring algorithm,
and normalization will change when CSS researchers (§13.5) provide validated
alternatives.
