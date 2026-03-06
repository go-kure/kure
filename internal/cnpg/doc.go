// Package cnpg provides helpers for building CloudNativePG resources
// such as Database CRs (postgresql.cnpg.io/v1) and ObjectStore CRs
// (barmancloud.cnpg.io/v1).
package cnpg

import (
	// Anchor CNPG dependencies in go.mod. These are used by builder files
	// in this package.
	_ "github.com/cloudnative-pg/cloudnative-pg/api/v1"
	_ "github.com/cloudnative-pg/plugin-barman-cloud/api/v1"
)
