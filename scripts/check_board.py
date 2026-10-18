#!/usr/bin/env python3
"""Wait until every league workload (everything outside team-rocket) holds 8/8.

  check_board.py [--image-prefix ghcr.io/<owner>/<repo>/]

With --image-prefix, every league workload must also be running an image from
that registry, so "8/8" can't be earned by something other than what you meant
to test. Reads the League Board's /api/league. Stdlib only.
"""
import argparse
import json
import sys
import time
import urllib.request

BOARD = "http://localhost:8088/api/league"


def problems(prefix):
    with urllib.request.urlopen(BOARD, timeout=10) as r:
        league = [w for w in json.load(r)["workloads"] if w["namespace"] != "team-rocket"]
    if not league:
        return ["the board shows no league workloads yet"]
    out = []
    for w in league:
        where = f"{w['namespace']}/{w['name']}"
        if w["earned"] != 8:
            out.append(f"{where}: {w['earned']}/8 badges")
        if prefix and not w["image"].startswith(prefix):
            out.append(f"{where}: runs {w['image']}, not an image from {prefix}")
    return out


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--image-prefix", default="")
    ap.add_argument("--timeout", type=int, default=150, help="seconds")
    args = ap.parse_args()
    deadline = time.time() + args.timeout
    while True:
        try:
            found = problems(args.image_prefix)
        except OSError as e:
            found = [f"board unreachable: {e}"]
        if not found:
            print("every league workload holds 8/8" + (f", all from {args.image_prefix}" if args.image_prefix else ""))
            return 0
        if time.time() > deadline:
            print("::error::" + "; ".join(found))
            return 1
        time.sleep(5)


if __name__ == "__main__":
    sys.exit(main())
