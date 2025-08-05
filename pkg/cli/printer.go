package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"text/tabwriter"

	"gopkg.in/yaml.v3"
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/go-kure/kure/pkg/cmd/shared/options"
)

// Printer interface for outputting resources
type Printer interface {
	Print(objects []runtime.Object, writer io.Writer) error
}

// PrinterOptions contains options for printing
type PrinterOptions struct {
	OutputFormat string
	NoHeaders    bool
	ShowLabels   bool
	Wide         bool
}

// NewPrinter creates a new printer based on global options
func NewPrinter(globalOpts *options.GlobalOptions) Printer {
	opts := PrinterOptions{
		OutputFormat: globalOpts.Output,
		NoHeaders:    globalOpts.NoHeaders,
		ShowLabels:   globalOpts.ShowLabels,
		Wide:         globalOpts.Wide,
	}

	switch globalOpts.Output {
	case "json":
		return &jsonPrinter{}
	case "table", "wide":
		return &tablePrinter{options: opts}
	case "name":
		return &namePrinter{}
	default:
		return &yamlPrinter{}
	}
}

// yamlPrinter prints resources as YAML
type yamlPrinter struct{}

func (p *yamlPrinter) Print(objects []runtime.Object, writer io.Writer) error {
	if len(objects) == 0 {
		return nil
	}

	for i, obj := range objects {
		if i > 0 {
			fmt.Fprintln(writer, "---")
		}

		data, err := yaml.Marshal(obj)
		if err != nil {
			return fmt.Errorf("failed to marshal object to YAML: %w", err)
		}

		if _, err := writer.Write(data); err != nil {
			return fmt.Errorf("failed to write YAML: %w", err)
		}
	}

	return nil
}

// jsonPrinter prints resources as JSON
type jsonPrinter struct{}

func (p *jsonPrinter) Print(objects []runtime.Object, writer io.Writer) error {
	if len(objects) == 0 {
		fmt.Fprint(writer, "{}\n")
		return nil
	}

	if len(objects) == 1 {
		return json.NewEncoder(writer).Encode(objects[0])
	}

	// Multiple objects - wrap in array
	return json.NewEncoder(writer).Encode(objects)
}

// tablePrinter prints resources in table format
type tablePrinter struct {
	options PrinterOptions
}

func (p *tablePrinter) Print(objects []runtime.Object, writer io.Writer) error {
	if len(objects) == 0 {
		return nil
	}

	w := tabwriter.NewWriter(writer, 0, 0, 2, ' ', 0)
	defer w.Flush()

	// Print headers
	if !p.options.NoHeaders {
		headers := []string{"NAME", "KIND", "NAMESPACE"}
		if p.options.Wide {
			headers = append(headers, "API-VERSION", "CREATED")
		}
		if p.options.ShowLabels {
			headers = append(headers, "LABELS")
		}
		fmt.Fprintln(w, joinTabs(headers))
	}

	// Print objects
	for _, obj := range objects {
		if unstructuredObj, ok := obj.(interface {
			GetName() string
			GetKind() string
			GetNamespace() string
			GetAPIVersion() string
			GetLabels() map[string]string
		}); ok {
			row := []string{
				unstructuredObj.GetName(),
				unstructuredObj.GetKind(),
				unstructuredObj.GetNamespace(),
			}

			if p.options.Wide {
				row = append(row, unstructuredObj.GetAPIVersion(), "<unknown>")
			}

			if p.options.ShowLabels {
				labels := formatLabels(unstructuredObj.GetLabels())
				row = append(row, labels)
			}

			fmt.Fprintln(w, joinTabs(row))
		}
	}

	return nil
}

// namePrinter prints only resource names
type namePrinter struct{}

func (p *namePrinter) Print(objects []runtime.Object, writer io.Writer) error {
	for _, obj := range objects {
		if namedObj, ok := obj.(interface {
			GetName() string
			GetKind() string
		}); ok {
			fmt.Fprintf(writer, "%s/%s\n", namedObj.GetKind(), namedObj.GetName())
		}
	}
	return nil
}

// Helper functions
func joinTabs(strs []string) string {
	if len(strs) == 0 {
		return ""
	}

	result := strs[0]
	for _, s := range strs[1:] {
		result += "\t" + s
	}
	return result
}

func formatLabels(labels map[string]string) string {
	if len(labels) == 0 {
		return "<none>"
	}

	result := ""
	first := true
	for k, v := range labels {
		if !first {
			result += ","
		}
		result += k + "=" + v
		first = false
	}
	return result
}

// PrintObjects is a convenience function for printing objects
func PrintObjects(objects []runtime.Object, globalOpts *options.GlobalOptions, writer io.Writer) error {
	printer := NewPrinter(globalOpts)
	return printer.Print(objects, writer)
}
