---
component: duw-queue-monitor
repo: uladzk/duw-queue-monitor
commit: eb3b6bcfad73ce4a23ec989f96f937a46616fa6d
verified: 2026-07-23
confidence: high
audience: coding-agents, solo-developer
---

# queue-stats-reports service (L2)

Run-to-completion CLI, deployed as three k8s CronJobs (daily / weekly / monthly). Package `internal/statsreporting`, entry `cmd/queuestatsreports/main.go`. Reads `queue_daily_stats` from Postgres, formats a Polish summary, posts it to the broadcast channel, exits. Read-only against the DB.

## CLI & wiring (`cmd/queuestatsreports/main.go`)

- `--period` flag, values `daily|weekly|monthly` (`main.go:22`). Empty → error `"--period flag is required (daily, weekly, monthly)"` and exit 1 (`main.go:32-33`).
- Opens `sql.Open("postgres", STATS_POSTGRES_CONNECTION_STRING)` + `db.Ping()`; a ping failure aborts (`main.go:46-53`).
- Builds `TelegramNotifier` (10s http client, `main.go:56`), `dailystats.NewRepository(db)` as the `StatsReader`, `SystemDateTimeProvider`, and `Reporter`.
- Target chat is `chatID = "@" + NOTIFICATION_TELEGRAM_BROADCAST_CHANNEL_NAME` (`main.go:63`) — same channel-handle convention as queue-monitor, not the feedback chat id.
- `reporter.SendReport(ctx, period, chatID)` (`main.go:65`), then exits. No loop, no signal handling (it's a Job).

## Report generation (`reporter.go`)

`Reporter.SendReport` (`reporter.go:33`) switches on `period`; unknown → error `"invalid report period: %q, expected: daily, weekly, monthly"`. All windows are computed in **UTC**.

| Period | Window (`reporter.go`) | Query call |
|--------|------------------------|-----------|
| daily | today 00:00 → today 00:00 (single day; `start==end`) (`:46-58`) | `GetByDateRange(queueID, start, start)` |
| weekly | Monday of current week 00:00 → today 00:00 (Sunday treated as day 7) (`:60-81`) | `GetByDateRange(queueID, monday, today)` |
| monthly | 1st of current month 00:00 → today 00:00 (`:83-98`) | `GetByDateRange(queueID, first, today)` |

`queueID`/`queueName` come from `STATS_QUEUE_ID` (default 24) / `STATS_QUEUE_NAME` (default `Odbiór karty pobytu`) (`config.go:12-13`). These are independent of queue-monitor's `STATUS_MONITORED_QUEUE_ID` — keep them aligned (both default 24) or reports and monitored queue diverge.

## Message formatting (`notification.go`)

Interfaces `StatsReader.GetByDateRange(...) []dailystats.QueueDailyStat` (`notification.go:12`) and `MessageSender.SendMessage` (`notification.go:16`). Builders (all Polish HTML, empty stats → a "Brak danych" variant):
- `buildDailyMsg` (`notification.go:41`): uses `stats[0].TotalTicketsAvailable` (wydanych) and `stats[0].TakenTickets` (pobranych). Only the first row.
- `buildWeeklyMsg` (`notification.go:48`): per-day lines with Polish weekday abbrev (`polishWeekdays` map, `notification.go:31`) and `DD.MM` date, then summed totals. Uses `int32` accumulators.
- `buildMonthlyMsg` (`notification.go:71`): sums all rows into a single total line.

Field meaning is fixed by the writer in queue-monitor (`monitor.go:79`): `total_tickets_available` = DUW `max_tickets` (day capacity), `taken_tickets` = DUW `registered_tickets`.

## Config (`config.go`)

| Env var | Default | Required |
|---------|---------|----------|
| `STATS_POSTGRES_CONNECTION_STRING` | — | yes |
| `STATS_QUEUE_ID` | 24 | |
| `STATS_QUEUE_NAME` | Odbiór karty pobytu | |
| `NOTIFICATION_TELEGRAM_BROADCAST_CHANNEL_NAME` | — | yes |
| `NOTIFICATION_TELEGRAM_BOT_TOKEN` | — | yes |
| `LOG_LEVEL` | info | |

## Deployment (see `06-infra.md`)

Three CronJobs in `infra/k8s/base/queue-stats-reports-cronjob.yml`, all `timeZone: Europe/Warsaw`, `concurrencyPolicy: Forbid`, `backoffLimit: 2`, `suspend: false`:
- daily `0 18 * * 1-5` → `--period=daily`
- weekly `5 18 * * 5` → `--period=weekly`
- monthly `10 18 28-31 * *` with a shell guard that runs only if tomorrow is the 1st (`[ $(date -d @$(($(date +%s) + 86400)) +%d) = '01' ]`) — k8s cron has no "last day of month". BusyBox `date` needs epoch arithmetic (no `date -d tomorrow`).

The Dockerfile uses `ENTRYPOINT ["./queuestatsreports"]` so CronJob `args: ["--period=..."]` append cleanly; the monthly job overrides with `command: ["/bin/sh","-c"]` for the guard.

## Tests

`reporter_test.go` (window math + message building with a fake time provider and stub reader).
