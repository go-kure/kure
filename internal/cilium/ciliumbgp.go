package cilium

import (
	ciliumv2 "github.com/cilium/cilium/pkg/k8s/apis/cilium.io/v2"
	slimv1 "github.com/cilium/cilium/pkg/k8s/slim/k8s/apis/meta/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// CreateCiliumBGPClusterConfig returns a new CiliumBGPClusterConfig with
// TypeMeta and ObjectMeta set. The resource is cluster-scoped.
func CreateCiliumBGPClusterConfig(name string) *ciliumv2.CiliumBGPClusterConfig {
	return &ciliumv2.CiliumBGPClusterConfig{
		TypeMeta: metav1.TypeMeta{
			Kind:       ciliumv2.BGPCCKindDefinition,
			APIVersion: ciliumv2.SchemeGroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
	}
}

// SetCiliumBGPClusterConfigSpec sets the full spec on the config.
func SetCiliumBGPClusterConfigSpec(obj *ciliumv2.CiliumBGPClusterConfig, spec ciliumv2.CiliumBGPClusterConfigSpec) {
	obj.Spec = spec
}

// SetCiliumBGPClusterConfigNodeSelector sets the node selector.
// Uses Cilium's slim LabelSelector from github.com/cilium/cilium/pkg/k8s/slim/k8s/apis/meta/v1.
func SetCiliumBGPClusterConfigNodeSelector(obj *ciliumv2.CiliumBGPClusterConfig, sel *slimv1.LabelSelector) {
	obj.Spec.NodeSelector = sel
}

// AddCiliumBGPClusterConfigBGPInstance appends a BGP instance.
func AddCiliumBGPClusterConfigBGPInstance(obj *ciliumv2.CiliumBGPClusterConfig, instance ciliumv2.CiliumBGPInstance) {
	obj.Spec.BGPInstances = append(obj.Spec.BGPInstances, instance)
}

// CreateCiliumBGPPeerConfig returns a new CiliumBGPPeerConfig with TypeMeta
// and ObjectMeta set. The resource is cluster-scoped.
func CreateCiliumBGPPeerConfig(name string) *ciliumv2.CiliumBGPPeerConfig {
	return &ciliumv2.CiliumBGPPeerConfig{
		TypeMeta: metav1.TypeMeta{
			Kind:       ciliumv2.BGPPCKindDefinition,
			APIVersion: ciliumv2.SchemeGroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
	}
}

// SetCiliumBGPPeerConfigSpec sets the full spec on the config.
func SetCiliumBGPPeerConfigSpec(obj *ciliumv2.CiliumBGPPeerConfig, spec ciliumv2.CiliumBGPPeerConfigSpec) {
	obj.Spec = spec
}

// SetCiliumBGPPeerConfigTransport sets the transport configuration.
func SetCiliumBGPPeerConfigTransport(obj *ciliumv2.CiliumBGPPeerConfig, transport *ciliumv2.CiliumBGPTransport) {
	obj.Spec.Transport = transport
}

// SetCiliumBGPPeerConfigTimers sets the BGP timer configuration.
func SetCiliumBGPPeerConfigTimers(obj *ciliumv2.CiliumBGPPeerConfig, timers *ciliumv2.CiliumBGPTimers) {
	obj.Spec.Timers = timers
}

// SetCiliumBGPPeerConfigAuthSecretRef sets the BGP authentication secret name.
func SetCiliumBGPPeerConfigAuthSecretRef(obj *ciliumv2.CiliumBGPPeerConfig, ref string) {
	obj.Spec.AuthSecretRef = &ref
}

// SetCiliumBGPPeerConfigEBGPMultihop sets the eBGP multihop TTL.
func SetCiliumBGPPeerConfigEBGPMultihop(obj *ciliumv2.CiliumBGPPeerConfig, ttl int32) {
	obj.Spec.EBGPMultihop = &ttl
}

// SetCiliumBGPPeerConfigGracefulRestart sets the graceful restart configuration.
func SetCiliumBGPPeerConfigGracefulRestart(obj *ciliumv2.CiliumBGPPeerConfig, gr *ciliumv2.CiliumBGPNeighborGracefulRestart) {
	obj.Spec.GracefulRestart = gr
}

// AddCiliumBGPPeerConfigFamily appends an address family with advertisements.
func AddCiliumBGPPeerConfigFamily(obj *ciliumv2.CiliumBGPPeerConfig, family ciliumv2.CiliumBGPFamilyWithAdverts) {
	obj.Spec.Families = append(obj.Spec.Families, family)
}

// CreateCiliumBGPAdvertisement returns a new CiliumBGPAdvertisement with
// TypeMeta and ObjectMeta set. The resource is cluster-scoped.
func CreateCiliumBGPAdvertisement(name string) *ciliumv2.CiliumBGPAdvertisement {
	return &ciliumv2.CiliumBGPAdvertisement{
		TypeMeta: metav1.TypeMeta{
			Kind:       ciliumv2.BGPAKindDefinition,
			APIVersion: ciliumv2.SchemeGroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
	}
}

// SetCiliumBGPAdvertisementSpec sets the full spec on the advertisement.
func SetCiliumBGPAdvertisementSpec(obj *ciliumv2.CiliumBGPAdvertisement, spec ciliumv2.CiliumBGPAdvertisementSpec) {
	obj.Spec = spec
}

// AddCiliumBGPAdvertisementEntry appends a BGP advertisement entry.
func AddCiliumBGPAdvertisementEntry(obj *ciliumv2.CiliumBGPAdvertisement, advert ciliumv2.BGPAdvertisement) {
	obj.Spec.Advertisements = append(obj.Spec.Advertisements, advert)
}

// CreateCiliumBGPNodeConfig returns a new CiliumBGPNodeConfig with TypeMeta
// and ObjectMeta set. The resource is cluster-scoped.
func CreateCiliumBGPNodeConfig(name string) *ciliumv2.CiliumBGPNodeConfig {
	return &ciliumv2.CiliumBGPNodeConfig{
		TypeMeta: metav1.TypeMeta{
			Kind:       ciliumv2.BGPNCKindDefinition,
			APIVersion: ciliumv2.SchemeGroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
	}
}

// SetCiliumBGPNodeConfigSpec sets the full spec on the node config.
func SetCiliumBGPNodeConfigSpec(obj *ciliumv2.CiliumBGPNodeConfig, spec ciliumv2.CiliumBGPNodeSpec) {
	obj.Spec = spec
}

// AddCiliumBGPNodeConfigBGPInstance appends a BGP node instance.
func AddCiliumBGPNodeConfigBGPInstance(obj *ciliumv2.CiliumBGPNodeConfig, instance ciliumv2.CiliumBGPNodeInstance) {
	obj.Spec.BGPInstances = append(obj.Spec.BGPInstances, instance)
}

// CreateCiliumBGPNodeConfigOverride returns a new CiliumBGPNodeConfigOverride
// with TypeMeta and ObjectMeta set. The resource is cluster-scoped.
func CreateCiliumBGPNodeConfigOverride(name string) *ciliumv2.CiliumBGPNodeConfigOverride {
	return &ciliumv2.CiliumBGPNodeConfigOverride{
		TypeMeta: metav1.TypeMeta{
			Kind:       ciliumv2.BGPNCOKindDefinition,
			APIVersion: ciliumv2.SchemeGroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
	}
}

// SetCiliumBGPNodeConfigOverrideSpec sets the full spec on the override.
func SetCiliumBGPNodeConfigOverrideSpec(obj *ciliumv2.CiliumBGPNodeConfigOverride, spec ciliumv2.CiliumBGPNodeConfigOverrideSpec) {
	obj.Spec = spec
}

// AddCiliumBGPNodeConfigOverrideBGPInstance appends a BGP instance override.
func AddCiliumBGPNodeConfigOverrideBGPInstance(obj *ciliumv2.CiliumBGPNodeConfigOverride, instance ciliumv2.CiliumBGPNodeConfigInstanceOverride) {
	obj.Spec.BGPInstances = append(obj.Spec.BGPInstances, instance)
}
