# The Elite Four: the CI pipeline

The gyms guard the **cluster**. The Elite Four guard the **pipeline**: nothing reaches the registry, and so nothing
reaches the gyms, without beating them first. Workflow:
[`.github/workflows/league.yml`](../.github/workflows/league.yml).

```
pallet-town ──► lorelei ─┐
  tests         bruno   ─┼──► lance (main only)   build · push · sign · attest
                agatha  ─┘
                         └──► champion            make demo on kind · 8/8 badges · Rocket denied · ZAP
```

## Pallet Town: tests

`go vet`, `go test -race`, the 29 Kyverno CLI policy tests (`make policy-test`) and hadolint on the Dockerfile. The
policy tests are the important part: **policies are code, so they get tests**, including deliberately tricky cases like
`lookalike-registry` and `soft-init`.

## <img src="../assets/crests/131.svg" width="40"> Lorelei: secret scanning

[gitleaks](https://github.com/gitleaks/gitleaks) over the **full Git history** (`fetch-depth: 0`). A secret deleted in
a later commit is still in the history. The only real fix is to rotate it.

Try it locally: `docker run --rm -v $PWD:/repo ghcr.io/gitleaks/gitleaks:v8.28.0 dir /repo`. It finds your
`.cosign/cosign.key`. That's correct, and it's why `.cosign/` is in `.gitignore`.

The CI job also fails if gitleaks reports **0 commits scanned**. Run inside a container against a mounted repo, git
refuses to read it ("dubious ownership"), and gitleaks then reports "no leaks found" after checking nothing. A gate
that can pass without looking is worse than no gate.

## <img src="../assets/crests/68.svg" width="40"> Bruno: static analysis

- **govulncheck** reports only vulnerabilities your code can *actually reach*, with the call path. While this repo was
  being built it caught GO-2026-6617 (an HTTP/2 server crash), reachable through `client-go` and `net/http`, which was
  fixed by bumping `golang.org/x/net` and the Go toolchain. That's the gate doing its job on day one.
- **Semgrep** with the Go, Dockerfile, Kubernetes and GitHub Actions rule packs. `deploy/rocket/` and the policy-test
  fixtures are excluded in [`.semgrepignore`](../.semgrepignore), with the reason written down. A good exclusion is
  narrow and explains itself.

## <img src="../assets/crests/94.svg" width="40"> Agatha: dependencies and SBOM

- **Trivy** in filesystem mode: vulnerable dependencies, IaC misconfiguration and secrets, failing on fixable
  HIGH/CRITICAL findings (`ignore-unfixed` keeps the gate actionable).
- **SPDX SBOM** of the source, uploaded as a build artifact. An SBOM answers "are we affected?" in minutes the next time a
  Log4Shell happens.

## <img src="../assets/crests/149.svg" width="40"> Lance: build, sign, attest

Runs only on `main`, after all three Elite Four pass, once per service (matrix):

1. **Build and push** to `ghcr.io/<you>/poke-league/<app>:<sha>` with Buildx, for amd64 and arm64 (BuildKit SBOM +
   `provenance: mode=max`).
2. **Keyless cosign signing.** The job's GitHub OIDC token is exchanged with Sigstore's Fulcio for a short-lived
   certificate saying "built by this repo's workflow on this ref", and the signature is recorded in the public Rekor
   transparency log. No signing key exists to leak.
3. **Signed SBOM attestation** (`cosign attest --type spdxjson`), so the SBOM travels with the image and can be verified.
4. **SLSA build provenance** via `actions/attest-build-provenance`.

Permissions are granted per job: only Lance gets `packages: write`, `id-token: write` and `attestations: write`.

**Verify it yourself.** Anyone can check that an image was built by this repo's workflow, with no key needed:

```bash
docker run --rm ghcr.io/sigstore/cosign/cosign:v3.0.6 verify \
  ghcr.io/vincenzodavinci/poke-league/dex-api:<commit-sha> \
  --certificate-identity-regexp '^https://github.com/vincenzodavinci/poke-league/' \
  --certificate-oidc-issuer https://token.actions.githubusercontent.com
```

Use cosign **v3** here. The installer gives CI cosign v3, which stores signatures as Sigstore bundles via OCI referrers.
A v2 client looks for the old `sha256-<digest>.sig` tag and reports *no signatures found*, even though the image is
signed. (The local `make sign` flow uses v2 and the classic format, so the two never mix.)

Locally, Koga trusts a cosign **key pair** by default, because a laptop has no OIDC identity. The
[Hall of Fame](#-hall-of-fame-ci--cluster-trust) closes that gap: a cluster that trusts Lance's identity instead.

## <img src="../assets/crests/6.svg" width="40"> Champion: end-to-end

A fresh runner, `make demo`, then `make check` (which you can run locally too), then DAST:

1. Every non-Rocket workload holds **8/8 badges** (read from `/api/league`).
2. **Every Team Rocket manifest is denied** when retargeted at `indigo-plateau`, and **by exactly the right gyms**. The
   step fails if a request is admitted, if it fails for any reason other than an admission denial (a missing namespace,
   an API error), or if the set of gyms that refused it isn't the expected one. Arbok must be refused by Koga *alone*,
   which proves the other seven gyms really pass it. Then a **positive control**: a fresh copy of the real `dex-api`
   must still be admitted, because a gate that refuses everything would pass every negative test.
3. **OWASP ZAP baseline** (DAST) scans the live League Board. The strict CSP and security headers in
   [`internal/httpx`](../internal/httpx/httpx.go) are what it checks for.

## Control validation matrix

Every control has a case that must pass and a case that must fail, and CI runs both. A control that is only ever tested
against bad input can't tell "blocking" apart from "broken".

| Gym | Must pass | Must fail (incl. subtle bypasses) | Where |
|---|---|---|---|
| Brock · hardened pods | `champion` | `meowth`, `koffing`, `board-with-token`, `soft-init` (only the *init* container is soft), `sneaky-debug` (only the *ephemeral* container is) | Pallet Town (Kyverno CLI), Champion |
| Misty · no plaintext secrets | `champion`, `koffing` | `meowth`, `sneaky-debug` (token in an ephemeral container) | Pallet Town, Champion |
| Lt. Surge · resource limits | `champion` | `meowth`, `koffing`, `board-with-token` | Pallet Town, Champion |
| Erika · trusted images | `champion`, `board-with-token`, `tag-and-digest` | `meowth`, `koffing`, `latest-tag`, `no-tag` (implicit `:latest`), `lookalike-registry` (`poke-league-evil/`), `mutable-tag` (`:v1.2.3`), `not-a-digest` (truncated), `sneaky-init`, `sneaky-debug` | Pallet Town, Champion |
| Koga · signed images | the 3 league images, signed by `make sign` | `arbok`: right registry, pinned by digest, flawless manifest, unsigned extra layer; `arbok (init container)`: the same image hidden in an init container behind a signed app | Champion (needs a real registry) |
| Koga · CI identity (`TRUST=ci`) | the 3 images Lance signed keylessly, pulled from GHCR | a poisoned image pushed straight to GHCR, refused by Koga alone, as the app and as an init container | Hall of Fame |
| Sabrina · health probes | `champion` | `meowth`, `koffing` | Pallet Town, Champion |
| Blaine · no host access | `champion`, `meowth` | `koffing`, `sneaky-debug` (privileged ephemeral container) | Pallet Town, Champion |
| Prod: no `kubectl debug` | adding a debug container in Pallet Town (positive control) | the same request in Indigo Plateau, refused by `indigo-no-debug` | Champion (`expect_debug_denied.sh`) |
| Board egress | the board's label reaches `kubernetes.default.svc`; `example.com` is reachable from `default` (control) | the board's label reaching `example.com` | Champion (`expect_egress.sh`) |
| The board tells the truth | all 6 league workloads at 8/8, prod cards covering 2 pods | Team Rocket shown failing (Arbok on Koga); any report the board couldn't read | Champion (`check_board.py`), Go unit tests |
| Giovanni · least privilege | `champion`, `board-with-token` (annotated opt-in) | `meowth`, `koffing` | Pallet Town, Champion |
| All 17 policies loaded | `make policies` waits for 17/17 Ready | a policy set with Koga missing fails to render | Champion (`make demo`) |
| Lorelei · secret scan | full history, no findings | a scan of 0 commits fails the job | Lorelei |

The `Must pass`/`Must fail` resources live in [`deploy/policies/tests/resources.yaml`](../deploy/policies/tests/resources.yaml),
and the expected verdicts in [`kyverno-test.yaml`](../deploy/policies/tests/kyverno-test.yaml). Run them locally with
`make policy-test`.

## <img src="../assets/crests/150.svg" width="40"> Hall of Fame: CI → cluster trust

Lance proves *who built* an image. The Hall of Fame proves the cluster *acts on it*. It runs after Lance, on a fresh
runner:

1. `make demo TRUST=ci` builds a league that pulls the three images Lance just pushed to GHCR, pinned by digest, with
   **no local build and no key**. Koga is swapped for its [CI-trust edition](../deploy/policies/ci-trust/05-koga-signed-images.yaml),
   which admits an image only if its signature carries this exact identity:
   ```
   issuer:  https://token.actions.githubusercontent.com
   subject: https://github.com/<you>/poke-league/.github/workflows/league.yml@refs/heads/main
   ```
   Erika's registry allowlist moves to `ghcr.io/<you>/poke-league/`.
2. `make check TRUST=ci`: every league workload holds 8/8 **and runs an image from GHCR**, so the badges can't come from
   anything else.
3. **The attack.** The job pushes a poisoned `dex-api` (real bytes plus one layer) straight to GHCR, which is what a
   stolen registry token allows. Right registry, right name, flawless manifest. It must be refused by **Koga alone**.

Signatures from a fork, a pull request, another branch or another workflow carry a different subject, so they're
refused too. Try it locally (once CI has published images for `origin/main`):

```bash
make down && make demo TRUST=ci && make check TRUST=ci
```

**What this does not prove.** Every job in `league.yml` on `main` signs with the same identity, so a compromised step
*inside this workflow* could sign too. That's why the workflow pins actions to SHAs and keeps `id-token: write` on
Lance alone. Pinning further is possible by matching the certificate's extra GitHub claims (Kyverno's
`additionalExtensions`), or by requiring the SLSA provenance Lance already attaches.

## Supply-chain hygiene in the workflow itself

- Every third-party action is **pinned to a full commit SHA**, with the version in a comment. Tags can be moved; the
  2025 `tj-actions/changed-files` compromise did exactly that. [Dependabot](../.github/dependabot.yml) bumps the SHAs.
- `permissions: contents: read` at the top, with more granted per job only where needed.
- `persist-credentials: false` on checkout, so later steps can't reuse the Git token.
- `concurrency` cancels superseded runs.

Lint the workflow locally: `docker run --rm -v $PWD:/repo -w /repo rhysd/actionlint:1.7.7`

## Running it

The pipeline runs on every push and pull request. No secrets need configuring: signing is keyless, and gitleaks runs
as a plain CLI container, with no license needed.
