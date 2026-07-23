---
component: duw-queue-monitor
repo: uladzk/duw-queue-monitor
commit: eb3b6bcfad73ce4a23ec989f96f937a46616fa6d
verified: 2026-07-23
confidence: high
audience: coding-agents, solo-developer
---

# System overview (L1)

## What it is

A three-service Go system that notifies a Telegram channel when the DUW residence-card queue becomes available, and reports usage statistics. All three binaries live in one Go module (`github.com/uladzk/duw-queue-monitor`) and are built from `cmd/<service>/main.go`. Configuration is entirely environment-variable-driven (`github.com/caarlos0/env/v11`).

| Service | Kind | Entry point | Role |
|---------|------|-------------|------|
| queue-monitor | long-running Deployment | `cmd/queuemonitor/main.go` | Polls DUW API, runs the state machine, posts availability alerts, writes daily stats. |
| telegram-bot | long-running Deployment | `cmd/telegrambot/main.go` | Interactive bot: `/feedback` command, forwards user feedback to an admin chat. |
| queue-stats-reports | CronJob (run-to-completion CLI) | `cmd/queuestatsreports/main.go` | `--period=daily\|weekly\|monthly`; reads Postgres, posts a summary, exits. |

Data stores: **Redis** (queue-monitor state, key `monitor:state`) and **PostgreSQL** via CloudNativePG (table `queue_daily_stats`, written by queue-monitor, read by queue-stats-reports).

## Context diagram

```mermaid
flowchart LR
  DUW["DUW API\nrezerwacje.duw.pl/status_kolejek/query.php"]
  subgraph cluster["OVH MKS cluster (mks-duw-prd-waw)"]
    QM["queue-monitor\n(Deployment, Recreate)"]
    TB["telegram-bot\n(Deployment, Recreate)"]
    QSR["queue-stats-reports\n(3 CronJobs)"]
    REDIS["Redis\n(redis-service:6379)"]
    PG["PostgreSQL (CNPG)\ncluster 'postgres', db duw_stats\npostgres-rw:5432"]
  end
  TG["Telegram Bot API\napi.telegram.org"]
  USER["Channel subscribers /\nfeedback users"]

  QM -->|GET status, UA:''| DUW
  QM <-->|monitor:state JSON| REDIS
  QM -->|UpsertDailyStats| PG
  QM -->|sendMessage @channel| TG
  QSR -->|GetStatsByDateRange| PG
  QSR -->|sendMessage @channel| TG
  TB <-->|long-poll getUpdates| TG
  TB -->|forward feedback to chat_id| TG
  TG --> USER
```

Secrets (bot token, channel name, feedback chat id, Postgres credentials) come from Infisical via External Secrets Operator; see `06-infra.md`.

## One request lifecycle: queue-monitor tick

The dominant flow. `Runner.Run` loops on a ticker (`STATUS_CHECK_INTERVAL_SECONDS`, prd=5s) and calls `CheckAndProcessStatus` each tick after an immediate first check (`runner.go:41`).

```mermaid
sequenceDiagram
  participant T as Ticker (runner.go:38)
  participant W as WeekdayQueueMonitor
  participant M as DefaultQueueMonitor
  participant C as StatusCollector
  participant D as DUW API
  participant S as QueueState (state machine)
  participant N as TelegramNotifier
  participant R as Redis
  participant P as Postgres

  T->>W: CheckAndProcessStatus(ctx)
  alt weekend or outside working hours (UTC)
    W-->>T: return nil (skip)  %% monitorweekday.go:44-47
  else within window
    W->>M: CheckAndProcessStatus(ctx)
    M->>C: GetQueueStatus(ctx)
    C->>D: GET ...query.php?status=  (User-Agent: "")
    D-->>C: {"result": {"Wrocław":[Queue,...]}}
    C-->>M: *Queue (id==24), TicketsLeft clamped >=0
    M->>S: state.Handle(ctx, queue)
    alt transition or ticket-count change
      S->>N: SendMessage("@"+channel, msg)
      N->>N: POST /bot{token}/sendMessage (retry x5)
    end
    S-->>M: newState
    opt newState == Inactive && stats enabled
      M->>P: SaveDailyStats(maxTickets, registeredTickets)
    end
  end
  Note over T,R: On SIGTERM: Runner.saveMonitorState -> Redis SET monitor:state (TTL 60s)
```

Key invariants of this flow:
- The **clock gate runs before the API call** — no DUW request outside the working window (`monitorweekday.go`).
- The state machine emits **at most one notification per tick**, and only on a real change (`state*.go`).
- State is written to Redis **on shutdown only** (`runner.go:53-59`); it is read once at startup (`runner.go:87-105`). It is not written every tick — the in-memory state is authoritative during a run.
- Daily-stats write is a **side effect of the → Inactive transition**, i.e. once per day at end-of-window (`monitor.go:65-67`).

## Where to go next

- Poll loop / state machine / persistence internals → `02-queue-monitor.md`
- Bot handlers → `03-telegram-bot.md`
- Reports & CLI → `04-stats-reports.md`
- Postgres/sqlc, migrations, Redis schema, logger, notifier → `05-data-and-shared.md`
- k8s/terraform/deploy → `06-infra.md`
- CI, Docker, versioning, local run → `07-ci-cd.md`
- Making a change → `08-change-guide.md`
- Symptoms & failure modes → `09-reliability.md`
