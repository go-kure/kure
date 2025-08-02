package layout

import "fmt"

// Profile identifies a supported layout profile.
type Profile string

const (
	// FluxProfile represents defaults matching FluxCD conventions.
	FluxProfile Profile = "flux"
	// ArgoProfile represents defaults matching Argo CD conventions.
	ArgoProfile Profile = "argocd"
)

// DefaultConfigForProfile returns a Config initialised with defaults for the
// given profile. Unknown profiles fall back to FluxProfile.
func DefaultConfigForProfile(p Profile) Config {
	switch p {
	case ArgoProfile:
		return Config{
			ManifestsDir:        "applications",
			FluxDir:             "applications",
			FilePer:             FilePerResource,
			ApplicationFileMode: AppFileSingle,
			ManifestFileName:    DefaultManifestFileName,
			KustomizationFileName: func(name string) string {
				return fmt.Sprintf("application-%s.yaml", name)
			},
		}
	default:
		return DefaultLayoutConfig()
	}
}
