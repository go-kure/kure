package main

import (
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/cli-runtime/pkg/printers"

	"github.com/go-kure/kure/pkg/appsets"
	"github.com/go-kure/kure/pkg/cluster"
)

func runCluster(args []string) error {
	fs := flag.NewFlagSet("cluster", flag.ExitOnError)
	var configPath, manifestsPath, fluxPath string
	fs.StringVar(&configPath, "config", "", "Path to cluster config YAML file")
	fs.StringVar(&manifestsPath, "manifests", "manifests", "Output path for Kubernetes manifests")
	fs.StringVar(&fluxPath, "flux", "flux", "Output path for FluxCD resources")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if configPath == "" {
		return fmt.Errorf("--config path is required")
	}
	cfg, err := cluster.LoadClusterConfigFromYAML(configPath)
	if err != nil {
		return fmt.Errorf("failed to load cluster config: %w", err)
	}
	if err := cluster.WriteCluster(*cfg, manifestsPath, fluxPath); err != nil {
		return fmt.Errorf("failed to write cluster files: %w", err)
	}
	log.Println("Cluster generated successfully.")
	return nil
}

func applyPatch(basePath, patchPath string) ([]*unstructured.Unstructured, error) {
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
	set, err := appsets.LoadPatchableAppSet([]io.Reader{baseFile}, patchFile)
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

func runPatch(args []string) error {
	fs := flag.NewFlagSet("patch", flag.ExitOnError)
	var basePath, patchPath string
	fs.StringVar(&basePath, "base", "", "Path to base YAML file")
	fs.StringVar(&patchPath, "patch", "", "Path to patch file")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if basePath == "" || patchPath == "" {
		return fmt.Errorf("--base and --patch are required")
	}
	objs, err := applyPatch(basePath, patchPath)
	if err != nil {
		return err
	}
	y := printers.YAMLPrinter{}
	for _, obj := range objs {
		if err := y.PrintObj(obj, os.Stdout); err != nil {
			return err
		}
	}
	return nil
}

func main() {
	if len(os.Args) > 1 && os.Args[1] == "patch" {
		if err := runPatch(os.Args[2:]); err != nil {
			log.Fatalf("patch error: %v", err)
		}
		return
	}
	if err := runCluster(os.Args[1:]); err != nil {
		log.Fatalf("cluster error: %v", err)
	}
}
