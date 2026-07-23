---
component: duw-queue-monitor
repo: uladzk/duw-queue-monitor
commit: eb3b6bcfad73ce4a23ec989f96f937a46616fa6d
verified: 2026-07-23
confidence: high
audience: coding-agents, solo-developer
---

# Reliability & symptom map (L2)

How this system fails and where the mechanism lives. Symptom strings are grep targets. Incidents below are drawn from the operator's runbook memory and confirmed against current code; where the code has since been fixed, that is stated.

## Symptom → mechanism → anchor

| Symptom (observed) | Mechanism | Anchor |
|--------------------|-----------|--------|
| **Duplicate notifications** — many identical availability posts within minutes (e.g. ~22 identical "K…/…" posts ~09:35 UTC, 2026-06-16) | Telegram send is non-idempotent under timeout. `sendMessageWithRetries` bounds the whole retry loop by a 5s `context.WithTimeout`; when Telegram is slow the send times out and is treated as a **failure**, but the message was actually delivered. The state machine only advances state after a *successful* send (`Handle` returns the old state on error), so the next tick re-sends. | `internal/notifications/telegram.go:48-90` (timeout bounds retries); `internal/queuemonitor/stateactiveenabled.go:15-36` (return `s` on send error) |
| **Morning "flood" of availability posts** at window open | At day start DUW may report `ticket_value == ""`; `buildQueueAvailableMsg` then uses the *short* template, and each tick where `TicketsLeft` differs re-notifies (`ActiveEnabled` re-fires on any ticket-count change). Empty ticket value + churny counts = many posts. | `internal/queuemonitor/notification.go:28-31`; `stateactiveenabled.go:31` |
| **Monitor loop frozen / no posts** (2026-06-08, v1.5.0) | DUW API returned a **negative `tickets_left`**; older code errored on it and the tick failed every time. **Fixed:** now clamped to 0 with a warning, so transitions/stats still proceed. If you see this again, the clamp was removed or a different negative field appeared. | `internal/queuemonitor/statuscollector.go:66-72` (`"DUW API returned negative TicketsLeft, clamping to 0"`) |
| **"Bot stopped publishing"** report | Historically a FALSE ALARM (2026-06-15): monitor was healthy; the perceived silence was the inactive-notification path plus the working-hours gate. DUW logs are only in `kubectl logs` (not shipped to Loki). Check the clock gate first. | `internal/queuemonitor/monitorweekday.go:52-63` |
| **No data from DUW / empty result** | DUW returns nothing unless `User-Agent` header is empty; or the queue id/city moved. Collector errors `"failed to find the queue status for the queue with id: %v"`. | `statuscollector.go:54` (empty UA), `:61-82` (match + not-found error) |
| **TLS handshake failure to DUW** | DUW's cert chain needs the Certum OV CA, injected in the queue-monitor image. A base-image bump that drops it breaks TLS. | `cmd/queuemonitor/Dockerfile:13-19` |
| **State lost after restart → an extra post** | State is written to Redis only on graceful shutdown, TTL 60s. A crash (no SIGTERM) or downtime >60s loses state; reload starts Uninitialized-equivalent and the first active tick notifies. | `internal/queuemonitor/runner.go:53-59` (save on shutdown only), `monitorstate.go:29` (TTL) |
| **Postgres unreachable at boot** (stats on) | `buildRunner` does `db.Ping()` at startup and aborts if it fails — the pod crashloops rather than running without stats. | `cmd/queuemonitor/main.go:97-99` |

## Concurrency & failure-handling model

- **Single instance per service.** Deployments are `replicas: 1` with `strategy: Recreate` (`infra/k8s/base/queue-monitor-deployment.yml`, `telegram-bot-deployment.yml`) — chosen specifically so a rolling update never runs two monitors that both post. CronJobs use `concurrencyPolicy: Forbid`. There is no leader election; correctness relies on there being exactly one pod.
- **In-memory state is authoritative during a run;** Redis is only a restart cushion (write-on-shutdown, read-on-start). This is a deliberate simplification — the code comments call in-cluster Redis "super reliable" and skip retries (`monitorstate.go:40,63`).
- **Per-tick errors are swallowed** and logged (`runner.go:62-64`); the loop keeps going. So a transient DUW/Telegram failure self-heals on the next tick — which is also why the timeout-based duplicate bug re-sends rather than dropping.
- **Retries:** DUW fetch = 3 attempts / 500ms fixed / 4s budget; Telegram send = 5 attempts / 500ms fixed / 5s budget. Both budgets are context timeouts on the *whole* loop, not per-attempt.

## Resource envelope

Tight by design (cost-optimized single OVH `d2-4` node). queue-monitor 100m/128Mi, telegram-bot 100m req/128Mi limit, CronJobs 50m/64Mi→100m/128Mi, CNPG 100m/256Mi→500m/512Mi, Redis 100m/256Mi. Redis persistence disabled (`save ""`, `appendonly no`) — a Redis restart wipes `monitor:state` (acceptable, see above). Postgres storage 1Gi.

## Known non-idempotency — ACCEPTED, won't fix

**Decision (owner, 2026-06-16, re-confirmed 2026-07-23): accepted as-is — do not propose a fix.** Duplicates are strictly preferable to a dropped alert (a missed "queue available" message can cost someone their appointment); the monitor is intentionally at-least-once, and the one incident in a year self-recovered in ~2 minutes. The duw-doctor checker detects the flood signature as a known issue.

If this is ever revisited: any change to the send/advance ordering must ensure the state advances even when a send *times out but may have succeeded*, or must make sends idempotent (e.g. dedupe key). Don't "fix" it by only widening the timeout — that reduces frequency, not the race — and don't advance-on-timeout (dropping messages was explicitly rejected).

---

## Ideas / not-yet-built (speculative — not current behavior)

The following are tracked ideas from the operator's backlog, listed here only so an agent doesn't mistake their absence for a bug. They are **not** in the code at this pin:
- Intra-day ticket time-series (DUW-94): today only an end-of-day snapshot is stored (`monitor.go:77-84`).
- Drain-rate / ETA in alerts: depends on DUW-94 data.
- A health-watchdog "duw-doctor" (DUW-95/96): a separate checker service, not part of these three binaries.
- GitOps for terraform (DUW-93), CNPG backups to OVH Object Storage (DUW-83).
