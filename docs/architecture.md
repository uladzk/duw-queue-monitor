# Architecture

DUW Queue Monitor watches the appointment queue at Dolnośląski Urząd Wojewódzki (DUW) and posts real-time Telegram notifications when slots open. Three Go services share one module and run on OVH Managed Kubernetes.

This is the short, human-facing view. For the exhaustive, commit-pinned reference with `path:line` anchors, see the [code map](agents/map/_map.md).

## System overview

```mermaid
flowchart LR
  duw["DUW API"] -->|"status (polled)"| qm

  subgraph k8s ["Kubernetes — OVH MKS"]
    qm["queue-monitor<br/>Deployment"]
    sr["queue-stats-reports<br/>CronJobs ×3"]
    tb["telegram-bot<br/>Deployment"]
    redis[("Redis<br/>monitor state")]
    pg[("PostgreSQL<br/>daily stats · CNPG")]
    qm -->|state| redis
    qm -->|stats| pg
    sr -->|read| pg
  end

  qm -->|alerts| tg["Telegram<br/>channel + bot chats"]
  sr -->|reports| tg
  tb <-->|commands / feedback| tg
```

Supporting platform flows — secrets and images:

```mermaid
flowchart LR
  inf["Infisical<br/>secrets manager"] -->|sync| eso["External Secrets<br/>Operator"] -->|k8s Secrets| pods["service pods"]
  ghcr["ghcr.io<br/>container images"] -.->|image pull| pods
```

## Queue check — one tick

The monitor polls on a fixed interval (`STATUS_CHECK_INTERVAL_SECONDS`). A client-side gate skips weekends and hours outside `WORKING_HOUR_START_UTC`–`WORKING_HOUR_END_UTC`, because the DUW API reports the queue as active even when the office is closed.

```mermaid
sequenceDiagram
  participant R as Runner
  participant W as WeekdayQueueMonitor
  participant M as QueueMonitor
  participant C as StatusCollector
  participant D as DUW API
  participant T as Telegram

  R->>W: tick
  alt weekend or outside working hours (UTC)
    W-->>R: skip — no API call
  else working hours
    W->>M: CheckAndProcessStatus
    M->>C: GetQueueStatus
    C->>D: GET query.php?status= (retry ×3)
    D-->>C: queue list JSON
    C-->>M: monitored queue (by id + city)
    M->>M: state.Handle(queue) — compare with previous state
    alt state transition or tickets_left changed
      M->>T: sendMessage to channel
    else no change
      M-->>M: silent
    end
  end
```

Monitor state is persisted to Redis (key `monitor:state`) on shutdown so a restart resumes without duplicate notifications.

## Monitor state machine

Notifications fire only on state changes — never on a repeated identical state. Transitions labeled `notify:` emit a channel message; staying `Inactive` is silent.

```mermaid
stateDiagram-v2
  direction LR
  [*] --> Inactive
  Inactive --> ActiveDisabled: notify: opens, bookings off
  Inactive --> ActiveEnabled: notify: opens, bookings on
  ActiveDisabled --> ActiveEnabled: notify: bookings on
  ActiveEnabled --> ActiveDisabled: notify: bookings off
  ActiveEnabled --> ActiveEnabled: notify: tickets_left changed
  ActiveEnabled --> Inactive: notify: closes + save stats
  ActiveDisabled --> Inactive: notify: closes + save stats
```

Startup actually passes through an `Uninitialized` state that behaves like `Inactive`, so a monitor restarted mid-day notifies about an already-open queue. Daily statistics are written to PostgreSQL on the active→inactive transition (end of the office day), gated by the `FF_DAILY_STATS_ENABLED` feature flag.

## CI/CD and deployment

CI validates every PR; image publishing and cluster deployment are separate, manual steps — there is no continuous deployment.

```mermaid
flowchart LR
  dev["Developer"]
  pr["PR to main"]
  ci["Build and Test<br/>(pull_request.yaml)"]
  lint["golangci-lint<br/>(lint.yml)"]
  pub["publish.yml<br/>(workflow_dispatch)"]
  ghcr["ghcr.io image<br/>+ git tag service-version"]
  k8s["OVH MKS cluster"]

  dev --> pr
  pr --> ci
  pr --> lint
  dev -->|manual trigger| pub
  pub --> ghcr
  dev -->|"manual kubectl apply -k<br/>overlays/ovh-prd"| k8s
  k8s -.->|pull images| ghcr
```

Kubernetes manifests are Kustomize-managed (`infra/k8s/`: shared base + per-cloud/per-env overlays); cluster infrastructure is Terraform-managed (`infra/terraform/ovh/`). Secrets flow from Infisical into the cluster via External Secrets Operator — nothing secret lives in the repo.
