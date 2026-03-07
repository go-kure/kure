package metallb

import (
	intmetallb "github.com/go-kure/kure/internal/metallb"

	metallbv1beta1 "go.universe.tf/metallb/api/v1beta1"
)

// IPAddressPool converts the config to a MetalLB IPAddressPool object.
func IPAddressPool(cfg *IPAddressPoolConfig) *metallbv1beta1.IPAddressPool {
	if cfg == nil {
		return nil
	}
	obj := intmetallb.CreateIPAddressPool(cfg.Name, cfg.Namespace, metallbv1beta1.IPAddressPoolSpec{})
	for _, addr := range cfg.Addresses {
		intmetallb.AddIPAddressPoolAddress(obj, addr) //nolint:errcheck,gosec // obj is freshly created
	}
	if cfg.AutoAssign != nil {
		intmetallb.SetIPAddressPoolAutoAssign(obj, *cfg.AutoAssign) //nolint:errcheck,gosec // obj is freshly created
	}
	if cfg.AvoidBuggyIPs != nil {
		intmetallb.SetIPAddressPoolAvoidBuggyIPs(obj, *cfg.AvoidBuggyIPs) //nolint:errcheck,gosec // obj is freshly created
	}
	if cfg.AllocateTo != nil {
		intmetallb.SetIPAddressPoolAllocateTo(obj, cfg.AllocateTo) //nolint:errcheck,gosec // obj is freshly created
	}
	return obj
}

// BGPPeer converts the config to a MetalLB BGPPeer object.
func BGPPeer(cfg *BGPPeerConfig) *metallbv1beta1.BGPPeer {
	if cfg == nil {
		return nil
	}
	obj := intmetallb.CreateBGPPeer(cfg.Name, cfg.Namespace, metallbv1beta1.BGPPeerSpec{
		MyASN:   cfg.MyASN,
		ASN:     cfg.ASN,
		Address: cfg.Address,
	})
	if cfg.Port != 0 {
		intmetallb.SetBGPPeerPort(obj, cfg.Port) //nolint:errcheck,gosec // obj is freshly created
	}
	if cfg.HoldTime != nil {
		intmetallb.SetBGPPeerHoldTime(obj, *cfg.HoldTime) //nolint:errcheck,gosec // obj is freshly created
	}
	if cfg.KeepaliveTime != nil {
		intmetallb.SetBGPPeerKeepaliveTime(obj, *cfg.KeepaliveTime) //nolint:errcheck,gosec // obj is freshly created
	}
	if cfg.SrcAddress != "" {
		intmetallb.SetBGPPeerSrcAddress(obj, cfg.SrcAddress) //nolint:errcheck,gosec // obj is freshly created
	}
	if cfg.RouterID != "" {
		intmetallb.SetBGPPeerRouterID(obj, cfg.RouterID) //nolint:errcheck,gosec // obj is freshly created
	}
	if cfg.EBGPMultiHop != nil {
		intmetallb.SetBGPPeerEBGPMultiHop(obj, *cfg.EBGPMultiHop) //nolint:errcheck,gosec // obj is freshly created
	}
	if cfg.Password != "" {
		intmetallb.SetBGPPeerPassword(obj, cfg.Password) //nolint:errcheck,gosec // obj is freshly created
	}
	if cfg.BFDProfile != "" {
		intmetallb.SetBGPPeerBFDProfile(obj, cfg.BFDProfile) //nolint:errcheck,gosec // obj is freshly created
	}
	for _, sel := range cfg.NodeSelectors {
		intmetallb.AddBGPPeerNodeSelector(obj, sel) //nolint:errcheck,gosec // obj is freshly created
	}
	return obj
}

// BGPAdvertisement converts the config to a MetalLB BGPAdvertisement object.
func BGPAdvertisement(cfg *BGPAdvertisementConfig) *metallbv1beta1.BGPAdvertisement {
	if cfg == nil {
		return nil
	}
	obj := intmetallb.CreateBGPAdvertisement(cfg.Name, cfg.Namespace, metallbv1beta1.BGPAdvertisementSpec{})
	for _, pool := range cfg.IPAddressPools {
		intmetallb.AddBGPAdvertisementIPAddressPool(obj, pool) //nolint:errcheck,gosec // obj is freshly created
	}
	for _, peer := range cfg.Peers {
		intmetallb.AddBGPAdvertisementPeer(obj, peer) //nolint:errcheck,gosec // obj is freshly created
	}
	for _, c := range cfg.Communities {
		intmetallb.AddBGPAdvertisementCommunity(obj, c) //nolint:errcheck,gosec // obj is freshly created
	}
	if cfg.LocalPref != 0 {
		intmetallb.SetBGPAdvertisementLocalPref(obj, cfg.LocalPref) //nolint:errcheck,gosec // obj is freshly created
	}
	for _, sel := range cfg.NodeSelectors {
		intmetallb.AddBGPAdvertisementNodeSelector(obj, sel) //nolint:errcheck,gosec // obj is freshly created
	}
	return obj
}

// L2Advertisement converts the config to a MetalLB L2Advertisement object.
func L2Advertisement(cfg *L2AdvertisementConfig) *metallbv1beta1.L2Advertisement {
	if cfg == nil {
		return nil
	}
	obj := intmetallb.CreateL2Advertisement(cfg.Name, cfg.Namespace, metallbv1beta1.L2AdvertisementSpec{})
	for _, pool := range cfg.IPAddressPools {
		intmetallb.AddL2AdvertisementIPAddressPool(obj, pool) //nolint:errcheck,gosec // obj is freshly created
	}
	for _, iface := range cfg.Interfaces {
		intmetallb.AddL2AdvertisementInterface(obj, iface) //nolint:errcheck,gosec // obj is freshly created
	}
	for _, sel := range cfg.NodeSelectors {
		intmetallb.AddL2AdvertisementNodeSelector(obj, sel) //nolint:errcheck,gosec // obj is freshly created
	}
	return obj
}

// BFDProfile converts the config to a MetalLB BFDProfile object.
func BFDProfile(cfg *BFDProfileConfig) *metallbv1beta1.BFDProfile {
	if cfg == nil {
		return nil
	}
	obj := intmetallb.CreateBFDProfile(cfg.Name, cfg.Namespace, metallbv1beta1.BFDProfileSpec{})
	if cfg.DetectMultiplier != nil {
		intmetallb.SetBFDProfileDetectMultiplier(obj, *cfg.DetectMultiplier) //nolint:errcheck,gosec // obj is freshly created
	}
	if cfg.EchoInterval != nil {
		intmetallb.SetBFDProfileEchoInterval(obj, *cfg.EchoInterval) //nolint:errcheck,gosec // obj is freshly created
	}
	if cfg.EchoMode != nil {
		intmetallb.SetBFDProfileEchoMode(obj, *cfg.EchoMode) //nolint:errcheck,gosec // obj is freshly created
	}
	if cfg.PassiveMode != nil {
		intmetallb.SetBFDProfilePassiveMode(obj, *cfg.PassiveMode) //nolint:errcheck,gosec // obj is freshly created
	}
	return obj
}
