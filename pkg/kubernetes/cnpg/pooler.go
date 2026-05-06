package cnpg

import (
	cnpgv1 "github.com/cloudnative-pg/cloudnative-pg/api/v1"

	intcnpg "github.com/go-kure/kure/internal/cnpg"
)

// Pooler converts PoolerConfig to a CNPG Pooler object.
func Pooler(cfg *PoolerConfig) *cnpgv1.Pooler {
	if cfg == nil {
		return nil
	}
	opts := cfg.Options
	if opts == nil {
		opts = &PoolerOptions{}
	}

	poolerType := cnpgv1.PoolerTypeRW
	if opts.Type == "ro" {
		poolerType = cnpgv1.PoolerTypeRO
	}

	pgBouncer := &cnpgv1.PgBouncerSpec{}
	if opts.PgBouncer != nil {
		pgBouncer = opts.PgBouncer
	}

	spec := cnpgv1.PoolerSpec{
		Cluster:   cnpgv1.LocalObjectReference{Name: opts.ClusterName},
		Type:      poolerType,
		PgBouncer: pgBouncer,
	}
	if opts.Instances > 0 {
		instances := opts.Instances
		spec.Instances = &instances
	}

	return intcnpg.CreatePooler(cfg.Name, cfg.Namespace, spec)
}
