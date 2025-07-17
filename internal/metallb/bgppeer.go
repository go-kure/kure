package metallb

import (
	"errors"

	metallbv1beta1 "go.universe.tf/metallb/api/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// CreateBGPPeer returns a new BGPPeer object with the provided name, namespace and spec.
func CreateBGPPeer(name, namespace string, spec metallbv1beta1.BGPPeerSpec) *metallbv1beta1.BGPPeer {
	obj := &metallbv1beta1.BGPPeer{
		TypeMeta: metav1.TypeMeta{
			Kind:       "BGPPeer",
			APIVersion: metallbv1beta1.GroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: spec,
	}
	return obj
}

// AddBGPPeerNodeSelector appends a node selector to the BGPPeer spec.
func AddBGPPeerNodeSelector(obj *metallbv1beta1.BGPPeer, sel metallbv1beta1.NodeSelector) error {
	if obj == nil {
		return errors.New("nil BGPPeer")
	}
	obj.Spec.NodeSelectors = append(obj.Spec.NodeSelectors, sel)
	return nil
}

// SetBGPPeerPort sets the peer port on the BGPPeer spec.
func SetBGPPeerPort(obj *metallbv1beta1.BGPPeer, port uint16) error {
	if obj == nil {
		return errors.New("nil BGPPeer")
	}
	obj.Spec.Port = port
	return nil
}

// SetBGPPeerHoldTime sets the hold time on the BGPPeer spec.
func SetBGPPeerHoldTime(obj *metallbv1beta1.BGPPeer, d metav1.Duration) error {
	if obj == nil {
		return errors.New("nil BGPPeer")
	}
	obj.Spec.HoldTime = d
	return nil
}

// SetBGPPeerKeepaliveTime sets the keepalive time on the BGPPeer spec.
func SetBGPPeerKeepaliveTime(obj *metallbv1beta1.BGPPeer, d metav1.Duration) error {
	if obj == nil {
		return errors.New("nil BGPPeer")
	}
	obj.Spec.KeepaliveTime = d
	return nil
}

// SetBGPPeerSrcAddress sets the source address on the BGPPeer spec.
func SetBGPPeerSrcAddress(obj *metallbv1beta1.BGPPeer, addr string) error {
	if obj == nil {
		return errors.New("nil BGPPeer")
	}
	obj.Spec.SrcAddress = addr
	return nil
}

// SetBGPPeerRouterID sets the router ID on the BGPPeer spec.
func SetBGPPeerRouterID(obj *metallbv1beta1.BGPPeer, id string) error {
	if obj == nil {
		return errors.New("nil BGPPeer")
	}
	obj.Spec.RouterID = id
	return nil
}

// SetBGPPeerEBGPMultiHop sets the eBGP multi-hop flag on the BGPPeer spec.
func SetBGPPeerEBGPMultiHop(obj *metallbv1beta1.BGPPeer, multi bool) error {
	if obj == nil {
		return errors.New("nil BGPPeer")
	}
	obj.Spec.EBGPMultiHop = multi
	return nil
}

// SetBGPPeerPassword sets the password on the BGPPeer spec.
func SetBGPPeerPassword(obj *metallbv1beta1.BGPPeer, pw string) error {
	if obj == nil {
		return errors.New("nil BGPPeer")
	}
	obj.Spec.Password = pw
	return nil
}

// SetBGPPeerBFDProfile sets the BFD profile name on the BGPPeer spec.
func SetBGPPeerBFDProfile(obj *metallbv1beta1.BGPPeer, profile string) error {
	if obj == nil {
		return errors.New("nil BGPPeer")
	}
	obj.Spec.BFDProfile = profile
	return nil
}
