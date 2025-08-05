package io

import (
	"fmt"
	"io"
	"strings"
	"time"

	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/cli-runtime/pkg/printers"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// OutputFormat represents the supported output formats for printing resources
type OutputFormat string

const (
	OutputFormatYAML  OutputFormat = "yaml"
	OutputFormatJSON  OutputFormat = "json"
	OutputFormatTable OutputFormat = "table"
	OutputFormatWide  OutputFormat = "wide"
	OutputFormatName  OutputFormat = "name"
)

// PrintOptions contains configuration for resource printing
type PrintOptions struct {
	// OutputFormat specifies the desired output format
	OutputFormat OutputFormat
	// NoHeaders suppresses table headers when true
	NoHeaders bool
	// ShowLabels displays resource labels in table output
	ShowLabels bool
	// ColumnLabels is a list of label keys to display as columns
	ColumnLabels []string
	// SortBy specifies the column to sort by (for table output)
	SortBy string
}

// ResourcePrinter provides a unified interface for printing Kubernetes resources
// in various formats, compatible with kubectl output styles
type ResourcePrinter struct {
	options PrintOptions
}

// NewResourcePrinter creates a new ResourcePrinter with the given options
func NewResourcePrinter(options PrintOptions) *ResourcePrinter {
	return &ResourcePrinter{
		options: options,
	}
}

// Print outputs the given resources to the writer using the configured format
func (rp *ResourcePrinter) Print(resources []*client.Object, w io.Writer) error {
	if len(resources) == 0 {
		return nil
	}

	switch rp.options.OutputFormat {
	case OutputFormatYAML:
		return rp.printYAML(resources, w)
	case OutputFormatJSON:
		return rp.printJSON(resources, w)
	case OutputFormatTable:
		return rp.printTable(resources, w, false)
	case OutputFormatWide:
		return rp.printTable(resources, w, true)
	case OutputFormatName:
		return rp.printNames(resources, w)
	default:
		return fmt.Errorf("unsupported output format: %s", rp.options.OutputFormat)
	}
}

// printYAML outputs resources in YAML format
func (rp *ResourcePrinter) printYAML(resources []*client.Object, w io.Writer) error {
	data, err := EncodeObjectsToYAML(resources)
	if err != nil {
		return fmt.Errorf("encode to YAML: %w", err)
	}
	_, err = w.Write(data)
	return err
}

// printJSON outputs resources in JSON format
func (rp *ResourcePrinter) printJSON(resources []*client.Object, w io.Writer) error {
	data, err := EncodeObjectsToJSON(resources)
	if err != nil {
		return fmt.Errorf("encode to JSON: %w", err)
	}
	_, err = w.Write(data)
	return err
}

// printNames outputs resource names in kubectl-compatible format
func (rp *ResourcePrinter) printNames(resources []*client.Object, w io.Writer) error {
	for _, obj := range resources {
		if obj == nil {
			continue
		}
		
		gvk := (*obj).GetObjectKind().GroupVersionKind()
		name := (*obj).GetName()
		namespace := (*obj).GetNamespace()
		
		// Format: kind/name or kind.group/name
		kind := strings.ToLower(gvk.Kind)
		if gvk.Group != "" {
			kind = fmt.Sprintf("%s.%s", kind, gvk.Group)
		}
		
		resourceName := fmt.Sprintf("%s/%s", kind, name)
		if namespace != "" {
			fmt.Fprintf(w, "%s (namespace: %s)\n", resourceName, namespace)
		} else {
			fmt.Fprintf(w, "%s\n", resourceName)
		}
	}
	return nil
}

// printTable outputs resources in table format using k8s.io/cli-runtime/pkg/printers
func (rp *ResourcePrinter) printTable(resources []*client.Object, w io.Writer, wide bool) error {
	if len(resources) == 0 {
		return nil
	}

	// Create a table printer
	printer := printers.NewTablePrinter(printers.PrintOptions{
		NoHeaders:    rp.options.NoHeaders,
		WithNamespace: true,
		Wide:         wide,
		ShowLabels:   rp.options.ShowLabels,
		ColumnLabels: rp.options.ColumnLabels,
	})

	// Convert resources to runtime objects for the table printer
	runtimeObjects := make([]runtime.Object, 0, len(resources))
	for _, obj := range resources {
		if obj != nil {
			runtimeObjects = append(runtimeObjects, *obj)
		}
	}

	// Print each resource
	for _, obj := range runtimeObjects {
		if err := printer.PrintObj(obj, w); err != nil {
			return fmt.Errorf("print object: %w", err)
		}
	}

	return nil
}

// PrintSingle is a convenience function for printing a single resource
func (rp *ResourcePrinter) PrintSingle(resource client.Object, w io.Writer) error {
	if resource == nil {
		return nil
	}
	return rp.Print([]*client.Object{&resource}, w)
}

// FormatAge returns a human-readable age string for a resource
func FormatAge(t *metav1.Time) string {
	if t == nil {
		return "<unknown>"
	}
	
	duration := time.Since(t.Time)
	if duration.Seconds() < 60 {
		return fmt.Sprintf("%.0fs", duration.Seconds())
	}
	if duration.Minutes() < 60 {
		return fmt.Sprintf("%.0fm", duration.Minutes())
	}
	if duration.Hours() < 24 {
		return fmt.Sprintf("%.0fh", duration.Hours())
	}
	return fmt.Sprintf("%.0fd", duration.Hours()/24)
}

// GetResourceAge returns the age of a resource based on its creation timestamp
func GetResourceAge(obj client.Object) string {
	if obj == nil {
		return "<unknown>"
	}
	
	accessor, err := meta.Accessor(obj)
	if err != nil {
		return "<unknown>"
	}
	
	creationTime := accessor.GetCreationTimestamp()
	return FormatAge(&creationTime)
}

// GetResourceStatus attempts to extract a status string from common status fields
func GetResourceStatus(obj client.Object) string {
	if obj == nil {
		return "Unknown"
	}

	// Try to get status from common fields using unstructured access
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

	// Look for common status indicators
	if phase, ok := statusMap["phase"]; ok {
		if phaseStr, ok := phase.(string); ok {
			return phaseStr
		}
	}

	if ready, ok := statusMap["ready"]; ok {
		if readyBool, ok := ready.(bool); ok {
			if readyBool {
				return "Ready"
			}
			return "NotReady"
		}
	}

	// Check conditions for Ready status
	if conditions, ok := statusMap["conditions"]; ok {
		if conditionsSlice, ok := conditions.([]interface{}); ok {
			for _, condition := range conditionsSlice {
				if condMap, ok := condition.(map[string]interface{}); ok {
					if condType, ok := condMap["type"].(string); ok && condType == "Ready" {
						if statusVal, ok := condMap["status"].(string); ok {
							if statusVal == "True" {
								return "Ready"
							}
							return "NotReady"
						}
					}
				}
			}
		}
	}

	return "Unknown"
}

// CreateTablePrinter creates a table printer configured for Kure resources
func CreateTablePrinter(options PrintOptions) *ResourcePrinter {
	// Ensure table format for table printer
	if options.OutputFormat != OutputFormatTable && options.OutputFormat != OutputFormatWide {
		options.OutputFormat = OutputFormatTable
	}
	
	return NewResourcePrinter(options)
}

// PrintToString returns the printed output as a string instead of writing to a writer
func (rp *ResourcePrinter) PrintToString(resources []*client.Object) (string, error) {
	var buf strings.Builder
	err := rp.Print(resources, &buf)
	return buf.String(), err
}

// ValidateOutputFormat checks if the given format string is valid
func ValidateOutputFormat(format string) (OutputFormat, error) {
	switch strings.ToLower(format) {
	case "yaml", "yml":
		return OutputFormatYAML, nil
	case "json":
		return OutputFormatJSON, nil
	case "table":
		return OutputFormatTable, nil
	case "wide":
		return OutputFormatWide, nil
	case "name":
		return OutputFormatName, nil
	default:
		return "", fmt.Errorf("unsupported output format: %s. Supported formats: yaml, json, table, wide, name", format)
	}
}