#!/usr/bin/env bash
# OVH Public Cloud usage probe — investigates a billing/cost alert.
#
# Pulls live current usage, the end-of-month forecast, and a full resource
# inventory for the project, then prints a VERDICT: is this the known OVH
# "forecast bug" (projection = month-to-date total x ~720h) or a real spike?
#
# Auth: signed OVH API requests. Run it THROUGH infisical so the OVH API
# creds are injected as env vars and never printed:
#
#   cd <duw repo>   # any dir works; infisical just needs the project
#   infisical run --projectId=145e0d1a-6378-4338-a9eb-2d77178f96e7 --env=shared \
#     -- bash .claude/skills/duw-ovh-billing-alert-response/scripts/ovh-usage-probe.sh
#
# Required env (provided by infisical): TF_VAR_ovh_application_key/secret/consumer_key
# Override the project with OVH_CLOUD_PROJECT=<id> if it ever changes.
#
# Robustness: the OVH /instance endpoint intermittently returns HTTP 500
# ({"class":"Server::InternalServerError"}). We retry and validate the JSON
# shape so a transient error never miscounts as a "real spike". NOTE: do not
# add `set -e` — a single failed jq must not abort the whole probe.
set -uo pipefail

AK="${TF_VAR_ovh_application_key:?missing OVH application key (run via infisical)}"
AS="${TF_VAR_ovh_application_secret:?missing OVH application secret}"
CK="${TF_VAR_ovh_consumer_key:?missing OVH consumer key}"
BASE="https://eu.api.ovh.com/1.0"
PROJ="${OVH_CLOUD_PROJECT:-89b751600cb74c63ac8f954fa7e151f2}"   # duw-monitor

TS="$(curl -s --retry 3 --retry-delay 1 "$BASE/auth/time")"

# raw signed GET (curl retries 5xx/connection errors with its own backoff)
ovhget() {
  local url="$BASE$1"
  local sig="\$1\$$(printf '%s' "${AS}+${CK}+GET+${url}++${TS}" | shasum -a 1 | awk '{print $1}')"
  curl -s --retry 5 --retry-delay 2 --retry-all-errors --retry-connrefused \
    -H "X-Ovh-Application: $AK" -H "X-Ovh-Consumer: $CK" \
    -H "X-Ovh-Timestamp: $TS" -H "X-Ovh-Signature: $sig" "$url"
}

# fetch with body-level validation: retry if empty or an OVH error object
fetch() {
  local body=""
  for _ in 1 2 3; do
    body="$(ovhget "$1")"
    if [ -n "$body" ] && ! printf '%s' "$body" | jq -e 'objects | (.class // .message) // empty' >/dev/null 2>&1; then
      printf '%s' "$body"; return 0
    fi
  done
  printf '%s' "$body"; return 1   # last (error) body
}

# length only if the value is a JSON array, else -1 (unknown / fetch error)
arr_len() { printf '%s' "$1" | jq 'if type=="array" then length else -1 end' 2>/dev/null || echo -1; }

echo "############ ACCOUNT ############"
fetch "/me" | jq '{nichandle, state, country}' 2>/dev/null || echo "(me failed)"

CUR="$(fetch "/cloud/project/$PROJ/usage/current")"
FC="$(fetch "/cloud/project/$PROJ/usage/forecast")"
INST="$(fetch "/cloud/project/$PROJ/instance")";   INST_OK=$?
VOL="$(fetch "/cloud/project/$PROJ/volume")";       VOL_OK=$?
SNAP="$(fetch "/cloud/project/$PROJ/snapshot")";    SNAP_OK=$?
KUBE="$(fetch "/cloud/project/$PROJ/kube")"

echo; echo "############ CURRENT USAGE (month-to-date) ############"
echo "$CUR" | jq '{
  lastUpdate,
  instances: (.hourlyUsage.instance // [] | map({ref:.reference, region, price:.totalPrice})),
  volumes:   (.hourlyUsage.volume   // [] | map({type, region, price:.totalPrice})),
  storage:   (.hourlyUsage.storage  // [] | map(select(.totalPrice>0) | {bucket:.bucketName, price:.totalPrice}))
}' 2>/dev/null || echo "(current parse failed)"

echo; echo "############ FORECAST (OVH month-end projection) ############"
echo "$FC" | jq '{
  forecast_total: .totalPrice.value,
  instance_forecast: (.hourlyUsage.instance // [] | map({ref:.reference, hours:.quantity.value, price:.totalPrice, implied_hourly:(if .quantity.value>0 then (.totalPrice/.quantity.value) else null end)}))
}' 2>/dev/null || echo "(forecast parse failed)"

echo; echo "############ INVENTORY ############"
N_INST="$(arr_len "$INST")"; N_VOL="$(arr_len "$VOL")"; N_SNAP="$(arr_len "$SNAP")"; N_KUBE="$(arr_len "$KUBE")"
echo "instances: $N_INST$([ "$INST_OK" -ne 0 ] && echo '  (FETCH ERROR - OVH API returned an error)')"
printf '%s' "$INST" | jq 'if type=="array" then map({id, name, flavor:(.flavor.name // .planCode // "?"), region, status}) else . end' 2>/dev/null
echo "volumes:   $N_VOL"; printf '%s' "$VOL" | jq 'if type=="array" then map({name, size, type, region, status}) else . end' 2>/dev/null
echo "snapshots: $N_SNAP"
echo "kube:      $N_KUBE"

echo; echo "############ VERDICT ############"
CUR_TOTAL="$(echo "$CUR" | jq '[(.hourlyUsage.instance//[]),(.hourlyUsage.volume//[]),(.hourlyUsage.storage//[])] | flatten | map(.totalPrice // 0) | add // 0' 2>/dev/null || echo 0)"
FC_TOTAL="$(echo "$FC" | jq '.totalPrice.value // 0' 2>/dev/null || echo 0)"
# implied hourly rate the forecast assigns the instance — the real bug signal.
# Real d2-4 ~ EUR 0.0206/h; the bug sets this to the month-to-date TOTAL (~EUR 11+/h).
IMPLIED="$(printf '%s' "$FC" | jq '[.hourlyUsage.instance[]? | select((.quantity.value//0)>0) | (.totalPrice/.quantity.value)] | max // 0' 2>/dev/null || echo 0)"
GPU="$(printf '%s' "$INST" | jq '[.[]? | select((.flavor.name // "") | test("t1|t2|a10|a100|h100|l4|l40|gpu";"i"))] | length' 2>/dev/null || echo 0)"

python3 - "$CUR_TOTAL" "$FC_TOTAL" "$IMPLIED" "$N_INST" "$N_SNAP" "$N_VOL" "$GPU" <<'PY'
import sys
cur, fc, implied = float(sys.argv[1]), float(sys.argv[2]), float(sys.argv[3])
n_inst, n_snap, n_vol, gpu = (int(sys.argv[4]), int(sys.argv[5]), int(sys.argv[6]), int(sys.argv[7]))
ratio = (fc/cur) if cur > 0 else float('inf')
print(f"month-to-date total : EUR {cur:.4f}")
print(f"forecast total      : EUR {fc:.2f}")
print(f"forecast / current  : {ratio:.0f}x")
print(f"forecast hourly rate : EUR {implied:.4f}/h  (real d2-4 ~ EUR 0.0206/h; the bug shows the month-to-date total here)")
print(f"instances={n_inst}  volumes={n_vol}  snapshots={n_snap}  gpu_flavors={gpu}")
print()
baseline_ok = (n_inst <= 1 and n_snap == 0 and n_vol <= 1 and gpu == 0)
if -1 in (n_inst, n_snap, n_vol):
    print("=> INCONCLUSIVE — the OVH API returned an error for part of the inventory")
    print("   (a transient HTTP 500 on /instance is common). Re-run the probe before")
    print("   concluding. Cross-check with: kubectl --context mks-duw-prd-waw get nodes")
elif not baseline_ok:
    print("=> POSSIBLE REAL SPIKE — DO NOT DISMISS.")
    print("   Unexpected resources are present (extra instances / GPU / snapshots /")
    print("   volumes). This is NOT the benign forecast bug. Follow SKILL.md Step 4")
    print("   (real-spike / possible-compromise response): identify & stop rogue")
    print("   resources, rotate creds, then contact OVH.")
elif implied >= 0.5:
    print("=> LIKELY OVH FORECAST BUG (benign).")
    print(f"   Inventory is the expected minimal footprint, but the forecast prices the")
    print(f"   node at EUR {implied:.2f}/h (~ the month-to-date total) instead of ~EUR 0.02/h.")
    print("   The real invoice is metered hourly = current usage (~EUR 15/mo). Proceed to")
    print("   build the support-ticket artifacts (see SKILL.md Step 2).")
else:
    print("=> FORECAST HEALTHY — no inflation.")
    print(f"   Inventory is the expected minimal footprint and the forecast hourly rate")
    print(f"   (EUR {implied:.4f}/h) matches the real instance price; the projection (EUR {fc:.2f})")
    print("   tracks real usage (~EUR 15/mo). If you were tracking the forecast bug, it")
    print("   appears RESOLVED — the support ticket can be closed.")
PY
