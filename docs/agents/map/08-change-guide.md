---
component: duw-queue-monitor
repo: uladzk/duw-queue-monitor
commit: eb3b6bcfad73ce4a23ec989f96f937a46616fa6d
verified: 2026-07-23
confidence: high
audience: coding-agents, solo-developer
---

# Change guide (L2)

Files-to-touch recipes for the change types this repo actually sees. Conventional commits (`feat|fix|refactor|test|build|ops(scope): ...`); scope = the service/module. Branch off `main` (never commit to `main`). See `duw-git-workflow` skill for git conventions. Deploy is Kustomize `apply -k` after a `publish.yml` run.

## Ship any code change end-to-end

1. Branch, implement, `go build -v ./...` + `go test -v ./...` + `golangci-lint` locally (complexity ≤15).
2. PR to `main` → `pull_request.yaml` + `lint.yml` must be green.
3. Merge (no squash — individual commits preserved, project memory).
4. `publish.yml` (workflow_dispatch) with `service` + new semver → pushes `ghcr.io/uladzk/duw-queue-monitor/<service>:<version>` and tags `<service>-<version>`.
5. Bump the tag in the target overlay's `images:` block (`infra/k8s/overlays/ovh-prd/kustomization.yaml`), `kubectl apply -k infra/k8s/overlays/ovh-prd/`.
6. Verify the live version / behavior on the cluster — the pipeline "succeeded" is not proof of a live rollout.

## Add a field to the daily-stats table

The contract spans DB → sqlc → writer → reader.
1. New migration `db/migrations/0000N_<desc>.sql` (goose `Up`/`Down`).
2. If queried, edit `internal/dailystats/queries.sql`.
3. Regenerate: `cd internal/dailystats && sqlc generate` (updates `models.go`, `queries.sql.go`, `querier.go` — do not hand-edit). sqlc reads `db/migrations/` as its schema, so step 1 must exist first.
4. Update the hand-written wrapper `internal/dailystats/repository.go` (int→int32 conversions, method signature).
5. Writer side: `internal/queuemonitor/dailystats.go` (interface) + `internal/queuemonitor/monitor.go:79` (the `SaveDailyStats` call maps `Queue` fields → columns).
6. Reader side: `internal/statsreporting/notification.go` builders + `QueueDailyStat` field use.
7. **Rollout order:** ship the `duw-migrations` image + run the migrations Job *before* the service images that depend on the new column. Publish `duw-migrations` (`context ./db`), bump `duw-migrations-*-*-*` in `migrations-job.yml`, apply, confirm the Job succeeded, then roll the services.
Gate: the `(queue_id, date)` upsert is idempotent; a re-run won't duplicate rows. Test: `dailystats/repository_integration_test.go` (testcontainers Postgres).

## Add/adjust a queue-monitor notification or state transition

1. State logic lives in `internal/queuemonitor/state*.go` (one file per state). Add branches inside the relevant state's `Handle`; return the target state. Keep `gocyclo ≤ 15`.
2. Message text/format: `internal/queuemonitor/notification.go` (`msg*` constants + `buildQueueAvailableMsg` / `sendNotification`). Preserve HTML tags (Telegram `parse_mode:HTML`).
3. If a new state is added: register it in `StateFromPersistence` / `StateToPersistence` (`state.go`) or reloads after restart will mis-map. Add its `Name()` to both switches.
4. Test in `internal/queuemonitor/monitor_test.go` (transition table). Retire/extend `monitorstate_integration_test.go` if the persisted shape changes.
Gate: a new field on `MonitorState` (`monitorstate.go`) is JSON-persisted to Redis; old in-flight state (TTL 60s) will lack it — default-zero is fine, but don't make it load-bearing without a migration path.

## Add a telegram-bot command

1. Implement a `Handler` (+ optional `ReplyHandler`) in `internal/telegrambot/handlers/` mirroring `feedback.go`: `Register` calls `b.RegisterHandler(HandlerTypeMessageText, "<cmd>", MatchTypeCommand, ...)`; `GetReplyPatterns` returns the exact prompt strings if you use `ForceReply`.
2. Add it to the `handlersMap` in `NewHandlerRegistry` (`registry.go:47`) — this is the single wiring point; it also drives the `/`-menu and `SetMyCommands`.
3. Reply routing is by **exact** `ReplyToMessage.Text` match (`registry.go:34`) — keep the prompt constant and the registered pattern identical.
4. Test alongside `handlers/*_test.go` + `registry_test.go`.
Note: menu command descriptions currently equal the command name (`registry.go:74-76`); improving that is a `GetAvailableCommands` change.

## Add/change an env var (config)

Config structs: `internal/queuemonitor/config.go`, `internal/statsreporting/config.go`, `internal/telegrambot/config.go`, `internal/notifications/config.go`, `internal/logger/config.go`. Add the `env:"NAME" envDefault:"..."` tag (mark `,required` only if boot should fail without it — required vars crash startup). Then surface it in the k8s manifests (`infra/k8s/base/*-deployment.yml` or `-cronjob.yml`), and via ExternalSecret if it is a secret (`*-external-secret.yml` + the Infisical path). Reminder: `USE_TELEGRAM_NOTIFICATIONS` in the manifests has no struct field — don't assume manifest env == code config.

## Change working hours / interval / retry

Pure env changes — no code. Interval: `STATUS_CHECK_INTERVAL_SECONDS` (base=5, dev overlay=120). Hours: `WORKING_HOUR_START_UTC`/`WORKING_HOUR_END_UTC` (prd overlay patches END to 17 in `overlays/ovh-prd/patches/queue-monitor-prd-env.yaml`). Retries: `STATUS_CHECK_*` and `NOTIFICATION_TELEGRAM_*`. Edit the overlay patch or base manifest, `apply -k`. See `02-queue-monitor.md` / `05-data-and-shared.md` for the full tables.

## Change a CronJob schedule

`infra/k8s/base/queue-stats-reports-cronjob.yml`. Monthly is a shell-guarded `28-31` job (k8s has no last-day cron); keep the BusyBox-compatible epoch-arithmetic guard (no `date -d tomorrow`). `apply -k`.

## Infra / terraform change

OVH is live (`infra/terraform/ovh/{platform-shared,mks,k8s}`), Azure is inactive reference — don't `apply` Azure. Run terraform via `infra/scripts/provision-ovh.sh <module> <env>` (needs `envs/<env>/backend.hcl`). Commit prefix `ops(<module-specific-scope>): ...` (e.g. `ops(ovh-mks): ...`), not generic `infra`. Deployments are manual — never `apply`/deploy unless explicitly asked.
