package cilium

import (
	ciliumv2 "github.com/cilium/cilium/pkg/k8s/apis/cilium.io/v2"
	"github.com/cilium/cilium/pkg/policy/api"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// CreateCiliumCIDRGroup returns a new CiliumCIDRGroup with TypeMeta and
// ObjectMeta set. ExternalCIDRs is initialised to an empty slice.
func CreateCiliumCIDRGroup(name string) *ciliumv2.CiliumCIDRGroup {
	return &ciliumv2.CiliumCIDRGroup{
		TypeMeta: metav1.TypeMeta{
			Kind:       ciliumv2.CCGKindDefinition,
			APIVersion: ciliumv2.SchemeGroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Spec: ciliumv2.CiliumCIDRGroupSpec{
			ExternalCIDRs: []api.CIDR{},
		},
	}
}

// AddCiliumCIDRGroupCIDR appends a CIDR to the group's ExternalCIDRs list.
func AddCiliumCIDRGroupCIDR(obj *ciliumv2.CiliumCIDRGroup, cidr api.CIDR) {
	obj.Spec.ExternalCIDRs = append(obj.Spec.ExternalCIDRs, cidr)
}

// SetCiliumCIDRGroupCIDRs replaces the ExternalCIDRs list on the group.
func SetCiliumCIDRGroupCIDRs(obj *ciliumv2.CiliumCIDRGroup, cidrs []api.CIDR) {
	obj.Spec.ExternalCIDRs = cidrs
}
