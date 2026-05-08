package metallb

import (
	metallbv1beta1 "go.universe.tf/metallb/api/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// CreateIPAddressPool returns a new IPAddressPool with TypeMeta and ObjectMeta set.
func CreateIPAddressPool(name, namespace string) *metallbv1beta1.IPAddressPool {
	return &metallbv1beta1.IPAddressPool{
		TypeMeta: metav1.TypeMeta{
			Kind:       "IPAddressPool",
			APIVersion: metallbv1beta1.GroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
	}
}

// CreateBGPPeer returns a new BGPPeer with TypeMeta and ObjectMeta set.
func CreateBGPPeer(name, namespace string) *metallbv1beta1.BGPPeer {
	return &metallbv1beta1.BGPPeer{
		TypeMeta: metav1.TypeMeta{
			Kind:       "BGPPeer",
			APIVersion: metallbv1beta1.GroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
	}
}

// CreateBGPAdvertisement returns a new BGPAdvertisement with TypeMeta and ObjectMeta set.
func CreateBGPAdvertisement(name, namespace string) *metallbv1beta1.BGPAdvertisement {
	return &metallbv1beta1.BGPAdvertisement{
		TypeMeta: metav1.TypeMeta{
			Kind:       "BGPAdvertisement",
			APIVersion: metallbv1beta1.GroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
	}
}

// CreateL2Advertisement returns a new L2Advertisement with TypeMeta and ObjectMeta set.
func CreateL2Advertisement(name, namespace string) *metallbv1beta1.L2Advertisement {
	return &metallbv1beta1.L2Advertisement{
		TypeMeta: metav1.TypeMeta{
			Kind:       "L2Advertisement",
			APIVersion: metallbv1beta1.GroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
	}
}

// CreateBFDProfile returns a new BFDProfile with TypeMeta and ObjectMeta set.
func CreateBFDProfile(name, namespace string) *metallbv1beta1.BFDProfile {
	return &metallbv1beta1.BFDProfile{
		TypeMeta: metav1.TypeMeta{
			Kind:       "BFDProfile",
			APIVersion: metallbv1beta1.GroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
	}
}
