// Package kubernetes provides helper functions for working with core
// Kubernetes resource types.
//
// # GVK Utilities
//
// [GetGroupVersionKind] resolves the GroupVersionKind of any runtime.Object
// registered with the package scheme.  [IsGVKAllowed] checks a GVK against a
// user-defined allow list.
//
// # Scheme Registration
//
// [RegisterSchemes] initialises a shared runtime.Scheme that covers Kubernetes
// built-in types, FluxCD CRDs, cert-manager, MetalLB, and External Secrets.
// The scheme is registered lazily on first use and is safe for concurrent
// access.
//
// # HPA Builders
//
// [CreateHorizontalPodAutoscaler] allocates a fully initialised
// autoscaling/v2 HorizontalPodAutoscaler.  The remaining HPA helpers follow
// the Add*/Set* convention used throughout Kure:
//
//   - [SetHPAScaleTargetRef] — target Deployment / StatefulSet
//   - [SetHPAMinMaxReplicas] — replica bounds
//   - [AddHPACPUMetric], [AddHPAMemoryMetric], [AddHPACustomMetric] — scaling metrics
//   - [SetHPABehavior] — scale-up / scale-down policies
//   - [SetHPALabels], [SetHPAAnnotations] — metadata
//
// All setter/adder functions return an error when passed a nil HPA pointer,
// using [github.com/go-kure/kure/pkg/errors.ErrNilHorizontalPodAutoscaler].
package kubernetes
