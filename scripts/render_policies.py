#!/usr/bin/env python3
"""Copy deploy/policies to .build/policies with Koga's root of trust rendered in.

  render_policies.py                       key mode: trust the cosign key in .cosign/
  render_policies.py --trust ci \\
      --registry ghcr.io/<owner>/<repo>/ \\
      --identity <workflow identity>        CI mode: trust only what CI signed keylessly

Fails loudly instead of producing a policy set with a gym missing: a gate that
silently isn't there is worse than no gate, because everyone assumes it is.
"""
import argparse
import os
import shutil
import sys

ROOT = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
SRC = os.path.join(ROOT, "deploy", "policies")
OUT = os.path.join(ROOT, ".build", "policies")
KEY = os.path.join(ROOT, ".cosign", "cosign.pub")
KOGA = os.path.join("gyms", "05-koga-signed-images.yaml")
KOGA_CI = os.path.join("ci-trust", "05-koga-signed-images.yaml")
ERIKA = os.path.join("gyms", "04-erika-trusted-images.yaml")
LOCAL_REGISTRY = "kind-registry:5000/poke-league/"
PLACEHOLDERS = ("__LEAGUE_COSIGN_PUBLIC_KEY__", "__CI_IDENTITY__")


def replace(rel, old, new, count=None):
    path = os.path.join(OUT, rel)
    with open(path) as f:
        text = f.read()
    n = text.count(old)
    if n == 0 or (count is not None and n != count):
        sys.exit(f"{rel}: expected {count or 'at least one'} {old!r}, found {n}")
    with open(path, "w") as f:
        f.write(text.replace(old, new))


def render_key():
    with open(KEY) as f:
        key = f.read().strip()
    if "BEGIN PUBLIC KEY" not in key:
        sys.exit(f"{KEY} is not a PEM public key")
    with open(os.path.join(OUT, KOGA)) as f:
        line = next((l for l in f.read().splitlines() if PLACEHOLDERS[0] in l), "")
    # The key sits in a YAML block scalar, so every PEM line needs its indent.
    indent = line[: len(line) - len(line.lstrip())]
    replace(KOGA, PLACEHOLDERS[0], key.replace("\n", "\n" + indent), count=1)
    print(f"rendered {KOGA} to trust {os.path.relpath(KEY, ROOT)}")


def render_ci(registry, identity):
    if not (registry.startswith("ghcr.io/") and registry.endswith("/")):
        sys.exit(f"--registry must look like ghcr.io/<owner>/<repo>/, got {registry!r}")
    if not (identity.startswith("https://github.com/") and "/.github/workflows/" in identity):
        sys.exit(f"--identity must be a GitHub Actions workflow identity, got {identity!r}")
    shutil.copyfile(os.path.join(OUT, KOGA_CI), os.path.join(OUT, KOGA))
    replace(KOGA, PLACEHOLDERS[1], identity, count=1)
    # Erika's allowlist follows the images: in CI mode the league registry is GHCR.
    replace(ERIKA, LOCAL_REGISTRY, registry)
    print(f"rendered {KOGA} to trust {identity}")
    print(f"rendered {ERIKA} to trust {registry}")


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--trust", choices=["key", "ci"], default="key")
    ap.add_argument("--registry")
    ap.add_argument("--identity")
    args = ap.parse_args()
    if args.trust == "ci" and not (args.registry and args.identity):
        ap.error("--trust ci needs --registry and --identity")

    shutil.rmtree(OUT, ignore_errors=True)
    shutil.copytree(SRC, OUT)
    if args.trust == "key":
        render_key()
    else:
        render_ci(args.registry, args.identity)

    # Whatever the mode, no placeholder may survive into a policy that gets applied.
    gyms = os.path.join(OUT, "gyms")
    for name in sorted(os.listdir(gyms)):
        with open(os.path.join(gyms, name)) as f:
            text = f.read()
        for ph in PLACEHOLDERS:
            if ph in text:
                sys.exit(f"gyms/{name}: {ph} was never rendered")


if __name__ == "__main__":
    main()
