package handler

import (
	"context"
	"log/slog"
)

// GetMetadataFields handles GET /metadata-fields (Task C).
//
// Returns the corpus-wide extraction status for every observed Tier-B / Tier-C
// metadata field: how often AĒR actually populated it, across how many sources,
// and whether it is constant everywhere. Backs the Reflection "metadata fields"
// surface, which explains the tier system and frames an empty field as
// structural absence (a publisher choice, WP-003 §3.2) rather than an AĒR
// extraction defect. The tier and the prose description are curated client-side;
// only the live measurements are served here so the page cannot drift from the
// real corpus.
func (s *Server) GetMetadataFields(ctx context.Context, _ GetMetadataFieldsRequestObject) (GetMetadataFieldsResponseObject, error) {
	stats, err := s.db.GetGlobalFieldStats(ctx)
	if err != nil {
		slog.Error("handler failure", "op", "GetMetadataFields", "error", err)
		return GetMetadataFields500JSONResponse{Message: genericInternalError}, nil
	}

	resp := GetMetadataFields200JSONResponse{}
	resp.Fields = make([]struct {
		Constant          bool    `json:"constant"`
		ConstantValue     *string `json:"constantValue,omitempty"`
		DistinctValues    int     `json:"distinctValues"`
		Field             string  `json:"field"`
		PopulatedArticles int     `json:"populatedArticles"`
		PopulationRate    float64 `json:"populationRate"`
		SourcesObserved   int     `json:"sourcesObserved"`
		SourcesPopulated  int     `json:"sourcesPopulated"`
		TotalArticles     int     `json:"totalArticles"`
	}, len(stats))
	for i, st := range stats {
		var cv *string
		if st.Constant {
			v := st.ConstantValue
			cv = &v
		}
		resp.Fields[i] = struct {
			Constant          bool    `json:"constant"`
			ConstantValue     *string `json:"constantValue,omitempty"`
			DistinctValues    int     `json:"distinctValues"`
			Field             string  `json:"field"`
			PopulatedArticles int     `json:"populatedArticles"`
			PopulationRate    float64 `json:"populationRate"`
			SourcesObserved   int     `json:"sourcesObserved"`
			SourcesPopulated  int     `json:"sourcesPopulated"`
			TotalArticles     int     `json:"totalArticles"`
		}{
			Constant:          st.Constant,
			ConstantValue:     cv,
			DistinctValues:    int(st.DistinctValues), //nolint:gosec // bounded by field cardinality
			Field:             st.Field,
			PopulatedArticles: int(st.PopulatedArticles), //nolint:gosec // bounded by TTL
			PopulationRate:    st.PopulationRate,
			SourcesObserved:   st.SourcesObserved,
			SourcesPopulated:  st.SourcesPopulated,
			TotalArticles:     int(st.TotalArticles), //nolint:gosec // bounded by TTL
		}
	}
	return resp, nil
}
