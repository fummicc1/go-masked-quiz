package main

import (
	_ "embed"
	"io"
	"net/http"
)

// embeddedQuizzes is the offline fallback: a snapshot of the published bundle
// compiled into the binary, so a first run with no network still has content.
//
// go:embed cannot reach outside the module, so this is a copy of cdn/v2 rather
// than a reference to it. The copy has drifted from the published bundle before
// now, so refresh it with `go generate ./...` rather than by hand.
//
//go:generate cp ../cdn/v2/quizzes.json assets/quizzes.json
//go:embed assets/quizzes.json
var embeddedQuizzes []byte

// maxBundleBytes caps how much of a remote response is read. The published
// bundle is a few MB; anything far larger is a misconfigured endpoint rather
// than data worth loading onto a phone.
const maxBundleBytes = 32 << 20

func readAllLimited(resp *http.Response) ([]byte, error) {
	return io.ReadAll(io.LimitReader(resp.Body, maxBundleBytes))
}
