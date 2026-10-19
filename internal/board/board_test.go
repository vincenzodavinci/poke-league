package board

import (
	"context"
	"errors"
	"strings"
	"testing"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	dynfake "k8s.io/client-go/dynamic/fake"
	"k8s.io/client-go/kubernetes/fake"
	clienttesting "k8s.io/client-go/testing"
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
		"report": map[string]any{"summary": map[string]any{
			"criticalCount": crit, "highCount": high, "mediumCount": int64(0), "lowCount": int64(0)}},
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
	if len(league.Workloads) != 1 || league.Workloads[0].Replicas != 2 {
		t.Fatalf("want 1 card for 2 replicas, got %+v", league.Workloads)
	}
}

// A healthy replica must never hide a failing one, whichever the API lists first.
func TestCardShowsTheWorstReplica(t *testing.T) {
	bad := allGyms("pass")
	bad["brock-hardened-pods"] = "fail"
	bad["erika-trusted-images"] = "fail"
	for _, order := range [][2]string{{"dex-good", "dex-bad"}, {"dex-bad", "dex-good"}} {
		b := newBoard(
			pod("pallet-town", order[0], "dex-api", "137", "rs-1"),
			pod("pallet-town", order[1], "dex-api", "137", "rs-2"),
			policyReport("pallet-town", "dex-good", allGyms("pass")),
			policyReport("pallet-town", "dex-bad", bad),
			vulnReport("pallet-town", "rs-1", 0, 0),
			vulnReport("pallet-town", "rs-2", 0, 0),
		)
		league, _ := b.Collect(context.Background())
		w := league.Workloads[0]
		if w.Earned != 6 || w.Shiny || w.Pod != "dex-bad" || w.Replicas != 2 {
			t.Fatalf("order %v: want 6/8 from dex-bad across 2 replicas, got earned=%d shiny=%v pod=%s replicas=%d",
				order, w.Earned, w.Shiny, w.Pod, w.Replicas)
		}
	}
}

func TestSkippedIsNotEarned(t *testing.T) {
	results := allGyms("pass")
	results["koga-signed-images"] = "skip"
	b := newBoard(
		pod("pallet-town", "dex-1", "dex-api", "137", "rs"),
		policyReport("pallet-town", "dex-1", results),
		vulnReport("pallet-town", "rs", 0, 0),
	)
	league, _ := b.Collect(context.Background())
	w := league.Workloads[0]
	if w.Earned != 7 || w.Shiny || w.Badges[4].Status != Unknown || !strings.HasPrefix(w.Badges[4].Message, "skipped") {
		t.Fatalf("a skipped gym must not award its badge: got earned=%d koga=%+v", w.Earned, w.Badges[4])
	}
}

// With no Trivy at all, an 8/8 workload is not shiny: there's no evidence it's clean.
func TestNoTrivyMeansNotShiny(t *testing.T) {
	b := newBoard(
		pod("pallet-town", "dex-1", "dex-api", "137", "rs"),
		policyReport("pallet-town", "dex-1", allGyms("pass")),
	)
	b.Dyn.(*dynfake.FakeDynamicClient).PrependReactor("list", "vulnerabilityreports",
		func(clienttesting.Action) (bool, runtime.Object, error) {
			return true, nil, apierrors.NewNotFound(schema.GroupResource{Group: vulnReportGVR.Group, Resource: "vulnerabilityreports"}, "")
		})
	league, _ := b.Collect(context.Background())
	w := league.Workloads[0]
	if w.Earned != 8 || w.Shiny || w.Scan != NotInstalled || len(league.Problems) != 0 {
		t.Fatalf("want 8/8, not shiny, scan not-installed, no problems; got %+v problems=%v", w, league.Problems)
	}
}

func TestIncompleteReportIsNotClean(t *testing.T) {
	broken := vulnReport("pallet-town", "rs", 0, 0)
	unstructured.RemoveNestedField(broken.Object, "report", "summary", "criticalCount")
	b := newBoard(
		pod("pallet-town", "dex-1", "dex-api", "137", "rs"),
		policyReport("pallet-town", "dex-1", allGyms("pass")),
		broken,
	)
	league, _ := b.Collect(context.Background())
	w := league.Workloads[0]
	if w.Shiny || w.Scan != ScanError || w.HP != nil {
		t.Fatalf("a report missing its critical count must not read as zero CVEs, got %+v", w)
	}
}

func TestUnscannedWorkload(t *testing.T) {
	b := newBoard(
		pod("pallet-town", "dex-1", "dex-api", "137", "rs"),
		vulnReport("pallet-town", "some-other-rs", 0, 0),
	)
	league, _ := b.Collect(context.Background())
	if w := league.Workloads[0]; w.Scan != NotScanned || w.HP != nil {
		t.Fatalf("want not-scanned, got %+v", w)
	}
}

// RBAC mistakes and API errors are reported, not shown as an empty, quiet board.
func TestReadErrorsAreProblemsNotPasses(t *testing.T) {
	b := newBoard(pod("pallet-town", "dex-1", "dex-api", "137", "rs"))
	deny := func(clienttesting.Action) (bool, runtime.Object, error) {
		return true, nil, apierrors.NewForbidden(schema.GroupResource{}, "", errors.New("RBAC says no"))
	}
	b.Dyn.(*dynfake.FakeDynamicClient).PrependReactor("list", "policyreports", deny)
	b.Dyn.(*dynfake.FakeDynamicClient).PrependReactor("list", "vulnerabilityreports", deny)
	league, err := b.Collect(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	w := league.Workloads[0]
	if len(league.Problems) != 2 || w.Scan != ScanError || w.Earned != 0 || w.Badges[0].Status != Unknown {
		t.Fatalf("want 2 problems, scan error, unknown badges; got problems=%v %+v", league.Problems, w)
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
