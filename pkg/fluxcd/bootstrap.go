package fluxcd

import (
	"time"

	sourcev1beta2 "github.com/fluxcd/source-controller/api/v1beta2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/go-kure/kure/pkg/kio"
)

// NewOCIRepositoryYAML constructs an OCIRepository resource from config.
func NewOCIRepositoryYAML(cfg *OCIRepositoryConfig) *sourcev1beta2.OCIRepository {
	return &sourcev1beta2.OCIRepository{
		TypeMeta: metav1.TypeMeta{
			Kind:       "OCIRepository",
			APIVersion: "source.toolkit.fluxcd.io/v1beta2",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      cfg.Name,
			Namespace: cfg.Namespace,
		},
		Spec: sourcev1beta2.OCIRepositorySpec{
			URL:       cfg.URL,
			Reference: &sourcev1beta2.OCIRepositoryRef{Tag: cfg.Ref},
			Interval:  metav1.Duration{Duration: parseDurationOrDefault(cfg.Interval)},
		},
	}
}

// WriteYAMLResource marshals the object to YAML at the given path.
func WriteYAMLResource(path string, obj client.Object) error {
	return kio.SaveFile(path, obj)
}

// PatchOCIRepositoryFromFile loads an OCIRepository from path and applies a patch function.
func PatchOCIRepositoryFromFile(path string, patchFn func(*sourcev1beta2.OCIRepository) error) error {
	var repo sourcev1beta2.OCIRepository
	if err := kio.LoadFile(path, &repo); err != nil {
		return err
	}

	if err := patchFn(&repo); err != nil {
		return err
	}

	return kio.SaveFile(path, repo)
}

func parseDurationOrDefault(s string) time.Duration {
	d, err := time.ParseDuration(s)
	if err != nil {
		return 5 * time.Minute
	}
	return d
}
