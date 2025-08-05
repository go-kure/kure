package main

import (
	"bufio"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/go-kure/kure/pkg/patch"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func main() {
	var (
		baseFile    = flag.String("base", "", "Base YAML file containing Kubernetes resources")
		patchFile   = flag.String("patch", "", "Patch file (.patch or .yaml format)")
		outputFile  = flag.String("output", "", "Output file for patched resources (default: stdout)")
		validate    = flag.Bool("validate", false, "Validate patch syntax without applying")
		debug       = flag.Bool("debug", false, "Enable debug logging")
		format      = flag.String("format", "yaml", "Output format: yaml|json")
		interactive = flag.Bool("interactive", false, "Interactive patch editor mode")
		list        = flag.Bool("list", false, "List available resources in base file")
	)
	flag.Parse()

	if *debug {
		os.Setenv("KURE_DEBUG", "1")
	}

	if *validate {
		if err := validatePatchFile(*patchFile); err != nil {
			log.Fatalf("Validation failed: %v", err)
		}
		fmt.Println("Patch file validation successful")
		return
	}

	if *list {
		if *baseFile == "" {
			fmt.Println("Base file required for listing resources")
			flag.Usage()
			os.Exit(1)
		}
		if err := listResources(*baseFile); err != nil {
			log.Fatalf("Failed to list resources: %v", err)
		}
		return
	}

	if *interactive {
		if *baseFile == "" {
			fmt.Println("Base file required for interactive mode")
			flag.Usage()
			os.Exit(1)
		}
		if err := interactiveMode(*baseFile); err != nil {
			log.Fatalf("Interactive mode failed: %v", err)
		}
		return
	}

	if *baseFile == "" || *patchFile == "" {
		flag.Usage()
		os.Exit(1)
	}

	if err := applyPatch(*baseFile, *patchFile, *outputFile, *format); err != nil {
		log.Fatalf("Patch application failed: %v", err)
	}
}

func validatePatchFile(patchPath string) error {
	if patchPath == "" {
		return fmt.Errorf("patch file path required")
	}

	file, err := os.Open(filepath.Clean(patchPath))
	if err != nil {
		return fmt.Errorf("failed to open patch file: %w", err)
	}
	defer file.Close()

	_, err = patch.LoadPatchFile(file)
	if err != nil {
		return fmt.Errorf("failed to parse patch file: %w", err)
	}

	return nil
}

func applyPatch(basePath, patchPath, outputPath, format string) error {
	baseFile, err := os.Open(filepath.Clean(basePath))
	if err != nil {
		return fmt.Errorf("failed to open base file: %w", err)
	}
	defer baseFile.Close()

	patchFile, err := os.Open(filepath.Clean(patchPath))
	if err != nil {
		return fmt.Errorf("failed to open patch file: %w", err)
	}
	defer patchFile.Close()

	// Load resources with structure preservation
	documentSet, err := patch.LoadResourcesWithStructure(baseFile)
	if err != nil {
		return fmt.Errorf("failed to load base resources: %w", err)
	}

	// Load patches
	patches, err := patch.LoadPatchFile(patchFile)
	if err != nil {
		return fmt.Errorf("failed to load patches: %w", err)
	}

	// Create patchable set with structure preservation
	patchableSet, err := patch.NewPatchableAppSetWithStructure(documentSet, patches)
	if err != nil {
		return fmt.Errorf("failed to create patchable set: %w", err)
	}

	// Resolve and apply patches
	resolved, err := patchableSet.Resolve()
	if err != nil {
		return fmt.Errorf("failed to resolve patches: %w", err)
	}

	for _, r := range resolved {
		if err := r.Apply(); err != nil {
			return fmt.Errorf("failed to apply patches to resource %s: %w", r.Name, err)
		}
	}

	// Write output
	if outputPath == "" {
		// Write to stdout
		return documentSet.WriteToFile("/dev/stdout")
	} else {
		// Create output directory if needed
		if err := os.MkdirAll(filepath.Dir(outputPath), 0755); err != nil {
			return fmt.Errorf("failed to create output directory: %w", err)
		}
		return documentSet.WriteToFile(outputPath)
	}
}

func listResources(basePath string) error {
	file, err := os.Open(filepath.Clean(basePath))
	if err != nil {
		return fmt.Errorf("failed to open base file: %w", err)
	}
	defer file.Close()

	resources, err := patch.LoadResourcesFromMultiYAML(file)
	if err != nil {
		return fmt.Errorf("failed to load resources: %w", err)
	}

	fmt.Printf("Found %d resources in %s:\n\n", len(resources), basePath)
	for i, resource := range resources {
		fmt.Printf("%d. %s/%s (kind: %s)\n", 
			i+1, 
			resource.GetNamespace(), 
			resource.GetName(), 
			resource.GetKind())
	}
	
	return nil
}

func interactiveMode(basePath string) error {
	file, err := os.Open(filepath.Clean(basePath))
	if err != nil {
		return fmt.Errorf("failed to open base file: %w", err)
	}
	defer file.Close()

	resources, err := patch.LoadResourcesFromMultiYAML(file)
	if err != nil {
		return fmt.Errorf("failed to load resources: %w", err)
	}

	scanner := bufio.NewScanner(os.Stdin)
	
	fmt.Println("=== Interactive Patch Editor ===")
	fmt.Printf("Loaded %d resources from %s\n", len(resources), basePath)
	fmt.Println()
	fmt.Println("Commands:")
	fmt.Println("  list                    - List all resources")
	fmt.Println("  help                    - Show this help")
	fmt.Println("  patch <path> <value>    - Apply a patch")
	fmt.Println("  show <resource>         - Show resource details")
	fmt.Println("  exit/quit               - Exit editor")
	fmt.Println()
	fmt.Println("Patch syntax examples:")
	fmt.Println("  patch spec.replicas 3")
	fmt.Println("  patch metadata.labels.env production")
	fmt.Println("  patch spec.containers[name=main].image nginx:1.21")
	fmt.Println("  patch spec.containers[-].name sidecar")
	fmt.Println()

	for {
		fmt.Print("patch> ")
		if !scanner.Scan() {
			break
		}
		
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		
		parts := strings.Fields(line)
		if len(parts) == 0 {
			continue
		}
		
		cmd := strings.ToLower(parts[0])
		switch cmd {
		case "help":
			fmt.Println("Commands:")
			fmt.Println("  list                    - List all resources")
			fmt.Println("  help                    - Show this help")
			fmt.Println("  patch <path> <value>    - Apply a patch")
			fmt.Println("  show <resource>         - Show resource details")
			fmt.Println("  exit/quit               - Exit editor")
			
		case "list":
			fmt.Printf("Resources (%d total):\n", len(resources))
			for i, resource := range resources {
				fmt.Printf("  %d. %s/%s (kind: %s)\n", 
					i+1, 
					resource.GetNamespace(), 
					resource.GetName(), 
					resource.GetKind())
			}
			
		case "patch":
			if len(parts) < 3 {
				fmt.Println("Usage: patch <path> <value>")
				fmt.Println("Example: patch spec.replicas 3")
				continue
			}
			path := parts[1]
			value := strings.Join(parts[2:], " ")
			
			if err := applyInteractivePatch(resources, path, value); err != nil {
				fmt.Printf("Patch failed: %v\n", err)
			} else {
				fmt.Println("Patch applied successfully")
			}
			
		case "show":
			if len(parts) < 2 {
				fmt.Println("Usage: show <resource-name>")
				continue
			}
			resourceName := parts[1]
			if err := showResource(resources, resourceName); err != nil {
				fmt.Printf("Show failed: %v\n", err)
			}
			
		case "exit", "quit":
			fmt.Println("Goodbye!")
			return nil
			
		default:
			fmt.Printf("Unknown command: %s. Type 'help' for available commands.\n", cmd)
		}
	}
	
	return nil
}

func applyInteractivePatch(resources []*unstructured.Unstructured, path, value string) error {
	// Parse the patch operation
	op, err := patch.ParsePatchLine(path, value)
	if err != nil {
		return fmt.Errorf("failed to parse patch: %w", err)
	}
	
	// Create patch specs
	patches := []patch.PatchSpec{{Patch: op}}
	
	// Create patchable set
	set, err := patch.NewPatchableAppSet(resources, patches)
	if err != nil {
		return fmt.Errorf("failed to create patchable set: %w", err)
	}
	
	// Resolve and apply
	resolved, err := set.Resolve()
	if err != nil {
		return fmt.Errorf("failed to resolve patches: %w", err)
	}
	
	for _, r := range resolved {
		if err := r.Apply(); err != nil {
			return fmt.Errorf("failed to apply patch: %w", err)
		}
		fmt.Printf("Applied patch to %s\n", r.Name)
	}
	
	return nil
}

func showResource(resources []*unstructured.Unstructured, name string) error {
	for _, resource := range resources {
		if resource.GetName() == name {
			fmt.Printf("Resource: %s/%s\n", resource.GetNamespace(), resource.GetName())
			fmt.Printf("Kind: %s\n", resource.GetKind())
			fmt.Printf("API Version: %s\n", resource.GetAPIVersion())
			fmt.Println("Fields:")
			
			// Show some key fields
			if spec, found, _ := unstructured.NestedMap(resource.Object, "spec"); found {
				fmt.Printf("  spec: %d fields\n", len(spec))
			}
			if metadata, found, _ := unstructured.NestedMap(resource.Object, "metadata"); found {
				fmt.Printf("  metadata: %d fields\n", len(metadata))
				if labels, found, _ := unstructured.NestedStringMap(metadata, "labels"); found {
					fmt.Printf("    labels: %v\n", labels)
				}
			}
			return nil
		}
	}
	return fmt.Errorf("resource not found: %s", name)
}