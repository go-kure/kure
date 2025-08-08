package main

import (
	"fmt"
	"log"

	"gopkg.in/yaml.v3"

	"github.com/go-kure/kure/pkg/io"
	"github.com/go-kure/kure/pkg/stack"
	_ "github.com/go-kure/kure/pkg/stack/generators" // Register all generators
)

// Example YAML configurations using GVK format
const appWorkloadYAML = `
apiVersion: generators.gokure.dev/v1alpha1
kind: AppWorkload
metadata:
  name: nginx-app
  namespace: web
spec:
  workload: Deployment
  replicas: 3
  containers:
    - name: nginx
      image: nginx:1.21
      ports:
        - containerPort: 80
          name: http
      resources:
        requests:
          memory: "128Mi"
          cpu: "100m"
        limits:
          memory: "256Mi"
          cpu: "500m"
  services:
    - name: nginx-service
      type: LoadBalancer
      ports:
        - port: 80
          targetPort: 80
          protocol: TCP
`

const fluxHelmYAML = `
apiVersion: generators.gokure.dev/v1alpha1
kind: FluxHelm
metadata:
  name: postgresql
  namespace: database
spec:
  chart:
    name: postgresql
    version: 12.0.0
  source:
    type: HelmRepository
    url: https://charts.bitnami.com/bitnami
    interval: 10m
  values:
    auth:
      database: myapp
      username: myuser
    persistence:
      enabled: true
      size: 10Gi
  release:
    createNamespace: true
    cleanupOnFail: true
  interval: 30m
  timeout: 5m
`

const fluxHelmOCIYAML = `
apiVersion: generators.gokure.dev/v1alpha1
kind: FluxHelm
metadata:
  name: podinfo
  namespace: apps
spec:
  chart:
    name: podinfo
    version: "6.*"
  source:
    type: OCIRepository
    ociUrl: oci://ghcr.io/stefanprodan/charts/podinfo
    interval: 10m
  values:
    replicaCount: 2
    service:
      type: ClusterIP
  interval: 10m
`

// DemoGVKGenerators demonstrates the new GVK-based generator system
func DemoGVKGenerators() {
	fmt.Println("\n=== GVK-Based Generators Demo ===\n")

	// Demo 1: Parse and generate AppWorkload
	fmt.Println("1. AppWorkload Generator:")
	fmt.Println("-" * 40)
	
	var appWrapper stack.ApplicationWrapper
	if err := yaml.Unmarshal([]byte(appWorkloadYAML), &appWrapper); err != nil {
		log.Fatalf("Failed to unmarshal AppWorkload: %v", err)
	}

	fmt.Printf("Parsed: %s %s\n", appWrapper.APIVersion, appWrapper.Kind)
	fmt.Printf("Name: %s, Namespace: %s\n", appWrapper.Metadata.Name, appWrapper.Metadata.Namespace)

	app := appWrapper.ToApplication()
	objects, err := app.Config.Generate(app)
	if err != nil {
		log.Fatalf("Failed to generate AppWorkload resources: %v", err)
	}

	fmt.Printf("Generated %d resources:\n", len(objects))
	for _, obj := range objects {
		o := *obj
		fmt.Printf("  - %s: %s/%s\n", o.GetObjectKind().GroupVersionKind().Kind, 
			o.GetNamespace(), o.GetName())
	}

	// Demo 2: Parse and generate FluxHelm with HelmRepository
	fmt.Println("\n2. FluxHelm Generator (HelmRepository):")
	fmt.Println("-" * 40)

	var helmWrapper stack.ApplicationWrapper
	if err := yaml.Unmarshal([]byte(fluxHelmYAML), &helmWrapper); err != nil {
		log.Fatalf("Failed to unmarshal FluxHelm: %v", err)
	}

	fmt.Printf("Parsed: %s %s\n", helmWrapper.APIVersion, helmWrapper.Kind)
	fmt.Printf("Name: %s, Namespace: %s\n", helmWrapper.Metadata.Name, helmWrapper.Metadata.Namespace)

	helmApp := helmWrapper.ToApplication()
	helmObjects, err := helmApp.Config.Generate(helmApp)
	if err != nil {
		log.Fatalf("Failed to generate FluxHelm resources: %v", err)
	}

	fmt.Printf("Generated %d resources:\n", len(helmObjects))
	for _, obj := range helmObjects {
		o := *obj
		fmt.Printf("  - %s: %s/%s\n", o.GetObjectKind().GroupVersionKind().Kind,
			o.GetNamespace(), o.GetName())
	}

	// Output first resource as YAML
	if len(helmObjects) > 0 {
		fmt.Println("\nSample HelmRepository YAML:")
		fmt.Println("-" * 40)
		yamlStr, _ := io.SerializeObject(*helmObjects[0])
		fmt.Println(yamlStr)
	}

	// Demo 3: Parse and generate FluxHelm with OCIRepository
	fmt.Println("\n3. FluxHelm Generator (OCIRepository):")
	fmt.Println("-" * 40)

	var ociWrapper stack.ApplicationWrapper
	if err := yaml.Unmarshal([]byte(fluxHelmOCIYAML), &ociWrapper); err != nil {
		log.Fatalf("Failed to unmarshal FluxHelm OCI: %v", err)
	}

	fmt.Printf("Parsed: %s %s\n", ociWrapper.APIVersion, ociWrapper.Kind)
	fmt.Printf("Name: %s, Namespace: %s\n", ociWrapper.Metadata.Name, ociWrapper.Metadata.Namespace)

	ociApp := ociWrapper.ToApplication()
	ociObjects, err := ociApp.Config.Generate(ociApp)
	if err != nil {
		log.Fatalf("Failed to generate FluxHelm OCI resources: %v", err)
	}

	fmt.Printf("Generated %d resources:\n", len(ociObjects))
	for _, obj := range ociObjects {
		o := *obj
		fmt.Printf("  - %s: %s/%s\n", o.GetObjectKind().GroupVersionKind().Kind,
			o.GetNamespace(), o.GetName())
	}

	// Demo 4: Show multiple applications in a bundle
	fmt.Println("\n4. Bundle with Multiple Generator Types:")
	fmt.Println("-" * 40)

	bundle := stack.NewBundle("mixed-apps")
	bundle.AddApplication(appWrapper.ToApplication())
	bundle.AddApplication(helmWrapper.ToApplication())
	bundle.AddApplication(ociWrapper.ToApplication())

	bundleObjects, err := bundle.Generate()
	if err != nil {
		log.Fatalf("Failed to generate bundle resources: %v", err)
	}

	fmt.Printf("Bundle generated %d total resources from %d applications\n", 
		len(bundleObjects), len(bundle.Applications))

	// Demo 5: List all registered generator types
	fmt.Println("\n5. Registered Generator Types:")
	fmt.Println("-" * 40)
	
	registeredTypes := stack.generators.ListKinds()
	for _, gvk := range registeredTypes {
		fmt.Printf("  - %s\n", gvk)
	}
}

// RunGVKDemo can be called from main.go to run this demo
func RunGVKDemo() {
	DemoGVKGenerators()
}