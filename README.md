# duw-queue-monitor

[![Build and Test](https://github.com/uladzk/duw-queue-monitor/actions/workflows/pull_request.yaml/badge.svg?branch=main)](https://github.com/uladzk/duw-queue-monitor/actions/workflows/pull_request.yaml)
[![Lint](https://github.com/uladzk/duw-queue-monitor/actions/workflows/lint.yml/badge.svg?branch=main)](https://github.com/uladzk/duw-queue-monitor/actions/workflows/lint.yml)
[![Go Version](https://img.shields.io/github/go-mod/go-version/uladzk/duw-queue-monitor)](go.mod)
[![License](https://img.shields.io/github/license/uladzk/duw-queue-monitor)](LICENSE)
[![queue-monitor prd version](https://img.shields.io/badge/dynamic/yaml?url=https%3A%2F%2Fraw.githubusercontent.com%2Fuladzk%2Fduw-queue-monitor%2Fmain%2Finfra%2Fk8s%2Foverlays%2Fovh-prd%2Fkustomization.yaml&query=%24.images%5B0%5D.newTag&label=queue-monitor%20(prd)&color=blue)](infra/k8s/overlays/ovh-prd/kustomization.yaml)

Near real-time Telegram notifications about the appointment-queue status at Dolnośląski Urząd Wojewódzki (DUW, the Lower Silesian Voivodeship Office in Wrocław, Poland).

## Services

Three Go services in one module, deployed to Kubernetes:

| Service | Kind | Purpose |
|---------|------|---------|
| `queue-monitor` | Deployment | Polls the DUW API, detects queue state transitions, posts alerts to the Telegram channel |
| `telegram-bot` | Deployment | Handles user commands and feedback |
| `queue-stats-reports` | CronJobs ×3 | Sends daily/weekly/monthly queue statistics reports |

State: Redis (monitor state), PostgreSQL via CloudNativePG (queue statistics).

## Documentation

- [Architecture overview](docs/architecture.md) — system, runtime flow, state machine, CI/CD diagrams
- [Code map](docs/agents/map/_map.md) — exhaustive, commit-pinned reference with source anchors
- `infra/` — Kustomize manifests (`infra/k8s/`) and Terraform (`infra/terraform/`)

## Development

```bash
# Build
go build -v ./...

# Test (integration tests require Docker)
go test -v ./...

# Lint
golangci-lint run

# Build a service image
docker build -f cmd/queuemonitor/Dockerfile .
```

CI runs build, tests, and `golangci-lint` on every PR. Images are published to `ghcr.io/uladzk/duw-queue-monitor/{service}` via the manual publish workflow; deployments are manual (`kubectl apply -k infra/k8s/overlays/<cloud>-<env>/`).

## License

[MIT](LICENSE)
