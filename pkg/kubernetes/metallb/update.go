package metallb

import (
	intmetallb "github.com/go-kure/kure/internal/metallb"

	metallbv1beta1 "go.universe.tf/metallb/api/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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

// AddIPAddressPoolAddress delegates to the internal helper.
func AddIPAddressPoolAddress(obj *metallbv1beta1.IPAddressPool, addr string) error {
	return intmetallb.AddIPAddressPoolAddress(obj, addr)
}

// SetIPAddressPoolAutoAssign delegates to the internal helper.
func SetIPAddressPoolAutoAssign(obj *metallbv1beta1.IPAddressPool, auto bool) error {
	return intmetallb.SetIPAddressPoolAutoAssign(obj, auto)
}

// SetIPAddressPoolAvoidBuggyIPs delegates to the internal helper.
func SetIPAddressPoolAvoidBuggyIPs(obj *metallbv1beta1.IPAddressPool, avoid bool) error {
	return intmetallb.SetIPAddressPoolAvoidBuggyIPs(obj, avoid)
}

// SetIPAddressPoolAllocateTo delegates to the internal helper.
func SetIPAddressPoolAllocateTo(obj *metallbv1beta1.IPAddressPool, alloc *metallbv1beta1.ServiceAllocation) error {
	return intmetallb.SetIPAddressPoolAllocateTo(obj, alloc)
}

// AddBGPPeerNodeSelector delegates to the internal helper.
func AddBGPPeerNodeSelector(obj *metallbv1beta1.BGPPeer, sel metallbv1beta1.NodeSelector) error {
	return intmetallb.AddBGPPeerNodeSelector(obj, sel)
}

// SetBGPPeerPort delegates to the internal helper.
func SetBGPPeerPort(obj *metallbv1beta1.BGPPeer, port uint16) error {
	return intmetallb.SetBGPPeerPort(obj, port)
}

// SetBGPPeerHoldTime delegates to the internal helper.
func SetBGPPeerHoldTime(obj *metallbv1beta1.BGPPeer, d metav1.Duration) error {
	return intmetallb.SetBGPPeerHoldTime(obj, d)
}

// SetBGPPeerKeepaliveTime delegates to the internal helper.
func SetBGPPeerKeepaliveTime(obj *metallbv1beta1.BGPPeer, d metav1.Duration) error {
	return intmetallb.SetBGPPeerKeepaliveTime(obj, d)
}

// SetBGPPeerSrcAddress delegates to the internal helper.
func SetBGPPeerSrcAddress(obj *metallbv1beta1.BGPPeer, addr string) error {
	return intmetallb.SetBGPPeerSrcAddress(obj, addr)
}

// SetBGPPeerRouterID delegates to the internal helper.
func SetBGPPeerRouterID(obj *metallbv1beta1.BGPPeer, id string) error {
	return intmetallb.SetBGPPeerRouterID(obj, id)
}

// SetBGPPeerEBGPMultiHop delegates to the internal helper.
func SetBGPPeerEBGPMultiHop(obj *metallbv1beta1.BGPPeer, multi bool) error {
	return intmetallb.SetBGPPeerEBGPMultiHop(obj, multi)
}

// SetBGPPeerPassword delegates to the internal helper.
func SetBGPPeerPassword(obj *metallbv1beta1.BGPPeer, pw string) error {
	return intmetallb.SetBGPPeerPassword(obj, pw)
}

// SetBGPPeerBFDProfile delegates to the internal helper.
func SetBGPPeerBFDProfile(obj *metallbv1beta1.BGPPeer, profile string) error {
	return intmetallb.SetBGPPeerBFDProfile(obj, profile)
}

// AddBGPAdvertisementIPAddressPool delegates to the internal helper.
func AddBGPAdvertisementIPAddressPool(obj *metallbv1beta1.BGPAdvertisement, pool string) error {
	return intmetallb.AddBGPAdvertisementIPAddressPool(obj, pool)
}

// AddBGPAdvertisementNodeSelector delegates to the internal helper.
func AddBGPAdvertisementNodeSelector(obj *metallbv1beta1.BGPAdvertisement, sel metav1.LabelSelector) error {
	return intmetallb.AddBGPAdvertisementNodeSelector(obj, sel)
}

// AddBGPAdvertisementCommunity delegates to the internal helper.
func AddBGPAdvertisementCommunity(obj *metallbv1beta1.BGPAdvertisement, c string) error {
	return intmetallb.AddBGPAdvertisementCommunity(obj, c)
}

// AddBGPAdvertisementPeer delegates to the internal helper.
func AddBGPAdvertisementPeer(obj *metallbv1beta1.BGPAdvertisement, peer string) error {
	return intmetallb.AddBGPAdvertisementPeer(obj, peer)
}

// SetBGPAdvertisementLocalPref delegates to the internal helper.
func SetBGPAdvertisementLocalPref(obj *metallbv1beta1.BGPAdvertisement, pref uint32) error {
	return intmetallb.SetBGPAdvertisementLocalPref(obj, pref)
}

// AddL2AdvertisementIPAddressPool delegates to the internal helper.
func AddL2AdvertisementIPAddressPool(obj *metallbv1beta1.L2Advertisement, pool string) error {
	return intmetallb.AddL2AdvertisementIPAddressPool(obj, pool)
}

// AddL2AdvertisementNodeSelector delegates to the internal helper.
func AddL2AdvertisementNodeSelector(obj *metallbv1beta1.L2Advertisement, sel metav1.LabelSelector) error {
	return intmetallb.AddL2AdvertisementNodeSelector(obj, sel)
}

// AddL2AdvertisementInterface delegates to the internal helper.
func AddL2AdvertisementInterface(obj *metallbv1beta1.L2Advertisement, iface string) error {
	return intmetallb.AddL2AdvertisementInterface(obj, iface)
}

// SetBFDProfileDetectMultiplier delegates to the internal helper.
func SetBFDProfileDetectMultiplier(obj *metallbv1beta1.BFDProfile, mult uint32) error {
	return intmetallb.SetBFDProfileDetectMultiplier(obj, mult)
}

// SetBFDProfileEchoInterval delegates to the internal helper.
func SetBFDProfileEchoInterval(obj *metallbv1beta1.BFDProfile, interval uint32) error {
	return intmetallb.SetBFDProfileEchoInterval(obj, interval)
}

// SetBFDProfileEchoMode delegates to the internal helper.
func SetBFDProfileEchoMode(obj *metallbv1beta1.BFDProfile, mode bool) error {
	return intmetallb.SetBFDProfileEchoMode(obj, mode)
}

// SetBFDProfilePassiveMode delegates to the internal helper.
func SetBFDProfilePassiveMode(obj *metallbv1beta1.BFDProfile, mode bool) error {
	return intmetallb.SetBFDProfilePassiveMode(obj, mode)
}
