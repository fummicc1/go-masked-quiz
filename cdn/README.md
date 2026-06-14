# CDN (Cloudflare Pages)

Static delivery of the quiz bundle. No server, no database, no Functions — just
files, so there is no per-request billing to exploit and Cloudflare's DDoS
mitigation applies by default.

## Layout
```
cdn/
├── _headers          # Cache-Control / CORS / content-type for /v3/*.json
└── v3/
    └── quizzes.json  # the bundle (schema v3); refreshed by the generate workflow
```

## Cloudflare Pages setup (one-time, dashboard)
1. Create a Pages project connected to this GitHub repo.
2. Build settings: **no build command**, **output directory `cdn`**, production branch `main`.
3. After deploy, the bundle is served at:
   `https://<project>.pages.dev/v3/quizzes.json`
4. Enable **Bot Fight Mode** (free) under Security.

## Updating the bundle
`cdn/v3/quizzes.json` is updated automatically by the daily `generate` workflow
in the `golang-proposal` fork (see that repo's `.github/workflows/generate.yml`):
upstream sync → `quizgen generate` → commit here only when the content changes.

## iOS client
Set `Configuration.quizDataURL` to the Pages URL once deployed. The loader is
remote → cache → bundle, so the app keeps working if the CDN is unreachable.
