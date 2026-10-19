#!/usr/bin/env bash
# Usage: expect_debug_denied.sh
#
# `kubectl debug` attaches an ephemeral container to a running pod through the
# pods/ephemeralcontainers endpoint, which pod-creation admission never sees.
# Prod must refuse it (indigo-no-debug); dev must allow it, which is the
# positive control: a request that's merely broken can't pass as a denial.
#
# The debug container is fully "restricted"-compliant, so in prod the only
# thing that can refuse it is the policy, not Pod Security Admission.
set -euo pipefail
ctx=${CTX:-kind-kanto}
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT

attempt() { # server-side dry run of adding a debug container to a dex-api pod in $1
  local ns=$1 pod
  pod=$(kubectl --context "$ctx" -n "$ns" get pod -l app.kubernetes.io/name=dex-api -o jsonpath='{.items[0].metadata.name}')
  kubectl --context "$ctx" -n "$ns" get pod "$pod" -o json | python3 -I -c '
import json, sys
p = json.load(sys.stdin)
p["spec"].setdefault("ephemeralContainers", []).append({
    "name": "rocket-debug", "image": "busybox:1.36", "command": ["sleep", "3600"], "targetContainerName": "app",
    "securityContext": {"allowPrivilegeEscalation": False, "runAsNonRoot": True, "runAsUser": 65532,
                        "capabilities": {"drop": ["ALL"]}, "seccompProfile": {"type": "RuntimeDefault"}}})
json.dump(p, sys.stdout)' >"$tmp/pod.json"
  kubectl --context "$ctx" replace --raw "/api/v1/namespaces/$ns/pods/$pod/ephemeralcontainers?dryRun=All" \
    -f "$tmp/pod.json" >"$tmp/out" 2>"$tmp/err"
}

if ! attempt pallet-town; then
  echo "::error::positive control: adding a debug container in pallet-town failed:"; cat "$tmp/err"; exit 1
fi
if attempt indigo-plateau; then
  echo "::error::a debug container was attached to a pod in indigo-plateau"; exit 1
fi
if ! grep -q 'denied the request' "$tmp/err" || ! grep -q 'indigo-no-debug-enforce' "$tmp/err"; then
  echo "::error::kubectl debug in indigo-plateau failed, but not because indigo-no-debug denied it:"; cat "$tmp/err"; exit 1
fi
echo "denied as expected: kubectl debug in indigo-plateau by [indigo-no-debug] (and allowed in pallet-town)"
