#!/usr/bin/env python3
"""Copy deploy/policies to .build/policies with the league's cosign public key
rendered into Koga's ImageValidatingPolicy.

Fails loudly instead of producing a policy set with a gym missing: a gate that
silently isn't there is worse than no gate, because everyone assumes it is.
"""
import os
import shutil
import sys

ROOT = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
SRC = os.path.join(ROOT, "deploy", "policies")
OUT = os.path.join(ROOT, ".build", "policies")
KEY = os.path.join(ROOT, ".cosign", "cosign.pub")
KOGA = os.path.join("gyms", "05-koga-signed-images.yaml")
PLACEHOLDER = "__LEAGUE_COSIGN_PUBLIC_KEY__"


def main():
    with open(KEY) as f:
        key = f.read().strip()
    if "BEGIN PUBLIC KEY" not in key:
        sys.exit(f"{KEY} is not a PEM public key")

    shutil.rmtree(OUT, ignore_errors=True)
    shutil.copytree(SRC, OUT)

    path = os.path.join(OUT, KOGA)
    with open(path) as f:
        policy = f.read()
    if policy.count(PLACEHOLDER) != 1:
        sys.exit(f"{KOGA}: expected exactly one {PLACEHOLDER}")
    # The key sits in a YAML block scalar, so every PEM line needs its indent.
    line = next(l for l in policy.splitlines() if PLACEHOLDER in l)
    indent = line[: len(line) - len(line.lstrip())]
    policy = policy.replace(PLACEHOLDER, key.replace("\n", "\n" + indent))
    with open(path, "w") as f:
        f.write(policy)
    print(f"rendered {os.path.relpath(path, ROOT)} with {os.path.relpath(KEY, ROOT)}")


if __name__ == "__main__":
    main()
