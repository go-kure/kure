// Package prometheus exposes the generated constructors and the admissible
// sugar for Prometheus operator resources: ServiceMonitor, PodMonitor,
// PrometheusRule and the other monitoring.coreos.com kinds. Each constructor
// returns a controller-runtime object carrying identity only; the upstream
// monitoringv1 struct is the construction API.
//
// ## Constructors
//
// Create<Kind> is generated from the registered scheme and emits apiVersion,
// kind, metadata.name and metadata.namespace. Spec fields are the caller's own
// assignments:
//
//	sm := prometheus.CreateServiceMonitor("my-app", "monitoring")
//	sm.Spec = monitoringv1.ServiceMonitorSpec{
//	        Selector:  metav1.LabelSelector{MatchLabels: map[string]string{"app": "my-app"}},
//	        Endpoints: []monitoringv1.Endpoint{{Port: "metrics"}},
//	}
//
// ## Update helpers
//
// Functions prefixed with Set or Add write exactly the one field they name
// and fall into the builder contract's admitted classes: the Add* helpers
// append, the Set*SampleLimit and SetRuleGroupInterval helpers assign a
// pointer-typed field. ObjectMeta labels and annotations use the generic
// kubernetes.AddLabel / kubernetes.AddAnnotation; the *TargetLabel appenders
// here write the scrape spec's own target-label lists, which are different
// fields. CreateRuleGroup is the one hand-written sub-type constructor that
// remains, because a rule group is the unit a caller assembles repeatedly.
//
// The config-struct layer this package used to carry — ServiceMonitor,
// PodMonitor and PrometheusRule taking a *Config — was retired by release 2
// of the builder contract; see docs/builder-contract-release-2.md for the
// field-by-field mapping.
package prometheus
