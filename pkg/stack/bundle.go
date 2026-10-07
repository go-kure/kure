package stack

import (
	"fmt"
	"slices"
	"time"

	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/go-kure/kure/pkg/errors"
)

const (
	// AnnotationFluxPruneKey is the Flux kustomize-controller annotation key
	// used to control pruning behavior on individual resources.
	AnnotationFluxPruneKey = "kustomize.toolkit.fluxcd.io/prune"
	// AnnotationFluxPruneDisabled is the value that prevents a resource from
	// being pruned during Flux garbage collection.
	AnnotationFluxPruneDisabled = "disabled"
	// AnnotationFluxForceKey is the Flux kustomize-controller annotation key
	// that controls, per resource, whether an immutable-field change is
	// applied by deleting and recreating the resource.
	AnnotationFluxForceKey = "kustomize.toolkit.fluxcd.io/force"
	// AnnotationFluxForceEnabled is the value that allows that recreation.
	AnnotationFluxForceEnabled = "enabled"
)

// Bundle represents a unit of deployment, typically the resources that
// are reconciled by a single Flux Kustomization.
type Bundle struct {
	// Name identifies the application set.
	Name string
	// KustomizationName names the Flux Kustomization generated for the
	// bundle, and with it every reference to that Kustomization: dependsOn
	// entries of bundles that depend on this one, and an umbrella's health
	// check on this child. Empty means Name. Under the ArgoCD workflow the
	// same value names the Application and its spec.dependencies entries.
	// Name stays the bundle's identity and, unless DirName is set, its
	// directory; UnitName returns the name in effect. A value set here must
	// be a DNS-1123 subdomain (Validate); the Flux workflow also limits the
	// name in effect to 63 characters (ValidateKustomizationName).
	KustomizationName string
	// DirName names the bundle's directory in the rendered tree, wherever the
	// bundle's name becomes one. Empty means Name. That is the case for an
	// umbrella child, for a node's bundle under the layout rule
	// BundleGrouping GroupByName, and for the root node's bundle under
	// GroupFlat, which has a directory inside the root node's. Under
	// GroupFlat below the root a node's bundle is rendered in its node's
	// directory, which the node names, and DirName has no effect on it. When
	// a flat NodeGrouping merges several bundles into the root node they
	// share one directory there: it takes the first merged bundle's DirName,
	// or its Name without one, and a DirName on a later bundle of that
	// directory has no effect.
	//
	// The path the bundle's Flux Kustomization or ArgoCD Application applies
	// follows the directory; that object's name does not (see
	// KustomizationName), and Name stays the bundle's identity. A value set
	// here must be one path segment (ValidateDirectoryName, checked by
	// Validate): not "." or "..", and without "/", "\" or a NUL byte. It is
	// no object name, so it need not be a DNS-1123 subdomain.
	DirName string
	// ParentPath is the hierarchical path to the parent bundle (e.g., "cluster/infrastructure")
	// Empty for root bundles. This avoids circular references while maintaining hierarchy.
	ParentPath string
	// DependsOn lists other bundles this bundle depends on
	DependsOn []*Bundle
	// NamedDependsOn lists names of kustomizations this bundle depends on, by name.
	// Unlike DependsOn, names need not resolve to in-scope Bundle objects. A
	// name is a Kustomization's: for a bundle that sets KustomizationName it
	// is that value, not the bundle's Name.
	// Both fields are merged into Kustomization.Spec.DependsOn.
	NamedDependsOn []string
	// Children holds bundles whose readiness this bundle aggregates. When
	// non-empty, this bundle acts as an umbrella: with Wait unset or false the
	// Flux engine writes a health check per child, so the umbrella is Ready
	// only when all Children are. With Wait true it writes none, and the
	// umbrella's wait covers the children only where its build applies their
	// Flux Kustomization CRs: under the integrated placements, which render
	// them into this bundle's directory, not under FluxSeparate (pkg/stack/fluxcd
	// README, "Umbrella Bundles"). Children bundles must be
	// standalone — they cannot simultaneously be the Bundle of a stack.Node.
	Children []*Bundle
	// Interval controls how often Flux reconciles the bundle. It must parse as
	// a Go duration (e.g. "10m"); empty means the generator's default.
	// Validate rejects any other value.
	Interval string
	// SourceRef specifies the source for the bundle.
	SourceRef *SourceRef
	// Applications holds the Kubernetes objects that belong to the application.
	Applications []*Application
	// Labels are the labels of what is generated for the bundle. The Flux
	// workflow sets them on the bundle's Kustomization and, under
	// FluxIntegratedPerLayout, on the Kustomization of each application of the
	// bundle that has a directory of its own and of each layout below it; the
	// ArgoCD workflow sets them on the bundle's Application. Generate adds
	// them to the objects of the bundle's applications, an object's own value
	// winning. A tree walked from a cluster is not built with Generate: the
	// walker generates each application on its own, so no object in such a
	// tree gets them.
	Labels map[string]string
	// Annotations are the annotations of what is generated for the bundle: on
	// the same Flux Kustomizations as Labels, and added by Generate to the
	// objects of the bundle's applications, an object's own value winning. The
	// ArgoCD workflow does not read them. As for Labels, no object in a tree
	// walked from a cluster gets them.
	Annotations map[string]string
	// Description provides a human-readable description of the bundle.
	Description string
	// Prune enables garbage collection of resources removed from the bundle.
	Prune *bool
	// Wait causes the Kustomization to wait for resources to become ready.
	Wait *bool
	// Timeout is the maximum duration to wait for resources to be ready (e.g. "5m").
	// It must parse as a Go duration; empty leaves the field unset.
	Timeout string
	// RetryInterval is the interval between retry attempts for failed reconciliations (e.g. "2m").
	// It must parse as a Go duration; empty leaves the field unset.
	RetryInterval string
	// Force causes Flux to re-apply resources even if there are no detected changes.
	Force *bool
	// Suspend disables reconciliation when true. Set to false to resume.
	Suspend *bool
	// HealthChecks lists resources whose health is monitored during reconciliation.
	// When specified, the Kustomization waits for these resources to become ready.
	HealthChecks []HealthCheck
	// Patches lists strategic merge or JSON patches to apply to resources after
	// kustomize build. Each patch targets resources matching its selector.
	Patches []Patch
	// PostBuild configures variable substitution performed after kustomize build.
	PostBuild *PostBuild

	// Internal fields for runtime hierarchy navigation (not serialized)
	parent  *Bundle            `yaml:"-"` // Runtime parent reference for efficient traversal
	pathMap map[string]*Bundle `yaml:"-"` // Runtime path lookup map (shared across tree)
}

// SourceRef defines a reference to a Flux source.
// When Kind, Name and Namespace are set, the Kustomization will reference an existing source.
// When URL is also set, the resource generator will create the source CRD.
type SourceRef struct {
	Kind      string
	Name      string
	Namespace string
	// URL is the repository URL (OCI or Git). When set, the resource generator
	// creates the source CRD in addition to referencing it.
	URL string
	// Tag is the tag or semver reference for OCI sources.
	Tag string
	// Branch is the branch reference for Git sources.
	Branch string
}

// HealthCheck defines a resource to be monitored for health during reconciliation.
type HealthCheck struct {
	// APIVersion of the resource (e.g. "apps/v1", "helm.toolkit.fluxcd.io/v2").
	APIVersion string
	// Kind of the resource (e.g. "Deployment", "HelmRelease").
	Kind string
	// Name of the resource.
	Name string
	// Namespace of the resource. When empty, defaults to the Kustomization namespace.
	Namespace string
}

// Patch defines a strategic merge or JSON patch applied to resources after kustomize build.
type Patch struct {
	// Patch is the patch content in strategic merge patch or JSON patch format.
	Patch string
	// Target selects which resources the patch applies to.
	// When nil the patch applies to all resources.
	Target *PatchSelector
}

// PatchSelector selects Kubernetes resources by GVK and metadata filters.
type PatchSelector struct {
	// Group of the target resource (e.g. "apps").
	Group string
	// Version of the target resource (e.g. "v1").
	Version string
	// Kind of the target resource (e.g. "Deployment").
	Kind string
	// Name of the target resource.
	Name string
	// Namespace of the target resource.
	Namespace string
	// LabelSelector is a label selector expression.
	LabelSelector string
	// AnnotationSelector is an annotation selector expression.
	AnnotationSelector string
}

// PostBuild configures variable substitution performed after kustomize build.
type PostBuild struct {
	// Substitute contains inline key-value substitution variables.
	// Values are substituted for ${VAR} occurrences in manifests.
	Substitute map[string]string
	// SubstituteFrom lists ConfigMaps and Secrets whose data is merged into
	// the substitution variables.
	SubstituteFrom []SubstituteRef
}

// SubstituteRef defines a reference to a ConfigMap or Secret used as a
// source of PostBuild substitution variables.
type SubstituteRef struct {
	// Kind is ConfigMap or Secret.
	Kind string
	// Name of the ConfigMap or Secret.
	Name string
	// Optional allows the reference to be absent without causing an error.
	Optional bool
}

// NewBundle constructs a Bundle with the given name, resources and labels.
// It returns an error if validation fails.
func NewBundle(name string, resources []*Application, labels map[string]string) (*Bundle, error) {
	a := &Bundle{Name: name, Applications: resources, Labels: labels}
	if err := a.Validate(); err != nil {
		return nil, err
	}
	return a, nil
}

// UnitName returns the name of the reconciliation unit generated for the
// bundle, the object the delivery engine applies it with: a Flux
// Kustomization, or an ArgoCD Application. It is KustomizationName when that
// is set and Name otherwise. Bundles a layout grouping merges into one
// directory share one unit, named after the first of them; the layout's
// origin index resolves that (layout.OriginIndex.UnitName).
func (a *Bundle) UnitName() string {
	if a.KustomizationName != "" {
		return a.KustomizationName
	}
	return a.Name
}

// Validate performs basic sanity checks on the Bundle. When the bundle has
// umbrella Children, Validate recursively walks the child subtree checking for
// cycles, duplicate names, and DependsOn contradictions.
func (a *Bundle) Validate() error {
	if a == nil {
		return errors.ErrNilBundle
	}
	if a.Name == "" {
		return errors.NewValidationError("name", "", "Bundle", nil)
	}
	for i, r := range a.Applications {
		if r == nil {
			return errors.ResourceValidationError("Bundle", a.Name, "applications", fmt.Sprintf("application at index %d is nil", i), nil)
		}
	}
	visited := make(map[*Bundle]bool)
	if err := a.validateChildren(a.Name, visited); err != nil {
		return err
	}
	return a.validateNames(a.Name, make(map[*Bundle]bool))
}

// validateNames checks the name of the bundle and of every umbrella
// descendant against both rules a bundle name has to meet for every delivery
// engine: it is the name of the object the engine applies the bundle with, a
// DNS-1123 subdomain, and, under layout.GroupByName, of its directory. A
// KustomizationName, when set, names that object instead and is checked as a
// DNS-1123 subdomain too; Name is checked as before, being still the bundle's
// identity and, without a DirName, its directory. A DirName, when set, names
// that directory instead and is checked as a directory name
// (ValidateDirectoryName) only: it is no object's name, so it is no DNS-1123
// subdomain and may hold upper case. It is checked whether or not the layout
// rules give the bundle a directory, as a name is. The
// 63-character limit of a Flux Kustomization name is Flux's and is checked by
// the Flux workflow where it builds the Kustomization, on the name in effect
// (UnitName), not here: a longer name is valid for the ArgoCD workflow.
// path is the bundle's path from the bundle Validate was called on,
// so the error names the bundle and not only its last segment. It runs after
// validateChildren, which has already refused a nil child, a child without a
// name (reported with its parent and index, not with this path) and a cycle;
// seen keeps a bundle reachable twice from being reported twice.
func (a *Bundle) validateNames(path string, seen map[*Bundle]bool) error {
	if seen[a] {
		return nil
	}
	seen[a] = true
	if err := ValidateDirectoryName(a.Name); err != nil {
		return errors.ResourceValidationError("Bundle", path, "name", err.Error(), nil)
	}
	if err := validateBundleName(a.Name); err != nil {
		return errors.ResourceValidationError("Bundle", path, "name", err.Error(), nil)
	}
	if a.KustomizationName != "" {
		if err := validateKustomizationNameField(a.KustomizationName); err != nil {
			return errors.ResourceValidationError("Bundle", path, "kustomizationName", err.Error(), nil)
		}
	}
	if a.DirName != "" {
		if err := ValidateDirectoryName(a.DirName); err != nil {
			return errors.ResourceValidationError("Bundle", path, "dirName", err.Error(), nil)
		}
	}
	for _, c := range a.Children {
		if err := c.validateNames(path+"/"+c.Name, seen); err != nil {
			return err
		}
	}
	return nil
}

// validateChildren performs recursive umbrella-children validation: cycle
// detection, duration syntax, nil/self/duplicate/empty-name checks, and
// DependsOn/Children disjointness. Cycle detection uses a visited pointer set
// shared across the whole recursion.
//
// Two kinds of name are compared. A bundle's own identity is its Name: a
// child without one, a child named like its parent and two children with one
// name are refused on Name. A reference to a bundle's Kustomization is
// compared on UnitName, the name that Kustomization gets: a DependsOn bundle
// against a NamedDependsOn entry, a child against the bundle's DependsOn and
// NamedDependsOn, and a child's NamedDependsOn against its parent. With a
// KustomizationName set, a NamedDependsOn entry equal to the bundle's Name
// therefore names another Kustomization, and one equal to its
// KustomizationName names the bundle's own. A child against the bundle's
// DependsOn is compared on both: a DependsOn bundle is resolved by its Name,
// so one with a child's Name is that child whatever KustomizationName it has.
//
// path is the bundle's path from the bundle Validate was called on. The
// refusal of a nil DependsOn entry names the bundle by it, so an umbrella
// descendant is found by its ancestors; the other refusals here name the
// bundle that holds the fault by its Name, as before.
func (a *Bundle) validateChildren(path string, visited map[*Bundle]bool) error {
	if visited[a] {
		return errors.ResourceValidationError("Bundle", a.Name, "children",
			fmt.Sprintf("umbrella cycle detected at %q", a.Name), nil)
	}
	visited[a] = true
	if err := a.validateDurations(); err != nil {
		return err
	}
	depNames := make(map[string]bool, len(a.DependsOn))
	depBundleNames := make(map[string]bool, len(a.DependsOn))
	for i, dep := range a.DependsOn {
		if dep == nil {
			return errors.ResourceValidationError("Bundle", path, "dependsOn",
				fmt.Sprintf("dependency at index %d is nil", i), nil)
		}
		depNames[dep.UnitName()] = true
		depBundleNames[dep.Name] = true
	}
	// Validate NamedDependsOn: empty names, duplicates, cross-field duplicates.
	namedDepNames := make(map[string]bool, len(a.NamedDependsOn))
	for _, name := range a.NamedDependsOn {
		if name == "" {
			return errors.ResourceValidationError("Bundle", a.Name, "namedDependsOn",
				"named dependency has empty name", nil)
		}
		if namedDepNames[name] {
			return errors.ResourceValidationError("Bundle", a.Name, "namedDependsOn",
				fmt.Sprintf("duplicate named dependency %q", name), nil)
		}
		if depNames[name] {
			return errors.ResourceValidationError("Bundle", a.Name, "namedDependsOn",
				fmt.Sprintf("dependency %q appears in both DependsOn and NamedDependsOn", name), nil)
		}
		namedDepNames[name] = true
	}
	childNames := make(map[string]bool, len(a.Children))
	for i, c := range a.Children {
		if c == nil {
			return errors.ResourceValidationError("Bundle", a.Name, "children",
				fmt.Sprintf("child at index %d is nil", i), nil)
		}
		if c == a {
			return errors.ResourceValidationError("Bundle", a.Name, "children",
				"bundle cannot be its own child", nil)
		}
		if c.Name == "" {
			return errors.ResourceValidationError("Bundle", a.Name, "children",
				fmt.Sprintf("child at index %d has empty name", i), nil)
		}
		if c.Name == a.Name {
			return errors.ResourceValidationError("Bundle", a.Name, "children",
				fmt.Sprintf("child name %q equals parent name", c.Name), nil)
		}
		if childNames[c.Name] {
			return errors.ResourceValidationError("Bundle", a.Name, "children",
				fmt.Sprintf("duplicate child name %q", c.Name), nil)
		}
		childNames[c.Name] = true
		if depBundleNames[c.Name] || depNames[c.UnitName()] {
			return errors.ResourceValidationError("Bundle", a.Name, "children",
				fmt.Sprintf("child %q also appears in dependsOn", c.Name), nil)
		}
		if namedDepNames[c.UnitName()] {
			return errors.ResourceValidationError("Bundle", a.Name, "children",
				fmt.Sprintf("child %q also appears in namedDependsOn", c.Name), nil)
		}
		if slices.Contains(c.DependsOn, a) {
			return errors.ResourceValidationError("Bundle", a.Name, "children",
				fmt.Sprintf("child %q depends on parent %q", c.Name, a.Name), nil)
		}
		if slices.Contains(c.NamedDependsOn, a.UnitName()) {
			return errors.ResourceValidationError("Bundle", a.Name, "children",
				fmt.Sprintf("child %q has parent %q in namedDependsOn", c.Name, a.Name), nil)
		}
		if err := c.validateChildren(path+"/"+c.Name, visited); err != nil {
			return err
		}
	}
	return nil
}

// validateDurations rejects a non-empty Interval, Timeout or RetryInterval
// that does not parse as a Go duration. Empty is the declared "unset" state,
// not a parse failure. It runs from validateChildren, so it covers the bundle
// and every umbrella descendant.
func (a *Bundle) validateDurations() error {
	for _, f := range []struct{ field, value string }{
		{"interval", a.Interval},
		{"timeout", a.Timeout},
		{"retryInterval", a.RetryInterval},
	} {
		if f.value == "" {
			continue
		}
		if _, err := time.ParseDuration(f.value); err != nil {
			return errors.ResourceValidationError("Bundle", a.Name, f.field,
				fmt.Sprintf("%s %q is not a valid duration: %v", f.field, f.value, err), err)
		}
	}
	return nil
}

// IsUmbrella reports whether the bundle acts as an umbrella (has Children
// that contribute their Flux Kustomizations into this bundle's directory).
func (a *Bundle) IsUmbrella() bool {
	return a != nil && len(a.Children) > 0
}

// InitializeUmbrella walks the umbrella Children subtree and sets each child's
// runtime parent pointer (via SetParent) so that code can walk upward from any
// child (GetParent). Idempotent and safe to call multiple times.
func (a *Bundle) InitializeUmbrella() {
	if a == nil {
		return
	}
	for _, c := range a.Children {
		if c == nil {
			continue
		}
		c.SetParent(a)
		c.InitializeUmbrella()
	}
}

func (a *Bundle) Generate() ([]*client.Object, error) {
	var resources []*client.Object
	for _, app := range a.Applications {
		addresources, err := app.Generate()
		if err != nil {
			return nil, err
		}
		resources = append(resources, addresources...)
	}

	// Propagate bundle labels to all generated resources.
	// Application-specific labels take precedence.
	// Direct mutation on *r is correct: client.Object is an interface backed by
	// a pointer, so the underlying concrete object is modified in place.
	if len(a.Labels) > 0 {
		for _, r := range resources {
			labels := (*r).GetLabels()
			if labels == nil {
				labels = make(map[string]string, len(a.Labels))
			}
			for k, v := range a.Labels {
				if _, exists := labels[k]; !exists {
					labels[k] = v
				}
			}
			(*r).SetLabels(labels)
		}
	}

	// Propagate bundle annotations to all generated resources.
	// Application-specific annotations take precedence.
	if len(a.Annotations) > 0 {
		for _, r := range resources {
			annotations := (*r).GetAnnotations()
			if annotations == nil {
				annotations = make(map[string]string, len(a.Annotations))
			}
			for k, v := range a.Annotations {
				if _, exists := annotations[k]; !exists {
					annotations[k] = v
				}
			}
			(*r).SetAnnotations(annotations)
		}
	}

	return resources, nil
}

// GetParent returns the runtime parent reference (may be nil).
func (b *Bundle) GetParent() *Bundle {
	return b.parent
}

// GetParentPath returns the hierarchical path to the parent bundle.
func (b *Bundle) GetParentPath() string {
	return b.ParentPath
}

// SetParent sets the parent bundle and updates the ParentPath accordingly.
// This method maintains both the serializable path and runtime reference.
func (b *Bundle) SetParent(parent *Bundle) {
	b.parent = parent
	if parent == nil {
		b.ParentPath = ""
	} else {
		b.ParentPath = parent.GetPath()
	}
}

// GetPath returns the full hierarchical path of this bundle.
func (b *Bundle) GetPath() string {
	if b.ParentPath == "" {
		return b.Name
	}
	return b.ParentPath + "/" + b.Name
}

// InitializePathMap builds the runtime path lookup map for efficient hierarchy navigation.
// This should be called on the root bundle after the tree structure is complete.
func (b *Bundle) InitializePathMap(allBundles []*Bundle) {
	pathMap := make(map[string]*Bundle)

	// Build path map for all bundles
	for _, bundle := range allBundles {
		if bundle.Name != "" {
			pathMap[bundle.GetPath()] = bundle
		}
	}

	// Set path map and parent references on all bundles
	for _, bundle := range allBundles {
		bundle.pathMap = pathMap
		if bundle.ParentPath != "" {
			if parent, exists := pathMap[bundle.ParentPath]; exists {
				bundle.parent = parent
			}
		}
	}
}
