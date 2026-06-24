// Barrel for $lib/presentations — the presentation registry (Pillar↔presentation
// placement, metric→presentation support) exposed as one import surface.
export {
  cellContentId,
  hasCellMethodologyContent,
  CROSS_PROBE_DEFAULT_METRIC,
  DEFAULT_METRIC_NAME,
  defaultPresentationForPillar,
  getPillar,
  getPresentation,
  listPillars,
  listPresentations,
  pillarForPresentation,
  PILLAR_DEFINITIONS,
  presentationsForPillar,
  resolvePresentation,
  type AnalyticalDiscipline,
  type CellParamKind,
  type PillarDefinition,
  type PresentationDefinition,
  type PresentationCellProps
} from './registry';

export {
  isIntegerMetric,
  isMetadataMetric,
  isPureCountMetric,
  metricSupportsPresentation,
  panelSubjectKind,
  presentationsForMetric,
  type PanelSubjectKind
} from './metric-presentation';

export { cellSubjects, type CellSubject, type SubjectRole } from './cell-subjects';
