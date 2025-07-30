package application

import (
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func TestSplitByLabels(t *testing.T) {
	r1 := &unstructured.Unstructured{}
	r1.SetAPIVersion("v1")
	r1.SetKind("ConfigMap")
	r1.SetName("cm1")
	r1.SetLabels(map[string]string{"env": "prod"})

	r2 := &unstructured.Unstructured{}
	r2.SetAPIVersion("v1")
	r2.SetKind("ConfigMap")
	r2.SetName("cm2")
	r2.SetLabels(map[string]string{"env": "dev"})

	r3 := &unstructured.Unstructured{}
	r3.SetAPIVersion("v1")
	r3.SetKind("ConfigMap")
	r3.SetName("cm3")
	r3.SetLabels(map[string]string{"env": "prod", "tier": "front"})

	as, err := New("demo", []client.Object{r1, r2, r3}, map[string]string{"app": "demo"})
	if err != nil {
		t.Fatalf("new: %v", err)
	}

	rules := []LabelRule{
		{Name: "prod", Match: map[string]string{"env": "prod"}},
		{Name: "dev", Match: map[string]string{"env": "dev"}},
	}

	splits, err := as.SplitByLabels(rules)
	if err != nil {
		t.Fatalf("split: %v", err)
	}

	if len(splits) != 2 {
		t.Fatalf("expected 2 splits got %d", len(splits))
	}

	if splits[0].Name != "prod" || len(splits[0].Resources) != 2 {
		t.Fatalf("unexpected prod split")
	}
	if splits[1].Name != "dev" || len(splits[1].Resources) != 1 {
		t.Fatalf("unexpected dev split")
	}
}

func TestValidate(t *testing.T) {
	as := &ApplicationGroup{Name: "", Resources: []client.Object{}}
	if err := as.Validate(); err == nil {
		t.Fatalf("expected validation error")
	}
}
