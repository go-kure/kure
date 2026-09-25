package io_test

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/go-kure/kure/pkg/io"
	"github.com/go-kure/kure/pkg/kubernetes"
)

// The README's Go blocks are generated from these functions
// (scripts/gen-doc-examples.sh); each prints what it read or wrote so go test
// checks the example as well as compiling it.

func ExampleParseFile() {
	// Parse a multi-document YAML file into typed Kubernetes objects
	objects, err := io.ParseFile("testdata/manifests.yaml")
	if err != nil {
		panic(err)
	}

	// Parse YAML bytes directly
	yamlData := []byte("apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: app-config\n")
	more, err := io.ParseYAML(yamlData)
	if err != nil {
		panic(err)
	}

	for _, obj := range append(objects, more...) {
		fmt.Printf("%T %s\n", obj, obj.GetName())
	}
	// Output:
	// *v1.Deployment web
	// *v1.Service web
	// *v1.ConfigMap app-config
}

func ExampleParseYAMLWithOptions() {
	yamlData := []byte(`apiVersion: v1
kind: Pod
metadata:
  name: web
---
apiVersion: example.com/v1
kind: Widget
metadata:
  name: my-widget
`)

	opts := io.ParseOptions{AllowUnstructured: true}
	objects, err := io.ParseYAMLWithOptions(yamlData, opts)
	if err != nil {
		panic(err)
	}
	// Known types are returned as typed objects (e.g. *corev1.Pod).
	// Unknown types are returned as *unstructured.Unstructured.
	for _, obj := range objects {
		fmt.Printf("%T\n", obj)
	}
	// Output:
	// *v1.Pod
	// *unstructured.Unstructured
}

func ExampleSaveFile() {
	dir, err := os.MkdirTemp("", "kure-io-example")
	if err != nil {
		panic(err)
	}
	defer func() { _ = os.RemoveAll(dir) }()
	path := filepath.Join(dir, "service.yaml")

	// Save an object to file
	service := kubernetes.CreateService("web", "default")
	if err := io.SaveFile(path, service); err != nil {
		panic(err)
	}

	// Load a single object from file
	var loaded corev1.Service
	if err := io.LoadFile(path, &loaded); err != nil {
		panic(err)
	}
	fmt.Println(loaded.Kind, loaded.Namespace+"/"+loaded.Name)
	// Output: Service default/web
}

func ExampleMarshal() {
	deployment := kubernetes.CreateDeployment("web", "default")

	// Serialize to YAML
	var buf bytes.Buffer
	if err := io.Marshal(&buf, deployment); err != nil {
		panic(err)
	}

	// Deserialize from YAML
	var obj appsv1.Deployment
	if err := io.Unmarshal(&buf, &obj); err != nil {
		panic(err)
	}
	fmt.Println(obj.Kind, obj.Namespace+"/"+obj.Name)
	// Output: Deployment default/web
}

func ExampleEncodeObjectsToYAML() {
	var configMap client.Object = kubernetes.CreateConfigMap("app-config", "default")
	var service client.Object = kubernetes.CreateService("web", "default")
	objects := []*client.Object{&configMap, &service}

	// Encode as multi-document YAML
	yamlData, err := io.EncodeObjectsToYAML(objects)
	if err != nil {
		panic(err)
	}

	// Encode as JSON: one object per line, each followed by a --- separator
	jsonData, err := io.EncodeObjectsToJSON(objects)
	if err != nil {
		panic(err)
	}

	fmt.Print(string(yamlData))
	fmt.Print(string(jsonData))
	// Output:
	// apiVersion: v1
	// kind: ConfigMap
	// metadata:
	//   name: app-config
	//   namespace: default
	// ---
	// apiVersion: v1
	// kind: Service
	// metadata:
	//   name: web
	//   namespace: default
	// spec: {}
	// {"kind":"ConfigMap","apiVersion":"v1","metadata":{"name":"app-config","namespace":"default"}}
	// ---
	// {"kind":"Service","apiVersion":"v1","metadata":{"name":"web","namespace":"default"},"spec":{},"status":{"loadBalancer":{}}}
	// ---
}

func ExampleEncodeObjectsToYAMLWithOptions() {
	configMap := kubernetes.CreateConfigMap("app-config", "default")
	configMap.Data = map[string]string{"LOG_LEVEL": "info"}
	var obj client.Object = configMap
	objects := []*client.Object{&obj}

	// Encode with Kubernetes-conventional field ordering:
	// apiVersion, kind, metadata, spec, ... status (last)
	opts := io.EncodeOptions{KubernetesFieldOrder: true}
	yamlData, err := io.EncodeObjectsToYAMLWithOptions(objects, opts)
	if err != nil {
		panic(err)
	}
	fmt.Print(string(yamlData))
	// Output:
	// apiVersion: v1
	// kind: ConfigMap
	// metadata:
	//   name: app-config
	//   namespace: default
	// data:
	//   LOG_LEVEL: info
}

func ExampleServerFieldStripping() {
	// A Service as read back from a cluster: a resourceVersion, and an empty status
	service := kubernetes.CreateService("web", "default")
	service.ResourceVersion = "12345"
	var obj client.Object = service
	objects := []*client.Object{&obj}

	for _, opts := range []io.EncodeOptions{
		// Default behavior — full stripping (the zero value, which EncodeObjectsToYAML uses)
		{},
		// Explicit full stripping with field ordering
		{KubernetesFieldOrder: true, ServerFieldStripping: io.StripServerFieldsFull},
		// Basic stripping (only null creationTimestamp and empty status)
		{ServerFieldStripping: io.StripServerFieldsBasic},
		// No stripping — preserve all fields as-is
		{ServerFieldStripping: io.StripServerFieldsNone},
	} {
		yamlData, err := io.EncodeObjectsToYAMLWithOptions(objects, opts)
		if err != nil {
			panic(err)
		}
		out := string(yamlData)
		fmt.Println(strings.Contains(out, "resourceVersion:"), strings.Contains(out, "status:"))
	}
	// Output:
	// false false
	// false false
	// true false
	// true true
}

func ExampleNewResourcePrinter() {
	configMap := kubernetes.CreateConfigMap("app-config", "default")
	configMap.Labels = map[string]string{"app": "web"}
	var obj client.Object = configMap
	objects := []*client.Object{&obj}

	// Print as YAML to stdout
	if err := io.PrintObjectsAsYAML(objects, os.Stdout); err != nil {
		panic(err)
	}

	// Print as table
	if err := io.PrintObjectsAsTable(objects, false, false, os.Stdout); err != nil {
		panic(err)
	}

	// Use ResourcePrinter for configurable output
	printer := io.NewResourcePrinter(io.PrintOptions{
		OutputFormat: io.OutputFormatTable,
		ShowLabels:   true,
	})
	if err := printer.Print(objects, os.Stdout); err != nil {
		panic(err)
	}

	// Validate a format string before use
	format, err := io.ValidateOutputFormat("name")
	if err != nil {
		panic(err)
	}

	// Format-agnostic printing (selects the printer by format)
	if err := io.PrintObjects(objects, format, io.PrintOptions{}, os.Stdout); err != nil {
		panic(err)
	}
	// Output:
	// apiVersion: v1
	// kind: ConfigMap
	// metadata:
	//   labels:
	//     app: web
	//   name: app-config
	//   namespace: default
	// NAMESPACE  NAME        READY    AGE
	// default    app-config  Unknown  106752d
	// NAMESPACE   NAME         AGE         LABELS
	// default     app-config   <unknown>   app=web
	// configmap/app-config (namespace: default)
}
