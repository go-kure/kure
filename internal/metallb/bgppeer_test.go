package metallb

import (
	metallbv1beta1 "go.universe.tf/metallb/api/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"testing"
)

func TestCreateBGPPeer(t *testing.T) {
	spec := metallbv1beta1.BGPPeerSpec{MyASN: 64512, ASN: 64512, Address: "1.1.1.1"}
	peer := CreateBGPPeer("peer", "default", spec)
	if peer.Name != "peer" || peer.Namespace != "default" {
		t.Fatalf("unexpected metadata")
	}
	if peer.Spec.Address != "1.1.1.1" {
		t.Fatalf("address not set")
	}
}

func TestBGPPeerHelpers(t *testing.T) {
	p := CreateBGPPeer("peer", "ns", metallbv1beta1.BGPPeerSpec{})
	AddBGPPeerNodeSelector(p, metallbv1beta1.NodeSelector{MatchLabels: map[string]string{"disktype": "ssd"}})
	SetBGPPeerPort(p, 179)
	SetBGPPeerHoldTime(p, metav1.Duration{})
	SetBGPPeerKeepaliveTime(p, metav1.Duration{})
	SetBGPPeerSrcAddress(p, "1.1.1.2")
	SetBGPPeerRouterID(p, "2.2.2.2")
	SetBGPPeerEBGPMultiHop(p, true)
	SetBGPPeerPassword(p, "pw")
	SetBGPPeerBFDProfile(p, "profile")

	if len(p.Spec.NodeSelectors) != 1 || p.Spec.NodeSelectors[0].MatchLabels["disktype"] != "ssd" {
		t.Errorf("node selector not added")
	}
	if p.Spec.Port != 179 {
		t.Errorf("port not set")
	}
	if p.Spec.SrcAddress != "1.1.1.2" {
		t.Errorf("src address not set")
	}
	if p.Spec.RouterID != "2.2.2.2" {
		t.Errorf("router id not set")
	}
	if !p.Spec.EBGPMultiHop {
		t.Errorf("ebgpMultiHop not set")
	}
	if p.Spec.Password != "pw" {
		t.Errorf("password not set")
	}
	if p.Spec.BFDProfile != "profile" {
		t.Errorf("bfd profile not set")
	}
}
