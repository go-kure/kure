package patch

import (
	"fmt"
	"io"
	"os"
	"strings"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// PatchableAppSet represents a collection of resources together with the
// patches that should be applied to them.
type PatchableAppSet struct {
	Resources    []*unstructured.Unstructured
	DocumentSet  *YAMLDocumentSet // Preserves original YAML structure
	Patches      []struct {
		Target string
		Patch  PatchOp
	}
}

// Resolve groups patches by their target resource and returns them as
// ResourceWithPatches objects.
func (s *PatchableAppSet) Resolve() ([]*ResourceWithPatches, error) {
	// First create a unique key for each resource to avoid name collisions
	resourceMap := make(map[string]*unstructured.Unstructured)
	resourceKeys := make([]string, 0)
	
	for _, r := range s.Resources {
		name := r.GetName()
		kindName := fmt.Sprintf("%s.%s", strings.ToLower(r.GetKind()), name)
		
		// Use kind.name as the primary key to ensure uniqueness
		resourceMap[kindName] = r
		resourceKeys = append(resourceKeys, kindName)
		
		// Also allow lookup by name alone if it's unique
		if _, exists := resourceMap[name]; !exists {
			resourceMap[name] = r
		}
	}
	
	// Group patches by target, using the unique resource key for grouping
	out := make(map[string]*ResourceWithPatches)
	for _, p := range s.Patches {
		if resource, ok := resourceMap[p.Target]; ok {
			// Use the resource's unique key (kind.name) as the map key for grouping
			resourceKey := fmt.Sprintf("%s.%s", strings.ToLower(resource.GetKind()), resource.GetName())
			
			if rw, exists := out[resourceKey]; exists {
				rw.Patches = append(rw.Patches, p.Patch)
			} else {
				out[resourceKey] = &ResourceWithPatches{
					Name: resource.GetName(),
					Base: resource.DeepCopy(),
					Patches: []PatchOp{p.Patch},
				}
			}
		} else {
			return nil, fmt.Errorf("target not found: %s", p.Target)
		}
	}
	
	// Convert to result slice, preserving original resource order
	var result []*ResourceWithPatches
	for _, key := range resourceKeys {
		if rw, exists := out[key]; exists {
			result = append(result, rw)
		}
	}
	
	return result, nil
}

// WriteToFile writes the patched resources to a file while preserving structure
func (s *PatchableAppSet) WriteToFile(filename string) error {
	if s.DocumentSet == nil {
		return fmt.Errorf("no document set available for structure preservation")
	}

	// First, resolve and apply all patches
	resolved, err := s.Resolve()
	if err != nil {
		return fmt.Errorf("failed to resolve patches: %w", err)
	}

	// Group patches by target for efficient application
	patchesByTarget := make(map[string][]PatchOp)
	for _, r := range resolved {
		patchesByTarget[r.Name] = r.Patches
	}

	// Apply patches to documents while preserving structure
	for _, doc := range s.DocumentSet.Documents {
		resourceName := doc.Resource.GetName()
		if patches, exists := patchesByTarget[resourceName]; exists {
			if err := doc.ApplyPatchesToDocument(patches); err != nil {
				return fmt.Errorf("failed to apply patches to document %s: %w", resourceName, err)
			}
		}
	}

	// Write to file
	return s.DocumentSet.WriteToFile(filename)
}

// WritePatchedFiles writes separate files for each patch set applied
func (s *PatchableAppSet) WritePatchedFiles(originalPath string, patchFiles []string, outputDir string) error {
	if s.DocumentSet == nil {
		return fmt.Errorf("no document set available for structure preservation")
	}
	
	// Enable debug for this operation
	oldDebug := Debug
	Debug = true  
	defer func() { Debug = oldDebug }()

	for _, patchFile := range patchFiles {
		// Generate output filename
		outputFile := GenerateOutputFilename(originalPath, patchFile, outputDir)
		
		// Create a copy of the document set for this patch
		docSetCopy, err := s.DocumentSet.Copy()
		if err != nil {
			return fmt.Errorf("failed to copy document set: %w", err)
		}

		// Load patches from the specific patch file
		patchReader, err := openFile(patchFile)
		if err != nil {
			return fmt.Errorf("failed to open patch file %s: %w", patchFile, err)
		}
		defer patchReader.Close()

		patches, err := LoadPatchFile(patchReader)
		if err != nil {
			return fmt.Errorf("failed to load patches from %s: %w", patchFile, err)
		}

		// Create a proper PatchableAppSet with structure preservation
		patchableSet, err := NewPatchableAppSetWithStructure(docSetCopy, patches)
		if err != nil {
			// If the error is about a missing target, skip this patch file with a warning
			if strings.Contains(err.Error(), "explicit target not found") {
				fmt.Printf("⚠️  Skipping %s: contains patches for resources not present in base YAML\n", patchFile)
				if Debug {
					fmt.Printf("   Details: %v\n", err)
				}
				continue
			}
			return fmt.Errorf("failed to create patchable set for %s: %w", patchFile, err)
		}

		// Resolve and apply patches
		resolved, err := patchableSet.Resolve()
		if err != nil {
			return fmt.Errorf("failed to resolve patches from %s: %w", patchFile, err)
		}

		// Apply patches to the resources in memory
		for _, r := range resolved {
			if err := r.Apply(); err != nil {
				return fmt.Errorf("failed to apply patches to resource %s: %w", r.Name, err)
			}
		}

		// Update the document set resources with the patched versions
		for _, r := range resolved {
			// Use kind and name to find the correct document when there are duplicates
			doc := docSetCopy.FindDocumentByKindAndName(r.Base.GetKind(), r.Name)
			if doc == nil {
				// Fallback to name-only search if kind-specific search fails
				doc = docSetCopy.FindDocumentByName(r.Name)
			}
			if doc != nil {
				doc.Resource = r.Base
				// Update the YAML node from the patched resource
				if err := doc.UpdateDocumentFromResource(); err != nil {
					return fmt.Errorf("failed to update document structure for %s: %w", r.Name, err)
				}
			}
		}

		// Create output directory if it doesn't exist
		if outputDir != "" && outputDir != "." {
			if err := os.MkdirAll(outputDir, 0755); err != nil {
				return fmt.Errorf("failed to create output directory %s: %w", outputDir, err)
			}
		}
		
		// Write to output file
		if err := docSetCopy.WriteToFile(outputFile); err != nil {
			return fmt.Errorf("failed to write patched file %s: %w", outputFile, err)
		}

		if Debug {
			fmt.Printf("Wrote patched resources to: %s\n", outputFile)
		}
	}

	return nil
}

// Helper function to open a file (to be replaced with actual file operations)
var openFile = func(filename string) (io.ReadCloser, error) {
	return os.Open(filename)
}
