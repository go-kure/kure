package fluxcd_test

import (
	"fmt"
	"strings"
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/go-kure/kure/pkg/stack"
	fluxstack "github.com/go-kure/kure/pkg/stack/fluxcd"
	"github.com/go-kure/kure/pkg/stack/layout"
)

// An application may emit a typed object that holds an Unstructured in a
// runtime.RawExtension. The copy of the typed object follows Object whatever
// Raw holds, and apimachinery's copy of an Unstructured panics on a plain int.
// An entry point normalizes that held object as it does the item of a List
// (go-kure/kure#1252).

// sourceNamedRevision is a ControllerRevision with the identity of the Source
// a bundle's SourceRef derives, so the integration compares the two, which
// copies the typed object. Its RawExtension holds u beside Raw.
func sourceNamedRevision(u *unstructured.Unstructured) client.Object {
	return &appsv1.ControllerRevision{
		TypeMeta:   metav1.TypeMeta{APIVersion: "source.toolkit.fluxcd.io/v1", Kind: "GitRepository"},
		ObjectMeta: metav1.ObjectMeta{Name: "web-git", Namespace: "flux-system"},
		Data:       runtime.RawExtension{Raw: []byte("{}"), Object: u},
	}
}

// sourceCluster is a cluster whose application emits obj and whose web node
// derives the Source web-git.
func sourceCluster(obj client.Object) (*stack.Cluster, layout.LayoutRules) {
	webRef := &stack.SourceRef{Kind: "GitRepository", Name: "web-git", Namespace: "flux-system", URL: "https://example.com/web.git", Branch: "main"}
	sources := stack.NewApplication("sources", "default", &fakeAppConfig{objs: []*client.Object{&obj}})
	web := &stack.Node{Name: "web", Bundle: &stack.Bundle{Name: "web", SourceRef: webRef, Applications: []*stack.Application{cmApp("web-app")}}}
	c := &stack.Cluster{Name: "demo", Node: &stack.Node{Name: "platform", Bundle: srBundle("platform", sources), Children: []*stack.Node{web}}}
	return c, propertyGroupings["nodeOnly"]
}

// answerOrPanic returns what run answered, or the panic it made.
func answerOrPanic(run func() error) (err error) {
	defer func() {
		if p := recover(); p != nil {
			err = fmt.Errorf("panic: %v", p)
		}
	}()
	return run()
}

// integrateHolding runs the integration on sourceCluster(obj).
func integrateHolding(obj client.Object, placement layout.FluxPlacement) error {
	return answerOrPanic(func() error {
		c, rules := sourceCluster(obj)
		rules.FluxPlacement = placement
		_, err := fluxstack.NewLayoutIntegrator(fluxstack.NewResourceGenerator()).CreateLayoutWithResources(c, rules)
		return err
	})
}

func TestCallerObject_HeldUnstructuredInRawExtensionDoesNotPanic(t *testing.T) {
	t.Parallel()
	for _, placement := range []layout.FluxPlacement{layout.FluxIntegratedPerLayout, layout.FluxIntegratedPerBundle} {
		for name, n := range map[string]any{"plain int": 1, "int64": int64(1)} {
			t.Run(name+"/"+string(placement), func(t *testing.T) {
				t.Parallel()
				u := unstructuredObj("v1", "ConfigMap", "default", "cm")
				u.Object["n"] = n
				err := integrateHolding(sourceNamedRevision(u), placement)
				// Both are the same Source identity held twice: the answer is the
				// duplicate refusal, never a panic.
				if err == nil || strings.HasPrefix(err.Error(), "panic") || !strings.Contains(err.Error(), "already has GitRepository") {
					t.Errorf("got %v, want the refusal of the Source held twice", err)
				}
			})
		}
	}
}
