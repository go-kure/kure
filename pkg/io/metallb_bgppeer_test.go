package io

import (
	"testing"

	metallbv1beta1 "go.universe.tf/metallb/api/v1beta1"
	metallbv1beta2 "go.universe.tf/metallb/api/v1beta2"
)

// MetalLB serves BGPPeer at v1beta1, which it deprecates, and at v1beta2, the
// stored version. A manifest at either one parses to that version's own typed
// object: the two differ in how they select nodes, so one type cannot stand in
// for the other.
func TestParseMetalLBBGPPeerAtBothVersions(t *testing.T) {
	const data = `apiVersion: metallb.io/v1beta1
kind: BGPPeer
metadata:
  name: old
  namespace: metallb-system
spec:
  myASN: 64500
  peerASN: 64501
  peerAddress: 10.0.0.1
  nodeSelectors:
  - matchLabels:
      role: worker
    matchExpressions:
    - key: zone
      operator: In
      values: [a]
---
apiVersion: metallb.io/v1beta2
kind: BGPPeer
metadata:
  name: new
  namespace: metallb-system
spec:
  myASN: 64500
  peerASN: 64501
  peerAddress: 10.0.0.1
  nodeSelectors:
  - matchLabels:
      role: worker
    matchExpressions:
    - key: zone
      operator: In
      values: [a]
`
	objs, err := ParseYAML([]byte(data))
	if err != nil {
		t.Fatalf("ParseYAML: %v", err)
	}
	if len(objs) != 2 {
		t.Fatalf("parsed %d objects, want 2", len(objs))
	}

	old, ok := objs[0].(*metallbv1beta1.BGPPeer)
	if !ok {
		t.Fatalf("the v1beta1 document parsed as %T, want *v1beta1.BGPPeer", objs[0])
	}
	if old.APIVersion != "metallb.io/v1beta1" || old.Kind != "BGPPeer" || old.Name != "old" {
		t.Errorf("v1beta1 object = %s %s %q", old.APIVersion, old.Kind, old.Name)
	}
	if old.Spec.MyASN != 64500 || old.Spec.ASN != 64501 || old.Spec.Address != "10.0.0.1" {
		t.Errorf("v1beta1 spec = %+v", old.Spec)
	}
	if len(old.Spec.NodeSelectors) != 1 ||
		old.Spec.NodeSelectors[0].MatchLabels["role"] != "worker" ||
		len(old.Spec.NodeSelectors[0].MatchExpressions) != 1 ||
		old.Spec.NodeSelectors[0].MatchExpressions[0].Operator != "In" {
		t.Errorf("v1beta1 node selectors = %+v", old.Spec.NodeSelectors)
	}

	current, ok := objs[1].(*metallbv1beta2.BGPPeer)
	if !ok {
		t.Fatalf("the v1beta2 document parsed as %T, want *v1beta2.BGPPeer", objs[1])
	}
	if current.APIVersion != "metallb.io/v1beta2" || current.Kind != "BGPPeer" || current.Name != "new" {
		t.Errorf("v1beta2 object = %s %s %q", current.APIVersion, current.Kind, current.Name)
	}
	if current.Spec.MyASN != 64500 || current.Spec.ASN != 64501 || current.Spec.Address != "10.0.0.1" {
		t.Errorf("v1beta2 spec = %+v", current.Spec)
	}
	if len(current.Spec.NodeSelectors) != 1 ||
		current.Spec.NodeSelectors[0].MatchLabels["role"] != "worker" ||
		len(current.Spec.NodeSelectors[0].MatchExpressions) != 1 ||
		current.Spec.NodeSelectors[0].MatchExpressions[0].Operator != "In" {
		t.Errorf("v1beta2 node selectors = %+v", current.Spec.NodeSelectors)
	}
}
