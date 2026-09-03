// Package kubernetes provides the scheme, the generic constructor and the
// admissible sugar helpers for building Kubernetes objects under the builder
// contract (ADR-038). The contract text is the package README; this comment
// is the map.
//
// # Construction
//
// The upstream Go struct is the construction API: obtain an object, then
// write its fields directly (obj.Spec.Replicas = ptr.To[int32](3)). kure does
// not wrap plain field access.
//
// [Create] allocates any type registered in the scheme and sets exactly its
// TypeMeta (from the scheme), metadata.name and metadata.namespace. Nothing
// else: no labels, no annotations, no selector, no defaults. An unregistered
// type panics. Cluster-scoped kinds take "" as namespace.
//
//	d := kubernetes.Create[appsv1.Deployment]("web", "default")
//
// The per-kind wrappers (CreateDeployment, CreateNamespace, ...) in
// zz_generated_create.go are generated from the scheme by
// pkg/kubernetes/internal/gen; cluster-scoped kinds take only a name. They
// are one-line calls to [Create] and are never hand-written.
//
// # Metadata
//
// [SetLabels], [AddLabel], [SetAnnotations] and [AddAnnotation] work on any
// metav1.Object and are the only metadata helpers the contract keeps.
//
// # Sugar
//
// An exported Set*/Add* function is admitted by class: slice append or map
// insert, pointer-typed field assignment (which covers nil-init of an
// intermediate), or an upstream struct literal of two or more fields or with
// a nested literal. Every admitted operation carries a value the caller
// supplied, happens in the helper's own body rather than in a closure it
// never calls, and, when it goes through a local, that local came from the
// field it is written back to. A body that assigns nil to a field, writes a value the caller did
// not pass, or takes its object by value, and a helper that returns anything,
// are inadmissible whatever else they do. A test classifies every helper
// with go/ast and fails on anything outside those classes that is not listed
// in testdata/admission_exclusions.txt; that list holds the legacy helpers
// (bare forwarders, error-returning and validating setters) until the prune
// work item of the builder-contract epic deletes them or rewrites the
// class-shaped error-returning ones as void helpers, and it only ever
// shrinks. The target for every helper that survives: panic on a nil receiver,
// take exactly the value it writes, never default or validate.
//
// # GVK Utilities
//
// [GetGroupVersionKind] resolves the GroupVersionKind of any runtime.Object
// registered with the package scheme. [IsGVKAllowed] checks a GVK against a
// user-defined allow list.
//
// # Scheme Registration
//
// [RegisterSchemes] initialises the shared runtime.Scheme covering the
// Kubernetes built-in types and every CRD family kure builds for. The scheme
// is registered lazily on first use and is safe for concurrent access.
//
// # PSA Security Context Helpers
//
// [RestrictedPodSecurityContext], [BaselinePodSecurityContext] and
// [PrivilegedPodSecurityContext] return PodSecurityContext values for the
// corresponding Pod Security Standards levels; [RestrictedSecurityContext],
// [BaselineSecurityContext] and [PrivilegedSecurityContext] do the same for
// containers. [PodSecurityContextForLevel] and [SecurityContextForLevel]
// select by [PSALevel]. [ValidateContainerPSA] and [ValidatePodSpecPSA] are
// explicit, opt-in checks; nothing in this package calls them for you.
package kubernetes
