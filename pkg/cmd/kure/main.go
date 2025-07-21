package main

import (
	"flag"
	"log"

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
		log.Fatalf("Error: --config path is required")
	}

	cfg, err := cluster.LoadClusterConfigFromYAML(configPath)
	if err != nil {
		log.Fatalf("Failed to load cluster config: %v", err)
	}

	if err := cluster.WriteCluster(*cfg, manifestsPath, fluxPath); err != nil {
		log.Fatalf("Failed to write cluster files: %v", err)
	}

	log.Println("Cluster generated successfully.")
}
