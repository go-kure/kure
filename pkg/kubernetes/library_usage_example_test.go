package kubernetes_test

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	kustv1 "github.com/fluxcd/kustomize-controller/api/v1"
	sourcev1 "github.com/fluxcd/source-controller/api/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/go-kure/kure/pkg/errors"
	"github.com/go-kure/kure/pkg/io"
	"github.com/go-kure/kure/pkg/kubernetes"
	"github.com/go-kure/kure/pkg/kubernetes/fluxcd"
	"github.com/go-kure/kure/pkg/manifest"
	"github.com/go-kure/kure/pkg/stack"
)

// The Go blocks of the "Using Kure as a Library" guide
// (site/content/guides/library-usage.md) are generated from these functions
// (scripts/gen-doc-examples.sh).

// frontendConfig is the stack.ApplicationConfig the guide's domain-model
// example uses: one Deployment named after the application.
var frontendConfig stack.ApplicationConfig = deploymentApp{}

type deploymentApp struct{}

func (deploymentApp) Generate(app *stack.Application) ([]*client.Object, error) {
	var obj client.Object = kubernetes.CreateDeployment(app.Name, app.Namespace)
	return []*client.Object{&obj}, nil
}

// exportedObjects is what the guide's encoding examples serialize: a
// ConfigMap carrying the metadata an API server sets, as when it is read back
// from a cluster.
func exportedObjects() []*client.Object {
	cm := kubernetes.CreateConfigMap("app-config", "default")
	cm.Data = map[string]string{"LOG_LEVEL": "info"}
	cm.ResourceVersion = "48213"
	cm.UID = "0f6c1d9e-2b7a-4c55-9d3e-6a1b2c3d4e5f"
	cm.CreationTimestamp = metav1.NewTime(time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC))
	var obj client.Object = cm
	return []*client.Object{&obj}
}

func Example_libraryUsageCreate() {
	dep := kubernetes.CreateDeployment("web", "default") // identity only
	dep.Spec.Replicas = ptr.To[int32](3)
	dep.Spec.Selector = &metav1.LabelSelector{MatchLabels: map[string]string{"app": "web"}}
	dep.Spec.Template.Spec.ServiceAccountName = "web"
	fmt.Println(dep.Kind, dep.Name, *dep.Spec.Replicas, dep.Spec.Selector.MatchLabels["app"])
	// Output: Deployment web 3 web
}

func Example_libraryUsageHelpers() {
	dep := kubernetes.CreateDeployment("web", "default")

	kubernetes.AddLabel(dep, "tier", "frontend") // works on any kind
	kubernetes.SetDeploymentReplicas(dep, 3)
	fmt.Println(dep.Labels["tier"], *dep.Spec.Replicas)
	// Output: frontend 3
}

func Example_libraryUsageFlux() {
	// Create a GitRepository source
	repo := fluxcd.CreateGitRepository("my-repo", "flux-system")
	repo.Spec.URL = "https://github.com/org/repo"
	fluxcd.SetGitRepositoryReference(repo, &sourcev1.GitRepositoryRef{Branch: "main"})
	repo.Spec.Interval = metav1.Duration{Duration: 5 * time.Minute}

	// Create a Kustomization that references the source
	ks := fluxcd.CreateKustomization("my-app", "flux-system")
	ks.Spec.SourceRef = kustv1.CrossNamespaceSourceReference{
		Kind: "GitRepository",
		Name: "my-repo",
	}
	ks.Spec.Path = "./clusters/production"
	ks.Spec.Interval = metav1.Duration{Duration: 10 * time.Minute}
	ks.Spec.Prune = true
	fmt.Println(repo.Spec.Reference.Branch, ks.Spec.SourceRef.Kind+"/"+ks.Spec.SourceRef.Name, ks.Spec.Path)
	// Output: main GitRepository/my-repo ./clusters/production
}

func Example_libraryUsageYAML() {
	cm := kubernetes.CreateConfigMap("app-config", "default")
	cm.Data = map[string]string{"LOG_LEVEL": "info"}
	var obj client.Object = cm
	objects := []*client.Object{&obj}
	dir, err := os.MkdirTemp("", "kure-library-usage")
	if err != nil {
		panic(err)
	}
	defer func() { _ = os.RemoveAll(dir) }()

	// Serialize a single object
	if err := io.Marshal(os.Stdout, cm); err != nil {
		panic(err)
	}

	// Write multiple objects to stdout as YAML
	if err := io.PrintObjectsAsYAML(objects, os.Stdout); err != nil {
		panic(err)
	}

	// Save to file
	if err := io.SaveFile(filepath.Join(dir, "output.yaml"), cm); err != nil {
		panic(err)
	}
	// Output:
	// apiVersion: v1
	// data:
	//   LOG_LEVEL: info
	// kind: ConfigMap
	// metadata:
	//   name: app-config
	//   namespace: default
	// apiVersion: v1
	// data:
	//   LOG_LEVEL: info
	// kind: ConfigMap
	// metadata:
	//   name: app-config
	//   namespace: default
}

func Example_libraryUsageEncodeDefault() {
	objects := exportedObjects()

	// Default: strips server-managed fields and uses standard key order
	data, err := io.EncodeObjectsToYAMLWithOptions(objects, io.EncodeOptions{
		KubernetesFieldOrder: true,
	})
	if err != nil {
		panic(err)
	}
	fmt.Print(string(data))
	// Output:
	// apiVersion: v1
	// kind: ConfigMap
	// metadata:
	//   creationTimestamp: "2026-01-02T03:04:05Z"
	//   name: app-config
	//   namespace: default
	// data:
	//   LOG_LEVEL: info
}

func Example_libraryUsageEncodeNone() {
	objects := exportedObjects()

	// Preserve server fields (e.g. for debugging)
	data, err := io.EncodeObjectsToYAMLWithOptions(objects, io.EncodeOptions{
		ServerFieldStripping: io.StripServerFieldsNone,
	})
	if err != nil {
		panic(err)
	}
	fmt.Print(string(data))
	// Output:
	// apiVersion: v1
	// data:
	//   LOG_LEVEL: info
	// kind: ConfigMap
	// metadata:
	//   creationTimestamp: "2026-01-02T03:04:05Z"
	//   name: app-config
	//   namespace: default
	//   resourceVersion: "48213"
	//   uid: 0f6c1d9e-2b7a-4c55-9d3e-6a1b2c3d4e5f
}

func Example_libraryUsageKindFor() {
	k, ok := kubernetes.KindFor("apps/v1", "Deployment") // exact group/version/kind
	if ok {
		fmt.Println(k.Namespaced, k.Module, k.ScopeSource)
	}

	namespaced, known := kubernetes.IsNamespaced("autoscaling/v1", "HorizontalPodAutoscaler")
	fmt.Println(namespaced, known)
	// Output:
	// true k8s.io/api builtin
	// true true
}

func Example_libraryUsageScopeSource() {
	k, _ := kubernetes.KindForAnyVersion("cilium.io/v2", "CiliumNetworkPolicy")
	fmt.Println(k.ScopeSource == kubernetes.ScopeSourceBuiltin) // false: declared by a marker

	// pkg/manifest asks both forms of the scope question:
	fmt.Println(manifest.IsNamespacedKind("cilium.io/v2", "CiliumNetworkPolicy"))        // true
	fmt.Println(manifest.IsNamespacedBuiltinKind("cilium.io/v2", "CiliumNetworkPolicy")) // false, not a built-in
	// Output:
	// false
	// true
	// false
}

func Example_libraryUsageMaturity() {
	for _, f := range kubernetes.MaturityForType("k8s.io/api/core/v1", "PodSpec") {
		fmt.Println(f.Field, f.Stability, f.Gates)
	}
	gated := kubernetes.GatedFields() // every field behind a feature gate
	fmt.Println(len(gated) > 0)
}

func Example_libraryUsageDomainModel() {
	cluster, err := stack.NewClusterBuilder("production").
		WithNode("apps").
		WithBundle("web").
		WithApplication("frontend", frontendConfig).
		End().
		End().
		Build()
	if err != nil {
		panic(err)
	}
	fmt.Println(cluster.Name, cluster.Node.Name, cluster.Node.Bundle.Name)
	// Output: production apps web
}

func Example_libraryUsageErrors() {
	load := func(path string) error {
		_, err := io.ParseFile(path)
		if err != nil {
			return errors.Wrap(err, "failed to generate manifests")
		}
		return nil
	}
	fmt.Println(load("does-not-exist.yaml"))
	// Output: failed to generate manifests: open does-not-exist.yaml: no such file or directory
}
