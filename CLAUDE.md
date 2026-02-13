# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

DUW Queue Monitor is a notification system that monitors queue status at Dolnośląski Urząd Wojewódzki (DUW) and sends real-time notifications via Telegram when queues become available. The system consists of three services written in Go:

- **queue-monitor**: Periodically checks queue status from the DUW API and sends notifications when status changes
- **telegram-bot**: Telegram bot interface for user interactions and feedback
- **queue-stats-reports**: CLI tool that sends daily/weekly/monthly queue statistics reports to Telegram (deployed as K8s CronJobs)

## Development Commands

### Build and Test

```bash
# Build all packages
go build -v ./...

# Run all tests
go test -v ./...

# Run tests for specific package
go test -v ./internal/queuemonitor

# Build Docker image for a service
docker build -t queue-monitor-test:latest -f cmd/queuemonitor/Dockerfile .
docker build -t telegram-bot-test:latest -f cmd/telegrambot/Dockerfile .
docker build -t queue-stats-reports-test:latest -f cmd/queuestatsreports/Dockerfile .

# Run Docker container locally (requires environment variables)
docker run --rm queue-monitor-test:latest
```

### Git Workflow

Use conventional commit format: `{type}({scope}): {description}`

Common types:
- `build`: Build system or Docker changes
- `feat`: New feature
- `fix`: Bug fix
- `refactor`: Code refactoring
- `test`: Adding or modifying tests

Example: `build(queue-monitor): add ca certs to image`

## Architecture

### Core Components

**Queue Monitor Service** (`cmd/queuemonitor/main.go`):
- `Runner` (`internal/queuemonitor/runner.go`): Main loop that orchestrates periodic status checks
- `StatusCollector` (`internal/queuemonitor/statuscollector.go`): Fetches queue status from DUW API with retry logic
- `QueueMonitor` (`internal/queuemonitor/monitor.go`): State machine that detects queue status changes and triggers notifications
- `WeekdayQueueMonitor` (`internal/queuemonitor/monitorweekday.go`): Wrapper that only runs monitoring on weekdays
- `MonitorStateRepository` (`internal/queuemonitor/monitorstate.go`): Persists monitor state to Redis for resilience across restarts

**Telegram Bot Service** (`cmd/telegrambot/main.go`):
- `HandlerRegistry` (`internal/telegrambot/registry.go`): Manages Telegram message handlers
- `Profile` (`internal/telegrambot/profile.go`): Configures bot commands and description
- Handlers in `internal/telegrambot/handlers/`: Individual command handlers (feedback, start, etc.)

**Queue Stats Reports Service** (`cmd/queuestatsreports/main.go`):
- `Reporter` (`internal/statsreporting/reporter.go`): Generates formatted daily/weekly/monthly reports
- `StatsReader` (`internal/dailystats/`): Reads queue statistics from PostgreSQL
- Invoked with `--period=daily|weekly|monthly` flag
- Deployed as three K8s CronJobs (daily, weekly, monthly)

**Shared Components**:
- `Logger` (`internal/logger/`): Structured logging with configurable levels
- `TelegramNotifier` (`internal/notifications/telegram.go`): Sends Telegram messages, used by all services

### Key Architectural Patterns

1. **State Machine Pattern**: The queue monitor maintains state (`MonitorState`) to detect transitions (queue enabled→disabled, tickets available→exhausted) and only notifies on actual changes, not repeated states.

2. **Dependency Injection**: Services are built through constructor functions that inject dependencies (logger, HTTP client, Redis client), making them testable.

3. **Graceful Shutdown**: Both services use `context.Context` with signal handling to save state and shutdown cleanly.

4. **Retry Logic**: The `StatusCollector` uses `github.com/avast/retry-go/v4` to handle transient API failures.

5. **Environment-based Configuration**: All configuration is loaded from environment variables using `github.com/caarlos0/env/v11`.

### Data Flow (Queue Monitor)

1. `Runner.Run()` starts periodic ticker loop
2. Each tick calls `QueueMonitor.CheckAndProcessStatus()`
3. `StatusCollector.GetQueueStatus()` fetches current queue state from DUW API
4. `QueueMonitor` compares new state with previous state
5. On state change, `Notifier.Notify()` sends Telegram message
6. State is persisted to Redis on each check and during shutdown

### State Persistence

The queue monitor stores its state in Redis (`MonitorState` struct):
- `QueueActive`: Whether queue is currently active
- `QueueEnabled`: Whether queue bookings are enabled
- `LastTicketProcessed`: Last ticket number processed
- `TicketsLeft`: Number of tickets remaining

This allows the monitor to resume from where it left off after restarts and avoid duplicate notifications.

## Infrastructure

### Deployment Structure

- `infra/k8s/`: Kubernetes deployment manifests
  - `queue-monitor-deployment.yml`: Production deployment
  - `queue-monitor-deployment-dev.yml`: Development deployment
  - `telegram-bot-deployment.yml`: Bot deployment (same for dev/prd)
  - `queue-stats-reports-cronjob.yml`: CronJob definitions (daily/weekly/monthly)
  - `queue-stats-reports-external-secret.yml`: Telegram secrets for reports service
  - External secrets for sensitive configuration

- `infra/terraform/`: Terraform infrastructure as code
  - `aks/`: AKS cluster configuration
  - `k8s/`: Kubernetes resources (namespaces, RBAC)
  - `platform-shared/`: Shared resources (ACR, Key Vault)

### Kubernetes Environments

- **Development**: `aks-duw-dev-plc`
- **Production**: `aks-duw-prd-plc`

Use `kubectx` to switch between contexts.

### Container Registry

Images are stored in Azure Container Registry:
- Registry: `acrduwshared.azurecr.io`
- Image format: `acrduwshared.azurecr.io/{service}:{version}`

### CI/CD

**Pull Request Workflow** (`.github/workflows/pull_request.yaml`):
- Runs on PRs to main
- Executes: `go build -v ./...` and `go test -v ./...`

**Publish Workflow** (`.github/workflows/publish.yml`):
- Manually triggered via workflow_dispatch
- Inputs: service name (queue-monitor/telegram-bot/queue-stats-reports/duw-migrations), semantic version
- Builds and pushes Docker image to ACR
- Creates git tag: `{service}-{version}`

### Queue Stats Reports Operations

```bash
# Manual trigger for testing
kubectl create job --from=cronjob/queue-stats-reports-daily test-daily-run
kubectl create job --from=cronjob/queue-stats-reports-weekly test-weekly-run

# Unsuspend weekly/monthly reports when ready
kubectl patch cronjob queue-stats-reports-weekly -p '{"spec":{"suspend":false}}'
kubectl patch cronjob queue-stats-reports-monthly -p '{"spec":{"suspend":false}}'

# Check CronJob status
kubectl get cronjobs | grep queue-stats-reports
```

## Testing

### Test Organization

Tests are located alongside source files with `_test.go` suffix:
- Unit tests: Mock dependencies and test individual components
- Integration tests: Use `testcontainers-go` for Redis integration tests (e.g., `monitorstate_integration_test.go`)

### Running Tests

```bash
# All tests
go test -v ./...

# Specific package
go test -v ./internal/queuemonitor

# Specific test
go test -v ./internal/queuemonitor -run TestQueueMonitor_CheckAndProcessStatus
