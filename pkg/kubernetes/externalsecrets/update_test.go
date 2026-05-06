package externalsecrets

import (
	"testing"
	"time"

	esv1 "github.com/external-secrets/external-secrets/apis/externalsecrets/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestSetExternalSecretSpec(t *testing.T) {
	es := ExternalSecret(&ExternalSecretConfig{
		Name:      "test",
		Namespace: "default",
	})

	newSpec := esv1.ExternalSecretSpec{
		SecretStoreRef: esv1.SecretStoreRef{
			Name: "new-store",
			Kind: "SecretStore",
		},
	}

	SetExternalSecretSpec(es, newSpec)

	if es.Spec.SecretStoreRef.Name != "new-store" {
		t.Errorf("expected SecretStoreRef.Name 'new-store', got %s", es.Spec.SecretStoreRef.Name)
	}
}

func TestSetSecretStoreSpec(t *testing.T) {
	ss := SecretStore(&SecretStoreConfig{
		Name:      "test",
		Namespace: "default",
	})

	newSpec := esv1.SecretStoreSpec{
		Controller: "replaced",
	}

	SetSecretStoreSpec(ss, newSpec)

	if ss.Spec.Controller != "replaced" {
		t.Errorf("expected Controller 'replaced', got %s", ss.Spec.Controller)
	}
}

func TestSetClusterSecretStoreSpec(t *testing.T) {
	css := ClusterSecretStore(&ClusterSecretStoreConfig{
		Name: "test",
	})

	newSpec := esv1.SecretStoreSpec{
		Controller: "cluster-replaced",
	}

	SetClusterSecretStoreSpec(css, newSpec)

	if css.Spec.Controller != "cluster-replaced" {
		t.Errorf("expected Controller 'cluster-replaced', got %s", css.Spec.Controller)
	}
}

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

func TestSetExternalSecretSecretStoreRef(t *testing.T) {
	es := ExternalSecret(&ExternalSecretConfig{
		Name:      "test",
		Namespace: "default",
	})

	ref := esv1.SecretStoreRef{
		Name: "updated-store",
		Kind: "ClusterSecretStore",
	}

	SetExternalSecretSecretStoreRef(es, ref)

	if es.Spec.SecretStoreRef.Name != "updated-store" {
		t.Errorf("expected SecretStoreRef.Name 'updated-store', got %s", es.Spec.SecretStoreRef.Name)
	}
	if es.Spec.SecretStoreRef.Kind != "ClusterSecretStore" {
		t.Errorf("expected SecretStoreRef.Kind 'ClusterSecretStore', got %s", es.Spec.SecretStoreRef.Kind)
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

func TestSetSecretStoreController(t *testing.T) {
	ss := SecretStore(&SecretStoreConfig{
		Name:      "test",
		Namespace: "default",
	})

	SetSecretStoreController(ss, "new-controller")

	if ss.Spec.Controller != "new-controller" {
		t.Errorf("expected Controller 'new-controller', got %s", ss.Spec.Controller)
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

func TestSetClusterSecretStoreController(t *testing.T) {
	css := ClusterSecretStore(&ClusterSecretStoreConfig{
		Name: "test",
	})

	SetClusterSecretStoreController(css, "global")

	if css.Spec.Controller != "global" {
		t.Errorf("expected Controller 'global', got %s", css.Spec.Controller)
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

func TestSetTarget(t *testing.T) {
	es := ExternalSecret(&ExternalSecretConfig{
		Name:      "test",
		Namespace: "default",
	})

	target := esv1.ExternalSecretTarget{
		Name:           "my-secret",
		CreationPolicy: esv1.CreatePolicyOwner,
	}
	SetTarget(es, target)

	if es.Spec.Target.Name != "my-secret" {
		t.Errorf("expected Target.Name 'my-secret', got %s", es.Spec.Target.Name)
	}
	if es.Spec.Target.CreationPolicy != esv1.CreatePolicyOwner {
		t.Errorf("expected CreationPolicy 'Owner', got %s", es.Spec.Target.CreationPolicy)
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
