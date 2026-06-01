package handler

import (
	"context"
	"log/slog"
	"strings"
	"time"

	"github.com/frogfromlake/aer/services/bff-api/internal/storage"
)

// outlierTopicID is BERTopic's reserved outlier class (Phase 120). The
// extractor stores it on disk; this handler relabels it for the rendering
// layer per the Phase 121 design ("uncategorised" ridge, not hidden).
const outlierTopicID int32 = -1

// outlierLabel is the canonical label surfaced to the frontend when
// `includeOutlier=true` returns the outlier topic.
const outlierLabel = "uncategorised"

// GetTopicDistribution backs the /topics/distribution endpoint (Phase 120).
//
// Resolves the multi-scope (scopeId / probeIds / sourceIds) like the other
// view-mode endpoints, validates the optional language filter against the
// Capability Manifest (Phase 118a / ADR-024), and aggregates
// `aer_gold.topic_assignments` for the union of resolved sources. Topics
// are partitioned by language at the model layer, so requesting cross-
// language results returns one entry per (language, topic_id) pair —
// frontends must render per-language sub-collections (Phase 121
// "language-partition awareness" requirement).
func (s *Server) GetTopicDistribution(ctx context.Context, request GetTopicDistributionRequestObject) (GetTopicDistributionResponseObject, error) {
	rawScope := ""
	if request.Params.Scope != nil {
		rawScope = string(*request.Params.Scope)
	}
	kind, sources, _, reason, ok := s.resolveScopeMulti(rawScope, request.Params.ScopeId, request.Params.ProbeIds, request.Params.SourceIds)
	if !ok {
		if strings.HasPrefix(reason, "unknown probe") {
			return GetTopicDistribution404JSONResponse{Message: reason}, nil
		}
		return GetTopicDistribution400JSONResponse{Message: reason}, nil
	}

	// topic_distribution is synchronic: with NO window it shows the single
	// newest BERTopic sweep (one coherent model — BERTopic topic_ids are unique
	// only within a sweep, so aggregating across sweeps would conflate distinct
	// topics; see WP-004 §3.4). A supplied window keeps the diachronic overlap
	// behaviour the evolution view relies on (it passes explicit per-bucket
	// windows). Each bound is independently optional; resolveWindow opens the
	// missing side.
	latestSweep := request.Params.Start == nil && request.Params.End == nil
	var start, end time.Time
	if !latestSweep {
		var msg string
		start, end, msg = resolveWindow(request.Params.Start, request.Params.End)
		if msg != "" {
			return GetTopicDistribution400JSONResponse{Message: msg}, nil
		}
	}

	if errBody, ok := s.validateLanguageQueryParam(request.Params.Language); !ok {
		gate := errBody.Gate
		anchor := errBody.WorkingPaperAnchor
		alts := errBody.Alternatives
		return GetTopicDistribution400JSONResponse{
			Message:            errBody.Message,
			Gate:               &gate,
			WorkingPaperAnchor: &anchor,
			Alternatives:       &alts,
		}, nil
	}

	var minConfidence float32
	if request.Params.MinConfidence != nil {
		minConfidence = *request.Params.MinConfidence
	}
	includeOutlier := false
	if request.Params.IncludeOutlier != nil {
		includeOutlier = *request.Params.IncludeOutlier
	}

	rows, err := s.db.GetTopicDistribution(ctx, storage.TopicDistributionParams{
		Sources:        sources,
		Language:       request.Params.Language,
		Start:          start,
		End:            end,
		LatestSweep:    latestSweep,
		MinConfidence:  minConfidence,
		IncludeOutlier: includeOutlier,
		Limit:          50,
	})
	if err != nil {
		slog.Error("handler failure", "op", "GetTopicDistribution", "error", err)
		return GetTopicDistribution500JSONResponse{Message: genericInternalError}, nil
	}

	// In latest-sweep mode the echoed window reflects the actual sweep the rows
	// came from (so the UI can label "topics over <start>–<end>"); in windowed
	// mode it echoes the requested window.
	windowStart, windowEnd := start, end
	if latestSweep && len(rows) > 0 {
		windowStart, windowEnd = rows[0].WindowStart, rows[0].WindowEnd
	}
	resp := GetTopicDistribution200JSONResponse{
		Scope:       strPtr(string(kind)),
		ScopeId:     request.Params.ScopeId,
		WindowStart: windowStart,
		WindowEnd:   windowEnd,
	}
	if request.Params.Language != nil {
		lang := *request.Params.Language
		resp.Language = &lang
	}
	resp.Topics = make([]struct {
		ArticleCount  int64   `json:"articleCount"`
		AvgConfidence float32 `json:"avgConfidence"`
		Label         string  `json:"label"`
		Language      string  `json:"language"`
		ModelHash     *string `json:"modelHash,omitempty"`
		TopicId       int32   `json:"topicId"`
	}, len(rows))
	for i, r := range rows {
		label := r.Label
		if r.TopicID == outlierTopicID {
			// Always relabel the outlier — the storage row may carry an
			// empty string or BERTopic's verbose c-TF-IDF placeholder; the
			// frontend renders this as a greyed "uncategorised" ridge.
			label = outlierLabel
		}
		var modelHash *string
		if r.ModelHash != "" {
			h := r.ModelHash
			modelHash = &h
		}
		resp.Topics[i] = struct {
			ArticleCount  int64   `json:"articleCount"`
			AvgConfidence float32 `json:"avgConfidence"`
			Label         string  `json:"label"`
			Language      string  `json:"language"`
			ModelHash     *string `json:"modelHash,omitempty"`
			TopicId       int32   `json:"topicId"`
		}{
			ArticleCount:  r.ArticleCount,
			AvgConfidence: float32(r.AvgConf),
			Label:         label,
			Language:      r.Language,
			ModelHash:     modelHash,
			TopicId:       r.TopicID,
		}
	}
	return resp, nil
}
