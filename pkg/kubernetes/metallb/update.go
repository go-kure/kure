package metallb

import (
	metallbv1beta1 "go.universe.tf/metallb/api/v1beta1"
)

// SetIPAddressPoolSpec replaces the spec on the IPAddressPool object.
func SetIPAddressPoolSpec(obj *metallbv1beta1.IPAddressPool, spec metallbv1beta1.IPAddressPoolSpec) {
	obj.Spec = spec
}

// SetBGPPeerSpec replaces the spec on the BGPPeer object.
func SetBGPPeerSpec(obj *metallbv1beta1.BGPPeer, spec metallbv1beta1.BGPPeerSpec) {
	obj.Spec = spec
}

// SetBGPAdvertisementSpec replaces the spec on the BGPAdvertisement object.
func SetBGPAdvertisementSpec(obj *metallbv1beta1.BGPAdvertisement, spec metallbv1beta1.BGPAdvertisementSpec) {
	obj.Spec = spec
}

// SetL2AdvertisementSpec replaces the spec on the L2Advertisement object.
func SetL2AdvertisementSpec(obj *metallbv1beta1.L2Advertisement, spec metallbv1beta1.L2AdvertisementSpec) {
	obj.Spec = spec
}

// SetBFDProfileSpec replaces the spec on the BFDProfile object.
func SetBFDProfileSpec(obj *metallbv1beta1.BFDProfile, spec metallbv1beta1.BFDProfileSpec) {
	obj.Spec = spec
}
