package kubernetes_test

import (
	"fmt"

	appsv1 "k8s.io/api/apps/v1"

	"github.com/go-kure/kure/pkg/kubernetes"
)

// The Go blocks of the "Builder Contract" concept page
// (site/content/concepts/builder-contract.md) are generated from these
// functions and from ExampleCreateDeployment (scripts/gen-doc-examples.sh).

func Example_builderContractNamespace() {
	ns := kubernetes.CreateNamespace("platform")
	fmt.Println(ns.Kind, ns.Name, ns.Namespace == "")
	// Output: Namespace platform true
}

func Example_builderContractGeneric() {
	d := kubernetes.Create[appsv1.Deployment]("web", "default")
	fmt.Println(d.APIVersion, d.Kind, d.Namespace, d.Name)
	// Output: apps/v1 Deployment default web
}

func Example_builderContractStrategy() {
	d := kubernetes.CreateDeployment("web", "default")

	// There is no SetDeploymentStrategy. This is what it would have done.
	d.Spec.Strategy = appsv1.DeploymentStrategy{Type: appsv1.RecreateDeploymentStrategyType}
	fmt.Println(d.Spec.Strategy.Type)
	// Output: Recreate
}
