package fluxcd

import (
	"errors"

	fluxv1 "github.com/controlplaneio-fluxcd/flux-operator/api/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// CreateFluxReport returns a new FluxReport object.
func CreateFluxReport(name, namespace string, spec fluxv1.FluxReportSpec) *fluxv1.FluxReport {
	obj := &fluxv1.FluxReport{
		TypeMeta: metav1.TypeMeta{
			Kind:       fluxv1.FluxReportKind,
			APIVersion: fluxv1.GroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: spec,
	}
	return obj
}

// SetFluxReportDistribution sets the distribution status.
func SetFluxReportDistribution(fr *fluxv1.FluxReport, dist fluxv1.FluxDistributionStatus) error {
	if fr == nil {
		return errors.New("nil FluxReport")
	}
	fr.Spec.Distribution = dist
	return nil
}

// SetFluxReportCluster sets the cluster info.
func SetFluxReportCluster(fr *fluxv1.FluxReport, c *fluxv1.ClusterInfo) error {
	if fr == nil {
		return errors.New("nil FluxReport")
	}
	fr.Spec.Cluster = c
	return nil
}

// SetFluxReportOperator sets the operator info.
func SetFluxReportOperator(fr *fluxv1.FluxReport, op *fluxv1.OperatorInfo) error {
	if fr == nil {
		return errors.New("nil FluxReport")
	}
	fr.Spec.Operator = op
	return nil
}

// AddFluxReportComponentStatus appends a component status.
func AddFluxReportComponentStatus(fr *fluxv1.FluxReport, cs fluxv1.FluxComponentStatus) error {
	if fr == nil {
		return errors.New("nil FluxReport")
	}
	fr.Spec.ComponentsStatus = append(fr.Spec.ComponentsStatus, cs)
	return nil
}

// AddFluxReportReconcilerStatus appends a reconciler status.
func AddFluxReportReconcilerStatus(fr *fluxv1.FluxReport, rs fluxv1.FluxReconcilerStatus) error {
	if fr == nil {
		return errors.New("nil FluxReport")
	}
	fr.Spec.ReconcilersStatus = append(fr.Spec.ReconcilersStatus, rs)
	return nil
}

// SetFluxReportSyncStatus sets the sync status.
func SetFluxReportSyncStatus(fr *fluxv1.FluxReport, s *fluxv1.FluxSyncStatus) error {
	if fr == nil {
		return errors.New("nil FluxReport")
	}
	fr.Spec.SyncStatus = s
	return nil
}
