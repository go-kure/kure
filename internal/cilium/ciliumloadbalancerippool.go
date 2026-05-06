package cilium

import (
	ciliumv2 "github.com/cilium/cilium/pkg/k8s/apis/cilium.io/v2"
	slimv1 "github.com/cilium/cilium/pkg/k8s/slim/k8s/apis/meta/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// CreateCiliumLoadBalancerIPPool returns a new CiliumLoadBalancerIPPool with
// TypeMeta and ObjectMeta set. The resource is cluster-scoped.
func CreateCiliumLoadBalancerIPPool(name string) *ciliumv2.CiliumLoadBalancerIPPool {
	return &ciliumv2.CiliumLoadBalancerIPPool{
		TypeMeta: metav1.TypeMeta{
			Kind:       ciliumv2.PoolKindDefinition,
			APIVersion: ciliumv2.SchemeGroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
	}
}

// SetCiliumLoadBalancerIPPoolSpec sets the full spec on the pool.
func SetCiliumLoadBalancerIPPoolSpec(obj *ciliumv2.CiliumLoadBalancerIPPool, spec ciliumv2.CiliumLoadBalancerIPPoolSpec) {
	obj.Spec = spec
}

// SetCiliumLoadBalancerIPPoolServiceSelector sets the service selector.
// Uses Cilium's slim LabelSelector from github.com/cilium/cilium/pkg/k8s/slim/k8s/apis/meta/v1.
func SetCiliumLoadBalancerIPPoolServiceSelector(obj *ciliumv2.CiliumLoadBalancerIPPool, sel *slimv1.LabelSelector) {
	obj.Spec.ServiceSelector = sel
}

// AddCiliumLoadBalancerIPPoolBlock appends a CIDR block to the pool.
func AddCiliumLoadBalancerIPPoolBlock(obj *ciliumv2.CiliumLoadBalancerIPPool, block ciliumv2.CiliumLoadBalancerIPPoolIPBlock) {
	obj.Spec.Blocks = append(obj.Spec.Blocks, block)
}

// SetCiliumLoadBalancerIPPoolDisabled enables or disables the pool.
func SetCiliumLoadBalancerIPPoolDisabled(obj *ciliumv2.CiliumLoadBalancerIPPool, disabled bool) {
	obj.Spec.Disabled = disabled
}

// SetCiliumLoadBalancerIPPoolAllowFirstLastIPs controls first/last IP
// allocation behaviour.
func SetCiliumLoadBalancerIPPoolAllowFirstLastIPs(obj *ciliumv2.CiliumLoadBalancerIPPool, allow ciliumv2.AllowFirstLastIPType) {
	obj.Spec.AllowFirstLastIPs = allow
}
