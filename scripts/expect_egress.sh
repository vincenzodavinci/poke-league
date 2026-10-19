#!/usr/bin/env bash
# Usage: expect_egress.sh
#
# The board's NetworkPolicies, tested with real traffic. A pod that wears the
# board's label in pallet-town (where gyms only warn, so it can run) must reach
# the API server and must NOT reach the internet over HTTPS.
#
# Positive control: the same request from the `default` namespace (no
# NetworkPolicies) must succeed, or "blocked" might just mean "offline".
#
# Timing matters. kindnet's policy engine learns a new pod's IP a few seconds
# after the pod starts, and until then lets its traffic through (it fails
# open). A probe that curls the moment it starts measures that window, not the
# policy. So the probes are long-lived pods: wait until the block is in force
# (at most 60s), then require it to hold. A policy that never takes effect
# still fails.
set -euo pipefail
ctx=${CTX:-kind-kanto}
img=curlimages/curl:8.11.1
outside=https://example.com
k() { kubectl --context "$ctx" "$@"; }
trap 'k -n pallet-town delete pod egress-probe --wait=false >/dev/null 2>&1 || true
      k -n default delete pod egress-control --wait=false >/dev/null 2>&1 || true' EXIT

start() { # start <namespace> <name> <labels>
  k -n "$1" run "$2" --restart=Never --image="$img" --labels="$3" --command -- sleep 600 >/dev/null 2>&1
  k -n "$1" wait --for=condition=Ready "pod/$2" --timeout=120s >/dev/null
}
reaches() { # reaches <namespace> <pod> <url>: exit 0 if an HTTPS connection got any answer
  k -n "$1" exec "$2" -- curl -sk -o /dev/null --connect-timeout 4 --max-time 8 "$3" >/dev/null 2>&1
}

start default egress-control egress-probe=control
start pallet-town egress-probe app.kubernetes.io/name=league-board

if ! reaches default egress-control "$outside"; then
  echo "::error::positive control: $outside is unreachable even without NetworkPolicies; can't test egress"; exit 1
fi
blocked=false
for _ in $(seq 1 12); do
  if ! reaches pallet-town egress-probe "$outside"; then blocked=true; break; fi
  sleep 5
done
if ! $blocked || reaches pallet-town egress-probe "$outside"; then
  echo "::error::a pod labelled as the board reaches $outside: egress isn't pinned to the API server"; exit 1
fi
if ! reaches pallet-town egress-probe https://kubernetes.default.svc/version; then
  echo "::error::the board's egress policy blocks the API server"; exit 1
fi
echo "egress as expected: the board reaches the API server and nothing else (internet reachable from default)"
