package metallb

import (
	"errors"

	metallbv1beta1 "go.universe.tf/metallb/api/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// CreateL2Advertisement returns a new L2Advertisement object with the given name, namespace and spec.
func CreateL2Advertisement(name, namespace string, spec metallbv1beta1.L2AdvertisementSpec) *metallbv1beta1.L2Advertisement {
	obj := &metallbv1beta1.L2Advertisement{
		TypeMeta: metav1.TypeMeta{
			Kind:       "L2Advertisement",
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

// AddL2AdvertisementIPAddressPool appends an IPAddressPool reference to the L2Advertisement spec.
func AddL2AdvertisementIPAddressPool(obj *metallbv1beta1.L2Advertisement, pool string) error {
	if obj == nil {
		return errors.New("nil L2Advertisement")
	}
	obj.Spec.IPAddressPools = append(obj.Spec.IPAddressPools, pool)
	return nil
}

// AddL2AdvertisementNodeSelector appends a node selector to the L2Advertisement spec.
func AddL2AdvertisementNodeSelector(obj *metallbv1beta1.L2Advertisement, sel metav1.LabelSelector) error {
	if obj == nil {
		return errors.New("nil L2Advertisement")
	}
	obj.Spec.NodeSelectors = append(obj.Spec.NodeSelectors, sel)
	return nil
}

// AddL2AdvertisementInterface appends a network interface name to the L2Advertisement spec.
func AddL2AdvertisementInterface(obj *metallbv1beta1.L2Advertisement, iface string) error {
	if obj == nil {
		return errors.New("nil L2Advertisement")
	}
	obj.Spec.Interfaces = append(obj.Spec.Interfaces, iface)
	return nil
}
