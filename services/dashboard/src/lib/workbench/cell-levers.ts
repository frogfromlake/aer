// Phase 126 — shared cell-shape lever constants.
//
// The panel-level controls (`PanelControls.svelte`) and the per-cell override
// popover (`CellConfigPopover.svelte`) render the SAME levers — one bound to the
// panel default, the other to a single cell's override. Keeping the defaults and
// the channel option tables here (one source of truth) stops the two surfaces
// from drifting (e.g. a new network colour channel added to one but not the
// other, or a slider that opens at a different default than the cell renders).
import type { NetworkColorChannel, NetworkSizeChannel } from '$lib/state/url-internals';

// Cell-render defaults — must match what the cells actually render when the
// lever is unset (DistributionCell bins, CoOccurrenceNetworkCell topN/spread).
export const DEFAULT_BINS = 30;
export const DEFAULT_TOPN = 60;
export const DEFAULT_FORCE_STRENGTH = 50;

export const NET_SIZE_CHANNELS: ReadonlyArray<{ id: NetworkSizeChannel; label: string }> = [
  { id: 'total_count', label: 'Weight' },
  { id: 'degree', label: 'Degree' },
  // Phase 125 — size by a per-article metric (mean over the entity's articles).
  { id: 'metric', label: 'Metric' }
];
export const NET_COLOR_CHANNELS: ReadonlyArray<{ id: NetworkColorChannel; label: string }> = [
  // Co-occurrence redesign — colour by detected theme-cluster (Louvain). The
  // default: each topic-region gets its own colour (the "Kriesel" map effect).
  { id: 'community', label: 'Theme cluster' },
  { id: 'label', label: 'Entity type' },
  { id: 'presence', label: 'Source presence' },
  { id: 'source_overlay', label: 'Source overlay' },
  { id: 'uniform', label: 'Uniform' },
  // Phase 125 — colour by a per-article metric.
  { id: 'metric', label: 'Metric' }
];
