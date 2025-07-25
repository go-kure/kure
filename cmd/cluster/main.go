package main

import (
	"log"
	"os"

	"github.com/go-kure/kure/pkg/cluster"
	"github.com/go-kure/kure/pkg/cluster/api"
	"github.com/go-kure/kure/pkg/fluxcd"
	"github.com/go-kure/kure/pkg/kio"
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
	c.SetFilePer(api.FilePerResource)

	app := api.AppDeploymentConfig{
		Name:     "my-app",
		Image:    "ghcr.io/my-org/my-app:v1",
		Ports:    []int{80},
		Replicas: ptr(2),
		Ingress: &api.IngressConfig{
			Host:   "my-app.example.com",
			TLS:    true,
			Issuer: "letsencrypt",
		},
	}

	group := api.AppGroup{
		Name:      "apps",
		Namespace: "default",
		Apps:      []api.AppDeploymentConfig{app},
	}

	c.AddAppSet(cluster.AppSet{AppGroup: group})

	if err := kio.Marshal(os.Stdout, c); err != nil {
		log.Fatalf("failed to marshal cluster: %v", err)
	}
}
