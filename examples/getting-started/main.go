// Package main demonstrates the complete Kure pipeline from Cluster definition
// to on-disk manifest directory. It builds a Cluster using the fluent
// ClusterBuilder, registers applications with custom ApplicationConfig
// implementations, runs the FluxCD workflow engine, produces a Layout, and
// writes the result to a structured manifest directory.
//
// Run with:
//
//	go run ./examples/getting-started/
package main

import (
	"os"
	"path/filepath"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/go-kure/kure/pkg/errors"
	pkgkubernetes "github.com/go-kure/kure/pkg/kubernetes"
	"github.com/go-kure/kure/pkg/logger"
	"github.com/go-kure/kure/pkg/stack"
	"github.com/go-kure/kure/pkg/stack/fluxcd"
	"github.com/go-kure/kure/pkg/stack/layout"
)

var log = logger.Default()

func main() {
	if err := run(); err != nil {
		log.Error("error: %v", err)
		os.Exit(1)
	}
}

func run() error {
	// ---------------------------------------------------------------
	// Step 1: Build a Cluster using the fluent ClusterBuilder API.
	//
	// The builder creates a hierarchical structure:
	//   cluster "staging"
	//     node "infrastructure"
	//       bundle "platform-services"
	//         application "redis"   (RedisConfig)
	//         application "web-app" (WebAppConfig)
	// ---------------------------------------------------------------
	log.Info("Step 1: Building cluster with ClusterBuilder...")

	sourceRef := &stack.SourceRef{
		Kind:      "OCIRepository",
		Name:      "manifests",
		Namespace: "flux-system",
	}

	cluster, err := stack.NewClusterBuilder("staging").
		WithGitOps(&stack.GitOpsConfig{Type: "flux"}).
		WithNode("infrastructure").
		WithBundle("platform-services").
		WithApplication("redis", &RedisConfig{
			Namespace: "cache",
			Image:     "redis:7-alpine",
		}).
		WithApplication("web-app", &WebAppConfig{
			Namespace: "web",
			Image:     "nginx:1.27-alpine",
			Replicas:  2,
			Port:      80,
		}).
		WithSourceRef(sourceRef).
		End(). // end bundle -> back to node "infrastructure"
		End(). // end node -> back to cluster builder
		Build()
	if err != nil {
		return errors.Wrap(err, "build cluster")
	}

	log.Info("Cluster %q built with root node %q", cluster.Name, cluster.Node.Name)

	// ---------------------------------------------------------------
	// Step 2: Create the FluxCD workflow engine.
	//
	// NewWorkflowEngineWithConfig lets us set the Kustomization mode
	// and Flux placement strategy. FluxSeparate places Flux resources
	// in a dedicated directory rather than alongside manifests.
	// ---------------------------------------------------------------
	log.Info("Step 2: Creating FluxCD workflow engine...")

	wf := fluxcd.NewWorkflowEngineWithConfig(
		layout.KustomizationExplicit,
		layout.FluxSeparate,
	)

	// ---------------------------------------------------------------
	// Step 3: Run the workflow to produce a ManifestLayout.
	//
	// CreateLayoutWithResources walks the cluster tree, generates
	// Kubernetes resources from each ApplicationConfig, adds Flux
	// Kustomization resources, and returns a ManifestLayout tree.
	// ---------------------------------------------------------------
	log.Info("Step 3: Running workflow to produce layout...")

	rules := layout.DefaultLayoutRules()
	rules.ClusterName = cluster.Name

	result, err := wf.CreateLayoutWithResources(cluster, rules)
	if err != nil {
		return errors.Wrap(err, "create layout")
	}

	ml, ok := result.(*layout.ManifestLayout)
	if !ok {
		return errors.Errorf("unexpected result type from CreateLayoutWithResources")
	}

	log.Info("Layout produced with %d top-level children", len(ml.Children))

	// ---------------------------------------------------------------
	// Step 4: Write the layout to disk.
	//
	// WriteManifest serialises each resource as YAML and creates
	// kustomization.yaml files so Flux can reconcile the directory.
	// ---------------------------------------------------------------
	log.Info("Step 4: Writing manifests to disk...")

	outputDir, err := outputDirectory()
	if err != nil {
		return errors.Wrap(err, "prepare output directory")
	}

	cfg := layout.DefaultLayoutConfig()
	if err := layout.WriteManifest(outputDir, cfg, ml); err != nil {
		return errors.Wrap(err, "write manifests")
	}

	log.Info("Manifests written to: %s", outputDir)
	log.Info("Done! Inspect the output directory to see the generated manifests.")

	return nil
}

// ---------------------------------------------------------------
// ApplicationConfig implementations
//
// Each type below implements stack.ApplicationConfig by providing
// a Generate method that returns Kubernetes objects.
// ---------------------------------------------------------------

// RedisConfig generates a Deployment for a Redis instance.
type RedisConfig struct {
	Namespace string
	Image     string
}

// Generate produces a single-replica Redis Deployment.
func (c *RedisConfig) Generate(app *stack.Application) ([]*client.Object, error) {
	labels := map[string]string{"app": app.Name}
	replicas := int32(1)

	dep := &appsv1.Deployment{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "apps/v1",
			Kind:       "Deployment",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      app.Name,
			Namespace: c.Namespace,
			Labels:    labels,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{MatchLabels: labels},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: labels},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{{
						Name:  app.Name,
						Image: c.Image,
						Ports: []corev1.ContainerPort{{
							ContainerPort: 6379,
							Protocol:      corev1.ProtocolTCP,
						}},
					}},
				},
			},
		},
	}

	return []*client.Object{pkgkubernetes.ToClientObject(dep)}, nil
}

// WebAppConfig generates a Deployment and a Service for a web application.
type WebAppConfig struct {
	Namespace string
	Image     string
	Replicas  int32
	Port      int32
}

// Generate produces a Deployment and a ClusterIP Service.
func (c *WebAppConfig) Generate(app *stack.Application) ([]*client.Object, error) {
	labels := map[string]string{"app": app.Name}

	dep := &appsv1.Deployment{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "apps/v1",
			Kind:       "Deployment",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      app.Name,
			Namespace: c.Namespace,
			Labels:    labels,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &c.Replicas,
			Selector: &metav1.LabelSelector{MatchLabels: labels},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: labels},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{{
						Name:  app.Name,
						Image: c.Image,
						Ports: []corev1.ContainerPort{{
							Name:          "http",
							ContainerPort: c.Port,
							Protocol:      corev1.ProtocolTCP,
						}},
					}},
				},
			},
		},
	}

	svc := &corev1.Service{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Service",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      app.Name,
			Namespace: c.Namespace,
			Labels:    labels,
		},
		Spec: corev1.ServiceSpec{
			Selector: labels,
			Ports: []corev1.ServicePort{{
				Name:     "http",
				Port:     c.Port,
				Protocol: corev1.ProtocolTCP,
			}},
		},
	}

	return []*client.Object{
		pkgkubernetes.ToClientObject(dep),
		pkgkubernetes.ToClientObject(svc),
	}, nil
}

// outputDirectory returns the path where manifests will be written.
// When the OUT_DIR environment variable is set it is used as-is;
// otherwise a temporary directory is created.
func outputDirectory() (string, error) {
	if dir := os.Getenv("OUT_DIR"); dir != "" {
		if err := os.MkdirAll(filepath.Clean(dir), 0755); err != nil { //nolint:gosec // G703: CLI tool, output dir from env
			return "", err
		}
		return dir, nil
	}

	dir, err := os.MkdirTemp("", "kure-getting-started-*")
	if err != nil {
		return "", err
	}

	// Print the temp dir path so users know where to find the output.
	abs, err := filepath.Abs(dir)
	if err != nil {
		abs = dir
	}
	log.Info("(using temp directory: %s)", abs)
	return dir, nil
}
