// Package patch provides core structures and helpers for loading,
// patching, and managing sets of Kubernetes resources and declarative patch instructions.
//
// This package defines the low-level model used internally by Kure to support
// patch-based generation of Kubernetes manifests without relying on templating
// engines, overlays, or DSLs.
//
// ## Core Concepts
//
// The patch package revolves around the PatchableAppSet, which holds:
//
//   - A set of Kubernetes resources, loaded as *unstructured.Unstructured
//   - A set of declarative patch instructions, using a concise single-line syntax
//   - Patches are always applied to a base resource that must be loaded or
//     provided when the PatchableAppSet is created
//
// Patches are matched to target resources by name or kind.name unless a `target:`
// field is explicitly specified.
//
// ## Patch Format
//
// The supported patch syntax allows field replacement, deletion, or insertion into lists.
// Patches may be written in YAML in one of two formats:
//
//   - Flat map syntax:
//
//     spec.replicas: 3
//     spec.template.spec.containers[0].image: nginx:latest
//
//   - Targeted patch list:
//
//   - target: my-deployment
//     patch:
//     spec.replicas: 5
//     spec.template.metadata.labels.foo: bar
//     metadata.labels[delete=app]: ""
//
// Supported path selectors include:
//
//   - Replace by index: `spec.containers[3]`
//   - Replace by key:   `spec.containers[name=web]`
//   - Insert before:    `spec.containers[-=2]` or `[-=name=web]`
//   - Insert after:     `spec.containers[+=-1]` or `[+=name=web]`
//   - Append:           `spec.containers[-]`
//
// ## Patch Operations
//
// Patch operations are defined via the PatchOp struct:
//
//	type PatchOp struct {
//	    Path       string
//	    Value      interface{}
//	    Op         string // "replace", "delete", "insertbefore", "insertafter", "append"
//	    ParsedPath []PathPart
//	}
//
// Each patch is parsed and validated via NormalizePath() and mapped to a
// list of field operations at runtime.
//
// ## Main Types
//
//	PatchableAppSet — Holds resources and patch operations
//	PatchOp         — Describes a single patch instruction
//	PathPart        — Parsed component of a patch path
//
// ## Main Functions
//
//		LoadPatchFile(r io.Reader) ([]PatchOp, error)
//		LoadResourcesFromMultiYAML(r io.Reader) ([]*unstructured.Unstructured, error)
//	     NewPatchableAppSet(resources []*unstructured.Unstructured, patches []PatchSpec) (*PatchableAppSet, error)
//	     LoadPatchableAppSet(resourceReaders []io.Reader, patchReader io.Reader) (*PatchableAppSet, error)
//
// These helpers allow loading resources and patches from YAML files or from programmatic input.
//
// ## Debugging
//
// Enable verbose patch resolution and loading by setting:
//
//	export KURE_DEBUG=1
//
// ## Future Extensions
//
// - Path validation against Kubernetes schemas
// - Patch conflict resolution strategies
// - Cluster-wide configuration influence
//
// This package is designed for use by higher-level tools like Wharf, Crane, or Kur8.
package patch
