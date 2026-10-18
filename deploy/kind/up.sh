#!/usr/bin/env bash
# Create the "kanto" kind cluster plus a local OCI registry, following
# https://kind.sigs.k8s.io/docs/user/local-registry/
#
#   host pushes to   localhost:5001/...
#   cluster pulls    kind-registry:5000/...   (same registry, seen from the kind network)
set -euo pipefail

REG_NAME=kind-registry
REG_PORT=5001
HERE="$(cd "$(dirname "$0")" && pwd)"

if [ "$(docker inspect -f '{{.State.Running}}' "$REG_NAME" 2>/dev/null || true)" != "true" ]; then
  docker run -d --restart=always -p "127.0.0.1:${REG_PORT}:5000" --network bridge --name "$REG_NAME" registry:2
fi

if ! kind get clusters | grep -qx kanto; then
  if ! kind create cluster --config "$HERE/cluster.yaml"; then
    cat >&2 <<'HINT'

kind could not start the node. The usual cause is the inotify limit
("Too many open files" in `docker logs kanto-control-plane`):
  Colima: colima ssh -- sudo sysctl -w fs.inotify.max_user_instances=1024 fs.inotify.max_user_watches=1048576
  Linux:  sudo sysctl -w fs.inotify.max_user_instances=1024 fs.inotify.max_user_watches=1048576
HINT
    exit 1
  fi
fi

# Tell containerd on the node where kind-registry:5000 lives (plain HTTP, local only).
for node in $(kind get nodes --name kanto); do
  docker exec "$node" mkdir -p /etc/containerd/certs.d/kind-registry:5000
  printf '[host."http://%s:5000"]\n' "$REG_NAME" |
    docker exec -i "$node" cp /dev/stdin /etc/containerd/certs.d/kind-registry:5000/hosts.toml
done

if [ "$(docker inspect -f '{{json .NetworkSettings.Networks.kind}}' "$REG_NAME")" = "null" ]; then
  docker network connect kind "$REG_NAME"
fi

# Advertise the registry to tooling (KEP-1755).
kubectl --context kind-kanto apply -f - <<EOF
apiVersion: v1
kind: ConfigMap
metadata:
  name: local-registry-hosting
  namespace: kube-public
data:
  localRegistryHosting.v1: |
    host: "localhost:${REG_PORT}"
    hostFromClusterNetwork: "${REG_NAME}:5000"
EOF

echo "kanto is up: registry localhost:${REG_PORT} (in-cluster ${REG_NAME}:5000)"
