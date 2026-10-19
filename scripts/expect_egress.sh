#!/usr/bin/env bash
# Usage: expect_egress.sh
#
# The board's NetworkPolicies, tested with real traffic. A throwaway pod that
# wears the board's label in pallet-town (where gyms only warn, so it can run)
# must reach the API server and must NOT reach the internet over HTTPS.
#
# Positive control: the same request from the `default` namespace (no
# NetworkPolicies) must succeed, or "blocked" might just mean "offline".
set -euo pipefail
ctx=${CTX:-kind-kanto}
img=curlimages/curl:8.11.1

probe() { # probe <namespace> <labels> <url>: exit 0 if an HTTP(S) connection got any answer
  kubectl --context "$ctx" -n "$1" run "egress-probe-$RANDOM" --rm -i --quiet --restart=Never \
    --image="$img" --labels="$2" --command -- \
    curl -sk -o /dev/null --connect-timeout 5 --max-time 10 "$3" >/dev/null 2>&1
}

outside=https://example.com
if ! probe default "egress-probe=control" "$outside"; then
  echo "::error::positive control: $outside is unreachable even without NetworkPolicies; can't test egress"; exit 1
fi
board="app.kubernetes.io/name=league-board"
if ! probe pallet-town "$board" https://kubernetes.default.svc/version; then
  echo "::error::the board's egress policy blocks the API server"; exit 1
fi
if probe pallet-town "$board" "$outside"; then
  echo "::error::a pod labelled as the board reached $outside: egress isn't pinned to the API server"; exit 1
fi
echo "egress as expected: the board reaches the API server and nothing else (internet reachable from default)"
