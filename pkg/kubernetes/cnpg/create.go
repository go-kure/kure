package cnpg

import (
	"encoding/json"

	barmanApi "github.com/cloudnative-pg/barman-cloud/pkg/api"
	cnpgv1 "github.com/cloudnative-pg/cloudnative-pg/api/v1"
	machineryapi "github.com/cloudnative-pg/machinery/pkg/api"
	barmanv1 "github.com/cloudnative-pg/plugin-barman-cloud/api/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"

	intcnpg "github.com/go-kure/kure/internal/cnpg"
	"github.com/go-kure/kure/pkg/errors"
)

// ObjectStore converts ObjectStoreOptions to a Barman Cloud ObjectStore object.
func ObjectStore(cfg *ObjectStoreConfig) *barmanv1.ObjectStore {
	if cfg == nil {
		return nil
	}
	opts := cfg.Options
	if opts == nil {
		opts = &ObjectStoreOptions{}
	}
	bos := barmanApi.BarmanObjectStoreConfiguration{
		DestinationPath: opts.DestinationPath,
		EndpointURL:     opts.EndpointURL,
		ServerName:      opts.ServerName,
	}
	if opts.SecretName != "" {
		akKey := opts.AccessKeyIDKey
		if akKey == "" {
			akKey = "ACCESS_KEY_ID"
		}
		sakKey := opts.SecretAccessKeyKey
		if sakKey == "" {
			sakKey = "SECRET_ACCESS_KEY"
		}
		bos.AWS = &barmanApi.S3Credentials{
			AccessKeyIDReference: &machineryapi.SecretKeySelector{
				LocalObjectReference: machineryapi.LocalObjectReference{Name: opts.SecretName},
				Key:                  akKey,
			},
			SecretAccessKeyReference: &machineryapi.SecretKeySelector{
				LocalObjectReference: machineryapi.LocalObjectReference{Name: opts.SecretName},
				Key:                  sakKey,
			},
		}
	}
	spec := barmanv1.ObjectStoreSpec{
		Configuration:   bos,
		RetentionPolicy: opts.RetentionPolicy,
	}
	return intcnpg.CreateObjectStore(cfg.Name, cfg.Namespace, spec)
}

// Database converts DatabaseOptions to a CNPG Database object.
func Database(cfg *DatabaseConfig) *cnpgv1.Database {
	if cfg == nil {
		return nil
	}
	opts := cfg.Options
	if opts == nil {
		opts = &DatabaseOptions{}
	}
	spec := cnpgv1.DatabaseSpec{
		ClusterRef: corev1.LocalObjectReference{Name: opts.ClusterName},
		Name:       opts.DBName,
		Owner:      opts.Owner,
	}
	if opts.Ensure == "absent" {
		spec.Ensure = cnpgv1.EnsureAbsent
	}
	if opts.ReclaimPolicy == "delete" {
		spec.ReclaimPolicy = cnpgv1.DatabaseReclaimDelete
	}
	for _, ext := range opts.Extensions {
		e := cnpgv1.ExtensionSpec{}
		e.Name = ext.Name
		if ext.Ensure == "absent" {
			e.Ensure = cnpgv1.EnsureAbsent
		} else {
			e.Ensure = cnpgv1.EnsurePresent
		}
		spec.Extensions = append(spec.Extensions, e)
	}
	return intcnpg.CreateDatabase(cfg.Name, cfg.Namespace, spec)
}

// Cluster converts ClusterOptions to a CNPG Cluster object.
// It returns an error only if ExternalClusters contains an invalid BarmanObjectStore map.
func Cluster(cfg *ClusterConfig) (*cnpgv1.Cluster, error) {
	if cfg == nil {
		return nil, nil
	}
	opts := cfg.Options
	if opts == nil {
		opts = &ClusterOptions{}
	}

	imageName := opts.ImageName
	enablePDB := opts.Instances > 1
	spec := cnpgv1.ClusterSpec{
		Instances:             int(opts.Instances),
		ImageName:             imageName,
		EnablePDB:             &enablePDB,
		PrimaryUpdateStrategy: cnpgv1.PrimaryUpdateStrategyUnsupervised,
		StorageConfiguration:  cnpgv1.StorageConfiguration{Size: opts.StorageSize},
	}

	if len(opts.InheritedLabels) > 0 || len(opts.InheritedAnnotations) > 0 {
		spec.InheritedMetadata = &cnpgv1.EmbeddedObjectMetadata{
			Labels:      opts.InheritedLabels,
			Annotations: opts.InheritedAnnotations,
		}
	}

	if opts.Resources != nil {
		rr, err := buildResourceRequirements(opts.Resources)
		if err != nil {
			return nil, err
		}
		spec.Resources = rr
	}

	if opts.Backup != nil && (opts.Backup.DestinationPath != "" || opts.Backup.RetentionPolicy != "") {
		bos := &barmanApi.BarmanObjectStoreConfiguration{
			DestinationPath: opts.Backup.DestinationPath,
			EndpointURL:     opts.Backup.EndpointURL,
		}
		if c := opts.Backup.S3Credentials; c != nil && c.SecretName != "" {
			akKey := c.AccessKeyIDKey
			if akKey == "" {
				akKey = "ACCESS_KEY_ID"
			}
			sakKey := c.SecretAccessKeyKey
			if sakKey == "" {
				sakKey = "SECRET_ACCESS_KEY"
			}
			bos.AWS = &barmanApi.S3Credentials{
				AccessKeyIDReference: &machineryapi.SecretKeySelector{
					LocalObjectReference: machineryapi.LocalObjectReference{Name: c.SecretName},
					Key:                  akKey,
				},
				SecretAccessKeyReference: &machineryapi.SecretKeySelector{
					LocalObjectReference: machineryapi.LocalObjectReference{Name: c.SecretName},
					Key:                  sakKey,
				},
			}
		}
		spec.Backup = &cnpgv1.BackupConfiguration{
			RetentionPolicy:   opts.Backup.RetentionPolicy,
			BarmanObjectStore: bos,
		}
	}

	if opts.Monitoring != nil && opts.Monitoring.EnablePodMonitor {
		mon := &cnpgv1.MonitoringConfiguration{EnablePodMonitor: true}
		for _, cq := range opts.Monitoring.CustomQueriesConfigMap {
			mon.CustomQueriesConfigMap = append(mon.CustomQueriesConfigMap, cnpgv1.ConfigMapKeySelector{
				LocalObjectReference: machineryapi.LocalObjectReference{Name: cq.Name},
				Key:                  cq.Key,
			})
		}
		spec.Monitoring = mon
	}

	if opts.Bootstrap != nil {
		if opts.Bootstrap.RecoverySource != "" {
			spec.Bootstrap = &cnpgv1.BootstrapConfiguration{
				Recovery: &cnpgv1.BootstrapRecovery{Source: opts.Bootstrap.RecoverySource},
			}
		} else if opts.Bootstrap.PgBasebackupSource != "" {
			spec.Bootstrap = &cnpgv1.BootstrapConfiguration{
				PgBaseBackup: &cnpgv1.BootstrapPgBaseBackup{Source: opts.Bootstrap.PgBasebackupSource},
			}
		}
	}

	if len(opts.ExternalClusters) > 0 {
		extClusters := make([]cnpgv1.ExternalCluster, 0, len(opts.ExternalClusters))
		for _, ec := range opts.ExternalClusters {
			extCluster := cnpgv1.ExternalCluster{
				Name:                 ec.Name,
				ConnectionParameters: ec.ConnectionParameters,
			}
			if ec.BarmanObjectStore != nil {
				data, err := json.Marshal(ec.BarmanObjectStore)
				if err != nil {
					return nil, errors.Wrapf(err, "external cluster %q: marshal barman object store", ec.Name)
				}
				var bos barmanApi.BarmanObjectStoreConfiguration
				if err := json.Unmarshal(data, &bos); err != nil {
					return nil, errors.Wrapf(err, "external cluster %q: unmarshal barman object store", ec.Name)
				}
				extCluster.BarmanObjectStore = &bos
			}
			extClusters = append(extClusters, extCluster)
		}
		spec.ExternalClusters = extClusters
	}

	pgConfig := cnpgv1.PostgresConfiguration{}
	if len(opts.PostgresParams) > 0 {
		pgConfig.Parameters = opts.PostgresParams
	}
	if opts.Synchronous != nil && opts.Synchronous.Method != "" {
		sync := &cnpgv1.SynchronousReplicaConfiguration{
			Method: cnpgv1.SynchronousReplicaConfigurationMethod(opts.Synchronous.Method),
			Number: int(opts.Synchronous.Number),
		}
		if opts.Synchronous.DataDurability != "" {
			sync.DataDurability = cnpgv1.DataDurabilityLevel(opts.Synchronous.DataDurability)
		}
		pgConfig.Synchronous = sync
	}
	if pgConfig.Parameters != nil || pgConfig.Synchronous != nil {
		spec.PostgresConfiguration = pgConfig
	}

	if opts.ObjectStoreName != "" {
		isWALArchiver := true
		spec.Plugins = []cnpgv1.PluginConfiguration{
			{
				Name:          "barman-cloud.barmancloud.cnpg.io",
				IsWALArchiver: &isWALArchiver,
				Parameters:    map[string]string{"objectStoreName": opts.ObjectStoreName},
			},
		}
	}

	if opts.Affinity != nil {
		enablePAA := opts.Affinity.EnablePodAntiAffinity
		spec.Affinity = cnpgv1.AffinityConfiguration{
			EnablePodAntiAffinity: &enablePAA,
			TopologyKey:           opts.Affinity.TopologyKey,
			PodAntiAffinityType:   opts.Affinity.PodAntiAffinityType,
			NodeSelector:          opts.Affinity.NodeSelector,
		}
	}

	if len(opts.ManagedRoles) > 0 {
		roles := make([]cnpgv1.RoleConfiguration, 0, len(opts.ManagedRoles))
		for _, role := range opts.ManagedRoles {
			rc := cnpgv1.RoleConfiguration{
				Name:        role.Name,
				Comment:     role.Comment,
				Login:       role.Login,
				Superuser:   role.Superuser,
				CreateDB:    role.CreateDB,
				CreateRole:  role.CreateRole,
				Replication: role.Replication,
				Inherit:     role.Inherit,
				InRoles:     role.InRoles,
			}
			if role.ConnectionLimit != nil {
				rc.ConnectionLimit = *role.ConnectionLimit
			}
			if role.Ensure == "absent" {
				rc.Ensure = cnpgv1.EnsureAbsent
			}
			if role.PasswordSecret != "" {
				rc.PasswordSecret = &cnpgv1.LocalObjectReference{Name: role.PasswordSecret}
			}
			roles = append(roles, rc)
		}
		spec.Managed = &cnpgv1.ManagedConfiguration{Roles: roles}
	}

	return intcnpg.CreateCluster(cfg.Name, cfg.Namespace, spec), nil
}

// ScheduledBackup converts the config to a CNPG ScheduledBackup object.
func ScheduledBackup(cfg *ScheduledBackupConfig) *cnpgv1.ScheduledBackup {
	if cfg == nil {
		return nil
	}
	return intcnpg.CreateScheduledBackup(cfg.Name, cfg.Namespace, cfg.Spec)
}

func buildResourceRequirements(r *ResourceOptions) (corev1.ResourceRequirements, error) {
	rr := corev1.ResourceRequirements{}
	if r.RequestsCPU != "" || r.RequestsMemory != "" {
		rr.Requests = corev1.ResourceList{}
		if r.RequestsCPU != "" {
			q, err := resource.ParseQuantity(r.RequestsCPU)
			if err != nil {
				return rr, errors.Wrapf(err, "invalid cpu request %q", r.RequestsCPU)
			}
			rr.Requests[corev1.ResourceCPU] = q
		}
		if r.RequestsMemory != "" {
			q, err := resource.ParseQuantity(r.RequestsMemory)
			if err != nil {
				return rr, errors.Wrapf(err, "invalid memory request %q", r.RequestsMemory)
			}
			rr.Requests[corev1.ResourceMemory] = q
		}
	}
	if r.LimitsCPU != "" || r.LimitsMemory != "" {
		rr.Limits = corev1.ResourceList{}
		if r.LimitsCPU != "" {
			q, err := resource.ParseQuantity(r.LimitsCPU)
			if err != nil {
				return rr, errors.Wrapf(err, "invalid cpu limit %q", r.LimitsCPU)
			}
			rr.Limits[corev1.ResourceCPU] = q
		}
		if r.LimitsMemory != "" {
			q, err := resource.ParseQuantity(r.LimitsMemory)
			if err != nil {
				return rr, errors.Wrapf(err, "invalid memory limit %q", r.LimitsMemory)
			}
			rr.Limits[corev1.ResourceMemory] = q
		}
	}
	return rr, nil
}
