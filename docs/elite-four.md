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

1. **Build and push** to `ghcr.io/<you>/poke-league/<app>:<sha>` with Buildx (BuildKit SBOM + `provenance: mode=max`).
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

> Locally, Koga trusts a cosign **key pair** because a laptop has no OIDC identity. Switching Koga to verify Lance's
> keyless identity (`keyless.identities` with issuer `https://token.actions.githubusercontent.com`) is the natural next
> step once you run against GHCR.

## <img src="../assets/crests/6.svg" width="40"> Champion: end-to-end

A fresh runner, `make demo`, then three assertions:

1. Every non-Rocket workload holds **8/8 badges** (read from `/api/league`).
2. **Every Team Rocket manifest is denied** when retargeted at `indigo-plateau`.
3. **OWASP ZAP baseline** (DAST) scans the live League Board. The strict CSP and security headers in
   [`internal/httpx`](../internal/httpx/httpx.go) are what it checks for.

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
