# duw-queue-monitor

A notification system that monitors queue availability at Dolnośląski Urząd Wojewódzki (DUW) and delivers near real-time alerts via Telegram. When appointment slots open up, subscribers receive an instant message in the broadcast channel.

## Table of Contents

- [System Overview](#system-overview)
- [Services](#services)
  - [queue-monitor](#queue-monitor)
  - [telegram-bot](#telegram-bot)
  - [queue-stats-reports](#queue-stats-reports)
- [Architecture](#architecture)
  - [State Machine](#state-machine)
  - [Data Flow](#data-flow)
  - [Key Design Patterns](#key-design-patterns)
- [DUW API](#duw-api)
- [Database Schema](#database-schema)
- [Configuration](#configuration)
- [Infrastructure](#infrastructure)
- [Development](#development)

---

## System Overview

```
┌─────────────────────────────────────────────────────────┐
│                DUW API (rezerwacje.duw.pl)               │
└──────────────────────────┬──────────────────────────────┘
                           │ polls every 5s
                           ▼
┌─────────────────────────────────────────────────────────┐
│                    queue-monitor                         │
│  StatusCollector → State Machine → TelegramNotifier     │
│       ↕ Redis (state)    ↕ PostgreSQL (daily stats)     │
└──────────────────────────┬──────────────────────────────┘
                           │
                           ▼
┌─────────────────────────────────────────────────────────┐
│                  Telegram Bot API                        │
├──────────────────┬──────────────────────────────────────┤
│  Broadcast       │  telegram-bot (handles user          │
│  Channel         │  interactions & /feedback command)   │
│  (notifications) │                                      │
└──────────────────┴──────────────────────────────────────┘
                           ▲
                           │ scheduled CronJob
┌─────────────────────────────────────────────────────────┐
│             queue-stats-reports (daily/weekly/monthly)   │
│             reads PostgreSQL → sends HTML report         │
└─────────────────────────────────────────────────────────┘
```

---

## Services

### queue-monitor

**Path:** `cmd/queuemonitor/`

The core service. It polls the DUW queue API on a fixed interval, runs a state machine to detect meaningful changes, and sends Telegram notifications when the queue status changes. On weekends and outside DUW working hours (06:00–18:00 UTC on weekdays) monitoring is suspended to prevent false alerts.

State is persisted to Redis so the service can resume correctly after a restart without sending duplicate notifications.

Optionally (controlled by the `FF_DAILY_STATS_ENABLED` flag) it saves per-day ticket statistics to PostgreSQL when the queue becomes inactive at end-of-day.

### telegram-bot

**Path:** `cmd/telegrambot/`

A long-running Telegram bot that handles user interactions. Currently exposes one command:

| Command | Behaviour |
|---------|-----------|
| `/feedback` | Prompts the user for free-text feedback, then forwards it to an admin chat |

All other messages trigger the default handler which displays the available command menu.

### queue-stats-reports

**Path:** `cmd/queuestatsreports/`

A CLI tool invoked with `--period=daily|weekly|monthly`. It queries the PostgreSQL statistics table and sends a formatted HTML report to the Telegram broadcast channel. Deployed as three Kubernetes CronJobs (daily at 18:00 UTC on weekdays, weekly, and monthly).

---

## Architecture

### State Machine

`DefaultQueueMonitor` is a state machine with four states:

```
         ┌──────────────────────────────────────┐
         │           UninitializedState          │
         │  (starting state, no prior state)     │
         └───────────────┬──────────────────────┘
                         │
          ┌──────────────┼──────────────┐
          ▼              ▼              ▼
   ┌─────────────┐ ┌───────────────┐ ┌────────────────────┐
   │InactiveState│ │ActiveDisabled │ │  ActiveEnabledState │
   │             │ │    State      │ │                     │
   │queue.Active │ │queue.Active=✓ │ │queue.Active=✓       │
   │  = false    │ │queue.Enabled  │ │queue.Enabled=✓      │
   │             │ │  = false      │ │TicketsLeft > 0      │
   └─────────────┘ └───────────────┘ └────────────────────┘
```

Each state implements the `QueueState` interface. Calling `Handle(ctx, queue)` on a state receives the latest queue snapshot and returns the next state (which may be the same state or a different one). Notifications are fired inside the state transition logic only when a meaningful change is detected, keeping the system quiet during repeated identical status responses.

State is serialised as JSON and stored in Redis with a TTL so that restarts pick up where the monitor left off.

### Data Flow

1. `Runner.Run()` starts a ticker loop at the configured interval.
2. Each tick calls `DefaultQueueMonitor.CheckAndProcessStatus()`.
3. `StatusCollector.GetQueueStatus()` fetches the latest queue snapshot from the DUW API, retrying up to `STATUS_CHECK_MAX_ATTEMPTS` times on failure.
4. The current `QueueState.Handle()` compares the snapshot against the previous state and decides whether to send a Telegram notification.
5. The new state is stored in `h.state`.
6. At shutdown (or end-of-day transition to `InactiveState`) the state is persisted back to Redis / PostgreSQL.

### Key Design Patterns

| Pattern | Where used |
|---------|------------|
| **State** | `QueueState` interface + four concrete state types in `internal/queuemonitor/state*.go` |
| **Repository** | `MonitorStateRepository` (Redis), `DailyStatsRepository` (PostgreSQL) |
| **Handler / Registry** | `HandlerRegistry` + `Handler` interface in `internal/telegrambot/` |
| **Retry** | `avast/retry-go` in `StatusCollector.getStatusWithRetries()` |
| **Dependency Injection** | Constructor functions accept all dependencies; `main.go` wires them up |
| **Feature Flag** | `FF_DAILY_STATS_ENABLED` env var guards the statistics collection path |
| **Graceful Shutdown** | `context.Context` + OS signal handler ensures state is saved before exit |

---

## DUW API

**Endpoint:** `https://rezerwacje.duw.pl/status_kolejek/query.php?status=<queue_id>`

The API returns a JSON object keyed by city name. Each city value is an array of queue objects:

```go
type Response struct {
    Result map[string][]Queue `json:"result"` // city → queues
}

type Queue struct {
    ID                int    `json:"id"`
    Name              string `json:"name"`
    Enabled           bool   `json:"enabled"`            // registrations open
    Active            bool   `json:"active"`              // DUW operating today
    TicketValue       string `json:"ticket_value"`        // ticket currently being served
    TicketsLeft       int    `json:"tickets_left"`        // available registration slots
    RegisteredTickets int    `json:"registered_tickets"`  // slots already taken
    MaxTickets        int    `json:"max_tickets"`         // daily capacity
}
```

**Quirks:**
- The `User-Agent` request header must be set to an empty string; a non-empty value causes the API to return no data.
- `TicketsLeft` can occasionally be negative due to a bug in the DUW API. The `StatusCollector` treats this as an error and retries.

---

## Database Schema

Managed with [Goose](https://github.com/pressly/goose) migrations (`db/migrations/`).

```sql
CREATE TABLE queue_daily_stats (
    id                     BIGSERIAL PRIMARY KEY,
    queue_id               INTEGER       NOT NULL,
    queue_name             VARCHAR(255)  NOT NULL,
    date                   DATE          NOT NULL,
    total_tickets_available INTEGER,      -- total capacity issued for the day
    taken_tickets          INTEGER,       -- tickets registered by users
    created_at             TIMESTAMPTZ   DEFAULT NOW(),
    updated_at             TIMESTAMPTZ   DEFAULT NOW(),
    UNIQUE(queue_id, date)
);
```

A row is upserted once per queue per day when the queue transitions to `InactiveState` (end of day).

---

## Configuration

All configuration is loaded from environment variables via [`caarlos0/env`](https://github.com/caarlos0/env).

### queue-monitor

| Variable | Description | Example |
|----------|-------------|---------|
| `STATUS_API_URL` | DUW API endpoint | `https://rezerwacje.duw.pl/status_kolejek/query.php?status=` |
| `STATUS_MONITORED_QUEUE_ID` | Numeric queue ID to watch | `24` |
| `STATUS_MONITORED_QUEUE_CITY` | City name key in API response | `Wrocław` |
| `STATUS_CHECK_INTERVAL_SECONDS` | Polling interval | `5` |
| `STATUS_CHECK_TIMEOUT_MS` | Per-attempt HTTP timeout | `4000` |
| `STATUS_CHECK_MAX_ATTEMPTS` | Retry count on API failure | `3` |
| `STATUS_CHECK_ATTEMPT_DELAY_MS` | Delay between retries | `500` |
| `MONITOR_HTTP_CLIENT_TIMEOUT_SECONDS` | HTTP client timeout | `5` |
| `STATE_REDIS_CONNECTION_STRING` | Redis URL for state storage | `redis://redis:6379` |
| `STATE_TTL_SECONDS` | Redis key TTL | `60` |
| `FF_DAILY_STATS_ENABLED` | Enable stats persistence | `true` |
| `STATS_POSTGRES_CONNECTION_STRING` | PostgreSQL connection string | `postgres://...` |
| `NOTIFICATION_TELEGRAM_BOT_TOKEN` | Telegram bot token | `123:ABC...` |
| `NOTIFICATION_TELEGRAM_BROADCAST_CHANNEL_NAME` | Channel to send notifications to | `@my_channel` |
| `LOG_LEVEL` | Structured log level | `info` |

### telegram-bot

| Variable | Description |
|----------|-------------|
| `NOTIFICATION_TELEGRAM_BOT_TOKEN` | Telegram bot token |
| `NOTIFICATION_TELEGRAM_FEEDBACK_CHAT_ID` | Admin chat ID for forwarded feedback |

### queue-stats-reports

| Variable | Description |
|----------|-------------|
| `STATS_POSTGRES_CONNECTION_STRING` | PostgreSQL connection string |
| `STATS_QUEUE_ID` | Queue ID for report filtering |
| `STATS_QUEUE_NAME` | Display name used in report |
| `NOTIFICATION_TELEGRAM_BOT_TOKEN` | Telegram bot token |
| `NOTIFICATION_TELEGRAM_BROADCAST_CHANNEL_NAME` | Channel to send the report to |

---

## Infrastructure

### Kubernetes (`infra/k8s/`)

| Manifest | Type | Description |
|----------|------|-------------|
| `queue-monitor-deployment.yml` | Deployment | Production queue monitor (1 replica, Recreate strategy) |
| `queue-monitor-deployment-dev.yml` | Deployment | Development queue monitor |
| `telegram-bot-deployment.yml` | Deployment | Telegram bot (shared dev/prd) |
| `queue-stats-reports-cronjob.yml` | CronJob ×3 | Daily (18:00 UTC Mon–Fri), weekly, monthly reports |
| `queue-stats-reports-external-secret.yml` | ExternalSecret | Injects Telegram credentials for reports |

Secrets are injected at runtime via the **ExternalSecrets** operator backed by Infisical. No secrets are stored in the manifests.

Two AKS environments are used:
- **Development:** `aks-duw-dev-plc`
- **Production:** `aks-duw-prd-plc`

Switch between them with `kubectx`.

### Terraform (`infra/terraform/`)

| Module | Purpose |
|--------|---------|
| `aks/` | Azure Kubernetes Service cluster provisioning |
| `k8s/` | Kubernetes namespaces, RBAC, and resource configuration |
| `platform-shared/` | Shared resources: Azure Container Registry (`acrduwshared.azurecr.io`), Key Vault |

Docker images are stored in ACR with the format `acrduwshared.azurecr.io/{service}:{version}`.

### CI/CD (`.github/workflows/`)

| Workflow | Trigger | Steps |
|----------|---------|-------|
| `pull_request.yaml` | PR to `main` | `go build -v ./...`, `go test -v ./...` |
| `publish.yml` | Manual (`workflow_dispatch`) | Builds & pushes Docker image to ACR, creates git tag `{service}-{version}` |

---

## Development

### Prerequisites

- Go 1.23+
- Docker (for building images and integration tests)

### Build & Test

```bash
# Build all packages
go build -v ./...

# Run all tests (unit + integration)
go test -v ./...

# Run tests for a specific package
go test -v ./internal/queuemonitor

# Run a specific test
go test -v ./internal/queuemonitor -run TestQueueMonitor_CheckAndProcessStatus
```

### Docker

```bash
# Build service images
docker build -t queue-monitor:latest          -f cmd/queuemonitor/Dockerfile .
docker build -t telegram-bot:latest           -f cmd/telegrambot/Dockerfile .
docker build -t queue-stats-reports:latest    -f cmd/queuestatsreports/Dockerfile .
```

### Database Migrations

```bash
# Apply migrations (requires GOOSE_DBSTRING to be set)
goose -dir db/migrations postgres "$GOOSE_DBSTRING" up
```

### Useful kubectl Commands

```bash
# Manually trigger a stats report job
kubectl create job --from=cronjob/queue-stats-reports-daily  test-daily-run
kubectl create job --from=cronjob/queue-stats-reports-weekly test-weekly-run

# Unsuspend a CronJob
kubectl patch cronjob queue-stats-reports-weekly -p '{"spec":{"suspend":false}}'

# Check CronJob status
kubectl get cronjobs | grep queue-stats-reports
```
