# Architecture

```
                         kind cluster "kanto"                                        host
┌──────────────────────────────────────────────────────────────────────────┐
│  ns/kyverno         admission controller ─ validates every Pod/Deployment│
│                     reports controller   ─ writes PolicyReports          │
│                                                                          │
│  ns/indigo-plateau  (prod · gyms ENFORCE · PSA restricted · 2 replicas)  │
│    league-board ──► dex-api                                             ◄── localhost:8088
│        │       └──► sprite-svc                                           │     (NodePort 30080)
│        └─ reads PolicyReports + VulnerabilityReports (read-only RBAC)    │
│  ns/pallet-town     (dev  · gyms AUDIT  · PSA warn) same app             │
│  ns/team-rocket     (lab  · gyms AUDIT  · PSA privileged) meowth koffing arbok
│  ns/trivy-system    trivy-operator ─ writes VulnerabilityReports (opt.)  │
└───────────────────────────────▲──────────────────────────────────────────┘
                                │ pulls images + cosign signatures
                     kind-registry:5000  (localhost:5001 from the host)
```

## The services

| Service | Pokémon | Does | Security notes |
|---|---|---|---|
| `dex-api` | Magnemite | `/api/pokemon`, `/api/pokemon/{id}`, `/api/types` | Data embedded at build time, input length-bounded, IDs parsed as integers |
| `sprite-svc` | Mew (it contains every Pokémon) | `/sprites/{pokemon,badges,items}/...` | Assets embedded with `go:embed`: no disk paths to traverse, no upstream fetch to SSRF |
| `league-board` | Porygon | UI + `/api/league`, proxies the other two | Fixed proxy targets from config; 5 s cache in front of the API server; the only pod with an API token |

All three share [`internal/httpx`](../internal/httpx/httpx.go): a strict same-origin CSP and the other security headers,
read/write/idle timeouts and a header-size cap (Slowloris), JSON access logs, and graceful shutdown on SIGTERM.

The UI ([`web/`](../web)) is plain JavaScript with no build step and no dependencies, written to live inside
`script-src 'self'`: no inline scripts or styles, and every piece of server data goes in through `textContent`
(`el()` in `app.js`), never `innerHTML`.

## How the board scores a workload

- **Badges**: Kyverno writes a `PolicyReport` per Pod. Each result names a policy; `brock-hardened-pods` and
  `brock-hardened-pods-enforce` both map to Brock ([`gyms.go`](../internal/board/gyms.go)). Any failing rule loses the
  badge, and a gym with no result yet is "unknown".
- **HP**: Trivy Operator writes a `VulnerabilityReport` per ReplicaSet container. The board maps pod → owning ReplicaSet
  and computes `HP = 100 − 25·critical − 8·high − 2·medium` ([`board.go`](../internal/board/board.go)).
- **Shiny**: 8/8 badges and zero high or critical CVEs.
- **Pokémon**: the `poke-league.dev/pokemon` annotation, or a stable hash of the app name.

Both report types are read with the dynamic client, so a missing CRD (Trivy not installed) degrades to "awaiting scan"
instead of an error. All of it is unit-tested against fake clients ([`board_test.go`](../internal/board/board_test.go)).

## Design decisions

- **CEL `ValidatingPolicy`, not `ClusterPolicy`.** Kyverno 1.19 deprecates `kyverno.io/v1 ClusterPolicy` in favour of
  the CEL-based `policies.kyverno.io/v1` types, which use the same language as Kubernetes' own
  ValidatingAdmissionPolicy. A learning repo shouldn't teach the API on its way out.
- **One policy, two overlays.** `validationActions` applies to a whole policy, so instead of hand-maintaining audit and
  enforce copies, Kustomize generates both from one source ([`deploy/policies`](../deploy/policies)).
- **Deploy by digest.** `make deploy` renders `.build/overlays/*` with `image@sha256:...`. That makes rollouts
  reproducible and verification exact, and avoids the tag-caching trap described below.
- **Two independent admission layers.** Pod Security Admission (built in) and Kyverno (custom). Either one alone stops
  Koffing in prod.
- **NetworkPolicies are default-deny.** Only browser → board, board → backends, board → API server and DNS are allowed.
  kind's default CNI (kindnet) enforces NetworkPolicies, so this is real, not decoration.
- **Every cluster command names its context.** See field note 1.

## Threat model and what's simplified

**What the league defends against.**
- *A compromised app container.* Brock, Blaine and Giovanni keep an attacker with code execution in a very small box: no
  root, no capabilities, no writable filesystem, no host access, no API token. The NetworkPolicies stop lateral movement.
- *Misconfigured manifests reaching prod.* Every gym runs at admission, and the same rules run in audit mode in dev first.
- *A poisoned image in the registry.* Koga verifies the signature on the exact digest being deployed.
- *Secrets in Git.* Misty catches them in manifests, and Lorelei scans the whole history.
- *A compromised dependency or CI action.* Agatha and Bruno scan dependencies and code, and every action is pinned to a
  commit SHA.

**What it assumes, or leaves out.**
- *Cluster admins and Kyverno itself are trusted.* Anyone who can edit policies, `PolicyException`s or the Kyverno
  namespace can turn the gyms off.
- *Gyms apply by namespace label.* Only namespaces labelled `poke-league.dev/league=true` are judged, so whoever can
  edit namespace labels can opt out. Production should match every namespace and exclude only system ones.
- *Kyverno runs one replica, and its webhook fails closed.* If Kyverno is down, new pods in league namespaces can't be
  created. That's the safe default, but in production it needs HA (3+ replicas).
- *No runtime detection.* The gyms judge what's *deployed*, not what a running process *does*. See Koffing's discussion
  question.
- *Images are signed with a key on your laptop.* CI signs keylessly, but the local cluster trusts the key pair (see
  [Lance](elite-four.md#-lance-build-sign-attest)).

**Educational simplifications.** One kind node, a plain-HTTP local registry, Kubernetes Secrets stored base64 in etcd
without encryption at rest, no TLS between services, a NodePort for the board, and `kubectl apply` instead of GitOps.
Each is a deliberate trade for a laptop that runs everything with one `make demo`.

**Taking it to EKS, AKS or OpenShift.**

| Here | In production |
|---|---|
| `kind-registry:5000` + cosign key pair | ECR, ACR or Quay, and Koga verifies Lance's **keyless** GitHub Actions identity |
| Kubernetes Secrets | External Secrets Operator or the CSI driver, backed by AWS Secrets Manager, Key Vault or Vault, plus etcd encryption via KMS |
| One Kyverno replica | 3+ replicas, a PodDisruptionBudget, and alerts on webhook latency and errors |
| Pod Security Admission | the same on EKS and AKS; on OpenShift, Security Context Constraints (`restricted-v2`) do this job |
| `make deploy` | Argo CD or Flux, so every change is reviewed, and rollback is a `git revert` |
| default SA tokens off | plus IRSA / EKS Pod Identity or Azure Workload Identity for pods that need cloud access |
| — | runtime detection (Falco or Tetragon), audit logs shipped off-cluster, mTLS (a service mesh) |

## Field notes: real bugs from building this

These all happened while the repo was being built. Each one is a lesson in its own right.

1. **The wrong cluster.** The first `helm install kyverno` ran before the kind cluster existed, so Helm used the
   kubeconfig's *current context*, which was a different, real cluster. Since then every `kubectl` and `helm` call in the
   Makefile carries `--context kind-kanto` / `--kube-context kind-kanto`. Never rely on the current context in automation.
2. **The gym that silently didn't open.** An inline Python one-liner did `open(path, "w").write(open(path).read()...)`.
   The write-open truncates the file before the read, so Koga's policy rendered empty, and Kustomize happily applied 14
   policies instead of 16. Nothing errored. An unsigned image then ran in prod. The fix was
   [`render_policies.py`](../scripts/render_policies.py), which validates its input and output, plus a `make policies`
   check that **all 16 policies are Ready** before it returns. A control you think is on is worse than one you know is off.
3. **Signatures follow content, not tags.** The first Arbok was `docker tag signed-dex-api arbok-unsigned`, so it had
   the same digest and was therefore validly signed, and Koga correctly let it in. Poisoning it needs new bytes.
4. **Verification caches outlive tags.** After that tag was re-pointed at unsigned content, Kyverno's cached "verified"
   result for `:arbok-unsigned` still let it in. Use digests, or at minimum never reuse a tag.
5. **Day-one vulnerability.** govulncheck flagged GO-2026-6617, reachable from `client-go` and from our own HTTP server,
   on the first scan. The fix was bumping `golang.org/x/net` to v0.60.0 and the toolchain to Go 1.27.2. The container
   images were already safe, because the digest-pinned `golang:1.27-alpine` was 1.27.2.
6. **Port 8080 was taken.** A local `llama-server` was bound to `127.0.0.1:8080`, and requests that looked like they
   reached the board went to it. The board now uses 8088.
7. **"Too many open files".** kind nodes run systemd, which needs inotify instances. On Colima (and some Linux hosts)
   the default `fs.inotify.max_user_instances` runs out once a few clusters exist. See Troubleshooting.
8. **The gate that passed when it couldn't tell.** The first "Team Rocket can't get into prod" check failed only if
   `kubectl` printed `deployment.apps/... created`. Anything else counted as "denied as expected", including a request
   that failed because the namespace didn't exist. The check now requires an admission denial, from exactly the
   expected gyms, plus a known-good workload that must still be admitted (see
   [Champion](elite-four.md#-champion-end-to-end)). Same lesson as note 2: a security check must fail closed.

## Troubleshooting

| Symptom | Fix |
|---|---|
| `kind create` fails with *could not find a log line ... Multi-User System* | `colima ssh -- sudo sysctl -w fs.inotify.max_user_instances=1024 fs.inotify.max_user_watches=1048576` (Linux: same `sysctl` on the host) |
| Board at `localhost:8088` doesn't load | `lsof -nP -iTCP:8088`; then `make board-dev` uses a port-forward instead |
| All badges "unknown" | Kyverno's reports controller runs a background scan every few minutes; check `make reports` |
| HP shows "awaiting Trivy scan" | `make trivy`; the first vulnerability DB download takes a few minutes |
| `only N/16 gyms are ready` | `kubectl --context kind-kanto get vpol,ivpol` and `-o yaml` the one that isn't Ready |
