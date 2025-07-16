package fluxcd

import (
	kustv1 "github.com/fluxcd/kustomize-controller/api/v1"
	"github.com/fluxcd/pkg/apis/kustomize"
	metaapi "github.com/fluxcd/pkg/apis/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func CreateKustomization(name string, namespace string, spec kustv1.KustomizationSpec) *kustv1.Kustomization {
	obj := &kustv1.Kustomization{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Kustomization",
			APIVersion: kustv1.GroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: spec,
	}
	return obj
}

// SetKustomizationInterval updates the reconciliation interval.
func SetKustomizationInterval(k *kustv1.Kustomization, interval metav1.Duration) {
	k.Spec.Interval = interval
}

// SetKustomizationRetryInterval sets the retry interval.
func SetKustomizationRetryInterval(k *kustv1.Kustomization, interval metav1.Duration) {
	k.Spec.RetryInterval = &interval
}

// SetKustomizationPath sets the path field.
func SetKustomizationPath(k *kustv1.Kustomization, path string) {
	k.Spec.Path = path
}

// SetKustomizationKubeConfig specifies a kubeconfig reference.
func SetKustomizationKubeConfig(k *kustv1.Kustomization, ref *metaapi.KubeConfigReference) {
	k.Spec.KubeConfig = ref
}

// SetKustomizationSourceRef sets the source reference.
func SetKustomizationSourceRef(k *kustv1.Kustomization, ref kustv1.CrossNamespaceSourceReference) {
	k.Spec.SourceRef = ref
}

// SetKustomizationPrune sets the prune option.
func SetKustomizationPrune(k *kustv1.Kustomization, prune bool) {
	k.Spec.Prune = prune
}

// SetKustomizationDeletionPolicy sets the deletion policy.
func SetKustomizationDeletionPolicy(k *kustv1.Kustomization, policy string) {
	k.Spec.DeletionPolicy = policy
}

// AddKustomizationHealthCheck appends a health check reference.
func AddKustomizationHealthCheck(k *kustv1.Kustomization, ref metaapi.NamespacedObjectKindReference) {
	k.Spec.HealthChecks = append(k.Spec.HealthChecks, ref)
}

// AddKustomizationComponent adds a component path.
func AddKustomizationComponent(k *kustv1.Kustomization, component string) {
	k.Spec.Components = append(k.Spec.Components, component)
}

// AddKustomizationDependsOn appends a dependency reference.
func AddKustomizationDependsOn(k *kustv1.Kustomization, ref metaapi.NamespacedObjectReference) {
	k.Spec.DependsOn = append(k.Spec.DependsOn, ref)
}

// SetKustomizationServiceAccountName sets the service account name.
func SetKustomizationServiceAccountName(k *kustv1.Kustomization, name string) {
	k.Spec.ServiceAccountName = name
}

// SetKustomizationSuspend sets the suspend flag.
func SetKustomizationSuspend(k *kustv1.Kustomization, suspend bool) {
	k.Spec.Suspend = suspend
}

// SetKustomizationTargetNamespace overrides the target namespace.
func SetKustomizationTargetNamespace(k *kustv1.Kustomization, namespace string) {
	k.Spec.TargetNamespace = namespace
}

// SetKustomizationTimeout sets the timeout duration.
func SetKustomizationTimeout(k *kustv1.Kustomization, timeout metav1.Duration) {
	k.Spec.Timeout = &timeout
}

// SetKustomizationForce sets the force flag.
func SetKustomizationForce(k *kustv1.Kustomization, force bool) {
	k.Spec.Force = force
}

// SetKustomizationWait sets the wait flag.
func SetKustomizationWait(k *kustv1.Kustomization, wait bool) {
	k.Spec.Wait = wait
}

// AddKustomizationImage appends an image transformation.
func AddKustomizationImage(k *kustv1.Kustomization, img kustomize.Image) {
	k.Spec.Images = append(k.Spec.Images, img)
}

// AddKustomizationPatch appends a strategic merge or JSON patch.
func AddKustomizationPatch(k *kustv1.Kustomization, patch kustomize.Patch) {
	k.Spec.Patches = append(k.Spec.Patches, patch)
}

// SetKustomizationNamePrefix sets the name prefix.
func SetKustomizationNamePrefix(k *kustv1.Kustomization, prefix string) {
	k.Spec.NamePrefix = prefix
}

// SetKustomizationNameSuffix sets the name suffix.
func SetKustomizationNameSuffix(k *kustv1.Kustomization, suffix string) {
	k.Spec.NameSuffix = suffix
}

// SetKustomizationCommonMetadata sets common labels and annotations.
func SetKustomizationCommonMetadata(k *kustv1.Kustomization, cm *kustv1.CommonMetadata) {
	k.Spec.CommonMetadata = cm
}

// SetKustomizationDecryption sets the decryption configuration.
func SetKustomizationDecryption(k *kustv1.Kustomization, d *kustv1.Decryption) {
	k.Spec.Decryption = d
}

// SetKustomizationPostBuild sets the post build configuration.
func SetKustomizationPostBuild(k *kustv1.Kustomization, pb *kustv1.PostBuild) {
	k.Spec.PostBuild = pb
}

// CreatePostBuild returns a PostBuild with initialized fields.
func CreatePostBuild() *kustv1.PostBuild {
	return &kustv1.PostBuild{Substitute: map[string]string{}, SubstituteFrom: []kustv1.SubstituteReference{}}
}

// AddPostBuildSubstitute adds a substitute variable.
func AddPostBuildSubstitute(pb *kustv1.PostBuild, key, value string) {
	if pb.Substitute == nil {
		pb.Substitute = make(map[string]string)
	}
	pb.Substitute[key] = value
}

// AddPostBuildSubstituteFrom adds a substitution source reference.
func AddPostBuildSubstituteFrom(pb *kustv1.PostBuild, ref kustv1.SubstituteReference) {
	pb.SubstituteFrom = append(pb.SubstituteFrom, ref)
}

// CreateSubstituteReference constructs a SubstituteReference.
func CreateSubstituteReference(kind, name string, optional bool) kustv1.SubstituteReference {
	return kustv1.SubstituteReference{Kind: kind, Name: name, Optional: optional}
}

// CreateDecryption constructs a Decryption specification.
func CreateDecryption(provider string, secret *metaapi.LocalObjectReference) *kustv1.Decryption {
	return &kustv1.Decryption{Provider: provider, SecretRef: secret}
}

// CreateCommonMetadata constructs CommonMetadata with initialized maps.
func CreateCommonMetadata() *kustv1.CommonMetadata {
	return &kustv1.CommonMetadata{Annotations: map[string]string{}, Labels: map[string]string{}}
}

// AddCommonMetadataLabel adds a label to CommonMetadata.
func AddCommonMetadataLabel(cm *kustv1.CommonMetadata, key, value string) {
	if cm.Labels == nil {
		cm.Labels = make(map[string]string)
	}
	cm.Labels[key] = value
}

// AddCommonMetadataAnnotation adds an annotation to CommonMetadata.
func AddCommonMetadataAnnotation(cm *kustv1.CommonMetadata, key, value string) {
	if cm.Annotations == nil {
		cm.Annotations = make(map[string]string)
	}
	cm.Annotations[key] = value
}
