package bootstrap

import (
	sourcev1beta2 "github.com/fluxcd/source-controller/api/v1beta2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/go-kure/kure/pkg/api"
)

func NewOCIRepositoryYAML(cfg *api.OCIRepositoryConfig) *sourcev1beta2.OCIRepository {
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
