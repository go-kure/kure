package fluxcd

import (
	stderrors "errors"
	"strings"
	"testing"
	"time"

	kustv1 "github.com/fluxcd/kustomize-controller/api/v1"
	sourcev1 "github.com/fluxcd/source-controller/api/v1"

	kerrors "github.com/go-kure/kure/pkg/errors"
	"github.com/go-kure/kure/pkg/stack"
	"github.com/go-kure/kure/pkg/stack/layout"
)

// TestConstructorsSeedTheExportedDefaults pins that each constructor's seeded
// value equals the exported identifier, so changing one without the other
// fails here.
//
// What it cannot catch, and no runtime test can: a literal re-inlined in a
// constructor with the same value the identifier holds. These assertions
// compare values, not source-level references, and "flux-system" reintroduced
// at the assignment is indistinguishable from DefaultNamespace at run time.
// Source-level use is a review property here, not a tested one.
func TestConstructorsSeedTheExportedDefaults(t *testing.T) {
	rg := NewResourceGenerator()
	if rg.Mode != DefaultMode {
		t.Errorf("ResourceGenerator.Mode = %v, want DefaultMode (%v)", rg.Mode, DefaultMode)
	}
	if rg.DefaultInterval != DefaultInterval {
		t.Errorf("ResourceGenerator.DefaultInterval = %v, want DefaultInterval (%v)", rg.DefaultInterval, DefaultInterval)
	}
	if rg.DefaultNamespace != DefaultNamespace {
		t.Errorf("ResourceGenerator.DefaultNamespace = %q, want DefaultNamespace (%q)", rg.DefaultNamespace, DefaultNamespace)
	}
	if rg.Prune != nil {
		t.Errorf("ResourceGenerator.Prune = %v, want nil so nothing is injected", *rg.Prune)
	}

	bg := NewBootstrapGenerator()
	if bg.DefaultInterval != DefaultInterval {
		t.Errorf("BootstrapGenerator.DefaultInterval = %v, want DefaultInterval (%v)", bg.DefaultInterval, DefaultInterval)
	}
	if bg.DefaultNamespace != DefaultNamespace {
		t.Errorf("BootstrapGenerator.DefaultNamespace = %q, want DefaultNamespace (%q)", bg.DefaultNamespace, DefaultNamespace)
	}
	if bg.BootstrapName != DefaultBootstrapName {
		t.Errorf("BootstrapGenerator.BootstrapName = %q, want DefaultBootstrapName (%q)", bg.BootstrapName, DefaultBootstrapName)
	}
}

// TestBootstrapNameIsOverrideable pins the field that makes DefaultBootstrapName
// an overrideable default rather than a buried literal. BootstrapConfig carries
// no name input, so before this field the value could not be changed at all,
// while the package documented it as overrideable.
func TestBootstrapNameIsOverrideable(t *testing.T) {
	bg := NewBootstrapGenerator()
	bg.BootstrapName = "gitops"

	config := &stack.BootstrapConfig{
		Enabled:    true,
		FluxMode:   "gotk",
		SourceKind: "GitRepository",
		SourceURL:  "https://github.com/org/fleet.git",
	}
	objs, err := bg.GenerateBootstrap(config, &stack.Node{Name: "prod"})
	if err != nil {
		t.Fatalf("GenerateBootstrap: %v", err)
	}
	var found bool
	for _, o := range objs {
		if k, ok := o.(*kustv1.Kustomization); ok {
			found = true
			if k.Name != "gitops" {
				t.Errorf("bootstrap Kustomization name = %q, want the assigned %q", k.Name, "gitops")
			}
		}
	}
	if !found {
		t.Fatal("no bootstrap Kustomization emitted")
	}

	fi, err := bg.GenerateFluxInstance(config, &stack.Node{Name: "prod"})
	if err != nil {
		t.Fatalf("GenerateFluxInstance: %v", err)
	}
	if fi.Name != "gitops" {
		t.Errorf("FluxInstance name = %q, want the assigned %q", fi.Name, "gitops")
	}
}

// TestBootstrapNameEmptyFallsBackToTheDefault pins that adding the
// BootstrapName field did not make a nameless object reachable.
//
// A caller that builds the generator as a struct literal rather than through
// NewBootstrapGenerator leaves BootstrapName at "". Before the field existed
// both names were a literal and could not be absent; without the fallback the
// same caller would now emit metadata.name: "" on the Kustomization and the
// FluxInstance, which the API server rejects.
func TestBootstrapNameEmptyFallsBackToTheDefault(t *testing.T) {
	bg := &BootstrapGenerator{
		DefaultNamespace: DefaultNamespace,
		DefaultInterval:  DefaultInterval,
	}

	config := &stack.BootstrapConfig{
		Enabled:    true,
		FluxMode:   ModeGotk,
		SourceKind: "GitRepository",
		SourceURL:  "https://github.com/org/fleet.git",
	}
	objs, err := bg.GenerateBootstrap(config, &stack.Node{Name: "prod"})
	if err != nil {
		t.Fatalf("GenerateBootstrap: %v", err)
	}
	var found bool
	for _, o := range objs {
		if k, ok := o.(*kustv1.Kustomization); ok {
			found = true
			if k.Name != DefaultBootstrapName {
				t.Errorf("bootstrap Kustomization name = %q, want DefaultBootstrapName (%q)",
					k.Name, DefaultBootstrapName)
			}
		}
	}
	if !found {
		t.Fatal("no bootstrap Kustomization emitted")
	}

	fi, err := bg.GenerateFluxInstance(config, &stack.Node{Name: "prod"})
	if err != nil {
		t.Fatalf("GenerateFluxInstance: %v", err)
	}
	if fi.Name != DefaultBootstrapName {
		t.Errorf("FluxInstance name = %q, want DefaultBootstrapName (%q)", fi.Name, DefaultBootstrapName)
	}
}

// TestDefaultFluxModeDrivesTheDispatch pins that DefaultFluxMode is the mode
// GenerateBootstrap actually dispatches on, not merely the value assigned when
// FluxMode is empty.
//
// The switch, the validation error's list of accepted values and
// SupportedBootstrapModes all previously restated the literal. Changing the
// constant alone would then have made an empty FluxMode fall through to
// default: and error, so the exported default was not the authority it claimed
// to be. This asserts the two agree by construction rather than by coincidence.
func TestDefaultFluxModeDrivesTheDispatch(t *testing.T) {
	bg := NewBootstrapGenerator()

	// An empty FluxMode must reach the same branch DefaultFluxMode names, and
	// that branch must not be the error branch.
	empty, err := bg.GenerateBootstrap(&stack.BootstrapConfig{Enabled: true}, &stack.Node{Name: "prod"})
	if err != nil {
		t.Fatalf("an empty FluxMode must resolve to DefaultFluxMode (%q): %v", DefaultFluxMode, err)
	}
	named, err := bg.GenerateBootstrap(
		&stack.BootstrapConfig{Enabled: true, FluxMode: DefaultFluxMode}, &stack.Node{Name: "prod"})
	if err != nil {
		t.Fatalf("GenerateBootstrap with FluxMode=DefaultFluxMode: %v", err)
	}
	if len(empty) != len(named) {
		t.Errorf("empty FluxMode produced %d resources, DefaultFluxMode produced %d — different branches",
			len(empty), len(named))
	}

	// Both supported modes are accepted, and the reported set is the set.
	modes := bg.SupportedBootstrapModes()
	if len(modes) != 2 || modes[0] != DefaultFluxMode || modes[1] != ModeGotk {
		t.Errorf("SupportedBootstrapModes() = %v, want [%q %q]", modes, DefaultFluxMode, ModeGotk)
	}
	for _, mode := range modes {
		if _, err := bg.GenerateBootstrap(
			&stack.BootstrapConfig{Enabled: true, FluxMode: mode}, &stack.Node{Name: "prod"}); err != nil {
			t.Errorf("SupportedBootstrapModes reports %q but GenerateBootstrap rejects it: %v", mode, err)
		}
	}

	// An unsupported mode still errors, and the error reports the supported set
	// rather than a second hand-written copy of it.
	_, err = bg.GenerateBootstrap(
		&stack.BootstrapConfig{Enabled: true, FluxMode: "helm"}, &stack.Node{Name: "prod"})
	if err == nil {
		t.Fatal("an unrecognised FluxMode must be an error")
	}
	var verr *kerrors.ValidationError
	if !stderrors.As(err, &verr) {
		t.Fatalf("error is %T, want *errors.ValidationError", err)
	}
	if len(verr.ValidValues) != len(modes) {
		t.Fatalf("ValidValues = %v, want SupportedBootstrapModes() (%v)", verr.ValidValues, modes)
	}
	for i, mode := range modes {
		if verr.ValidValues[i] != mode {
			t.Errorf("ValidValues[%d] = %q, want %q", i, verr.ValidValues[i], mode)
		}
	}
}

// TestDefaultValues states each exported default's value here, so a silent
// change to one shows up as a test diff rather than only as changed YAML.
func TestDefaultValues(t *testing.T) {
	if DefaultInterval != 60*time.Minute {
		t.Errorf("DefaultInterval = %v, want 60m", DefaultInterval)
	}
	if DefaultMode != layout.KustomizationExplicit {
		t.Errorf("DefaultMode = %v, want layout.KustomizationExplicit", DefaultMode)
	}
	want := map[string]string{
		"DefaultNamespace":         "flux-system",
		"DefaultSourceName":        "flux-system",
		"DefaultBootstrapName":     "flux-system",
		"DefaultFluxDirName":       "flux-system",
		"DefaultBootstrapPathRoot": "manifests",
		"DefaultFluxMode":          "flux-operator",
		"ModeGotk":                 "gotk",
		"DefaultSourceKind":        "OCIRepository",
		"DefaultSourceRef":         "latest",
		"DefaultSyncPath":          "./",
	}
	got := map[string]string{
		"DefaultNamespace":         DefaultNamespace,
		"DefaultSourceName":        DefaultSourceName,
		"DefaultBootstrapName":     DefaultBootstrapName,
		"DefaultFluxDirName":       DefaultFluxDirName,
		"DefaultBootstrapPathRoot": DefaultBootstrapPathRoot,
		"DefaultFluxMode":          DefaultFluxMode,
		"ModeGotk":                 ModeGotk,
		"DefaultSourceKind":        DefaultSourceKind,
		"DefaultSourceRef":         DefaultSourceRef,
		"DefaultSyncPath":          DefaultSyncPath,
	}
	if len(got) != len(want) {
		t.Errorf("%d identifiers asserted but %d listed — a new one needs a row in both maps",
			len(got), len(want))
	}
	for name, w := range want {
		if got[name] != w {
			t.Errorf("%s = %q, want %q", name, got[name], w)
		}
	}
}

func TestPruneValue(t *testing.T) {
	yes, no := true, false
	for name, tc := range map[string]struct {
		in   *bool
		want bool
	}{
		"unset means no garbage collection": {nil, false},
		"explicit false":                    {&no, false},
		"explicit true":                     {&yes, true},
	} {
		t.Run(name, func(t *testing.T) {
			if got := pruneValue(tc.in); got != tc.want {
				t.Errorf("pruneValue = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestWaitValue(t *testing.T) {
	yes, no := true, false
	for name, tc := range map[string]struct {
		in   *bool
		want bool
	}{
		"unset":          {nil, false},
		"explicit false": {&no, false},
		"explicit true":  {&yes, true},
	} {
		t.Run(name, func(t *testing.T) {
			if got := waitValue(tc.in); got != tc.want {
				t.Errorf("waitValue = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestSourceName(t *testing.T) {
	if got := sourceName(nil); got != DefaultSourceName {
		t.Errorf("sourceName(nil) = %q, want %q", got, DefaultSourceName)
	}
	if got := sourceName(&stack.Node{}); got != DefaultSourceName {
		t.Errorf("sourceName(unnamed node) = %q, want %q", got, DefaultSourceName)
	}
	if got := sourceName(&stack.Node{Name: "prod"}); got != "prod" {
		t.Errorf("sourceName(named node) = %q, want %q", got, "prod")
	}
}

// TestSourceGeneratorsRejectAnAbsentURL is the placeholder's replacement. The
// OCI path used to substitute oci://registry.example.com/flux-system, which
// reached emitted YAML as if a caller had asked for it.
func TestSourceGeneratorsRejectAnAbsentURL(t *testing.T) {
	bg := NewBootstrapGenerator()
	for name, kind := range map[string]string{
		"OCIRepository": "OCIRepository",
		"GitRepository": "GitRepository",
		"unset kind":    "",
	} {
		t.Run(name, func(t *testing.T) {
			obj, err := bg.generateSource(&stack.BootstrapConfig{Enabled: true, SourceKind: kind}, nil)
			if err == nil {
				t.Fatalf("expected an error for an absent sourceURL, got object %v", obj)
			}
			if obj != nil {
				t.Errorf("expected no object alongside the error, got %v", obj)
			}
			if !strings.Contains(err.Error(), "sourceURL") {
				t.Errorf("error should name the missing field, got %q", err)
			}
			if strings.Contains(err.Error(), "registry.example.com") {
				t.Errorf("error must not mention the removed placeholder, got %q", err)
			}
		})
	}
}

// TestOCISourceUsesDefaultSourceRef keeps the one string fallback the OCI path
// still applies visible: an absent SourceRef becomes DefaultSourceRef, not a
// guess made inline.
func TestOCISourceUsesDefaultSourceRef(t *testing.T) {
	bg := NewBootstrapGenerator()
	obj, err := bg.generateOCISource(&stack.BootstrapConfig{SourceURL: "oci://example.test/repo"}, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	or, ok := obj.(*sourcev1.OCIRepository)
	if !ok {
		t.Fatalf("unexpected type %T", obj)
	}
	if or.Name != DefaultSourceName {
		t.Errorf("source name = %q, want %q", or.Name, DefaultSourceName)
	}
	if or.Spec.URL != "oci://example.test/repo" {
		t.Errorf("source URL = %q, want the caller's URL", or.Spec.URL)
	}
	if or.Spec.Reference == nil || or.Spec.Reference.Tag != DefaultSourceRef {
		t.Errorf("reference = %v, want tag %q", or.Spec.Reference, DefaultSourceRef)
	}
	if or.Spec.Interval.Duration != DefaultInterval {
		t.Errorf("interval = %v, want DefaultInterval (%v)", or.Spec.Interval.Duration, DefaultInterval)
	}
}

// TestOCISourceKeepsAnExplicitRef is the other half: DefaultSourceRef applies
// only when the caller named none.
func TestOCISourceKeepsAnExplicitRef(t *testing.T) {
	bg := NewBootstrapGenerator()
	obj, err := bg.generateOCISource(&stack.BootstrapConfig{
		SourceURL: "oci://example.test/repo",
		SourceRef: "v1.2.3",
	}, &stack.Node{Name: "prod"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	or := obj.(*sourcev1.OCIRepository)
	if or.Spec.Reference == nil || or.Spec.Reference.Tag != "v1.2.3" {
		t.Errorf("reference = %v, want tag %q", or.Spec.Reference, "v1.2.3")
	}
	if or.Name != "prod" {
		t.Errorf("source name = %q, want the root node's name", or.Name)
	}
}

// TestLayoutKustomizationPruneIsAnInput pins the third prune site. A
// layout.ManifestLayout carries no prune setting of its own, so Kustomizations
// generated from one read ResourceGenerator.Prune; that site used to be an
// unconditional true. It is reached only through the LayoutIntegrator in
// FluxIntegratedPerLayout placement, and the two bundle-level prune tests do not
// cover it, so reverting it alone would leave the suite green.
func TestLayoutKustomizationPruneIsAnInput(t *testing.T) {
	yes, no := true, false
	for name, tc := range map[string]struct {
		prune *bool
		want  bool
	}{
		"unset stays off":     {nil, false},
		"explicit true is on": {&yes, true},
		"explicit false":      {&no, false},
	} {
		t.Run(name, func(t *testing.T) {
			g := NewResourceGenerator()
			g.Prune = tc.prune
			ml := &layout.ManifestLayout{Name: "platform", Namespace: "clusters/prod"}

			obj := g.createKustomizationForLayout(ml, kustv1.CrossNamespaceSourceReference{
				Kind: DefaultSourceKind,
				Name: DefaultSourceName,
			})
			k, ok := obj.(*kustv1.Kustomization)
			if !ok {
				t.Fatalf("expected a Kustomization, got %T", obj)
			}
			if k.Spec.Prune != tc.want {
				t.Errorf("prune = %v, want %v", k.Spec.Prune, tc.want)
			}
		})
	}
}

// TestSeparateFluxLayoutUsesDeclaredDefaults pins the separate-flux layout's
// injected values. The directory name and the file granularity were literals
// here; the granularity now comes from layout.DefaultLayoutRules rather than a
// second copy of it, so a change to the layout package's own default is
// followed rather than silently diverged from.
//
// Mode must stay unset. Seeding it would be inert (see addSeparateFluxToLayout)
// while making DefaultMode look overrideable at a site that never reads
// ResourceGenerator.Mode.
//
// As in TestConstructorsSeedTheExportedDefaults, these compare values and
// cannot detect an equal literal re-inlined at the assignment.
func TestSeparateFluxLayoutUsesDeclaredDefaults(t *testing.T) {
	li := NewLayoutIntegrator(NewResourceGenerator())
	ml := &layout.ManifestLayout{Name: "prod", Namespace: "clusters/prod"}
	cluster := &stack.Cluster{
		Name: "prod",
		Node: &stack.Node{
			Name:   "root",
			Bundle: &stack.Bundle{Name: "apps"},
		},
	}

	if err := li.IntegrateWithLayout(ml, cluster, layout.LayoutRules{
		FluxPlacement: layout.FluxSeparate,
	}); err != nil {
		t.Fatalf("IntegrateWithLayout: %v", err)
	}

	var fluxLayout *layout.ManifestLayout
	for _, child := range ml.Children {
		if child.Name == DefaultFluxDirName {
			fluxLayout = child
		}
	}
	if fluxLayout == nil {
		t.Fatalf("no child named DefaultFluxDirName (%q); children are %v",
			DefaultFluxDirName, childNames(ml))
	}
	if fluxLayout.Mode != layout.KustomizationUnset {
		t.Errorf("Mode = %v, want KustomizationUnset — the layout writer resolves it and this layout has no children for a mode to select between",
			fluxLayout.Mode)
	}
	if want := layout.DefaultLayoutRules().FilePer; fluxLayout.FilePer != want {
		t.Errorf("FilePer = %v, want layout.DefaultLayoutRules().FilePer (%v)", fluxLayout.FilePer, want)
	}
}

func childNames(ml *layout.ManifestLayout) []string {
	names := make([]string, 0, len(ml.Children))
	for _, c := range ml.Children {
		names = append(names, c.Name)
	}
	return names
}

// TestNormalizeRulesPlacementFollowsTheLayoutPackage pins that unset placement
// resolves to whatever layout.DefaultLayoutRules declares, not to a constant
// this package keeps in parallel with it.
func TestNormalizeRulesPlacementFollowsTheLayoutPackage(t *testing.T) {
	got := normalizeRulesPlacement(layout.LayoutRules{FluxPlacement: layout.FluxUnset})
	if want := layout.DefaultLayoutRules().FluxPlacement; got.FluxPlacement != want {
		t.Errorf("FluxPlacement = %v, want layout.DefaultLayoutRules().FluxPlacement (%v)",
			got.FluxPlacement, want)
	}

	explicit := normalizeRulesPlacement(layout.LayoutRules{FluxPlacement: layout.FluxIntegratedPerLayout})
	if explicit.FluxPlacement != layout.FluxIntegratedPerLayout {
		t.Errorf("an explicit placement must survive normalization, got %v", explicit.FluxPlacement)
	}
}

// The two bootstrap dangling-reference and nil-root-node tests live in
// bootstrap_generator_test.go: both defects predate this change and were fixed
// in their own commits, so their tests sit with the rest of the bootstrap
// generator's tests rather than with the defaults.

// TestGotkBootstrapWithoutASourceURLEmitsNoSource pins the guard that makes an
// absent SourceURL mean "the caller supplies the source", which is not an error.
// The private source generators reject an empty URL; GenerateBootstrap must not
// reach them. Removing the guard turns a supported call into an error, and
// before this test nothing failed when it did.
func TestGotkBootstrapWithoutASourceURLEmitsNoSource(t *testing.T) {
	bg := NewBootstrapGenerator()
	objs, err := bg.GenerateBootstrap(&stack.BootstrapConfig{
		Enabled:  true,
		FluxMode: "gotk",
		// No SourceURL: the caller supplies the source themselves.
	}, &stack.Node{Name: "prod"})
	if err != nil {
		t.Fatalf("an absent SourceURL must not be an error: %v", err)
	}

	var kustomizations int
	for _, o := range objs {
		switch o.(type) {
		case *kustv1.Kustomization:
			kustomizations++
		case *sourcev1.GitRepository:
			t.Error("emitted a GitRepository for a caller who named no SourceURL")
		case *sourcev1.OCIRepository:
			t.Error("emitted an OCIRepository for a caller who named no SourceURL")
		}
	}
	if kustomizations == 0 {
		t.Error("expected the bootstrap Kustomization to still be emitted")
	}
}
