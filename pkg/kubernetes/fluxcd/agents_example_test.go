package fluxcd_test

import (
	"fmt"

	kustv1 "github.com/fluxcd/kustomize-controller/api/v1"

	"github.com/go-kure/kure/pkg/kubernetes/fluxcd"
)

// The Flux Integration block of AGENTS.md is generated from this function
// (scripts/gen-doc-examples.sh).

func Example_agentsFlux() {
	ks := fluxcd.CreateKustomization("app", "default")
	ks.Spec.Path = "./manifests"
	ks.Spec.SourceRef = kustv1.CrossNamespaceSourceReference{
		Kind: "GitRepository",
		Name: "repo",
	}
	fmt.Println(ks.Spec.Path, ks.Spec.SourceRef.Kind+"/"+ks.Spec.SourceRef.Name)
	// Output: ./manifests GitRepository/repo
}
