# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Repository layout

Four independent components share one repo, all centered on a self-hosted n8n + PostgreSQL + Evolution API (WhatsApp) stack used for the "Stammtisch" (weekly Thursday meetup) workflows:

- `docker-compose.yml` — local dev stack (n8n, Postgres 18, Evolution API). Commented-out Ollama / Open-WebUI services exist but are off by default. Several volumes are declared `external: true` (e.g. `n8n_n8n_data`, `n8n_evolution_instances`) so `docker compose up` will fail until they are pre-created with `docker volume create <name>`.
- `deployment/` — GitOps deployment to a k3s cluster via ArgoCD. See "Deployment" below.
- `wrapped/` — Go web app rendering a Spotify-Wrapped-style year recap of Stammtisch attendance. See "Wrapped app" below.
- `whatsapp-bot/` — Go service that ports the n8n "Zumba" workflow (the active one, `HG0zPlWsmPI3Mt7z`, which lives in the `zumba-staging` n8n instance). Receives Evolution-API webhook events, classifies Zu-/Absagen via Google Gemini, writes to `stammtisch_abwesenheit`, and answers `statistik` requests with the ranking. Same conventions as the other Go services (vanilla `net/http`, `lib/pq`); no UI, no mock fallback. The classifier prompt (`internal/classifier/system-prompt.txt`) and stats query (`internal/store/stats.sql`) are verbatim copies of the workflow's; the `statistik` ranking text (`internal/report`) is a 1:1 port of the n8n Code node — keep all three in sync with `system-prompt.txt` / `whatsapp-statistic.sql` and the `true`/`false`/`invalid` contract. `reference/zumba-workflow.json` is the exported source workflow. A new `/test` endpoint runs the same logic with the day/group guards bypassed and returns a structured `Outcome` JSON (used by the admin-ui Bot-Test page).
- `zumba-admin-ui/` — Go admin UI (templ + HTMX, `lib/pq`, mock fallback) for the Stammtisch data; reads the `zumba` DB. Phase 1 (read-only dashboard/members/days/excluded) and Phase 2 (writes: toggle attendance via `/toggle-absence`, manage excluded days — Thursday-validated — with HTMX + toasts) are implemented. The **Bot-Test** page (`/bot-test`) is a UI for the `whatsapp-bot` webhook: pick Statistik/Absage/Zusage, edit the example JSON, and it server-side-proxies to `BOT_URL/test`, rendering the bot's `Outcome`. Helm templates live in `deployment/helm-charts/zumba/templates/admin-ui/` (gated by `adminUi.enabled`; per-env `ingress.host`; needs an admin-ui image in `adminUi.image.repository`).
- `whatsapp-statistic/` — three standalone JS snippets (`original.js`, `dashboard.js`, `rpg.js`) that are pasted into n8n Code nodes. They consume `$input.all()` from a Postgres query node and emit a WhatsApp-formatted ranking message. They are siblings, not versions: pick one styling and keep them in sync with the SQL in `whatsapp-statistic.sql`.
- `system-prompt.txt`, `absagen.sql`, `whatsapp-statistic.sql` — the n8n LLM classifier prompt and the SQL queries it relies on. The classifier returns exactly `true` / `false` / `invalid`; do not change that contract without also updating the consuming n8n workflow.

The remote is `github.com/Undermyspell/ai` (referenced from `deployment/argocd/applicationset.yaml`); the deployment path inside that repo is `n8n/deployment/...`, i.e. the GitOps targets assume this directory is checked in under `n8n/` upstream.

## Domain model (shared across components)

The "Stammtisch" data model is **attendance-by-default**: a user is present on a Thursday unless they explicitly send a cancellation message. This is fundamental — there is no "attended" table.

Postgres tables (schema `public`):
- `users` — `userId`, `userName`, `startDate` (when the user joined; nullable). All evaluations clamp the start date to no earlier than `2025-12-01`.
- `stammtisch_abwesenheit` — one row per cancellation: `userId`, `date`, `message` (nullable). Only rows with `EXTRACT(DOW FROM date) = 4` (Thursday) are valid.
- `excluded_days` — Thursdays that don't count (holidays etc.). Always filter via `NOT IN (SELECT date FROM excluded_days)`.

Two databases share the same Postgres instance: `n8n` (n8n's own state) and `zumba` (the Stammtisch domain data the wrapped app reads). Evolution API uses the `evolution` schema in the `n8n` DB.

The "2026 Wrapped" period is **01.12.2025 – 30.11.2026** (defined in `wrapped/internal/handlers/wrapped.go`). All evaluation queries cap the end date at "today" so future Thursdays don't count as missed.

## Wrapped app (`wrapped/`)

Go web server that renders `/2026` from either the live Postgres or hardcoded mock data.

### Commands (run from `wrapped/`)
```bash
make dev      # hot reload via Air; runs `templ generate && go build` on change
make build    # one-shot build to ./tmp/server
make run      # build + run
make test     # go test -v ./...
go test -v ./internal/evaluations/2026 -run TestX   # run a single test
```
Air is configured via `.air.toml`; it watches `.go .templ .html .tpl .tmpl` and runs `templ generate` before each build. Any change to a `.templ` file requires regeneration before the binary will compile — `make dev` handles this; `make build` does **not**, so run `templ generate ./...` manually after editing templates if you're not using Air. Generated `*_templ.go` files are gitignored.

### DB connection
`internal/database/postgres.go` reads `DB_HOST/DB_PORT/DB_USER/DB_PASSWORD/DB_NAME/DB_SSLMODE` from env, defaulting to `192.168.178.46:5433` / db `zumba`. **If the DB is unreachable, the app silently falls back to mock data** (`data/mock.go`, 15 hardcoded users) and logs a warning — a green-looking page does not mean the DB query path works. To force-test the DB path, set the env vars and confirm the `✅ Connected to PostgreSQL` log line.

### Pipeline
Request → `handlers.WrappedHandler.Handle2026` → `repository.RejectionRepository.GetRawDataByDateRange` (4 queries: users, rejections, excluded days, valid Thursdays) → `evaluations/2026.Evaluator.Evaluate` (pure function over `RawData`, no DB) → `viewbuilder.Build` (turns `EvaluationResult` into a `PageViewModel` of presentation strings/colors/CSS delay classes) → templ render.

The split between `evaluations/2026/` (domain stats) and `viewbuilder/` (presentation) is intentional: keep date math, streak logic, and category classification in `evaluations/`; keep emoji, color classes, copy strings, and Tailwind-specific output in `viewbuilder/`. A new year (e.g. 2027) is meant to be added as `evaluations/2027/` + `web/templates/years/2027/` + a new handler — do not edit the 2026 packages in place.

`viewbuilder.buildAIStats` randomly picks one of three German summary blurbs per request. `buildHeatmap` and friends still hardcode `2025-MM` keys for `MonthStats` despite the period extending into 2026 — when extending, audit those formats.

## Deployment (`deployment/`)

GitOps via ArgoCD `ApplicationSet` → 2 Applications: `zumba-staging` (ns `zumba-staging`, `http://zumba-stage.pi.home`) and `zumba-production` (ns `zumba-production`, `http://zumba.pi.home`). Each Application has **two sources**:

1. The Helm chart `helm-charts/zumba/` (n8n + Postgres + Evolution API + IngressRoute) with `valueFiles: ../../environments/{{.env}}/values.yaml`.
2. Kustomize at `environments/{{.env}}/` which only emits SealedSecrets.

Because of source 1's relative `valueFiles`, the Helm chart and `environments/` must stay co-located under `deployment/`. Renovate updates the n8n image tag in `helm-charts/zumba/values.yaml` and the local `docker-compose.yml` together (see recent commits).

Secrets are encrypted **per environment** with bitnami SealedSecrets and committed to git; staging and production use different keys. Generate via `deployment/scripts/create-sealed-secret.sh <env> <name> KEY=VALUE...` — see `deployment/README.md` for the full operator runbook (rotation, restart, troubleshooting, ArgoCD UI access).

Common ops shortcuts:
```bash
kubectl get applications -n argocd
kubectl logs -n zumba-staging -l app.kubernetes.io/component=n8n -f
kubectl rollout restart deployment/zumba-n8n -n zumba-staging
```

## Conventions

- All user-facing strings are **German**. Don't translate UI/SQL/log text or "fix" date ordering (DD.MM, ISO week starts Monday, Thursday=ISODOW 4).
- Do not commit decrypted Kubernetes Secrets. Only `SealedSecret` resources belong in `environments/*/sealed-secrets/`.
- Don't edit `*_templ.go` files — they're generated from sibling `.templ` files.
