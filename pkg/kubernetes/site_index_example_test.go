package kubernetes_test

import (
	"os"

	"github.com/go-kure/kure/pkg/io"
	"github.com/go-kure/kure/pkg/kubernetes"
)

// The Go block of the site's landing page (site/content/_index.md) is
// generated from this function (scripts/gen-doc-examples.sh).

func Example_siteIndex() {
	cm := kubernetes.CreateConfigMap("app-config", "default")
	kubernetes.AddConfigMapData(cm, "DATABASE_HOST", "postgres.db.svc")
	kubernetes.AddConfigMapData(cm, "DATABASE_PORT", "5432")

	if err := io.Marshal(os.Stdout, cm); err != nil {
		panic(err)
	}
	// Output:
	// apiVersion: v1
	// data:
	//   DATABASE_HOST: postgres.db.svc
	//   DATABASE_PORT: "5432"
	// kind: ConfigMap
	// metadata:
	//   name: app-config
	//   namespace: default
}
