package fluxcd

import (
	"errors"

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
func AddFluxInstanceComponent(obj *fluxv1.FluxInstance, c fluxv1.Component) error {
	if obj == nil {
		return errors.New("nil FluxInstance")
	}
	obj.Spec.Components = append(obj.Spec.Components, c)
	return nil
}

// SetFluxInstanceDistribution sets the distribution of the FluxInstance.
func SetFluxInstanceDistribution(obj *fluxv1.FluxInstance, dist fluxv1.Distribution) error {
	if obj == nil {
		return errors.New("nil FluxInstance")
	}
	obj.Spec.Distribution = dist
	return nil
}

// SetFluxInstanceCommonMetadata sets the common metadata.
func SetFluxInstanceCommonMetadata(obj *fluxv1.FluxInstance, cm *fluxv1.CommonMetadata) error {
	if obj == nil {
		return errors.New("nil FluxInstance")
	}
	obj.Spec.CommonMetadata = cm
	return nil
}

// SetFluxInstanceCluster sets the cluster information.
func SetFluxInstanceCluster(obj *fluxv1.FluxInstance, cluster *fluxv1.Cluster) error {
	if obj == nil {
		return errors.New("nil FluxInstance")
	}
	obj.Spec.Cluster = cluster
	return nil
}

// SetFluxInstanceSharding sets the sharding specification.
func SetFluxInstanceSharding(obj *fluxv1.FluxInstance, shard *fluxv1.Sharding) error {
	if obj == nil {
		return errors.New("nil FluxInstance")
	}
	obj.Spec.Sharding = shard
	return nil
}

// SetFluxInstanceStorage sets the storage specification.
func SetFluxInstanceStorage(obj *fluxv1.FluxInstance, st *fluxv1.Storage) error {
	if obj == nil {
		return errors.New("nil FluxInstance")
	}
	obj.Spec.Storage = st
	return nil
}

// SetFluxInstanceKustomize sets the kustomize specification.
func SetFluxInstanceKustomize(obj *fluxv1.FluxInstance, k *fluxv1.Kustomize) error {
	if obj == nil {
		return errors.New("nil FluxInstance")
	}
	obj.Spec.Kustomize = k
	return nil
}

// SetFluxInstanceWait sets the wait flag.
func SetFluxInstanceWait(obj *fluxv1.FluxInstance, wait bool) error {
	if obj == nil {
		return errors.New("nil FluxInstance")
	}
	obj.Spec.Wait = &wait
	return nil
}

// SetFluxInstanceMigrateResources sets the migrateResources flag.
func SetFluxInstanceMigrateResources(obj *fluxv1.FluxInstance, m bool) error {
	if obj == nil {
		return errors.New("nil FluxInstance")
	}
	obj.Spec.MigrateResources = &m
	return nil
}

// SetFluxInstanceSync sets the sync configuration.
func SetFluxInstanceSync(obj *fluxv1.FluxInstance, sync *fluxv1.Sync) error {
	if obj == nil {
		return errors.New("nil FluxInstance")
	}
	obj.Spec.Sync = sync
	return nil
}
