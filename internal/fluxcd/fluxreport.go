package fluxcd

import (
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
func SetFluxReportDistribution(fr *fluxv1.FluxReport, dist fluxv1.FluxDistributionStatus) {
	fr.Spec.Distribution = dist
}

// SetFluxReportCluster sets the cluster info.
func SetFluxReportCluster(fr *fluxv1.FluxReport, c *fluxv1.ClusterInfo) {
	fr.Spec.Cluster = c
}

// SetFluxReportOperator sets the operator info.
func SetFluxReportOperator(fr *fluxv1.FluxReport, op *fluxv1.OperatorInfo) {
	fr.Spec.Operator = op
}

// AddFluxReportComponentStatus appends a component status.
func AddFluxReportComponentStatus(fr *fluxv1.FluxReport, cs fluxv1.FluxComponentStatus) {
	fr.Spec.ComponentsStatus = append(fr.Spec.ComponentsStatus, cs)
}

// AddFluxReportReconcilerStatus appends a reconciler status.
func AddFluxReportReconcilerStatus(fr *fluxv1.FluxReport, rs fluxv1.FluxReconcilerStatus) {
	fr.Spec.ReconcilersStatus = append(fr.Spec.ReconcilersStatus, rs)
}

// SetFluxReportSyncStatus sets the sync status.
func SetFluxReportSyncStatus(fr *fluxv1.FluxReport, s *fluxv1.FluxSyncStatus) {
	fr.Spec.SyncStatus = s
}
