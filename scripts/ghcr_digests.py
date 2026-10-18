#!/usr/bin/env python3
"""Look up the digests of the images CI published to GHCR for one commit.

  ghcr_digests.py ghcr.io/<owner>/<repo> <tag> dex-api sprite-svc league-board
  ghcr_digests.py --print ghcr.io/<owner>/<repo> <tag> dex-api    (just print it)

Writes .build/digests/<app> (the same files `make images` writes), so
`make deploy` pins prod to exactly the bytes Lance built and signed.
Asks the registry itself, anonymously: the packages are public, and a
registry's answer is the only digest that counts (a local `docker inspect`
can disagree with it). Stdlib only.
"""
import json
import os
import ssl
import sys
import urllib.error
import urllib.request

ROOT = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
OUT = os.path.join(ROOT, ".build", "digests")
ACCEPT = ", ".join([
    "application/vnd.oci.image.index.v1+json",
    "application/vnd.docker.distribution.manifest.list.v2+json",
    "application/vnd.oci.image.manifest.v1+json",
    "application/vnd.docker.distribution.manifest.v2+json",
])


def _tls_context():
    # python.org builds on macOS ship without a CA bundle; fall back to the
    # system one instead of ever disabling verification.
    ctx = ssl.create_default_context()
    if not ctx.get_ca_certs() and os.path.exists("/etc/ssl/cert.pem"):
        ctx.load_verify_locations("/etc/ssl/cert.pem")
    return ctx


TLS = _tls_context()


def digest(repo, tag):
    """repo is <owner>/<repo>/<app>, without the ghcr.io/ prefix."""
    url = f"https://ghcr.io/token?scope=repository:{repo}:pull"
    with urllib.request.urlopen(url, timeout=30, context=TLS) as r:
        token = json.load(r)["token"]
    req = urllib.request.Request(f"https://ghcr.io/v2/{repo}/manifests/{tag}", method="HEAD",
                                 headers={"Authorization": f"Bearer {token}", "Accept": ACCEPT})
    try:
        with urllib.request.urlopen(req, timeout=30, context=TLS) as r:
            d = r.headers["Docker-Content-Digest"]
    except urllib.error.HTTPError as e:
        if e.code in (401, 403, 404):
            sys.exit(f"ghcr.io/{repo}:{tag} not found ({e.code}). CI publishes an image only after a "
                     "green run on main: push, wait for Lance, or set CI_SHA to a published commit.")
        raise
    if not (d and d.startswith("sha256:") and len(d) == 71):
        sys.exit(f"ghcr.io/{repo}:{tag}: unexpected digest {d!r}")
    return d


def main():
    args = sys.argv[1:]
    only_print = args[:1] == ["--print"]
    if only_print:
        args = args[1:]
    if len(args) < 3 or not args[0].startswith("ghcr.io/"):
        sys.exit(__doc__)
    registry, tag, apps = args[0].removeprefix("ghcr.io/").rstrip("/"), args[1], args[2:]
    os.makedirs(OUT, exist_ok=True)
    for app in apps:
        d = digest(f"{registry}/{app}", tag)
        if only_print:
            print(d)
            continue
        with open(os.path.join(OUT, app), "w") as f:
            f.write(d + "\n")
        print(f"    ghcr.io/{registry}/{app}@{d}")


if __name__ == "__main__":
    main()
