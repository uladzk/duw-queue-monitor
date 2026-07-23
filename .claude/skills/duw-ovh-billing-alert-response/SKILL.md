---
name: duw-ovh-billing-alert-response
description: >-
  Use when an OVH Cloud billing/cost alert fires or an OVH "Estimate of my next
  invoice" looks wrong — e.g. an email like "[ci162106-ovh] Public Cloud OVH:
  Alert dotyczący wykorzystanych zasobów dla duw-monitor", or the dashboard
  projecting a huge next invoice (hundreds/thousands of EUR) while real usage is
  tiny. Investigates whether it is the known OVH forecast bug (projection ≈
  month-to-date total × ~720h) or a genuine cost spike / account compromise — by
  inspecting the Kubernetes cluster and querying the OVH usage API — then
  generates copy-ready customer-support artifacts (full ticket body, short
  chat-box version, and an attachment comment). Trigger this whenever the user
  mentions an OVH billing/cost alert, an inflated or wrong OVH invoice estimate,
  unexpected OVH charges, the duw-monitor project bill, or asks "why is my OVH
  projection so high / is this a bug" — even if they don't name the skill.
---

# OVH billing alert response

## What's going on (the known pattern)

OVH's "Estimate of my next invoice" / on-demand usage alert has a forecast bug:
it can take the **month-to-date accumulated cost** of an instance, treat it as
the **hourly rate**, and multiply by ~720 (hours in a month). For the
`duw-monitor` project — one cheap `d2-4` Kubernetes node, real cost ~€15/month —
this produces an absurd projection (e.g. €11 month-to-date → €7,964 projected).
The **real invoice is metered hourly** and tracks actual usage, so the estimate
is cosmetic. This was first diagnosed 2026-06-23 (OVH ticket `CS16080468`).

**But never assume.** A sudden spike *can* be real (autoscaled/rogue instances,
GPU VMs from a compromised account, large egress, snapshots). So always triage
before reassuring. The probe in Step 1 distinguishes the two automatically.

## Project constants

| Thing | Value |
|---|---|
| kube context | `mks-duw-prd-waw` |
| OVH endpoint | `ovh-eu` → `https://eu.api.ovh.com/1.0` |
| OVH project (service_name) | `89b751600cb74c63ac8f954fa7e151f2` (`duw-monitor`) |
| nichandle | `ci162106-ovh` |
| Infisical (OVH API creds) | project `145e0d1a-6378-4338-a9eb-2d77178f96e7`, env `shared`, vars `TF_VAR_ovh_application_key/secret/consumer_key` |
| Expected baseline | 1× `d2-4` node, 1× 1Gi volume, no LoadBalancers, no snapshots; workloads: queue-monitor, telegram-bot, CNPG postgres, redis, external-secrets, stats CronJobs, duw-doctor-checker |
| Alert config (terraform) | `infra/terraform/ovh/platform-shared/envs/shared/shared.tfvars` → `monthly_threshold_eur`; resource in `.../platform-shared/alerting.tf`, `delay=86400` (24h re-send) |

## Step 1 — Triage: forecast bug or real spike?

Run both checks. They're read-only.

**1a. Cluster** (rogue pods / extra nodes / LoadBalancers cost money):
```bash
kubectl --context mks-duw-prd-waw get nodes -o wide
kubectl --context mks-duw-prd-waw get pods -A -o wide
kubectl --context mks-duw-prd-waw get svc -A          # any type=LoadBalancer?
kubectl --context mks-duw-prd-waw get pvc -A
```
Expect exactly the baseline above. Extra nodes, unknown pods, or a LoadBalancer
service are red flags.

**1b. OVH project** (this is the authoritative check — sees resources kubectl
can't, like out-of-band VMs). Run the bundled probe through infisical so the API
secrets are injected, never printed:
```bash
infisical run --projectId=145e0d1a-6378-4338-a9eb-2d77178f96e7 --env=shared \
  -- bash .claude/skills/duw-ovh-billing-alert-response/scripts/ovh-usage-probe.sh
```
The probe prints current usage, the forecast, a full inventory, and a
**VERDICT**: `LIKELY OVH FORECAST BUG (benign)`, `POSSIBLE REAL SPIKE`, or
`INCONCLUSIVE`.

**Decision rule:** benign only if inventory matches baseline (≤1 instance, ≤1
small volume, 0 snapshots, no GPU) **and** the forecast is a large multiple
(>30×) of real usage. Anything else → treat as a possible real spike (Step 4).

## Step 2 — If it's the forecast bug: build support artifacts

Capture from the probe: month-to-date total, forecast total, the over-priced
instance id, its implied €/hour, and a realistic month figure (current scaled to
full month, ~€15). Also grab `ALERT_DATE` and the `ref=...` from the alert email
if the user has it.

Read `references/support-templates.md` and fill the placeholders. Then deliver
all three via the **`create-quick-copy`** skill — one `/tmp/copy-*.sh` per
artifact (pbcopy heredoc), shown in chat AND copy-ready:
1. **Full ticket body** — for the classic ticket form, or saved as a `.txt` on
   the Desktop to attach to a ticket.
2. **Short version** — for OVH's beta AI-assistant chat box (long bodies don't
   paste well there).
3. **Attachment comment** — a one-liner to post with the attached `.txt`.

Always show the artifacts in the conversation too — the copy scripts are a faster
delivery channel, not a replacement.

## Step 3 — File with OVH support

OVH's **new beta support UI** is an AI chat ("Describe your issue…"):
1. Paste the **short version**.
2. Reply: "please contact support, I want to file a customer support request."
3. Confirm: "Yes, continue ticket creation." → a ticket `CS…` is created.
4. To add the full detail: use the 📎 box ("Add information about a ticket…") to
   attach the `.txt` and paste the **attachment comment**.

Prefer the **classic form** for the full body: click "Return to classic version"
→ "New request" → paste the full ticket into the description field.

Ask support for **written confirmation tied to the project id** that the actual
invoice will be metered usage (~€15), not the projected figure — the generic
"estimates are informational only" line is not enough.

## Step 4 — If it's a REAL spike (do not reassure)

The probe flagged unexpected resources. Treat as an incident:
1. Identify the rogue resources from the probe inventory (extra/GPU instances,
   snapshots, large volumes, unexpected egress).
2. Cross-check the cluster (Step 1a) — is it autoscaling, a misconfig, or
   out-of-band (not created by terraform/k8s = likely compromise)?
3. If compromise is plausible: stop/delete the rogue resources, **rotate the OVH
   API credentials** (the `TF_VAR_ovh_*` in Infisical) and any leaked cloud
   creds, review IAM/API-key access, and open an OVH support ticket reporting
   suspected fraud. Do **not** tell the user it's a harmless bug.

## Step 5 — Optional follow-ups

- **Mute repeat alerts** (the alert re-sends every 24h on the bad estimate):
  raise `monthly_threshold_eur` in
  `infra/terraform/ovh/platform-shared/envs/shared/shared.tfvars`, then apply:
  `infra/scripts/provision-ovh.sh platform-shared shared`. Keeps the guardrail;
  don't edit it in the dashboard (drifts from terraform). Revert once OVH fixes
  the forecast.
- **Public visibility**: file an issue on `ovh/public-cloud-roadmap` (GitHub) so
  other customers hitting the same `month-to-date × 720` bug can find it.
