package board

import "strings"

// Gym is one Kanto gym. Each gym is a Kyverno ValidatingPolicy (Koga: an
// ImageValidatingPolicy) in deploy/policies/gyms; passing it earns the badge.
type Gym struct {
	Key     string `json:"key"`
	Leader  string `json:"leader"`
	Badge   string `json:"badge"`
	BadgeID int    `json:"badgeId"` // sprites/badges/<id>.png
	Type    string `json:"type"`
	Pokemon int    `json:"pokemon"` // the leader's ace, for the gym card
	Policy  string `json:"policy"`
	Lesson  string `json:"lesson"`
}

// Gyms in badge order. The order matches the deploy/policies/gyms file names.
var Gyms = []Gym{
	{"brock", "Brock", "Boulder", 1, "rock", 95, "brock-hardened-pods",
		"Rock-solid pods: non-root, read-only root filesystem, no privilege escalation, all capabilities dropped, seccomp on."},
	{"misty", "Misty", "Cascade", 2, "water", 121, "misty-no-plaintext-secrets",
		"No leaks: secret-looking env vars must come from a Secret, never a literal in the manifest."},
	{"surge", "Lt. Surge", "Thunder", 3, "electric", 26, "surge-resource-limits",
		"Power budget: every container declares CPU/memory requests and a memory limit, so one pod can't starve the node."},
	{"erika", "Erika", "Rainbow", 4, "grass", 45, "erika-trusted-images",
		"Know your roots: images come from the league registry, pinned to a version, never :latest."},
	{"koga", "Koga", "Soul", 5, "poison", 110, "koga-signed-images",
		"Antidote for poisoned images: only images signed with the league's cosign key may run."},
	{"sabrina", "Sabrina", "Marsh", 6, "psychic", 65, "sabrina-health-probes",
		"Foresight: liveness and readiness probes let Kubernetes see trouble before users do."},
	{"blaine", "Blaine", "Volcano", 7, "fire", 59, "blaine-no-host-access",
		"Don't burn the host: no privileged containers, host namespaces or hostPath mounts."},
	{"giovanni", "Giovanni", "Earth", 8, "ground", 112, "giovanni-least-privilege",
		"Ground rules for identity: no default ServiceAccount, and no API token unless the workload declares it needs one."},
}

// gymKey maps a policy name to its gym. Prod copies carry an "-enforce" suffix
// (deploy/policies/enforce) and count toward the same badge.
func gymKey(policy string) string {
	policy = strings.TrimSuffix(policy, "-enforce")
	for _, g := range Gyms {
		if g.Policy == policy {
			return g.Key
		}
	}
	return ""
}
