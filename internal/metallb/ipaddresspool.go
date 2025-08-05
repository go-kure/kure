package metallb

import (
	"github.com/go-kure/kure/internal/validation"

	metallbv1beta1 "go.universe.tf/metallb/api/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// CreateIPAddressPool returns a new IPAddressPool object with the given name, namespace and spec.
func CreateIPAddressPool(name, namespace string, spec metallbv1beta1.IPAddressPoolSpec) *metallbv1beta1.IPAddressPool {
	obj := &metallbv1beta1.IPAddressPool{
		TypeMeta: metav1.TypeMeta{
			Kind:       "IPAddressPool",
			APIVersion: metallbv1beta1.GroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: spec,
	}
	return obj
}

// AddIPAddressPoolAddress adds an address range to the IPAddressPool spec.
func AddIPAddressPoolAddress(obj *metallbv1beta1.IPAddressPool, addr string) error {
	validator := validation.NewValidator()
	if err := validator.ValidateIPAddressPool(obj); err != nil {
		return err
	}
	obj.Spec.Addresses = append(obj.Spec.Addresses, addr)
	return nil
}

// SetIPAddressPoolAutoAssign sets the autoAssign flag on the IPAddressPool spec.
func SetIPAddressPoolAutoAssign(obj *metallbv1beta1.IPAddressPool, auto bool) error {
	validator := validation.NewValidator()
	if err := validator.ValidateIPAddressPool(obj); err != nil {
		return err
	}
	obj.Spec.AutoAssign = &auto
	return nil
}

// SetIPAddressPoolAvoidBuggyIPs sets the avoidBuggyIPs flag on the IPAddressPool spec.
func SetIPAddressPoolAvoidBuggyIPs(obj *metallbv1beta1.IPAddressPool, avoid bool) error {
	validator := validation.NewValidator()
	if err := validator.ValidateIPAddressPool(obj); err != nil {
		return err
	}
	obj.Spec.AvoidBuggyIPs = avoid
	return nil
}

// SetIPAddressPoolAllocateTo sets the allocation policy on the IPAddressPool spec.
func SetIPAddressPoolAllocateTo(obj *metallbv1beta1.IPAddressPool, alloc *metallbv1beta1.ServiceAllocation) error {
	validator := validation.NewValidator()
	if err := validator.ValidateIPAddressPool(obj); err != nil {
		return err
	}
	obj.Spec.AllocateTo = alloc
	return nil
}
