package metallb

import (
	metallbv1beta1 "go.universe.tf/metallb/api/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// CreateBGPAdvertisement returns a new BGPAdvertisement object with the given name, namespace and spec.
func CreateBGPAdvertisement(name, namespace string, spec metallbv1beta1.BGPAdvertisementSpec) *metallbv1beta1.BGPAdvertisement {
	obj := &metallbv1beta1.BGPAdvertisement{
		TypeMeta: metav1.TypeMeta{
			Kind:       "BGPAdvertisement",
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

// AddBGPAdvertisementIPAddressPool appends an IPAddressPool name to the BGPAdvertisement spec.
func AddBGPAdvertisementIPAddressPool(obj *metallbv1beta1.BGPAdvertisement, pool string) {
	obj.Spec.IPAddressPools = append(obj.Spec.IPAddressPools, pool)
}

// AddBGPAdvertisementNodeSelector appends a node selector to the BGPAdvertisement spec.
func AddBGPAdvertisementNodeSelector(obj *metallbv1beta1.BGPAdvertisement, sel metav1.LabelSelector) {
	obj.Spec.NodeSelectors = append(obj.Spec.NodeSelectors, sel)
}

// AddBGPAdvertisementCommunity appends a BGP community to the BGPAdvertisement spec.
func AddBGPAdvertisementCommunity(obj *metallbv1beta1.BGPAdvertisement, c string) {
	obj.Spec.Communities = append(obj.Spec.Communities, c)
}

// AddBGPAdvertisementPeer appends a peer name to the BGPAdvertisement spec.
func AddBGPAdvertisementPeer(obj *metallbv1beta1.BGPAdvertisement, peer string) {
	obj.Spec.Peers = append(obj.Spec.Peers, peer)
}

// SetBGPAdvertisementLocalPref sets the localPref value on the BGPAdvertisement spec.
func SetBGPAdvertisementLocalPref(obj *metallbv1beta1.BGPAdvertisement, pref uint32) {
	obj.Spec.LocalPref = pref
}
