#!/usr/bin/env python3
"""Fetch Pokémon artwork, badges, items and Kanto dex data from PokeAPI.

Writes:
  assets/pokemon/<id>.png      official artwork, resized to 256px
  assets/badges/<1-8>.png      Kanto gym badges (Boulder .. Earth)
  assets/items/<name>.png      item sprites
  data/kanto.json        FireRed-accurate dex entries (Gen 3 types,
                               FireRed flavor text)

Re-run with `make assets`. Stdlib only, so it runs anywhere Python 3 does.
"""
import json
import os
import ssl
import subprocess
import sys
import time
import urllib.request
from concurrent.futures import ThreadPoolExecutor

ROOT = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
SPRITES = "https://raw.githubusercontent.com/PokeAPI/sprites/master/sprites"
API = "https://pokeapi.co/api/v2"
KANTO = range(1, 152)
ITEMS = ["poke-ball", "great-ball", "ultra-ball", "master-ball", "exp-share", "rare-candy"]

# Gen 3 used the pre-Fairy type chart; PokeAPI reports these through past_types.
GEN_ORDER = ["generation-i", "generation-ii", "generation-iii", "generation-iv", "generation-v",
             "generation-vi", "generation-vii", "generation-viii", "generation-ix"]


def _tls_context():
    # python.org builds on macOS ship without a CA bundle; fall back to the
    # system one instead of ever disabling verification.
    ctx = ssl.create_default_context()
    if not ctx.get_ca_certs() and os.path.exists("/etc/ssl/cert.pem"):
        ctx.load_verify_locations("/etc/ssl/cert.pem")
    return ctx


TLS = _tls_context()


def get(url, retries=4):
    for attempt in range(retries):
        try:
            req = urllib.request.Request(url, headers={"User-Agent": "poke-league-asset-fetcher"})
            with urllib.request.urlopen(req, timeout=30, context=TLS) as r:
                return r.read()
        except Exception:
            if attempt == retries - 1:
                raise
            time.sleep(1 + attempt)


def save(path, data):
    os.makedirs(os.path.dirname(path), exist_ok=True)
    with open(path, "wb") as f:
        f.write(data)


def fetch_art(i):
    path = os.path.join(ROOT, "assets", "pokemon", f"{i}.png")
    if os.path.exists(path):
        return
    save(path, get(f"{SPRITES}/pokemon/other/official-artwork/{i}.png"))
    # macOS ships sips; elsewhere the full-size art is kept as-is.
    subprocess.run(["sips", "-Z", "256", path], capture_output=True)


def gen3_types(p):
    """Types as they were in FireRed (e.g. Clefairy is Normal, not Fairy)."""
    types = [t["type"]["name"] for t in sorted(p["types"], key=lambda t: t["slot"])]
    g3 = GEN_ORDER.index("generation-iii")
    # past_types entries mean "these were the types up to and including <generation>";
    # the earliest entry that still covers Gen 3 wins.
    for past in sorted(p.get("past_types", []), key=lambda x: GEN_ORDER.index(x["generation"]["name"])):
        if GEN_ORDER.index(past["generation"]["name"]) >= g3:
            types = [t["type"]["name"] for t in sorted(past["types"], key=lambda t: t["slot"])]
            break
    return types


def fetch_entry(i):
    p = json.loads(get(f"{API}/pokemon/{i}"))
    s = json.loads(get(f"{API}/pokemon-species/{i}"))
    flavor = next((f["flavor_text"] for f in s["flavor_text_entries"]
                   if f["language"]["name"] == "en" and f["version"]["name"] == "firered"), "")
    return {
        "id": i,
        "name": p["name"],
        "types": gen3_types(p),
        "stats": {st["stat"]["name"]: st["base_stat"] for st in p["stats"]},
        "height": p["height"],
        "weight": p["weight"],
        "flavor": " ".join(flavor.split()),
    }


def main():
    with ThreadPoolExecutor(8) as ex:
        list(ex.map(fetch_art, KANTO))
    for b in range(1, 9):
        save(os.path.join(ROOT, "assets", "badges", f"{b}.png"), get(f"{SPRITES}/badges/{b}.png"))
    for it in ITEMS:
        save(os.path.join(ROOT, "assets", "items", f"{it}.png"), get(f"{SPRITES}/items/{it}.png"))
    with ThreadPoolExecutor(8) as ex:
        dex = list(ex.map(fetch_entry, KANTO))
    out = os.path.join(ROOT, "data", "kanto.json")
    os.makedirs(os.path.dirname(out), exist_ok=True)
    with open(out, "w") as f:
        json.dump(dex, f, indent=1, ensure_ascii=False)
    print(f"ok: {len(dex)} dex entries, artwork for {len(KANTO)} Pokémon, 8 badges, {len(ITEMS)} items")


if __name__ == "__main__":
    sys.exit(main())
