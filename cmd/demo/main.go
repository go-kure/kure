package main

import (
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"

	apiv1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/cli-runtime/pkg/printers"

	intkubernetes "github.com/go-kure/kure/internal/kubernetes"
	kio "github.com/go-kure/kure/pkg/io"
	"github.com/go-kure/kure/pkg/kubernetes"
	"github.com/go-kure/kure/pkg/stack"
	"github.com/go-kure/kure/pkg/stack/layout"

	// Import implementations to register workflow factories
	_ "github.com/go-kure/kure/pkg/stack/argocd"
	_ "github.com/go-kure/kure/pkg/stack/fluxcd"

	// Import generators to register them
	_ "github.com/go-kure/kure/pkg/stack/generators/appworkload"
	_ "github.com/go-kure/kure/pkg/stack/generators/fluxhelm"
)

func logError(msg string, err error) {
	if err != nil {
		log.Printf("%s: %v", msg, err)
	}
}

func main() {
	fmt.Println("=== Kure Demo Suite ===")
	fmt.Println("Running all demos from examples/ configurations")
	fmt.Println()

	// Run all demos
	demos := []struct {
		name string
		fn   func() error
	}{
		{"Internal API Examples", runInternals},
		{"App Workloads", runAppWorkloads},
		{"Clusters", runClusters},
		{"Multi-OCI Packages", runMultiOCIDemo},
		{"Bootstrap Configurations", runBootstrapDemo},
	}

	for _, demo := range demos {
		fmt.Printf("=== %s ===\n", demo.name)
		if err := demo.fn(); err != nil {
			log.Printf("Demo '%s' failed: %v", demo.name, err)
		}
		fmt.Println()
	}

	fmt.Println("=== All Demos Complete ===")
}

// runInternals demonstrates internal API usage (no external config needed)
func runInternals() error {
	fmt.Println("Demonstrating internal Kubernetes API builders...")

	y := printers.YAMLPrinter{}

	// Create a few example resources to demonstrate the internal APIs
	ns := intkubernetes.CreateNamespace("demo")
	intkubernetes.AddNamespaceLabel(ns, "env", "demo")

	sa := kubernetes.CreateServiceAccount("demo-sa", "demo")
	logError("add serviceaccount secret", kubernetes.AddServiceAccountSecret(sa, apiv1.ObjectReference{Name: "sa-secret"}))

	secret := intkubernetes.CreateSecret("demo-secret", "demo")
	intkubernetes.AddSecretData(secret, "cert", []byte("data"))

	cm := intkubernetes.CreateConfigMap("demo-config", "demo")
	intkubernetes.AddConfigMapData(cm, "foo", "bar")

	// Print a few examples
	objects := []runtime.Object{ns, sa, secret, cm}
	for _, obj := range objects {
		logError("failed to print YAML", y.PrintObj(obj, os.Stdout))
	}

	fmt.Printf("Generated %d internal API examples\n", len(objects))
	return nil
}

// runAppWorkloads processes all app-workload configs from examples/demo/app-workloads/
func runAppWorkloads() error {
	exampleDir := "examples/demo/app-workloads"
	outputDir := "out/app-workloads"

	if err := os.RemoveAll(outputDir); err != nil {
		return err
	}

	return filepath.Walk(exampleDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() || !strings.HasSuffix(path, ".yaml") {
			return err
		}

		fmt.Printf("Processing app workload: %s\n", path)

		file, err := os.Open(path)
		if err != nil {
			return err
		}
		defer func() { _ = file.Close() }()

		dec := yaml.NewDecoder(file)
		var apps []*stack.Application

		for {
			var wrapper stack.ApplicationWrapper
			if err := dec.Decode(&wrapper); err != nil {
				if errors.Is(err, io.EOF) {
					break
				}
				return err
			}
			app := wrapper.ToApplication()
			apps = append(apps, app)
		}

		if len(apps) == 0 {
			return nil
		}

		bundle, err := stack.NewBundle("example", apps, nil)
		if err != nil {
			return err
		}

		resources, err := bundle.Generate()
		if err != nil {
			return err
		}

		// Create output file
		relPath, _ := filepath.Rel(exampleDir, path)
		outputPath := filepath.Join(outputDir, strings.TrimSuffix(relPath, ".yaml")+"-generated.yaml")

		if err := os.MkdirAll(filepath.Dir(outputPath), 0755); err != nil {
			return err
		}

		outFile, err := os.Create(outputPath)
		if err != nil {
			return err
		}
		defer func() { _ = outFile.Close() }()

		out, err := kio.EncodeObjectsToYAML(resources)
		if err != nil {
			return err
		}

		if _, err = outFile.Write(out); err != nil {
			return err
		}

		fmt.Printf("Generated app workload manifests: %s\n", outputPath)
		return nil
	})
}

// runClusters processes all cluster configs from examples/demo/clusters/.
// Per-cluster failures are logged but do not abort the walk, so one broken
// example does not hide the output of the others.
func runClusters() error {
	clustersDir := "examples/demo/clusters"

	return filepath.Walk(clustersDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() || !strings.HasSuffix(path, "cluster.yaml") {
			return err
		}

		fmt.Printf("Processing cluster: %s\n", path)
		if err := runClusterExample(path); err != nil {
			log.Printf("cluster %s failed: %v", path, err)
		}
		return nil
	})
}

// runClusterExample processes a single cluster configuration
func runClusterExample(clusterFile string) error {
	file, err := os.Open(filepath.Clean(clusterFile)) //nolint:gosec // G703: CLI tool reads user-specified file paths
	if err != nil {
		return err
	}
	defer func() { _ = file.Close() }()

	dec := yaml.NewDecoder(file)
	var cl stack.Cluster
	if err := dec.Decode(&cl); err != nil {
		return err
	}
	if cl.Node == nil {
		return nil
	}

	// Determine base directory for loading app configs
	baseDir := filepath.Dir(clusterFile)

	// Umbrella path: when the cluster YAML provides a Bundle with Children,
	// respect it as-is and load each child's workloads from <baseDir>/<childName>/.
	// The umbrella itself holds no Applications — its job is to aggregate
	// child readiness via spec.wait + spec.healthChecks.
	if cl.Node.Bundle != nil && len(cl.Node.Bundle.Children) > 0 {
		if err := loadUmbrellaChildrenApps(cl.Node.Bundle, baseDir); err != nil {
			return err
		}
	} else {
		// Create bundles and load app configs
		rootBundle, err := stack.NewBundle(cl.Node.Name, nil, nil)
		if err != nil {
			return err
		}
		cl.Node.Bundle = rootBundle

		for _, child := range cl.Node.Children {
			child.SetParent(cl.Node)
			childBundle, err := stack.NewBundle(child.Name, nil, nil)
			if err != nil {
				return err
			}
			child.Bundle = childBundle
			childBundle.SetParent(rootBundle)

			// Load app configs from child directory
			if err := loadNodeApps(child, baseDir); err != nil {
				return err
			}
		}
	}

	// Configure output
	repoDir := filepath.Join("out", cl.Name+"-repo")
	if err := os.RemoveAll(repoDir); err != nil {
		return err
	}

	// Generate layout
	cfg := layout.Config{ManifestsDir: "clusters"}
	rules := layout.DefaultLayoutRules()
	rules.ClusterName = cl.Name
	rules.FluxPlacement = layout.FluxIntegrated

	wf, err := stack.NewWorkflow("flux")
	if err != nil {
		return err
	}

	result, err := wf.CreateLayoutWithResources(&cl, rules)
	if err != nil {
		return err
	}

	ml, ok := result.(*layout.ManifestLayout)
	if !ok {
		return fmt.Errorf("unexpected result type from CreateLayoutWithResources")
	}

	// Write manifests
	if err := layout.WriteManifest(repoDir, cfg, ml); err != nil {
		return err
	}

	fmt.Printf("Generated cluster manifests: %s\n", repoDir)

	// Show bootstrap info if configured
	if cl.GitOps != nil && cl.GitOps.Bootstrap != nil && cl.GitOps.Bootstrap.Enabled {
		fmt.Printf("Bootstrap enabled: %s mode\n", cl.GitOps.Bootstrap.FluxMode)
	}

	return nil
}

// loadNodeApps loads application configs for a node from the filesystem
func loadNodeApps(node *stack.Node, baseDir string) error {
	dir := filepath.Join(baseDir, node.Name)
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		ext := filepath.Ext(entry.Name())
		if ext != ".yaml" && ext != ".yml" {
			continue
		}

		fp := filepath.Join(dir, entry.Name())
		f, err := os.Open(filepath.Clean(fp)) //nolint:gosec // G703: CLI tool reads user-specified file paths
		if err != nil {
			return err
		}

		dec := yaml.NewDecoder(f)
		for {
			var wrapper stack.ApplicationWrapper
			if err := dec.Decode(&wrapper); err != nil {
				if errors.Is(err, io.EOF) {
					break
				}
				_ = f.Close()
				return err
			}

			app := wrapper.ToApplication()
			bundle, err := stack.NewBundle(wrapper.Metadata.Name, []*stack.Application{app}, nil)
			if err != nil {
				_ = f.Close()
				return err
			}
			bundle.SetParent(node.Bundle)
			childNode := &stack.Node{Name: wrapper.Metadata.Name, Bundle: bundle}
			childNode.SetParent(node)
			node.Children = append(node.Children, childNode)
		}
		_ = f.Close()
	}

	return nil
}

// loadUmbrellaChildrenApps attaches each umbrella child's workloads by reading
// AppWorkload YAML files from baseDir/<child.Name>/ into child.Applications.
// The parent pointer on each child is set so the flux engine can derive paths
// relative to the umbrella.
func loadUmbrellaChildrenApps(umbrella *stack.Bundle, baseDir string) error {
	for _, child := range umbrella.Children {
		if child == nil {
			continue
		}
		child.SetParent(umbrella)
		apps, err := loadApplicationsFromDir(filepath.Join(baseDir, child.Name))
		if err != nil {
			return err
		}
		child.Applications = apps
	}
	return nil
}

// loadApplicationsFromDir decodes each .yaml file in dir as one or more
// stack.ApplicationWrapper documents and returns the corresponding Applications.
func loadApplicationsFromDir(dir string) ([]*stack.Application, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	var apps []*stack.Application
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		ext := filepath.Ext(entry.Name())
		if ext != ".yaml" && ext != ".yml" {
			continue
		}

		fp := filepath.Join(dir, entry.Name())
		f, err := os.Open(filepath.Clean(fp)) //nolint:gosec // G703: CLI tool reads user-specified file paths
		if err != nil {
			return nil, err
		}

		dec := yaml.NewDecoder(f)
		for {
			var wrapper stack.ApplicationWrapper
			if err := dec.Decode(&wrapper); err != nil {
				if errors.Is(err, io.EOF) {
					break
				}
				_ = f.Close()
				return nil, err
			}
			apps = append(apps, wrapper.ToApplication())
		}
		_ = f.Close()
	}
	return apps, nil
}

// runMultiOCIDemo processes multi-OCI configurations from examples/demo/multi-oci/
func runMultiOCIDemo() error {
	fmt.Println("Processing multi-OCI package configurations...")

	clusterFile := "examples/demo/multi-oci/cluster.yaml"
	file, err := os.Open(clusterFile)
	if err != nil {
		return fmt.Errorf("open multi-oci cluster config: %w", err)
	}
	defer func() { _ = file.Close() }()

	dec := yaml.NewDecoder(file)
	var cl stack.Cluster
	if err := dec.Decode(&cl); err != nil {
		return err
	}

	// Load node applications from multi-oci subdirectories
	baseDir := "examples/demo/multi-oci"
	rootBundle, err := stack.NewBundle(cl.Node.Name, nil, nil)
	if err != nil {
		return err
	}
	cl.Node.Bundle = rootBundle

	for _, child := range cl.Node.Children {
		child.SetParent(cl.Node)

		// Parse packageRef from cluster.yaml
		// PackageRef is set from the YAML parsing

		if err := loadNodeApps(child, baseDir); err != nil {
			log.Printf("Warning: could not load apps for node %s: %v", child.Name, err)
		}
	}

	// Generate standard layout (by node instead of by package type)
	rules := layout.DefaultLayoutRules()
	ml, err := layout.WalkCluster(&cl, rules)
	if err != nil {
		return err
	}

	baseOutputDir := "out/multi-oci-demo"
	if err := os.RemoveAll(baseOutputDir); err != nil {
		return err
	}

	// Write manifests using standard layout
	cfg := layout.Config{ManifestsDir: ""}
	if err := layout.WriteManifest(baseOutputDir, cfg, ml); err != nil {
		return err
	}

	fmt.Printf("Generated multi-OCI packages: %s\n", baseOutputDir)
	fmt.Printf("Found %d nodes\n", len(cl.Node.Children))

	return nil
}

// runBootstrapDemo processes bootstrap configurations from examples/demo/bootstrap/
func runBootstrapDemo() error {
	bootstrapDir := "examples/demo/bootstrap"

	return filepath.Walk(bootstrapDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() || !strings.HasSuffix(path, ".yaml") {
			return err
		}

		fmt.Printf("Processing bootstrap config: %s\n", path)

		file, err := os.Open(path)
		if err != nil {
			return err
		}
		defer func() { _ = file.Close() }()

		dec := yaml.NewDecoder(file)
		var cl stack.Cluster
		if err := dec.Decode(&cl); err != nil {
			return err
		}

		// Ensure basic structure
		if cl.Node == nil {
			cl.Node = &stack.Node{Name: "flux-system"}
		}
		if cl.Node.Bundle == nil {
			cl.Node.Bundle = &stack.Bundle{Name: "infrastructure"}
		}

		// Generate bootstrap manifests using correct workflow
		rules := layout.DefaultLayoutRules()
		rules.FluxPlacement = layout.FluxSeparate

		var ml *layout.ManifestLayout

		// Determine GitOps provider
		provider := "flux" // default
		if cl.GitOps != nil && cl.GitOps.Type != "" {
			provider = cl.GitOps.Type
		}

		// Create workflow using interface
		wf, err := stack.NewWorkflow(provider)
		if err != nil {
			return err
		}

		if cl.GitOps != nil && cl.GitOps.Type == "argocd" {
			// For ArgoCD, generate bootstrap resources directly
			bootstrapObjs, err := wf.GenerateBootstrap(cl.GitOps.Bootstrap, cl.Node)
			if err != nil {
				return err
			}

			// Create a basic manifest layout for ArgoCD
			ml = &layout.ManifestLayout{
				Name:      cl.Node.Name,
				Namespace: cl.Name,
				Resources: bootstrapObjs,
			}
		} else {
			// Default to Flux workflow
			result, err := wf.CreateLayoutWithResources(&cl, rules)
			if err != nil {
				return err
			}
			var ok bool
			ml, ok = result.(*layout.ManifestLayout)
			if !ok {
				return fmt.Errorf("unexpected result type from CreateLayoutWithResources")
			}
		}

		// Output to separate directory per config
		baseName := strings.TrimSuffix(filepath.Base(path), ".yaml")
		outputDir := filepath.Join("out", "bootstrap", baseName)
		if err := os.RemoveAll(outputDir); err != nil {
			return err
		}

		cfg := layout.Config{ManifestsDir: ""}
		if err := layout.WriteManifest(outputDir, cfg, ml); err != nil {
			return err
		}

		fmt.Printf("Generated bootstrap manifests: %s\n", outputDir)
		return nil
	})
}
