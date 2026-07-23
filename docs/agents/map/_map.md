---
component: duw-queue-monitor
repo: uladzk/duw-queue-monitor
commit: eb3b6bcfad73ce4a23ec989f96f937a46616fa6d
verified: 2026-07-23
confidence: high
audience: coding-agents, solo-developer
anchor-paths:
  - cmd/
  - internal/
  - db/migrations/
  - infra/k8s/
  - infra/terraform/ovh/
  - .github/workflows/
---

# DUW Queue Monitor — code map (L0 index)

Notification system that watches the Dolnośląski Urząd Wojewódzki (DUW) residence-card queue and posts near-real-time Telegram alerts when it becomes available. Three Go binaries share one module (`github.com/uladzk/duw-queue-monitor`, go 1.25.3) and two data stores. Production runs on OVH MKS cluster `mks-duw-prd-waw`; images at `ghcr.io/uladzk/duw-queue-monitor/*`. Azure terraform/manifests are preserved but the Azure runtime was destroyed (see `06-infra.md`).

## Must-not-be-wrong facts

1. **Three services, one module.** `queue-monitor` (long-running loop), `telegram-bot` (interactive bot), `queue-stats-reports` (CLI run as CronJobs). Entry points: `cmd/queuemonitor/main.go`, `cmd/telegrambot/main.go`, `cmd/queuestatsreports/main.go`. Shared: `internal/logger`, `internal/notifications`.
2. **queue-monitor is a persisted state machine.** States `Uninitialized|Inactive|ActiveEnabled|ActiveDisabled` (`internal/queuemonitor/state*.go`); a notification fires only on transition or ticket-count change, never on repeated identical state. State is persisted to Redis key `monitor:state` (`internal/queuemonitor/monitorstate.go:29`).
3. **DUW API is polled, User-Agent must be empty.** GET `https://rezerwacje.duw.pl/status_kolejek/query.php?status=` with header `User-Agent: ""` — DUW returns no data otherwise (`internal/queuemonitor/statuscollector.go:54`). Response is `{"result": {city: [Queue,...]}}`; the monitored queue is matched by `id==24` in city `Wrocław` (defaults, `config.go:16-17`).
4. **`WeekdayQueueMonitor` gates the loop by clock, not the API.** It skips checks on Sat/Sun and outside `WORKING_HOUR_START_UTC`..`WORKING_HOUR_END_UTC` (default 5..18; prd overlay patches end to 17). Reason: DUW API reports the queue "active" on weekends (`internal/queuemonitor/monitorweekday.go:52-63`).
5. **Notifications go to a channel handle, feedback to a chat id.** State alerts use `chatID = "@" + NOTIFICATION_TELEGRAM_BROADCAST_CHANNEL_NAME` (`notification.go:40`); bot feedback forwards raw `NOTIFICATION_TELEGRAM_FEEDBACK_CHAT_ID` (no `@`). All Telegram sends go through `internal/notifications/telegram.go` (POST `{base}/bot{token}/sendMessage`, `parse_mode:HTML`).
6. **Daily stats persist only on the ActiveEnabled/ActiveDisabled → Inactive transition**, gated by `FF_DAILY_STATS_ENABLED` and a non-nil Postgres repo (`monitor.go:65-67`). It writes `MaxTickets` as `total_tickets_available` and `RegisteredTickets` as `taken_tickets` (`monitor.go:79`).
7. **Postgres table is `queue_daily_stats`, accessed via sqlc.** Two queries: `UpsertDailyStats` (on-conflict `(queue_id,date)`) and `GetStatsByDateRange` (`internal/dailystats/queries.sql`). Migrations under `db/migrations/` run by goose. Reports service reads this table; queue-monitor writes it.
8. **Config is 100% env vars** (`caarlos0/env`), no config files. Redis conn (`STATE_REDIS_CONNECTION_STRING`) and channel name are `required`; the service crashes at startup if unset.
9. **Publish target is registry-var-controlled.** `.github/workflows/publish.yml` pushes to GHCR unless repo var `PUBLISH_TO_GHCR == 'false'` (then ACR). Prod deploys GHCR images; the ACR refs in `infra/k8s/base/*` are rewritten by each overlay's kustomize `images:` block.
10. **k8s is Kustomize base + 4 overlays** (`azure-prd`, `azure-dev`, `ovh-prd`, `ovh-dev`). Only ovh-* are live. Deployments use `strategy: Recreate` (queue-monitor/telegram-bot) to avoid two instances double-posting.

## Doc index

| Doc | Read when… |
|-----|-----------|
| `01-overview.md` | You need the system shape, the context diagram, and one full request lifecycle. |
| `02-queue-monitor.md` | Touching the poll loop, state machine, retry, Redis persistence, or working-hours gating. |
| `03-telegram-bot.md` | Touching the interactive bot, command handlers, or feedback forwarding. |
| `04-stats-reports.md` | Touching report generation, the `--period` CLI, or message formatting. |
| `05-data-and-shared.md` | Touching Postgres/sqlc, migrations, Redis schema, the logger, or the Telegram notifier. |
| `06-infra.md` | Touching k8s manifests, overlays, CNPG/Redis, terraform (ovh live / azure inactive), or provisioning. |
| `07-ci-cd.md` | Touching workflows, Dockerfiles, publish/release, versioning, or local run. |
| `08-change-guide.md` | Planning a change — files-to-touch recipes, contract gates, deploy constraints per change type. |
| `09-reliability.md` | Diagnosing a symptom (duplicate posts, morning flood, frozen loop) or reasoning about failure modes. |

## Provenance

Pinned to commit `eb3b6bcf` on branch `docs-code-map`, verified 2026-07-23 against source. Every mechanism claim carries a `path:line` anchor. This is architecture/design-level documentation — it ages in months. Re-verify only when commits since the pin substantially touch the `anchor-paths` above, when observed code contradicts a claim, or before high-stakes work in a mapped area. CLAUDE.md in the repo root is a useful orientation but its infra section is partly stale (Azure) — trust this map's `06-infra.md` over it.
