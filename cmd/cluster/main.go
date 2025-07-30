package main

import (
	"log"
	"os"

	"github.com/go-kure/kure/pkg/application"
	"github.com/go-kure/kure/pkg/cluster"
	"github.com/go-kure/kure/pkg/fluxcd"
	"github.com/go-kure/kure/pkg/kio"
	"github.com/go-kure/kure/pkg/layout"
)

func ptr[T any](v T) *T { return &v }

func main() {
	c := cluster.NewCluster("prod", "10m", "flux-system", &fluxcd.OCIRepositoryConfig{
		Name:      "flux-system",
		Namespace: "flux-system",
		URL:       "oci://ghcr.io/my-org/flux-manifests",
		Ref:       "main",
		Interval:  "10m",
	})
	c.SetFilePer(layout.FilePerResource)

	app := application.AppWorkloadConfig{
		Name:     "my-app",
		Image:    "ghcr.io/my-org/my-app:v1",
		Ports:    []int{80},
		Replicas: ptr(2),
		Ingress: &application.IngressConfig{
			Host:   "my-app.example.com",
			TLS:    true,
			Issuer: "letsencrypt",
		},
	}

	group := layout.AppGroup{
		Name:      "apps",
		Namespace: "default",
		Apps:      []application.AppWorkloadConfig{app},
	}

	c.AddAppSet(cluster.AppSet{AppGroup: group})

	if err := kio.Marshal(os.Stdout, c); err != nil {
		log.Fatalf("failed to marshal cluster: %v", err)
	}
}
