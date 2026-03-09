package metallb

import (
	"testing"

	metallbv1beta1 "go.universe.tf/metallb/api/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestCreateL2Advertisement(t *testing.T) {
	adv := CreateL2Advertisement("adv", "default", metallbv1beta1.L2AdvertisementSpec{})
	if adv.Name != "adv" || adv.Namespace != "default" {
		t.Fatalf("unexpected metadata")
	}
}

func TestL2AdvertisementHelpers(t *testing.T) {
	adv := CreateL2Advertisement("adv", "ns", metallbv1beta1.L2AdvertisementSpec{})
	AddL2AdvertisementIPAddressPool(adv, "pool")
	AddL2AdvertisementNodeSelector(adv, metav1.LabelSelector{MatchLabels: map[string]string{"role": "lb"}})
	AddL2AdvertisementInterface(adv, "eth0")

	if len(adv.Spec.IPAddressPools) != 1 || adv.Spec.IPAddressPools[0] != "pool" {
		t.Errorf("pool not added")
	}
	if len(adv.Spec.NodeSelectors) != 1 || adv.Spec.NodeSelectors[0].MatchLabels["role"] != "lb" {
		t.Errorf("selector not added")
	}
	if len(adv.Spec.Interfaces) != 1 || adv.Spec.Interfaces[0] != "eth0" {
		t.Errorf("interface not added")
	}
}
