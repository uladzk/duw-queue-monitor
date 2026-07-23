# CLAUDE.md

## Start here

`docs/agents/map/_map.md` — commit-pinned code map; read it before touching code and trust it over any other doc. Project skills live in `.claude/skills/`; `duw-git-workflow` is **required reading before any git operation** (others: `duw-develop-and-publish`, `duw-deploy`, `duw-release`, `duw-ovh-billing-alert-response`).

## Project

DUW Queue Monitor — real-time Telegram notifications for the appointment queue at Dolnośląski Urząd Wojewódzki (Wrocław, Poland). Three Go services in one module (`queue-monitor`, `telegram-bot`, `queue-stats-reports`) + Redis (monitor state) + PostgreSQL/CNPG (stats). Production: OVH MKS `mks-duw-prd-waw`.

## House rules

- Lint gate: golangci-lint v2 with the repo `.golangci.yml`; gofmt is enforced.
- Integration tests need Docker (testcontainers); they run as part of `go test ./...`.
- Tests follow the existing structure and AAA comments — mirror neighboring tests.
- Deployments are manual (`kubectl apply -k`); merging is never deploying. Never publish or deploy unless explicitly asked.
- No standing dev cluster — provision on demand, destroy after.
- `infra/terraform/azure/*` and the azure overlays are inactive but preserved intentionally — do not delete or "clean up".
