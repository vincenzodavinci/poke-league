# Poké League: `make demo` builds the whole league on a local kind cluster.
#
# Every kubectl/helm call is pinned to the kind-kanto context, so nothing here
# can touch whatever cluster your kubeconfig happens to point at.

SHELL := /usr/bin/env bash
.SHELLFLAGS := -euo pipefail -c
.DEFAULT_GOAL := help

CTX      := kind-kanto
K        := kubectl --context $(CTX)
H        := helm --kube-context $(CTX)
REG      := localhost:5001/poke-league
REG_IN   := kind-registry:5000/poke-league
APPS     := dex-api sprite-svc league-board
TAG      := $(shell git rev-parse --short HEAD 2>/dev/null || echo dev)
BUILD    := .build
BUILDPLATFORM := linux/$(shell uname -m | sed 's/x86_64/amd64/; s/aarch64/arm64/')

# Where the league's images come from, and who Koga trusts:
#   TRUST=key  (default) built here, pushed to the local registry, signed with
#              the key pair in .cosign/. Works offline, no GitHub account needed.
#   TRUST=ci   built by CI (Lance) and pulled from GHCR. Koga admits an image
#              only if this repo's workflow, running on main, signed it.
TRUST       ?= key
REPO        ?= $(shell git remote get-url origin 2>/dev/null | sed -E 's|^.*github\.com[:/]||; s|\.git$$||')
GHCR        := ghcr.io/$(shell echo '$(REPO)' | tr '[:upper:]' '[:lower:]')
CI_IDENTITY ?= https://github.com/$(REPO)/.github/workflows/league.yml@refs/heads/main
CI_SHA      ?= $(shell git rev-parse origin/main 2>/dev/null)
ifeq ($(TRUST),ci)
SUPPLY      := ci-images
IMAGES_FROM := $(GHCR)
ARBOK_FROM  := $(GHCR)
POLICY_DEPS :=
RENDER_ARGS := --trust ci --registry $(GHCR)/ --identity $(CI_IDENTITY)
# Arbok's image sits in the local registry, not GHCR, so Erika refuses it first (see expect_denied.sh)
ARBOK_GYMS  := erika
else ifeq ($(TRUST),key)
SUPPLY      := images sign
IMAGES_FROM := $(REG_IN)
ARBOK_FROM  := $(REG)
POLICY_DEPS := .cosign/cosign.key
RENDER_ARGS :=
ARBOK_GYMS  := koga
else
$(error TRUST must be key or ci, not "$(TRUST)")
endif

# What a registry may answer with for an image: index or single manifest, OCI or Docker.
MANIFEST_TYPES := application/vnd.oci.image.index.v1+json, application/vnd.docker.distribution.manifest.list.v2+json, application/vnd.oci.image.manifest.v1+json, application/vnd.docker.distribution.manifest.v2+json

KYVERNO_CHART := 3.9.1
TRIVY_CHART   := 0.37.0
COSIGN_IMAGE  := ghcr.io/sigstore/cosign/cosign:v2.6.1
KYVERNO_CLI   := ghcr.io/kyverno/kyverno-cli:v1.19.1
HADOLINT      := hadolint/hadolint:v2.14.0

# cosign runs in a container on the kind network so it reaches the registry
# by the same name the cluster uses.
COSIGN := docker run --rm --network kind --user $$(id -u):$$(id -g) \
	-v $(CURDIR)/.cosign:/keys -w /keys -e COSIGN_PASSWORD= $(COSIGN_IMAGE)

##@ Learn
help: ## Show this help
	@awk 'BEGIN{FS=":.*##"; printf "\n\033[1mPoké League\033[0m  make <target>\n"} \
	/^##@/{printf "\n\033[1;33m%s\033[0m\n", substr($$0,5)} \
	/^[a-zA-Z_-]+:.*##/{printf "  \033[36m%-14s\033[0m %s\n", $$1, $$2}' $(MAKEFILE_LIST)

##@ Develop
assets: ## Re-fetch the Kanto dex facts from PokeAPI, then regenerate the art
	python3 -I scripts/fetch_assets.py

crests: ## Regenerate the original art (crests, gym sigils, favicon) from data/kanto.json
	python3 -I scripts/make_crests.py

test: ## Go unit tests + Kyverno policy tests
	go test ./...
	$(MAKE) policy-test

policy-test: ## Test every gym against good and bad pods with the Kyverno CLI
	docker run --rm -v $(CURDIR)/deploy/policies:/policies -w /policies/tests $(KYVERNO_CLI) test .

lint: ## go vet + Dockerfile lint
	go vet ./...
	docker run --rm -i $(HADOLINT) < build/Dockerfile

##@ Build the league
demo: cluster kyverno $(SUPPLY) policies deploy rocket ## Everything, start to finish (TRUST=ci: from CI's signed GHCR images)
	@echo; echo "League Board: http://localhost:8088   (make trivy adds CVE-based HP)"

cluster: ## Create the kind cluster "kanto" + local registry
	./deploy/kind/up.sh

kyverno: ## Install Kyverno (the gym leaders)
	helm repo add kyverno https://kyverno.github.io/kyverno/ >/dev/null 2>&1 || true
	$(H) upgrade --install kyverno kyverno/kyverno --version $(KYVERNO_CHART) \
		-n kyverno --create-namespace --wait --timeout 10m \
		--set admissionController.replicas=1 --set backgroundController.replicas=1 \
		--set reportsController.replicas=1 --set cleanupController.replicas=1

images: ## Build and push all service images; record their digests
	@mkdir -p $(BUILD)/digests
	@for app in $(APPS); do \
	  echo "==> $$app"; \
	  docker build -q --build-arg APP=$$app --build-arg BUILDPLATFORM=$(BUILDPLATFORM) -f build/Dockerfile -t $(REG)/$$app:$(TAG) . >/dev/null; \
	  docker push -q $(REG)/$$app:$(TAG) >/dev/null; \
	  docker inspect --format '{{index .RepoDigests 0}}' $(REG)/$$app:$(TAG) | cut -d@ -f2 > $(BUILD)/digests/$$app; \
	  echo "    $(REG_IN)/$$app@$$(cat $(BUILD)/digests/$$app)"; \
	done

.cosign/cosign.key:
	@mkdir -p .cosign
	$(COSIGN) generate-key-pair
	@echo "Generated .cosign/cosign.key (gitignored) and .cosign/cosign.pub"

ci-images: ## TRUST=ci: use the images CI built and signed for CI_SHA (no local build, no key)
	@test -n "$(CI_SHA)" || { echo "CI_SHA is empty: set it to a commit that CI published"; exit 1; }
	@echo "==> images Lance published for $(CI_SHA)"
	python3 -I scripts/ghcr_digests.py $(GHCR) $(CI_SHA) $(APPS)

sign: .cosign/cosign.key ## Sign every image with the league key (Lance's job in CI)
	@for app in $(APPS); do \
	  ref=$(REG_IN)/$$app@$$(cat $(BUILD)/digests/$$app); \
	  echo "==> signing $$ref"; \
	  $(COSIGN) sign --yes --key /keys/cosign.key --allow-insecure-registry --allow-http-registry --tlog-upload=false $$ref; \
	done

policies: $(POLICY_DEPS) ## Open the 8 gyms (render Koga's root of trust, apply)
	python3 -I scripts/render_policies.py $(RENDER_ARGS)
	$(K) apply -k $(BUILD)/policies
	@echo "Waiting for all 17 policies (8 gyms in audit + 8 enforced, and prod's no-debug guard)..."
	@for i in $$(seq 1 60); do \
	  ready=$$($(K) get vpol,ivpol -o jsonpath='{range .items[*]}{.status.conditionStatus.ready}{"\n"}{end}' | grep -c true || true); \
	  [ "$$ready" = 17 ] && { echo "17/17 policies ready: the gyms are open"; exit 0; }; sleep 2; \
	done; echo "only $$ready/17 policies are ready:"; $(K) get vpol,ivpol; exit 1

deploy: ## Deploy the app to Pallet Town (dev) and Indigo Plateau (prod), pinned by digest
	$(K) apply -f deploy/k8s/cluster/
	@for env in pallet-town indigo-plateau; do \
	  mkdir -p $(BUILD)/overlays/$$env; \
	  { echo "resources: [../../../deploy/k8s/overlays/$$env]"; echo "images:"; \
	    for app in $(APPS); do \
	      echo "  - {name: $(REG_IN)/$$app, newName: $(IMAGES_FROM)/$$app, digest: $$(cat $(BUILD)/digests/$$app)}"; \
	    done; } > $(BUILD)/overlays/$$env/kustomization.yaml; \
	  $(K) apply -k $(BUILD)/overlays/$$env; \
	done
	$(K) -n indigo-plateau rollout status deploy --timeout=180s
	$(K) -n pallet-town rollout status deploy --timeout=180s

rocket: ## Release Team Rocket into their hideout (the CTF lab)
	@# Arbok: dex-api plus one extra layer. New content means a new digest that
	@# nobody signed. (A plain `docker tag` would keep the digest, and with it
	@# the signature: cosign signs content, not tags.)
	printf 'FROM %s\nLABEL dev.poke-league.tampered-by="team-rocket"\n' "$(ARBOK_FROM)/dex-api@$$(cat $(BUILD)/digests/dex-api)" | \
	  docker build -q -t $(REG)/dex-api:arbok-poisoned - >/dev/null
	docker push -q $(REG)/dex-api:arbok-poisoned >/dev/null
	@# Arbok is pinned by digest like every league image (Erika requires it), so
	@# Koga is the only gym it fails. The registry's answer is the digest that counts.
	@mkdir -p $(BUILD)/rocket $(BUILD)/digests
	curl -fsSI -H 'Accept: $(MANIFEST_TYPES)' http://localhost:5001/v2/poke-league/dex-api/manifests/arbok-poisoned | \
	  awk 'tolower($$1) == "docker-content-digest:" {print $$2}' | tr -d '\r' > $(BUILD)/digests/arbok
	@grep -Eqx 'sha256:[a-f0-9]{64}' $(BUILD)/digests/arbok || { echo "no digest for dex-api:arbok-poisoned"; exit 1; }
	cp deploy/rocket/*.yaml $(BUILD)/rocket/
	sed "s|dex-api:arbok-poisoned|dex-api@$$(cat $(BUILD)/digests/arbok)|" deploy/rocket/arbok.yaml > $(BUILD)/rocket/arbok.yaml
	$(K) apply -k $(BUILD)/rocket
	@echo "Team Rocket is blasting off... check the board."

trivy: ## Optional: Trivy Operator scans every workload; CVEs become HP damage
	helm repo add aqua https://aquasecurity.github.io/helm-charts/ >/dev/null 2>&1 || true
	$(H) upgrade --install trivy-operator aqua/trivy-operator --version $(TRIVY_CHART) \
		-n trivy-system --create-namespace --wait --timeout 10m \
		--set trivy.ignoreUnfixed=true \
		--set operator.scanJobsConcurrentLimit=2 \
		--set "trivy.insecureRegistries.league=kind-registry:5000" \
		--set targetNamespaces="pallet-town\,indigo-plateau\,team-rocket"

##@ Play
board: ## Open the League Board (prod)
	open http://localhost:8088 2>/dev/null || xdg-open http://localhost:8088

board-dev: ## Port-forward the Pallet Town board to http://localhost:8081
	$(K) -n pallet-town port-forward svc/league-board 8081:80

reports: ## Show every gym result straight from Kyverno's PolicyReports
	$(K) get policyreports -A -o custom-columns='NS:.metadata.namespace,KIND:.scope.kind,NAME:.scope.name,PASS:.summary.pass,FAIL:.summary.fail'

check: ## Prove the league works: 8/8 everywhere, Team Rocket refused by exactly the right gyms
	python3 -I scripts/check_board.py --arbok-fails $(ARBOK_GYMS) $(if $(filter ci,$(TRUST)),--image-prefix $(GHCR)/)
	@for app in meowth koffing arbok; do \
	  case $$app in \
	    meowth)  gyms="brock erika giovanni misty sabrina surge" ;; \
	    koffing) gyms="blaine brock erika giovanni sabrina surge" ;; \
	    arbok)   gyms="$(ARBOK_GYMS)" ;; \
	  esac; \
	  sed 's/namespace: team-rocket/namespace: indigo-plateau/' $(BUILD)/rocket/$$app.yaml | \
	    CTX=$(CTX) scripts/expect_denied.sh $$app "$$gyms"; \
	done
	@# The poisoned image hidden in an init container, behind a properly signed app image.
	python3 -I scripts/sneak_init.py $(BUILD)/rocket/arbok.yaml "$(IMAGES_FROM)/dex-api@$$(cat $(BUILD)/digests/dex-api)" | \
	  sed 's/namespace: team-rocket/namespace: indigo-plateau/' | \
	  CTX=$(CTX) scripts/expect_denied.sh "arbok (init container)" "$(ARBOK_GYMS)"
	CTX=$(CTX) scripts/expect_debug_denied.sh
	CTX=$(CTX) scripts/expect_admitted.sh dex-api

challenge: ## Try to sneak Team Rocket into prod (spoiler: the gyms say no)
	-sed 's/namespace: team-rocket/namespace: indigo-plateau/' deploy/rocket/meowth.yaml | $(K) apply --dry-run=server -f - 2>&1 | fold -s -w 120

##@ Clean up
down: ## Delete the cluster and the local registry
	kind delete cluster --name kanto
	docker rm -f kind-registry >/dev/null 2>&1 || true
	rm -rf $(BUILD)

.PHONY: help assets crests test policy-test lint demo cluster kyverno images ci-images sign policies deploy rocket trivy board board-dev reports check challenge down
