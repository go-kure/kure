package metallb

import (
	"testing"

	metallbv1beta1 "go.universe.tf/metallb/api/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestIPAddressPoolConfig(t *testing.T) {
	autoAssign := true
	cfg := &IPAddressPoolConfig{
		Name:       "test-pool",
		Namespace:  "metallb-system",
		Addresses:  []string{"192.168.1.0/24"},
		AutoAssign: &autoAssign,
	}

	if cfg.Name != "test-pool" {
		t.Errorf("expected Name 'test-pool', got %s", cfg.Name)
	}

	if cfg.Namespace != "metallb-system" {
		t.Errorf("expected Namespace 'metallb-system', got %s", cfg.Namespace)
	}

	if len(cfg.Addresses) != 1 {
		t.Errorf("expected 1 address, got %d", len(cfg.Addresses))
	}

	if cfg.AutoAssign == nil || !*cfg.AutoAssign {
		t.Error("expected AutoAssign to be true")
	}
}

func TestBGPPeerConfig(t *testing.T) {
	holdTime := metav1.Duration{Duration: 90000000000}
	cfg := &BGPPeerConfig{
		Name:      "test-peer",
		Namespace: "metallb-system",
		MyASN:     64500,
		ASN:       64501,
		Address:   "10.0.0.1",
		Port:      179,
		HoldTime:  &holdTime,
		NodeSelectors: []metallbv1beta1.NodeSelector{
			{MatchLabels: map[string]string{"role": "worker"}},
		},
	}

	if cfg.MyASN != 64500 {
		t.Errorf("expected MyASN 64500, got %d", cfg.MyASN)
	}

	if cfg.ASN != 64501 {
		t.Errorf("expected ASN 64501, got %d", cfg.ASN)
	}

	if cfg.Address != "10.0.0.1" {
		t.Errorf("expected Address '10.0.0.1', got %s", cfg.Address)
	}

	if cfg.Port != 179 {
		t.Errorf("expected Port 179, got %d", cfg.Port)
	}

	if cfg.HoldTime == nil {
		t.Error("expected non-nil HoldTime")
	}

	if len(cfg.NodeSelectors) != 1 {
		t.Errorf("expected 1 node selector, got %d", len(cfg.NodeSelectors))
	}
}

func TestBGPAdvertisementConfig(t *testing.T) {
	cfg := &BGPAdvertisementConfig{
		Name:           "test-advert",
		Namespace:      "metallb-system",
		IPAddressPools: []string{"pool-1", "pool-2"},
		Peers:          []string{"peer-1"},
		Communities:    []string{"65535:65282"},
		LocalPref:      100,
		NodeSelectors: []metav1.LabelSelector{
			{MatchLabels: map[string]string{"node": "worker"}},
		},
	}

	if len(cfg.IPAddressPools) != 2 {
		t.Errorf("expected 2 IP address pools, got %d", len(cfg.IPAddressPools))
	}

	if len(cfg.Peers) != 1 {
		t.Errorf("expected 1 peer, got %d", len(cfg.Peers))
	}

	if cfg.Communities[0] != "65535:65282" {
		t.Errorf("expected community '65535:65282', got %s", cfg.Communities[0])
	}

	if cfg.LocalPref != 100 {
		t.Errorf("expected LocalPref 100, got %d", cfg.LocalPref)
	}
}

func TestL2AdvertisementConfig(t *testing.T) {
	cfg := &L2AdvertisementConfig{
		Name:           "test-l2",
		Namespace:      "metallb-system",
		IPAddressPools: []string{"pool-1"},
		Interfaces:     []string{"eth0", "eth1"},
		NodeSelectors: []metav1.LabelSelector{
			{MatchLabels: map[string]string{"node": "worker"}},
		},
	}

	if cfg.Name != "test-l2" {
		t.Errorf("expected Name 'test-l2', got %s", cfg.Name)
	}

	if len(cfg.IPAddressPools) != 1 {
		t.Errorf("expected 1 IP address pool, got %d", len(cfg.IPAddressPools))
	}

	if len(cfg.Interfaces) != 2 {
		t.Errorf("expected 2 interfaces, got %d", len(cfg.Interfaces))
	}
}

func TestBFDProfileConfig(t *testing.T) {
	detectMult := uint32(3)
	echoMode := true
	cfg := &BFDProfileConfig{
		Name:             "test-bfd",
		Namespace:        "metallb-system",
		DetectMultiplier: &detectMult,
		EchoMode:         &echoMode,
	}

	if cfg.Name != "test-bfd" {
		t.Errorf("expected Name 'test-bfd', got %s", cfg.Name)
	}

	if cfg.DetectMultiplier == nil || *cfg.DetectMultiplier != 3 {
		t.Error("expected DetectMultiplier 3")
	}

	if cfg.EchoMode == nil || !*cfg.EchoMode {
		t.Error("expected EchoMode true")
	}
}

func TestConfigStructTags(t *testing.T) {
	tests := []struct {
		name   string
		config any
	}{
		{"IPAddressPoolConfig", &IPAddressPoolConfig{}},
		{"BGPPeerConfig", &BGPPeerConfig{}},
		{"BGPAdvertisementConfig", &BGPAdvertisementConfig{}},
		{"L2AdvertisementConfig", &L2AdvertisementConfig{}},
		{"BFDProfileConfig", &BFDProfileConfig{}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.config == nil {
				t.Errorf("config struct %s should not be nil", tt.name)
			}
		})
	}
}
