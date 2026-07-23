# OVH support-ticket templates

Fill the `{PLACEHOLDERS}` from the probe output, then deliver each via the
`create-quick-copy` skill (write a `/tmp/copy-*.sh` that `pbcopy`s the text).
Keep the copied text byte-identical to what you show the user.

Placeholders:
- `{PROJECT_ID}` — OVH Public Cloud project id (default `89b751600cb74c63ac8f954fa7e151f2`)
- `{NICHANDLE}` — customer id (default `ci162106-ovh`)
- `{REGION}` — e.g. `WAW1`
- `{ALERT_DATE}` — date/time from the alert email
- `{ALERT_REF}` — the `ref=...` in the alert email footer (omit the line if unknown)
- `{CURRENT_TOTAL}` — month-to-date EUR (probe "month-to-date total")
- `{FORECAST_TOTAL}` — projected EUR (probe "forecast total")
- `{INSTANCE_ID}` — the node instance id the forecast over-prices
- `{IMPLIED_HOURLY}` — probe "implied_hourly" for that instance
- `{EXPECTED_MONTH}` — realistic month figure (≈ current scaled to full month, ~EUR 15)

---

## 1. Full ticket body (classic ticket form, or attach as a .txt file)

```
Subject: Incorrect "Estimate of my next invoice" for Public Cloud project
{PROJECT_ID} - projection ~720x too high

Hello,

I'm reporting what appears to be a calculation bug in the "Estimate of my
next invoice" / on-demand usage projection for my Public Cloud project, and
I'd like confirmation that I will not be charged the projected amount.

ACCOUNT / PROJECT
- Customer (nichandle): {NICHANDLE}
- Public Cloud project: {PROJECT_ID}
- Region: {REGION}
- Triggering alert: OVH Public Cloud usage alert, sent {ALERT_DATE}, ref={ALERT_REF}

THE PROBLEM
The usage alert and the "Estimate of my next invoice" page project my invoice
at EUR {FORECAST_TOTAL}, while my actual month-to-date usage is only
EUR {CURRENT_TOTAL}. The projection is roughly 720x too high.

WHY THIS IS A CALCULATION ERROR, NOT REAL USAGE
Via the API endpoint /cloud/project/{id}/usage/forecast, the forecast values
my single Kubernetes node (instance {INSTANCE_ID}) at an implied unit price of
EUR {IMPLIED_HOURLY}/hour. That figure equals my month-to-date TOTAL from
/cloud/project/{id}/usage/current - i.e. the forecast appears to treat the
accumulated month-to-date cost as the hourly rate, then multiply by 720 hours
(a full month). A correct projection would scale the rate by elapsed hours and
land near EUR {EXPECTED_MONTH}.

NO REAL RESOURCE SPIKE EXISTS
The project contains only the expected minimal footprint (one Managed
Kubernetes node, one small block-storage volume, no snapshots, no load
balancers, no extra public IPs). Actual current usage is ~EUR {CURRENT_TOTAL},
tracking to ~EUR {EXPECTED_MONTH} for the month, consistent with prior months.

MY QUESTIONS
1. Please confirm this projection is an estimation/display bug and that my
   actual invoice will reflect real metered usage (~EUR {EXPECTED_MONTH}), not
   the EUR {FORECAST_TOTAL} figure.
2. Can the forecast for this project be corrected? The on-demand usage alert
   re-sends every 24h based on this incorrect estimate.
3. If this is a known issue, is there a tracking reference?

Thank you.
```

---

## 2. Short version (for the beta AI-assistant chat box)

```
Billing estimate bug - Public Cloud project {PROJECT_ID}, nichandle {NICHANDLE}, region {REGION}.

The "Estimate of my next invoice" projects EUR {FORECAST_TOTAL}, but my real month-to-date usage is only EUR {CURRENT_TOTAL}. The forecast API values my single node ({INSTANCE_ID}) at EUR {IMPLIED_HOURLY}/hour - it is treating my month-to-date TOTAL as if it were the hourly rate, then x720h. The project has only 1 node + 1 small disk (no load balancer, no extra public IPs, no snapshots).

Please confirm this is an estimation/display bug and that I will NOT be charged ~EUR {FORECAST_TOTAL} - my actual invoice should be ~EUR {EXPECTED_MONTH}. Can the forecast for this project be corrected? The alert keeps re-sending every 24h. Thank you.
```

In the beta chat: paste this, then reply "please contact support, I want to
file a customer support request" and confirm "Yes, continue ticket creation".

---

## 3. Comment to accompany the attachment (when adding detail to an open ticket)

```
Attaching full technical detail with a reproduction for this billing estimate discrepancy. Summary: the forecast API values my single node at EUR {IMPLIED_HOURLY}/hour (= my month-to-date total treated as the hourly rate), x720h = EUR {FORECAST_TOTAL}. Real usage is ~EUR {CURRENT_TOTAL} and the month should bill ~EUR {EXPECTED_MONTH}. Please confirm in writing that I will NOT be charged the ~EUR {FORECAST_TOTAL} estimate, and whether the forecast for project {PROJECT_ID} can be corrected.
```
