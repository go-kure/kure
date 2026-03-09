package cnpg

import (
	"testing"

	barmanapi "github.com/cloudnative-pg/barman-cloud/pkg/api"
	machineryapi "github.com/cloudnative-pg/machinery/pkg/api"
	barmanv1 "github.com/cloudnative-pg/plugin-barman-cloud/api/v1"
	corev1 "k8s.io/api/core/v1"
)

func TestCreateObjectStore(t *testing.T) {
	spec := barmanv1.ObjectStoreSpec{
		Configuration: barmanapi.BarmanObjectStoreConfiguration{
			DestinationPath: "s3://my-bucket/backups",
		},
		RetentionPolicy: "60d",
	}
	os := CreateObjectStore("backup-store", "postgres-system", spec)

	if os == nil {
		t.Fatal("expected non-nil ObjectStore")
	}
	if os.Name != "backup-store" {
		t.Errorf("expected name %q, got %q", "backup-store", os.Name)
	}
	if os.Namespace != "postgres-system" {
		t.Errorf("expected namespace %q, got %q", "postgres-system", os.Namespace)
	}
	if os.Kind != "ObjectStore" {
		t.Errorf("expected kind %q, got %q", "ObjectStore", os.Kind)
	}
	if os.APIVersion != "barmancloud.cnpg.io/v1" {
		t.Errorf("expected apiVersion %q, got %q", "barmancloud.cnpg.io/v1", os.APIVersion)
	}
	if os.Spec.Configuration.DestinationPath != "s3://my-bucket/backups" {
		t.Errorf("destination path mismatch")
	}
	if os.Spec.RetentionPolicy != "60d" {
		t.Errorf("retention policy mismatch")
	}
}

func TestObjectStoreFunctions(t *testing.T) {
	spec := barmanv1.ObjectStoreSpec{
		Configuration: barmanapi.BarmanObjectStoreConfiguration{
			DestinationPath: "s3://bucket/path",
		},
	}
	os := CreateObjectStore("store", "ns", spec)

	AddObjectStoreLabel(os, "app", "backup")
	if os.Labels["app"] != "backup" {
		t.Errorf("label not set")
	}

	AddObjectStoreAnnotation(os, "team", "dba")
	if os.Annotations["team"] != "dba" {
		t.Errorf("annotation not set")
	}

	SetObjectStoreDestinationPath(os, "s3://new-bucket/backups")
	if os.Spec.Configuration.DestinationPath != "s3://new-bucket/backups" {
		t.Errorf("destination path not updated")
	}

	SetObjectStoreEndpointURL(os, "https://s3.example.com")
	if os.Spec.Configuration.EndpointURL != "https://s3.example.com" {
		t.Errorf("endpoint URL not set")
	}

	creds := &barmanapi.S3Credentials{
		AccessKeyIDReference: &machineryapi.SecretKeySelector{
			LocalObjectReference: machineryapi.LocalObjectReference{Name: "aws-creds"},
			Key:                  "ACCESS_KEY_ID",
		},
		SecretAccessKeyReference: &machineryapi.SecretKeySelector{
			LocalObjectReference: machineryapi.LocalObjectReference{Name: "aws-creds"},
			Key:                  "SECRET_ACCESS_KEY",
		},
		RegionReference: &machineryapi.SecretKeySelector{
			LocalObjectReference: machineryapi.LocalObjectReference{Name: "aws-creds"},
			Key:                  "REGION",
		},
	}
	SetObjectStoreS3Credentials(os, creds)
	if os.Spec.Configuration.AWS == nil {
		t.Fatal("S3 credentials not set")
	}
	if os.Spec.Configuration.AWS.AccessKeyIDReference.Name != "aws-creds" {
		t.Errorf("access key secret name mismatch")
	}

	SetObjectStoreRetentionPolicy(os, "30d")
	if os.Spec.RetentionPolicy != "30d" {
		t.Errorf("retention policy not set")
	}

	walCfg := &barmanapi.WalBackupConfiguration{
		Compression: "gzip",
		MaxParallel: 4,
	}
	SetObjectStoreWalConfig(os, walCfg)
	if os.Spec.Configuration.Wal == nil || os.Spec.Configuration.Wal.Compression != "gzip" {
		t.Errorf("WAL config not set")
	}

	jobs := int32(4)
	dataCfg := &barmanapi.DataBackupConfiguration{
		Compression: "gzip",
		Jobs:        &jobs,
	}
	SetObjectStoreDataConfig(os, dataCfg)
	if os.Spec.Configuration.Data == nil || os.Spec.Configuration.Data.Compression != "gzip" {
		t.Errorf("data config not set")
	}

	envVar := corev1.EnvVar{Name: "AWS_REGION", Value: "eu-west-1"}
	AddObjectStoreEnvVar(os, envVar)
	if len(os.Spec.InstanceSidecarConfiguration.Env) != 1 {
		t.Errorf("expected 1 env var, got %d", len(os.Spec.InstanceSidecarConfiguration.Env))
	}
}
