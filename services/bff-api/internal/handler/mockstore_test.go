package handler

import (
	"context"
	"errors"
	"time"

	"github.com/frogfromlake/aer/services/bff-api/internal/storage"
)

// errTest is the shared sentinel for storage-error (→ 500) handler tests.
var errTest = errors.New("storage failure")

// timeAt parses an RFC-3339 timestamp for fixture rows, panicking on a bad
// literal (a test-author error, not a runtime condition).
func timeAt(s string) time.Time {
	ts, err := time.Parse(time.RFC3339, s)
	if err != nil {
		panic(err)
	}
	return ts
}

// mockStore is a test double for Store.
type mockStore struct {
	pingErr                   error
	metrics                   []storage.MetricRow
	metricsTruncated          bool
	metricsErr                error
	normalizedMetrics         []storage.MetricRow
	normalizedMetricsExcluded int64
	normalizedMetricsErr      error
	baselineExists            bool
	baselineExistsErr         error
	equivalenceExists         bool
	equivalenceExistsErr      error
	// Phase 115: cross-frame equivalence + percentile + probe summary mocks.
	percentileMetrics                 []storage.MetricRow
	percentileMetricsExcluded         int64
	percentileMetricsErr              error
	languagesForScopeRows             []string
	languagesForScopeErr              error
	countLanguagesForSourcesValue     int
	countLanguagesForSourcesErr       error
	documentTotalsBySource            map[string]int64
	documentTotalsBySourceErr         error
	checkEquivalenceForLanguagesValue bool
	checkEquivalenceForLanguagesErr   error
	// Phase 124: metric-class-aware normalization gate (separate from the
	// strict deviation/absolute reporting check above).
	checkNormalizationEquivForLanguagesValue bool
	checkNormalizationEquivForLanguagesErr   error
	probeEquivalenceRows                     []storage.ProbeEquivalenceMetric
	probeEquivalenceErr                      error
	// Phase 124: lead-lag + grant status.
	leadLag                 storage.LeadLagResult
	leadLagErr              error
	equivalenceStatus       *storage.EquivalenceStatusRow
	equivalenceStatusErr    error
	entities                []storage.EntityRow
	entitiesErr             error
	languageDetections      []storage.LanguageDetectionRow
	languageDetectionsErr   error
	availableMetrics        []storage.AvailableMetricRow
	availableMetricsErr     error
	validationStatus        string
	validationStatusErr     error
	culturalContextNotes    string
	culturalContextNotesErr error
	// Phase 102 view-mode mocks.
	distribution    storage.DistributionResult
	distributionErr error
	heatmap         []storage.HeatmapCell
	heatmapErr      error
	correlation     storage.CorrelationResult
	correlationErr  error
	cooccurrence    storage.CoOccurrenceResult
	cooccurrenceErr error
	// Phase 120 topic-distribution mocks.
	topicDistribution    []storage.TopicDistributionRow
	topicDistributionErr error
	capturedTopicParams  storage.TopicDistributionParams
	// Phase 103b silver-aggregation mocks.
	silverDistribution    storage.DistributionResult
	silverDistributionErr error
	silverHeatmap         []storage.HeatmapCell
	silverHeatmapXDim     string
	silverHeatmapYDim     string
	silverHeatmapErr      error
	silverCorrelation     storage.SilverCorrelationResult
	silverCorrelationErr  error
	capturedSilverField   string
	capturedSilverKind    storage.SilverAggregationKind
	// Phase 122f metadata-coverage mocks.
	metadataCoverage    []storage.MetadataCoverageCell
	metadataCoverageErr error
	fieldCardinality    map[storage.FieldKey]storage.FieldCardinality
	fieldCardinalityErr error
	globalFieldStats    []storage.GlobalFieldStat
	globalFieldStatsErr error
	// Phase 133: categorical metadata distribution + availability.
	categoricalDistribution    storage.CategoricalDistributionResult
	categoricalDistributionErr error
	scopeAvailableMetadata     storage.ScopeMetadataAvailability
	scopeAvailableMetadataErr  error
	crossTab                   storage.CrossTabResult
	crossTabErr                error
	parallelCoords             storage.ParallelCoordResult
	parallelCoordsErr          error
	sankey                     storage.SankeyResult
	sankeyErr                  error
	// Phase 131 mocks: time-series spread + paired-metric scatter.
	metricsSpread    []storage.MetricRow
	metricsSpreadErr error
	scatter          storage.ScatterResult
	scatterErr       error
	capturedScatterX string
	capturedScatterY string
	// captured args
	capturedStart      time.Time
	capturedEnd        time.Time
	capturedSource     *string
	capturedMetricName *string
	capturedLabel      *string
	capturedLanguage   *string
	capturedLimit      int
	capturedResolution storage.Resolution
	capturedSources    []string
	capturedBins       int
	capturedTopN       int
	capturedXDim       storage.HeatmapDimension
	capturedYDim       storage.HeatmapDimension
	capturedMetrics    []string
	// Phase 122d revision-surface mocks (silent-edit observability).
	revisionActivity          []storage.RevisionActivityCell
	revisionActivityErr       error
	revisionDiscourseShift    []storage.RevisionDiscourseShiftCell
	revisionDiscourseShiftErr error
	revisionEditClusters      []storage.RevisionEditClusterRow
	revisionEditClustersErr   error
	capturedMinSources        int
	articleRevisions          []storage.ArticleRevisionRow
	articleRevisionsErr       error
	articleRevisionDiff       *storage.ArticleRevisionDiffRow
	articleRevisionDiffErr    error
	revisionsArticles         []storage.RevisionArticleRow
	revisionsArticlesErr      error
	capturedRevisionsFilter   storage.RevisionsArticlesFilter
	// Phase 133 scope-availability mocks.
	scopeAvailableMetricsResult storage.ScopeMetricAvailability
	scopeAvailableMetricsErr    error
}

func (m *mockStore) Ping(_ context.Context) error {
	return m.pingErr
}

func (m *mockStore) GetMetrics(_ context.Context, start, end time.Time, sources []string, metricName *string, resolution storage.Resolution) (storage.MetricsResult, error) {
	m.capturedStart = start
	m.capturedEnd = end
	m.capturedSources = sources
	m.capturedMetricName = metricName
	m.capturedResolution = resolution
	return storage.MetricsResult{Rows: m.metrics, Truncated: m.metricsTruncated}, m.metricsErr
}

func (m *mockStore) GetMetricsWithSpread(_ context.Context, start, end time.Time, sources []string, metricName *string, resolution storage.Resolution) (storage.MetricsResult, error) {
	m.capturedStart = start
	m.capturedEnd = end
	m.capturedSources = sources
	m.capturedMetricName = metricName
	m.capturedResolution = resolution
	return storage.MetricsResult{Rows: m.metricsSpread, Truncated: m.metricsTruncated}, m.metricsSpreadErr
}

func (m *mockStore) GetMetricScatter(_ context.Context, xMetric, yMetric string, _, _ *string, sources []string, start, end time.Time, _ int, _ *storage.MetadataFilter) (storage.ScatterResult, error) {
	m.capturedStart = start
	m.capturedEnd = end
	m.capturedSources = sources
	m.capturedScatterX = xMetric
	m.capturedScatterY = yMetric
	return m.scatter, m.scatterErr
}

func (m *mockStore) GetNormalizedMetrics(_ context.Context, start, end time.Time, sources []string, metricName *string, resolution storage.Resolution) (storage.MetricsResult, int64, error) {
	m.capturedStart = start
	m.capturedEnd = end
	m.capturedSources = sources
	m.capturedMetricName = metricName
	m.capturedResolution = resolution
	return storage.MetricsResult{Rows: m.normalizedMetrics, Truncated: m.metricsTruncated}, m.normalizedMetricsExcluded, m.normalizedMetricsErr
}

func (m *mockStore) CheckBaselineExists(_ context.Context, _ string, _ *string) (bool, error) {
	return m.baselineExists, m.baselineExistsErr
}

func (m *mockStore) CheckEquivalenceExists(_ context.Context, _ string) (bool, error) {
	return m.equivalenceExists, m.equivalenceExistsErr
}

func (m *mockStore) GetPercentileNormalizedMetrics(_ context.Context, start, end time.Time, sources []string, metricName *string, resolution storage.Resolution) (storage.MetricsResult, int64, error) {
	m.capturedStart = start
	m.capturedEnd = end
	m.capturedSources = sources
	m.capturedMetricName = metricName
	m.capturedResolution = resolution
	return storage.MetricsResult{Rows: m.percentileMetrics, Truncated: m.metricsTruncated}, m.percentileMetricsExcluded, m.percentileMetricsErr
}

func (m *mockStore) CountLanguagesForSources(_ context.Context, _, _ time.Time, _ []string) (int, error) {
	return m.countLanguagesForSourcesValue, m.countLanguagesForSourcesErr
}

func (m *mockStore) GetDocumentTotalsBySource(_ context.Context, _ []string) (map[string]int64, error) {
	return m.documentTotalsBySource, m.documentTotalsBySourceErr
}

func (m *mockStore) LanguagesForScope(_ context.Context, _, _ time.Time, _ []string) ([]string, error) {
	return m.languagesForScopeRows, m.languagesForScopeErr
}

func (m *mockStore) CheckEquivalenceForLanguages(_ context.Context, _ string, _ []string) (bool, error) {
	return m.checkEquivalenceForLanguagesValue, m.checkEquivalenceForLanguagesErr
}

func (m *mockStore) CheckNormalizationEquivalenceForLanguages(_ context.Context, _ string, _ []string) (bool, error) {
	return m.checkNormalizationEquivForLanguagesValue, m.checkNormalizationEquivForLanguagesErr
}

func (m *mockStore) GetProbeEquivalence(_ context.Context, _, _ time.Time, _ []string) ([]storage.ProbeEquivalenceMetric, error) {
	return m.probeEquivalenceRows, m.probeEquivalenceErr
}

func (m *mockStore) GetTemporalLeadLag(_ context.Context, _, _ []string, _, _ time.Time, _ int) (storage.LeadLagResult, error) {
	return m.leadLag, m.leadLagErr
}

func (m *mockStore) GetMetricLeadLag(_ context.Context, sources []string, _, _ string, _, _ time.Time, _ int, _ *storage.MetadataFilter) (storage.LeadLagResult, error) {
	m.capturedSources = sources
	return m.leadLag, m.leadLagErr
}

func (m *mockStore) GetParallelCoords(_ context.Context, metrics, sources []string, _, _ time.Time, _ int, _ *storage.MetadataFilter) (storage.ParallelCoordResult, error) {
	m.capturedMetrics = metrics
	m.capturedSources = sources
	return m.parallelCoords, m.parallelCoordsErr
}

func (m *mockStore) GetSankey(_ context.Context, _, sources []string, _, _ time.Time, _ int) (storage.SankeyResult, error) {
	m.capturedSources = sources
	return m.sankey, m.sankeyErr
}

func (m *mockStore) GetEquivalenceStatus(_ context.Context, _ string) (*storage.EquivalenceStatusRow, error) {
	return m.equivalenceStatus, m.equivalenceStatusErr
}

func (m *mockStore) GetEntities(_ context.Context, start, end time.Time, sources []string, label *string, limit int) ([]storage.EntityRow, error) {
	m.capturedStart = start
	m.capturedEnd = end
	m.capturedSources = sources
	m.capturedLabel = label
	m.capturedLimit = limit
	return m.entities, m.entitiesErr
}

func (m *mockStore) GetLanguageDetections(_ context.Context, start, end time.Time, sources []string, language *string, limit int) ([]storage.LanguageDetectionRow, error) {
	m.capturedStart = start
	m.capturedEnd = end
	m.capturedSources = sources
	m.capturedLanguage = language
	m.capturedLimit = limit
	return m.languageDetections, m.languageDetectionsErr
}

func (m *mockStore) GetAvailableMetrics(_ context.Context, _, _ time.Time) ([]storage.AvailableMetricRow, error) {
	return m.availableMetrics, m.availableMetricsErr
}

func (m *mockStore) GetScopeAvailableMetrics(_ context.Context, _, _ time.Time, sources []string) (storage.ScopeMetricAvailability, error) {
	m.capturedSources = sources
	return m.scopeAvailableMetricsResult, m.scopeAvailableMetricsErr
}

func (m *mockStore) GetMetricValidationStatus(_ context.Context, _ string) (string, error) {
	return m.validationStatus, m.validationStatusErr
}

func (m *mockStore) GetMetricCulturalContextNotes(_ context.Context, _ string) (string, error) {
	return m.culturalContextNotes, m.culturalContextNotesErr
}

func (m *mockStore) GetMetricDistribution(_ context.Context, metricName string, sources []string, start, end time.Time, bins int, _ *storage.MetadataFilter) (storage.DistributionResult, error) {
	m.capturedMetricName = &metricName
	m.capturedSources = sources
	m.capturedStart = start
	m.capturedEnd = end
	m.capturedBins = bins
	return m.distribution, m.distributionErr
}

func (m *mockStore) GetMetricHeatmap(_ context.Context, metricName string, sources []string, xDim, yDim storage.HeatmapDimension, start, end time.Time) ([]storage.HeatmapCell, error) {
	m.capturedMetricName = &metricName
	m.capturedSources = sources
	m.capturedXDim = xDim
	m.capturedYDim = yDim
	m.capturedStart = start
	m.capturedEnd = end
	return m.heatmap, m.heatmapErr
}

func (m *mockStore) GetMetricCorrelation(_ context.Context, metricNames []string, sources []string, start, end time.Time, _ *storage.MetadataFilter) (storage.CorrelationResult, error) {
	m.capturedMetrics = metricNames
	m.capturedSources = sources
	m.capturedStart = start
	m.capturedEnd = end
	return m.correlation, m.correlationErr
}

func (m *mockStore) GetEntityCoOccurrence(_ context.Context, sources []string, start, end time.Time, topN int, _ string, _ string, _ int, _ bool, _ string, _ int) (storage.CoOccurrenceResult, error) {
	m.capturedSources = sources
	m.capturedStart = start
	m.capturedEnd = end
	m.capturedTopN = topN
	return m.cooccurrence, m.cooccurrenceErr
}

func (m *mockStore) GetTopicDistribution(_ context.Context, params storage.TopicDistributionParams) ([]storage.TopicDistributionRow, error) {
	m.capturedTopicParams = params
	m.capturedSources = params.Sources
	m.capturedStart = params.Start
	m.capturedEnd = params.End
	return m.topicDistribution, m.topicDistributionErr
}

func (m *mockStore) GetSilverDistribution(_ context.Context, field string, source string, start, end time.Time, bins int) (storage.DistributionResult, error) {
	m.capturedSilverField = field
	src := source
	m.capturedSource = &src
	m.capturedStart = start
	m.capturedEnd = end
	m.capturedBins = bins
	return m.silverDistribution, m.silverDistributionErr
}

func (m *mockStore) GetSilverHeatmap(_ context.Context, kind storage.SilverAggregationKind, source string, start, end time.Time) ([]storage.HeatmapCell, string, string, error) {
	m.capturedSilverKind = kind
	src := source
	m.capturedSource = &src
	m.capturedStart = start
	m.capturedEnd = end
	return m.silverHeatmap, m.silverHeatmapXDim, m.silverHeatmapYDim, m.silverHeatmapErr
}

func (m *mockStore) GetSilverCorrelation(_ context.Context, source string, start, end time.Time) (storage.SilverCorrelationResult, error) {
	src := source
	m.capturedSource = &src
	m.capturedStart = start
	m.capturedEnd = end
	return m.silverCorrelation, m.silverCorrelationErr
}

func (m *mockStore) GetMetadataCoverage(_ context.Context, sources []string) ([]storage.MetadataCoverageCell, error) {
	m.capturedSources = sources
	return m.metadataCoverage, m.metadataCoverageErr
}

func (m *mockStore) GetFieldCardinality(_ context.Context, sources []string) (map[storage.FieldKey]storage.FieldCardinality, error) {
	m.capturedSources = sources
	return m.fieldCardinality, m.fieldCardinalityErr
}

func (m *mockStore) GetGlobalFieldStats(_ context.Context) ([]storage.GlobalFieldStat, error) {
	return m.globalFieldStats, m.globalFieldStatsErr
}

func (m *mockStore) GetCategoricalDistribution(_ context.Context, _ string, sources []string, _, _ time.Time, _ int, _ *storage.MetadataFilter) (storage.CategoricalDistributionResult, error) {
	m.capturedSources = sources
	return m.categoricalDistribution, m.categoricalDistributionErr
}

func (m *mockStore) GetScopeAvailableMetadata(_ context.Context, _, _ time.Time, sources []string) (storage.ScopeMetadataAvailability, error) {
	m.capturedSources = sources
	return m.scopeAvailableMetadata, m.scopeAvailableMetadataErr
}

func (m *mockStore) GetCrossTab(_ context.Context, _, _ string, sources []string, _, _ time.Time, _ int, _ *storage.MetadataFilter) (storage.CrossTabResult, error) {
	m.capturedSources = sources
	return m.crossTab, m.crossTabErr
}

func (m *mockStore) GetRevisionActivity(_ context.Context, sources []string, start, end time.Time, _ storage.RevisionActivityResolution) ([]storage.RevisionActivityCell, error) {
	m.capturedSources = sources
	m.capturedStart = start
	m.capturedEnd = end
	return m.revisionActivity, m.revisionActivityErr
}

func (m *mockStore) GetRevisionDiscourseShift(_ context.Context, sources []string, start, end time.Time, _ storage.RevisionActivityResolution) ([]storage.RevisionDiscourseShiftCell, error) {
	m.capturedSources = sources
	m.capturedStart = start
	m.capturedEnd = end
	return m.revisionDiscourseShift, m.revisionDiscourseShiftErr
}

func (m *mockStore) GetRevisionEditClusters(_ context.Context, sources []string, start, end time.Time, _ storage.RevisionActivityResolution, minSources int) ([]storage.RevisionEditClusterRow, error) {
	m.capturedSources = sources
	m.capturedStart = start
	m.capturedEnd = end
	m.capturedMinSources = minSources
	return m.revisionEditClusters, m.revisionEditClustersErr
}

func (m *mockStore) GetArticleRevisions(_ context.Context, _ string) ([]storage.ArticleRevisionRow, error) {
	return m.articleRevisions, m.articleRevisionsErr
}

func (m *mockStore) GetArticleRevisionDiff(_ context.Context, _ string, _ int) (*storage.ArticleRevisionDiffRow, error) {
	return m.articleRevisionDiff, m.articleRevisionDiffErr
}

func (m *mockStore) GetRevisionsArticles(_ context.Context, filter storage.RevisionsArticlesFilter) ([]storage.RevisionArticleRow, error) {
	m.capturedRevisionsFilter = filter
	m.capturedSources = filter.Sources
	return m.revisionsArticles, m.revisionsArticlesErr
}
