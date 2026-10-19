// Package board builds the League Board: every workload in the league
// namespaces rendered as a Pokémon, with badges from Kyverno PolicyReports
// and HP from Trivy Operator VulnerabilityReports.
//
// The board is evidence, so it never rounds up: a badge is earned only when a
// gym evaluated the pod and passed it, a card shows its worst replica, and a
// clean CVE count needs a complete Trivy report. Anything the board couldn't
// read is shown as unknown and listed in League.Problems, never as a pass.
package board

import (
	"context"
	"fmt"
	"hash/fnv"
	"strconv"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
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

// Badge statuses, best to worst. A gym with several rules, or a workload with
// several replicas, gets the worst status any of them reports.
const (
	Earned  = "earned"  // the gym evaluated the pod and passed it
	Unknown = "unknown" // no report yet, or the gym skipped the pod: not proven either way
	Failed  = "failed"
)

var badgeRank = map[string]int{Earned: 0, Unknown: 1, Failed: 2}

// Badge is one gym result for a workload.
type Badge struct {
	Gym     string `json:"gym"`
	Status  string `json:"status"` // earned | unknown | failed
	Message string `json:"message,omitempty"`
}

// Scan states for a workload's CVE data. Only Scanned means a clean count is real.
const (
	Scanned      = "scanned"       // a complete Trivy report
	NotInstalled = "not-installed" // Trivy Operator isn't in the cluster (`make trivy`)
	NotScanned   = "not-scanned"   // Trivy is installed but hasn't reported on this workload yet
	ScanError    = "error"         // the reports couldn't be read, or one was incomplete
)

var scanRank = map[string]int{Scanned: 0, NotInstalled: 1, NotScanned: 2, ScanError: 3}

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
	Vulns     *Vulns  `json:"vulns"` // nil unless Scan is "scanned"
	Scan      string  `json:"scan"`
	HP        *int    `json:"hp"`
	Shiny     bool    `json:"shiny"`
	Replicas  int     `json:"replicas"` // pods behind this card; it shows the worst of them
}

// League is the whole board.
type League struct {
	Gyms      []Gym      `json:"gyms"`
	Workloads []Workload `json:"workloads"`
	Problems  []string   `json:"problems"` // reports the board couldn't read: its view is incomplete
}

// Board reads cluster state. Both clients are interfaces so tests can use fakes.
type Board struct {
	Kube       kubernetes.Interface
	Dyn        dynamic.Interface
	Namespaces []string
}

// Collect builds the current League.
func (b *Board) Collect(ctx context.Context) (League, error) {
	league := League{Gyms: Gyms, Workloads: []Workload{}, Problems: []string{}}
	for _, ns := range b.Namespaces {
		pods, err := b.Kube.CoreV1().Pods(ns).List(ctx, metav1.ListOptions{})
		if err != nil {
			return league, fmt.Errorf("list pods in %s: %w", ns, err)
		}
		badges, err := b.badgesByPod(ctx, ns)
		if err != nil {
			league.Problems = append(league.Problems, err.Error())
		}
		scans, err := b.vulnsByOwner(ctx, ns)
		if err != nil {
			league.Problems = append(league.Problems, err.Error())
		}
		// One card per app, showing its worst replica: a healthy pod must
		// never hide a failing one (a rollout, or audit mode in dev).
		var order []string
		cards := map[string]Workload{}
		for _, pod := range pods.Items {
			if pod.DeletionTimestamp != nil {
				continue
			}
			w := toWorkload(pod, badges[pod.Name], scans.forOwner(owner(pod)))
			prev, seen := cards[w.Name]
			if !seen {
				order = append(order, w.Name)
				cards[w.Name] = w
				continue
			}
			cards[w.Name] = worse(prev, w)
		}
		for _, name := range order {
			league.Workloads = append(league.Workloads, cards[name])
		}
	}
	return league, nil
}

// worse merges two replicas of one app into a card showing the worse of each.
func worse(a, b Workload) Workload {
	out := a
	if b.Earned < a.Earned || (b.Earned == a.Earned && b.Phase != string(corev1.PodRunning)) {
		out = b // the card names the worst pod, so you know which one to look at
	}
	out.Replicas = a.Replicas + b.Replicas
	out.Badges = make([]Badge, len(a.Badges))
	out.Earned = 0
	for i := range a.Badges {
		out.Badges[i] = a.Badges[i]
		if badgeRank[b.Badges[i].Status] > badgeRank[a.Badges[i].Status] {
			out.Badges[i] = b.Badges[i]
		}
		if out.Badges[i].Status == Earned {
			out.Earned++
		}
	}
	out.Scan, out.Vulns = a.Scan, a.Vulns
	if scanRank[b.Scan] > scanRank[a.Scan] {
		out.Scan, out.Vulns = b.Scan, b.Vulns
	} else if a.Scan == Scanned && b.Scan == Scanned {
		out.Vulns = &Vulns{
			Critical: max(a.Vulns.Critical, b.Vulns.Critical), High: max(a.Vulns.High, b.Vulns.High),
			Medium: max(a.Vulns.Medium, b.Vulns.Medium), Low: max(a.Vulns.Low, b.Vulns.Low),
		}
	}
	setHealth(&out)
	return out
}

func toWorkload(pod corev1.Pod, results map[string]Badge, scan scanResult) Workload {
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
		Vulns:     scan.vulns,
		Scan:      scan.state,
		Replicas:  1,
	}
	if len(pod.Spec.Containers) > 0 {
		w.Image = pod.Spec.Containers[0].Image
	}
	for _, g := range Gyms {
		badge, ok := results[g.Key]
		if !ok {
			badge = Badge{Gym: g.Key, Status: Unknown}
		}
		if badge.Status == Earned {
			w.Earned++
		}
		w.Badges = append(w.Badges, badge)
	}
	setHealth(&w)
	return w
}

// setHealth derives HP and shininess. Shiny needs every badge AND a real,
// complete scan with no high or critical CVEs: missing data is never clean.
func setHealth(w *Workload) {
	w.HP, w.Shiny = nil, false
	if w.Scan != Scanned || w.Vulns == nil {
		w.Vulns = nil
		return
	}
	hp := HP(*w.Vulns)
	w.HP = &hp
	w.Shiny = w.Earned == len(Gyms) && w.Vulns.Critical == 0 && w.Vulns.High == 0
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
// If Kyverno's CRD isn't there yet every badge stays unknown; any other error
// is returned, so the board says its view is incomplete instead of looking empty.
func (b *Board) badgesByPod(ctx context.Context, ns string) (map[string]map[string]Badge, error) {
	out := map[string]map[string]Badge{}
	list, err := b.Dyn.Resource(policyReportGVR).Namespace(ns).List(ctx, metav1.ListOptions{})
	if apierrors.IsNotFound(err) {
		return out, nil
	}
	if err != nil {
		return out, fmt.Errorf("policy reports in %s: %w", ns, err)
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
			msg, _ := m["message"].(string)
			var status string
			switch res, _ := m["result"].(string); res {
			case "pass":
				status = Earned
			case "skip":
				// Skipped means the gym didn't evaluate this pod: no evidence either way.
				status, msg = Unknown, "skipped: "+msg
			default: // fail, warn, error
				status = Failed
			}
			if out[pod] == nil {
				out[pod] = map[string]Badge{}
			}
			// A gym can have several rules: the badge takes the worst result.
			if prev, ok := out[pod][gym]; ok && badgeRank[prev.Status] >= badgeRank[status] {
				continue
			}
			out[pod][gym] = Badge{Gym: gym, Status: status, Message: msg}
		}
	}
	return out, nil
}

type scanResult struct {
	state string
	vulns *Vulns
}

// scans is what Trivy reported for one namespace.
type scans struct {
	state  string            // NotInstalled or ScanError for the whole namespace, else Scanned
	owners map[string]*Vulns // owner -> summed counts
	broken map[string]bool   // owners with at least one incomplete report
}

func (s scans) forOwner(key string) scanResult {
	switch {
	case s.state != Scanned:
		return scanResult{state: s.state}
	case s.broken[key]:
		return scanResult{state: ScanError}
	case s.owners[key] == nil:
		return scanResult{state: NotScanned}
	}
	return scanResult{state: Scanned, vulns: s.owners[key]}
}

// vulnsByOwner sums Trivy VulnerabilityReports per owning controller.
func (b *Board) vulnsByOwner(ctx context.Context, ns string) (scans, error) {
	out := scans{state: Scanned, owners: map[string]*Vulns{}, broken: map[string]bool{}}
	list, err := b.Dyn.Resource(vulnReportGVR).Namespace(ns).List(ctx, metav1.ListOptions{})
	if apierrors.IsNotFound(err) {
		out.state = NotInstalled // optional: `make trivy`
		return out, nil
	}
	if err != nil {
		out.state = ScanError
		return out, fmt.Errorf("vulnerability reports in %s: %w", ns, err)
	}
	for _, rep := range list.Items {
		labels := rep.GetLabels()
		key := labels["trivy-operator.resource.kind"] + "/" + labels["trivy-operator.resource.name"]
		counts := map[string]int{}
		for _, field := range []string{"criticalCount", "highCount", "mediumCount", "lowCount"} {
			n, found, err := unstructured.NestedInt64(rep.Object, "report", "summary", field)
			if !found || err != nil {
				out.broken[key] = true // a missing count is not a zero
			}
			counts[field] = int(n)
		}
		v := out.owners[key]
		if v == nil {
			v = &Vulns{}
			out.owners[key] = v
		}
		v.Critical += counts["criticalCount"]
		v.High += counts["highCount"]
		v.Medium += counts["mediumCount"]
		v.Low += counts["lowCount"]
	}
	return out, nil
}
