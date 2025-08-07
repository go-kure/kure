package launcher

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/go-kure/kure/pkg/errors"
	"github.com/go-kure/kure/pkg/logger"
	"gopkg.in/yaml.v3"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// outputBuilder implements the Builder interface
type outputBuilder struct {
	logger   logger.Logger
	writer   FileWriter
	resolver Resolver
	processor PatchProcessor
	outputWriter io.Writer // configurable output writer
}

// NewBuilder creates a new output builder
func NewBuilder(log logger.Logger) Builder {
	if log == nil {
		log = logger.Default()
	}
	return &outputBuilder{
		logger:       log,
		writer:       &defaultFileWriter{},
		resolver:     NewResolver(log),
		processor:    NewPatchProcessor(log, nil),
		outputWriter: os.Stdout, // default to stdout
	}
}

// Build generates final manifests and writes them according to options
func (b *outputBuilder) Build(ctx context.Context, inst *PackageInstance, buildOpts BuildOptions, opts *LauncherOptions) error {
	// Check context cancellation
	select {
	case <-ctx.Done():
		return errors.Wrap(ctx.Err(), "context cancelled during build")
	default:
	}

	if inst == nil || inst.Definition == nil {
		return errors.New("package instance or definition is nil")
	}

	if opts == nil {
		opts = DefaultOptions()
	}

	b.logger.Info("Building package %s", inst.Definition.Metadata.Name)

	// Step 1: Resolve variables
	resolved, err := b.resolver.Resolve(ctx, inst.Definition.Parameters, inst.UserValues, opts)
	if err != nil {
		return errors.Wrap(err, "failed to resolve variables")
	}

	// Convert resolved parameters back to regular map for patch processing
	resolvedParams := make(ParameterMap)
	for k, v := range resolved {
		resolvedParams[k] = v.Value
	}

	// Step 2: Apply patches
	patched, err := b.processor.ApplyPatches(ctx, inst.Definition, inst.EnabledPatches, resolvedParams)
	if err != nil {
		return errors.Wrap(err, "failed to apply patches")
	}

	// Step 3: Build final resources
	resources, err := b.buildResources(ctx, patched, resolvedParams, buildOpts)
	if err != nil {
		return errors.Wrap(err, "failed to build resources")
	}

	// Step 4: Write output
	if err := b.writeOutput(ctx, resources, buildOpts); err != nil {
		return errors.Wrap(err, "failed to write output")
	}

	b.logger.Info("Successfully built %d resources", len(resources))
	return nil
}

// SetOutputWriter sets the output writer for stdout output
func (b *outputBuilder) SetOutputWriter(w io.Writer) {
	b.outputWriter = w
}

// buildResources converts package resources to final manifests
func (b *outputBuilder) buildResources(ctx context.Context, def *PackageDefinition, params ParameterMap, opts BuildOptions) ([]*unstructured.Unstructured, error) {
	var result []*unstructured.Unstructured

	for _, resource := range def.Resources {
		// Check context cancellation
		select {
		case <-ctx.Done():
			return nil, errors.Wrap(ctx.Err(), "context cancelled during resource building")
		default:
		}

		// Skip if filtering is enabled and resource doesn't match
		if opts.FilterKind != "" && resource.Kind != opts.FilterKind {
			continue
		}
		if opts.FilterName != "" && resource.Metadata.Name != opts.FilterName {
			continue
		}
		if opts.FilterNamespace != "" && resource.Metadata.Namespace != opts.FilterNamespace {
			continue
		}

		// Convert to unstructured
		obj := resource.Raw
		if obj == nil {
			// Create from metadata if Raw is nil
			obj = &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": resource.APIVersion,
					"kind":       resource.Kind,
					"metadata": map[string]interface{}{
						"name":        resource.Metadata.Name,
						"namespace":   resource.Metadata.Namespace,
						"labels":      resource.Metadata.Labels,
						"annotations": resource.Metadata.Annotations,
					},
				},
			}
		}

		// Apply any final transformations
		if opts.AddLabels != nil {
			labels := obj.GetLabels()
			if labels == nil {
				labels = make(map[string]string)
			}
			for k, v := range opts.AddLabels {
				labels[k] = v
			}
			obj.SetLabels(labels)
		}

		if opts.AddAnnotations != nil {
			annotations := obj.GetAnnotations()
			if annotations == nil {
				annotations = make(map[string]string)
			}
			for k, v := range opts.AddAnnotations {
				annotations[k] = v
			}
			obj.SetAnnotations(annotations)
		}

		result = append(result, obj)
	}

	return result, nil
}

// writeOutput writes resources to the specified destination
func (b *outputBuilder) writeOutput(ctx context.Context, resources []*unstructured.Unstructured, opts BuildOptions) error {
	// Determine output writer
	var writer io.Writer
	var closeFunc func() error

	switch opts.Output {
	case OutputStdout:
		writer = b.outputWriter
	case OutputFile:
		if opts.OutputPath == "" {
			return errors.New("output path required for file output")
		}

		// Create directory if needed
		dir := filepath.Dir(opts.OutputPath)
		if err := b.writer.MkdirAll(dir); err != nil {
			return errors.Wrap(err, "failed to create output directory")
		}

		// Open file for writing
		file, err := os.Create(opts.OutputPath)
		if err != nil {
			return errors.NewFileError("create", opts.OutputPath, "failed to create output file", err)
		}
		writer = file
		closeFunc = file.Close
	case OutputDirectory:
		// Write each resource to a separate file
		return b.writeDirectory(ctx, resources, opts)
	default:
		return errors.Errorf("unsupported output type: %s", opts.Output)
	}

	// Ensure file is closed if needed
	if closeFunc != nil {
		defer closeFunc()
	}

	// Write resources
	switch opts.Format {
	case FormatYAML:
		return b.writeYAML(writer, resources, opts)
	case FormatJSON:
		return b.writeJSON(writer, resources, opts)
	default:
		return errors.Errorf("unsupported format: %s", opts.Format)
	}
}

// writeYAML writes resources in YAML format
func (b *outputBuilder) writeYAML(w io.Writer, resources []*unstructured.Unstructured, opts BuildOptions) error {
	encoder := yaml.NewEncoder(w)
	encoder.SetIndent(2)

	for i, resource := range resources {
		// Add document separator for multi-document YAML
		if i > 0 && !opts.SeparateFiles {
			if _, err := fmt.Fprintln(w, "---"); err != nil {
				return errors.Wrap(err, "failed to write separator")
			}
		}

		// Convert to YAML
		data := resource.Object
		if err := encoder.Encode(data); err != nil {
			return errors.Wrap(err, "failed to encode resource to YAML")
		}
	}

	return nil
}

// writeJSON writes resources in JSON format
func (b *outputBuilder) writeJSON(w io.Writer, resources []*unstructured.Unstructured, opts BuildOptions) error {
	encoder := json.NewEncoder(w)
	if opts.PrettyPrint {
		encoder.SetIndent("", "  ")
	}

	// Wrap in array if multiple resources
	if len(resources) > 1 && !opts.SeparateFiles {
		var items []interface{}
		for _, resource := range resources {
			items = append(items, resource.Object)
		}
		return encoder.Encode(items)
	}

	// Single resource or separate files
	for _, resource := range resources {
		if err := encoder.Encode(resource.Object); err != nil {
			return errors.Wrap(err, "failed to encode resource to JSON")
		}
	}

	return nil
}

// writeDirectory writes each resource to a separate file
func (b *outputBuilder) writeDirectory(ctx context.Context, resources []*unstructured.Unstructured, opts BuildOptions) error {
	if opts.OutputPath == "" {
		return errors.New("output path required for directory output")
	}

	// Create output directory
	if err := b.writer.MkdirAll(opts.OutputPath); err != nil {
		return errors.Wrap(err, "failed to create output directory")
	}

	for i, resource := range resources {
		// Check context cancellation
		select {
		case <-ctx.Done():
			return errors.Wrap(ctx.Err(), "context cancelled during directory write")
		default:
		}

		// Generate filename
		filename := b.generateFilename(resource, i, opts)
		filepath := filepath.Join(opts.OutputPath, filename)

		// Write resource to file
		file, err := os.Create(filepath)
		if err != nil {
			return errors.NewFileError("create", filepath, "failed to create resource file", err)
		}

		// Write content
		var writeErr error
		switch opts.Format {
		case FormatYAML:
			writeErr = b.writeYAML(file, []*unstructured.Unstructured{resource}, opts)
		case FormatJSON:
			writeErr = b.writeJSON(file, []*unstructured.Unstructured{resource}, opts)
		default:
			writeErr = errors.Errorf("unsupported format: %s", opts.Format)
		}

		// Close file
		if err := file.Close(); err != nil && writeErr == nil {
			writeErr = err
		}

		if writeErr != nil {
			return writeErr
		}

		b.logger.Debug("Wrote resource to %s", filepath)
	}

	return nil
}

// generateFilename generates a filename for a resource
func (b *outputBuilder) generateFilename(resource *unstructured.Unstructured, index int, opts BuildOptions) string {
	// Build filename components
	var parts []string

	// Add index for ordering
	if opts.IncludeIndex {
		parts = append(parts, fmt.Sprintf("%03d", index))
	}

	// Add kind
	kind := strings.ToLower(resource.GetKind())
	if kind != "" {
		parts = append(parts, kind)
	}

	// Add name
	name := resource.GetName()
	if name != "" {
		parts = append(parts, name)
	}

	// Add namespace if present
	if ns := resource.GetNamespace(); ns != "" && opts.IncludeNamespace {
		parts = append(parts, ns)
	}

	// Join parts
	filename := strings.Join(parts, "-")
	if filename == "" {
		filename = fmt.Sprintf("resource-%d", index)
	}

	// Add extension
	switch opts.Format {
	case FormatYAML:
		filename += ".yaml"
	case FormatJSON:
		filename += ".json"
	}

	return filename
}

// defaultFileWriter implements FileWriter using os package
type defaultFileWriter struct{}

func (w *defaultFileWriter) WriteFile(path string, data []byte) error {
	return os.WriteFile(path, data, 0644)
}

func (w *defaultFileWriter) MkdirAll(path string) error {
	return os.MkdirAll(path, 0755)
}