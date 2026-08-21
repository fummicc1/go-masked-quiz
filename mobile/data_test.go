package main

import (
	"fmt"
	"strings"
	"testing"

	"github.com/fummicc1/go-masked-quiz/quizgen/quiz"
)

// TestQuizDataURLMatchesSchemaVersion guards the drift that already bit once:
// the schema was bumped to 2 while quizDataURL still pointed at the v1 path, so
// every remote fetch decoded to a version the client rejects and the app
// silently served the embedded snapshot forever.
func TestQuizDataURLMatchesSchemaVersion(t *testing.T) {
	want := fmt.Sprintf("/cdn/v%d/", quiz.SchemaVersion)
	if !strings.Contains(quizDataURL, want) {
		t.Errorf("quizDataURL = %q, want it to publish under %q (schema version %d)",
			quizDataURL, want, quiz.SchemaVersion)
	}
}

// TestEmbeddedBundleIsCurrentSchema keeps the offline fallback usable: an
// embedded snapshot from an older schema is rejected by decodeBundle, which
// would leave the app with nothing to show on a first run without network.
func TestEmbeddedBundleIsCurrentSchema(t *testing.T) {
	b, err := decodeBundle(embeddedQuizzes)
	if err != nil {
		t.Fatalf("decode embedded bundle: %v", err)
	}
	if len(b.Proposals) == 0 {
		t.Fatal("embedded bundle has no proposals")
	}
	for _, p := range b.Proposals {
		if len(p.Document.Blocks) == 0 && len(p.Quizzes) == 0 {
			t.Fatalf("proposal %q has neither document blocks nor quizzes", p.ID)
		}
	}
}
