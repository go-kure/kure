package fluxcd

import (
	helmv2 "github.com/fluxcd/helm-controller/api/v2"
	"github.com/fluxcd/pkg/apis/kustomize"
	"github.com/fluxcd/pkg/apis/meta"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func CreateHelmRelease(name string, namespace string, spec helmv2.HelmReleaseSpec) *helmv2.HelmRelease {
	obj := &helmv2.HelmRelease{
		TypeMeta: metav1.TypeMeta{
			Kind:       "HelmRelease",
			APIVersion: helmv2.GroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: spec,
	}
	return obj
}

// AddHelmReleaseLabel adds a label to the HelmRelease metadata.
func AddHelmReleaseLabel(obj *helmv2.HelmRelease, key, value string) {
	if obj.Labels == nil {
		obj.Labels = map[string]string{}
	}
	obj.Labels[key] = value
}

// AddHelmReleaseAnnotation adds an annotation to the HelmRelease metadata.
func AddHelmReleaseAnnotation(obj *helmv2.HelmRelease, key, value string) {
	if obj.Annotations == nil {
		obj.Annotations = map[string]string{}
	}
	obj.Annotations[key] = value
}

// SetHelmReleaseChart sets the inline HelmChartTemplate.
func SetHelmReleaseChart(obj *helmv2.HelmRelease, chart *helmv2.HelmChartTemplate) {
	obj.Spec.Chart = chart
}

// SetHelmReleaseChartRef sets the cross namespace chart reference.
func SetHelmReleaseChartRef(obj *helmv2.HelmRelease, ref *helmv2.CrossNamespaceSourceReference) {
	obj.Spec.ChartRef = ref
}

// SetHelmReleaseInterval sets the reconcile interval.
func SetHelmReleaseInterval(obj *helmv2.HelmRelease, interval metav1.Duration) {
	obj.Spec.Interval = interval
}

// SetHelmReleaseKubeConfig sets the KubeConfig reference.
func SetHelmReleaseKubeConfig(obj *helmv2.HelmRelease, cfg *meta.KubeConfigReference) {
	obj.Spec.KubeConfig = cfg
}

// SetHelmReleaseSuspend configures the suspend flag.
func SetHelmReleaseSuspend(obj *helmv2.HelmRelease, suspend bool) {
	obj.Spec.Suspend = suspend
}

// SetHelmReleaseReleaseName sets the Helm release name.
func SetHelmReleaseReleaseName(obj *helmv2.HelmRelease, name string) {
	obj.Spec.ReleaseName = name
}

// SetHelmReleaseTargetNamespace sets the target namespace of the release.
func SetHelmReleaseTargetNamespace(obj *helmv2.HelmRelease, ns string) {
	obj.Spec.TargetNamespace = ns
}

// SetHelmReleaseStorageNamespace sets the storage namespace of the release.
func SetHelmReleaseStorageNamespace(obj *helmv2.HelmRelease, ns string) {
	obj.Spec.StorageNamespace = ns
}

// AddHelmReleaseDependsOn appends a dependency to the HelmRelease.
func AddHelmReleaseDependsOn(obj *helmv2.HelmRelease, ref meta.NamespacedObjectReference) {
	obj.Spec.DependsOn = append(obj.Spec.DependsOn, ref)
}

// SetHelmReleaseTimeout sets the timeout for the Helm actions.
func SetHelmReleaseTimeout(obj *helmv2.HelmRelease, timeout metav1.Duration) {
	obj.Spec.Timeout = &timeout
}

// SetHelmReleaseMaxHistory sets the maximum history to retain.
func SetHelmReleaseMaxHistory(obj *helmv2.HelmRelease, h int) {
	obj.Spec.MaxHistory = &h
}

// SetHelmReleaseServiceAccountName sets the service account name.
func SetHelmReleaseServiceAccountName(obj *helmv2.HelmRelease, name string) {
	obj.Spec.ServiceAccountName = name
}

// SetHelmReleasePersistentClient sets the persistent client flag.
func SetHelmReleasePersistentClient(obj *helmv2.HelmRelease, b bool) {
	obj.Spec.PersistentClient = &b
}

// SetHelmReleaseDriftDetection sets the drift detection configuration.
func SetHelmReleaseDriftDetection(obj *helmv2.HelmRelease, dd *helmv2.DriftDetection) {
	obj.Spec.DriftDetection = dd
}

// SetHelmReleaseInstall sets the install configuration.
func SetHelmReleaseInstall(obj *helmv2.HelmRelease, install *helmv2.Install) {
	obj.Spec.Install = install
}

// SetHelmReleaseUpgrade sets the upgrade configuration.
func SetHelmReleaseUpgrade(obj *helmv2.HelmRelease, upgrade *helmv2.Upgrade) {
	obj.Spec.Upgrade = upgrade
}

// SetHelmReleaseRollback sets the rollback configuration.
func SetHelmReleaseRollback(obj *helmv2.HelmRelease, rollback *helmv2.Rollback) {
	obj.Spec.Rollback = rollback
}

// SetHelmReleaseUninstall sets the uninstall configuration.
func SetHelmReleaseUninstall(obj *helmv2.HelmRelease, uninstall *helmv2.Uninstall) {
	obj.Spec.Uninstall = uninstall
}

// SetHelmReleaseTest sets the test configuration.
func SetHelmReleaseTest(obj *helmv2.HelmRelease, test *helmv2.Test) {
	obj.Spec.Test = test
}

// AddHelmReleaseValuesFrom appends a valuesFrom reference.
func AddHelmReleaseValuesFrom(obj *helmv2.HelmRelease, ref helmv2.ValuesReference) {
	obj.Spec.ValuesFrom = append(obj.Spec.ValuesFrom, ref)
}

// SetHelmReleaseValues sets the values for the release.
func SetHelmReleaseValues(obj *helmv2.HelmRelease, values *apiextensionsv1.JSON) {
	obj.Spec.Values = values
}

// AddHelmReleasePostRenderer appends a post renderer.
func AddHelmReleasePostRenderer(obj *helmv2.HelmRelease, pr helmv2.PostRenderer) {
	obj.Spec.PostRenderers = append(obj.Spec.PostRenderers, pr)
}

// CreatePostRendererKustomize returns a Kustomize post-renderer with initialized slices.
func CreatePostRendererKustomize() *helmv2.Kustomize {
	return &helmv2.Kustomize{}
}

// AddPostRendererKustomizePatch appends a strategic merge or JSON patch.
func AddPostRendererKustomizePatch(k *helmv2.Kustomize, patch kustomize.Patch) {
	k.Patches = append(k.Patches, patch)
}

// AddPostRendererKustomizeImage appends an image transformation.
func AddPostRendererKustomizeImage(k *helmv2.Kustomize, img kustomize.Image) {
	k.Images = append(k.Images, img)
}
