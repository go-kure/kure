package metallb

import (
	metallbv1beta1 "go.universe.tf/metallb/api/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// IPAddressPoolConfig contains the configuration for a MetalLB IPAddressPool.
type IPAddressPoolConfig struct {
	Name          string                            `yaml:"name"`
	Namespace     string                            `yaml:"namespace"`
	Addresses     []string                          `yaml:"addresses"`
	AutoAssign    *bool                             `yaml:"autoAssign,omitempty"`
	AvoidBuggyIPs *bool                             `yaml:"avoidBuggyIPs,omitempty"`
	AllocateTo    *metallbv1beta1.ServiceAllocation `yaml:"allocateTo,omitempty"`
}

// BGPPeerConfig contains the configuration for a MetalLB BGPPeer.
type BGPPeerConfig struct {
	Name          string                        `yaml:"name"`
	Namespace     string                        `yaml:"namespace"`
	MyASN         uint32                        `yaml:"myASN"`
	ASN           uint32                        `yaml:"asn"`
	Address       string                        `yaml:"address"`
	Port          uint16                        `yaml:"port,omitempty"`
	HoldTime      *metav1.Duration              `yaml:"holdTime,omitempty"`
	KeepaliveTime *metav1.Duration              `yaml:"keepaliveTime,omitempty"`
	SrcAddress    string                        `yaml:"srcAddress,omitempty"`
	RouterID      string                        `yaml:"routerID,omitempty"`
	EBGPMultiHop  *bool                         `yaml:"ebgpMultiHop,omitempty"`
	Password      string                        `yaml:"password,omitempty"` //nolint:gosec // BGP auth password, not a credential
	BFDProfile    string                        `yaml:"bfdProfile,omitempty"`
	NodeSelectors []metallbv1beta1.NodeSelector `yaml:"nodeSelectors,omitempty"`
}

// BGPAdvertisementConfig contains the configuration for a MetalLB BGPAdvertisement.
type BGPAdvertisementConfig struct {
	Name           string                 `yaml:"name"`
	Namespace      string                 `yaml:"namespace"`
	IPAddressPools []string               `yaml:"ipAddressPools,omitempty"`
	Peers          []string               `yaml:"peers,omitempty"`
	Communities    []string               `yaml:"communities,omitempty"`
	LocalPref      uint32                 `yaml:"localPref,omitempty"`
	NodeSelectors  []metav1.LabelSelector `yaml:"nodeSelectors,omitempty"`
}

// L2AdvertisementConfig contains the configuration for a MetalLB L2Advertisement.
type L2AdvertisementConfig struct {
	Name           string                 `yaml:"name"`
	Namespace      string                 `yaml:"namespace"`
	IPAddressPools []string               `yaml:"ipAddressPools,omitempty"`
	Interfaces     []string               `yaml:"interfaces,omitempty"`
	NodeSelectors  []metav1.LabelSelector `yaml:"nodeSelectors,omitempty"`
}

// BFDProfileConfig contains the configuration for a MetalLB BFDProfile.
type BFDProfileConfig struct {
	Name             string  `yaml:"name"`
	Namespace        string  `yaml:"namespace"`
	DetectMultiplier *uint32 `yaml:"detectMultiplier,omitempty"`
	EchoInterval     *uint32 `yaml:"echoInterval,omitempty"`
	EchoMode         *bool   `yaml:"echoMode,omitempty"`
	PassiveMode      *bool   `yaml:"passiveMode,omitempty"`
}
