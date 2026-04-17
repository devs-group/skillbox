package github

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestSearch_SortsByStarsDesc verifies marketplace search returns results
// sorted by stargazers count descending regardless of upstream order.
func TestSearch_SortsByStarsDesc(t *testing.T) {
	type ghRepo struct {
		FullName        string `json:"full_name"`
		Description     string `json:"description"`
		StargazersCount int    `json:"stargazers_count"`
	}
	type ghItem struct {
		Name       string `json:"name"`
		Path       string `json:"path"`
		HTMLURL    string `json:"html_url"`
		Repository ghRepo `json:"repository"`
	}
	type ghResp struct {
		TotalCount int      `json:"total_count"`
		Items      []ghItem `json:"items"`
	}

	upstream := ghResp{
		TotalCount: 4,
		Items: []ghItem{
			{Name: "SKILL.md", Path: "skills/alpha/SKILL.md", Repository: ghRepo{FullName: "owner/alpha", StargazersCount: 10}},
			{Name: "SKILL.md", Path: "skills/beta/SKILL.md", Repository: ghRepo{FullName: "owner/beta", StargazersCount: 500}},
			{Name: "SKILL.md", Path: "skills/gamma/SKILL.md", Repository: ghRepo{FullName: "owner/gamma", StargazersCount: 0}},
			{Name: "SKILL.md", Path: "skills/delta/SKILL.md", Repository: ghRepo{FullName: "owner/delta", StargazersCount: 120}},
		},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.URL.Path, "/search/code") {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(upstream)
	}))
	defer srv.Close()

	m := &MarketplaceService{
		githubToken: "test-token",
		httpClient:  srv.Client(),
		apiBaseURL:  srv.URL,
	}

	resp, err := m.Search(context.Background(), "test", 1)
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}
	if len(resp.Results) != 4 {
		t.Fatalf("expected 4 results, got %d", len(resp.Results))
	}
	wantOrder := []int{500, 120, 10, 0}
	for i, want := range wantOrder {
		if resp.Results[i].Stars != want {
			t.Errorf("position %d: got stars=%d, want %d (full order: %+v)",
				i, resp.Results[i].Stars, want, starsOf(resp.Results))
		}
	}
}

func starsOf(rs []SearchResult) []int {
	out := make([]int, len(rs))
	for i, r := range rs {
		out[i] = r.Stars
	}
	return out
}
