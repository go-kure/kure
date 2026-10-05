package fluxcd

import (
	"fmt"
	"strings"
	"time"

	fluxv1 "github.com/controlplaneio-fluxcd/flux-operator/api/v1"
	"github.com/fluxcd/flux2/v2/pkg/manifestgen/install"
	kustv1 "github.com/fluxcd/kustomize-controller/api/v1"
	sourcev1 "github.com/fluxcd/source-controller/api/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/validation"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/go-kure/kure/pkg/errors"
	kio "github.com/go-kure/kure/pkg/io"
	pubfluxcd "github.com/go-kure/kure/pkg/kubernetes/fluxcd"
	"github.com/go-kure/kure/pkg/stack"
	"github.com/go-kure/kure/pkg/stack/layout"
)

// BootstrapGenerator implements the workflow.BootstrapGenerator interface for Flux.
// It handles the generation of bootstrap resources for setting up Flux.
type BootstrapGenerator struct {
	// DefaultNamespace is the namespace where bootstrap resources are created
	DefaultNamespace string
	// DefaultInterval is the default reconciliation interval
	DefaultInterval time.Duration
	// BootstrapName is the name given to the bootstrap Kustomization. It is not
	// derived from the root node, and BootstrapConfig carries no equivalent
	// input, so this field is the only way to override [DefaultBootstrapName].
	// Leaving it empty means the default, not a nameless object — see
	// [BootstrapGenerator.bootstrapName].
	//
	// It does not name the FluxInstance. That object's metadata.name is fixed
	// at [FluxInstanceName] by the flux-operator CRD, which admits no other
	// value; it used to take this field, so any bundle whose BootstrapName was
	// not "flux" — the default included — was rejected at apply
	// (go-kure/kure#847).
	BootstrapName string
}

// bootstrapName is the name the bootstrap Kustomization is given. An empty
// BootstrapName resolves to [DefaultBootstrapName] rather than emitting an
// object with no metadata.name.
//
// The field is new: before it, the name was the literal that
// DefaultBootstrapName now holds, so it could not be absent. A caller that
// builds a BootstrapGenerator as a struct literal instead of through
// NewBootstrapGenerator would otherwise have started emitting invalid YAML
// merely because a field appeared. DefaultNamespace and DefaultInterval carry
// the same hazard, but they carried it before this change too and resolving
// them here would alter existing behaviour rather than preserve it.
func (bg *BootstrapGenerator) bootstrapName() string {
	if bg.BootstrapName == "" {
		return DefaultBootstrapName
	}
	return bg.BootstrapName
}

// NewBootstrapGenerator creates a FluxCD bootstrap generator seeded with the
// exported defaults ([DefaultNamespace], [DefaultInterval],
// [DefaultBootstrapName]). Assign the fields afterwards to override any of them.
func NewBootstrapGenerator() *BootstrapGenerator {
	return &BootstrapGenerator{
		DefaultNamespace: DefaultNamespace,
		DefaultInterval:  DefaultInterval,
		BootstrapName:    DefaultBootstrapName,
	}
}

// GenerateBootstrap creates bootstrap resources for setting up Flux.
// When FluxMode is empty, flux-operator is used as the default.
//
// rules are the layout rules the tree is written with: both modes point Flux
// at the top directory of the tree a walk with them writes for rootNode
// (bootstrapDir). They are validated first, as a walk validates them, so
// invalid rules are an error whatever config is, a nil or disabled one
// included. The directory itself is asked for only where it is used, so the
// root node's name is checked as a directory name only where it becomes a
// path (validateRootName).
func (bg *BootstrapGenerator) GenerateBootstrap(config *stack.BootstrapConfig, rootNode *stack.Node, rules layout.LayoutRules) ([]client.Object, error) {
	if err := rules.Validate(); err != nil {
		return nil, err
	}
	if config == nil || !config.Enabled {
		return nil, nil
	}

	mode := config.FluxMode
	if mode == "" {
		mode = DefaultFluxMode
	}

	switch mode {
	case DefaultFluxMode:
		if err := validateSyncRootName(config, rootNode, rules); err != nil {
			return nil, err
		}
		dir, err := syncDir(config, rootNode, rules)
		if err != nil {
			return nil, err
		}
		return bg.generateFluxOperatorBootstrap(config, dir)
	case ModeGotk:
		// The bootstrap Kustomization's spec.path is built with or without
		// a SourceURL.
		if err := validateRootName(rootNode, rules); err != nil {
			return nil, err
		}
		if err := validateRootSourceName(rootNode); err != nil {
			return nil, err
		}
		dir, err := bootstrapDir(rootNode, rules)
		if err != nil {
			return nil, err
		}
		return bg.generateGotkBootstrap(config, rootNode, dir)
	default:
		return nil, errors.NewValidationError("fluxMode", config.FluxMode, "BootstrapConfig",
			bg.SupportedBootstrapModes())
	}
}

// SupportedBootstrapModes returns the bootstrap modes supported by this generator.
// [DefaultFluxMode] is the primary (recommended) mode; [ModeGotk] is the legacy
// one. This is the single list: GenerateBootstrap's validation error reports it
// rather than restating the modes.
func (bg *BootstrapGenerator) SupportedBootstrapModes() []string {
	return []string{DefaultFluxMode, ModeGotk}
}

// generateGotkBootstrap generates bootstrap resources using the standard Flux
// toolkit. dir is the directory the bootstrap Kustomization applies
// (bootstrapDir).
func (bg *BootstrapGenerator) generateGotkBootstrap(config *stack.BootstrapConfig, rootNode *stack.Node, dir string) ([]client.Object, error) {
	var resources []client.Object

	// Generate core Flux components
	gotkResources, err := bg.generateGotkComponents(config)
	if err != nil {
		return nil, errors.ResourceValidationError("BootstrapConfig", "gotk", "components",
			fmt.Sprintf("failed to generate gotk components: %v", err), err)
	}
	resources = append(resources, gotkResources...)

	// Generate flux-system Kustomization
	fluxSystemKust := bg.generateFluxSystemKustomization(config, rootNode, dir)
	resources = append(resources, fluxSystemKust)

	// Generate source for the root node based on SourceKind. An absent
	// SourceURL means the caller supplies the source itself, so no source is
	// emitted; it is not a request for a placeholder.
	if config.SourceURL != "" {
		source, err := bg.generateSource(config, rootNode)
		if err != nil {
			return nil, err
		}
		resources = append(resources, source)
	}

	return resources, nil
}

// generateFluxOperatorBootstrap generates bootstrap resources using the Flux Operator.
//
// Output order (also a valid apply order):
//  1. Flux Operator install bundle — Namespace, CRDs, RBAC, ServiceAccount,
//     Service, controller Deployment (from the embedded upstream install.yaml,
//     see FluxOperatorInstallObjects / FluxOperatorVersion).
//  2. FluxInstance CR — configured from BootstrapConfig.
//
// Prior to kure v0.1.0-rc.5 only the FluxInstance was emitted, which
// required every caller to provide the Flux Operator install bundle
// separately. Emitting the full set here makes the generator self-sufficient
// so callers can return a
// single apply-ready bundle.
//
// dir is the directory the FluxInstance's sync applies (bootstrapDir).
func (bg *BootstrapGenerator) generateFluxOperatorBootstrap(config *stack.BootstrapConfig, dir string) ([]client.Object, error) {
	installObjs, err := FluxOperatorInstallObjects()
	if err != nil {
		return nil, errors.ResourceValidationError("BootstrapConfig", "flux-operator", "install",
			fmt.Sprintf("failed to load vendored flux-operator install bundle: %v", err), err)
	}

	fluxInstance, err := bg.generateFluxInstance(config, dir)
	if err != nil {
		return nil, err
	}

	resources := make([]client.Object, 0, len(installObjs)+1)
	resources = append(resources, installObjs...)
	resources = append(resources, fluxInstance)
	return resources, nil
}

// generateGotkComponents generates the standard Flux toolkit components.
func (bg *BootstrapGenerator) generateGotkComponents(config *stack.BootstrapConfig) ([]client.Object, error) {
	// Create install options with defaults
	opts := install.MakeDefaultOptions()

	// Place the components in the same namespace as the rest of the bundle. The
	// root Kustomization and the root source already use bg.DefaultNamespace, so
	// leaving this at the upstream default splits a non-default bundle: the
	// controllers' Deployments, ServiceAccounts, Services, NetworkPolicies and
	// ResourceQuota stay in flux-system while the objects that rely on them land
	// elsewhere. WatchAllNamespaces defaults to true, so the split still
	// reconciles -- until someone narrows the controllers, at which point it goes
	// quiet with no error. Setting the option is enough for the cluster-scoped
	// objects too: install.Generate derives the emitted Namespace's name and the
	// ClusterRoleBinding names and subjects from it.
	//
	// The guard is not cosmetic. install.Generate uses this option as the emitted
	// Namespace's *name*, so assigning it unconditionally makes a struct-literal
	// generator (DefaultNamespace zero) fail the whole bundle with "missing
	// metadata.name in object {{v1 Namespace}}" rather than merely produce an
	// unnamespaced one. Such a generator is already inconsistent -- the root
	// Kustomization and source take the same empty value and emit no namespace at
	// all -- but that is a package-wide question about struct-literal
	// construction, and turning it into a hard error here is not the fix for it.
	if bg.DefaultNamespace != "" {
		opts.Namespace = bg.DefaultNamespace
	}

	// Build from the vendored GotkVersion bundle unless FluxVersion asks for a
	// different release. install.Generate skips its download whenever it is
	// given a manifests base, so the default path makes no network call and
	// its output depends only on kure's version (go-kure/kure#794). Any other
	// FluxVersion ("latest" included) keeps the upstream fetch as an explicit
	// opt-in.
	manifestsBase, cleanup, err := gotkManifestsBase(config.FluxVersion)
	if err != nil {
		return nil, errors.ResourceValidationError("BootstrapConfig", "gotk", "install",
			fmt.Sprintf("failed to prepare Flux installation manifests: %v", err), err)
	}
	defer cleanup()
	if manifestsBase != "" {
		opts.Version = GotkVersion
	} else {
		opts.Version = config.FluxVersion
	}

	// Set registry if specified
	if config.Registry != "" {
		opts.Registry = config.Registry
	}

	// Set image pull secret if specified
	if config.ImagePullSecret != "" {
		opts.ImagePullSecret = config.ImagePullSecret
	}

	// Set components if specified
	if len(config.Components) > 0 {
		opts.Components = config.Components
	}

	// Generate manifests
	content, err := install.Generate(opts, manifestsBase)
	if err != nil {
		return nil, errors.ResourceValidationError("BootstrapConfig", "gotk", "install",
			fmt.Sprintf("failed to generate Flux installation manifests: %v", err), err)
	}

	// Parse the generated manifests
	objects, err := kio.ParseYAML([]byte(content.Content))
	if err != nil {
		return nil, errors.NewParseError("gotk manifests", "failed to parse generated manifests", 0, 0, err)
	}

	return objects, nil
}

// rootName returns the root node's name, or "" when there is no root node or it
// is unnamed. A nil root node is legitimate — the caller may be bootstrapping
// before any node exists — so every site that reads the name goes through here
// rather than dereferencing. The bootstrap Kustomization's spec.path previously
// dereferenced it directly, one line below a nil-guarded call, and panicked.
func rootName(rootNode *stack.Node) string {
	if rootNode == nil {
		return ""
	}
	return rootNode.Name
}

// bootstrapDir returns the directory both bootstrap modes point Flux at,
// relative to the root of the source: the top directory of the tree a walk
// with rules writes for rootNode (layout.TopDirectory, which the walk takes
// its own top from). That is the cluster directory when the rules have a
// ClusterName; without one the root node's name, "cluster" for an unnamed root
// node, and "." — the root of the source — when there is no root node. Invalid
// rules are the error layout.TopDirectory returns for them.
//
// A rooted directory (a ClusterName such as "/prod") comes back from
// layout.TopDirectory without its leading slash, which is where the writers
// put it: they resolve it under the directory they write to.
//
// Each mode spells that one directory its own way. The gotk bootstrap
// Kustomization's spec.path is the directory itself with no "./" prefix, and
// "." for the root of the source, as layout.ManifestLayout.FullRepoPath
// spells every other spec.path the package writes. The FluxInstance's
// sync.path is [DefaultSyncPath] followed by the directory (syncPath).
//
// The directory used to be the root node's name whatever the rules were
// (go-kure/kure#979): the bootstrap was given the root node and not the rules,
// so under a ClusterName, and for an unnamed root node without one, Flux was
// pointed at a directory the walk did not write. Before that the gotk path was
// "manifests/<root>", a prefix no writer of the package produces, while the
// FluxInstance named "./<root>": two directories for one root.
func bootstrapDir(rootNode *stack.Node, rules layout.LayoutRules) (string, error) {
	return layout.TopDirectory(rootNode, rules)
}

// syncDir returns the directory a FluxInstance's sync applies (bootstrapDir),
// and "" when config builds no sync. Without a SourceURL the directory goes
// nowhere (generateFluxInstance), so it is not asked for: layout.TopDirectory
// refuses a root node name that is no directory name, and such a name stays
// accepted where no path is built from it (validateSyncRootName).
func syncDir(config *stack.BootstrapConfig, rootNode *stack.Node, rules layout.LayoutRules) (string, error) {
	if config == nil || config.SourceURL == "" {
		return "", nil
	}
	return bootstrapDir(rootNode, rules)
}

// syncPath spells the directory bootstrapDir returns as a FluxInstance
// sync.path: [DefaultSyncPath] alone for the root of the source, followed by
// the directory otherwise.
func syncPath(dir string) string {
	if dir == "." {
		return DefaultSyncPath
	}
	return DefaultSyncPath + dir
}

// sourceName returns the name a generated GitRepository or OCIRepository
// carries: the root node's name when it has one, [DefaultSourceName] otherwise.
// The bootstrap Kustomization's sourceRef must resolve through this same
// function: it previously hardcoded [DefaultSourceName] while the source object
// took the root node's name, so a named root node produced a Kustomization
// pointing at a source that was never emitted.
func sourceName(rootNode *stack.Node) string {
	if name := rootName(rootNode); name != "" {
		return name
	}
	return DefaultSourceName
}

// validateRootName checks the root node's name where it is about to become a
// path segment: of the gotk bootstrap Kustomization's spec.path and of the
// FluxInstance's sync.path. The bootstrap entry points take a node, not a
// cluster, so stack.ValidateCluster has not checked it. No root node and an
// unnamed root are valid: neither adds a segment. Nor does any root node under
// rules with a ClusterName, where the path is the cluster directory
// (bootstrapDir) and the name goes nowhere in it.
func validateRootName(rootNode *stack.Node, rules layout.LayoutRules) error {
	name := rootName(rootNode)
	if name == "" || rules.ClusterName != "" {
		return nil
	}
	if err := stack.ValidateDirectoryName(name); err != nil {
		return errors.ResourceValidationError("Node", name, "name", err.Error(), nil)
	}
	return nil
}

// validateRootSourceName checks the root node's name where it becomes an
// object name. In gotk mode a named root names the generated GitRepository or
// OCIRepository and the bootstrap Kustomization's sourceRef (sourceName), so
// the name has to be a DNS-1123 subdomain. No root node and an unnamed root
// take [DefaultSourceName].
func validateRootSourceName(rootNode *stack.Node) error {
	name := rootName(rootNode)
	if name == "" {
		return nil
	}
	if problems := validation.IsDNS1123Subdomain(name); len(problems) > 0 {
		return errors.ResourceValidationError("Node", name, "name",
			fmt.Sprintf("%q is not a valid name for the bootstrap source: %s", name, strings.Join(problems, "; ")), nil)
	}
	return nil
}

// validateSyncRootName checks the root node's name for a FluxInstance. The
// name is used only in sync.path, and generateFluxInstance builds a sync only
// when config has a SourceURL: without one the name goes nowhere and is not
// checked. A nil config builds no FluxInstance at all, so it is accepted here
// and the caller need not test for it first.
func validateSyncRootName(config *stack.BootstrapConfig, rootNode *stack.Node, rules layout.LayoutRules) error {
	if config == nil || config.SourceURL == "" {
		return nil
	}
	return validateRootName(rootNode, rules)
}

// resolvedSourceKind returns the kind of source object bootstrap will emit for
// config, and therefore the kind every reference to it must name. A
// GitRepository only when SourceKind says so; an OCIRepository for every other
// value, the empty string included ([DefaultSourceKind]).
//
// This is the sourceName problem in the other field. The three sites that need
// the kind — the source object, the bootstrap Kustomization's sourceRef, and the
// FluxInstance sync block — each decided it independently, and the sourceRef
// used the opposite test ("is it OCIRepository?" rather than "is it
// GitRepository?"). The two agreed only when SourceKind named one of them
// exactly; an empty or unrecognised kind emitted an OCIRepository beneath a
// sourceRef pointing at a GitRepository of the same name, which does not exist.
func resolvedSourceKind(config *stack.BootstrapConfig) string {
	if config != nil && config.SourceKind == "GitRepository" {
		return "GitRepository"
	}
	return DefaultSourceKind
}

// resolvedSyncRef returns the FluxInstance spec.sync.ref that, for an OCI tag,
// an empty ref or a Git branch name, selects the same revision the gotk source
// would. flux-operator renders sync.ref as the
// source's ref.tag for an OCIRepository and as ref.name — a full Git
// reference such as refs/heads/main — for a GitRepository, while the gotk path
// reads SourceRef as an OCI tag (with [DefaultSourceRef] for an empty one) or a
// Git branch. Passing SourceRef through verbatim therefore emitted an empty
// OCI tag where gotk uses DefaultSourceRef, and a bare branch name where Flux
// needs a full reference.
//
// A Git SourceRef that already starts with refs/ is kept, so tags and other
// references stay reachable in flux-operator mode. That case has no gotk
// equivalent: generateGitSource puts any Git SourceRef into ref.branch. An
// empty Git SourceRef stays empty, as it leaves the gotk GitRepository's
// reference unset.
func resolvedSyncRef(config *stack.BootstrapConfig) string {
	ref := config.SourceRef
	if resolvedSourceKind(config) == "GitRepository" {
		if ref == "" || strings.HasPrefix(ref, "refs/") {
			return ref
		}
		return "refs/heads/" + ref
	}
	if ref == "" {
		return DefaultSourceRef
	}
	return ref
}

// generateFluxSystemKustomization creates a Kustomization for the flux-system.
// Its spec.path is dir, the directory bootstrapDir names, "." for the root of
// the source. Its sourceRef stays on the root node's name (sourceName).
func (bg *BootstrapGenerator) generateFluxSystemKustomization(config *stack.BootstrapConfig, rootNode *stack.Node, dir string) client.Object {
	kust := &kustv1.Kustomization{
		TypeMeta: metav1.TypeMeta{
			APIVersion: kustv1.GroupVersion.String(),
			Kind:       "Kustomization",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      bg.bootstrapName(),
			Namespace: bg.DefaultNamespace,
		},
		Spec: kustv1.KustomizationSpec{
			Interval: metav1.Duration{Duration: bg.DefaultInterval},
			Path:     dir,
			Prune:    pruneValue(config.Prune),
			SourceRef: kustv1.CrossNamespaceSourceReference{
				Kind: resolvedSourceKind(config),
				Name: sourceName(rootNode),
			},
		},
	}

	return kust
}

// generateSource creates a source resource of the kind [resolvedSourceKind]
// names, so that the object emitted here and every reference to it elsewhere
// cannot disagree about what was created.
func (bg *BootstrapGenerator) generateSource(config *stack.BootstrapConfig, rootNode *stack.Node) (client.Object, error) {
	if resolvedSourceKind(config) == "GitRepository" {
		return bg.generateGitSource(config, rootNode)
	}
	return bg.generateOCISource(config, rootNode)
}

// generateGitSource creates a GitRepository source for bootstrap from config.
func (bg *BootstrapGenerator) generateGitSource(config *stack.BootstrapConfig, rootNode *stack.Node) (client.Object, error) {
	if config.SourceURL == "" {
		return nil, errors.ResourceValidationError("BootstrapConfig", sourceName(rootNode), "sourceURL",
			"a GitRepository source requires sourceURL; there is no default repository", nil)
	}

	gr := pubfluxcd.CreateGitRepository(sourceName(rootNode), bg.DefaultNamespace)
	gr.Spec.URL = config.SourceURL
	gr.Spec.Interval = metav1.Duration{Duration: bg.DefaultInterval}

	if config.SourceRef != "" {
		pubfluxcd.SetGitRepositoryReference(gr, &sourcev1.GitRepositoryRef{Branch: config.SourceRef})
	}

	return gr, nil
}

// generateOCISource creates an OCI source for bootstrap from config.
func (bg *BootstrapGenerator) generateOCISource(config *stack.BootstrapConfig, rootNode *stack.Node) (client.Object, error) {
	if config.SourceURL == "" {
		return nil, errors.ResourceValidationError("BootstrapConfig", sourceName(rootNode), "sourceURL",
			"an OCIRepository source requires sourceURL; there is no default registry", nil)
	}

	ref := config.SourceRef
	if ref == "" {
		ref = DefaultSourceRef
	}

	or := pubfluxcd.CreateOCIRepository(sourceName(rootNode), bg.DefaultNamespace)
	or.Spec.URL = config.SourceURL
	or.Spec.Interval = metav1.Duration{Duration: bg.DefaultInterval}
	pubfluxcd.SetOCIRepositoryReference(or, &sourcev1.OCIRepositoryRef{Tag: ref})

	return or, nil
}

// GenerateFluxInstance returns only the FluxInstance CR configured for
// the given bootstrap settings, without the full Flux Operator install bundle.
// Returns (nil, nil) when config is nil. Unlike GenerateBootstrap, this method
// does not check config.Enabled — the caller is responsible for that gate.
// An empty FluxVersion or Registry is an error, as on the bootstrap path.
//
// rules are the layout rules the tree is written with, as in
// GenerateBootstrap: the sync.path is the top directory of that tree, and
// invalid rules are an error whatever config is, nil included. The directory
// is asked for only when a sync is built (syncDir).
func (bg *BootstrapGenerator) GenerateFluxInstance(config *stack.BootstrapConfig, rootNode *stack.Node, rules layout.LayoutRules) (*fluxv1.FluxInstance, error) {
	if err := rules.Validate(); err != nil {
		return nil, err
	}
	if err := validateSyncRootName(config, rootNode, rules); err != nil {
		return nil, err
	}
	if config == nil {
		return nil, nil
	}
	dir, err := syncDir(config, rootNode, rules)
	if err != nil {
		return nil, err
	}
	obj, err := bg.generateFluxInstance(config, dir)
	if err != nil {
		return nil, err
	}
	fi, ok := obj.(*fluxv1.FluxInstance)
	if !ok {
		return nil, errors.Errorf("internal error: generateFluxInstance returned unexpected type %T", obj)
	}
	return fi, nil
}

// requireDistribution returns an error naming each of FluxVersion and Registry
// that config leaves empty. Both go verbatim into the FluxInstance's
// spec.distribution, and a FluxInstance with an empty version or registry
// cannot work; it used to be written anyway, as `version: ""` and
// `registry: ""`. No default is filled in: the caller supplies both.
//
// This is flux-operator mode only. gotk mode reads the same two fields, and
// there an empty FluxVersion means the vendored release and an empty Registry
// the upstream one.
func requireDistribution(config *stack.BootstrapConfig) error {
	var keys, fields []string
	if config.FluxVersion == "" {
		keys, fields = append(keys, "fluxVersion"), append(fields, "FluxVersion")
	}
	if config.Registry == "" {
		keys, fields = append(keys, "registry"), append(fields, "Registry")
	}
	if len(fields) == 0 {
		return nil
	}
	return errors.ResourceValidationError("BootstrapConfig", FluxInstanceName, strings.Join(keys, ", "),
		fmt.Sprintf("flux-operator mode requires %s for the FluxInstance distribution; there is no default",
			strings.Join(fields, " and ")), nil)
}

// generateFluxInstance creates a FluxInstance for flux-operator mode. It is the
// one place both entry points build it, so the distribution check lives here.
// dir is the directory its sync applies (bootstrapDir).
func (bg *BootstrapGenerator) generateFluxInstance(config *stack.BootstrapConfig, dir string) (client.Object, error) {
	if err := requireDistribution(config); err != nil {
		return nil, err
	}

	spec := fluxv1.FluxInstanceSpec{
		Distribution: fluxv1.Distribution{
			Version:  config.FluxVersion,
			Registry: config.Registry,
		},
	}

	// Add components if specified
	for _, comp := range config.Components {
		spec.Components = append(spec.Components, fluxv1.Component(comp))
	}

	// Add sync configuration if source is provided
	if config.SourceURL != "" {
		spec.Sync = &fluxv1.Sync{
			Name:     config.SyncName,
			Kind:     resolvedSourceKind(config),
			URL:      config.SourceURL,
			Ref:      resolvedSyncRef(config),
			Path:     syncPath(dir),
			Interval: &metav1.Duration{Duration: bg.DefaultInterval},
		}
	}

	// The name is not bg.bootstrapName(): the CRD accepts only FluxInstanceName
	// and rejects anything else at admission.
	fi := pubfluxcd.CreateFluxInstance(FluxInstanceName, bg.DefaultNamespace)
	fi.Spec = spec
	return fi, nil
}
