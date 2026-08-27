package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/fummicc1/go-masked-quiz/quizgen/quiz"
)

// TestQuizDataURLMatchesSchemaVersion guards the drift that already bit once:
// the schema was bumped to 2 while quizDataURL still pointed at the v1 path, so
// every remote fetch decoded to a version the client rejects.
func TestQuizDataURLMatchesSchemaVersion(t *testing.T) {
	want := fmt.Sprintf("/cdn/v%d/", quiz.SchemaVersion)
	if !strings.Contains(quizDataURL, want) {
		t.Errorf("quizDataURL = %q, want it to publish under %q (schema version %d)",
			quizDataURL, want, quiz.SchemaVersion)
	}
}

// deadContext is a context that is already cancelled, so fetchRemote fails
// without the test needing a network at all.
func deadContext(t *testing.T) context.Context {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	return ctx
}

// TestLoadBundleReportsFailure is the regression guard for dropping the
// embedded snapshot: a failed fetch with no cache has to surface as an error.
// While the snapshot existed this path returned a usable bundle, which is how a
// week of broken fetches went unnoticed.
func TestLoadBundleReportsFailure(t *testing.T) {
	b, src, err := loadBundle(deadContext(t), "")
	if err == nil {
		t.Fatal("loadBundle succeeded with no network and no cache; a silent fallback is back")
	}
	if src != "" {
		t.Errorf("source = %q, want empty on failure", src)
	}
	if len(b.Proposals) != 0 {
		t.Errorf("got %d proposals on failure, want none", len(b.Proposals))
	}
}

// TestLoadBundleUsesCacheWhenFetchFails covers the one tier left behind the
// network: a previous successful fetch keeps the app usable offline.
func TestLoadBundleUsesCacheWhenFetchFails(t *testing.T) {
	raw, err := os.ReadFile("testdata/bundle.json")
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	cache := filepath.Join(t.TempDir(), "quizzes.json")
	if err := os.WriteFile(cache, raw, 0o644); err != nil {
		t.Fatalf("seed cache: %v", err)
	}

	b, src, err := loadBundle(deadContext(t), cache)
	if err != nil {
		t.Fatalf("loadBundle with a warm cache: %v", err)
	}
	if src != SourceCache {
		t.Errorf("source = %q, want %q", src, SourceCache)
	}
	if len(b.Proposals) == 0 {
		t.Error("cached bundle has no proposals")
	}
}

// TestFilterProposals pins the matching contract to the display helpers: what
// the badge and title show is exactly what a query can hit, so a proposal the
// user can read on screen is always findable by retyping it.
func TestFilterProposals(t *testing.T) {
	props := []quiz.Proposal{
		{ID: "design-61405-range-over-func", Title: "Proposal: Range over func"},
		{ID: "issue-73787", Title: "encoding/json/v2: new API"},
	}
	cases := []struct {
		name  string
		query string
		want  []int
	}{
		{"empty keeps everything", "", []int{0, 1}},
		{"title is case-insensitive", "RANGE", []int{0}},
		{"design number", "61405", []int{0}},
		{"issue number", "73787", []int{1}},
		{"surrounding space is trimmed", "  json  ", []int{1}},
		{"no match", "zzz", []int{}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := filterProposals(props, tc.query)
			if len(got) != len(tc.want) {
				t.Fatalf("filterProposals(%q) = %v, want %v", tc.query, got, tc.want)
			}
			for i := range got {
				if got[i] != tc.want[i] {
					t.Fatalf("filterProposals(%q) = %v, want %v", tc.query, got, tc.want)
				}
			}
		})
	}
}

// TestFixtureIsCurrentSchema keeps the render fixture decodable. It is a subset
// of the published bundle, so it goes stale the same way the embedded snapshot
// did — the difference is that only tests depend on it.
func TestFixtureIsCurrentSchema(t *testing.T) {
	raw, err := os.ReadFile("testdata/bundle.json")
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	b, err := decodeBundle(raw)
	if err != nil {
		t.Fatalf("decode fixture: %v", err)
	}
	if len(b.Proposals) == 0 {
		t.Fatal("fixture has no proposals")
	}
	for _, p := range b.Proposals {
		if len(p.Document.Blocks) == 0 && len(p.Quizzes) == 0 {
			t.Fatalf("proposal %q has neither document blocks nor quizzes", p.ID)
		}
	}
}
