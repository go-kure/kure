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
//
// # PDB Builders
//
// [CreatePodDisruptionBudget] allocates a fully initialised policy/v1
// PodDisruptionBudget.  The remaining PDB helpers follow the Set* convention:
//
//   - [SetPDBMinAvailable], [SetPDBMaxUnavailable] — disruption budget (mutually exclusive)
//   - [SetPDBSelector] — label selector
//   - [SetPDBLabels], [SetPDBAnnotations] — metadata
//
// All setter functions return an error when passed a nil PDB pointer,
// using [github.com/go-kure/kure/pkg/errors.ErrNilPodDisruptionBudget].
//
// # Deployment Builders
//
// [CreateDeployment] allocates a fully initialised apps/v1 Deployment.
// The remaining helpers follow the Add*/Set* convention:
//
//   - [SetDeploymentPodSpec], [SetDeploymentReplicas], [SetDeploymentStrategy]
//   - [AddDeploymentContainer], [AddDeploymentInitContainer], [AddDeploymentVolume]
//   - [SetDeploymentServiceAccountName], [SetDeploymentSecurityContext]
//   - [SetDeploymentAffinity], [SetDeploymentNodeSelector]
//   - [AddDeploymentToleration], [AddDeploymentTopologySpreadConstraints]
//   - [AddDeploymentImagePullSecret]
//   - [SetDeploymentRevisionHistoryLimit], [SetDeploymentMinReadySeconds], [SetDeploymentProgressDeadlineSeconds]
//
// All setter/adder functions return an error when passed a nil Deployment pointer,
// using [github.com/go-kure/kure/pkg/errors.ErrNilDeployment].
//
// # CronJob Builders
//
// [CreateCronJob] allocates a fully initialised batch/v1 CronJob.
// The remaining helpers follow the Add*/Set* convention:
//
//   - [SetCronJobPodSpec], [SetCronJobSchedule], [SetCronJobConcurrencyPolicy]
//   - [AddCronJobContainer], [AddCronJobInitContainer], [AddCronJobVolume]
//   - [SetCronJobServiceAccountName], [SetCronJobSecurityContext]
//   - [SetCronJobAffinity], [SetCronJobNodeSelector]
//   - [AddCronJobToleration], [AddCronJobTopologySpreadConstraint]
//   - [AddCronJobImagePullSecret]
//   - [SetCronJobSuspend], [SetCronJobSuccessfulJobsHistoryLimit], [SetCronJobFailedJobsHistoryLimit]
//   - [SetCronJobStartingDeadlineSeconds], [SetCronJobTimeZone]
//
// All setter/adder functions return an error when passed a nil CronJob pointer,
// using [github.com/go-kure/kure/pkg/errors.ErrNilCronJob].
//
// # Service Builders
//
// [CreateService] allocates a fully initialised v1 Service.
// The remaining helpers follow the Add*/Set* convention:
//
//   - [AddServicePort], [SetServiceSelector], [SetServiceType]
//   - [SetServiceClusterIP], [AddServiceExternalIP], [SetServiceLoadBalancerIP]
//   - [SetServiceExternalTrafficPolicy], [SetServiceSessionAffinity]
//   - [SetServiceLoadBalancerClass], [SetServicePublishNotReadyAddresses]
//   - [AddServiceLabel], [AddServiceAnnotation], [SetServiceLabels], [SetServiceAnnotations]
//   - [AddServiceLoadBalancerSourceRange], [SetServiceLoadBalancerSourceRanges]
//   - [SetServiceIPFamilies], [SetServiceIPFamilyPolicy], [SetServiceInternalTrafficPolicy]
//   - [SetServiceAllocateLoadBalancerNodePorts], [SetServiceExternalName]
//   - [SetServiceHealthCheckNodePort], [SetServiceSessionAffinityConfig]
//
// All setter/adder functions return an error when passed a nil Service pointer,
// using [github.com/go-kure/kure/pkg/errors.ErrNilService].
//
// # Ingress Builders
//
// [CreateIngress] allocates a fully initialised networking/v1 Ingress.
// [CreateIngressRule] and [CreateIngressPath] create building blocks for rules.
// The remaining helpers follow the Add*/Set* convention:
//
//   - [AddIngressRule], [AddIngressRulePath], [AddIngressTLS]
//   - [SetIngressDefaultBackend], [SetIngressClassName]
//
// All setter/adder functions return an error when passed a nil Ingress pointer,
// using [github.com/go-kure/kure/pkg/errors.ErrNilIngress].
package kubernetes
