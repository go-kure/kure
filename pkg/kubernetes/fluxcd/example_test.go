package fluxcd_test

import (
	"fmt"
	"time"

	helmv2 "github.com/fluxcd/helm-controller/api/v2"
	kustv1 "github.com/fluxcd/kustomize-controller/api/v1"
	"github.com/fluxcd/pkg/apis/kustomize"
	"github.com/fluxcd/pkg/apis/meta"
	sourcev1 "github.com/fluxcd/source-controller/api/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/go-kure/kure/pkg/kubernetes/fluxcd"
)

// The README's Go blocks are generated from these functions
// (scripts/gen-doc-examples.sh); each prints what it built so go test checks
// the example as well as compiling it.

func ExampleCreateGitRepository() {
	obj := fluxcd.CreateGitRepository("my-repo", "flux-system")
	fmt.Println(obj.Kind, obj.Namespace+"/"+obj.Name)
	// Output: GitRepository flux-system/my-repo
}

func ExampleSetGitRepositoryReference() {
	gr := fluxcd.CreateGitRepository("my-repo", "flux-system")
	gr.Spec.URL = "https://github.com/org/repo"
	fluxcd.SetGitRepositoryReference(gr, &sourcev1.GitRepositoryRef{Branch: "main"})
	gr.Spec.Interval = metav1.Duration{Duration: 5 * time.Minute}
	fluxcd.SetGitRepositorySecretRef(gr, &meta.LocalObjectReference{Name: "git-credentials"})
	fmt.Println(gr.Spec.Reference.Branch, gr.Spec.Interval.Duration, gr.Spec.SecretRef.Name)
	// Output: main 5m0s git-credentials
}

func ExampleSetOCIRepositoryReference() {
	oci := fluxcd.CreateOCIRepository("my-manifests", "flux-system")
	oci.Spec.URL = "oci://registry.example.com/manifests"
	fluxcd.SetOCIRepositoryReference(oci, &sourcev1.OCIRepositoryRef{Tag: "latest"})
	oci.Spec.Interval = metav1.Duration{Duration: 10 * time.Minute}
	fluxcd.SetOCIRepositorySecretRef(oci, &meta.LocalObjectReference{Name: "registry-credentials"})
	fmt.Println(oci.Spec.Reference.Tag, oci.Spec.SecretRef.Name)
	// Output: latest registry-credentials
}

func ExampleSetHelmRepositorySecretRef() {
	hr := fluxcd.CreateHelmRepository("bitnami", "flux-system")
	hr.Spec.URL = "https://charts.bitnami.com/bitnami"
	hr.Spec.Type = "default"
	hr.Spec.Interval = metav1.Duration{Duration: 10 * time.Minute}
	fluxcd.SetHelmRepositoryTimeout(hr, &metav1.Duration{Duration: 60 * time.Second})
	hr.Spec.PassCredentials = true
	fluxcd.SetHelmRepositorySecretRef(hr, &meta.LocalObjectReference{Name: "bitnami-auth"})
	fmt.Println(hr.Spec.Timeout.Duration, hr.Spec.SecretRef.Name)
	// Output: 1m0s bitnami-auth
}

func ExampleCreateHelmRepository_oci() {
	hr := fluxcd.CreateHelmRepository("ghcr-charts", "flux-system")
	hr.Spec.URL = "oci://ghcr.io/example/charts"
	hr.Spec.Type = "oci"
	hr.Spec.Provider = "generic" // OCI-only: generic, aws, azure, gcp
	hr.Spec.Interval = metav1.Duration{Duration: 5 * time.Minute}
	fluxcd.SetHelmRepositorySecretRef(hr, &meta.LocalObjectReference{Name: "ghcr-auth"})
	fmt.Println(hr.Spec.Type, hr.Spec.Provider)
	// Output: oci generic
}

func ExampleCreateHelmChart() {
	hc := fluxcd.CreateHelmChart("redis", "flux-system")
	hc.Spec.Chart = "redis"
	hc.Spec.Version = "19.0.0"
	hc.Spec.SourceRef = sourcev1.LocalHelmChartSourceReference{
		Kind: "HelmRepository",
		Name: "bitnami",
	}
	hc.Spec.Interval = metav1.Duration{Duration: 10 * time.Minute}
	fmt.Println(hc.Spec.Chart, hc.Spec.Version, hc.Spec.SourceRef.Kind)
	// Output: redis 19.0.0 HelmRepository
}

func ExampleSetBucketSecretRef() {
	b := fluxcd.CreateBucket("my-bucket", "flux-system")
	b.Spec.Endpoint = "minio.example.com"
	b.Spec.BucketName = "manifests"
	b.Spec.Interval = metav1.Duration{Duration: 10 * time.Minute}
	fluxcd.SetBucketSecretRef(b, &meta.LocalObjectReference{Name: "minio-credentials"})
	fmt.Println(b.Spec.BucketName, b.Spec.SecretRef.Name)
	// Output: manifests minio-credentials
}

func ExampleAddKustomizationDependsOn() {
	k := fluxcd.CreateKustomization("my-app", "flux-system")
	k.Spec.SourceRef = kustv1.CrossNamespaceSourceReference{
		Kind: "GitRepository",
		Name: "my-repo",
	}
	k.Spec.Path = "./clusters/production/apps"
	k.Spec.Interval = metav1.Duration{Duration: 10 * time.Minute}
	k.Spec.Prune = true
	k.Spec.TargetNamespace = "production"
	k.Spec.Wait = true
	fluxcd.AddKustomizationDependsOn(k, kustv1.DependencyReference{Name: "cert-manager"})
	fmt.Println(k.Spec.Path, k.Spec.DependsOn[0].Name)
	// Output: ./clusters/production/apps cert-manager
}

func ExampleSetHelmReleaseValuesFromMap() {
	hr := fluxcd.CreateHelmRelease("redis", "apps")
	hr.Spec.ReleaseName = "redis-prod"
	hr.Spec.TargetNamespace = "apps"
	hr.Spec.Interval = metav1.Duration{Duration: 10 * time.Minute}
	fluxcd.SetHelmReleaseChart(hr, &helmv2.HelmChartTemplate{
		Spec: helmv2.HelmChartTemplateSpec{
			Chart:   "redis",
			Version: "19.0.0",
			SourceRef: helmv2.CrossNamespaceObjectReference{
				Kind:      "HelmRepository",
				Name:      "bitnami",
				Namespace: "flux-system",
			},
		},
	})
	// Panics if the map does not marshal (a channel, a function, a NaN).
	fluxcd.SetHelmReleaseValuesFromMap(hr, map[string]any{"replicaCount": 3})
	// Alternative — pre-marshalled JSON:
	// fluxcd.SetHelmReleaseValues(hr, &apiextensionsv1.JSON{Raw: []byte(`{"replicaCount":3}`)})
	fluxcd.AddHelmReleaseValuesFrom(hr, helmv2.ValuesReference{
		Kind: "ConfigMap",
		Name: "redis-defaults",
	})
	fmt.Println(hr.Spec.Chart.Spec.Chart, string(hr.Spec.Values.Raw), hr.Spec.ValuesFrom[0].Name)
	// Output: redis {"replicaCount":3} redis-defaults
}

func ExampleSetHelmReleaseChartRef() {
	hr := fluxcd.CreateHelmRelease("my-app", "apps")
	fluxcd.SetHelmReleaseChartRef(hr, &helmv2.CrossNamespaceSourceReference{
		Kind:      "OCIRepository",
		Name:      "my-oci-source",
		Namespace: "flux-system",
	})
	fmt.Println(hr.Spec.ChartRef.Kind, hr.Spec.ChartRef.Name)
	// Output: OCIRepository my-oci-source
}

func ExampleSetHelmReleaseDriftDetection() {
	hr := fluxcd.CreateHelmRelease("my-app", "apps")

	fluxcd.SetHelmReleaseDriftDetection(hr, fluxcd.CreateDriftDetection(helmv2.DriftDetectionEnabled))
	fluxcd.SetHelmReleaseInstallCRDs(hr, helmv2.CreateReplace)
	fluxcd.SetHelmReleaseInstallRemediation(hr, fluxcd.CreateInstallRemediation(3))
	fluxcd.SetHelmReleaseUpgradeCRDs(hr, helmv2.CreateReplace)
	fluxcd.SetHelmReleaseUpgradeRemediation(hr, fluxcd.CreateUpgradeRemediation(3))
	fmt.Println(hr.Spec.DriftDetection.Mode, hr.Spec.Install.CRDs, hr.Spec.Upgrade.Remediation.Retries)
	// Output: enabled CreateReplace 3
}

func ExampleAddHelmReleasePostRenderer() {
	hr := fluxcd.CreateHelmRelease("my-app", "apps")

	k := fluxcd.CreatePostRendererKustomize()
	fluxcd.AddPostRendererKustomizeImage(k, kustomize.Image{Name: "redis", NewTag: "7.0"})
	fluxcd.AddHelmReleasePostRenderer(hr, helmv2.PostRenderer{Kustomize: k})
	fmt.Println(hr.Spec.PostRenderers[0].Kustomize.Images[0].NewTag)
	// Output: 7.0
}

func ExampleSetProviderSecretRef() {
	provider := fluxcd.CreateProvider("slack", "flux-system")
	provider.Spec.Type = "slack" // plain fields: assigned, not set
	provider.Spec.Channel = "#alerts"
	fluxcd.SetProviderSecretRef(provider, &meta.LocalObjectReference{Name: "slack-webhook"})

	alert := fluxcd.CreateAlert("slack-alert", "flux-system")
	alert.Spec.ProviderRef = meta.LocalObjectReference{Name: "slack"}
	alert.Spec.EventSeverity = "error" // no Alert setters remain: all were bare assignments

	receiver := fluxcd.CreateReceiver("github-receiver", "flux-system")
	receiver.Spec.Type = "github"
	receiver.Spec.Events = []string{"push"}
	fluxcd.SetReceiverSecretRef(receiver, meta.LocalObjectReference{Name: "webhook-token"})
	fmt.Println(provider.Spec.SecretRef.Name, alert.Spec.EventSeverity, receiver.Spec.SecretRef.Name)
	// Output: slack-webhook error webhook-token
}

func ExampleCreateFluxInstance() {
	instance := fluxcd.CreateFluxInstance("flux", "flux-system")
	instance.Spec.Distribution.Variant = "upstream-alpine" // Distribution is a plain field
	// Pointer setters remain for the optional blocks: SetFluxInstanceCluster,
	// SetFluxInstanceSharding, SetFluxInstanceStorage, SetFluxInstanceKustomize,
	// SetFluxInstanceSync, SetFluxInstanceWait, SetFluxInstanceCommonMetadata and
	// SetFluxInstanceMigrateResources.
	fmt.Println(instance.Spec.Distribution.Variant)
	// Output: upstream-alpine
}

func ExampleSetExternalArtifactSourceRef() {
	ea := fluxcd.CreateExternalArtifact("my-artifact", "flux-system")
	fluxcd.SetExternalArtifactSourceRef(ea, &meta.NamespacedObjectKindReference{
		APIVersion: "source.toolkit.fluxcd.io/v1",
		Kind:       "OCIRepository",
		Name:       "my-oci-source",
		Namespace:  "flux-system",
	})
	fmt.Println(ea.Spec.SourceRef.Kind, ea.Spec.SourceRef.Name)
	// Output: OCIRepository my-oci-source
}

func ExampleAddArtifactGeneratorSource() {
	ag := fluxcd.CreateArtifactGenerator("my-gen", "flux-system")

	src := fluxcd.CreateSourceReference("app", "my-oci-source", "OCIRepository")
	src.Namespace = "flux-system"
	fluxcd.AddArtifactGeneratorSource(ag, src)

	out := fluxcd.CreateOutputArtifact("combined")
	out.Revision = "@app"
	cp := fluxcd.CreateCopyOperation("@app/manifests/**", "@artifact/manifests")
	fluxcd.AddOutputArtifactCopyOperation(&out, cp)
	fluxcd.AddArtifactGeneratorOutputArtifact(ag, out)
	fmt.Println(ag.Spec.Sources[0].Alias, ag.Spec.OutputArtifacts[0].Name, ag.Spec.OutputArtifacts[0].Copy[0].From)
	// Output: app combined @app/manifests/**
}
