package patch

import (
	"fmt"
	"io"
	"os"
	"strings"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/go-kure/kure/pkg/errors"
)

// PatchableAppSet represents a collection of resources together with the
// patches that should be applied to them.
type PatchableAppSet struct {
	Resources   []*unstructured.Unstructured
	DocumentSet *YAMLDocumentSet // Preserves original YAML structure
	KindLookup  KindLookup       // Used for strategic merge patches (may be nil)
	Patches     []struct {
		Target    string
		Patch     PatchOp
		Strategic *StrategicPatch
	}
}

// Resolve groups patches by their target resource and returns them as
// ResourceWithPatches objects.
func (s *PatchableAppSet) Resolve() ([]*ResourceWithPatches, error) {
	// First create a unique key for each resource to avoid name collisions
	resourceMap := make(map[string]*unstructured.Unstructured)
	resourceKeys := make([]string, 0)

	// Count occurrences of kind.name and short name to detect ambiguity
	kindNameCount := make(map[string]int)
	nameCount := make(map[string]int)
	for _, r := range s.Resources {
		kindName := fmt.Sprintf("%s.%s", strings.ToLower(r.GetKind()), r.GetName())
		kindNameCount[kindName]++
		nameCount[r.GetName()]++
	}

	for _, r := range s.Resources {
		name := r.GetName()
		kindName := fmt.Sprintf("%s.%s", strings.ToLower(r.GetKind()), name)
		canonical := CanonicalResourceKey(r)

		// Primary key: canonical (namespace-aware, always unique)
		resourceMap[canonical] = r
		resourceKeys = append(resourceKeys, canonical)

		// Secondary: kind.name without namespace (if unambiguous)
		if kindNameCount[kindName] == 1 && canonical != kindName {
			resourceMap[kindName] = r
		}

		// Tertiary: short name (if unambiguous)
		if nameCount[name] == 1 {
			resourceMap[name] = r
		}
	}

	// Group patches by target, using the canonical resource key for grouping
	out := make(map[string]*ResourceWithPatches)
	for _, p := range s.Patches {
		resource, ok := resourceMap[p.Target]
		if !ok {
			return nil, errors.ResourceNotFoundError("patch target", p.Target, "", nil)
		}

		resourceKey := CanonicalResourceKey(resource)

		rw, exists := out[resourceKey]
		if !exists {
			rw = &ResourceWithPatches{
				Name:       resource.GetName(),
				Base:       resource.DeepCopy(),
				KindLookup: s.KindLookup,
			}
			out[resourceKey] = rw
		}

		if p.Strategic != nil {
			rw.StrategicPatches = append(rw.StrategicPatches, *p.Strategic)
		} else {
			rw.Patches = append(rw.Patches, p.Patch)
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

// ResolveWithConflictCheck resolves patches and additionally checks for
// conflicts among strategic merge patches targeting the same resource.
func (s *PatchableAppSet) ResolveWithConflictCheck() ([]*ResourceWithPatches, []*ConflictReport, error) {
	resolved, err := s.Resolve()
	if err != nil {
		return nil, nil, err
	}

	var reports []*ConflictReport
	for _, rw := range resolved {
		if len(rw.StrategicPatches) < 2 {
			continue
		}

		// Collect patch maps for conflict detection
		patchMaps := make([]map[string]interface{}, len(rw.StrategicPatches))
		for i, sp := range rw.StrategicPatches {
			patchMaps[i] = sp.Patch
		}

		gvk := rw.Base.GroupVersionKind()
		report, err := DetectSMPConflicts(patchMaps, s.KindLookup, gvk)
		if err != nil {
			return nil, nil, fmt.Errorf("conflict detection failed for resource %s: %w", rw.Name, err)
		}
		if report.HasConflicts() {
			report.ResourceName = rw.Name
			report.ResourceKind = gvk.Kind
			reports = append(reports, report)
		}
	}

	return resolved, reports, nil
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

	// Build maps for both patch types, keyed by canonical resource key to avoid
	// cross-kind and cross-namespace collisions.
	patchesByTarget := make(map[string][]PatchOp)
	strategicByTarget := make(map[string]*ResourceWithPatches)
	for _, r := range resolved {
		key := CanonicalResourceKey(r.Base)
		patchesByTarget[key] = r.Patches
		if len(r.StrategicPatches) > 0 {
			strategicByTarget[key] = r
		}
	}

	// Apply strategic patches first, then field-level patches
	for _, doc := range s.DocumentSet.Documents {
		docKey := CanonicalResourceKey(doc.Resource)
		if rw, exists := strategicByTarget[docKey]; exists {
			if err := doc.ApplyStrategicPatchesToDocument(rw.StrategicPatches, s.KindLookup); err != nil {
				return fmt.Errorf("failed to apply strategic patches to document %s: %w", docKey, err)
			}
		}
		if patches, exists := patchesByTarget[docKey]; exists {
			if err := doc.ApplyPatchesToDocument(patches); err != nil {
				return fmt.Errorf("failed to apply patches to document %s: %w", docKey, err)
			}
		}
	}

	// Write to file
	return s.DocumentSet.WriteToFile(filename)
}

// WritePatchedFiles writes separate files for each patch set applied.
// Debug output is enabled based on the current value of the Debug flag.
func (s *PatchableAppSet) WritePatchedFiles(originalPath string, patchFiles []string, outputDir string) error {
	return s.WritePatchedFilesWithOptions(originalPath, patchFiles, outputDir, Debug)
}

// WritePatchedFilesWithOptions writes separate files for each patch set applied
// with explicit debug control, avoiding mutation of the global Debug flag.
func (s *PatchableAppSet) WritePatchedFilesWithOptions(originalPath string, patchFiles []string, outputDir string, debug bool) error {
	if s.DocumentSet == nil {
		return fmt.Errorf("no document set available for structure preservation")
	}

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

		patches, err := LoadPatchFile(patchReader)
		patchReader.Close()
		if err != nil {
			return fmt.Errorf("failed to load patches from %s: %w", patchFile, err)
		}

		// Create a proper PatchableAppSet with structure preservation
		patchableSet, err := NewPatchableAppSetWithStructure(docSetCopy, patches)
		if err != nil {
			// If the error is about a missing target, skip this patch file with a warning
			if strings.Contains(err.Error(), "explicit target not found") || strings.Contains(err.Error(), "not found in base resources") {
				fmt.Printf("⚠️  Skipping %s: contains patches for resources not present in base YAML\n", patchFile)
				if debug {
					fmt.Printf("   Details: %v\n", err)
				}
				continue
			}
			return fmt.Errorf("failed to create patchable set for %s: %w", patchFile, err)
		}

		// Propagate KindLookup so strategic merge patches use schema-aware merging
		patchableSet.KindLookup = s.KindLookup

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
			// Prefer namespace-aware lookup to avoid cross-namespace mismatches
			var doc *YAMLDocument
			if ns := r.Base.GetNamespace(); ns != "" {
				doc = docSetCopy.FindDocumentByKindNameAndNamespace(r.Base.GetKind(), r.Name, ns)
			}
			if doc == nil {
				doc = docSetCopy.FindDocumentByKindAndName(r.Base.GetKind(), r.Name)
			}
			if doc == nil {
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

		if debug {
			fmt.Printf("Wrote patched resources to: %s\n", outputFile)
		}
	}

	return nil
}

// Helper function to open a file (to be replaced with actual file operations)
var openFile = func(filename string) (io.ReadCloser, error) {
	return os.Open(filename)
}
