package metallb

import (
	"testing"

	metallbv1beta1 "go.universe.tf/metallb/api/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestCreateBGPAdvertisement(t *testing.T) {
	spec := metallbv1beta1.BGPAdvertisementSpec{
		LocalPref: 100,
	}
	adv := CreateBGPAdvertisement("adv", "default", spec)
	if adv.Name != "adv" || adv.Namespace != "default" {
		t.Fatalf("unexpected metadata")
	}
	if adv.Spec.LocalPref != 100 {
		t.Fatalf("localpref not set")
	}
}

func TestBGPAdvertisementHelpers(t *testing.T) {
	adv := CreateBGPAdvertisement("adv", "ns", metallbv1beta1.BGPAdvertisementSpec{})
	AddBGPAdvertisementIPAddressPool(adv, "pool")
	AddBGPAdvertisementNodeSelector(adv, metav1.LabelSelector{MatchLabels: map[string]string{"node": "1"}})
	AddBGPAdvertisementCommunity(adv, "64512:1")
	AddBGPAdvertisementPeer(adv, "peer1")
	SetBGPAdvertisementLocalPref(adv, 200)

	if len(adv.Spec.IPAddressPools) != 1 || adv.Spec.IPAddressPools[0] != "pool" {
		t.Errorf("pool not added")
	}
	if len(adv.Spec.NodeSelectors) != 1 || adv.Spec.NodeSelectors[0].MatchLabels["node"] != "1" {
		t.Errorf("node selector not added")
	}
	if len(adv.Spec.Communities) != 1 || adv.Spec.Communities[0] != "64512:1" {
		t.Errorf("community not added")
	}
	if len(adv.Spec.Peers) != 1 || adv.Spec.Peers[0] != "peer1" {
		t.Errorf("peer not added")
	}
	if adv.Spec.LocalPref != 200 {
		t.Errorf("localpref not set")
	}
}
