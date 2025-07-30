// Package kio provides utilities for reading, writing and parsing YAML
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
//	buf := new(kio.Buffer)
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
//	objs, err := kio.ParseFile("./manifests.yaml")
//	if err != nil {
//	    var pe *kio.ParseErrors
//	    if errors.As(err, &pe) {
//	        for _, e := range pe.Errors {
//	            log.Printf("parse error: %v", e)
//	        }
//	    }
//	}
//
// The package forms the foundation for the other packages within this
// repository but can be imported directly by any program that requires
// lightweight YAML handling and runtime object parsing.
package kio
