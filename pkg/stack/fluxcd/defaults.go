package fluxcd

import (
	"time"

	"github.com/go-kure/kure/pkg/stack/layout"
)

// The values a generator falls back to when the caller supplies nothing.
//
// pkg/stack is a workflow layer above pkg/kubernetes and may hold opinions —
// but as declared inputs with names a consumer can read, compare against and
// override, never as literals buried in a constructor. Every fallback the
// generators apply is one of the identifiers below; grep for the identifier to
// find every place the value can reach emitted YAML.
//
// These are defaults, not policy, and they are overridden in one of two ways.
// [DefaultNamespace], [DefaultInterval], [DefaultMode] and [DefaultBootstrapName]
// are copied into an exported field of [ResourceGenerator] or
// [BootstrapGenerator] by its constructor, and a caller that assigns the field
// afterwards is never overridden. The rest are applied where they are used, and
// are overridden by naming the corresponding input on [stack.BootstrapConfig],
// [stack.Bundle] or the root [stack.Node] — the per-identifier comments below say
// which input each one yields to.
//
// Three have no override at all: [DefaultSyncPath], [DefaultBootstrapPathRoot]
// and [DefaultFluxDirName]. Each is a fixed structural segment of a path the
// package builds, not a value a caller replaces, and their comments say so.
// They are named here anyway so the value is greppable and reviewable rather
// than a literal inside the function that emits it.
//
// Interval and namespace are the two that always reach output.
// KustomizationSpec.Interval is +required upstream with no omitempty
// (kustomize-controller api/v1/kustomization_types.go:68), so a Kustomization
// cannot be emitted without one; DefaultInterval is what a caller who names no
// interval gets.
const (
	// DefaultNamespace is the namespace generated Flux resources are placed in
	// when the caller names none.
	DefaultNamespace = "flux-system"

	// DefaultSourceName is the name given to a generated GitRepository or
	// OCIRepository when the root node has no name of its own.
	DefaultSourceName = "flux-system"

	// DefaultBootstrapName is the name given to the bootstrap Kustomization and
	// to the FluxInstance. Neither is derived from the root node. Override it by
	// assigning [BootstrapGenerator.BootstrapName].
	DefaultBootstrapName = "flux-system"

	// DefaultFluxDirName is the directory a separate Flux layout is placed in,
	// under FluxSeparate placement. It is a path segment, not a namespace: it
	// happens to share [DefaultNamespace]'s value and must not be derived from
	// it, because a caller who renames the namespace does not thereby rename
	// the directory. It has no override.
	DefaultFluxDirName = "flux-system"

	// DefaultBootstrapPathRoot is the first segment of the bootstrap
	// Kustomization's spec.path; the root node's name is joined onto it. It is a
	// fixed segment with no override — rename the root node to change the path.
	DefaultBootstrapPathRoot = "manifests"

	// DefaultFluxMode is the bootstrap mode used when BootstrapConfig.FluxMode
	// is empty. It is also the mode GenerateBootstrap dispatches on and the
	// first entry [BootstrapGenerator.SupportedBootstrapModes] reports, so
	// changing it here changes the default and the accepted spelling together
	// — the alternative left an empty FluxMode resolving to a mode the switch
	// no longer recognised.
	DefaultFluxMode = "flux-operator"

	// ModeGotk is the legacy bootstrap mode. It is not a default — nothing
	// falls back to it — but it is named here so the mode set has one
	// authority alongside [DefaultFluxMode] rather than a literal repeated in
	// the dispatch switch and the supported-mode list.
	ModeGotk = "gotk"

	// DefaultSourceKind is the kind of source object bootstrap emits, and
	// references, when BootstrapConfig.SourceKind does not name "GitRepository".
	// That includes the empty string: an unnamed kind yields an OCIRepository
	// for backward compatibility, which is generateSource's own behaviour.
	//
	// There is deliberately one identifier rather than one per emission site.
	// The bootstrap Kustomization's sourceRef, the source object itself and the
	// FluxInstance sync block previously each decided the kind for themselves,
	// and the first of the three used the opposite polarity to the other two —
	// so an empty SourceKind emitted an OCIRepository under a sourceRef naming a
	// GitRepository that was never created. [resolvedSourceKind] is now the only
	// place that decision is made.
	DefaultSourceKind = "OCIRepository"

	// DefaultSourceRef is the OCI tag used when BootstrapConfig.SourceRef is
	// empty. It has no GitRepository equivalent: an empty SourceRef leaves the
	// GitRepository reference unset rather than guessing a branch.
	DefaultSourceRef = "latest"

	// DefaultSyncPath is the path a FluxInstance sync block uses when the root
	// node has no name, and the prefix its name is appended to when it has one.
	// It has no override: it is the prefix a sync path is built from, not a
	// value a caller replaces.
	DefaultSyncPath = "./"
)

// DefaultInterval is the reconciliation interval used when the caller names
// none, on both generators.
//
// It also applies when Bundle.Interval is non-empty but does not parse: the
// parse error is swallowed and the default retained
// (see createKustomization in resource_generator.go). That is pre-existing
// behaviour, stated here rather than quietly implied by "names none", and
// tracked for a decision in issue go-kure/kure#762.
const DefaultInterval = 60 * time.Minute

// DefaultMode is the Kustomization path mode [ResourceGenerator] starts in. It
// has exactly one application site, NewResourceGenerator, and is overridden by
// assigning [ResourceGenerator.Mode]. The separate Flux layout deliberately
// leaves its own mode unset rather than seeding it from here — see
// addSeparateFluxToLayout for why that value could not take effect.
const DefaultMode = layout.KustomizationExplicit

// pruneValue resolves a tri-state prune input to the bool the upstream field
// requires.
//
// KustomizationSpec.Prune is +required with no omitempty
// (kustomize-controller api/v1/kustomization_types.go:99-101), so an unset
// input cannot leave the key out of the emitted YAML: nil emits prune: false,
// garbage collection off. That is the change of direction this release makes
// deliberately — an unset tri-state used to collapse onto prune: true, which
// enabled destructive garbage collection for a caller who never asked for it.
// A caller that wants pruning now says so.
func pruneValue(prune *bool) bool {
	return prune != nil && *prune
}

// waitValue resolves a tri-state wait input to the bool the upstream field
// requires.
//
// KustomizationSpec.Wait is +optional with omitempty
// (kustomize-controller api/v1/kustomization_types.go:174-178), so nil and
// false are the same emitted YAML: the key is absent and the Flux default,
// false, applies.
func waitValue(wait *bool) bool {
	return wait != nil && *wait
}
