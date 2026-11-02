package fluxcd_test

import (
	"errors"
	"strings"
	"testing"

	kustv1 "github.com/fluxcd/kustomize-controller/api/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/go-kure/kure/pkg/stack"
	fluxstack "github.com/go-kure/kure/pkg/stack/fluxcd"
	"github.com/go-kure/kure/pkg/stack/layout"
)

// Tests for go-kure/kure#1173: the API server decodes a Flux Kustomization
// case-sensitively and drops a field it does not know, so a key spelled
// `KubeConfig:` or `Wait:` sets nothing for Flux. kure reads a Kustomization it
// meets as unstructured content the same way, whatever the spelling.

// handWrittenAs integrates the tree of integrateWithHandWritten with hand-written
// held as unstructured content, after rename has changed the spelling of the
// keys of its spec.
func handWrittenAs(t *testing.T, placement layout.FluxPlacement, edit func(k *kustv1.Kustomization), rename map[string]string) error {
	t.Helper()
	k := handWritten("hand-written")
	edit(k)
	content, err := runtime.DefaultUnstructuredConverter.ToUnstructured(k)
	if err != nil {
		t.Fatalf("ToUnstructured: %v", err)
	}
	spec := content["spec"].(map[string]any)
	for from, to := range rename {
		v, ok := spec[from]
		if !ok {
			t.Fatalf("spec has no %q", from)
		}
		delete(spec, from)
		spec[to] = v
	}
	u := &unstructured.Unstructured{Object: content}
	u.SetAPIVersion(kustv1.GroupVersion.String())
	u.SetKind("Kustomization")
	var o client.Object = u
	app := stack.NewApplication("platform-app", "default", &fakeAppConfig{objs: []*client.Object{&o}})
	_, err = integrateNoWait(noWaitCluster(srBundle("platform", app), true), nodeOnly, placement)
	return err
}

// A key spelled `Wait:` is no wait to Flux, so the Kustomization it is on adds
// no edge: the tree whose directory it builds is accepted. The same
// Kustomization spelled `wait:` is the cycle through web's apply.
func TestCaseSensitiveRead_MisCasedWaitAddsNoEdge(t *testing.T) {
	for _, placement := range integratedPlacements {
		t.Run(string(placement), func(t *testing.T) {
			dir := webDirectory(t, placement)
			edit := func(k *kustv1.Kustomization) { k.Spec.Wait, k.Spec.Path = true, dir }

			err := handWrittenAs(t, placement, edit, nil)
			if !errors.Is(err, fluxstack.ErrUmbrellaDependency) || !strings.Contains(err.Error(), "ready flux-system/hand-written -> ready flux-system/web -> apply flux-system/web") {
				t.Fatalf("control: got %v, want ErrUmbrellaDependency naming the chain through web", err)
			}
			if err := handWrittenAs(t, placement, edit, map[string]string{"wait": "Wait"}); err != nil {
				t.Fatalf("got %v, want the tree accepted: Flux reads no wait from `Wait:`", err)
			}
		})
	}
}

// A key spelled `KubeConfig:` is no kubeConfig to Flux: the Kustomization
// applies to this cluster and waits, and a cycle through it is refused. The
// same Kustomization spelled `kubeConfig:` applies elsewhere and is accepted.
func TestCaseSensitiveRead_MisCasedKubeConfigStaysOnThisCluster(t *testing.T) {
	for _, placement := range integratedPlacements {
		t.Run(string(placement), func(t *testing.T) {
			dir := webDirectory(t, placement)
			edit := func(k *kustv1.Kustomization) {
				k.Spec.Wait, k.Spec.Path = true, dir
				k.Spec.KubeConfig = remoteCluster()
			}

			if err := handWrittenAs(t, placement, edit, nil); err != nil {
				t.Fatalf("control: got %v, want the tree accepted: `kubeConfig:` applies to another cluster", err)
			}
			err := handWrittenAs(t, placement, edit, map[string]string{"kubeConfig": "KubeConfig"})
			if !errors.Is(err, fluxstack.ErrUmbrellaDependency) || !strings.Contains(err.Error(), "ready flux-system/hand-written -> ready flux-system/web -> apply flux-system/web") {
				t.Fatalf("got %v, want ErrUmbrellaDependency naming the chain through web: Flux reads no kubeConfig from `KubeConfig:`", err)
			}
		})
	}
}
