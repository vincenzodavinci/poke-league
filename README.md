<p align="center">
  <img src="assets/crests/137.svg" width="140" alt="Porygon crest, the Poké League mascot">
</p>

<h1 align="center">Poké League</h1>

<p align="center">
  <b>Learn Kubernetes DevSecOps by earning all eight Kanto badges.</b><br>
  Every gym is a real security control. Every workload is a Pokémon. Team Rocket is trying to get into prod.
</p>

<p align="center">
  <a href="https://github.com/vincenzodavinci/poke-league/actions/workflows/league.yml"><img src="https://github.com/vincenzodavinci/poke-league/actions/workflows/league.yml/badge.svg" alt="League pipeline"></a>
  <img src="https://img.shields.io/badge/Kubernetes-kind-326CE5?logo=kubernetes&logoColor=white" alt="Kubernetes">
  <img src="https://img.shields.io/badge/policies-Kyverno%20CEL-FF7F00" alt="Kyverno CEL policies">
  <img src="https://img.shields.io/badge/signed%20with-cosign-4B32C3?logo=sigstore&logoColor=white" alt="cosign">
  <img src="https://img.shields.io/badge/Go-1.27-00ADD8?logo=go&logoColor=white" alt="Go 1.27">
  <img src="https://img.shields.io/badge/license-MIT-green" alt="MIT license">
</p>

<p align="center">
  <img src="assets/badges/1.svg" width="36" alt="Boulder Badge">
  <img src="assets/badges/2.svg" width="36" alt="Cascade Badge">
  <img src="assets/badges/3.svg" width="36" alt="Thunder Badge">
  <img src="assets/badges/4.svg" width="36" alt="Rainbow Badge">
  <img src="assets/badges/5.svg" width="36" alt="Soul Badge">
  <img src="assets/badges/6.svg" width="36" alt="Marsh Badge">
  <img src="assets/badges/7.svg" width="36" alt="Volcano Badge">
  <img src="assets/badges/8.svg" width="36" alt="Earth Badge">
</p>

<p align="center">
  <img src="docs/images/league.png" width="860" alt="The League Board: shiny, fully-badged workloads in dev and prod, and Team Rocket's workloads failing gyms in the lab">
</p>

## Why I built this

<img src="assets/crests/6.svg" width="110" align="right" alt="Charizard crest">

**This project exists because of my love for Pokémon FireRed.**

FireRed was the game I couldn't put down: picking a starter in Pallet Town, grinding through Viridian Forest, and
finally walking into the Indigo Plateau with a team I'd built badge by badge. Every gym leader taught one lesson, and
you couldn't skip ahead until you'd learned it.

When I started learning DevSecOps, it felt the same: a long route of separate skills (hardening, secrets, supply chain,
least privilege) that only click once you've fought through each one. So I built the league I wished I'd had. Every
gym is a real Kubernetes security control, the Elite Four is a real CI pipeline, and Team Rocket is every misconfigured
Deployment you'll ever meet. Even the Pokédex inside uses **FireRed's Gen 3 typings**, so Clefairy is Normal, the way it
was meant to be.

If you grew up with a Game Boy Advance and you're learning cloud security now, this one's for you. Gotta secure 'em all.

## What this is

A complete, runnable DevSecOps platform on a local [kind](https://kind.sigs.k8s.io/) cluster, themed as the Kanto Pokémon League:

| Pokémon world | Real thing |
|---|---|
| **8 Gym Leaders** | 8 [Kyverno](https://kyverno.io) admission policies written in CEL. Pass one, earn its badge. |
| **Pallet Town** | `pallet-town` namespace (dev). Gyms **audit**: failures show up as warnings and on the board. |
| **Indigo Plateau** | `indigo-plateau` namespace (prod). Gyms **enforce**: fail one and you're not admitted. |
| **The Elite Four + Champion** | The CI pipeline: secret scanning, SAST, SCA + SBOM, signing + provenance, then an end-to-end DAST run. |
| **HP** | Live CVE counts from the [Trivy Operator](https://github.com/aquasecurity/trivy-operator). Too many and your Pokémon faints. |
| **Shiny ✦** | 8/8 badges **and** no high or critical CVEs. |
| **Team Rocket** | Deliberately insecure workloads in a `team-rocket` lab namespace: a CTF to attack, then fix. |
| **The League Board** | A Pokédex-styled dashboard that reads Kyverno PolicyReports and Trivy VulnerabilityReports straight from the cluster. |

The app being secured is real too: three small Go services (a FireRed-accurate Kanto Pokédex API, a sprite server and the board), built on distroless images, hardened, signed and pinned by digest.

## Quick start

You need Docker (Docker Desktop, Colima or OrbStack), `kind`, `kubectl`, `helm`, Go 1.27+ and `make`.

```bash
make demo        # ~3 minutes: cluster, Kyverno, build + sign images, open the gyms, deploy, release Team Rocket
make board       # http://localhost:8088
make trivy       # optional: CVE scanning → HP bars (first scan takes a few minutes)
```

Then try to cheat:

```bash
make challenge   # sneak Team Rocket's Meowth into prod and read why every gym says no
make reports     # the raw PolicyReports the board is built from
make test        # Go unit tests + 29 Kyverno policy tests
make down        # delete everything
```

Every `kubectl`/`helm` command in the Makefile is pinned to the `kind-kanto` context, so it can't act on whatever cluster your kubeconfig points at.

## The gyms

| | Gym | Policy | What it teaches |
|:-:|---|---|---|
| <img src="assets/badges/1.svg" width="28"> <img src="assets/crests/95.svg" width="44"> | **Brock** · Boulder | `brock-hardened-pods` | Pod hardening: non-root, seccomp, no privilege escalation, read-only root FS, drop all capabilities |
| <img src="assets/badges/2.svg" width="28"> <img src="assets/crests/121.svg" width="44"> | **Misty** · Cascade | `misty-no-plaintext-secrets` | Secrets belong in `Secret` refs, never as literals in Git |
| <img src="assets/badges/3.svg" width="28"> <img src="assets/crests/26.svg" width="44"> | **Lt. Surge** · Thunder | `surge-resource-limits` | Requests and limits: scheduling, and stopping noisy-neighbour DoS |
| <img src="assets/badges/4.svg" width="28"> <img src="assets/crests/45.svg" width="44"> | **Erika** · Rainbow | `erika-trusted-images` | Trusted registries, pinned versions, why `:latest` is not a version |
| <img src="assets/badges/5.svg" width="28"> <img src="assets/crests/110.svg" width="44"> | **Koga** · Soul | `koga-signed-images` | Supply-chain integrity: only cosign-signed images run |
| <img src="assets/badges/6.svg" width="28"> <img src="assets/crests/65.svg" width="44"> | **Sabrina** · Marsh | `sabrina-health-probes` | Liveness/readiness probes and self-healing |
| <img src="assets/badges/7.svg" width="28"> <img src="assets/crests/59.svg" width="44"> | **Blaine** · Volcano | `blaine-no-host-access` | Container escape routes: privileged, host namespaces, hostPath |
| <img src="assets/badges/8.svg" width="28"> <img src="assets/crests/112.svg" width="44"> | **Giovanni** · Earth | `giovanni-least-privilege` | Workload identity: own ServiceAccount, API tokens only on opt-in, least-privilege RBAC |

Each one is explained, with a hands-on exercise, in **[docs/gyms.md](docs/gyms.md)**.

<p align="center">
  <img src="docs/images/gyms.png" width="860" alt="The Gyms tab: each badge, its leader's ace, the policy name and the lesson">
</p>

## Team Rocket's hideout

<p>
  <img src="assets/crests/52.svg" width="64" alt="Meowth">
  <img src="assets/crests/109.svg" width="64" alt="Koffing">
  <img src="assets/crests/24.svg" width="64" alt="Arbok">
</p>

Three workloads that fail on purpose, each a pattern you'll meet in real clusters:

- **Meowth** · the copy-pasted Deployment: root, a public image with 8 critical CVEs, a plaintext password, the default ServiceAccount with its token mounted. *1/8 badges, fainted.*
- **Koffing** · the forgotten debug pod: privileged, `hostPID`, the node's root filesystem at `/host`. *1/8.*
- **Arbok** · the poisoned image: a flawless manifest, but an image nobody signed. Seven gyms wave it through; only Koga catches it. *7/8.*

Steal the token, escape to the node, then fix each one until it's shiny: **[docs/team-rocket.md](docs/team-rocket.md)**.

## The Elite Four (CI)

[`.github/workflows/league.yml`](.github/workflows/league.yml) runs on every push and pull request:

| | Stage | Tools |
|---|---|---|
| <img src="assets/crests/131.svg" width="40"> | **Lorelei** · secrets | gitleaks, over the full Git history |
| <img src="assets/crests/68.svg" width="40"> | **Bruno** · SAST | Semgrep (Go, Dockerfile, Kubernetes, Actions rules), govulncheck |
| <img src="assets/crests/94.svg" width="40"> | **Agatha** · dependencies | Trivy (vulns, misconfig, secrets), SPDX SBOM |
| <img src="assets/crests/149.svg" width="40"> | **Lance** · supply chain | Buildx, keyless cosign signing (OIDC → Fulcio → Rekor), signed SBOM attestation, SLSA provenance |
| <img src="assets/crests/6.svg" width="40"> | **Champion** · end-to-end | `make demo` on a fresh kind cluster, assert 8/8 badges, assert Team Rocket is denied in prod, OWASP ZAP DAST scan |

Every third-party action is pinned to a commit SHA, permissions are least-privilege per job, and Dependabot keeps Go modules, base-image digests and actions current. Details: **[docs/elite-four.md](docs/elite-four.md)**.

## The Pokédex

The app the league protects: all 151 Kanto Pokémon with **Gen 3 (FireRed) typings**, so Clefairy is Normal and Magnemite is Electric/Steel, plus base stats, heights and weights.

<p align="center">
  <img src="docs/images/pokedex.png" width="860" alt="The Pokédex tab: searchable grid of Kanto Pokémon with Gen 3 type colors">
</p>

## The art: Dex Crests

Every Pokémon is shown as an original **Dex Crest**, generated by [`scripts/make_crests.py`](scripts/make_crests.py)
from its real Gen 3 data, so no official artwork is used:

| Data | Becomes |
|---|---|
| Primary type | the silhouette (flame tips for Fire, waves for Water, crenellations for Rock…) and the core glyph |
| Secondary type | the rim color |
| HP · Attack · Defense | size · number and reach of points · armor rings |
| Sp. Atk · Sp. Def · Speed | facets on the core gem · a halo · motion arcs |

<p>
  <img src="assets/crests/6.svg" width="72" alt="Charizard crest">
  <img src="assets/crests/94.svg" width="72" alt="Gengar crest">
  <img src="assets/crests/143.svg" width="72" alt="Snorlax crest">
  <img src="assets/crests/95.svg" width="72" alt="Onix crest">
  <img src="assets/crests/151.svg" width="72" alt="Mew crest">
  <img src="assets/crests/129.svg" width="72" alt="Magikarp crest">
</p>

Charizard is fire-orange with flame tips and speed arcs; Snorlax is huge (all that HP); Magikarp, with base Attack 10,
is small and humble. The gym badges are original sigils too.

## How it fits together

```
 browser ──► league-board (NodePort) ──► dex-api      Kanto dex JSON (embedded)
               │      │                └► sprite-svc   crests and gym sigils (embedded)
               │      └─ reads ─► PolicyReports (Kyverno) + VulnerabilityReports (Trivy)
               │
  kubectl apply ─► Kyverno admission ─► 8 gyms × {audit: pallet-town, team-rocket | enforce: indigo-plateau}
                                         Koga verifies cosign signatures against kind-registry:5000
```

Architecture, design decisions and the field notes from building it (the bugs worth learning from) are in **[docs/architecture.md](docs/architecture.md)**.

## Learning path

1. `make demo`, open the board, and look around.
2. Read a gym in [docs/gyms.md](docs/gyms.md), then break it on purpose in Pallet Town and watch the badge go dark.
3. Run `make challenge` to see enforcement in prod.
4. Work through the [Team Rocket labs](docs/team-rocket.md): attack first, then fix.
5. `make trivy` and watch CVEs become HP damage. Patch Meowth's image and revive it.
6. Push to GitHub and watch the Elite Four run ([docs/elite-four.md](docs/elite-four.md)).
7. Write a ninth gym. Ideas: require NetworkPolicies, ban `NodePort` in prod, require the SBOM attestation that Lance already produces.

## Repository layout

```
assets/            original art: 151 Dex Crests, 8 gym sigils, favicon · embedded into sprite-svc
data/              FireRed-accurate Kanto dex · embedded into dex-api
cmd/               dex-api, sprite-svc, league-board
internal/          httpx (secure headers, timeouts, graceful shutdown), dex, sprites, board
web/               the League Board UI: plain JS under a strict CSP, no build step
build/Dockerfile   one multi-stage, digest-pinned, distroless Dockerfile for every service
deploy/kind/       the "kanto" cluster + local registry
deploy/k8s/        namespaces + PSA, RBAC, hardened Deployments, NetworkPolicies, dev/prod overlays
deploy/policies/   the 8 gyms (CEL), audit/enforce overlays, Kyverno CLI tests
deploy/rocket/     Team Rocket's lab workloads
scripts/           fetch_assets.py (dex facts), make_crests.py (the art), render_policies.py (cosign key → Koga)
.github/           the Elite Four pipeline, Dependabot
```

## Credits and legal

A non-commercial, fan-made learning project, **not affiliated with or endorsed by** Nintendo, Creatures Inc., GAME FREAK
inc. or The Pokémon Company. Pokémon, character, place and badge names are their trademarks, used here only to refer
to them. **No official artwork, sprites or game text are included.** Every image is original, generated by
`scripts/make_crests.py`. Dex facts (names, types, stats) come from [PokeAPI](https://pokeapi.co).

Everything here, including the original art, is MIT licensed. See [LICENSE](LICENSE).
