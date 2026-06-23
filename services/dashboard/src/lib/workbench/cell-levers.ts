// Phase 126 — shared cell-shape lever constants.
//
// The panel-level controls (`PanelControls.svelte`) and the per-cell override
// popover (`CellConfigPopover.svelte`) render the SAME levers — one bound to the
// panel default, the other to a single cell's override. Keeping the defaults and
// the channel option tables here (one source of truth) stops the two surfaces
// from drifting (e.g. a new network colour channel added to one but not the
// other, or a slider that opens at a different default than the cell renders).
import type { NetworkColorChannel, NetworkSizeChannel } from '$lib/state/url-internals';
// Relative import (not `$lib/...`) so the module resolves in the node/vitest
// environment as well as the bundler — mirrors the other localized `.ts`
// helpers (e.g. `presentations/how-to-read.ts`).
import { m } from '../paraglide/messages.js';

// Cell-render defaults — must match what the cells actually render when the
// lever is unset (DistributionCell bins, CoOccurrenceNetworkCell topN/spread).
export const DEFAULT_BINS = 30;
export const DEFAULT_TOPN = 60;
export const DEFAULT_FORCE_STRENGTH = 50;

// ── Slider input bounds — ONE source of truth for both lever surfaces (the
// panel-level `levers/ConfigValueLevers` and the per-cell `CellConfigValueLevers`
// popover), so a `min`/`max`/`step` never drifts between the two. Sliders open
// narrower than the clamp ceiling on purpose (the clamp in
// `cell-config-popover-internals` still accepts a wider programmatic/inherited
// value). topN has no constant MAX here: its ceiling is view-dependent —
// `computeTopNMax` gives co-occurrence 6000 edges, metadata-field views 200
// (server clamp), all other views 500 — so both surfaces derive it per panel.
export const BINS_MIN = 5;
export const BINS_MAX = 120;
export const BINS_STEP = 1;
export const TOPN_MIN = 5;
export const TOPN_STEP = 5;
export const FORCE_MIN = 0;
export const FORCE_MAX = 100;
export const FORCE_STEP = 1;

// `label` is a getter (not a plain string) so the rendered option text stays
// locale-reactive — resolved against the active locale at each render, never
// frozen at module load.
export const NET_SIZE_CHANNELS: ReadonlyArray<{ id: NetworkSizeChannel; label: () => string }> = [
  { id: 'total_count', label: () => m.levers_netsize_weight() },
  { id: 'degree', label: () => m.levers_netsize_degree() },
  // Phase 125 — size by a per-article metric (mean over the entity's articles).
  { id: 'metric', label: () => m.levers_netsize_metric() }
];
export const NET_COLOR_CHANNELS: ReadonlyArray<{ id: NetworkColorChannel; label: () => string }> = [
  // Co-occurrence redesign — colour by detected theme-cluster (Louvain). The
  // default: each topic-region gets its own colour (the "Kriesel" map effect).
  { id: 'community', label: () => m.levers_netcolor_community() },
  { id: 'label', label: () => m.levers_netcolor_label() },
  { id: 'presence', label: () => m.levers_netcolor_presence() },
  { id: 'source_overlay', label: () => m.levers_netcolor_source_overlay() },
  { id: 'uniform', label: () => m.levers_netcolor_uniform() },
  // Phase 125 — colour by a per-article metric.
  { id: 'metric', label: () => m.levers_netcolor_metric() }
];
