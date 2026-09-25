package fluxcd_test

import (
	"fmt"
	"os"
	"time"

	kustv1 "github.com/fluxcd/kustomize-controller/api/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/go-kure/kure/pkg/io"
	"github.com/go-kure/kure/pkg/kubernetes/fluxcd"
)

// The Go blocks of docs/quickstart.md and of the "Design Philosophy" concept
// page (site/content/concepts/design-philosophy.md) are generated from these
// functions (scripts/gen-doc-examples.sh).

func Example_quickstart() {
	ks := fluxcd.CreateKustomization("hello-world", "flux-system")
	ks.Spec.SourceRef = kustv1.CrossNamespaceSourceReference{
		Kind: "GitRepository",
		Name: "flux-system",
	}
	ks.Spec.Path = "./clusters/production"
	ks.Spec.Interval = metav1.Duration{Duration: 5 * time.Minute}
	ks.Spec.Prune = true

	if err := io.Marshal(os.Stdout, ks); err != nil {
		panic(err)
	}
	// Output:
	// apiVersion: kustomize.toolkit.fluxcd.io/v1
	// kind: Kustomization
	// metadata:
	//   name: hello-world
	//   namespace: flux-system
	// spec:
	//   interval: 5m0s
	//   path: ./clusters/production
	//   prune: true
	//   sourceRef:
	//     kind: GitRepository
	//     name: flux-system
	// status: {}
}

func Example_designPhilosophy() {
	// Compile-time checked against the real API type — typos and type errors are
	// caught by the compiler, and the field names are the ones in the CRD.
	ks := fluxcd.CreateKustomization("my-app", "flux-system")
	ks.Spec.Path = "./clusters/production/apps"
	ks.Spec.Interval = metav1.Duration{Duration: 10 * time.Minute}
	ks.Spec.Prune = true
	fmt.Println(ks.Kind, ks.Spec.Path, ks.Spec.Interval.Duration, ks.Spec.Prune)
	// Output: Kustomization ./clusters/production/apps 10m0s true
}
