package fluxcd

// OCIRepositoryConfig describes an OCIRepository resource used by Flux.
type OCIRepositoryConfig struct {
	Name      string `yaml:"name"`
	Namespace string `yaml:"namespace"`
	URL       string `yaml:"url"`
	Ref       string `yaml:"ref"`
	Interval  string `yaml:"interval"`
}
