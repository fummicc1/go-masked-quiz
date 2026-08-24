package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"gioui.org/app"
	"github.com/fummicc1/go-masked-quiz/quizgen/quiz"
)

// quizDataURL serves the published bundle straight from the repo's cdn/ via
// jsDelivr, so there is no server to run.
const quizDataURL = "https://cdn.jsdelivr.net/gh/fummicc1/go-masked-quiz@main/cdn/v2/quizzes.json"

// Source records which tier of loadBundle satisfied the request, shown in the
// UI so a stale demo is never mistaken for live data.
type Source string

const (
	SourceRemote Source = "remote"
	SourceCache  Source = "cache"
)

// maxBundleBytes caps how much of a remote response is read. The published
// bundle is a few MB; anything far larger is a misconfigured endpoint rather
// than data worth loading onto a phone.
const maxBundleBytes = 32 << 20

// loadBundle fetches the published bundle, falling back to the cached copy of a
// previous fetch.
//
// There is deliberately no embedded snapshot behind these two tiers. One used to
// exist, and for the week the client spent pointed at a stale cdn path it turned
// a broken fetch into a silently outdated app instead of a visible failure.
// Reporting the error is what makes that class of bug survivable.
func loadBundle(ctx context.Context, cachePath string) (quiz.Bundle, Source, error) {
	b, raw, err := fetchRemote(ctx)
	if err == nil {
		if cachePath != "" {
			_ = os.MkdirAll(filepath.Dir(cachePath), 0o755)
			_ = os.WriteFile(cachePath, raw, 0o644)
		}
		return b, SourceRemote, nil
	}
	if cachePath != "" {
		if cached, rerr := os.ReadFile(cachePath); rerr == nil {
			if b, derr := decodeBundle(cached); derr == nil {
				return b, SourceCache, nil
			}
		}
	}
	return quiz.Bundle{}, "", err
}

func fetchRemote(ctx context.Context) (quiz.Bundle, []byte, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, quizDataURL, nil)
	if err != nil {
		return quiz.Bundle{}, nil, err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return quiz.Bundle{}, nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return quiz.Bundle{}, nil, fmt.Errorf("fetch quizzes: unexpected status %s", resp.Status)
	}
	raw, err := io.ReadAll(io.LimitReader(resp.Body, maxBundleBytes))
	if err != nil {
		return quiz.Bundle{}, nil, err
	}
	b, err := decodeBundle(raw)
	if err != nil {
		return quiz.Bundle{}, nil, err
	}
	return b, raw, nil
}

func decodeBundle(raw []byte) (quiz.Bundle, error) {
	var b quiz.Bundle
	if err := json.Unmarshal(raw, &b); err != nil {
		return quiz.Bundle{}, fmt.Errorf("decode bundle: %w", err)
	}
	if b.Version != quiz.SchemaVersion {
		return quiz.Bundle{}, fmt.Errorf("decode bundle: unsupported schema version %d", b.Version)
	}
	return b, nil
}

// cacheFilePath resolves the on-device cache location. app.DataDir panics if
// called before main is running, so it is only ever called from the app loop.
func cacheFilePath() string {
	dir, err := app.DataDir()
	if err != nil {
		return ""
	}
	return filepath.Join(dir, "quizzes.json")
}

// displayNumber is the short badge shown per proposal. Issue-sourced ids look
// like "issue-73787" and design docs like "design-61405-range-over-func", so
// take the first numeric segment of the id; fall back to the issue number, then
// to a placeholder.
func displayNumber(p quiz.Proposal) string {
	for _, part := range strings.Split(p.ID, "-") {
		if _, err := strconv.Atoi(part); err == nil {
			return part
		}
	}
	if p.IssueNumber > 0 {
		return strconv.Itoa(p.IssueNumber)
	}
	return "GO"
}

// displayTitle drops the redundant "Proposal:" prefix most design docs carry.
func displayTitle(p quiz.Proposal) string {
	return strings.TrimSpace(strings.TrimPrefix(p.Title, "Proposal:"))
}

// blankCount is how many blanks a proposal's document holds.
func blankCount(p quiz.Proposal) int {
	return len(p.Document.Blanks)
}
