package llm

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestEmbedRoundTrip(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/embed" {
			t.Errorf("path = %s, want /api/embed", r.URL.Path)
		}
		var req embedRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		// Vectors vary by position so a swapped or dropped input would show up
		// in the assertion, not just a wrong count.
		out := embedResponse{}
		for i := range req.Input {
			out.Embeddings = append(out.Embeddings, []float64{float64(i), 1})
		}
		json.NewEncoder(w).Encode(out)
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "test-embed")
	got, err := c.Embed(context.Background(), []string{"alpha", "beta"})
	if err != nil {
		t.Fatal(err)
	}
	want := [][]float64{{0, 1}, {1, 1}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Embed = %v, want %v", got, want)
	}
}

func TestEmbedRejectsCountMismatch(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(embedResponse{Embeddings: [][]float64{{1}}})
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "test-embed")
	if _, err := c.Embed(context.Background(), []string{"a", "b"}); err == nil {
		t.Fatal("want error on embedding/input count mismatch, got nil")
	}
}

func TestEmbedCacheRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sub", "embeddings.json")
	in := &EmbedCache{Model: "m", Vectors: map[string][]float64{"tok": {0.5, -1}}}
	if err := SaveEmbedCache(path, in); err != nil {
		t.Fatal(err)
	}
	out, err := LoadEmbedCache(path)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(in, out) {
		t.Fatalf("cache round trip = %+v, want %+v", out, in)
	}
}

func TestLoadEmbedCacheMissingIsNotExist(t *testing.T) {
	_, err := LoadEmbedCache(filepath.Join(t.TempDir(), "nope.json"))
	if !os.IsNotExist(err) {
		t.Fatalf("err = %v, want os.IsNotExist", err)
	}
}
