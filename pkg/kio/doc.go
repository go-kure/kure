// Package kio provides utilities for reading, writing and parsing YAML
// representations of Kubernetes resources.
//
// # YAML helpers
//
// Basic marshalling and unmarshalling is performed through the Marshal and
// Unmarshal functions which operate on the standard io.Reader and io.Writer
// interfaces.  When persisting data to disk the SaveFile and LoadFile helpers
// wrap file creation and reading so callers need only provide the destination
// path and the object to encode or decode.
//
// For in-memory operations the Buffer type implements both io.Reader and
// io.Writer and exposes Marshal and Unmarshal methods.  A Buffer can be reused
// for multiple round trips without allocating new byte slices.
//
// # Parsing runtime objects
//
// Kubernetes manifests frequently contain multiple YAML documents separated by
// `---`.  ParseFile reads such a manifest and decodes each document into a
// runtime.Object using the client-go scheme.  Several additional API schemes
// from projects like FluxCD, cert-manager and MetalLB are registered so their
// custom resources can be parsed without further setup.
//
// Any decoding errors are aggregated in a ParseErrors value which implements
// the error interface.  Successful objects are returned alongside the error so
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
// This package acts as a lightweight foundation for the other packages within
// the repository but can be imported directly by any program that needs simple
// YAML handling capabilities.
package kio
