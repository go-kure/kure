package appsets

import (
	"io"
	"os"
	"path/filepath"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// ApplyPatch loads resources and patch instructions from the provided
// file paths and returns the patched resources.
func ApplyPatch(basePath, patchPath string) ([]*unstructured.Unstructured, error) {
	baseFile, err := os.Open(filepath.Clean(basePath))
	if err != nil {
		return nil, err
	}
	defer baseFile.Close()

	patchFile, err := os.Open(filepath.Clean(patchPath))
	if err != nil {
		return nil, err
	}
	defer patchFile.Close()

	set, err := LoadPatchableAppSet([]io.Reader{baseFile}, patchFile)
	if err != nil {
		return nil, err
	}
	resources, err := set.Resolve()
	if err != nil {
		return nil, err
	}
	var patched []*unstructured.Unstructured
	for _, r := range resources {
		if err := r.Apply(); err != nil {
			return nil, err
		}
		patched = append(patched, r.Base)
	}
	return patched, nil
}
