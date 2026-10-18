#!/usr/bin/env bash
# Usage: expect_denied.sh <name> "<gyms>" < manifest.yaml
#
# Server-side dry run of a manifest, which must already target indigo-plateau.
# Passes only if admission control refused it, and the gyms that refused it are
# exactly <gyms> (space-separated, alphabetical: e.g. "erika koga").
# Anything else fails, including a request that errored for an unrelated
# reason: a security check that can't tell "blocked" from "broken" must fail.
#
# "Exactly" means the gyms Kubernetes reached. Koga (an ImageValidatingPolicy)
# runs in a separate webhook from the other seven, and the first webhook to
# deny ends the request, so Koga shows up only when it's the one gym failing.
# That's why the Arbok tests matter: there, Koga alone has to catch it.
set -euo pipefail
name=$1 want=$2
ctx=${CTX:-kind-kanto}
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT

if kubectl --context "$ctx" apply --dry-run=server -f - >"$tmp/out" 2>"$tmp/err"; then
  echo "::error::$name was admitted to indigo-plateau"; cat "$tmp/out"; exit 1
fi
if ! grep -q 'denied the request' "$tmp/err"; then
  echo "::error::$name failed, but not because a gym denied it:"; cat "$tmp/err"; exit 1
fi
if grep -q '^deployment.apps/' "$tmp/out"; then
  echo "::error::$name's Deployment was admitted"; cat "$tmp/out"; exit 1
fi
got=$(grep -oE 'Policy [a-z]+-[a-z-]+-enforce' "$tmp/err" | sed -E 's/Policy ([a-z]+)-.*/\1/' | sort -u | xargs)
if [ "$got" != "$want" ]; then
  echo "::error::$name: expected denials from [$want], got [$got]"; cat "$tmp/err"; exit 1
fi
echo "denied as expected: $name by [$got]"
