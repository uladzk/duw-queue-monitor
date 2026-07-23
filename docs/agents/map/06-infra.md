---
component: duw-queue-monitor
repo: uladzk/duw-queue-monitor
commit: eb3b6bcfad73ce4a23ec989f96f937a46616fa6d
verified: 2026-07-23
confidence: high
audience: coding-agents, solo-developer
---

# Infrastructure (L2)

Two layers: **k8s manifests** (Kustomize base + overlays) and **terraform** (OVH active, Azure preserved-but-inactive). Production runs on OVH MKS `mks-duw-prd-waw`; the Azure runtime was destroyed but its terraform/manifests are kept on `main` by a code-preservation rule.

## Kubernetes — Kustomize (`infra/k8s`)

`base/kustomization.yaml` lists all base resources. Apply an environment with `kubectl apply -k infra/k8s/overlays/<cloud>-<env>/`. **Caution:** on a fresh cluster a single-shot `apply -k` can race CRDs (CNPG/ESO) — split the apply (project memory: "Phase 9 must split apply").

### Base workloads (`base/`)

| Resource | File | Key facts |
|----------|------|-----------|
| queue-monitor Deployment | `queue-monitor-deployment.yml` | replicas 1, **`strategy: Recreate`**, `STATE_REDIS_CONNECTION_STRING=redis://redis-service:6379/0`, `STATUS_CHECK_INTERVAL_SECONDS=5`, `FF_DAILY_STATS_ENABLED=true`, Postgres uri from secret `postgres-app-duw-stats`, telegram creds from `queue-monitor-external-secret`. 100m/128Mi. |
| telegram-bot Deployment | `telegram-bot-deployment.yml` | replicas 1, `Recreate`, creds from `telegram-bot-external-secret` (incl. `NOTIFICATION_TELEGRAM_FEEDBACK_CHAT_ID`). |
| 3 CronJobs | `queue-stats-reports-cronjob.yml` | daily `0 18 * * 1-5`, weekly `5 18 * * 5`, monthly `10 18 28-31 * *` (shell guard), all `Europe/Warsaw`, `Forbid`, `suspend:false`. See `04-stats-reports.md`. |
| CNPG Cluster | `postgres/postgres-cluster.yml` | `kind: Cluster` name **`postgres`**, `instances:1`, db `duw_stats`, owner `duw_stats_admin`, bootstrap secret `postgres-external-secret`, storage 1Gi class `postgres-storage`. Service `postgres-rw:5432`. |
| migrations Job | `postgres/migrations-job.yml` | `duw-migrations-1-1-0`, goose up, `GOOSE_DBSTRING` from `postgres-app-duw-stats:uri`. |
| Redis | `redis/redis-deployment.yml` + `-service.yml` + `-config-map.yml` | Deployment **`redis-storage`** (`redis:8.0.3-alpine`), Service **`redis-service`** (ClusterIP :6379), `emptyDir` data (ephemeral), config disables RDB+AOF (`save ""`, `appendonly no`, `protected-mode no`). |
| StorageClass | `postgres/postgres-storageclass.yml` | `postgres-storage`, Azure provisioner `disk.csi.azure.com` (base default; OVH overlays replace it — see below). |
| ExternalSecrets | `*-external-secret.yml`, `postgres/postgres-*-external-secret.yml` | all reference `SecretStore infisical-secret-store`, `refreshInterval 1h`. |

### Secrets (Infisical via External Secrets Operator)

All `ExternalSecret`s pull from `SecretStore` **`infisical-secret-store`**. Paths:
- `/telegram/NOTIFICATION_TELEGRAM_BOT_TOKEN`, `/telegram/NOTIFICATION_TELEGRAM_BROADCAST_CHANNEL_NAME`, `/telegram/NOTIFICATION_TELEGRAM_FEEDBACK_CHAT_ID`.
- `/postgres/POSTGRES_USER`, `/postgres/POSTGRES_PASSWORD`.
- `postgres-app-duw-stats` is a **templated** ExternalSecret (`creationPolicy: Owner`) that builds `uri = postgresql://{{.username|urlquery}}:{{.password|urlquery}}@postgres-rw:5432/duw_stats` (`postgres-app-external-secret.yml`). CNPG does NOT auto-create this `-app` secret when a custom `initdb.secret` is used — it must exist as its own ExternalSecret (project memory).

### Overlays (`overlays/`)

Each overlay rewrites the base ACR image names to real registries via the kustomize `images:` block and patches storage/env.

| Overlay | Registry | queue-monitor tag | Storage patch | env patch |
|---------|----------|-------------------|---------------|-----------|
| **ovh-prd** (live prod) | `ghcr.io/uladzk/duw-queue-monitor/*` | 1.5.1 | delete Azure `postgres-storage` SC; CNPG storageClass → **`csi-cinder-classic`** | `WORKING_HOUR_END_UTC=17` |
| **ovh-dev** (live dev) | `ghcr.io/uladzk/duw-queue-monitor/*` | 1.4.0 | same as prd | `LOG_LEVEL=debug`, `STATUS_CHECK_INTERVAL_SECONDS=120` |
| **azure-prd** (inactive) | ACR `acrduwshared.azurecr.io/*` | 1.4.0 | none (uses base Azure SC) | none |
| **azure-dev** (inactive) | ACR | 1.4.0 | none | `LOG_LEVEL=debug`, `STATUS_CHECK_INTERVAL_SECONDS=120` |

telegram-bot 1.0.0, queue-stats-reports 1.1.0, duw-migrations 1.1.0 across all overlays. The image name mapping is `acrduwshared.azurecr.io/<svc>` → `ghcr.io/uladzk/duw-queue-monitor/<svc>` — the base uses ACR strings purely as kustomize match keys.

The OVH storage swap is two patches (`overlays/ovh-*/patches/`): `storageclass.yaml` uses `$patch: delete` to drop the Azure `StorageClass`, and `cnpg-storageclass.yaml` patches the CNPG Cluster's `spec.storage.storageClass` to `csi-cinder-classic` (OVH Cinder).

## Terraform (`infra/terraform`)

```mermaid
flowchart TB
  subgraph OVH["OVH (active)"]
    PS["platform-shared\nS3 state bucket 'duw-tfstate-shared',\nS3 user+creds, budget alert 15 EUR"]
    MKS["mks\ncluster mks-duw-{env}-waw,\nfixed nodepool (autoscale off)"]
    K8S["k8s\nESO helm v0.18.2 + Infisical SecretStore,\nCNPG helm v0.23.0"]
  end
  PS -.->|state backend| MKS
  MKS -->|remote_state kubeconfig| K8S
  subgraph AZ["Azure (preserved, inactive)"]
    APS["platform-shared: ACR acrduwshared,\nGHA OIDC fed-identity, tfstate storage"]
    AKS["aks: cluster aks-duw-{env}-plc"]
    AK8S["k8s: ESO + CNPG (same pattern)"]
  end
```

### OVH modules (active)

- **`ovh/platform-shared`** (`main.tf`, `alerting.tf`): OVH Cloud Project user with `objectstore_operator` role + S3 credentials + Object Storage bucket **`duw-tfstate-shared`** (the terraform state backend), plus an S3 access policy. `ovh_cloud_project_alerting` sets a monthly budget alert (`var.monthly_threshold_eur`, default 15 EUR, `delay 86400`). Provider `ovh/ovh 2.13.1`, `endpoint ovh-eu`, terraform ≥1.12.2. Chicken-and-egg: the very first apply must use local state, then migrate to S3 (`provision-ovh.sh` header comment; project memory).
- **`ovh/mks`** (`main.tf`): `ovh_cloud_project_kube` named **`mks-duw-${env}-waw`** + one `ovh_cloud_project_kube_nodepool` `default-pool` with **`autoscale = false`** and `min=max=desired=var.node_count` (prd: single `d2-4` node per project memory). Outputs `cluster_id`, `cluster_endpoint`, and a sensitive `kubeconfig` consumed by the k8s layer. Backend `s3`.
- **`ovh/k8s`** (`main.tf`, `data.tf`, `provider.tf`): reads the MKS kubeconfig via `terraform_remote_state` (`data.tf`, bucket `duw-tfstate-shared`, key `mks.${environment}.tfstate`, S3 endpoint `https://s3.waw.io.cloud.ovh.net`). Installs **ESO helm `v0.18.2`** (ns `external-secrets`) + the Infisical `SecretStore` (templated `resources/eso-infisical-secret-store.yml`, `environmentSlug` from `var.environment` — this scopes dev vs prd secrets and prevents cross-env leaks) and **CNPG helm `0.23.0`** (ns `cnpg-system`). k8s/helm providers pinned `2.37.1` / `3.0.2`.

### Azure modules (preserved, inactive — do not apply)

- `azure/platform-shared`: ACR **`acrduwshared`** (Basic), `ug-aks-admins` group, GitHub Actions OIDC federated identity (`gha-publisher-uladzk-duw-queue-monitor`, subject `repo:uladzk/duw-queue-monitor:ref:refs/heads/main`), tfstate storage account. Location `Poland Central`.
- `azure/aks`: `aks-duw-{env}-plc` cluster, Azure-RBAC, AcrPull for the kubelet identity.
- `azure/k8s`: same ESO+CNPG pattern as OVH.

These are on `main` intentionally (migrated-away cloud, code kept). Treat as reference; runtime is gone.

## Provisioning scripts (`infra/scripts`)

- `provision-ovh.sh <module> <env> [--skip-infisical-login] [-destroy]` — wraps Infisical + terraform for `infra/terraform/ovh/<module>`; needs `envs/<env>/backend.hcl`; injects OVH API + S3 creds from Infisical (project `145e0d1a-...`). No cloud CLI dependency.
- `provision.sh <module> <env> [...]` — Azure equivalent (Infisical + az CLI + terraform). Inactive alongside the Azure modules.

Neither script is a CI step; both are run manually. DUW-93 (backlog) tracks moving terraform to GitOps and splitting `ovh/k8s` into operators + secret-stores to retire a two-pass `-target` apply.

## Local run

`cmd/queuemonitor/docker-compose.dev.yml` brings up `redis:8.0.3-alpine`, `postgres:17-alpine` (db `duw_stats`, user `duw_stats_admin`/`localdev`), and a `migrations` service (builds `../../db`, runs goose up against local Postgres). See `07-ci-cd.md` for building/running the service binaries.
