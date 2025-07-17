package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/go-kure/kure/pkg/cluster"
)

func main() {
	var configPath string
	var manifestsPath string
	var fluxPath string

	flag.StringVar(&configPath, "config", "", "Path to cluster config YAML file")
	flag.StringVar(&manifestsPath, "manifests", "manifests", "Output path for Kubernetes manifests")
	flag.StringVar(&fluxPath, "flux", "flux", "Output path for FluxCD resources")
	flag.Parse()

	if configPath == "" {
		fmt.Println("Error: --config path is required")
		os.Exit(1)
	}

	cfg, err := cluster.LoadClusterConfigFromYAML(configPath)
	if err != nil {
		fmt.Println("Failed to load cluster config:", err)
		os.Exit(1)
	}

	if err := cluster.WriteCluster(*cfg, manifestsPath, fluxPath); err != nil {
		fmt.Println("Failed to write cluster files:", err)
		os.Exit(1)
	}

	fmt.Println("Cluster generated successfully.")
}
