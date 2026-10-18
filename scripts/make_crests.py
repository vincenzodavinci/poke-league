#!/usr/bin/env python3
"""Generate original art for the league: one Dex Crest per Pokémon, eight gym
sigils, and the favicon.

Nothing here is traced, recolored or derived from official artwork. Each crest
is computed from facts in data/kanto.json:

  primary type     -> body color          secondary type -> rim and spike color
  HP               -> emblem size         Attack         -> spike count and length
  Defense          -> armor rings         Sp. Atk        -> facets on the core gem
  Sp. Def          -> inner halo          Speed          -> motion arcs
  dex number       -> rotation and small variations, so no two crests match

Output: assets/crests/<id>.svg, assets/badges/<n>.svg, assets/favicon.svg
Stdlib only. Re-run with `make crests`.
"""
import json
import math
import os
import random

ROOT = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))

TYPE = {  # (fill, light, dark)
    "normal": ("#a8a878", "#d8d8b0", "#6d6d4e"), "fire": ("#f08030", "#ffc08a", "#9c531f"),
    "water": ("#6890f0", "#b0c8ff", "#445e9c"), "grass": ("#78c850", "#bfe8a0", "#4e8234"),
    "electric": ("#f8d030", "#fff0a0", "#a1871f"), "ice": ("#98d8d8", "#d8f4f4", "#638d8d"),
    "fighting": ("#c03028", "#f08080", "#7d1f1a"), "poison": ("#a040a0", "#e0a0e0", "#682a68"),
    "ground": ("#e0c068", "#f8e8b0", "#927d44"), "flying": ("#a890f0", "#d8ccff", "#6d5e9c"),
    "psychic": ("#f85888", "#ffb0c8", "#a13959"), "bug": ("#a8b820", "#dce890", "#6d7815"),
    "rock": ("#b8a038", "#e8d890", "#786824"), "ghost": ("#705898", "#c0a8e8", "#493963"),
    "dragon": ("#7038f8", "#b8a0ff", "#4924a1"), "steel": ("#b8b8d0", "#e8e8f8", "#787887"),
    "dark": ("#705848", "#b8a090", "#49392f"),
}


# Outer silhouette style per primary type.
SHAPE = {
    "fire": "flame", "water": "scallop", "ice": "scallop", "flying": "scallop",
    "electric": "bolt", "rock": "crenel", "ground": "crenel", "steel": "crenel",
    "grass": "petal", "psychic": "petal", "bug": "petal",
    "ghost": "wisp", "poison": "wisp", "dark": "wisp",
}


def silhouette(style, cx, cy, r, spike, n, rot, rng):
    """An SVG path around the body: n lobes of the given style."""
    def at(a, rad):
        return cx + rad * math.cos(a), cy + rad * math.sin(a)

    step = 2 * math.pi / n
    d = []
    if style == "star":
        for i in range(n * 2):
            a = rot + step * i / 2
            rad = r + (spike * (0.85 + 0.3 * rng.random()) if i % 2 == 0 else spike * 0.25)
            x, y = at(a, rad)
            d.append(("M" if i == 0 else "L") + f"{x:.1f} {y:.1f}")
    elif style in ("petal", "scallop"):
        reach = spike * (1.0 if style == "petal" else 0.55)
        x, y = at(rot, r)
        d.append(f"M{x:.1f} {y:.1f}")
        for i in range(n):
            a0, a1 = rot + step * i, rot + step * (i + 1)
            c1 = at(a0 + step * 0.15, r + reach * (1.5 if style == "petal" else 1.25))
            c2 = at(a1 - step * 0.15, r + reach * (1.5 if style == "petal" else 1.25))
            x1, y1 = at(a1, r)
            d.append(f"C{c1[0]:.1f} {c1[1]:.1f} {c2[0]:.1f} {c2[1]:.1f} {x1:.1f} {y1:.1f}")
    elif style == "flame":
        x, y = at(rot, r)
        d.append(f"M{x:.1f} {y:.1f}")
        for i in range(n):
            a0 = rot + step * i
            tip = at(a0 + step * 0.65, r + spike * (0.9 + 0.35 * rng.random()))
            c = at(a0 + step * 0.15, r + spike * 0.55)
            x1, y1 = at(a0 + step, r)
            d.append(f"Q{c[0]:.1f} {c[1]:.1f} {tip[0]:.1f} {tip[1]:.1f} Q{at(a0 + step * 0.8, r + spike * .2)[0]:.1f} "
                     f"{at(a0 + step * 0.8, r + spike * .2)[1]:.1f} {x1:.1f} {y1:.1f}")
    elif style == "bolt":
        for i in range(n * 3):
            a = rot + step * i / 3
            rad = r + [spike, spike * 0.2, spike * 0.6][i % 3]
            x, y = at(a, rad)
            d.append(("M" if i == 0 else "L") + f"{x:.1f} {y:.1f}")
    elif style == "crenel":
        for i in range(n):
            a0 = rot + step * i
            for f, rad in ((0.0, r + spike * .2), (0.15, r + spike * .2), (0.15, r + spike * .8),
                           (0.85, r + spike * .8), (0.85, r + spike * .2)):
                x, y = at(a0 + step * f, rad)
                d.append(("M" if i == 0 and f == 0.0 else "L") + f"{x:.1f} {y:.1f}")
    elif style == "wisp":
        x, y = at(rot, r + spike * .2)
        d.append(f"M{x:.1f} {y:.1f}")
        for i in range(n):
            a0 = rot + step * i
            tip = at(a0 + step * 0.5, r + spike * (0.7 + 0.5 * rng.random()))
            c1 = at(a0 + step * 0.05, r + spike * 1.0)
            x1, y1 = at(a0 + step, r + spike * .2)
            d.append(f"Q{c1[0]:.1f} {c1[1]:.1f} {tip[0]:.1f} {tip[1]:.1f} T{x1:.1f} {y1:.1f}")
    return " ".join(d) + " Z"


def pts(cx, cy, r, n, rot=0.0):
    return [(cx + r * math.cos(rot + 2 * math.pi * i / n), cy + r * math.sin(rot + 2 * math.pi * i / n)) for i in range(n)]


def poly(points):
    return " ".join(f"{x:.1f},{y:.1f}" for x, y in points)


def crest(p):
    rng = random.Random(p["id"] * 7919)
    s = p["stats"]
    t1 = p["types"][0]
    t2 = p["types"][1] if len(p["types"]) > 1 else t1
    fill, light, dark = TYPE[t1]
    fill2, light2, dark2 = TYPE[t2]
    cx = cy = 128
    rot = rng.uniform(0, 2 * math.pi)

    r_body = 52 + min(s["hp"], 160) / 160 * 26                    # HP: size
    n_spikes = 5 + min(s["attack"], 140) // 20                      # Attack: spikes
    spike = 14 + min(s["attack"], 140) / 140 * 30
    rings = 1 + min(s["defense"], 180) // 45                        # Defense: armor
    facets = 3 + min(s["special-attack"], 154) // 30                # Sp. Atk: gem facets
    halo = s["special-defense"] >= 90                               # Sp. Def: halo
    arcs = max(0, (s["speed"] - 50) // 25)                          # Speed: motion

    out = [f'<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 256 256" role="img" aria-label="{p["name"]} crest">',
           "<defs>",
           f'<radialGradient id="b" cx="38%" cy="32%" r="75%"><stop offset="0" stop-color="{light}"/>'
           f'<stop offset=".55" stop-color="{fill}"/><stop offset="1" stop-color="{dark}"/></radialGradient>',
           f'<linearGradient id="s" x1="0" y1="0" x2="1" y2="1"><stop offset="0" stop-color="{light2}"/>'
           f'<stop offset="1" stop-color="{dark2}"/></linearGradient>',
           f'<radialGradient id="g" cx="40%" cy="35%" r="70%"><stop offset="0" stop-color="#ffffff"/>'
           f'<stop offset=".6" stop-color="{light2}"/><stop offset="1" stop-color="{fill2}"/></radialGradient>',
           "</defs>"]

    # Speed: motion arcs behind the emblem
    for i in range(min(arcs, 4)):
        r = r_body + spike + 6 + i * 7
        a0 = rot + 0.4 + i * 0.25
        a1 = a0 + 1.1
        x0, y0 = cx + r * math.cos(a0), cy + r * math.sin(a0)
        x1, y1 = cx + r * math.cos(a1), cy + r * math.sin(a1)
        out.append(f'<path d="M{x0:.1f} {y0:.1f} A{r:.1f} {r:.1f} 0 0 1 {x1:.1f} {y1:.1f}" fill="none" '
                   f'stroke="{fill2}" stroke-width="{4 - i * .7:.1f}" stroke-linecap="round" opacity="{.75 - i * .15:.2f}"/>')

    # Attack: the outer silhouette. Its style comes from the primary type,
    # its point count and reach from Attack.
    out.append(f'<path d="{silhouette(SHAPE.get(t1, "star"), cx, cy, r_body, spike, n_spikes, rot, rng)}" '
               f'fill="url(#s)" stroke="#0b1024" stroke-width="3" stroke-linejoin="round"/>')

    # HP: the body
    out.append(f'<circle cx="{cx}" cy="{cy}" r="{r_body:.1f}" fill="url(#b)" stroke="#0b1024" stroke-width="3"/>')

    # Defense: armor rings
    for i in range(min(rings, 4)):
        r = r_body - 8 - i * 7
        out.append(f'<circle cx="{cx}" cy="{cy}" r="{r:.1f}" fill="none" stroke="{dark}" stroke-width="2.5" '
                   f'stroke-dasharray="{rng.choice(["0", "10 5", "3 4", "18 6"])}" opacity=".7"/>')

    # Sp. Def: a halo around the core
    core = 18 + min(s["special-attack"], 154) / 154 * 14
    if halo:
        out.append(f'<circle cx="{cx}" cy="{cy}" r="{core + 9:.1f}" fill="none" stroke="#ffffff" stroke-width="2" opacity=".75"/>')

    # Sp. Atk: the core gem
    gem = pts(cx, cy, core, facets, rot)
    out.append(f'<polygon points="{poly(gem)}" fill="url(#g)" stroke="#0b1024" stroke-width="2.5" stroke-linejoin="round"/>')
    for x, y in gem[: facets // 2 + 1]:
        out.append(f'<line x1="{cx}" y1="{cy}" x2="{x:.1f}" y2="{y:.1f}" stroke="#ffffff" stroke-width="1.2" opacity=".55"/>')

    # The primary type's glyph, set into the gem
    g = GLYPH.get(t1, GLYPH["normal"])
    k = core / 44
    out.append(f'<g transform="translate({cx} {cy}) scale({k:.3f})">'
               + (f'<path d="{g}" fill="none" stroke="#ffffff" stroke-width="7" stroke-linecap="round" stroke-linejoin="round"/>'
                  if t1 in STROKE_GLYPHS else
                  f'<path d="{g}" fill="#ffffff" fill-rule="evenodd" stroke="#0b1024" stroke-width="3" stroke-linejoin="round"/>')
               + "</g>")

    # A highlight to make it read as a polished object
    out.append(f'<ellipse cx="{cx - r_body * .35:.1f}" cy="{cy - r_body * .45:.1f}" rx="{r_body * .32:.1f}" '
               f'ry="{r_body * .16:.1f}" fill="#ffffff" opacity=".35" transform="rotate(-30 {cx - r_body * .35:.1f} {cy - r_body * .45:.1f})"/>')
    out.append("</svg>")
    return "\n".join(out)


# Gym sigils: a medal per gym, in its type's colors, with an original glyph.
GLYPH = {
    "rock": "M-26 18 L-6 -22 L6 -4 L14 -14 L28 18 Z",
    "water": "M-30 4 C-20 -8 -10 -8 0 4 C10 16 20 16 30 4 M-30 18 C-20 6 -10 6 0 18 C10 30 20 30 30 18 M-30 -10 C-20 -22 -10 -22 0 -10 C10 2 20 2 30 -10",
    "electric": "M6 -30 L-16 4 L-2 4 L-8 30 L16 -6 L2 -6 Z",
    "grass": "M0 30 C-26 10 -24 -18 0 -30 C24 -18 26 10 0 30 Z M0 30 L0 -22",
    "poison": "M0 -30 C14 -12 20 0 20 10 A20 20 0 0 1 -20 10 C-20 0 -14 -12 0 -30 Z",
    "psychic": "M-30 0 C-16 -18 16 -18 30 0 C16 18 -16 18 -30 0 Z M0 -9 A9 9 0 1 1 0 9 A9 9 0 1 1 0 -9",
    "fire": "M0 -30 C10 -14 22 -8 18 10 C15 24 6 30 0 30 C-6 30 -15 24 -18 10 C-20 0 -12 -6 -8 -16 C-4 -6 0 -4 2 -10 C4 -16 2 -22 0 -30 Z",
    "ground": "M-30 14 L-30 22 L30 22 L30 14 Z M-24 6 L-12 -6 L-4 2 L8 -18 L24 6 Z",
    # the remaining types, used on crests
    "normal": "M0 -30 L7 -7 L30 0 L7 7 L0 30 L-7 7 L-30 0 L-7 -7 Z",
    "ice": "M0 -30 L0 30 M-26 -15 L26 15 M-26 15 L26 -15 M-6 -24 L0 -18 L6 -24 M-6 24 L0 18 L6 24",
    "fighting": "M-22 -22 L22 22 M22 -22 L-22 22 M-28 -10 L-10 -28 M10 28 L28 10",
    "flying": "M-30 10 C-18 -12 -2 -16 0 -4 C2 -16 18 -12 30 10 C16 2 6 4 0 12 C-6 4 -16 2 -30 10 Z",
    "bug": "M0 -26 L22 -13 L22 13 L0 26 L-22 13 L-22 -13 Z M0 -10 L9 -5 L9 5 L0 10 L-9 5 L-9 -5 Z",
    "ghost": "M-20 26 L-20 -4 A20 20 0 0 1 20 -4 L20 26 L12 18 L4 26 L-4 18 L-12 26 Z M-9 -6 A4 5 0 1 0 -9 4 A4 5 0 1 0 -9 -6 M9 -6 A4 5 0 1 0 9 4 A4 5 0 1 0 9 -6",
    "dragon": "M-24 -24 C-10 -4 -10 14 -20 28 C4 18 14 -2 24 -28 C10 -14 -6 -14 -24 -24 Z",
    "steel": "M0 -28 L24 -14 L24 14 L0 28 L-24 14 L-24 -14 Z M0 -11 A11 11 0 1 1 0 11 A11 11 0 1 1 0 -11",
    "dark": "M8 -28 A28 28 0 1 0 8 28 A22 22 0 1 1 8 -28 Z",
}
STROKE_GLYPHS = {"water", "ice", "fighting"}
GYMS = [("rock", 1), ("water", 2), ("electric", 3), ("grass", 4), ("poison", 5), ("psychic", 6), ("fire", 7), ("ground", 8)]


def sigil(typ, n):
    fill, light, dark = TYPE[typ]
    stroke_only = typ in ("water",)
    ticks = "".join(
        f'<line x1="{64 + 54 * math.cos(a):.1f}" y1="{64 + 54 * math.sin(a):.1f}" x2="{64 + 49 * math.cos(a):.1f}" '
        f'y2="{64 + 49 * math.sin(a):.1f}" stroke="{light}" stroke-width="2"/>'
        for a in [2 * math.pi * i / (8 + n) for i in range(8 + n)])
    glyph = (f'<path d="{GLYPH[typ]}" fill="none" stroke="#ffffff" stroke-width="5" stroke-linecap="round" stroke-linejoin="round"/>'
             if stroke_only else
             f'<path d="{GLYPH[typ]}" fill="#ffffff" fill-rule="evenodd" stroke="#0b1024" stroke-width="2" stroke-linejoin="round"/>')
    return (f'<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 128 128" role="img" aria-label="{typ} gym sigil">'
            f'<defs><radialGradient id="m" cx="40%" cy="35%" r="70%"><stop offset="0" stop-color="{light}"/>'
            f'<stop offset=".6" stop-color="{fill}"/><stop offset="1" stop-color="{dark}"/></radialGradient></defs>'
            f'<circle cx="64" cy="64" r="58" fill="#0b1024"/>'
            f'<circle cx="64" cy="64" r="56" fill="url(#m)"/>{ticks}'
            f'<circle cx="64" cy="64" r="44" fill="none" stroke="#0b1024" stroke-width="2.5" opacity=".6"/>'
            f'<g transform="translate(64 64)">{glyph}</g>'
            f'<ellipse cx="44" cy="34" rx="16" ry="7" fill="#ffffff" opacity=".35" transform="rotate(-30 44 34)"/></svg>')


FAVICON = """<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 64 64">
<defs><linearGradient id="t" x1="0" y1="0" x2="0" y2="1"><stop offset="0" stop-color="#3fe0c8"/><stop offset="1" stop-color="#109c9a"/></linearGradient>
<linearGradient id="u" x1="0" y1="0" x2="0" y2="1"><stop offset="0" stop-color="#5a44d6"/><stop offset="1" stop-color="#1b1452"/></linearGradient>
<clipPath id="o"><circle cx="32" cy="32" r="29"/></clipPath></defs>
<g clip-path="url(#o)"><rect width="64" height="64" fill="url(#u)"/><path d="M0 40 C14 30 22 40 32 30 C42 20 50 30 64 20 L64 0 L0 0 Z" fill="url(#t)"/>
<path d="M0 40 C14 30 22 40 32 30 C42 20 50 30 64 20" fill="none" stroke="#ffcc33" stroke-width="3"/></g>
<circle cx="32" cy="32" r="29" fill="none" stroke="#0b1024" stroke-width="3"/>
<path d="M32 21 L41 25 L40 35 C39 40 36 43 32 45 C28 43 25 40 24 35 L23 25 Z" fill="#0b1024" stroke="#ffcc33" stroke-width="2.5" stroke-linejoin="round"/>
<circle cx="32" cy="31" r="3" fill="#ffcc33"/><path d="M30.5 32 L33.5 32 L34.5 39 L29.5 39 Z" fill="#ffcc33"/></svg>
"""


def main():
    with open(os.path.join(ROOT, "data", "kanto.json")) as f:
        dex = json.load(f)
    os.makedirs(os.path.join(ROOT, "assets", "crests"), exist_ok=True)
    os.makedirs(os.path.join(ROOT, "assets", "badges"), exist_ok=True)
    for p in dex:
        with open(os.path.join(ROOT, "assets", "crests", f"{p['id']}.svg"), "w") as f:
            f.write(crest(p))
    for typ, n in GYMS:
        with open(os.path.join(ROOT, "assets", "badges", f"{n}.svg"), "w") as f:
            f.write(sigil(typ, n))
    with open(os.path.join(ROOT, "assets", "favicon.svg"), "w") as f:
        f.write(FAVICON)
    print(f"ok: {len(dex)} crests, {len(GYMS)} gym sigils, favicon")


if __name__ == "__main__":
    main()
