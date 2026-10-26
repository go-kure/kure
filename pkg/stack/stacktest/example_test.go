package stacktest_test

import (
	"fmt"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/go-kure/kure/pkg/stack"
	"github.com/go-kure/kure/pkg/stack/stacktest"
)

// cachingConfig keeps the labels map it builds and hands the same map to
// every ConfigMap it generates.
type cachingConfig struct {
	labels map[string]string
}

func (c *cachingConfig) Generate(*stack.Application) ([]*client.Object, error) {
	if c.labels == nil {
		c.labels = map[string]string{"app": "web"}
	}
	var cm client.Object = &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: "web", Labels: c.labels},
	}
	return []*client.Object{&cm}, nil
}

// CheckGeneration finds the labels map the two results share: a label kure
// or the caller adds to one ConfigMap would appear on the other.
func ExampleCheckGeneration() {
	app := stack.NewApplication("web", "default", &cachingConfig{})
	report, err := stacktest.CheckGeneration(app)
	if err != nil {
		fmt.Println(err)
		return
	}
	for _, s := range report.Shared {
		fmt.Printf("%s shared: %s\n", s.Kind, s.First)
	}
	// Output:
	// map shared: [0].(*v1.ConfigMap).ObjectMeta.Labels
}

// wrapped is a wrapper that embeds the ApplicationConfig interface, so the
// wrapped config's Validate is no longer found.
type wrapped struct{ stack.ApplicationConfig }

type validatedConfig struct{ cachingConfig }

func (*validatedConfig) Validate() error { return nil }

// InterfacesOf compares a wrapper with the config it wraps.
func ExampleInterfacesOf() {
	inner := &validatedConfig{}
	missing := stacktest.InterfacesOf(wrapped{inner}).Missing(stacktest.InterfacesOf(inner))
	fmt.Println(missing)
	// Output:
	// [stack.Validator]
}
