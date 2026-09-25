package errors_test

import (
	stderrors "errors"
	"fmt"

	kerrors "github.com/go-kure/kure/pkg/errors"
	"github.com/go-kure/kure/pkg/kubernetes"
)

// This file imports this package under a distinct name, as the README's
// "Predefined Errors" section tells callers to; its block is generated from
// this function (scripts/gen-doc-examples.sh).

func Example_sentinel() {
	// A PSA validator given no pod spec returns the ErrNilPodSpec sentinel.
	err := kubernetes.ValidatePodSpecPSA(nil, kubernetes.PSARestricted)

	if stderrors.Is(err, kerrors.ErrNilPodSpec) {
		// handle nil pod spec
		fmt.Println("nil pod spec:", err)
	}
	// Output: nil pod spec: validation failed for PodSpec '' field 'spec': pod spec cannot be nil
}
