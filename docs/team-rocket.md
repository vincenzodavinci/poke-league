# Team Rocket's hideout: the CTF lab

> *Prepare for trouble, and make it double!*

`make rocket` deploys three insecure workloads into the `team-rocket` namespace. The gyms judge them in **audit** mode,
and Pod Security Admission is set to `privileged` there, so they actually run and you can attack them.
Everything stays inside your local kind cluster.

Each lab has two halves:
1. **Attack.** Prove why the misconfiguration matters.
2. **Fix.** Change the manifest until the workload earns all 8 badges and turns shiny on the board.

```bash
alias k='kubectl --context kind-kanto'
```

---

## <img src="../assets/crests/52.svg" width="48"> Meowth: the copy-pasted Deployment {#meowth}

[`deploy/rocket/meowth.yaml`](../deploy/rocket/meowth.yaml). **1/8 badges, and fainted once Trivy has scanned it.**

Root, a public `nginx:1.27` image, a plaintext `DB_PASSWORD`, no limits, no probes, and the namespace's `default`
ServiceAccount with its API token mounted. Every cluster has a few of these.

### Attack 1: the password is already leaked

```bash
k -n team-rocket get deploy meowth -o jsonpath='{.spec.template.spec.containers[0].env}'
```

Anyone who can *read Deployments* can read the password. So can anyone with access to the Git repo, forever.

### Attack 2: steal the token

```bash
k -n team-rocket exec -it deploy/meowth -- sh
# inside the pod:
TOKEN=$(cat /var/run/secrets/kubernetes.io/serviceaccount/token)
curl -sk -H "Authorization: Bearer $TOKEN" https://kubernetes.default.svc/api/v1/namespaces/team-rocket/secrets
```

`Forbidden`. The `default` ServiceAccount has no permissions, so right now the token is harmless. Now play the
well-meaning teammate who "just makes it work":

```bash
k -n team-rocket create secret generic giovannis-vault --from-literal=master-ball-code=150-mewtwo
k -n team-rocket create rolebinding just-make-it-work --clusterrole=edit --serviceaccount=team-rocket:default
```

Run the same `curl` from inside Meowth again, adding `/giovannis-vault` to the URL, and base64-decode the result. You've
read a secret using nothing but a compromised web container. And because it's the `default` ServiceAccount,
**every pod in the namespace** just got the same power.

Clean up: `k -n team-rocket delete rolebinding just-make-it-work; k -n team-rocket delete secret giovannis-vault`

### Attack 3: count the CVEs

After `make trivy`:

```bash
k -n team-rocket get vulnerabilityreports -o wide
```

### Fix it

Get Meowth to 8/8. You'll need to:
- run as non-root with a read-only root filesystem (hint: `nginxinc/nginx-unprivileged` listens on 8080 and needs
  writable `emptyDir`s for `/tmp` and its cache dir),
- push the image into the league registry and sign it (`docker push localhost:5001/poke-league/...`, then the `cosign sign`
  line from the Makefile),
- move `DB_PASSWORD` to a `secretKeyRef`,
- give it its own ServiceAccount with `automountServiceAccountToken: false`,
- add requests/limits and probes.

When it's shiny, try deploying it to `indigo-plateau`.

---

## <img src="../assets/crests/109.svg" width="48"> Koffing: the forgotten debug pod {#koffing}

[`deploy/rocket/koffing.yaml`](../deploy/rocket/koffing.yaml). **1/8 badges.**

Someone needed to "check something on the node" and never cleaned up: `privileged: true`, `hostPID: true`, and the node's
root filesystem mounted at `/host`.

### Attack: become the node

```bash
k -n team-rocket exec -it deploy/koffing -- chroot /host sh
# you are now in the node's filesystem, as root:
ls /etc/kubernetes/                 # admin.conf, super-admin.conf, pki/ ...
ps -e | head                        # the node's processes (hostPID): kubelet, containerd, etcd ...
```

`/etc/kubernetes/admin.conf` is a **cluster-admin kubeconfig**. One debug pod gives you the whole cluster. On a cloud
node you'd also find kubelet credentials and could reach the instance metadata service.

### Fix it

Usually the right fix is **delete it**. For real node debugging, use `kubectl debug node/<name>`, which is temporary,
audited and leaves nothing behind. If you want the badges anyway, Koffing needs everything Brock, Blaine and Giovanni ask
for, which ends up as a completely different pod. That's the point.

**Discuss:** Blaine catches this at admission. What catches an attacker who already *has* a pod like this? (Runtime
detection: look at Falco or Tetragon, and add one as a tenth gym.)

---

## <img src="../assets/crests/24.svg" width="48"> Arbok: the poisoned image {#arbok}

[`deploy/rocket/arbok.yaml`](../deploy/rocket/arbok.yaml). **7/8 badges.**

The manifest is flawless. The image is `kind-registry:5000/poke-league/dex-api:arbok-poisoned`: the right registry, a
real tag, the right name. `make rocket` built it as dex-api plus one extra layer, which is what an attacker with push
access to your registry would do, and nobody signed it.

### Attack: walk it into prod

```bash
sed 's/namespace: team-rocket/namespace: indigo-plateau/' deploy/rocket/arbok.yaml | k apply -f -
```

```
admission webhook "ivpol.validate.kyverno.svc-fail" denied the request:
Policy koga-signed-images-enforce failed: Koga: every image must be signed with the league's cosign key.
```

Seven gyms waved it through. Only Koga checks who built the bytes.

### Investigate

```bash
# The signature lives in the registry next to the image, keyed by digest:
docker run --rm --network kind -v $PWD/.cosign:/keys ghcr.io/sigstore/cosign/cosign:v2.6.1 \
  verify --key /keys/cosign.pub --allow-insecure-registry --allow-http-registry --insecure-ignore-tlog \
  kind-registry:5000/poke-league/dex-api:dev            # verified
# ...and the same for :arbok-poisoned                   # no signatures found
```

### Two things worth knowing

1. **Re-tagging doesn't remove a signature.** Cosign signs the *digest* (the content). `docker tag dex-api:dev
   dex-api:anything` is still signed, because it's the same bytes. The first version of this lab made that exact mistake,
   which is why `make rocket` now adds a layer.
2. **Tags are mutable, and verification results get cached.** Kyverno caches signature checks by image reference. Point a
   tag at new content and a cached "valid" can outlive the change. Deploy by digest (`make deploy` does) and both problems
   disappear.

### Fix it

Sign it (`cosign sign` with the league key, using the line in the Makefile's `sign` target), then watch Arbok go 8/8.
Then ask whether you *should* have signed it. Signing proves who built an image, not that it's safe. In real pipelines
only CI holds the signing identity (see Lance in [elite-four.md](elite-four.md)).

---

## Bonus challenges

- **Ekans and SSRF.** Write a sprite service that fetches `?url=` from anywhere, the classic SSRF. Deploy it to
  team-rocket and reach something it shouldn't (the API server, another namespace's service). Then explain why
  [`internal/sprites`](../internal/sprites/sprites.go) embeds its images instead, and why the `league-board`
  NetworkPolicy only allows egress to two pods and the API server.
- **Lateral movement.** From Meowth, try to reach `dex-api` in `pallet-town`
  (`curl dex-api.pallet-town.svc/api/pokemon/1`). Why does it time out? (Read
  [`networkpolicies.yaml`](../deploy/k8s/base/networkpolicies.yaml).)
