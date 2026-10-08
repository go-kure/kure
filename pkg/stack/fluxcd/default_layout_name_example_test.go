package fluxcd_test

import (
	"fmt"

	"github.com/go-kure/kure/pkg/stack/fluxcd"
)

// The default per-layout Kustomization name, as the layout integrator writes
// it for a layout without KustomizationName: unchanged where it fits a Flux
// Kustomization name, shortened where it is longer.
func ExampleDefaultLayoutKustomizationName() {
	// The application "api" of the bundle "web": "<unit>-<layout name>".
	fmt.Println(fluxcd.DefaultLayoutKustomizationName("web", "api"))

	// Over 63 characters: the layout name stays, a prefix of the unit and a
	// hash replace the rest.
	fmt.Println(fluxcd.DefaultLayoutKustomizationName(
		"platform-services-payments", "checkout-service-payments-reconciler-worker"))
	// Output:
	// web-api
	// platform-s-7365aca5-checkout-service-payments-reconciler-worker
}
