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
	if err := AddL2AdvertisementIPAddressPool(adv, "pool"); err != nil {
		t.Fatalf("AddL2AdvertisementIPAddressPool returned error: %v", err)
	}
	if err := AddL2AdvertisementNodeSelector(adv, metav1.LabelSelector{MatchLabels: map[string]string{"role": "lb"}}); err != nil {
		t.Fatalf("AddL2AdvertisementNodeSelector returned error: %v", err)
	}
	if err := AddL2AdvertisementInterface(adv, "eth0"); err != nil {
		t.Fatalf("AddL2AdvertisementInterface returned error: %v", err)
	}

	if len(adv.Spec.IPAddressPools) != 1 || adv.Spec.IPAddressPools[0] != "pool" {
		t.Errorf("pool not added")
	}
	if len(adv.Spec.NodeSelectors) != 1 || adv.Spec.NodeSelectors[0].MatchLabels["role"] != "lb" {
		t.Errorf("selector not added")
	}
	if len(adv.Spec.Interfaces) != 1 || adv.Spec.Interfaces[0] != "eth0" {
		t.Errorf("interface not added")
	}

	if err := AddL2AdvertisementIPAddressPool(nil, "x"); err == nil {
		t.Errorf("expected error when adv nil")
	}
	if err := AddL2AdvertisementNodeSelector(nil, metav1.LabelSelector{}); err == nil {
		t.Errorf("expected error when adv nil")
	}
	if err := AddL2AdvertisementInterface(nil, "x"); err == nil {
		t.Errorf("expected error when adv nil")
	}
}
