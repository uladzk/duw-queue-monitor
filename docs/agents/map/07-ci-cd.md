---
component: duw-queue-monitor
repo: uladzk/duw-queue-monitor
commit: eb3b6bcfad73ce4a23ec989f96f937a46616fa6d
verified: 2026-07-23
confidence: high
audience: coding-agents, solo-developer
---

# CI/CD, Docker & local run (L2)

## Workflows (`.github/workflows`)

| Workflow | Trigger | Does |
|----------|---------|------|
| `pull_request.yaml` | `pull_request` + `push` to `main` | `go build -v ./...` then `go test -v ./...` on Go 1.25.3. |
| `lint.yml` | `pull_request` + `push` to `main` | `golangci-lint-action@v8`, golangci-lint `v2.12.2`. |
| `publish.yml` | `workflow_dispatch` | Build + push one service image, create git tag. |
| `release.yml` | `workflow_dispatch` | Same build/push, plus AI-generated release notes + Discord notify. |

CI is build+test+lint only — no coverage gate, no image build on PR.

### publish.yml (the one agents use to ship)

Inputs: `service` (choice: `queue-monitor`, `telegram-bot`, `queue-stats-reports`, `duw-migrations`) and `version` (semver string).

**Registry is chosen by repo variable `PUBLISH_TO_GHCR`** (`publish.yml:29,38,45,...`):
- default / any value ≠ `'false'` → **GHCR** `ghcr.io/uladzk/duw-queue-monitor/<service>` (login via `GITHUB_TOKEN`; adds `org.opencontainers.image.source` label — required so `GITHUB_TOKEN` may push to a pre-existing user-owned package, project memory).
- `PUBLISH_TO_GHCR == 'false'` → **ACR** `acrduwshared.azurecr.io/<service>` (Azure OIDC login). Inactive in practice — Azure runtime is gone.

Steps: verify the target tag does not already exist (fails if it does, `publish.yml:53-77`), compute build config, build+push `:{version}` and `:latest`, then create git ref `refs/tags/<service>-<version>` (`publish.yml:118-128`).

**Build-config mapping** (`publish.yml:78-89`):
- `duw-migrations` → context `./db`, dockerfile `./db/Dockerfile`.
- everything else → context `.`, dockerfile `./cmd/<service-with-hyphens-stripped>/Dockerfile` (`queue-monitor`→`queuemonitor`, etc.).

So git tag format is **`<service>-<version>`** (e.g. `queue-monitor-1.5.1`). Note when creating a GitHub release for an existing tag: omit `--target` or it 422s (project memory).

### release.yml

Superset of publish: same build/push job, then `generate-release-notes` (classifies PUBLIC vs INTERNAL changes since the last `<service>-*` tag using `actions/ai-inference@v1` with `anthropic/claude-haiku-4.5`), an optional `create-release-notes-pr` into a separate notes repo, and a `notify-discord` step. Needs `packages: write` + `models: read`.

## Dockerfiles (`cmd/*/Dockerfile`, `db/Dockerfile`)

All Go images are 2-stage: build on `golang:1.25.3-alpine3.22`, final on `alpine:3.22`. No exposed ports (queue-monitor & bot are outbound-only; stats-reports is a CLI).

| Image | Build flags | Entrypoint | Distinctive |
|-------|-------------|-----------|-------------|
| `cmd/queuemonitor/Dockerfile` | `CGO_ENABLED=0 -trimpath -ldflags=-s` | `CMD ["./queuemonitor"]` | **Installs the Certum OV CA cert** (`ovcasha2.cer` → `/usr/local/share/ca-certificates`, `update-ca-certificates`, copied into final stage) — needed because DUW's TLS chain fails to validate against stock roots (`Dockerfile:13-19`). If TLS to DUW breaks after a base-image bump, look here. |
| `cmd/telegrambot/Dockerfile` | `go build -o` (no trim/CGO flags) | `CMD ["./telegrambot"]` | plain |
| `cmd/queuestatsreports/Dockerfile` | `CGO_ENABLED=0 -trimpath -ldflags=-s` | `ENTRYPOINT ["./queuestatsreports"]` | ENTRYPOINT (not CMD) so CronJob `args` append the `--period` flag |
| `db/Dockerfile` | — | goose-docker base | `FROM ghcr.io/kukymbr/goose-docker:3.26.0`, `COPY migrations/ /migrations/` |

## golangci config (`.golangci.yml`)

`default: none`, then enable `govet`, `staticcheck`, `unused`, `ineffassign`, `misspell`, `gocyclo` (min-complexity 15). Formatter `gofmt`. A function over cyclomatic-complexity 15 will fail lint — relevant when adding state/branches to the monitor.

## Local run

```bash
# stores + migrations
docker compose -f cmd/queuemonitor/docker-compose.dev.yml up -d
# build/test everything
go build -v ./...
go test -v ./...
# run one service (needs env: STATE_REDIS_CONNECTION_STRING, NOTIFICATION_TELEGRAM_*, etc.)
go run ./cmd/queuemonitor
go run ./cmd/queuestatsreports --period=daily
```

`docker-compose.dev.yml` (`cmd/queuemonitor/`) starts `redis:8.0.3-alpine` (:6379), `postgres:17-alpine` (db `duw_stats`, user `duw_stats_admin`/`localdev`, :5432, with a `pg_isready` healthcheck), and a `migrations` service that builds `../../db` and runs goose `up` (`GOOSE_DBSTRING=postgres://duw_stats_admin:localdev@postgres:5432/duw_stats?sslmode=disable`) once Postgres is healthy. The services themselves are not in the compose file — run them from `go run` / built binaries pointed at `localhost`.
