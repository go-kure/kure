package fluxcd_test

import (
	"reflect"
	"testing"

	fluxv1 "github.com/controlplaneio-fluxcd/flux-operator/api/v1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"

	"github.com/go-kure/kure/pkg/stack"
	fluxstack "github.com/go-kure/kure/pkg/stack/fluxcd"
	"github.com/go-kure/kure/pkg/stack/layout"
)

// go-kure/kure#1078: BootstrapConfig.ImagePullSecret is the pull secret for
// the Flux controller images, and both modes carry it. gotk mode always did,
// through the install options; flux-operator mode dropped it, so a cluster
// pulling the controllers from a private registry got a FluxInstance without
// it and the controllers could not pull.

// imagePullSecretCases are the two inputs: unset must keep today's output in
// both modes, set must reach the controllers in both.
var imagePullSecretCases = []struct {
	name   string
	secret string
}{
	{name: "unset", secret: ""},
	{name: "set", secret: "flux-pull"},
}

// TestFluxOperatorImagePullSecret covers both flux-operator entry points: the
// secret becomes the FluxInstance's spec.distribution.imagePullSecret, compared
// as the whole distribution so nothing else moves with it.
func TestFluxOperatorImagePullSecret(t *testing.T) {
	for _, tc := range imagePullSecretCases {
		t.Run(tc.name, func(t *testing.T) {
			config := &stack.BootstrapConfig{
				Enabled:         true,
				FluxMode:        "flux-operator",
				FluxVersion:     "2.x",
				Registry:        "registry.example.com/fluxcd",
				ImagePullSecret: tc.secret,
			}
			want := fluxv1.Distribution{
				Version:         "2.x",
				Registry:        "registry.example.com/fluxcd",
				ImagePullSecret: tc.secret,
			}

			bg := fluxstack.NewBootstrapGenerator()
			resources, err := bg.GenerateBootstrap(config, &stack.Node{Name: "prod"}, layout.LayoutRules{})
			if err != nil {
				t.Fatalf("GenerateBootstrap() error = %v", err)
			}
			if got := findFluxInstance(t, resources).Spec.Distribution; got != want {
				t.Errorf("GenerateBootstrap: spec.distribution = %+v, want %+v", got, want)
			}

			fi, err := bg.GenerateFluxInstance(config, &stack.Node{Name: "prod"}, layout.LayoutRules{})
			if err != nil {
				t.Fatalf("GenerateFluxInstance() error = %v", err)
			}
			if got := fi.Spec.Distribution; got != want {
				t.Errorf("GenerateFluxInstance: spec.distribution = %+v, want %+v", got, want)
			}
		})
	}
}

// TestGotkImagePullSecret is the gotk side of the same field: every controller
// Deployment carries the secret in its pod template, and none carries one when
// it is unset. The default FluxVersion builds from the vendored bundle, so this
// runs offline.
func TestGotkImagePullSecret(t *testing.T) {
	for _, tc := range imagePullSecretCases {
		t.Run(tc.name, func(t *testing.T) {
			resources, err := fluxstack.NewBootstrapGenerator().GenerateBootstrap(&stack.BootstrapConfig{
				Enabled:         true,
				FluxMode:        "gotk",
				ImagePullSecret: tc.secret,
			}, &stack.Node{Name: "prod"}, layout.LayoutRules{})
			if err != nil {
				t.Fatalf("GenerateBootstrap() error = %v", err)
			}

			var want []corev1.LocalObjectReference
			if tc.secret != "" {
				want = []corev1.LocalObjectReference{{Name: tc.secret}}
			}
			deployments := 0
			for _, obj := range resources {
				dep, ok := obj.(*appsv1.Deployment)
				if !ok {
					continue
				}
				deployments++
				if got := dep.Spec.Template.Spec.ImagePullSecrets; !reflect.DeepEqual(got, want) {
					t.Errorf("Deployment %s: imagePullSecrets = %v, want %v", dep.Name, got, want)
				}
			}
			if deployments == 0 {
				t.Fatalf("gotk bootstrap returned no typed Deployment among %d resources", len(resources))
			}
		})
	}
}
