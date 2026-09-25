package externalsecrets_test

import (
	"fmt"
	"time"

	esv1 "github.com/external-secrets/external-secrets/apis/externalsecrets/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/go-kure/kure/pkg/kubernetes"
	"github.com/go-kure/kure/pkg/kubernetes/externalsecrets"
)

// The README's Go blocks are generated from these functions
// (scripts/gen-doc-examples.sh); each prints what it built so go test checks
// the example as well as compiling it.

func ExampleCreateExternalSecret() {
	obj := externalsecrets.CreateExternalSecret("my-secret", "default")
	cl := externalsecrets.CreateClusterSecretStore("global-vault")
	fmt.Println(obj.Kind, obj.Namespace+"/"+obj.Name, cl.Kind, cl.Name)
	// Output: ExternalSecret default/my-secret ClusterSecretStore global-vault
}

func ExampleAddExternalSecretData() {
	es := externalsecrets.CreateExternalSecret("my-secret", "default")
	es.Spec.SecretStoreRef = esv1.SecretStoreRef{Name: "vault", Kind: "ClusterSecretStore"}
	es.Spec.Target = esv1.ExternalSecretTarget{Name: "my-secret"}
	externalsecrets.AddExternalSecretData(es, esv1.ExternalSecretData{
		SecretKey: "password",
		RemoteRef: esv1.ExternalSecretDataRemoteRef{Key: "secret/data/myapp"},
	})
	externalsecrets.SetRefreshInterval(es, metav1.Duration{Duration: time.Hour})
	fmt.Println(es.Spec.Data[0].SecretKey, es.Spec.RefreshInterval.Duration)
	// Output: password 1h0m0s
}

func ExampleSetSecretStoreProvider() {
	ss := externalsecrets.CreateSecretStore("aws-store", "default")
	externalsecrets.SetSecretStoreProvider(ss, &esv1.SecretStoreProvider{
		AWS: &esv1.AWSProvider{Service: esv1.AWSServiceSecretsManager, Region: "us-east-1"},
	})
	ss.Spec.Controller = "my-controller"
	fmt.Println(ss.Spec.Provider.AWS.Region, ss.Spec.Controller)
	// Output: us-east-1 my-controller
}

func ExampleSetClusterSecretStoreProvider() {
	css := externalsecrets.CreateClusterSecretStore("global-vault")
	externalsecrets.SetClusterSecretStoreProvider(css, &esv1.SecretStoreProvider{
		AWS: &esv1.AWSProvider{Service: esv1.AWSServiceSecretsManager, Region: "us-east-1"},
	})
	fmt.Println(css.Spec.Provider.AWS.Region)
	// Output: us-east-1
}

func ExampleAddDataFrom() {
	es := externalsecrets.CreateExternalSecret("my-secret", "default")
	ss := externalsecrets.CreateSecretStore("aws-store", "default")
	css := externalsecrets.CreateClusterSecretStore("global-vault")
	provider := &esv1.SecretStoreProvider{
		AWS: &esv1.AWSProvider{Service: esv1.AWSServiceSecretsManager, Region: "us-east-1"},
	}

	// Replace the full spec
	es.Spec = esv1.ExternalSecretSpec{Target: esv1.ExternalSecretTarget{Name: "my-secret"}}

	// Granular updates
	externalsecrets.AddExternalSecretData(es, esv1.ExternalSecretData{
		SecretKey: "password",
		RemoteRef: esv1.ExternalSecretDataRemoteRef{Key: "secret/data/myapp"},
	})
	externalsecrets.AddDataFrom(es, esv1.ExternalSecretDataFromRemoteRef{
		Extract: &esv1.ExternalSecretDataRemoteRef{Key: "secret/data/shared"},
	})
	es.Spec.SecretStoreRef = esv1.SecretStoreRef{Name: "vault", Kind: "ClusterSecretStore"}

	externalsecrets.SetSecretStoreProvider(ss, provider)
	ss.Spec.Controller = "my-controller"

	externalsecrets.SetClusterSecretStoreProvider(css, provider)
	css.Spec.Controller = "global"

	// Labels and annotations use the generic helpers, which work over any object
	// with ObjectMeta -- this package carries no per-kind metadata helpers
	kubernetes.AddLabel(es, "app", "myapp")
	kubernetes.AddAnnotation(ss, "desc", "value")
	kubernetes.AddLabel(css, "team", "platform")

	fmt.Println(len(es.Spec.Data), es.Spec.DataFrom[0].Extract.Key, css.Spec.Controller, es.Labels["app"])
	// Output: 1 secret/data/shared global myapp
}
