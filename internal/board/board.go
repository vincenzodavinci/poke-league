// Package board builds the League Board: every workload in the league
// namespaces rendered as a Pokémon, with badges from Kyverno PolicyReports
// and HP from Trivy Operator VulnerabilityReports.
package board

import (
	"context"
	"fmt"
	"hash/fnv"
	"strconv"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
)

var (
	policyReportGVR = schema.GroupVersionResource{Group: "wgpolicyk8s.io", Version: "v1alpha2", Resource: "policyreports"}
	vulnReportGVR   = schema.GroupVersionResource{Group: "aquasecurity.github.io", Version: "v1alpha1", Resource: "vulnerabilityreports"}
)

// PokemonAnnotation picks the Pokémon a workload appears as.
const PokemonAnnotation = "poke-league.dev/pokemon"

// Badge is one gym result for a workload.
type Badge struct {
	Gym     string `json:"gym"`
	Status  string `json:"status"` // earned | failed | unknown
	Message string `json:"message,omitempty"`
}

// Vulns counts CVEs by severity across the workload's containers.
type Vulns struct {
	Critical int `json:"critical"`
	High     int `json:"high"`
	Medium   int `json:"medium"`
	Low      int `json:"low"`
}

// Workload is one card on the board.
type Workload struct {
	Namespace string  `json:"namespace"`
	Name      string  `json:"name"`
	Pod       string  `json:"pod"`
	Phase     string  `json:"phase"`
	Pokemon   int     `json:"pokemon"`
	Image     string  `json:"image"`
	Badges    []Badge `json:"badges"`
	Earned    int     `json:"earned"`
	Vulns     *Vulns  `json:"vulns"` // nil when Trivy hasn't scanned it (or isn't installed)
	HP        *int    `json:"hp"`
	Shiny     bool    `json:"shiny"`
}

// League is the whole board.
type League struct {
	Gyms      []Gym      `json:"gyms"`
	Workloads []Workload `json:"workloads"`
}

// Board reads cluster state. Both clients are interfaces so tests can use fakes.
type Board struct {
	Kube       kubernetes.Interface
	Dyn        dynamic.Interface
	Namespaces []string
}

// Collect builds the current League.
func (b *Board) Collect(ctx context.Context) (League, error) {
	league := League{Gyms: Gyms, Workloads: []Workload{}}
	for _, ns := range b.Namespaces {
		pods, err := b.Kube.CoreV1().Pods(ns).List(ctx, metav1.ListOptions{})
		if err != nil {
			return league, fmt.Errorf("list pods in %s: %w", ns, err)
		}
		badges := b.badgesByPod(ctx, ns)
		vulns := b.vulnsByOwner(ctx, ns)
		seen := map[string]bool{}
		for _, pod := range pods.Items {
			if pod.DeletionTimestamp != nil {
				continue
			}
			w := toWorkload(pod, badges[pod.Name], vulns[owner(pod)])
			// One card per app, not per replica.
			if seen[w.Name] {
				continue
			}
			seen[w.Name] = true
			league.Workloads = append(league.Workloads, w)
		}
	}
	return league, nil
}

func toWorkload(pod corev1.Pod, results map[string]Badge, v *Vulns) Workload {
	name := pod.Labels["app.kubernetes.io/name"]
	if name == "" {
		name = pod.Name
	}
	w := Workload{
		Namespace: pod.Namespace,
		Name:      name,
		Pod:       pod.Name,
		Phase:     string(pod.Status.Phase),
		Pokemon:   pokemonFor(pod, name),
		Vulns:     v,
	}
	if len(pod.Spec.Containers) > 0 {
		w.Image = pod.Spec.Containers[0].Image
	}
	for _, g := range Gyms {
		badge, ok := results[g.Key]
		if !ok {
			badge = Badge{Gym: g.Key, Status: "unknown"}
		}
		if badge.Status == "earned" {
			w.Earned++
		}
		w.Badges = append(w.Badges, badge)
	}
	if v != nil {
		hp := HP(*v)
		w.HP = &hp
		w.Shiny = w.Earned == len(Gyms) && v.Critical == 0 && v.High == 0
	}
	return w
}

// HP turns a vulnerability count into hit points out of 100.
func HP(v Vulns) int {
	hp := 100 - 25*v.Critical - 8*v.High - 2*v.Medium
	return max(hp, 0)
}

func pokemonFor(pod corev1.Pod, name string) int {
	if n, err := strconv.Atoi(pod.Annotations[PokemonAnnotation]); err == nil && n >= 1 && n <= 151 {
		return n
	}
	h := fnv.New32a()
	_, _ = h.Write([]byte(name))
	return int(h.Sum32()%151) + 1
}

// owner is the ReplicaSet (or other controller) that Trivy names its report after.
func owner(pod corev1.Pod) string {
	for _, ref := range pod.OwnerReferences {
		if ref.Controller != nil && *ref.Controller {
			return ref.Kind + "/" + ref.Name
		}
	}
	return "Pod/" + pod.Name
}

// badgesByPod reads Kyverno's per-resource PolicyReports: pod -> gym -> badge.
// A missing CRD or RBAC error just leaves every badge "unknown".
func (b *Board) badgesByPod(ctx context.Context, ns string) map[string]map[string]Badge {
	out := map[string]map[string]Badge{}
	list, err := b.Dyn.Resource(policyReportGVR).Namespace(ns).List(ctx, metav1.ListOptions{})
	if err != nil {
		return out
	}
	for _, rep := range list.Items {
		kind, _, _ := unstructured.NestedString(rep.Object, "scope", "kind")
		pod, _, _ := unstructured.NestedString(rep.Object, "scope", "name")
		if kind != "Pod" {
			continue
		}
		results, _, _ := unstructured.NestedSlice(rep.Object, "results")
		for _, r := range results {
			m, ok := r.(map[string]any)
			if !ok {
				continue
			}
			policy, _ := m["policy"].(string)
			gym := gymKey(policy)
			if gym == "" {
				continue
			}
			status := "failed"
			if res, _ := m["result"].(string); res == "pass" || res == "skip" {
				status = "earned"
			}
			msg, _ := m["message"].(string)
			if out[pod] == nil {
				out[pod] = map[string]Badge{}
			}
			// A gym can have several rules: any failure loses the badge.
			if prev, ok := out[pod][gym]; ok && prev.Status == "failed" {
				continue
			}
			out[pod][gym] = Badge{Gym: gym, Status: status, Message: msg}
		}
	}
	return out
}

// vulnsByOwner sums Trivy VulnerabilityReports per owning controller.
func (b *Board) vulnsByOwner(ctx context.Context, ns string) map[string]*Vulns {
	out := map[string]*Vulns{}
	list, err := b.Dyn.Resource(vulnReportGVR).Namespace(ns).List(ctx, metav1.ListOptions{})
	if err != nil {
		return out
	}
	for _, rep := range list.Items {
		labels := rep.GetLabels()
		key := labels["trivy-operator.resource.kind"] + "/" + labels["trivy-operator.resource.name"]
		v := out[key]
		if v == nil {
			v = &Vulns{}
			out[key] = v
		}
		v.Critical += count(rep, "criticalCount")
		v.High += count(rep, "highCount")
		v.Medium += count(rep, "mediumCount")
		v.Low += count(rep, "lowCount")
	}
	return out
}

func count(u unstructured.Unstructured, field string) int {
	n, _, _ := unstructured.NestedInt64(u.Object, "report", "summary", field)
	return int(n)
}
