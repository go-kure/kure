package fluxcd

import (
	fluxv1 "github.com/controlplaneio-fluxcd/flux-operator/api/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// CreateFluxInstance returns a new FluxInstance object.
func CreateFluxInstance(name, namespace string, spec fluxv1.FluxInstanceSpec) *fluxv1.FluxInstance {
	obj := &fluxv1.FluxInstance{
		TypeMeta: metav1.TypeMeta{
			Kind:       fluxv1.FluxInstanceKind,
			APIVersion: fluxv1.GroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: spec,
	}
	return obj
}

// AddFluxInstanceComponent appends a component to the FluxInstance spec.
func AddFluxInstanceComponent(obj *fluxv1.FluxInstance, c fluxv1.Component) {
	obj.Spec.Components = append(obj.Spec.Components, c)
}

// SetFluxInstanceDistribution sets the distribution of the FluxInstance.
func SetFluxInstanceDistribution(obj *fluxv1.FluxInstance, dist fluxv1.Distribution) {
	obj.Spec.Distribution = dist
}

// SetFluxInstanceCommonMetadata sets the common metadata.
func SetFluxInstanceCommonMetadata(obj *fluxv1.FluxInstance, cm *fluxv1.CommonMetadata) {
	obj.Spec.CommonMetadata = cm
}

// SetFluxInstanceCluster sets the cluster information.
func SetFluxInstanceCluster(obj *fluxv1.FluxInstance, cluster *fluxv1.Cluster) {
	obj.Spec.Cluster = cluster
}

// SetFluxInstanceSharding sets the sharding specification.
func SetFluxInstanceSharding(obj *fluxv1.FluxInstance, shard *fluxv1.Sharding) {
	obj.Spec.Sharding = shard
}

// SetFluxInstanceStorage sets the storage specification.
func SetFluxInstanceStorage(obj *fluxv1.FluxInstance, st *fluxv1.Storage) {
	obj.Spec.Storage = st
}

// SetFluxInstanceKustomize sets the kustomize specification.
func SetFluxInstanceKustomize(obj *fluxv1.FluxInstance, k *fluxv1.Kustomize) {
	obj.Spec.Kustomize = k
}

// SetFluxInstanceWait sets the wait flag.
func SetFluxInstanceWait(obj *fluxv1.FluxInstance, wait bool) {
	obj.Spec.Wait = &wait
}

// SetFluxInstanceMigrateResources sets the migrateResources flag.
func SetFluxInstanceMigrateResources(obj *fluxv1.FluxInstance, m bool) {
	obj.Spec.MigrateResources = &m
}

// SetFluxInstanceSync sets the sync configuration.
func SetFluxInstanceSync(obj *fluxv1.FluxInstance, sync *fluxv1.Sync) {
	obj.Spec.Sync = sync
}
