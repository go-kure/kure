// Package prometheus exposes helper functions for constructing Prometheus
// operator resources.  Each function returns a fully initialized
// controller-runtime object that can be serialized to YAML or modified further
// by the calling application.
//
// ## Overview
//
// The package mirrors the constructors and setters found under
// `internal/prometheus` so applications can build Prometheus operator manifests
// programmatically without depending on the internal packages directly.
//
// Resources covered include [ServiceMonitor], [PodMonitor], and
// [PrometheusRule].
//
// ## Constructors
//
// Constructors accept configuration structs and return fully initialized
// resources.  A minimal example creating a ServiceMonitor looks like:
//
//	sm := prometheus.ServiceMonitor(&prometheus.ServiceMonitorConfig{
//	        Name:      "my-app",
//	        Namespace: "monitoring",
//	        Selector:  metav1.LabelSelector{MatchLabels: map[string]string{"app": "my-app"}},
//	        Endpoints: []monitoringv1.Endpoint{{Port: "metrics"}},
//	})
//
// ## Update helpers
//
// Additional functions prefixed with `Set` or `Add` expose granular control
// over the generated objects.  They delegate to the internal package to
// perform the actual mutations while keeping the public API stable.
package prometheus
