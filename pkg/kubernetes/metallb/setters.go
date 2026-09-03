package metallb

import (
	metallbv1beta1 "go.universe.tf/metallb/api/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// IPAddressPool setters

// AddIPAddressPoolAddress adds an address range to the IPAddressPool spec.
func AddIPAddressPoolAddress(obj *metallbv1beta1.IPAddressPool, addr string) {
	obj.Spec.Addresses = append(obj.Spec.Addresses, addr)
}

// SetIPAddressPoolAutoAssign sets the autoAssign flag on the IPAddressPool spec.
func SetIPAddressPoolAutoAssign(obj *metallbv1beta1.IPAddressPool, auto bool) {
	obj.Spec.AutoAssign = &auto
}

// SetIPAddressPoolAllocateTo sets the allocation policy on the IPAddressPool spec.
func SetIPAddressPoolAllocateTo(obj *metallbv1beta1.IPAddressPool, alloc *metallbv1beta1.ServiceAllocation) {
	obj.Spec.AllocateTo = alloc
}

// BGPPeer setters

// AddBGPPeerNodeSelector appends a node selector to the BGPPeer spec.
func AddBGPPeerNodeSelector(obj *metallbv1beta1.BGPPeer, sel metallbv1beta1.NodeSelector) {
	obj.Spec.NodeSelectors = append(obj.Spec.NodeSelectors, sel)
}

// BGPAdvertisement setters

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

// L2Advertisement setters

// AddL2AdvertisementIPAddressPool appends an IPAddressPool reference to the L2Advertisement spec.
func AddL2AdvertisementIPAddressPool(obj *metallbv1beta1.L2Advertisement, pool string) {
	obj.Spec.IPAddressPools = append(obj.Spec.IPAddressPools, pool)
}

// AddL2AdvertisementNodeSelector appends a node selector to the L2Advertisement spec.
func AddL2AdvertisementNodeSelector(obj *metallbv1beta1.L2Advertisement, sel metav1.LabelSelector) {
	obj.Spec.NodeSelectors = append(obj.Spec.NodeSelectors, sel)
}

// AddL2AdvertisementInterface appends a network interface name to the L2Advertisement spec.
func AddL2AdvertisementInterface(obj *metallbv1beta1.L2Advertisement, iface string) {
	obj.Spec.Interfaces = append(obj.Spec.Interfaces, iface)
}

// BFDProfile setters

// SetBFDProfileDetectMultiplier sets the detect multiplier on the BFDProfile spec.
func SetBFDProfileDetectMultiplier(obj *metallbv1beta1.BFDProfile, mult uint32) {
	obj.Spec.DetectMultiplier = &mult
}

// SetBFDProfileEchoInterval sets the echo interval on the BFDProfile spec.
func SetBFDProfileEchoInterval(obj *metallbv1beta1.BFDProfile, interval uint32) {
	obj.Spec.EchoInterval = &interval
}

// SetBFDProfileEchoMode sets the echo mode on the BFDProfile spec.
func SetBFDProfileEchoMode(obj *metallbv1beta1.BFDProfile, mode bool) {
	obj.Spec.EchoMode = &mode
}

// SetBFDProfilePassiveMode sets the passive mode on the BFDProfile spec.
func SetBFDProfilePassiveMode(obj *metallbv1beta1.BFDProfile, mode bool) {
	obj.Spec.PassiveMode = &mode
}
