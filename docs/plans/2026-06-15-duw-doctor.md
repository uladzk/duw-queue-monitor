# DUW Doctor Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use **superpowers:executing-plans** to implement this plan task-by-task.
> Use **superpowers:test-driven-development** for every Phase-A *logic* task (RED → run/FAIL → minimal impl → run/PASS → commit). Follow the project skills: **duw-git-workflow** (branches/commits/PRs), **duw-develop-and-publish** (build/test/publish images), **duw-deploy** (deploy to prd). Use **superpowers:verification-before-completion** before any "done" claim — report literal command output.
> **Context/why:** read the companion briefing `project_duw_doctor` in project memory first — it explains the investigation that motivated this and the design rationale. Linked incidents: `reference_negative_tickets_left_incident_2026_06_08` (the freeze this must catch), `reference_duplicate_notifications_incident_2026_06_16` (known issue #2, WONTFIX), `reference_monitor_feedback_investigation_2026_06_15` (the false alarm + channel-read approach).

**Goal:** A self-hosted health watchdog ("duw-doctor") that continuously checks whether queue-monitor's behavior is consistent with the DUW source of truth and, on a sustained real inconsistency, alerts the operator on Telegram — and (Phase B) triggers a cloud Claude deep-dive that classifies the incident. It can never mutate the cluster.

**Architecture:** A cheap deterministic **checker** runs in-cluster as a CronJob (Go, no LLM): it fetches the DUW API (source of truth) and the public `t.me/s` channel (observed end-user truth), compares expected-vs-observed monitor state with debounce+cooldown, and on a sustained mismatch captures a read-only cluster snapshot and sends a Telegram alert. The **doctor** (LLM deep-dive, Phase B) runs in GitHub's cloud via the Claude GitHub Action — never in the cluster, no cluster creds.

**Tech Stack:** Go 1.25 (checker; reuses `internal/queuemonitor` + `internal/notifications` + `internal/logger`), `github.com/PuerkitoBio/goquery` (parse `t.me/s`), `k8s.io/client-go` (read pods/logs/events/deployments + get/update one state ConfigMap), Kustomize (base + `ovh-prd` overlay), External Secrets Operator / Infisical (Telegram secret, reused), Claude GitHub Action `anthropics/claude-code-action` (Phase B).

---

## Revision note — refined 2026-06-16 (all open items resolved against live repo/cluster)

This revision validated every assumption against the code at HEAD and tightened Phase A to execution-ready depth. **What changed vs the original draft:**

1. **Env names now mirror the real Infisical keys** (was placeholder `ALERT_TELEGRAM_*`): the checker uses `NOTIFICATION_TELEGRAM_BOT_TOKEN` + `NOTIFICATION_TELEGRAM_FEEDBACK_CHAT_ID`, the exact keys the telegram-bot already consumes. See *Resolved open items #2*.
2. **Reuse, don't reimplement:** the checker reuses `queuemonitor.StatusCollector` (DUW fetch: empty-User-Agent gotcha + retry + negative-`tickets_left` clamp) and `notifications.TelegramNotifier` (HTML send + retry). See #1.
3. **TLS is solved in the Dockerfile, not Go:** the image must install the Certum OV CA into its trust store (DUW's server has a broken cert chain), exactly like `cmd/queuemonitor/Dockerfile`. The Go `http.Client` is just `{Timeout}`. See #1.
4. **Dropped `STALE_TOLERANCE_SECONDS`:** `DEBOUNCE_K`(=3) × the 3-min cadence already enforces ~9 min of sustained mismatch, so a separate stale timer is redundant *and* would force cross-run tracking of DUW-state-change timestamps. Removing it keeps `Evaluate` a pure function of `(current observation, prev counter)`. (DRY / YAGNI.)
5. **Flood window fixed to be relative to `now`,** not to the newest post — otherwise a flood that happened hours ago would re-trigger an escalation on every run forever.
6. **Component layout corrected** to the repo's actual Kustomize pattern: a `base/duw-doctor/` subdir whose files are referenced individually from `base/kustomization.yaml` (the repo has **no** nested per-component `kustomization.yaml`; `postgres/` and `redis/` follow this exact pattern).
7. **Duplicate-flood detector tested via constructed `[]Post`** (pure unit test), not a synthesized HTML fixture — the parser is already covered by `channel.html`, and run-length logic is independent of parsing.

None of the **locked decisions** changed (hybrid checker+cloud-doctor; two phases; `ANTHROPIC_API_KEY` repo secret for the doctor; alerts to the existing private feedback chat; single read-only checker SA; testing on prd).

---

## Resolved open items (verified at HEAD, 2026-06-16)

### 1. `Queue` struct + HTTP/retry/TLS to reuse (`internal/queuemonitor/statuscollector.go`, `config.go`)

```go
// internal/queuemonitor/statuscollector.go
type Response struct {
    Result map[string][]Queue `json:"result"`   // keyed by city name, e.g. "Wrocław"
}
type Queue struct {
    ID                int    `json:"id"`
    Name              string `json:"name"`
    Enabled           bool   `json:"enabled"`            // accepts new registrations
    Active            bool   `json:"active"`             // operating today
    TicketValue       string `json:"ticket_value"`       // string, not int
    TicketsLeft       int    `json:"tickets_left"`       // negative values clamped to 0 by GetQueueStatus
    RegisteredTickets int    `json:"registered_tickets"`
    MaxTickets        int    `json:"max_tickets"`
}
```

- **Reuse `StatusCollector` verbatim** — `queuemonitor.NewStatusCollector(cfg *QueueMonitorConfig, httpClient *http.Client, log *logger.Logger)` then `GetQueueStatus(ctx) (*Queue, error)`. It already: sets `req.Header.Set("User-Agent", "")` (**critical — DUW returns no data otherwise**), retries via `retry-go` (`Attempts`/`FixedDelay`/`Context+timeout`), checks `StatusCode == 200`, decodes into `Response`, finds the queue by city+id, and **clamps negative `TicketsLeft` to 0**. We construct `QueueMonitorConfig` as a struct literal (not via `env.Parse`), so its `,required` Redis/Postgres tags do not apply.
- **HTTP client** in the monitor is plain `&http.Client{Timeout: ...}` (`cmd/queuemonitor/main.go:76`). **No custom `tls.Config` in Go.**
- **TLS / CA trust lives in `cmd/queuemonitor/Dockerfile`**: it downloads `http://repository.certum.pl/ovcasha2.cer`, converts DER→PEM, runs `update-ca-certificates`, and copies `/etc/ssl/certs/` into the final image. The DUW endpoint fails TLS validation without this. **The checker Dockerfile must replicate this step** (Task A3.1).
- Relevant config defaults (`internal/queuemonitor/config.go`): `STATUS_MONITORED_QUEUE_ID=24`, `STATUS_MONITORED_QUEUE_CITY=Wrocław`, `STATUS_API_URL=https://rezerwacje.duw.pl/status_kolejek/query.php?status=`, `STATUS_CHECK_TIMEOUT_MS=4000`, `STATUS_CHECK_MAX_ATTEMPTS=3`, `STATUS_CHECK_ATTEMPT_DELAY_MS=500`. Window: `WORKING_HOUR_START_UTC=5`, `WORKING_HOUR_END_UTC=18` (default) — **prd pins `=17`** via the ovh-prd patch.

### 2. Telegram feedback secret — Infisical path/keys + ExternalSecret to mirror

- **Existing ExternalSecret to mirror:** `infra/k8s/base/telegram-bot-external-secret.yml` (name `telegram-bot-external-secret`, store `infisical-secret-store`, `refreshInterval: 1h`).
- **Infisical remote keys (reuse — do NOT create new):**
  - `/telegram/NOTIFICATION_TELEGRAM_BOT_TOKEN`  → local key `NOTIFICATION_TELEGRAM_BOT_TOKEN`
  - `/telegram/NOTIFICATION_TELEGRAM_FEEDBACK_CHAT_ID` → local key `NOTIFICATION_TELEGRAM_FEEDBACK_CHAT_ID`
- **SecretStore:** `infisical-secret-store` (ns `default`), provider Infisical, region `eu`, project slug `duw-monitor-secrets-app`, env slug `prd` (templated by Terraform `infra/terraform/ovh/k8s/...`), `secretsPath: /`. Universal-auth creds in K8s secret `infisical-universal-auth-credentials`.
- **Decision (operator, 2026-06-16): reuse the existing Secret directly.** The CronJob references the existing `telegram-bot-external-secret` K8s Secret via `secretKeyRef` (keys `NOTIFICATION_TELEGRAM_BOT_TOKEN` + `NOTIFICATION_TELEGRAM_FEEDBACK_CHAT_ID`). **No new ExternalSecret, no new Infisical secret, no new chat.** Both workloads live in ns `default`, so the dependency is safe.
- The telegram-bot Deployment proves the keys are real (`infra/k8s/base/telegram-bot-deployment.yml`): env `NOTIFICATION_TELEGRAM_BOT_TOKEN` and `NOTIFICATION_TELEGRAM_FEEDBACK_CHAT_ID`, both `valueFrom.secretKeyRef: telegram-bot-external-secret`.

### 3. `t.me/s` parser to port (cheerio → goquery)

- Source: `~/.claude/mcp-servers/duw-mcp-server/src/services/telegram-web.ts`, function `parseChannelHtml` (selectors: `.tgme_widget_message[data-post]`; id = last `/`-segment of `data-post`; text = `.tgme_widget_message_text` with `<br>`→`\n` and ` `→space, trimmed; datetime = `a.tgme_widget_message_date time[datetime]`; sort ascending by id).
- Fixture present at `~/.claude/mcp-servers/duw-mcp-server/test/fixtures/channel.html` (92 KB). **Verified contents:** 20 posts, ids `17963…17982` ascending, timestamps `2026-06-15T08:14:00Z … 16:00:06Z`. Newest post (17982, 16:00:06) is the **📊 daily summary** ("...podsumowanie dnia: Wydanych biletów: 240 Pobranych biletów: 240") — **not** a status post. Most recent *status* post is 17981 (13:38:54Z) "🌙 ... jest **nieaktywna** ..." → `Inactive`. Post 17980 (08:47:09Z) "💤 ... **niedostępna** ..." → `ActiveDisabled`. The other 17 are "🔔 ... **dostępna** ...".
- **Substring trap (must guard):** `"niedostępna"` contains `"dostępna"`. The status classifier MUST test `nieaktywna` → `niedostępna` → `dostępna` in that order.

### 4. Publish workflow service input + ovh-prd `images:` block

- `.github/workflows/publish.yml` is `workflow_dispatch` with input `service` (`type: choice`, `options: [queue-monitor, telegram-bot, queue-stats-reports, duw-migrations]`) + `version`. **Add `duw-doctor-checker` to `options`.**
- Service→Dockerfile mapping strips hyphens: `SERVICE_NAME_TRANSFORMED=$(echo "$service" | sed 's/-//g')` → `./cmd/${TRANSFORMED}/Dockerfile`. So **the cmd dir must be `cmd/duwdoctorchecker/`** → `duw-doctor-checker` resolves to `./cmd/duwdoctorchecker/Dockerfile`. ✓
- Images push to `ghcr.io/uladzk/duw-queue-monitor/${service}:${version}` (and `latest`); git tag `${service}-${version}` (e.g. `duw-doctor-checker-1.0.0`). GHCR push requires the `org.opencontainers.image.source` label — the workflow already adds it.
- ovh-prd overlay (`infra/k8s/overlays/ovh-prd/kustomization.yaml`) `images:` entries use `name:` (the ACR base name) / `newName:` (`ghcr.io/uladzk/duw-queue-monitor/<svc>`) / `newTag:`. **Add:**
  ```yaml
  - name: acrduwshared.azurecr.io/duw-doctor-checker
    newName: ghcr.io/uladzk/duw-queue-monitor/duw-doctor-checker
    newTag: "1.0.0"
  ```

### 5. Kustomize base structure

- `infra/k8s/base/kustomization.yaml` lists every manifest individually in `resources:`. Simple services are flat files (`queue-monitor-deployment.yml`, …); complex ones live in a subdir referenced by relative path (`postgres/postgres-cluster.yml`, `redis/redis-deployment.yml`). **There is no nested `kustomization.yaml`.**
- **Mirror this:** create `infra/k8s/base/duw-doctor/*.yml` and add each to `base/kustomization.yaml` `resources:` as `duw-doctor/<file>.yml` (Task A4.x).

---

## Zero-context primer (read before coding)

- **Target:** queue-monitor watches **one** queue — **id 24, "odbiór karty", city `Wrocław`** — and posts to public Telegram channel **`duw_queue_updates`**. A separate daily-stats CronJob posts a 📊 summary at 16:00 UTC.
- **Monitor true window:** weekdays **05:00–17:00 UTC**. Outside it the monitor is **idle by design** — not a fault (`internal/queuemonitor/monitorweekday.go::isDuwOffTime`: weekend, or `hour < start || hour >= end`).
- **State mapping:** `!active → Inactive`; `active && !enabled → ActiveDisabled`; `active && enabled → ActiveEnabled`. Channel texts: `🔔 ... dostępna` (available), `💤 ... niedostępna` (no tickets), `🌙 ... nieaktywna` (closed), `📊 ... podsumowanie dnia` (daily summary — *not* a status).
- **Logs are event-based** (~100–150 lines/weekday). Steady states produce no logs → "log silence" is NOT a freeze signal; the DUW-vs-Telegram comparison is.
- **Failure class to catch** (2026-06-08): DUW returned `tickets_left=-1`, the state machine froze, posting stopped while DUW changed. **Signature: DUW moved (e.g. `active=false`) but the channel's last status post never reflected it.** (1.5.1 now clamps `-1→0`, but the watchdog guards the general freeze class.)
- **Known issue #2 — duplicate-notification flood** (2026-06-16, **WONTFIX**): a Telegram-API latency window makes the monitor treat delivered-but-timed-out sends as failures and re-post the same notification each ~5s tick (real instance: 22 identical `🔔 ... K110 ... Pozostało biletów: 8` in ~62s). **Signature: a run of ≥N byte-identical consecutive status posts within a short window.** The monitor's *state is correct* during a flood, so the expected-vs-observed check misses it → it needs its own volume check. Classify as the known, accepted bug — detect-and-inform only.
- **Telegram reads need no auth:** `https://t.me/s/duw_queue_updates`. Markup: `div.tgme_widget_message[data-post="duw_queue_updates/<id>"]`; text `.tgme_widget_message_text`; timestamp `a.tgme_widget_message_date time[datetime]`.

---

## Architecture & data flow

```
PHASE A (in-cluster, no LLM):
  CronJob duw-doctor-checker  (every 3 min, weekdays, padded window; DRY_RUN-gated)
    ├─ reuse StatusCollector → DUW Queue (source of truth)   ── SchemaValid? ExpectedFromQueue
    ├─ GET t.me/s/duw_queue_updates (browser UA) → ParseChannel → ObservedState (last STATUS post)
    ├─ get/update ConfigMap duw-doctor-state                 ── debounce + cooldown (read-only SA + 1 CM)
    └─ on sustained mismatch (Evaluate → Escalate, not in cooldown):
         ├─ best-effort snapshot: queue-monitor pod status + logs --since=3h + events + deploy status
         └─ Telegram alert → PRIVATE feedback chat (concise summary; full snapshot to Job stdout)

PHASE B (adds the cloud doctor — OUTLINE ONLY this round):
  checker escalation ALSO opens a GitHub issue (snapshot in body) ending "@claude deep-dive ..."
         ↓ (GitHub cloud — NO cluster creds)
  .github/workflows/claude.yml (anthropics/claude-code-action, ANTHROPIC_API_KEY repo secret)
    └─ checks out repo, reads snapshot from issue, live-polls DUW + t.me/s,
       classifies REAL_INCIDENT | DUW_CONTRACT_DRIFT | TRANSIENT/FALSE_ALARM | WINDOW_EDGE | KNOWN_DUPLICATE_FLOOD,
       posts verdict to the issue + Telegram. Never remediates.
```

The doctor has **zero cluster access** → structurally cannot restart/redeploy. The checker is **read-only** except `get/update` on its own state ConfigMap.

---

## Component layout (exact paths — matches the repo's flat+subdir pattern)

```
duw-queue-monitor/
├── cmd/duwdoctorchecker/
│   ├── main.go                      # checker entrypoint (integration glue; build/vet + prd-verified)
│   └── Dockerfile                   # mirror cmd/queuemonitor/Dockerfile (incl. Certum OV CA)
├── internal/duwdoctor/
│   ├── window.go        + window_test.go     # ZoneAt(now,start,end,pad) → Out|Padding|In
│   ├── expected.go      + expected_test.go   # ExpectedFromQueue(Queue) → State
│   ├── telegram.go      + telegram_test.go   # ParseChannel + ObservedState + MaxIdenticalRun (goquery)
│   ├── state.go         + state_test.go      # DoctorState (de)serialize + InCooldown
│   ├── check.go         + check_test.go      # Evaluate(CheckInput) → CheckResult (pure)
│   └── testdata/channel.html                 # real t.me/s fixture (copied from duw-mcp-server)
├── infra/k8s/base/duw-doctor/
│   ├── serviceaccount-checker.yml
│   ├── role-checker.yml             # READ-ONLY pods/logs/events/deploys + get/update one CM
│   ├── rolebinding-checker.yml
│   ├── configmap-state.yml          # duw-doctor-state  (data: state.json: "{}")
│   └── cronjob-checker.yml          # references the existing telegram-bot-external-secret
# referenced individually from infra/k8s/base/kustomization.yaml; image added to ovh-prd overlay
# Phase B only (outline):
├── .github/workflows/claude.yml
└── docs/duw-doctor-deepdive.md
```

---

## Checker configuration (env on the CronJob)

| Env | Default | Meaning |
|---|---|---|
| `DUW_API_URL` | `https://rezerwacje.duw.pl/status_kolejek/query.php?status=` | source of truth |
| `MONITOR_QUEUE_ID` | `24` | queue under watch |
| `MONITOR_QUEUE_CITY` | `Wrocław` | city key in the DUW response map |
| `TELEGRAM_CHANNEL` | `duw_queue_updates` | public channel to read |
| `WORKING_HOUR_START_UTC` | `5` | monitor true-window start (keep synced w/ monitor overlay) |
| `WORKING_HOUR_END_UTC` | `17` | monitor true-window end |
| `CHECKER_PAD_MINUTES` | `30` | ± boundary grace for the checker window |
| `DEBOUNCE_K` | `3` | consecutive in-window mismatches before escalation |
| `COOLDOWN_MINUTES` | `60` | suppress re-alerts after one fires |
| `DUPLICATE_FLOOD_THRESHOLD` | `5` | ≥ this many byte-identical consecutive recent posts ⇒ duplicate-flood (issue #2; real incident hit 22) |
| `DUPLICATE_WINDOW_MINUTES` | `10` | look-back from **now** for the identical-run count |
| `LOG_SINCE_HOURS` | `3` | pod-log span in the snapshot |
| `STATE_CONFIGMAP` | `duw-doctor-state` | debounce/cooldown store |
| `POD_NAMESPACE` | `default` | injected via downward API |
| `MONITOR_DEPLOYMENT` | `queue-monitor` | deploy to snapshot |
| `MONITOR_POD_SELECTOR` | `app=queue-monitor` | pods to snapshot |
| `CHECKER_HTTP_TIMEOUT_SECONDS` | `10` | client timeout (DUW + t.me) |
| `NOTIFICATION_TELEGRAM_BOT_TOKEN` | (secret) | feedback bot token — from existing `telegram-bot-external-secret`; feeds `notifications.TelegramConfig` |
| `NOTIFICATION_TELEGRAM_FEEDBACK_CHAT_ID` | (secret) | **the existing private feedback chat id** — from existing `telegram-bot-external-secret` |
| `DRY_RUN` | `false` | if true: log intended escalation; do NOT send alert / open issue |
| **Phase B** `ENABLE_GITHUB_DOCTOR` | `false` | escalation also opens a `@claude` GitHub issue |
| **Phase B** `GITHUB_REPO` | `uladzk/duw-queue-monitor` | issue target |
| **Phase B** `GITHUB_TOKEN` | (secret) | token with `issues:write` only |

---

# PHASE A — In-cluster checker + Telegram alert (BUILD NOW)

## A0 — Scaffolding

### Task A0.1: Branch + deps + fixture
**Files:** `go.mod`, `go.sum`, `internal/duwdoctor/testdata/channel.html`

**Steps:**
1. `git checkout main && git pull --ff-only && git checkout -b duw-96-phase-a-doctor-checker`
2. `go get github.com/PuerkitoBio/goquery@latest k8s.io/client-go@latest k8s.io/apimachinery@latest && go mod tidy`
3. `mkdir -p internal/duwdoctor/testdata && cp ~/.claude/mcp-servers/duw-mcp-server/test/fixtures/channel.html internal/duwdoctor/testdata/channel.html`
4. Verify: `go build ./... ` (expect success — no new packages referenced yet) and `ls -l internal/duwdoctor/testdata/channel.html` (expect ~92 KB).
5. **Commit:** `chore(duw-doctor): scaffold checker module deps and channel fixture`

---

## A1 — Pure logic (strict TDD; mirror `internal/queuemonitor/*_test.go` — table-driven + AAA comments)

> Every task: write the failing test → run (FAIL) → minimal impl → run (PASS) → commit. Tests and impl land in the **same commit** (DUW convention does not split them).

### Task A1.1: `window.go` — `ZoneAt`

**Files:** Create `internal/duwdoctor/window.go`, `internal/duwdoctor/window_test.go`

**Step 1 — failing test** (`window_test.go`):
```go
package duwdoctor

import (
	"testing"
	"time"
)

func TestZoneAt(t *testing.T) {
	// Arrange
	const start, end, pad = 5, 17, 30
	mk := func(s string) time.Time {
		ts, err := time.Parse(time.RFC3339, s)
		if err != nil {
			t.Fatalf("bad time %q: %v", s, err)
		}
		return ts
	}
	cases := []struct {
		name string
		now  string
		want Zone
	}{
		{"saturday", "2026-06-13T10:00:00Z", ZoneOutOfWindow},
		{"sunday", "2026-06-14T10:00:00Z", ZoneOutOfWindow},
		{"mon-0420-before-pad", "2026-06-15T04:20:00Z", ZoneOutOfWindow},
		{"mon-0440-pad", "2026-06-15T04:40:00Z", ZonePadding},
		{"mon-0500-in", "2026-06-15T05:00:00Z", ZoneInWindow},
		{"mon-1659-in", "2026-06-15T16:59:00Z", ZoneInWindow},
		{"mon-1715-pad", "2026-06-15T17:15:00Z", ZonePadding},
		{"mon-1740-out", "2026-06-15T17:40:00Z", ZoneOutOfWindow},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// Act
			got := ZoneAt(mk(tc.now), start, end, pad)
			// Assert
			if got != tc.want {
				t.Fatalf("ZoneAt(%s) = %v, want %v", tc.now, got, tc.want)
			}
		})
	}
}
```

**Step 2 — run/FAIL:** `go test ./internal/duwdoctor/ -run TestZoneAt -v` → FAIL (undefined: `ZoneAt`, `Zone`).

**Step 3 — minimal impl** (`window.go`):
```go
package duwdoctor

import "time"

type Zone int

const (
	ZoneOutOfWindow Zone = iota
	ZonePadding
	ZoneInWindow
)

// ZoneAt classifies now (compared in UTC) against the monitor's true window
// (weekdays, [startHour, endHour) UTC) with ±padMinutes of boundary grace.
// Mirrors WeekdayQueueMonitor.isDuwOffTime semantics, plus the padding band.
func ZoneAt(now time.Time, startHour, endHour, padMinutes int) Zone {
	u := now.UTC()
	if u.Weekday() == time.Saturday || u.Weekday() == time.Sunday {
		return ZoneOutOfWindow
	}
	start := time.Date(u.Year(), u.Month(), u.Day(), startHour, 0, 0, 0, time.UTC)
	end := time.Date(u.Year(), u.Month(), u.Day(), endHour, 0, 0, 0, time.UTC)
	if !u.Before(start) && u.Before(end) {
		return ZoneInWindow
	}
	pad := time.Duration(padMinutes) * time.Minute
	inLeadPad := !u.Before(start.Add(-pad)) && u.Before(start)
	inTrailPad := !u.Before(end) && u.Before(end.Add(pad))
	if inLeadPad || inTrailPad {
		return ZonePadding
	}
	return ZoneOutOfWindow
}
```

**Step 4 — run/PASS:** `go test ./internal/duwdoctor/ -run TestZoneAt -v` → PASS.

**Step 5 — commit:** `feat(duw-doctor): add working-window zone classifier`

### Task A1.2: `expected.go` — `ExpectedFromQueue`

**Files:** Create `internal/duwdoctor/expected.go`, `internal/duwdoctor/expected_test.go`

**Step 1 — failing test:**
```go
package duwdoctor

import (
	"testing"

	"github.com/uladzk/duw-queue-monitor/internal/queuemonitor"
)

func TestExpectedFromQueue(t *testing.T) {
	// Arrange
	cases := []struct {
		name string
		q    queuemonitor.Queue
		want State
	}{
		{"inactive", queuemonitor.Queue{Active: false, Enabled: true}, StateInactive},
		{"active-disabled", queuemonitor.Queue{Active: true, Enabled: false}, StateActiveDisabled},
		{"active-enabled", queuemonitor.Queue{Active: true, Enabled: true}, StateActiveEnabled},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// Act / Assert
			if got := ExpectedFromQueue(tc.q); got != tc.want {
				t.Fatalf("ExpectedFromQueue(%+v) = %v, want %v", tc.q, got, tc.want)
			}
		})
	}
}
```

**Step 2 — run/FAIL:** `go test ./internal/duwdoctor/ -run TestExpectedFromQueue -v` → FAIL (undefined `ExpectedFromQueue`, `State`).

**Step 3 — minimal impl** (`expected.go`):
```go
package duwdoctor

import "github.com/uladzk/duw-queue-monitor/internal/queuemonitor"

type State string

const (
	StateInactive       State = "Inactive"
	StateActiveDisabled State = "ActiveDisabled"
	StateActiveEnabled  State = "ActiveEnabled"
)

// ExpectedFromQueue maps a DUW queue state to the state the monitor should reflect.
func ExpectedFromQueue(q queuemonitor.Queue) State {
	switch {
	case !q.Active:
		return StateInactive
	case !q.Enabled:
		return StateActiveDisabled
	default:
		return StateActiveEnabled
	}
}
```

**Step 4 — run/PASS:** → PASS.
**Step 5 — commit:** `feat(duw-doctor): map DUW queue to expected monitor state`

### Task A1.3: `telegram.go` — `ParseChannel` + `ObservedState`

**Files:** Create `internal/duwdoctor/telegram.go`, `internal/duwdoctor/telegram_test.go` (testdata already present from A0.1)

**Step 1 — failing test:**
```go
package duwdoctor

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func readFixture(t *testing.T, name string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatalf("read fixture %s: %v", name, err)
	}
	return string(b)
}

func TestParseChannel_Fixture(t *testing.T) {
	// Arrange
	html := readFixture(t, "channel.html")
	// Act
	posts, err := ParseChannel(html)
	// Assert
	if err != nil {
		t.Fatalf("ParseChannel error: %v", err)
	}
	if len(posts) != 20 {
		t.Fatalf("got %d posts, want 20", len(posts))
	}
	if posts[0].ID != 17963 || posts[len(posts)-1].ID != 17982 {
		t.Fatalf("ids: first=%d last=%d, want 17963/17982", posts[0].ID, posts[len(posts)-1].ID)
	}
}

func TestObservedState_SkipsDailySummary(t *testing.T) {
	// Arrange — newest post (17982) is the 📊 daily summary; most recent STATUS post is 🌙 (17981)
	posts, err := ParseChannel(readFixture(t, "channel.html"))
	if err != nil {
		t.Fatalf("ParseChannel error: %v", err)
	}
	// Act
	st, at, ok := ObservedState(posts)
	// Assert
	if !ok || st != StateInactive {
		t.Fatalf("ObservedState = (%v, ok=%v), want (Inactive, true)", st, ok)
	}
	want := time.Date(2026, 6, 15, 13, 38, 54, 0, time.UTC)
	if !at.Equal(want) {
		t.Fatalf("status post time = %s, want %s", at, want)
	}
}

func TestClassifyStatus_SubstringTrap(t *testing.T) {
	// Arrange — "niedostępna" contains "dostępna"; must NOT classify as ActiveEnabled
	cases := map[string]State{
		"💤 Kolejka odbiór karty jest obecnie niedostępna (na razie nie ma wolnych biletów).": StateActiveDisabled,
		"🔔 Kolejka odbiór karty jest dostępna! Pozostało biletów: 8":                          StateActiveEnabled,
		"🌙 Kolejka odbiór karty jest nieaktywna — prawdopodobnie koniec godzin pracy DUW.":     StateInactive,
		"📊 Kolejka Odbiór karty pobytu — podsumowanie dnia:":                                   "", // not a status
	}
	for text, want := range cases {
		st, ok := classifyStatus(text)
		if want == "" {
			if ok {
				t.Fatalf("classifyStatus(%q) ok=true, want not-a-status", text)
			}
			continue
		}
		// Act / Assert
		if !ok || st != want {
			t.Fatalf("classifyStatus(%q) = (%v, %v), want %v", text, st, ok, want)
		}
	}
}
```

**Step 2 — run/FAIL:** `go test ./internal/duwdoctor/ -run 'TestParseChannel|TestObservedState|TestClassifyStatus' -v` → FAIL (undefined symbols).

**Step 3 — minimal impl** (`telegram.go`):
```go
package duwdoctor

import (
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
)

type Post struct {
	ID   int
	At   time.Time
	Text string
}

// ParseChannel parses t.me/s/<channel> HTML into posts sorted ascending by id.
// Ported from duw-mcp-server src/services/telegram-web.ts (parseChannelHtml).
func ParseChannel(html string) ([]Post, error) {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		return nil, err
	}
	var posts []Post
	doc.Find(".tgme_widget_message[data-post]").Each(func(_ int, s *goquery.Selection) {
		dp, ok := s.Attr("data-post") // "<channel>/<id>"
		if !ok {
			return
		}
		parts := strings.Split(dp, "/")
		id, err := strconv.Atoi(parts[len(parts)-1])
		if err != nil {
			return
		}
		textSel := s.Find(".tgme_widget_message_text").First()
		textSel.Find("br").ReplaceWithHtml("\n")
		// NOTE: the first arg below is a literal non-breaking space (U+00A0), normalized to
		// a regular space — same as the cheerio source's .replace(/<nbsp>/g, " ").
		text := strings.TrimSpace(strings.ReplaceAll(textSel.Text(), " ", " "))
		if text == "" {
			return // media-only / service post
		}
		dt, _ := s.Find("a.tgme_widget_message_date time").Attr("datetime")
		at, _ := time.Parse(time.RFC3339, dt)
		posts = append(posts, Post{ID: id, At: at.UTC(), Text: text})
	})
	sort.Slice(posts, func(i, j int) bool { return posts[i].ID < posts[j].ID })
	return posts, nil
}

// ObservedState returns the state implied by the most recent STATUS post
// (scanning newest→oldest, skipping the daily summary / non-status posts).
func ObservedState(posts []Post) (State, time.Time, bool) {
	for i := len(posts) - 1; i >= 0; i-- {
		if st, ok := classifyStatus(posts[i].Text); ok {
			return st, posts[i].At, true
		}
	}
	return "", time.Time{}, false
}

// classifyStatus order matters: "niedostępna" contains "dostępna".
func classifyStatus(text string) (State, bool) {
	switch {
	case strings.Contains(text, "nieaktywna"):
		return StateInactive, true
	case strings.Contains(text, "niedostępna"):
		return StateActiveDisabled, true
	case strings.Contains(text, "dostępna"):
		return StateActiveEnabled, true
	default:
		return "", false
	}
}
```

**Step 4 — run/PASS:** → PASS.
**Step 5 — commit:** `feat(duw-doctor): parse t.me/s channel into observed state`

### Task A1.4: `telegram.go` — `MaxIdenticalRun` (duplicate-flood detector, issue #2)

**Files:** append to `internal/duwdoctor/telegram.go`; add test to `telegram_test.go`

**Step 1 — failing test:**
```go
func TestMaxIdenticalRun(t *testing.T) {
	base := time.Date(2026, 6, 16, 9, 35, 0, 0, time.UTC)
	now := base.Add(70 * time.Second)
	window := 10 * time.Minute
	flood := func(n int) []Post {
		out := make([]Post, n)
		for i := 0; i < n; i++ {
			out[i] = Post{ID: 18000 + i, At: base.Add(time.Duration(i) * 3 * time.Second),
				Text: "🔔 Kolejka odbiór karty jest dostępna! Pozostało biletów: 8"}
		}
		return out
	}
	cases := []struct {
		name  string
		posts []Post
		now   time.Time
		want  int
	}{
		{"22-identical-recent", flood(22), now, 22},
		{"flood-too-old", flood(22), base.Add(2 * time.Hour), 0},
		{"mixed-recent", []Post{
			{ID: 1, At: now.Add(-2 * time.Minute), Text: "a"},
			{ID: 2, At: now.Add(-1 * time.Minute), Text: "b"},
			{ID: 3, At: now, Text: "c"},
		}, now, 1},
		{"empty", nil, now, 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := MaxIdenticalRun(tc.posts, tc.now, window); got != tc.want {
				t.Fatalf("MaxIdenticalRun(%s) = %d, want %d", tc.name, got, tc.want)
			}
		})
	}
}
```

**Step 2 — run/FAIL:** `go test ./internal/duwdoctor/ -run TestMaxIdenticalRun -v` → FAIL (undefined `MaxIdenticalRun`).

**Step 3 — minimal impl** (append to `telegram.go`):
```go
// MaxIdenticalRun returns the longest run of consecutive byte-identical Text values
// among posts whose At is within `window` of `now`. Window is relative to NOW (not the
// newest post) so an old flood does not re-trigger forever.
func MaxIdenticalRun(posts []Post, now time.Time, window time.Duration) int {
	cutoff := now.Add(-window)
	var recent []Post
	for _, p := range posts {
		if !p.At.Before(cutoff) {
			recent = append(recent, p)
		}
	}
	if len(recent) == 0 {
		return 0
	}
	best, cur := 1, 1
	for i := 1; i < len(recent); i++ {
		if recent[i].Text == recent[i-1].Text {
			cur++
		} else {
			cur = 1
		}
		if cur > best {
			best = cur
		}
	}
	return best
}
```
(Posts arrive sorted ascending by id from `ParseChannel`; this preserves chronological order for the run scan.)

**Step 4 — run/PASS:** → PASS.
**Step 5 — commit:** `feat(duw-doctor): detect duplicate-notification floods`

### Task A1.5: `state.go` — debounce/cooldown state

**Files:** Create `internal/duwdoctor/state.go`, `internal/duwdoctor/state_test.go`

**Step 1 — failing test:**
```go
package duwdoctor

import (
	"testing"
	"time"
)

func TestDoctorState_RoundTrip(t *testing.T) {
	// Arrange
	now := time.Date(2026, 6, 16, 9, 0, 0, 0, time.UTC)
	in := DoctorState{ConsecutiveMismatches: 2, LastEscalationAt: now, LastReason: "monitor_missed_transition"}
	// Act
	raw, err := in.Marshal()
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	out, err := ParseState(raw)
	if err != nil {
		t.Fatalf("ParseState: %v", err)
	}
	// Assert
	if out.ConsecutiveMismatches != 2 || out.LastReason != "monitor_missed_transition" || !out.LastEscalationAt.Equal(now) {
		t.Fatalf("round-trip mismatch: %+v", out)
	}
}

func TestParseState_Empty(t *testing.T) {
	for _, raw := range []string{"", "{}"} {
		s, err := ParseState(raw)
		if err != nil {
			t.Fatalf("ParseState(%q): %v", raw, err)
		}
		if s.ConsecutiveMismatches != 0 || !s.LastEscalationAt.IsZero() {
			t.Fatalf("ParseState(%q) = %+v, want zero", raw, s)
		}
	}
}

func TestInCooldown(t *testing.T) {
	now := time.Date(2026, 6, 16, 9, 0, 0, 0, time.UTC)
	cd := 60 * time.Minute
	if (DoctorState{}).InCooldown(now, cd) {
		t.Fatal("zero LastEscalationAt must not be in cooldown")
	}
	within := DoctorState{LastEscalationAt: now.Add(-30 * time.Minute)}
	if !within.InCooldown(now, cd) {
		t.Fatal("30m < 60m must be in cooldown")
	}
	after := DoctorState{LastEscalationAt: now.Add(-90 * time.Minute)}
	if after.InCooldown(now, cd) {
		t.Fatal("90m > 60m must not be in cooldown")
	}
}
```

**Step 2 — run/FAIL:** `go test ./internal/duwdoctor/ -run 'TestDoctorState|TestParseState|TestInCooldown' -v` → FAIL.

**Step 3 — minimal impl** (`state.go`):
```go
package duwdoctor

import (
	"encoding/json"
	"strings"
	"time"
)

type DoctorState struct {
	ConsecutiveMismatches int       `json:"consecutiveMismatches"`
	LastEscalationAt      time.Time `json:"lastEscalationAt"`
	LastReason            string    `json:"lastReason"`
}

func ParseState(data string) (DoctorState, error) {
	if strings.TrimSpace(data) == "" {
		return DoctorState{}, nil
	}
	var s DoctorState
	err := json.Unmarshal([]byte(data), &s)
	return s, err
}

func (s DoctorState) Marshal() (string, error) {
	b, err := json.Marshal(s)
	return string(b), err
}

func (s DoctorState) InCooldown(now time.Time, cooldown time.Duration) bool {
	if s.LastEscalationAt.IsZero() {
		return false
	}
	return now.Before(s.LastEscalationAt.Add(cooldown))
}
```

**Step 4 — run/PASS:** → PASS.
**Step 5 — commit:** `feat(duw-doctor): add debounce and cooldown state`

### Task A1.6: `check.go` — `Evaluate` (pure, no I/O — the heart of the checker)

**Files:** Create `internal/duwdoctor/check.go`, `internal/duwdoctor/check_test.go`

**Rules:**
- **Duplicate-flood branch first**, independent of state comparison (state is correct during a flood): if `MaxIdenticalRun ≥ FloodThreshold` and not in cooldown → Escalate `duplicate_flood: N identical posts` (reset mismatch counter).
- `ZoneOutOfWindow` → OK, reset counter (monitor off by design).
- `!SchemaValid` → debounced escalation `duw_contract_drift` (DUW unreachable/contract changed).
- match (`ObservedOK && Expected == Observed`) → OK, reset counter.
- `ZonePadding` + mismatch → OK, counter unchanged (boundary grace).
- `ZoneInWindow` + mismatch → increment; at `≥ DebounceK` and not in cooldown → Escalate `monitor_missed_transition: expected=X observed=Y`.

**Step 1 — failing test:**
```go
package duwdoctor

import (
	"testing"
	"time"
)

func TestEvaluate(t *testing.T) {
	now := time.Date(2026, 6, 8, 14, 0, 0, 0, time.UTC) // Monday, in-window
	const k = 3
	const cd = 60 * time.Minute
	base := func() CheckInput {
		return CheckInput{
			Now: now, Zone: ZoneInWindow, SchemaValid: true,
			Expected: StateActiveEnabled, Observed: StateActiveEnabled, ObservedOK: true,
			MaxIdenticalRun: 1, DebounceK: k, Cooldown: cd, FloodThreshold: 5,
		}
	}
	t.Run("out-of-window resets", func(t *testing.T) {
		in := base()
		in.Zone = ZoneOutOfWindow
		in.Observed = StateInactive // mismatch, but monitor is off
		in.Prev = DoctorState{ConsecutiveMismatches: 2}
		r := Evaluate(in)
		if r.Escalate || r.Next.ConsecutiveMismatches != 0 {
			t.Fatalf("out-of-window: %+v", r)
		}
	})
	t.Run("match resets", func(t *testing.T) {
		in := base()
		in.Prev = DoctorState{ConsecutiveMismatches: 2}
		r := Evaluate(in)
		if r.Escalate || r.Next.ConsecutiveMismatches != 0 {
			t.Fatalf("match: %+v", r)
		}
	})
	t.Run("06-08 freeze signature escalates at K", func(t *testing.T) {
		// DUW closed (Inactive) but channel still shows ActiveEnabled; K-1 already accrued.
		in := base()
		in.Expected, in.Observed = StateInactive, StateActiveEnabled
		in.Prev = DoctorState{ConsecutiveMismatches: k - 1}
		r := Evaluate(in)
		if !r.Escalate || r.Reason == "" || r.Next.LastEscalationAt.IsZero() {
			t.Fatalf("expected escalation, got %+v", r)
		}
	})
	t.Run("single blip below K does not escalate", func(t *testing.T) {
		in := base()
		in.Expected, in.Observed = StateInactive, StateActiveEnabled
		in.Prev = DoctorState{ConsecutiveMismatches: 0}
		r := Evaluate(in)
		if r.Escalate || r.Next.ConsecutiveMismatches != 1 {
			t.Fatalf("single blip: %+v", r)
		}
	})
	t.Run("padding mismatch does not accrue", func(t *testing.T) {
		in := base()
		in.Zone = ZonePadding
		in.Expected, in.Observed = StateActiveEnabled, StateInactive
		in.Prev = DoctorState{ConsecutiveMismatches: 1}
		r := Evaluate(in)
		if r.Escalate || r.Next.ConsecutiveMismatches != 1 {
			t.Fatalf("padding: %+v", r)
		}
	})
	t.Run("cooldown suppresses escalation", func(t *testing.T) {
		in := base()
		in.Expected, in.Observed = StateInactive, StateActiveEnabled
		in.Prev = DoctorState{ConsecutiveMismatches: k - 1, LastEscalationAt: now.Add(-10 * time.Minute)}
		r := Evaluate(in)
		if r.Escalate {
			t.Fatalf("cooldown should suppress, got %+v", r)
		}
	})
	t.Run("schema invalid debounces to contract drift", func(t *testing.T) {
		in := base()
		in.SchemaValid = false
		in.Prev = DoctorState{ConsecutiveMismatches: k - 1}
		r := Evaluate(in)
		if !r.Escalate || r.Reason != "duw_contract_drift" {
			t.Fatalf("contract drift: %+v", r)
		}
	})
	t.Run("flood escalates regardless of state match", func(t *testing.T) {
		in := base() // state matches
		in.MaxIdenticalRun = 22
		r := Evaluate(in)
		if !r.Escalate || r.Next.LastEscalationAt.IsZero() {
			t.Fatalf("flood: %+v", r)
		}
	})
	t.Run("flood in cooldown does not escalate", func(t *testing.T) {
		in := base()
		in.MaxIdenticalRun = 22
		in.Prev = DoctorState{LastEscalationAt: now.Add(-5 * time.Minute)}
		r := Evaluate(in)
		if r.Escalate {
			t.Fatalf("flood cooldown: %+v", r)
		}
	})
}
```

**Step 2 — run/FAIL:** `go test ./internal/duwdoctor/ -run TestEvaluate -v` → FAIL (undefined `Evaluate`, `CheckInput`, `CheckResult`).

**Step 3 — minimal impl** (`check.go`):
```go
package duwdoctor

import (
	"fmt"
	"time"
)

type CheckInput struct {
	Now             time.Time
	Zone            Zone
	SchemaValid     bool  // DUW response decoded and queue found
	Expected        State // from DUW
	Observed        State // from channel
	ObservedOK      bool  // a status post was found
	MaxIdenticalRun int   // duplicate-flood signal
	Prev            DoctorState
	DebounceK       int
	Cooldown        time.Duration
	FloodThreshold  int
}

type CheckResult struct {
	Escalate bool
	Reason   string
	Next     DoctorState
}

func Evaluate(in CheckInput) CheckResult {
	next := in.Prev

	// Duplicate-flood — independent of state comparison (state is correct during a flood).
	if in.FloodThreshold > 0 && in.MaxIdenticalRun >= in.FloodThreshold {
		if in.Prev.InCooldown(in.Now, in.Cooldown) {
			return CheckResult{Escalate: false, Reason: "duplicate_flood (cooldown)", Next: next}
		}
		next.ConsecutiveMismatches = 0
		next.LastEscalationAt = in.Now
		next.LastReason = fmt.Sprintf("duplicate_flood: %d identical posts", in.MaxIdenticalRun)
		return CheckResult{Escalate: true, Reason: next.LastReason, Next: next}
	}

	// Monitor off by design → never a fault.
	if in.Zone == ZoneOutOfWindow {
		next.ConsecutiveMismatches = 0
		return CheckResult{Escalate: false, Reason: "out_of_window", Next: next}
	}

	// DUW unreachable / contract changed.
	if !in.SchemaValid {
		next.ConsecutiveMismatches++
		if next.ConsecutiveMismatches >= in.DebounceK && !in.Prev.InCooldown(in.Now, in.Cooldown) {
			next.LastEscalationAt = in.Now
			next.LastReason = "duw_contract_drift"
			return CheckResult{Escalate: true, Reason: next.LastReason, Next: next}
		}
		return CheckResult{Escalate: false, Reason: "duw_contract_drift (debouncing)", Next: next}
	}

	if in.ObservedOK && in.Expected == in.Observed {
		next.ConsecutiveMismatches = 0
		return CheckResult{Escalate: false, Reason: "ok", Next: next}
	}

	// Mismatch. Padding band: boundary grace — neither accrue nor escalate.
	if in.Zone == ZonePadding {
		return CheckResult{Escalate: false, Reason: "padding_mismatch", Next: next}
	}

	next.ConsecutiveMismatches++
	if next.ConsecutiveMismatches >= in.DebounceK && !in.Prev.InCooldown(in.Now, in.Cooldown) {
		next.LastEscalationAt = in.Now
		next.LastReason = fmt.Sprintf("monitor_missed_transition: expected=%s observed=%s", in.Expected, observedLabel(in))
		return CheckResult{Escalate: true, Reason: next.LastReason, Next: next}
	}
	return CheckResult{Escalate: false, Reason: "mismatch (debouncing)", Next: next}
}

func observedLabel(in CheckInput) string {
	if !in.ObservedOK {
		return "none"
	}
	return string(in.Observed)
}
```

**Step 4 — run/PASS:** `go test ./internal/duwdoctor/ -v` → all PASS.
**Step 5 — commit:** `feat(duw-doctor): evaluate checks with debounce and classification`

---

## A2 — Entrypoint (integration glue — build/vet verified; behavior validated on prd)

### Task A2.1: `cmd/duwdoctorchecker/main.go`

**Files:** Create `cmd/duwdoctorchecker/main.go`

Structure (complete, but this is I/O glue — not TDD; correctness is proven by the prd DRY_RUN run in A5):

1. **Config** via `caarlos0/env` (`Config` struct with the env table above; reuse `notifications.TelegramConfig` for the bot token). Build `logger` exactly like `cmd/queuemonitor/main.go::buildLogger`.
2. **HTTP client:** `httpClient := &http.Client{Timeout: time.Duration(cfg.HttpTimeoutSeconds) * time.Second}`.
3. **DUW fetch — reuse `StatusCollector`:**
   ```go
   qmCfg := &queuemonitor.QueueMonitorConfig{
       StatusMonitoredQueueId:   cfg.MonitorQueueID,
       StatusMonitoredQueueCity: cfg.MonitorQueueCity,
       StatusApiUrl:             cfg.DuwApiUrl,
       StatusCheckTimeoutMs:     4000,
       StatusCheckMaxAttempts:   3,
       StatusCheckAttemptDelayMs: 500,
   }
   q, err := queuemonitor.NewStatusCollector(qmCfg, httpClient, log).GetQueueStatus(ctx)
   schemaValid := err == nil
   var expected duwdoctor.State
   if schemaValid { expected = duwdoctor.ExpectedFromQueue(*q) }
   ```
4. **Channel fetch:** GET `https://t.me/s/<cfg.TelegramChannel>` with header `User-Agent: Mozilla/5.0 (compatible; duw-doctor-checker)`, read body, `posts, _ := duwdoctor.ParseChannel(body)`; `observed, _, observedOK := duwdoctor.ObservedState(posts)`; `run := duwdoctor.MaxIdenticalRun(posts, now, dupWindow)`. Wrap in a small 3-attempt retry (mirror StatusCollector's retry intent) so a single t.me blip does not look like an outage.
5. **State store via client-go** (`rest.InClusterConfig()` → `kubernetes.NewForConfig`): read ConfigMap `cfg.StateConfigMap` (`data["state.json"]`) → `duwdoctor.ParseState`. Build `CheckInput` (Zone from `duwdoctor.ZoneAt(now, start, end, pad)`), run `duwdoctor.Evaluate`, write `Next` back via `ConfigMaps(ns).Update` (set `data["state.json"]`).
6. **On `Escalate && !DryRun`:** best-effort snapshot (each call wrapped so a failure logs and continues, never blocks the alert):
   - `clientset.CoreV1().Pods(ns).List(metav1.ListOptions{LabelSelector: cfg.MonitorPodSelector})` → names + phase + restart counts
   - per pod: `clientset.CoreV1().Pods(ns).GetLogs(name, &corev1.PodLogOptions{SinceSeconds: ptr(int64(cfg.LogSinceHours*3600))}).Stream(ctx)`
   - `clientset.CoreV1().Events(ns).List(...)` (recent) and `clientset.AppsV1().Deployments(ns).Get(ctx, cfg.MonitorDeployment, ...)` status
   - **Telegram alert** via `notifications.NewTelegramNotifier(&tgCfg, log, httpClient).SendMessage(ctx, cfg.AlertChatID, text)` — concise HTML summary: reason, expected vs observed, the last 3 channel posts (time + text), restart count, and (Phase B) a deep-dive hint. Print the FULL snapshot to stdout (kept in the Job logs).
7. **On `Escalate && DryRun`:** log the intended escalation (reason + summary) only; do not send.
8. Exit 0 on a normal/cooldown/no-escalation run; exit non-zero only on a hard error (e.g. cannot reach the cluster API to read/write state) so failed Jobs are visible.

### Task A2.2: build + vet
- `go build ./... && go vet ./...` → clean (expect no output).
- **Commit:** `feat(duw-doctor): checker entrypoint with telegram alert`

---

## A3 — Image + publish wiring

### Task A3.1: `cmd/duwdoctorchecker/Dockerfile` (mirror queuemonitor's — **Certum CA is mandatory**)
```dockerfile
FROM golang:1.25.3-alpine3.22 AS build
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags=-s -o bin/duwdoctorchecker ./cmd/duwdoctorchecker

# Certum OV CA — required to validate TLS against the DUW API (broken server chain).
RUN apk --no-cache add curl ca-certificates openssl && \
    curl -s http://repository.certum.pl/ovcasha2.cer -o /tmp/certum-ov-ca.cer && \
    openssl x509 -inform DER -in /tmp/certum-ov-ca.cer -out /usr/local/share/ca-certificates/certum-ov-ca.crt && \
    update-ca-certificates && \
    rm /tmp/certum-ov-ca.cer

FROM alpine:3.22 AS final
WORKDIR /app
COPY --from=build /etc/ssl/certs/ /etc/ssl/certs/
COPY --from=build /app/bin/duwdoctorchecker .
CMD ["./duwdoctorchecker"]
```
- Verify locally: `docker build -t duw-doctor-checker-test:latest -f cmd/duwdoctorchecker/Dockerfile .` → success.
- **Commit:** `build(duw-doctor): add checker Dockerfile`

### Task A3.2: add service to publish workflow
- Edit `.github/workflows/publish.yml` `options:` → `[queue-monitor, telegram-bot, queue-stats-reports, duw-migrations, duw-doctor-checker]`.
- Local check per **duw-develop-and-publish:** `go test ./internal/duwdoctor/... && docker build -f cmd/duwdoctorchecker/Dockerfile .`
- **Commit:** `build(duw-doctor): add duw-doctor-checker to publish workflow`
- **(Operator triggers the actual publish — not in this plan.)**

---

## A4 — Manifests (RBAC is the safety guarantee)

> Every doctor manifest carries `labels: { app: duw-doctor-checker }` so the prd rollout can be label-scoped (never a whole-overlay `apply -k`, which would re-create the `duw-migrations` Job — see `reference_negative_tickets_left_incident_2026_06_08`).

### Task A4.1: `infra/k8s/base/duw-doctor/role-checker.yml` (namespaced Role, ns `default`)
```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
  name: duw-doctor-checker
  namespace: default
  labels:
    app: duw-doctor-checker
rules:
  - apiGroups: [""]
    resources: ["pods", "pods/log", "events"]
    verbs: ["get", "list", "watch"]
  - apiGroups: ["apps"]
    resources: ["deployments", "replicasets"]
    verbs: ["get", "list"]
  - apiGroups: [""]
    resources: ["configmaps"]
    resourceNames: ["duw-doctor-state"]
    verbs: ["get", "update"]
```
**Deliberately absent:** every `create/patch/delete` on workloads, `pods/exec`, `pods/eviction`, `jobs`, `secrets`. (Secrets reach the pod as a kubelet-injected env from the ESO Secret — no get-secret RBAC needed.)

### Task A4.2: SA + RoleBinding + state ConfigMap + ExternalSecret + CronJob

`serviceaccount-checker.yml`:
```yaml
apiVersion: v1
kind: ServiceAccount
metadata:
  name: duw-doctor-checker
  namespace: default
  labels:
    app: duw-doctor-checker
```

`rolebinding-checker.yml`:
```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
metadata:
  name: duw-doctor-checker
  namespace: default
  labels:
    app: duw-doctor-checker
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: Role
  name: duw-doctor-checker
subjects:
  - kind: ServiceAccount
    name: duw-doctor-checker
    namespace: default
```

`configmap-state.yml`:
```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: duw-doctor-state
  namespace: default
  labels:
    app: duw-doctor-checker
data:
  state.json: "{}"
```

> Secrets: the CronJob references the **existing** `telegram-bot-external-secret` Secret directly (operator decision) — **no new ExternalSecret manifest**.

`cronjob-checker.yml` (image uses the ACR base name so the ovh-prd overlay rewrites it):
```yaml
apiVersion: batch/v1
kind: CronJob
metadata:
  name: duw-doctor-checker
  namespace: default
  labels:
    app: duw-doctor-checker
spec:
  schedule: "*/3 4-18 * * 1-5"
  timeZone: "Etc/UTC"
  concurrencyPolicy: Forbid
  startingDeadlineSeconds: 120
  successfulJobsHistoryLimit: 1
  failedJobsHistoryLimit: 3
  jobTemplate:
    spec:
      backoffLimit: 1
      template:
        metadata:
          labels:
            app: duw-doctor-checker
        spec:
          serviceAccountName: duw-doctor-checker
          restartPolicy: Never
          containers:
            - name: duw-doctor-checker
              image: acrduwshared.azurecr.io/duw-doctor-checker:1.0.0
              env:
                - name: LOG_LEVEL
                  value: "info"
                - name: DRY_RUN
                  value: "true"            # flipped to "false" at go-live (A6.1)
                - name: POD_NAMESPACE
                  valueFrom:
                    fieldRef:
                      fieldPath: metadata.namespace
                - name: NOTIFICATION_TELEGRAM_BOT_TOKEN
                  valueFrom:
                    secretKeyRef:
                      name: telegram-bot-external-secret
                      key: NOTIFICATION_TELEGRAM_BOT_TOKEN
                - name: NOTIFICATION_TELEGRAM_FEEDBACK_CHAT_ID
                  valueFrom:
                    secretKeyRef:
                      name: telegram-bot-external-secret
                      key: NOTIFICATION_TELEGRAM_FEEDBACK_CHAT_ID
              resources:
                requests:
                  cpu: 25m
                  memory: 32Mi
                limits:
                  cpu: 100m
                  memory: 64Mi
```
> `timeZone: "Etc/UTC"` (not the stats CronJobs' `Europe/Warsaw`) — intentional, because the monitor's working window is defined in UTC.

### Task A4.3: wire into base + overlay
- Add to `infra/k8s/base/kustomization.yaml` `resources:` (after the existing entries):
  ```yaml
  - duw-doctor/serviceaccount-checker.yml
  - duw-doctor/role-checker.yml
  - duw-doctor/rolebinding-checker.yml
  - duw-doctor/configmap-state.yml
  - duw-doctor/cronjob-checker.yml
  ```
- Add the image entry to `infra/k8s/overlays/ovh-prd/kustomization.yaml` `images:` (see *Resolved open items #4*).
- Verify both overlays still build:
  - `kubectl kustomize infra/k8s/overlays/ovh-prd >/dev/null && echo OK`
  - `kubectl kustomize infra/k8s/overlays/ovh-dev >/dev/null && echo OK` (dev overlay must not break even though there's no dev cluster)
- **Commit:** `ops(duw-doctor): add checker manifests and read-only rbac` (optionally split the overlay image into `ops(duw-doctor): wire checker image into ovh-prd overlay`).

---

## A5 — Deploy + verify on PRD (no dev cluster exists — test SAFELY)

> The OVH dev cluster was destroyed; all testing is on `mks-duw-prd-waw`. The checker is read-only + Telegram-only, so it cannot harm the monitor — **but never scale, patch, or restart the live `queue-monitor`** (that would stop real notifications during work hours). Reading its pods/logs is read-only and safe. **Each deploy step below requires the operator's explicit go-ahead (publish is operator-triggered).**

### Task A5.1: publish image, pin it, apply ONLY the doctor resources, then PROVE RBAC
1. *(Operator)* publish `duw-doctor-checker` `1.0.0` via the publish workflow; confirm `ghcr.io/uladzk/duw-queue-monitor/duw-doctor-checker:1.0.0` exists.
2. Confirm context: `kubectl config current-context` → `mks-duw-prd-waw` (or `kubectx mks-duw-prd-waw`).
3. Label-scoped apply (DRY_RUN=true is baked into the manifest):
   ```bash
   kubectl kustomize infra/k8s/overlays/ovh-prd | kubectl apply -f - -l app=duw-doctor-checker
   ```
   Expect: serviceaccount/role/rolebinding/configmap/externalsecret/cronjob `created`; **no other workload touched**.
4. **RBAC gate — every mutation/exec/secret check must print `no`:**
   ```bash
   SA=system:serviceaccount:default:duw-doctor-checker
   kubectl auth can-i get  pods/log      --as=$SA -n default   # yes
   kubectl auth can-i get  configmaps    --as=$SA -n default   # yes (scoped to duw-doctor-state)
   kubectl auth can-i update configmaps  --as=$SA -n default   # yes (scoped)
   kubectl auth can-i delete pods        --as=$SA -n default   # no
   kubectl auth can-i patch  deployments --as=$SA -n default   # no
   kubectl auth can-i create pods/exec   --as=$SA -n default   # no
   kubectl auth can-i create jobs        --as=$SA -n default   # no
   kubectl auth can-i get    secrets     --as=$SA -n default   # no
   kubectl auth can-i '*'    '*'         --as=$SA -n default   # no
   ```
   **HARD GATE:** if any of the bottom six prints anything but `no`, STOP and fix the Role before continuing.

### Task A5.2: safe functional tests on prd (monitor untouched)
1. **Steady-state / no-false-alarm (DRY_RUN, real data):** confirm the reused Secret is present — `kubectl get secret telegram-bot-external-secret -o jsonpath='{.data}' | wc -c` (non-empty; already synced for the telegram-bot). Trigger a manual run against live DUW + the real channel:
   ```bash
   kubectl create job --from=cronjob/duw-doctor-checker duw-doctor-dryrun-1
   kubectl wait --for=condition=complete job/duw-doctor-dryrun-1 --timeout=120s
   kubectl logs job/duw-doctor-dryrun-1
   ```
   Expect: in-window run logs `ok` (expected==observed), **no** "intended escalation". Let the scheduled DRY_RUN CronJob run across a **full weekday** and confirm zero spurious "intended escalation" lines (the primary false-alarm guard — watch especially the morning open ~05:38 UTC and the 16:00 UTC 📊 summary). Clean up: `kubectl delete job duw-doctor-dryrun-1`.
2. **Escalation path (config-induced mismatch, monitor untouched):** run a ONE-OFF Job with overrides that *guarantee* a mismatch without disturbing anything — point at a channel with no DUW status posts, force the window open, set `DEBOUNCE_K=1`, `DRY_RUN=false`. The alert lands in the **production feedback chat** (operator decision — no separate test chat). **APPROVAL GATE: get explicit operator go-ahead before running this — it fires a real Telegram alert into the production chat.** Save as a scratch manifest (do not commit) and apply, or:
   ```bash
   # scratch one-off Job (env overrides only; same image/SA/secret as the CronJob)
   kubectl create job duw-doctor-escal-1 --from=cronjob/duw-doctor-checker --dry-run=client -o yaml \
     > /tmp/escal.yaml
   # edit /tmp/escal.yaml: add env TELEGRAM_CHANNEL=durov, WORKING_HOUR_START_UTC=0,
   #   WORKING_HOUR_END_UTC=23, DEBOUNCE_K=1, DRY_RUN=false
   #   (chat id inherited from the existing secret = production feedback chat)
   kubectl apply -f /tmp/escal.yaml
   kubectl wait --for=condition=complete job/duw-doctor-escal-1 --timeout=120s
   kubectl logs job/duw-doctor-escal-1
   ```
   Confirm: alert delivered to the **test** chat, full snapshot in the Job stdout, reason `monitor_missed_transition` (or `padding`/`ok` if you mis-set the window — adjust). Clean up: `kubectl delete job duw-doctor-escal-1`. **No override touches the real CronJob or queue-monitor.**
3. Confirm the snapshot (alert + Job stdout) is rich enough to be useful — this also informs the Phase B issue body.

---

## A6 — Go live on prd

### Task A6.1: enable the scheduled checker (DRY_RUN=false)
- *(Operator go-ahead required.)* Set `DRY_RUN` to `"false"` in `cronjob-checker.yml`, commit `ops(duw-doctor): enable checker alerts on prd`, re-apply label-scoped. (The chat id is already the real feedback chat via the ESO Secret.)
- Watch the first full weekday: steady in-window runs exit `ok` cheaply; no spurious alerts.

### Task A6.2: PR (per duw-git-workflow)
- Open a **draft** PR; title `feat(duw-doctor): in-cluster health checker with telegram alert`; body **Changes / Problem / Solution** only; reference "Part of DUW-96" at the end of Solution. Note the duplicated `WORKING_HOUR_*_UTC` (must stay synced with the monitor overlay; future one-ConfigMap refactor).
- Report literal evidence per **superpowers:verification-before-completion**: PR URL, `git diff --stat`, the RBAC `can-i` matrix output, and the DRY_RUN steady-state log.

---

## Phase A — implementation Definition of Done

- [ ] `go build ./... && go vet ./... && go test ./internal/duwdoctor/...` all green (literal output shown).
- [ ] RBAC gate passed on prd — the six mutation/exec/secret `can-i` checks all print `no` (matrix output pasted).
- [ ] DRY_RUN steady-state ran across a **full weekday** on prd with **zero** spurious "intended escalation" (log shown), including the morning open and the 16:00 📊 summary.
- [ ] Config-induced synthetic mismatch delivered an alert to the **production feedback** chat with a useful snapshot (operator-approved before firing); override removed; queue-monitor never scaled/patched/restarted.
- [ ] Scheduled checker live with `DRY_RUN=false` → real feedback chat; first live weekday clean.
- [ ] Draft PR opened, Changes/Problem/Solution body, linked to DUW-96; no commit auto-published or auto-deployed.

---

# PHASE B — Hybrid cloud doctor (OUTLINE ONLY — build after Phase A is running)

> Build only once Phase A has produced real alerts (confirms snapshot richness). Three deliverables; detailed TDD plan to follow in a later round.

- **B1 — `docs/duw-doctor-deepdive.md` (standing prompt):** instruct Claude to read the snapshot from the issue body, check out + read repo source for root-cause, live-poll DUW + `t.me/s`, classify `REAL_INCIDENT | DUW_CONTRACT_DRIFT | TRANSIENT/FALSE_ALARM | WINDOW_EDGE | KNOWN_DUPLICATE_FLOOD`, post a concise verdict + evidence + recommended next action, and **never remediate**. For `duplicate_flood`: recognize the known, accepted Telegram-latency bug (`reference_duplicate_notifications_incident_2026_06_16`) — confirm the identical-post run co-occurs with `context deadline exceeded` send errors in the captured logs, classify `KNOWN_DUPLICATE_FLOOD`, recommend no action unless abnormally large/frequent or co-occurring with a real state mismatch.
- **B2 — `.github/workflows/claude.yml`:** `anthropics/claude-code-action`, trigger on `issues`/`issue_comment` mentioning `@claude` (or label `duw-doctor`), auth via `ANTHROPIC_API_KEY` repo secret, restricted to bot/owner. One-time manual prerequisites (document in the PR, NOT IaC): install the Claude GitHub App on the repo; add the `ANTHROPIC_API_KEY` repo secret.
- **B3 — extend the checker:** behind `ENABLE_GITHUB_DOCTOR`, on escalation also `POST /repos/{repo}/issues` (token scope `issues:write` only) with the full snapshot in the body, ending `@claude deep-dive this duw-doctor incident`. Add `GITHUB_TOKEN` to the ESO Secret (new Infisical key). Existing cooldown/debounce already prevents issue spam.
- **B4 — end-to-end test + tune:** config-induced synthetic incident on prd (per A5.2, monitor untouched, test chat) → checker opens issue → workflow runs → verdict comment + Telegram. Tune the prompt + snapshot contents. PR; link DUW-96.

---

## Commit breakdown (Phase A, per duw-git-workflow)

1. `chore(duw-doctor): scaffold checker module deps and channel fixture` (A0.1)
2. `feat(duw-doctor): add working-window zone classifier` (A1.1)
3. `feat(duw-doctor): map DUW queue to expected monitor state` (A1.2)
4. `feat(duw-doctor): parse t.me/s channel into observed state` (A1.3)
5. `feat(duw-doctor): detect duplicate-notification floods` (A1.4)
6. `feat(duw-doctor): add debounce and cooldown state` (A1.5)
7. `feat(duw-doctor): evaluate checks with debounce and classification` (A1.6)
8. `feat(duw-doctor): checker entrypoint with telegram alert` (A2)
9. `build(duw-doctor): add checker Dockerfile` (A3.1)
10. `build(duw-doctor): add duw-doctor-checker to publish workflow` (A3.2)
11. `ops(duw-doctor): add checker manifests and read-only rbac` (A4.1–A4.3)
12. `ops(duw-doctor): wire checker image into ovh-prd overlay` (A4.3 — optional split)
13. `ops(duw-doctor): enable checker alerts on prd` (A6.1)

No step auto-publishes or auto-deploys — publish is operator-triggered (A5.1), and every prd apply needs explicit go-ahead.

---

## Out of scope (YAGNI)
- In-cluster LLM / doctor pod (node too small — hybrid instead).
- Subscription/OAuth auth (start with API key; revisit if spend warrants).
- Live cluster access for the cloud doctor (snapshot-only).
- Persisting snapshots to object storage (live in the issue + Job logs; revisit with DUW-83).
- Any auto-remediation/restart (separate, human-approved — DUW-98).
- A separate `STALE_TOLERANCE_SECONDS` timer (subsumed by debounce × cadence — see Revision note #4).
