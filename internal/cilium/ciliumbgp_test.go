package cilium

import (
	"testing"

	ciliumv2 "github.com/cilium/cilium/pkg/k8s/apis/cilium.io/v2"
	slimv1 "github.com/cilium/cilium/pkg/k8s/slim/k8s/apis/meta/v1"
)

func TestCreateCiliumBGPClusterConfig(t *testing.T) {
	obj := CreateCiliumBGPClusterConfig("my-bgp")
	if obj == nil {
		t.Fatal("expected non-nil CiliumBGPClusterConfig")
	}
	if obj.Name != "my-bgp" {
		t.Errorf("expected Name 'my-bgp', got %s", obj.Name)
	}
	if obj.Namespace != "" {
		t.Errorf("expected empty Namespace for cluster-scoped resource, got %s", obj.Namespace)
	}
	if obj.Kind != "CiliumBGPClusterConfig" {
		t.Errorf("expected Kind 'CiliumBGPClusterConfig', got %s", obj.Kind)
	}
	if obj.APIVersion != "cilium.io/v2" {
		t.Errorf("expected APIVersion 'cilium.io/v2', got %s", obj.APIVersion)
	}
}

func TestSetCiliumBGPClusterConfigNodeSelector(t *testing.T) {
	obj := CreateCiliumBGPClusterConfig("p")
	sel := &slimv1.LabelSelector{MatchLabels: map[string]string{"bgp": "enabled"}}
	SetCiliumBGPClusterConfigNodeSelector(obj, sel)
	if obj.Spec.NodeSelector == nil {
		t.Fatal("expected non-nil NodeSelector")
	}
}

func TestAddCiliumBGPClusterConfigBGPInstance(t *testing.T) {
	obj := CreateCiliumBGPClusterConfig("p")
	instance := ciliumv2.CiliumBGPInstance{Name: "instance-1"}
	AddCiliumBGPClusterConfigBGPInstance(obj, instance)
	AddCiliumBGPClusterConfigBGPInstance(obj, instance)
	if len(obj.Spec.BGPInstances) != 2 {
		t.Fatalf("expected 2 BGP instances, got %d", len(obj.Spec.BGPInstances))
	}
}

func TestCreateCiliumBGPPeerConfig(t *testing.T) {
	obj := CreateCiliumBGPPeerConfig("my-peer")
	if obj == nil {
		t.Fatal("expected non-nil CiliumBGPPeerConfig")
	}
	if obj.Kind != "CiliumBGPPeerConfig" {
		t.Errorf("expected Kind 'CiliumBGPPeerConfig', got %s", obj.Kind)
	}
	if obj.APIVersion != "cilium.io/v2" {
		t.Errorf("expected APIVersion 'cilium.io/v2', got %s", obj.APIVersion)
	}
}

func TestSetCiliumBGPPeerConfigAuthSecretRef(t *testing.T) {
	obj := CreateCiliumBGPPeerConfig("p")
	SetCiliumBGPPeerConfigAuthSecretRef(obj, "bgp-secret")
	if obj.Spec.AuthSecretRef == nil || *obj.Spec.AuthSecretRef != "bgp-secret" {
		t.Errorf("unexpected AuthSecretRef: %v", obj.Spec.AuthSecretRef)
	}
}

func TestSetCiliumBGPPeerConfigEBGPMultihop(t *testing.T) {
	obj := CreateCiliumBGPPeerConfig("p")
	SetCiliumBGPPeerConfigEBGPMultihop(obj, 3)
	if obj.Spec.EBGPMultihop == nil || *obj.Spec.EBGPMultihop != 3 {
		t.Errorf("unexpected EBGPMultihop: %v", obj.Spec.EBGPMultihop)
	}
}

func TestAddCiliumBGPPeerConfigFamily(t *testing.T) {
	obj := CreateCiliumBGPPeerConfig("p")
	fam := ciliumv2.CiliumBGPFamilyWithAdverts{
		CiliumBGPFamily: ciliumv2.CiliumBGPFamily{Afi: "ipv4", Safi: "unicast"},
	}
	AddCiliumBGPPeerConfigFamily(obj, fam)
	AddCiliumBGPPeerConfigFamily(obj, fam)
	if len(obj.Spec.Families) != 2 {
		t.Fatalf("expected 2 families, got %d", len(obj.Spec.Families))
	}
}

func TestCreateCiliumBGPAdvertisement(t *testing.T) {
	obj := CreateCiliumBGPAdvertisement("my-advert")
	if obj == nil {
		t.Fatal("expected non-nil CiliumBGPAdvertisement")
	}
	if obj.Kind != "CiliumBGPAdvertisement" {
		t.Errorf("expected Kind 'CiliumBGPAdvertisement', got %s", obj.Kind)
	}
	if obj.APIVersion != "cilium.io/v2" {
		t.Errorf("expected APIVersion 'cilium.io/v2', got %s", obj.APIVersion)
	}
}

func TestAddCiliumBGPAdvertisementEntry(t *testing.T) {
	obj := CreateCiliumBGPAdvertisement("p")
	entry := ciliumv2.BGPAdvertisement{AdvertisementType: ciliumv2.BGPServiceAdvert}
	AddCiliumBGPAdvertisementEntry(obj, entry)
	AddCiliumBGPAdvertisementEntry(obj, entry)
	if len(obj.Spec.Advertisements) != 2 {
		t.Fatalf("expected 2 advertisement entries, got %d", len(obj.Spec.Advertisements))
	}
}

func TestCreateCiliumBGPNodeConfig(t *testing.T) {
	obj := CreateCiliumBGPNodeConfig("my-node-config")
	if obj == nil {
		t.Fatal("expected non-nil CiliumBGPNodeConfig")
	}
	if obj.Kind != "CiliumBGPNodeConfig" {
		t.Errorf("expected Kind 'CiliumBGPNodeConfig', got %s", obj.Kind)
	}
	if obj.APIVersion != "cilium.io/v2" {
		t.Errorf("expected APIVersion 'cilium.io/v2', got %s", obj.APIVersion)
	}
}

func TestAddCiliumBGPNodeConfigBGPInstance(t *testing.T) {
	obj := CreateCiliumBGPNodeConfig("p")
	instance := ciliumv2.CiliumBGPNodeInstance{Name: "instance-1"}
	AddCiliumBGPNodeConfigBGPInstance(obj, instance)
	if len(obj.Spec.BGPInstances) != 1 {
		t.Fatalf("expected 1 BGP node instance, got %d", len(obj.Spec.BGPInstances))
	}
}

func TestCreateCiliumBGPNodeConfigOverride(t *testing.T) {
	obj := CreateCiliumBGPNodeConfigOverride("my-override")
	if obj == nil {
		t.Fatal("expected non-nil CiliumBGPNodeConfigOverride")
	}
	if obj.Kind != "CiliumBGPNodeConfigOverride" {
		t.Errorf("expected Kind 'CiliumBGPNodeConfigOverride', got %s", obj.Kind)
	}
	if obj.APIVersion != "cilium.io/v2" {
		t.Errorf("expected APIVersion 'cilium.io/v2', got %s", obj.APIVersion)
	}
}

func TestAddCiliumBGPNodeConfigOverrideBGPInstance(t *testing.T) {
	obj := CreateCiliumBGPNodeConfigOverride("p")
	routerID := "10.0.0.1"
	instance := ciliumv2.CiliumBGPNodeConfigInstanceOverride{
		Name:     "instance-1",
		RouterID: &routerID,
	}
	AddCiliumBGPNodeConfigOverrideBGPInstance(obj, instance)
	if len(obj.Spec.BGPInstances) != 1 {
		t.Fatalf("expected 1 BGP instance override, got %d", len(obj.Spec.BGPInstances))
	}
	if *obj.Spec.BGPInstances[0].RouterID != "10.0.0.1" {
		t.Errorf("unexpected RouterID: %s", *obj.Spec.BGPInstances[0].RouterID)
	}
}
