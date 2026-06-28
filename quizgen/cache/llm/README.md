# LLM quiz cache

Committed, validated output of `quizgen llm-generate` — one `issue-<number>.json`
per golang/go proposal issue. Generation runs **locally** against a local ollama
server; the results are cached here and committed so CI can merge them
(`quizgen generate --llm-cache`) **without ever calling a model**.

Each entry records the `body_hash`, `model`, and `prompt_version` it was made
from. If an issue's body changes, the hash no longer matches and the entry is
treated as stale — `generate` skips it (mechanical quizzes still update) and
reports it, until you rerun `llm-generate` locally and commit the refresh.

Do not hand-edit these files; regenerate with `quizgen llm-generate`.
