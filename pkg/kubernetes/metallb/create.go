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
		intmetallb.AddIPAddressPoolAddress(obj, addr)
	}
	if cfg.AutoAssign != nil {
		intmetallb.SetIPAddressPoolAutoAssign(obj, *cfg.AutoAssign)
	}
	if cfg.AvoidBuggyIPs != nil {
		intmetallb.SetIPAddressPoolAvoidBuggyIPs(obj, *cfg.AvoidBuggyIPs)
	}
	if cfg.AllocateTo != nil {
		intmetallb.SetIPAddressPoolAllocateTo(obj, cfg.AllocateTo)
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
		intmetallb.SetBGPPeerPort(obj, cfg.Port)
	}
	if cfg.HoldTime != nil {
		intmetallb.SetBGPPeerHoldTime(obj, *cfg.HoldTime)
	}
	if cfg.KeepaliveTime != nil {
		intmetallb.SetBGPPeerKeepaliveTime(obj, *cfg.KeepaliveTime)
	}
	if cfg.SrcAddress != "" {
		intmetallb.SetBGPPeerSrcAddress(obj, cfg.SrcAddress)
	}
	if cfg.RouterID != "" {
		intmetallb.SetBGPPeerRouterID(obj, cfg.RouterID)
	}
	if cfg.EBGPMultiHop != nil {
		intmetallb.SetBGPPeerEBGPMultiHop(obj, *cfg.EBGPMultiHop)
	}
	if cfg.Password != "" {
		intmetallb.SetBGPPeerPassword(obj, cfg.Password)
	}
	if cfg.BFDProfile != "" {
		intmetallb.SetBGPPeerBFDProfile(obj, cfg.BFDProfile)
	}
	for _, sel := range cfg.NodeSelectors {
		intmetallb.AddBGPPeerNodeSelector(obj, sel)
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
		intmetallb.AddBGPAdvertisementIPAddressPool(obj, pool)
	}
	for _, peer := range cfg.Peers {
		intmetallb.AddBGPAdvertisementPeer(obj, peer)
	}
	for _, c := range cfg.Communities {
		intmetallb.AddBGPAdvertisementCommunity(obj, c)
	}
	if cfg.LocalPref != 0 {
		intmetallb.SetBGPAdvertisementLocalPref(obj, cfg.LocalPref)
	}
	for _, sel := range cfg.NodeSelectors {
		intmetallb.AddBGPAdvertisementNodeSelector(obj, sel)
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
		intmetallb.AddL2AdvertisementIPAddressPool(obj, pool)
	}
	for _, iface := range cfg.Interfaces {
		intmetallb.AddL2AdvertisementInterface(obj, iface)
	}
	for _, sel := range cfg.NodeSelectors {
		intmetallb.AddL2AdvertisementNodeSelector(obj, sel)
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
		intmetallb.SetBFDProfileDetectMultiplier(obj, *cfg.DetectMultiplier)
	}
	if cfg.EchoInterval != nil {
		intmetallb.SetBFDProfileEchoInterval(obj, *cfg.EchoInterval)
	}
	if cfg.EchoMode != nil {
		intmetallb.SetBFDProfileEchoMode(obj, *cfg.EchoMode)
	}
	if cfg.PassiveMode != nil {
		intmetallb.SetBFDProfilePassiveMode(obj, *cfg.PassiveMode)
	}
	return obj
}
