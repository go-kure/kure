package metallb

import (
	"testing"

	metallbv1beta1 "go.universe.tf/metallb/api/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
	if err := AddBGPPeerNodeSelector(p, metallbv1beta1.NodeSelector{MatchLabels: map[string]string{"disktype": "ssd"}}); err != nil {
		t.Fatalf("AddBGPPeerNodeSelector returned error: %v", err)
	}
	if err := SetBGPPeerPort(p, 179); err != nil {
		t.Fatalf("SetBGPPeerPort returned error: %v", err)
	}
	if err := SetBGPPeerHoldTime(p, metav1.Duration{}); err != nil {
		t.Fatalf("SetBGPPeerHoldTime returned error: %v", err)
	}
	if err := SetBGPPeerKeepaliveTime(p, metav1.Duration{}); err != nil {
		t.Fatalf("SetBGPPeerKeepaliveTime returned error: %v", err)
	}
	if err := SetBGPPeerSrcAddress(p, "1.1.1.2"); err != nil {
		t.Fatalf("SetBGPPeerSrcAddress returned error: %v", err)
	}
	if err := SetBGPPeerRouterID(p, "2.2.2.2"); err != nil {
		t.Fatalf("SetBGPPeerRouterID returned error: %v", err)
	}
	if err := SetBGPPeerEBGPMultiHop(p, true); err != nil {
		t.Fatalf("SetBGPPeerEBGPMultiHop returned error: %v", err)
	}
	if err := SetBGPPeerPassword(p, "pw"); err != nil {
		t.Fatalf("SetBGPPeerPassword returned error: %v", err)
	}
	if err := SetBGPPeerBFDProfile(p, "profile"); err != nil {
		t.Fatalf("SetBGPPeerBFDProfile returned error: %v", err)
	}

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

	if err := AddBGPPeerNodeSelector(nil, metallbv1beta1.NodeSelector{}); err == nil {
		t.Errorf("expected error when peer nil")
	}
	if err := SetBGPPeerPort(nil, 1); err == nil {
		t.Errorf("expected error when peer nil")
	}
	if err := SetBGPPeerHoldTime(nil, metav1.Duration{}); err == nil {
		t.Errorf("expected error when peer nil")
	}
	if err := SetBGPPeerKeepaliveTime(nil, metav1.Duration{}); err == nil {
		t.Errorf("expected error when peer nil")
	}
	if err := SetBGPPeerSrcAddress(nil, ""); err == nil {
		t.Errorf("expected error when peer nil")
	}
	if err := SetBGPPeerRouterID(nil, ""); err == nil {
		t.Errorf("expected error when peer nil")
	}
	if err := SetBGPPeerEBGPMultiHop(nil, false); err == nil {
		t.Errorf("expected error when peer nil")
	}
	if err := SetBGPPeerPassword(nil, ""); err == nil {
		t.Errorf("expected error when peer nil")
	}
	if err := SetBGPPeerBFDProfile(nil, ""); err == nil {
		t.Errorf("expected error when peer nil")
	}
}
