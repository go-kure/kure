package cnpg_test

import (
	"fmt"

	cnpgv1 "github.com/cloudnative-pg/cloudnative-pg/api/v1"
	corev1 "k8s.io/api/core/v1"

	"github.com/go-kure/kure/internal/cnpg"
)

// This example demonstrates creating a CNPG Database CR with extensions,
// which is the primary pattern for managing PostgreSQL databases via
// CloudNativePG.
func Example_composeDatabaseWithExtensions() {
	db := cnpg.CreateDatabase("app-db", "postgres-system", cnpgv1.DatabaseSpec{
		Name:       "app",
		Owner:      "app_user",
		ClusterRef: corev1.LocalObjectReference{Name: "pg-cluster"},
	})

	_ = cnpg.SetDatabaseReclaimPolicy(db, cnpgv1.DatabaseReclaimRetain)
	_ = cnpg.AddDatabaseLabel(db, "app", "myapp")

	_ = cnpg.AddDatabaseExtension(db, cnpgv1.ExtensionSpec{
		DatabaseObjectSpec: cnpgv1.DatabaseObjectSpec{
			Name:   "pg_stat_statements",
			Ensure: cnpgv1.EnsurePresent,
		},
	})
	_ = cnpg.AddDatabaseExtension(db, cnpgv1.ExtensionSpec{
		DatabaseObjectSpec: cnpgv1.DatabaseObjectSpec{
			Name:   "pgcrypto",
			Ensure: cnpgv1.EnsurePresent,
		},
	})

	fmt.Println("Name:", db.Name)
	fmt.Println("Kind:", db.Kind)
	fmt.Println("APIVersion:", db.APIVersion)
	fmt.Println("DB Name:", db.Spec.Name)
	fmt.Println("Owner:", db.Spec.Owner)
	fmt.Println("Cluster:", db.Spec.ClusterRef.Name)
	fmt.Println("ReclaimPolicy:", db.Spec.ReclaimPolicy)
	fmt.Println("Extensions:", len(db.Spec.Extensions))
	fmt.Println("Ext[0]:", db.Spec.Extensions[0].Name)
	fmt.Println("Ext[1]:", db.Spec.Extensions[1].Name)
	// Output:
	// Name: app-db
	// Kind: Database
	// APIVersion: postgresql.cnpg.io/v1
	// DB Name: app
	// Owner: app_user
	// Cluster: pg-cluster
	// ReclaimPolicy: retain
	// Extensions: 2
	// Ext[0]: pg_stat_statements
	// Ext[1]: pgcrypto
}
