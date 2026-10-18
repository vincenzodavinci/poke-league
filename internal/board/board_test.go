package board

import (
	"context"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	dynfake "k8s.io/client-go/dynamic/fake"
	"k8s.io/client-go/kubernetes/fake"
)

func pod(ns, name, app, pokemon, rs string) *corev1.Pod {
	yes := true
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Namespace:       ns,
			Name:            name,
			Labels:          map[string]string{"app.kubernetes.io/name": app},
			Annotations:     map[string]string{PokemonAnnotation: pokemon},
			OwnerReferences: []metav1.OwnerReference{{Kind: "ReplicaSet", Name: rs, Controller: &yes}},
		},
		Spec:   corev1.PodSpec{Containers: []corev1.Container{{Name: "app", Image: "kind-registry:5000/poke-league/" + app + ":v1"}}},
		Status: corev1.PodStatus{Phase: corev1.PodRunning},
	}
}

func policyReport(ns, podName string, results map[string]string) *unstructured.Unstructured {
	var rs []any
	for policy, result := range results {
		rs = append(rs, map[string]any{"policy": policy, "rule": "r", "result": result, "message": policy + " says " + result})
	}
	return &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "wgpolicyk8s.io/v1alpha2",
		"kind":       "PolicyReport",
		"metadata":   map[string]any{"namespace": ns, "name": "rep-" + podName},
		"scope":      map[string]any{"kind": "Pod", "name": podName},
		"results":    rs,
	}}
}

func vulnReport(ns, rs string, crit, high int64) *unstructured.Unstructured {
	return &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "aquasecurity.github.io/v1alpha1",
		"kind":       "VulnerabilityReport",
		"metadata": map[string]any{"namespace": ns, "name": "replicaset-" + rs + "-app", "labels": map[string]any{
			"trivy-operator.resource.kind": "ReplicaSet",
			"trivy-operator.resource.name": rs,
		}},
		"report": map[string]any{"summary": map[string]any{"criticalCount": crit, "highCount": high}},
	}}
}

func allGyms(result string) map[string]string {
	m := map[string]string{}
	for _, g := range Gyms {
		m[g.Policy] = result
	}
	return m
}

func newBoard(objs ...runtime.Object) *Board {
	var pods, dyn []runtime.Object
	for _, o := range objs {
		if _, ok := o.(*corev1.Pod); ok {
			pods = append(pods, o)
		} else {
			dyn = append(dyn, o)
		}
	}
	scheme := runtime.NewScheme()
	gvrs := map[schema.GroupVersionResource]string{
		policyReportGVR: "PolicyReportList",
		vulnReportGVR:   "VulnerabilityReportList",
	}
	return &Board{
		Kube:       fake.NewSimpleClientset(pods...),
		Dyn:        dynfake.NewSimpleDynamicClientWithCustomListKinds(scheme, gvrs, dyn...),
		Namespaces: []string{"pallet-town"},
	}
}

func TestShinyChampion(t *testing.T) {
	b := newBoard(
		pod("pallet-town", "porygon-abc", "league-board", "137", "league-board-1"),
		policyReport("pallet-town", "porygon-abc", allGyms("pass")),
		vulnReport("pallet-town", "league-board-1", 0, 0),
	)
	league, err := b.Collect(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	w := league.Workloads[0]
	if w.Earned != 8 || !w.Shiny || *w.HP != 100 || w.Pokemon != 137 {
		t.Fatalf("want a shiny Porygon with 8 badges and full HP, got %+v", w)
	}
}

func TestTeamRocket(t *testing.T) {
	results := allGyms("pass")
	results["brock-hardened-pods"] = "fail"
	results["blaine-no-host-access"] = "fail"
	b := newBoard(
		pod("pallet-town", "meowth-xyz", "meowth", "52", "meowth-1"),
		policyReport("pallet-town", "meowth-xyz", results),
		vulnReport("pallet-town", "meowth-1", 2, 5),
	)
	league, _ := b.Collect(context.Background())
	w := league.Workloads[0]
	if w.Earned != 6 || w.Shiny || *w.HP != 10 {
		t.Fatalf("want 6 badges, not shiny, 10 HP; got earned=%d shiny=%v hp=%d", w.Earned, w.Shiny, *w.HP)
	}
	if w.Badges[0].Status != "failed" || w.Badges[0].Message == "" {
		t.Fatalf("Brock's badge should be failed with a reason, got %+v", w.Badges[0])
	}
}

// Before Kyverno/Trivy have reported, the card still renders with unknowns.
func TestNoReportsYet(t *testing.T) {
	b := newBoard(pod("pallet-town", "dex-1", "dex-api", "not-a-number", "dex-rs"))
	league, _ := b.Collect(context.Background())
	w := league.Workloads[0]
	if w.HP != nil || w.Earned != 0 || w.Badges[4].Status != "unknown" {
		t.Fatalf("want unknown badges and nil HP, got %+v", w)
	}
	if w.Pokemon < 1 || w.Pokemon > 151 {
		t.Fatalf("fallback Pokémon out of range: %d", w.Pokemon)
	}
}

func TestOneCardPerApp(t *testing.T) {
	b := newBoard(
		pod("pallet-town", "dex-1", "dex-api", "137", "rs"),
		pod("pallet-town", "dex-2", "dex-api", "137", "rs"),
	)
	league, _ := b.Collect(context.Background())
	if len(league.Workloads) != 1 {
		t.Fatalf("want 1 card for 2 replicas, got %d", len(league.Workloads))
	}
}

func TestHP(t *testing.T) {
	cases := []struct {
		v    Vulns
		want int
	}{
		{Vulns{}, 100},
		{Vulns{High: 1}, 92},
		{Vulns{Critical: 4}, 0},
		{Vulns{Critical: 9, High: 9}, 0},
	}
	for _, c := range cases {
		if got := HP(c.v); got != c.want {
			t.Errorf("HP(%+v) = %d, want %d", c.v, got, c.want)
		}
	}
}

// Prod reports come from the "-enforce" copies of each gym.
func TestEnforceCopiesCountForTheSameBadge(t *testing.T) {
	results := map[string]string{}
	for _, g := range Gyms {
		results[g.Policy+"-enforce"] = "pass"
	}
	b := newBoard(
		pod("pallet-town", "board-1", "league-board", "137", "rs"),
		policyReport("pallet-town", "board-1", results),
	)
	league, _ := b.Collect(context.Background())
	if got := league.Workloads[0].Earned; got != 8 {
		t.Fatalf("want 8 badges from -enforce policies, got %d", got)
	}
}
