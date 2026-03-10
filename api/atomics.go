package api

import (
	"net/http"
	"sort"
)

func (s *Server) handleListAtomics(w http.ResponseWriter, r *http.Request) {
	tactic   := r.URL.Query().Get("tactic")
	platform := r.URL.Query().Get("platform")

	techniques := s.atomics.Filter(tactic, platform)

	type testSummary struct {
		Index int    `json:"index"`
		Name  string `json:"name"`
	}
	type item struct {
		ID          string        `json:"id"`
		DisplayName string        `json:"display_name"`
		Tactic      string        `json:"tactic"`
		Platforms   []string      `json:"platforms"`
		Tests       []testSummary `json:"tests"`
	}

	out := make([]item, 0, len(techniques))
	for _, t := range techniques {
		tests := make([]testSummary, len(t.Tests))
		for i, test := range t.Tests {
			tests[i] = testSummary{Index: i, Name: test.Name}
		}
		out = append(out, item{
			ID:          t.ID,
			DisplayName: t.DisplayName,
			Tactic:      t.Tactic,
			Platforms:   t.Platforms,
			Tests:       tests,
		})
	}

	// Stable sort by technique ID
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) handleGetAtomic(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	t, ok := s.atomics.Get(id)
	if !ok {
		writeError(w, http.StatusNotFound, "technique not found: "+id)
		return
	}
	writeJSON(w, http.StatusOK, t)
}
