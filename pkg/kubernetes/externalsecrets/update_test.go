package externalsecrets

import (
	"testing"
	"time"

	esv1 "github.com/external-secrets/external-secrets/apis/externalsecrets/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestAddExternalSecretLabel(t *testing.T) {
	es := ExternalSecret(&ExternalSecretConfig{
		Name:      "test",
		Namespace: "default",
	})

	AddExternalSecretLabel(es, "app", "myapp")

	if es.Labels["app"] != "myapp" {
		t.Errorf("expected label 'app'='myapp', got %s", es.Labels["app"])
	}
}

func TestAddExternalSecretAnnotation(t *testing.T) {
	es := ExternalSecret(&ExternalSecretConfig{
		Name:      "test",
		Namespace: "default",
	})

	AddExternalSecretAnnotation(es, "note", "test-annotation")

	if es.Annotations["note"] != "test-annotation" {
		t.Errorf("expected annotation 'note'='test-annotation', got %s", es.Annotations["note"])
	}
}

func TestAddExternalSecretData(t *testing.T) {
	es := ExternalSecret(&ExternalSecretConfig{
		Name:      "test",
		Namespace: "default",
	})

	data := esv1.ExternalSecretData{
		SecretKey: "api-key",
		RemoteRef: esv1.ExternalSecretDataRemoteRef{
			Key: "secret/api-key",
		},
	}

	AddExternalSecretData(es, data)

	if len(es.Spec.Data) != 1 {
		t.Fatalf("expected 1 data entry, got %d", len(es.Spec.Data))
	}
	if es.Spec.Data[0].SecretKey != "api-key" {
		t.Errorf("expected SecretKey 'api-key', got %s", es.Spec.Data[0].SecretKey)
	}
}

func TestAddSecretStoreLabel(t *testing.T) {
	ss := SecretStore(&SecretStoreConfig{
		Name:      "test",
		Namespace: "default",
	})

	AddSecretStoreLabel(ss, "env", "prod")

	if ss.Labels["env"] != "prod" {
		t.Errorf("expected label 'env'='prod', got %s", ss.Labels["env"])
	}
}

func TestAddSecretStoreAnnotation(t *testing.T) {
	ss := SecretStore(&SecretStoreConfig{
		Name:      "test",
		Namespace: "default",
	})

	AddSecretStoreAnnotation(ss, "desc", "test store")

	if ss.Annotations["desc"] != "test store" {
		t.Errorf("expected annotation 'desc'='test store', got %s", ss.Annotations["desc"])
	}
}

func TestSetSecretStoreProvider(t *testing.T) {
	ss := SecretStore(&SecretStoreConfig{
		Name:      "test",
		Namespace: "default",
	})

	provider := &esv1.SecretStoreProvider{
		AWS: &esv1.AWSProvider{
			Region: "ap-southeast-1",
		},
	}

	SetSecretStoreProvider(ss, provider)

	if ss.Spec.Provider == nil || ss.Spec.Provider.AWS == nil {
		t.Fatal("expected non-nil AWS provider")
	}
	if ss.Spec.Provider.AWS.Region != "ap-southeast-1" {
		t.Errorf("expected Region 'ap-southeast-1', got %s", ss.Spec.Provider.AWS.Region)
	}
}

func TestAddClusterSecretStoreLabel(t *testing.T) {
	css := ClusterSecretStore(&ClusterSecretStoreConfig{
		Name: "test",
	})

	AddClusterSecretStoreLabel(css, "team", "platform")

	if css.Labels["team"] != "platform" {
		t.Errorf("expected label 'team'='platform', got %s", css.Labels["team"])
	}
}

func TestAddClusterSecretStoreAnnotation(t *testing.T) {
	css := ClusterSecretStore(&ClusterSecretStoreConfig{
		Name: "test",
	})

	AddClusterSecretStoreAnnotation(css, "owner", "ops")

	if css.Annotations["owner"] != "ops" {
		t.Errorf("expected annotation 'owner'='ops', got %s", css.Annotations["owner"])
	}
}

func TestSetClusterSecretStoreProvider(t *testing.T) {
	css := ClusterSecretStore(&ClusterSecretStoreConfig{
		Name: "test",
	})

	provider := &esv1.SecretStoreProvider{
		AWS: &esv1.AWSProvider{
			Region: "us-west-2",
		},
	}

	SetClusterSecretStoreProvider(css, provider)

	if css.Spec.Provider == nil || css.Spec.Provider.AWS == nil {
		t.Fatal("expected non-nil AWS provider")
	}
	if css.Spec.Provider.AWS.Region != "us-west-2" {
		t.Errorf("expected Region 'us-west-2', got %s", css.Spec.Provider.AWS.Region)
	}
}

func TestSetRefreshInterval(t *testing.T) {
	es := ExternalSecret(&ExternalSecretConfig{
		Name:      "test",
		Namespace: "default",
	})

	d := metav1.Duration{Duration: 5 * time.Minute}
	SetRefreshInterval(es, d)

	if es.Spec.RefreshInterval == nil {
		t.Fatal("expected non-nil RefreshInterval")
	}
	if es.Spec.RefreshInterval.Duration != 5*time.Minute {
		t.Errorf("expected 5m, got %s", es.Spec.RefreshInterval.Duration)
	}
}

func TestAddDataFrom(t *testing.T) {
	es := ExternalSecret(&ExternalSecretConfig{
		Name:      "test",
		Namespace: "default",
	})

	source := esv1.ExternalSecretDataFromRemoteRef{
		Extract: &esv1.ExternalSecretDataRemoteRef{
			Key: "secret/all",
		},
	}
	AddDataFrom(es, source)

	if len(es.Spec.DataFrom) != 1 {
		t.Fatalf("expected 1 DataFrom entry, got %d", len(es.Spec.DataFrom))
	}
	if es.Spec.DataFrom[0].Extract == nil || es.Spec.DataFrom[0].Extract.Key != "secret/all" {
		t.Errorf("unexpected DataFrom[0]: %+v", es.Spec.DataFrom[0])
	}

	// Verify append behaviour: second call adds another entry.
	source2 := esv1.ExternalSecretDataFromRemoteRef{
		Extract: &esv1.ExternalSecretDataRemoteRef{
			Key: "secret/other",
		},
	}
	AddDataFrom(es, source2)
	if len(es.Spec.DataFrom) != 2 {
		t.Fatalf("expected 2 DataFrom entries after second append, got %d", len(es.Spec.DataFrom))
	}
}
