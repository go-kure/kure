package main

import (
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

	"github.com/go-kure/kure/internal/kubernetes"
	kio "github.com/go-kure/kure/pkg/io"
	"github.com/go-kure/kure/pkg/patch"
	"github.com/go-kure/kure/pkg/stack"
	"github.com/go-kure/kure/pkg/stack/layout"

	// Import implementations to register workflow factories
	_ "github.com/go-kure/kure/pkg/stack/argocd"
	_ "github.com/go-kure/kure/pkg/stack/fluxcd"

	// Import generators to register them
	_ "github.com/go-kure/kure/pkg/stack/generators/appworkload"
	_ "github.com/go-kure/kure/pkg/stack/generators/fluxhelm"
)

func ptr[T any](v T) *T { return &v }

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
		{"Patch System", runPatchDemo},
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
	ns := kubernetes.CreateNamespace("demo")
	kubernetes.AddNamespaceLabel(ns, "env", "demo")

	sa := kubernetes.CreateServiceAccount("demo-sa", "demo")
	logError("add serviceaccount secret", kubernetes.AddServiceAccountSecret(sa, apiv1.ObjectReference{Name: "sa-secret"}))

	secret := kubernetes.CreateSecret("demo-secret", "demo")
	logError("add secret data", kubernetes.AddSecretData(secret, "cert", []byte("data")))

	cm := kubernetes.CreateConfigMap("demo-config", "demo")
	logError("add configmap data", kubernetes.AddConfigMapData(cm, "foo", "bar"))

	// Print a few examples
	objects := []runtime.Object{ns, sa, secret, cm}
	for _, obj := range objects {
		logError("failed to print YAML", y.PrintObj(obj, os.Stdout))
	}

	fmt.Printf("Generated %d internal API examples\n", len(objects))
	return nil
}

// runAppWorkloads processes all app-workload configs from examples/app-workloads/
func runAppWorkloads() error {
	exampleDir := "examples/app-workloads"
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
		defer file.Close()

		dec := yaml.NewDecoder(file)
		var apps []*stack.Application

		for {
			var wrapper stack.ApplicationWrapper
			if err := dec.Decode(&wrapper); err != nil {
				if err == io.EOF {
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
		defer outFile.Close()

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

// runClusters processes all cluster configs from examples/clusters/
func runClusters() error {
	clustersDir := "examples/clusters"

	return filepath.Walk(clustersDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() || !strings.HasSuffix(path, "cluster.yaml") {
			return err
		}

		fmt.Printf("Processing cluster: %s\n", path)
		return runClusterExample(path)
	})
}

// runClusterExample processes a single cluster configuration
func runClusterExample(clusterFile string) error {
	file, err := os.Open(clusterFile)
	if err != nil {
		return err
	}
	defer file.Close()

	dec := yaml.NewDecoder(file)
	var cl stack.Cluster
	if err := dec.Decode(&cl); err != nil {
		return err
	}
	if cl.Node == nil {
		return nil
	}

	// Create bundles and load app configs
	rootBundle, err := stack.NewBundle(cl.Node.Name, nil, nil)
	if err != nil {
		return err
	}
	cl.Node.Bundle = rootBundle

	// Determine base directory for loading app configs
	baseDir := filepath.Dir(clusterFile)
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
		f, err := os.Open(fp)
		if err != nil {
			return err
		}

		dec := yaml.NewDecoder(f)
		for {
			var wrapper stack.ApplicationWrapper
			if err := dec.Decode(&wrapper); err != nil {
				if err == io.EOF {
					break
				}
				f.Close()
				return err
			}

			app := wrapper.ToApplication()
			bundle, err := stack.NewBundle(wrapper.Metadata.Name, []*stack.Application{app}, nil)
			if err != nil {
				f.Close()
				return err
			}
			bundle.SetParent(node.Bundle)
			childNode := &stack.Node{Name: wrapper.Metadata.Name, Bundle: bundle}
			childNode.SetParent(node)
			node.Children = append(node.Children, childNode)
		}
		f.Close()
	}

	return nil
}

// runMultiOCIDemo processes multi-OCI configurations from examples/multi-oci/
func runMultiOCIDemo() error {
	fmt.Println("Processing multi-OCI package configurations...")

	clusterFile := "examples/multi-oci/cluster.yaml"
	file, err := os.Open(clusterFile)
	if err != nil {
		return fmt.Errorf("open multi-oci cluster config: %w", err)
	}
	defer file.Close()

	dec := yaml.NewDecoder(file)
	var cl stack.Cluster
	if err := dec.Decode(&cl); err != nil {
		return err
	}

	// Load node applications from multi-oci subdirectories
	baseDir := "examples/multi-oci"
	rootBundle, err := stack.NewBundle(cl.Node.Name, nil, nil)
	if err != nil {
		return err
	}
	cl.Node.Bundle = rootBundle

	for _, child := range cl.Node.Children {
		child.SetParent(cl.Node)

		// Parse packageRef from cluster.yaml
		if child.PackageRef == nil && len(cl.Node.Children) > 0 {
			// This would be set from the YAML parsing, but let me check the structure
		}

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

// runBootstrapDemo processes bootstrap configurations from examples/bootstrap/
func runBootstrapDemo() error {
	bootstrapDir := "examples/bootstrap"

	return filepath.Walk(bootstrapDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() || !strings.HasSuffix(path, ".yaml") {
			return err
		}

		fmt.Printf("Processing bootstrap config: %s\n", path)

		file, err := os.Open(path)
		if err != nil {
			return err
		}
		defer file.Close()

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
		if err != nil {
			return err
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

// runPatchDemo processes patch configurations from examples/patches/
func runPatchDemo() error {
	fmt.Println("Processing patch system examples...")

	examplesDir := "examples/patches"
	baseYAML := filepath.Join(examplesDir, "cert-manager-simple.yaml")
	patchFiles := []string{
		filepath.Join(examplesDir, "resources.kpatch"),
		filepath.Join(examplesDir, "ingress.kpatch"),
		filepath.Join(examplesDir, "security.kpatch"),
		filepath.Join(examplesDir, "advanced.kpatch"),
	}

	// Check if files exist
	if _, err := os.Stat(baseYAML); os.IsNotExist(err) {
		return fmt.Errorf("base YAML file not found: %s", baseYAML)
	}

	// Load base resources
	baseFile, err := os.Open(baseYAML)
	if err != nil {
		return fmt.Errorf("failed to open base YAML: %w", err)
	}
	defer baseFile.Close()

	documentSet, err := patch.LoadResourcesWithStructure(baseFile)
	if err != nil {
		return fmt.Errorf("failed to load resources with structure: %w", err)
	}

	fmt.Printf("Loaded %d base resources with preserved structure\n", len(documentSet.Documents))

	// Create patchable set
	patchableSet := &patch.PatchableAppSet{
		Resources:   documentSet.GetResources(),
		DocumentSet: documentSet,
		Patches: make([]struct {
			Target    string
			Patch     patch.PatchOp
			Strategic *patch.StrategicPatch
		}, 0),
	}

	outputDir := "out/patches"
	if err := patchableSet.WritePatchedFiles(baseYAML, patchFiles, outputDir); err != nil {
		return fmt.Errorf("failed to write patched files: %w", err)
	}

	fmt.Printf("Generated patched files: %s\n", outputDir)
	return nil
}
