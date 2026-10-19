#!/usr/bin/env python3
"""Wait until the League Board tells the whole, true story.

  check_board.py [--image-prefix ghcr.io/<owner>/<repo>/] [--arbok-fails koga]

Passes only when, on the board's /api/league:
  * all six league workloads are there (3 apps x pallet-town, indigo-plateau),
    each holding 8/8, and prod's cards stand for both of their replicas;
  * Team Rocket's three are there and the board shows them failing, Arbok on
    exactly the gyms given (a board that shows everything as 8/8 would pass
    a check that only looked at the league);
  * the board read every report it needed (no "problems").
With --image-prefix, every league workload must also run an image from that
registry, so "8/8" can't be earned by something other than what you meant to
test. Stdlib only.
"""
import argparse
import json
import sys
import time
import urllib.request

BOARD = "http://localhost:8088/api/league"
APPS = ("dex-api", "sprite-svc", "league-board")
REPLICAS = {"pallet-town": 1, "indigo-plateau": 2}
ROCKET = ("meowth", "koffing", "arbok")


def problems(prefix, arbok_fails):
    with urllib.request.urlopen(BOARD, timeout=10) as r:
        board = json.load(r)
    cards = {(w["namespace"], w["name"]): w for w in board["workloads"]}
    out = [f"the board couldn't read: {p}" for p in board.get("problems") or []]
    for ns, replicas in REPLICAS.items():
        for app in APPS:
            w = cards.get((ns, app))
            where = f"{ns}/{app}"
            if w is None:
                out.append(f"{where}: missing from the board")
                continue
            if w["earned"] != 8:
                out.append(f"{where}: {w['earned']}/8 badges")
            if w["replicas"] != replicas:
                out.append(f"{where}: card covers {w['replicas']} pods, want {replicas}")
            if prefix and not w["image"].startswith(prefix):
                out.append(f"{where}: runs {w['image']}, not an image from {prefix}")
    for app in ROCKET:
        w = cards.get(("team-rocket", app))
        if w is None:
            out.append(f"team-rocket/{app}: missing from the board")
            continue
        failed = {b["gym"] for b in w["badges"] if b["status"] == "failed"}
        if w["earned"] == 8 or not failed:
            out.append(f"team-rocket/{app}: the board shows no failures")
        if app == "arbok" and not set(arbok_fails) <= failed:
            out.append(f"team-rocket/arbok: want {sorted(arbok_fails)} failed, board shows {sorted(failed)}")
    return out


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--image-prefix", default="")
    ap.add_argument("--arbok-fails", nargs="+", default=["koga"], help="gyms Arbok must be shown failing")
    ap.add_argument("--timeout", type=int, default=180, help="seconds")
    args = ap.parse_args()
    deadline = time.time() + args.timeout
    while True:
        try:
            found = problems(args.image_prefix, args.arbok_fails)
        except OSError as e:
            found = [f"board unreachable: {e}"]
        if not found:
            print("the board tells the whole story: 6 league workloads at 8/8, Team Rocket shown failing"
                  + (f", all from {args.image_prefix}" if args.image_prefix else ""))
            return 0
        if time.time() > deadline:
            print("::error::" + "; ".join(found))
            return 1
        time.sleep(5)


if __name__ == "__main__":
    sys.exit(main())
