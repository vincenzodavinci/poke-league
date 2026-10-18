# The eight gyms

Each gym is one Kyverno policy in [`deploy/policies/gyms/`](../deploy/policies/gyms), written once in
[CEL](https://kubernetes.io/docs/reference/using-api/cel/) and stamped out twice by Kustomize:

| Overlay | Namespaces | `validationActions` | Effect |
|---|---|---|---|
| [`audit/`](../deploy/policies/audit) | `pallet-town`, `team-rocket` | `[Audit, Warn]` | Result goes into a PolicyReport and `kubectl` prints a warning. Nothing is blocked. |
| [`enforce/`](../deploy/policies/enforce) (`-enforce` suffix) | `indigo-plateau` | `[Deny]` | Admission is refused. |

This is how most real platforms roll out a control: **audit first**, fix what it finds, **then enforce**. Writing the rule
once means dev and prod can never drift apart.

Every gym follows the same shape:

```yaml
matchConstraints:            # which requests: Pods in namespaces labelled poke-league.dev/league=true
variables:                   # name the offending containers so the message can say which one
validations:                 # CEL expressions that must be true
  - expression: size(variables.soft) == 0
    messageExpression: "'Brock: container(s) ' + variables.soft.join(', ') + ' need ...'"
autogen:                     # also check Deployments, so a bad one is rejected before any Pod exists
```

**How to use this page:** for each gym, read it, then do *Try it*. Every exercise runs in Pallet Town, where you'll get a
warning instead of a rejection, so you can't break anything.

A scratch pod to experiment with:

```bash
alias k='kubectl --context kind-kanto'
k -n pallet-town run trainer --image=kind-registry:5000/poke-league/dex-api:dev --dry-run=server -o yaml >/dev/null
```

That one line already fails four gyms (Brock, Lt. Surge, Sabrina, Giovanni), and Pod Security Admission warns about it
too. Read the warnings it prints.

---

## <img src="../assets/badges/1.png" width="28"> Gym 1 · Brock · `brock-hardened-pods`

**Rule.** The pod sets `runAsNonRoot: true` and `seccompProfile: RuntimeDefault`. Every container (init containers
included) sets `allowPrivilegeEscalation: false`, `readOnlyRootFilesystem: true` and `capabilities.drop: [ALL]`.

**Why.** Most container escapes and post-exploitation steps need one of these: root inside the container, setuid
binaries (privilege escalation), Linux capabilities such as `CAP_SYS_ADMIN` or `CAP_NET_RAW`, a writable filesystem to
drop tools onto, or unfiltered syscalls. Turn them all off and an attacker who gets code execution in your app is stuck in
a very small box.

**Where it's satisfied.** `deploy/k8s/base/*.yaml` (lines marked `# Brock`), plus the distroless base image, which runs
as UID 65532 and has no shell.

**Try it.** Copy `deploy/k8s/base/dex-api.yaml`, set `readOnlyRootFilesystem: false`, apply it to `pallet-town` and watch
the Boulder Badge go dark on the board. Then apply the same file to `indigo-plateau`.

**Go further.** Kubernetes ships the same idea built in: Pod Security Admission. Look at the
`pod-security.kubernetes.io/*` labels in [`namespaces.yaml`](../deploy/k8s/cluster/namespaces.yaml). Prod enforces
`restricted`, so Brock and PSA are two independent layers.

## <img src="../assets/badges/2.png" width="28"> Gym 2 · Misty · `misty-no-plaintext-secrets`

**Rule.** An env var whose name looks like a credential (`*PASSWORD*`, `*SECRET*`, `*TOKEN*`, `*API_KEY*`,
`*PRIVATE_KEY*`) must use `valueFrom.secretKeyRef`, not `value:`.

**Why.** A literal in a manifest lives in Git history forever, shows up in `kubectl get -o yaml` for anyone with read
access to Deployments, and gets copied into every fork. `Secret`s can be RBAC-restricted, encrypted at rest and rotated.

**Try it.**
```bash
k -n pallet-town create secret generic db --from-literal=password=hunter2
```
Then fix Meowth's `DB_PASSWORD` to use `secretKeyRef` (see the `champion` pod in
[`tests/resources.yaml`](../deploy/policies/tests/resources.yaml)).

**Honest limit.** A name pattern is a heuristic: `MY_CREDS=...` sails through. That's why Lorelei (gitleaks) also scans
the repo with entropy and signature rules. Defense in depth.

**Go further.** Kubernetes Secrets are only base64-encoded. Look at External Secrets Operator or Sealed Secrets, and at
encryption at rest for etcd.

## <img src="../assets/badges/3.png" width="28"> Gym 3 · Lt. Surge · `surge-resource-limits`

**Rule.** Every container declares CPU and memory **requests** and a memory **limit**.

**Why.** Requests are what the scheduler uses to place pods. Without them the node gets overcommitted. A memory limit
caps what one container can take. Without it, a leak or a deliberately memory-hungry request can OOM the whole node and
everything else on it, which makes it a cheap denial of service. (No CPU limit is required: CPU is compressible, and CPU
limits often cause more throttling harm than good. That's a real debate worth reading up on.)

**Try it.** Remove `limits` from a copy of `sprite-svc.yaml` and apply it to `pallet-town`.

## <img src="../assets/badges/4.png" width="28"> Gym 4 · Erika · `erika-trusted-images`

**Rule.** Images come from `kind-registry:5000/poke-league/` and carry a version tag or digest. `:latest`, and images with
no tag at all (which means `:latest`), are rejected.

**Why.** A registry allowlist stops "just pull it from Docker Hub" and typosquats. Look at the `lookalike-registry` test
case: `poke-league-evil/` is caught because the check includes the trailing slash. `:latest` is not a version. It can
point at different bytes tomorrow, so you can't reproduce, roll back or audit what ran.

**Where it's satisfied.** `make deploy` rewrites every image to `@sha256:<digest>`
([`.build/overlays`](../Makefile)), so prod runs exactly the bytes that were built and signed.

**Try it.** `k -n pallet-town run latest --image=kind-registry:5000/poke-league/dex-api:latest --dry-run=server`

## <img src="../assets/badges/5.png" width="28"> Gym 5 · Koga · `koga-signed-images`

**Rule.** Every image must carry a valid [cosign](https://github.com/sigstore/cosign) signature from the league's key.
This one is an `ImageValidatingPolicy`: Kyverno fetches the signature from the registry at admission time and verifies it.

**Why.** Erika checks where an image claims to come from. Koga proves who built it. If an attacker can push to your
registry (stolen CI token, misconfigured permissions), every other gym still passes. See Arbok.

**How the key gets in.** `make sign` generates `.cosign/cosign.key` (gitignored, never committed) and `cosign.pub`.
[`scripts/render_policies.py`](../scripts/render_policies.py) writes the public key into the policy, and it refuses to
produce a policy set with Koga missing. (Why that matters is in the field notes in [architecture.md](architecture.md).)

**Two lessons this gym taught while being built:**
1. **Signatures bind to digests, not tags.** `docker tag signed-image anything` is still signed, because it's the same
   bytes. To make Arbok poisoned, `make rocket` has to add a layer.
2. **Verification results are cached by image reference.** If a tag once pointed at signed content, the cached "valid"
   can outlive the tag being re-pointed. One more reason to deploy by digest.

**Try it.** `make challenge` covers Meowth. For Arbok, see [team-rocket.md](team-rocket.md#arbok).

**Go further.** In CI, Lance signs **keyless**: no long-lived key, just a short-lived certificate bound to the GitHub
Actions OIDC identity and recorded in the public Rekor log. Rewriting Koga to verify that identity (`keyless.identities`)
is a great exercise once the images live in GHCR.

## <img src="../assets/badges/6.png" width="28"> Gym 6 · Sabrina · `sabrina-health-probes`

**Rule.** Every container has a `livenessProbe` and a `readinessProbe`.

**Why.** Readiness keeps traffic away from a pod that isn't ready (still starting, or draining during a rollout).
Liveness restarts a container that's wedged. That's availability, which is the A in the CIA triad. Our services serve
`/healthz` and `/readyz`, and drain in-flight requests on SIGTERM ([`internal/httpx`](../internal/httpx/httpx.go)), so
rolling updates in prod (2 replicas) drop no requests.

## <img src="../assets/badges/7.png" width="28"> Gym 7 · Blaine · `blaine-no-host-access`

**Rule.** No `privileged` containers, no `hostNetwork`, `hostPID` or `hostIPC`, and no `hostPath` volumes.

**Why.** Each of these punches through the container boundary. A privileged container with `hostPID` can `nsenter` into
PID 1 on the node. A `hostPath: /` mount is the node's filesystem. `chroot /host` and you *are* the node, with its
kubelet credentials. See Koffing.

## <img src="../assets/badges/8.png" width="28"> Gym 8 · Giovanni · `giovanni-least-privilege`

**Rule.** Pods don't use the namespace's `default` ServiceAccount, and either set `automountServiceAccountToken: false`
or carry the annotation `poke-league.dev/needs-api-token: "true"`.

**Why.** Every pod gets an API token by default. Most apps never call the Kubernetes API, so that token is pure attack
surface. If anyone ever grants the `default` SA some permissions, every pod in the namespace silently gets them too.
Only `league-board` needs a token, so it opts in. Its RBAC ([`rbac.yaml`](../deploy/k8s/cluster/rbac.yaml)) is read-only
and limited to pods and reports in three namespaces. No secrets, no writes.

**Try it.** Do the Meowth lab in [team-rocket.md](team-rocket.md#meowth): steal its token and see what it can do.

---

## Write a ninth gym

Ideas, roughly in order of difficulty:

- **Require a NetworkPolicy** in every league namespace. Hint: this is a `Namespace`-level check, or a `GeneratingPolicy`
  that creates the default-deny automatically.
- **No `NodePort` Services in prod** (the board's own NodePort will need a `PolicyException`, which is a lesson in itself).
- **Require the SBOM attestation** that Lance already attaches in CI (`attestations` in an `ImageValidatingPolicy`).

Add a test case to [`deploy/policies/tests/`](../deploy/policies/tests) first, then make it pass. Then add the gym to
[`internal/board/gyms.go`](../internal/board/gyms.go) so it shows up on the board.
