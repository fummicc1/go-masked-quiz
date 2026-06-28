package llm

import "context"

// Generator produces a model response (expected to be a JSON string) for a
// system + user prompt. Both the local ollama client and the OpenAI-compatible
// client (used for Cloudflare Workers AI and similar) implement it, so the
// generation flow is provider-agnostic.
type Generator interface {
	Generate(ctx context.Context, system, user string, seed int64) (string, error)
}
