#!/usr/bin/env bash
# Usage: expect_admitted.sh <deployment>
#
# The positive control for expect_denied.sh: a gate that refuses everything
# would pass every negative test. Takes a running prod Deployment, renames the
# copy (so it's a CREATE, judged from scratch, not a no-op update) and requires
# a server-side dry run to admit it.
set -euo pipefail
src=$1
ctx=${CTX:-kind-kanto}
kubectl --context "$ctx" -n indigo-plateau get deploy "$src" -o json | python3 -I -c '
import json, sys
d = json.load(sys.stdin)
m = d["metadata"]
d["metadata"] = {"name": "champion-check", "namespace": m["namespace"], "labels": m.get("labels", {})}
d.pop("status", None)
json.dump(d, sys.stdout)' | kubectl --context "$ctx" apply --dry-run=server -f - >/dev/null
echo "admitted as expected: a fresh copy of $src"
