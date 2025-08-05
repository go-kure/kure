package io

import (
	"fmt"
	"io"
	"sort"
	"strings"
	"text/tabwriter"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// TableColumn represents a column in table output
type TableColumn struct {
	Header   string
	Width    int
	Accessor func(client.Object) string
	Priority int  // Lower values are shown first, higher values in wide mode
	WideOnly bool // Only shown in wide output
}

// DefaultColumns returns the default column set for Kubernetes resources
func DefaultColumns() []TableColumn {
	return []TableColumn{
		{
			Header:   "NAMESPACE",
			Width:    12,
			Priority: 0,
			Accessor: func(obj client.Object) string {
				ns := obj.GetNamespace()
				if ns == "" {
					return "<none>"
				}
				return ns
			},
		},
		{
			Header:   "NAME",
			Width:    20,
			Priority: 1,
			Accessor: func(obj client.Object) string {
				return obj.GetName()
			},
		},
		{
			Header:   "READY",
			Width:    8,
			Priority: 2,
			Accessor: func(obj client.Object) string {
				return GetResourceStatus(obj)
			},
		},
		{
			Header:   "STATUS",
			Width:    10,
			Priority: 3,
			WideOnly: true,
			Accessor: func(obj client.Object) string {
				// Additional status information for wide output
				return GetDetailedStatus(obj)
			},
		},
		{
			Header:   "AGE",
			Width:    8,
			Priority: 10,
			Accessor: func(obj client.Object) string {
				return GetResourceAge(obj)
			},
		},
	}
}

// KindSpecificColumns returns columns tailored for specific Kubernetes resource kinds
func KindSpecificColumns(gvk metav1.GroupVersionKind) []TableColumn {
	base := DefaultColumns()

	switch strings.ToLower(gvk.Kind) {
	case "pod":
		return podColumns(base)
	case "deployment":
		return deploymentColumns(base)
	case "service":
		return serviceColumns(base)
	case "configmap", "secret":
		return configColumns(base)
	default:
		return base
	}
}

// podColumns customizes columns for Pod resources
func podColumns(base []TableColumn) []TableColumn {
	// Insert pod-specific columns
	columns := make([]TableColumn, 0, len(base)+2)

	for _, col := range base {
		if col.Header == "READY" {
			// Replace generic READY with pod-specific READY
			col.Accessor = func(obj client.Object) string {
				return getPodReadyStatus(obj)
			}
			columns = append(columns, col)

			// Add RESTARTS column after READY
			columns = append(columns, TableColumn{
				Header:   "RESTARTS",
				Width:    8,
				Priority: col.Priority + 1,
				Accessor: func(obj client.Object) string {
					return getPodRestarts(obj)
				},
			})
		} else {
			if col.Priority > 2 {
				col.Priority++ // Shift priorities to make room for RESTARTS
			}
			columns = append(columns, col)
		}
	}

	// Add NODE column at the end for wide output
	columns = append(columns, TableColumn{
		Header:   "NODE",
		Width:    15,
		Priority: 20,
		WideOnly: true,
		Accessor: func(obj client.Object) string {
			return getPodNode(obj)
		},
	})

	return columns
}

// deploymentColumns customizes columns for Deployment resources
func deploymentColumns(base []TableColumn) []TableColumn {
	for i := range base {
		if base[i].Header == "READY" {
			base[i].Accessor = func(obj client.Object) string {
				return getDeploymentReadyStatus(obj)
			}
		}
	}

	// Add REPLICAS column for wide output
	base = append(base, TableColumn{
		Header:   "REPLICAS",
		Width:    12,
		Priority: 15,
		WideOnly: true,
		Accessor: func(obj client.Object) string {
			return getDeploymentReplicas(obj)
		},
	})

	return base
}

// serviceColumns customizes columns for Service resources
func serviceColumns(base []TableColumn) []TableColumn {
	// Insert service-specific columns
	columns := make([]TableColumn, 0, len(base)+2)

	for _, col := range base {
		if col.Header == "READY" {
			// Replace READY with TYPE for services
			columns = append(columns, TableColumn{
				Header:   "TYPE",
				Width:    12,
				Priority: col.Priority,
				Accessor: func(obj client.Object) string {
					return getServiceType(obj)
				},
			})

			// Add CLUSTER-IP after TYPE
			columns = append(columns, TableColumn{
				Header:   "CLUSTER-IP",
				Width:    15,
				Priority: col.Priority + 1,
				Accessor: func(obj client.Object) string {
					return getServiceClusterIP(obj)
				},
			})

			// Add EXTERNAL-IP for wide output
			columns = append(columns, TableColumn{
				Header:   "EXTERNAL-IP",
				Width:    15,
				Priority: col.Priority + 2,
				WideOnly: true,
				Accessor: func(obj client.Object) string {
					return getServiceExternalIP(obj)
				},
			})
		} else {
			if col.Priority > 2 {
				col.Priority += 2 // Shift priorities to make room for new columns
			}
			columns = append(columns, col)
		}
	}

	return columns
}

// configColumns customizes columns for ConfigMap and Secret resources
func configColumns(base []TableColumn) []TableColumn {
	for i := range base {
		if base[i].Header == "READY" {
			// Replace READY with DATA for config resources
			base[i].Header = "DATA"
			base[i].Width = 8
			base[i].Accessor = func(obj client.Object) string {
				return getConfigDataCount(obj)
			}
		}
	}
	return base
}

// SimpleTablePrinter provides a basic table printer implementation without k8s.io/cli-runtime dependency
type SimpleTablePrinter struct {
	columns   []TableColumn
	wide      bool
	noHeaders bool
}

// NewSimpleTablePrinter creates a simple table printer with default columns
func NewSimpleTablePrinter(wide, noHeaders bool) *SimpleTablePrinter {
	return &SimpleTablePrinter{
		columns:   DefaultColumns(),
		wide:      wide,
		noHeaders: noHeaders,
	}
}

// NewSimpleTablePrinterWithColumns creates a simple table printer with custom columns
func NewSimpleTablePrinterWithColumns(columns []TableColumn, wide, noHeaders bool) *SimpleTablePrinter {
	return &SimpleTablePrinter{
		columns:   columns,
		wide:      wide,
		noHeaders: noHeaders,
	}
}

// Print outputs resources in table format using the simple table printer
func (stp *SimpleTablePrinter) Print(resources []*client.Object, w io.Writer) error {
	if len(resources) == 0 {
		return nil
	}

	// Filter columns based on wide mode
	visibleColumns := stp.getVisibleColumns()

	// Create tabwriter for aligned output
	tw := tabwriter.NewWriter(w, 0, 8, 2, ' ', 0)
	defer tw.Flush()

	// Print headers
	if !stp.noHeaders {
		headers := make([]string, len(visibleColumns))
		for i, col := range visibleColumns {
			headers[i] = col.Header
		}
		fmt.Fprintln(tw, strings.Join(headers, "\t"))
	}

	// Sort resources by name for consistent output
	sortedResources := make([]*client.Object, len(resources))
	copy(sortedResources, resources)
	sort.Slice(sortedResources, func(i, j int) bool {
		if sortedResources[i] == nil || sortedResources[j] == nil {
			return false
		}
		iObj := *sortedResources[i]
		jObj := *sortedResources[j]

		// Sort by namespace first, then by name
		if iObj.GetNamespace() != jObj.GetNamespace() {
			return iObj.GetNamespace() < jObj.GetNamespace()
		}
		return iObj.GetName() < jObj.GetName()
	})

	// Print each resource
	for _, resource := range sortedResources {
		if resource == nil {
			continue
		}

		obj := *resource
		row := make([]string, len(visibleColumns))

		for i, col := range visibleColumns {
			row[i] = col.Accessor(obj)
		}

		fmt.Fprintln(tw, strings.Join(row, "\t"))
	}

	return nil
}

// getVisibleColumns returns columns that should be displayed based on wide mode
func (stp *SimpleTablePrinter) getVisibleColumns() []TableColumn {
	var visible []TableColumn

	for _, col := range stp.columns {
		if !col.WideOnly || stp.wide {
			visible = append(visible, col)
		}
	}

	// Sort by priority
	sort.Slice(visible, func(i, j int) bool {
		return visible[i].Priority < visible[j].Priority
	})

	return visible
}

// GetDetailedStatus provides additional status information for wide output
func GetDetailedStatus(obj client.Object) string {
	if obj == nil {
		return "Unknown"
	}

	// Try to get more detailed status information
	unstructured, ok := obj.(runtime.Unstructured)
	if !ok {
		return "Unknown"
	}

	statusVal, found := unstructured.UnstructuredContent()["status"]
	if !found {
		return "Unknown"
	}

	statusMap, ok := statusVal.(map[string]interface{})
	if !ok {
		return "Unknown"
	}

	// Look for detailed status fields
	var details []string

	if message, ok := statusMap["message"].(string); ok && message != "" {
		details = append(details, message)
	}

	if reason, ok := statusMap["reason"].(string); ok && reason != "" {
		details = append(details, reason)
	}

	if len(details) > 0 {
		return strings.Join(details, ", ")
	}

	return GetResourceStatus(obj)
}

// Helper functions for resource-specific column accessors

func getPodReadyStatus(obj client.Object) string {
	unstructured, ok := obj.(runtime.Unstructured)
	if !ok {
		return "Unknown"
	}

	statusVal, found := unstructured.UnstructuredContent()["status"]
	if !found {
		return "0/0"
	}

	statusMap, ok := statusVal.(map[string]interface{})
	if !ok {
		return "0/0"
	}

	// Check container statuses
	if containerStatuses, ok := statusMap["containerStatuses"].([]interface{}); ok {
		ready := 0
		total := len(containerStatuses)

		for _, cs := range containerStatuses {
			if csMap, ok := cs.(map[string]interface{}); ok {
				if isReady, ok := csMap["ready"].(bool); ok && isReady {
					ready++
				}
			}
		}

		return fmt.Sprintf("%d/%d", ready, total)
	}

	return "0/0"
}

func getPodRestarts(obj client.Object) string {
	unstructured, ok := obj.(runtime.Unstructured)
	if !ok {
		return "0"
	}

	statusVal, found := unstructured.UnstructuredContent()["status"]
	if !found {
		return "0"
	}

	statusMap, ok := statusVal.(map[string]interface{})
	if !ok {
		return "0"
	}

	// Sum restart counts from container statuses
	if containerStatuses, ok := statusMap["containerStatuses"].([]interface{}); ok {
		totalRestarts := 0

		for _, cs := range containerStatuses {
			if csMap, ok := cs.(map[string]interface{}); ok {
				if restartCount, ok := csMap["restartCount"].(float64); ok {
					totalRestarts += int(restartCount)
				}
			}
		}

		return fmt.Sprintf("%d", totalRestarts)
	}

	return "0"
}

func getPodNode(obj client.Object) string {
	unstructured, ok := obj.(runtime.Unstructured)
	if !ok {
		return "<none>"
	}

	specVal, found := unstructured.UnstructuredContent()["spec"]
	if !found {
		return "<none>"
	}

	specMap, ok := specVal.(map[string]interface{})
	if !ok {
		return "<none>"
	}

	if nodeName, ok := specMap["nodeName"].(string); ok && nodeName != "" {
		return nodeName
	}

	return "<none>"
}

func getDeploymentReadyStatus(obj client.Object) string {
	unstructured, ok := obj.(runtime.Unstructured)
	if !ok {
		return "0/0"
	}

	statusVal, found := unstructured.UnstructuredContent()["status"]
	if !found {
		return "0/0"
	}

	statusMap, ok := statusVal.(map[string]interface{})
	if !ok {
		return "0/0"
	}

	ready := int64(0)
	if readyReplicas, ok := statusMap["readyReplicas"].(float64); ok {
		ready = int64(readyReplicas)
	}

	desired := int64(0)
	if replicas, ok := statusMap["replicas"].(float64); ok {
		desired = int64(replicas)
	}

	return fmt.Sprintf("%d/%d", ready, desired)
}

func getDeploymentReplicas(obj client.Object) string {
	unstructured, ok := obj.(runtime.Unstructured)
	if !ok {
		return "0"
	}

	specVal, found := unstructured.UnstructuredContent()["spec"]
	if !found {
		return "0"
	}

	specMap, ok := specVal.(map[string]interface{})
	if !ok {
		return "0"
	}

	if replicas, ok := specMap["replicas"].(float64); ok {
		return fmt.Sprintf("%.0f", replicas)
	}

	return "1" // Default replica count
}

func getServiceType(obj client.Object) string {
	unstructured, ok := obj.(runtime.Unstructured)
	if !ok {
		return "Unknown"
	}

	specVal, found := unstructured.UnstructuredContent()["spec"]
	if !found {
		return "ClusterIP" // Default type
	}

	specMap, ok := specVal.(map[string]interface{})
	if !ok {
		return "ClusterIP"
	}

	if serviceType, ok := specMap["type"].(string); ok {
		return serviceType
	}

	return "ClusterIP"
}

func getServiceClusterIP(obj client.Object) string {
	unstructured, ok := obj.(runtime.Unstructured)
	if !ok {
		return "<none>"
	}

	specVal, found := unstructured.UnstructuredContent()["spec"]
	if !found {
		return "<none>"
	}

	specMap, ok := specVal.(map[string]interface{})
	if !ok {
		return "<none>"
	}

	if clusterIP, ok := specMap["clusterIP"].(string); ok && clusterIP != "" {
		return clusterIP
	}

	return "<none>"
}

func getServiceExternalIP(obj client.Object) string {
	unstructured, ok := obj.(runtime.Unstructured)
	if !ok {
		return "<none>"
	}

	statusVal, found := unstructured.UnstructuredContent()["status"]
	if found {
		if statusMap, ok := statusVal.(map[string]interface{}); ok {
			if lb, ok := statusMap["loadBalancer"].(map[string]interface{}); ok {
				if ingress, ok := lb["ingress"].([]interface{}); ok && len(ingress) > 0 {
					if ingressMap, ok := ingress[0].(map[string]interface{}); ok {
						if ip, ok := ingressMap["ip"].(string); ok && ip != "" {
							return ip
						}
						if hostname, ok := ingressMap["hostname"].(string); ok && hostname != "" {
							return hostname
						}
					}
				}
			}
		}
	}

	// Check spec for external IPs
	specVal, found := unstructured.UnstructuredContent()["spec"]
	if found {
		if specMap, ok := specVal.(map[string]interface{}); ok {
			if externalIPs, ok := specMap["externalIPs"].([]interface{}); ok && len(externalIPs) > 0 {
				if ip, ok := externalIPs[0].(string); ok {
					return ip
				}
			}
		}
	}

	return "<none>"
}

func getConfigDataCount(obj client.Object) string {
	unstructured, ok := obj.(runtime.Unstructured)
	if !ok {
		return "0"
	}

	dataVal, found := unstructured.UnstructuredContent()["data"]
	if !found {
		return "0"
	}

	if dataMap, ok := dataVal.(map[string]interface{}); ok {
		return fmt.Sprintf("%d", len(dataMap))
	}

	return "0"
}
