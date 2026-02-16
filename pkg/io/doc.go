// Package io provides utilities for reading, writing and parsing YAML
// representations of Kubernetes resources. It acts as a thin wrapper
// around sigs.k8s.io/yaml and the Kubernetes runtime scheme from client-go.
//
// # YAML helpers
//
// Basic marshalling and unmarshalling is performed through the Marshal and
// Unmarshal functions which operate on the standard io.Reader and io.Writer
// interfaces. When persisting data to disk the SaveFile and LoadFile helpers
// wrap file creation and reading so callers need only provide the destination
// path and the object to encode or decode.
//
// For in-memory operations the Buffer type implements both io.Reader and
// io.Writer and exposes Marshal and Unmarshal methods. A Buffer can be reused
// for multiple round trips without allocating new byte slices:
//
//	buf := new(io.Buffer)
//	if err := buf.Marshal(obj); err != nil {
//	    log.Fatalf("marshal object: %v", err)
//	}
//	if err := buf.Unmarshal(&out); err != nil {
//	    log.Fatalf("unmarshal object: %v", err)
//	}
//
// # Parsing runtime objects
//
// Kubernetes manifests frequently contain multiple YAML documents separated by
// `---`. ParseFile reads such a manifest and decodes each document into a
// runtime.Object using the client-go scheme. Several additional API schemes
// from projects like FluxCD, cert-manager and MetalLB are registered so their
// custom resources can be parsed without further setup.
//
// Any decoding errors are aggregated in a ParseErrors value which implements
// the error interface. Successful objects are returned alongside the error so
// callers may continue processing valid resources while reporting individual
// failures.
//
//	objs, err := io.ParseFile("./manifests.yaml")
//	if err != nil {
//	    var pe *io.ParseErrors
//	    if errors.As(err, &pe) {
//	        for _, e := range pe.Errors {
//	            log.Printf("parse error: %v", e)
//	        }
//	    }
//	}
//
// # Unstructured fallback
//
// By default the parser rejects objects whose GroupVersionKind is not
// registered in the kure scheme. [ParseYAMLWithOptions] and
// [ParseFileWithOptions] accept a [ParseOptions] value. When
// AllowUnstructured is true, unknown GVKs are decoded as
// *unstructured.Unstructured instead of returning an error, making it
// possible to process arbitrary Kubernetes YAML including CRDs that are
// not compiled into the binary:
//
//	opts := io.ParseOptions{AllowUnstructured: true}
//	objs, err := io.ParseYAMLWithOptions(data, opts)
//	for _, obj := range objs {
//	    if u, ok := obj.(*unstructured.Unstructured); ok {
//	        fmt.Println("unstructured:", u.GetKind(), u.GetName())
//	    }
//	}
//
// # Resource printing
//
// The io package includes comprehensive resource printing capabilities compatible
// with kubectl output formats. The ResourcePrinter provides unified formatting
// for YAML, JSON, table, wide, and name output modes:
//
//	printer := io.NewResourcePrinter(io.PrintOptions{
//		OutputFormat: io.OutputFormatTable,
//		NoHeaders:    false,
//		ShowLabels:   true,
//	})
//	err := printer.Print(resources, os.Stdout)
//
// Table output includes resource-specific column formatting for different
// Kubernetes kinds (Pod, Deployment, Service, ConfigMap) with appropriate
// status indicators, age formatting, and wide-mode additional details.
//
// For simple table printing, use the SimpleTablePrinter which provides
// kubectl-style table output without external dependencies:
//
//	printer := io.NewSimpleTablePrinter(wide, noHeaders)
//	printer.Print(resources, os.Stdout)
//
// Convenience functions are available for common operations:
//
//	io.PrintObjectsAsYAML(resources, os.Stdout)
//	io.PrintObjectsAsJSON(resources, os.Stdout)
//	io.PrintObjectsAsTable(resources, wide, noHeaders, os.Stdout)
//
// # Deterministic field ordering
//
// [EncodeObjectsToYAMLWithOptions] accepts [EncodeOptions] to control YAML
// output. When KubernetesFieldOrder is true, top-level fields are emitted in
// the conventional order used by kubectl, Helm, and Kustomize:
// apiVersion, kind, metadata, spec, data, stringData, then remaining fields
// alphabetically, with status last.
//
// The package forms the foundation for the other packages within this
// repository but can be imported directly by any program that requires
// lightweight YAML handling, runtime object parsing, and kubectl-compatible
// resource printing.
package io
