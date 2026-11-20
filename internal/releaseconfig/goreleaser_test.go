package releaseconfig

import (
	"fmt"
	"os"
	"testing"

	"gopkg.in/yaml.v3"
)

// changelogStepKept reports whether the GoReleaser configuration in data leaves the
// changelog step enabled. The key may be absent or an explicit boolean false; any
// other value is refused, because GoReleaser also takes a template string for
// disable, which can evaluate to true.
func changelogStepKept(data []byte) error {
	var cfg struct {
		Changelog struct {
			Disable any `yaml:"disable"`
		} `yaml:"changelog"`
	}
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return fmt.Errorf("parse: %w", err)
	}
	switch v := cfg.Changelog.Disable.(type) {
	case nil:
		return nil
	case bool:
		if !v {
			return nil
		}
	}
	return fmt.Errorf("changelog.disable is set to %v", cfg.Changelog.Disable)
}

// TestChangelogStepKept_Cases: absent and an explicit false keep the step; true, a quoted
// "true" and a template string do not.
func TestChangelogStepKept_Cases(t *testing.T) {
	for name, tc := range map[string]struct {
		yaml string
		kept bool
	}{
		"no changelog key":  {"version: 2\n", true},
		"no disable key":    {"changelog:\n  sort: asc\n", true},
		"explicit false":    {"changelog:\n  disable: false\n", true},
		"explicit true":     {"changelog:\n  disable: true\n", false},
		"quoted true":       {"changelog:\n  disable: \"true\"\n", false},
		"a template string": {"changelog:\n  disable: \"{{ .IsSnapshot }}\"\n", false},
	} {
		t.Run(name, func(t *testing.T) {
			if got := changelogStepKept([]byte(tc.yaml)) == nil; got != tc.kept {
				t.Errorf("changelogStepKept = %v, want %v", got, tc.kept)
			}
		})
	}
}

// TestGoreleaserConfig_KeepsTheChangelogStepForReleaseNotes: the shared release
// workflow hands GoReleaser the release notes with --release-notes, and GoReleaser
// loads that file only inside its changelog step. With `changelog.disable: true`
// every release step succeeds and the GitHub release body is one newline. With
// --release-notes set, GoReleaser makes no changelog of its own, so the step costs
// nothing and must stay enabled.
func TestGoreleaserConfig_KeepsTheChangelogStepForReleaseNotes(t *testing.T) {
	data, err := os.ReadFile("../../.goreleaser.yml")
	if err != nil {
		t.Fatalf("read .goreleaser.yml: %v", err)
	}
	if err := changelogStepKept(data); err != nil {
		t.Errorf("%v: GoReleaser would skip the step that loads --release-notes, and the release body would be empty", err)
	}
}
