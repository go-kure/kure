package fluxcd

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	fluxv1 "github.com/controlplaneio-fluxcd/flux-operator/api/v1"
	sourcev1 "github.com/fluxcd/source-controller/api/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/go-kure/kure/pkg/stack"
	"github.com/go-kure/kure/pkg/stack/layout"
)

// TestFluxDuration: the form a duration is written in is the one a
// metav1.Duration marshals to, and Flux takes it unless it is negative, or
// positive and under a millisecond. The authored form does not matter: "90s"
// is written "1m30s".
func TestFluxDuration(t *testing.T) {
	for _, tc := range []struct {
		d       time.Duration
		written string
		ok      bool
	}{
		{0, "0s", true},
		{time.Millisecond, "1ms", true},
		{1500 * time.Millisecond, "1.5s", true},
		{90 * time.Second, "1m30s", true},
		{90 * time.Minute, "1h30m0s", true},
		{time.Second + time.Microsecond, "1.000001s", true},
		{-time.Second, "-1s", false},
		{-90 * time.Minute, "-1h30m0s", false},
		{time.Microsecond, "1µs", false},
		{999 * time.Microsecond, "999µs", false},
		{500 * time.Nanosecond, "500ns", false},
	} {
		t.Run(tc.written, func(t *testing.T) {
			written, ok := fluxDuration(tc.d)
			if written != tc.written || ok != tc.ok {
				t.Errorf("fluxDuration(%d) = %q, %v, want %q, %v", tc.d, written, ok, tc.written, tc.ok)
			}
			marshalled, err := json.Marshal(metav1.Duration{Duration: tc.d})
			if err != nil {
				t.Fatal(err)
			}
			var inManifest string
			if err := json.Unmarshal(marshalled, &inManifest); err != nil {
				t.Fatal(err)
			}
			if inManifest != written {
				t.Errorf("a metav1.Duration of %d is written %q, fluxDuration checked %q", tc.d, inManifest, written)
			}
		})
	}
}

// TestFluxDurationPatternMatchesVendoredCRDs is a differential: the pattern
// durations are held to is the one the vendored CRDs carry on every duration
// field this package writes, in every version they serve: a Kustomization's
// interval, timeout and retryInterval and a GitRepository's and an
// OCIRepository's interval in the vendored flux2 bundle, and a FluxInstance's
// sync interval in the vendored Flux Operator install bundle. A release that
// changes a pattern fails here when its bundle is updated.
func TestFluxDurationPatternMatchesVendoredCRDs(t *testing.T) {
	gotk, err := NewBootstrapGenerator().generateGotkComponents(&stack.BootstrapConfig{Enabled: true, FluxMode: ModeGotk})
	if err != nil {
		t.Fatalf("generateGotkComponents: %v", err)
	}
	operator, err := FluxOperatorInstallObjects()
	if err != nil {
		t.Fatalf("FluxOperatorInstallObjects: %v", err)
	}

	for _, tc := range []struct {
		crd    string
		bundle string
		objs   []client.Object
		fields [][]string // each a path below spec
	}{
		{"kustomizations.kustomize.toolkit.fluxcd.io", "flux2 " + GotkVersion, gotk,
			[][]string{{"interval"}, {"timeout"}, {"retryInterval"}}},
		{"gitrepositories.source.toolkit.fluxcd.io", "flux2 " + GotkVersion, gotk, [][]string{{"interval"}}},
		{"ocirepositories.source.toolkit.fluxcd.io", "flux2 " + GotkVersion, gotk, [][]string{{"interval"}}},
		{"fluxinstances.fluxcd.controlplane.io", "flux-operator " + FluxOperatorVersion, operator, [][]string{{"sync", "interval"}}},
	} {
		t.Run(tc.crd, func(t *testing.T) {
			var crd *apiextensionsv1.CustomResourceDefinition
			for _, o := range tc.objs {
				if o.GetName() != tc.crd || o.GetObjectKind().GroupVersionKind().Kind != "CustomResourceDefinition" {
					continue
				}
				raw, err := json.Marshal(o)
				if err != nil {
					t.Fatal(err)
				}
				crd = &apiextensionsv1.CustomResourceDefinition{}
				if err := json.Unmarshal(raw, crd); err != nil {
					t.Fatalf("the vendored %s is not a CustomResourceDefinition: %v", tc.crd, err)
				}
			}
			if crd == nil {
				t.Fatalf("no CustomResourceDefinition %q in the vendored bundle (%s)", tc.crd, tc.bundle)
			}

			checked := 0
			for _, version := range crd.Spec.Versions {
				if !version.Served || version.Schema == nil || version.Schema.OpenAPIV3Schema == nil {
					continue
				}
				for _, path := range tc.fields {
					at := version.Schema.OpenAPIV3Schema.Properties["spec"]
					for _, name := range path {
						at = at.Properties[name]
					}
					if at.Pattern != fluxDurationPattern.String() {
						t.Errorf("%s %s spec.%s has pattern %q, fluxDurationPattern is %q",
							tc.crd, version.Name, strings.Join(path, "."), at.Pattern, fluxDurationPattern)
					}
					checked++
				}
			}
			if checked == 0 {
				t.Fatalf("the vendored %s (%s) serves no version with a schema: nothing was compared", tc.crd, tc.bundle)
			}
		})
	}
}

// TestDefaultIntervalOnSourcesAndSync: a generator's DefaultInterval is held
// to Flux's pattern at each of the five places it is written into an object
// that is no Kustomization: the GitRepository and the OCIRepository either
// generator creates, and the FluxInstance sync. Each site is called on its
// own, since on the bootstrap paths another object of the same generator is
// created first. Where the object is not created, nothing is refused.
func TestDefaultIntervalOnSourcesAndSync(t *testing.T) {
	gitRef := &stack.SourceRef{Kind: "GitRepository", Name: "src", Namespace: "flux-system", URL: "https://example.com/repo.git"}
	ociRef := &stack.SourceRef{Kind: "OCIRepository", Name: "src", Namespace: "flux-system", URL: "oci://example.com/repo"}
	gitConfig := &stack.BootstrapConfig{Enabled: true, SourceKind: "GitRepository", SourceURL: "https://example.com/repo.git"}
	ociConfig := &stack.BootstrapConfig{Enabled: true, SourceURL: "oci://example.com/repo"}
	syncConfig := &stack.BootstrapConfig{Enabled: true, FluxVersion: "2.x", Registry: "ghcr.io/fluxcd", SourceURL: "oci://example.com/repo"}
	root := &stack.Node{Name: "prod"}

	resource := func(d time.Duration) *ResourceGenerator {
		g := NewResourceGenerator()
		g.DefaultInterval = d
		return g
	}
	bootstrap := func(d time.Duration) *BootstrapGenerator {
		bg := NewBootstrapGenerator()
		bg.DefaultInterval = d
		return bg
	}

	sites := []struct {
		name      string
		generator string
		// write creates the object with that DefaultInterval and returns the
		// interval it carries.
		write func(d time.Duration) (*time.Duration, error)
	}{
		{"ResourceGenerator GitRepository", "ResourceGenerator", func(d time.Duration) (*time.Duration, error) {
			o, err := resource(d).createSource(gitRef, "b")
			if err != nil {
				return nil, err
			}
			return &o.(*sourcev1.GitRepository).Spec.Interval.Duration, nil
		}},
		{"ResourceGenerator OCIRepository", "ResourceGenerator", func(d time.Duration) (*time.Duration, error) {
			o, err := resource(d).createSource(ociRef, "b")
			if err != nil {
				return nil, err
			}
			return &o.(*sourcev1.OCIRepository).Spec.Interval.Duration, nil
		}},
		{"BootstrapGenerator GitRepository", "BootstrapGenerator", func(d time.Duration) (*time.Duration, error) {
			o, err := bootstrap(d).generateGitSource(gitConfig, root)
			if err != nil {
				return nil, err
			}
			return &o.(*sourcev1.GitRepository).Spec.Interval.Duration, nil
		}},
		{"BootstrapGenerator OCIRepository", "BootstrapGenerator", func(d time.Duration) (*time.Duration, error) {
			o, err := bootstrap(d).generateOCISource(ociConfig, root)
			if err != nil {
				return nil, err
			}
			return &o.(*sourcev1.OCIRepository).Spec.Interval.Duration, nil
		}},
		{"FluxInstance sync", "BootstrapGenerator", func(d time.Duration) (*time.Duration, error) {
			o, err := bootstrap(d).generateFluxInstance(syncConfig, "prod")
			if err != nil {
				return nil, err
			}
			return &o.(*fluxv1.FluxInstance).Spec.Sync.Interval.Duration, nil
		}},
	}
	for _, site := range sites {
		for _, tc := range []struct {
			d       time.Duration
			written string
		}{
			{-time.Minute, "-1m0s"},
			{500 * time.Microsecond, "500µs"},
		} {
			t.Run(site.name+"/"+tc.written, func(t *testing.T) {
				_, err := site.write(tc.d)
				if err == nil {
					t.Fatal("accepted")
				}
				for _, want := range []string{site.generator, "DefaultInterval",
					`interval "` + tc.written + `" (the generator's DefaultInterval) is written as "` + tc.written + `", which the Flux API does not take`} {
					if !strings.Contains(err.Error(), want) {
						t.Errorf("the refusal does not contain %q:\n%v", want, err)
					}
				}
			})
		}
		t.Run(site.name+"/taken", func(t *testing.T) {
			got, err := site.write(90 * time.Minute)
			if err != nil {
				t.Fatalf("refused: %v", err)
			}
			if *got != 90*time.Minute {
				t.Errorf("interval = %v, want the DefaultInterval 1h30m", *got)
			}
		})
	}

	t.Run("no object, no refusal", func(t *testing.T) {
		if o, err := resource(-time.Minute).createSource(&stack.SourceRef{Kind: "GitRepository", Name: "src"}, "b"); err != nil || o != nil {
			t.Errorf("createSource without a URL = %v, %v, want no source and no error", o, err)
		}
		noSync := *syncConfig
		noSync.SourceURL = ""
		o, err := bootstrap(-time.Minute).generateFluxInstance(&noSync, "prod")
		if err != nil {
			t.Fatalf("a FluxInstance without a sync is refused for an interval nothing takes: %v", err)
		}
		if o.(*fluxv1.FluxInstance).Spec.Sync != nil {
			t.Error("a FluxInstance without a SourceURL carries a sync")
		}
	})
}

// TestLayoutDurationRefusalNamesLayoutAndBundle pins the wording of a refused
// duration on a per-layout Kustomization: the layout by its directory, the
// field, the value as authored and as written, and, for an inherited value,
// the bundle it comes from by its path.
//
// The inherited case is asserted on layoutDuration itself. An integration
// generates the Kustomization of the bundle before those of the layouts
// below it, so a bundle's duration Flux does not take is refused there first,
// naming the bundle (TestFluxDurations_InheritedByALayout).
func TestLayoutDurationRefusalNamesLayoutAndBundle(t *testing.T) {
	l := &layout.ManifestLayout{Name: "db", Namespace: "prod/shop"}
	holder := &stack.Bundle{Name: "shop", ParentPath: "platform"}

	for _, tc := range []struct {
		name, own, inherited string
		wants                []string
		not                  string
	}{
		{
			name: "own, negative", own: "-1s", inherited: "5m",
			wants: []string{"ManifestLayout", "prod/shop/db", `timeout "-1s" is written as "-1s", which the Flux API does not take`, "digits with a unit of ms, s, m or h, so no negative duration and no positive one under a millisecond"},
			not:   "inherited",
		},
		{
			name: "own, under a millisecond", own: "1us",
			wants: []string{"prod/shop/db", `timeout "1us" is written as "1µs", which the Flux API does not take`},
			not:   "inherited",
		},
		{
			name: "inherited, negative", inherited: "-1s",
			wants: []string{"ManifestLayout", "prod/shop/db", `timeout "-1s" (inherited from bundle "platform/shop") is written as "-1s", which the Flux API does not take`},
		},
		{
			name: "inherited, under a millisecond", inherited: "1us",
			wants: []string{"prod/shop/db", `timeout "1us" (inherited from bundle "platform/shop") is written as "1µs", which the Flux API does not take`},
		},
		{
			// time.ParseDuration takes no day unit, so the value never
			// reaches the Flux pattern.
			name: "inherited, a day unit", inherited: "1d",
			wants: []string{"prod/shop/db", `timeout "1d" (inherited from bundle "platform/shop") is not a valid duration`},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := layoutDuration(l, holder, "timeout", tc.own, tc.inherited)
			if err == nil {
				t.Fatal("accepted")
			}
			for _, want := range tc.wants {
				if !strings.Contains(err.Error(), want) {
					t.Errorf("the refusal does not contain %q:\n%v", want, err)
				}
			}
			if tc.not != "" && strings.Contains(err.Error(), tc.not) {
				t.Errorf("the refusal says %q of a value the layout sets itself:\n%v", tc.not, err)
			}
		})
	}

	for _, tc := range []struct{ name, own, inherited string }{
		{"own", "1h30m", "-1s"},
		{"inherited", "", "1h30m"},
	} {
		t.Run("taken, "+tc.name, func(t *testing.T) {
			d, err := layoutDuration(l, holder, "timeout", tc.own, tc.inherited)
			if err != nil {
				t.Fatalf("refused: %v", err)
			}
			if d == nil || d.Duration != 90*time.Minute {
				t.Errorf("timeout = %v, want 1h30m", d)
			}
		})
	}
}
