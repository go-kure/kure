package layout

import (
	"archive/tar"
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

// go-kure/kure#752: an ExtraFile that would take a path the writer owns in
// its layout directory, or leave that directory, is refused by all three
// writers instead of silently replacing a generated file.

func extraFileLayout(extra ...ExtraFile) *ManifestLayout {
	return &ManifestLayout{
		Name:       "app",
		Namespace:  "default",
		FilePer:    FilePerResource,
		Resources:  []client.Object{testObject("v1", "ConfigMap", "foo", "default")},
		ExtraFiles: extra,
	}
}

// extraFileWriters runs ml through each writer and returns its error.
var extraFileWriters = map[string]func(t *testing.T, ml *ManifestLayout) error{
	"WriteToDisk":   func(t *testing.T, ml *ManifestLayout) error { return ml.WriteToDisk(t.TempDir()) },
	"WriteManifest": func(t *testing.T, ml *ManifestLayout) error { return WriteManifest(t.TempDir(), Config{}, ml) },
	"WriteToTar": func(t *testing.T, ml *ManifestLayout) error {
		var buf bytes.Buffer
		return ml.WriteToTar(&buf)
	},
}

func TestExtraFiles_Refused(t *testing.T) {
	child := func(ml *ManifestLayout, c *ManifestLayout) *ManifestLayout {
		ml.Children = append(ml.Children, c)
		return ml
	}
	for _, tc := range []struct {
		desc    string
		ml      *ManifestLayout
		wantErr string
	}{
		{"generated resource file", extraFileLayout(ExtraFile{Name: "default-configmap-foo.yaml"}), "would replace the generated resource file"},
		{"generated file in other case", extraFileLayout(ExtraFile{Name: "Default-ConfigMap-Foo.yaml"}), "would replace the generated resource file"},
		{"kustomization.yaml", extraFileLayout(ExtraFile{Name: "kustomization.yaml"}), "kustomize control file"},
		{"Kustomization", extraFileLayout(ExtraFile{Name: "Kustomization"}), "kustomize control file"},
		{"parent directory", extraFileLayout(ExtraFile{Name: "../escape.yaml"}), "must be a relative path"},
		{"dot segment", extraFileLayout(ExtraFile{Name: "./default-configmap-foo.yaml"}), "must be a relative path"},
		{"absolute", extraFileLayout(ExtraFile{Name: "/etc/escape.yaml"}), "must be a relative path"},
		{"empty", extraFileLayout(ExtraFile{Name: ""}), "must be a relative path"},
		{"non-portable character", extraFileLayout(ExtraFile{Name: "café.yaml"}), "must be a relative path"},
		{"listed twice", extraFileLayout(ExtraFile{Name: "values.yaml"}, ExtraFile{Name: "Values.yaml"}), "listed twice"},
		{"extra under another extra", extraFileLayout(ExtraFile{Name: "assets"}, ExtraFile{Name: "assets/values.yaml"}), "would sit beneath the extra file"},
		{"child directory", child(extraFileLayout(ExtraFile{Name: "web/values.yaml"}), &ManifestLayout{
			Name: "web", Namespace: "default/app",
			Resources: []client.Object{testObject("v1", "ConfigMap", "bar", "default")},
		}), "would take the directory of child layout"},
		{"generated file as directory", extraFileLayout(ExtraFile{Name: "default-configmap-foo.yaml/values.yaml"}), "would use the generated resource file"},
		{"control file as directory", extraFileLayout(ExtraFile{Name: "kustomization.yaml/values.yaml"}), "would use a kustomize control file"},
		{"umbrella child directory", child(extraFileLayout(ExtraFile{Name: "web"}), &ManifestLayout{
			Name: "web", Namespace: "default/app", UmbrellaChild: true,
			Resources: []client.Object{testObject("v1", "ConfigMap", "bar", "default")},
		}), "would take the directory of child layout"},
		{"AppFileSingle child file", child(extraFileLayout(ExtraFile{Name: "single.yaml"}), &ManifestLayout{
			Name: "single", Namespace: "default/app", ApplicationFileMode: AppFileSingle,
			Resources: []client.Object{testObject("v1", "ConfigMap", "bar", "default")},
		}), "would replace the child file"},
	} {
		for wname, write := range extraFileWriters {
			t.Run(tc.desc+"/"+wname, func(t *testing.T) {
				err := write(t, tc.ml)
				if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
					t.Fatalf("expected an error containing %q, got %v", tc.wantErr, err)
				}
			})
		}
	}
}

func TestExtraFiles_NothingWrittenForARefusedLayout(t *testing.T) {
	// The check runs before the layout's first file is written, so the
	// generated manifest is not written and then left for the extra to replace.
	dir := t.TempDir()
	if err := extraFileLayout(ExtraFile{Name: "default-configmap-foo.yaml"}).WriteToDisk(dir); err == nil {
		t.Fatal("expected an error")
	}
	if _, err := os.Stat(filepath.Join(dir, "default", "app", "default-configmap-foo.yaml")); err == nil {
		t.Error("the generated file was written before the refusal")
	}
}

func TestExtraFiles_AllowedNamesAreWritten(t *testing.T) {
	ml := extraFileLayout(
		ExtraFile{Name: "values.yaml", Content: []byte("a: 1\n")},
		ExtraFile{Name: "assets/dashboard.json", Content: []byte("{}\n")},
	)
	t.Run("WriteToDisk", func(t *testing.T) {
		dir := t.TempDir()
		if err := ml.WriteToDisk(dir); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		for name, want := range map[string]string{"values.yaml": "a: 1\n", "assets/dashboard.json": "{}\n"} {
			got, err := os.ReadFile(filepath.Join(dir, "default", "app", filepath.FromSlash(name)))
			if err != nil || string(got) != want {
				t.Errorf("%s = %q, %v; want %q", name, got, err, want)
			}
		}
	})
	t.Run("WriteManifest", func(t *testing.T) {
		if err := WriteManifest(t.TempDir(), Config{}, ml); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})
	t.Run("WriteToTar", func(t *testing.T) {
		var buf bytes.Buffer
		if err := ml.WriteToTar(&buf); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		names := map[string]bool{}
		tr := tar.NewReader(&buf)
		for {
			hdr, err := tr.Next()
			if err == io.EOF {
				break
			}
			if err != nil {
				t.Fatal(err)
			}
			names[hdr.Name] = true
		}
		for _, want := range []string{"default/app/values.yaml", "default/app/assets/dashboard.json"} {
			if !names[want] {
				t.Errorf("tar lacks %s; has %v", want, names)
			}
		}
	})
}

func TestExtraFiles_ChildNamespacePath(t *testing.T) {
	// A child's directory is its own output path relative to the parent's,
	// not always <parent>/<child name>: here root "." holds a child written
	// to platform/cert-manager.
	tree := func(extra string) *ManifestLayout {
		return &ManifestLayout{
			Namespace:  ".",
			FilePer:    FilePerResource,
			ExtraFiles: []ExtraFile{{Name: extra, Content: []byte("x: 1\n")}},
			Children: []*ManifestLayout{{
				Name:      "cert-manager",
				Namespace: "platform",
				FilePer:   FilePerResource,
				Resources: []client.Object{testObject("v1", "ConfigMap", "cert-manager-config", "cert-manager")},
			}},
		}
	}
	for _, extra := range []string{"platform/cert-manager/configmap.yaml", "platform/cert-manager", "platform"} {
		for wname, write := range extraFileWriters {
			t.Run(extra+"/"+wname, func(t *testing.T) {
				err := write(t, tree(extra))
				if err == nil || !strings.Contains(err.Error(), "would take the directory of child layout") {
					t.Fatalf("expected a child-directory refusal, got %v", err)
				}
			})
		}
	}
	// A sibling path is free.
	for wname, write := range extraFileWriters {
		t.Run("platform-values.yaml/"+wname, func(t *testing.T) {
			if err := write(t, tree("platform-values.yaml")); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestWriteManifest_ConfiguredParentSingleMode(t *testing.T) {
	// With Config.ApplicationFileMode AppFileSingle an unset parent writes
	// into its namespace directory "default", so the child's directory there
	// is "web", not a path under default/app.
	parent := extraFileLayout(ExtraFile{Name: "web/default-configmap-bar.yaml"})
	parent.Children = []*ManifestLayout{{
		Name: "web", Namespace: "default", ApplicationFileMode: AppFilePerResource, FilePer: FilePerResource,
		Resources: []client.Object{testObject("v1", "ConfigMap", "bar", "default")},
	}}
	err := WriteManifest(t.TempDir(), Config{ApplicationFileMode: AppFileSingle}, parent)
	if err == nil || !strings.Contains(err.Error(), "would take the directory of child layout") {
		t.Fatalf("expected a child-directory refusal, got %v", err)
	}
}

func TestExtraFiles_SingleChildDirectoryAncestor(t *testing.T) {
	// An AppFileSingle child written to default/app/nested needs "nested" to
	// be a directory; an extra file named "nested" is refused.
	for wname, write := range extraFileWriters {
		t.Run(wname, func(t *testing.T) {
			parent := extraFileLayout(ExtraFile{Name: "nested"})
			parent.Children = []*ManifestLayout{{
				Name: "single", Namespace: "default/app/nested", ApplicationFileMode: AppFileSingle,
				Resources: []client.Object{testObject("v1", "ConfigMap", "bar", "default")},
			}}
			err := write(t, parent)
			if err == nil || !strings.Contains(err.Error(), "would take a directory that") {
				t.Fatalf("expected a needed-directory refusal, got %v", err)
			}
		})
	}
}

func TestExtraFiles_ChildNamespaceCaseMismatch(t *testing.T) {
	// "Default/App/web" and "default/app/web" are one directory on a
	// case-insensitive volume, so the child is inside the parent's directory.
	for wname, write := range extraFileWriters {
		t.Run(wname, func(t *testing.T) {
			parent := extraFileLayout(ExtraFile{Name: "web/default-configmap-bar.yaml"})
			parent.Children = []*ManifestLayout{{
				Name: "web", Namespace: "Default/App", FilePer: FilePerResource,
				Resources: []client.Object{testObject("v1", "ConfigMap", "bar", "default")},
			}}
			err := write(t, parent)
			if err == nil || !strings.Contains(err.Error(), "would take the directory of child layout") {
				t.Fatalf("expected a child-directory refusal, got %v", err)
			}
		})
	}
}

func TestWriteManifest_RootedFilenameWithExtraFiles(t *testing.T) {
	// A custom file name starting with "/" must not stall the check: the
	// directory walk stops at "/" as well as ".".
	cfg := Config{ManifestFileName: func(_, _, _ string, _ FileExportMode) string { return "/configmap-foo.yaml" }}
	done := make(chan error, 1)
	go func() { done <- WriteManifest(t.TempDir(), cfg, extraFileLayout(ExtraFile{Name: "values.yaml"})) }()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("WriteManifest did not return")
	}
}

func TestWriteManifest_RootedFilenameCollision(t *testing.T) {
	// The writer joins "/configmap-foo.yaml" under the layout directory, so an
	// extra named "configmap-foo.yaml" is the same file and is refused.
	cfg := Config{ManifestFileName: func(_, _, _ string, _ FileExportMode) string { return "/configmap-foo.yaml" }}
	err := WriteManifest(t.TempDir(), cfg, extraFileLayout(ExtraFile{Name: "configmap-foo.yaml"}))
	if err == nil || !strings.Contains(err.Error(), "would replace the generated resource file") {
		t.Fatalf("expected a collision error, got %v", err)
	}
}

func TestExtraFiles_RootDirectoryCollision(t *testing.T) {
	// A layout written to "/" (the archive root, or the base on disk) still
	// has its generated names reserved.
	for wname, write := range extraFileWriters {
		t.Run(wname, func(t *testing.T) {
			ml := extraFileLayout(ExtraFile{Name: "default-configmap-foo.yaml", Content: []byte("REPLACED\n")})
			ml.Name, ml.Namespace = "", "/"
			err := write(t, ml)
			if err == nil || !strings.Contains(err.Error(), "would replace the generated resource file") {
				t.Fatalf("expected a collision error, got %v", err)
			}
		})
	}
}

// nestedBaseWriters runs ml through each writer with a disk base one level
// below a fresh temp dir, so a ".." that a writer did follow would still land
// inside the test's own temp tree.
var nestedBaseWriters = map[string]func(t *testing.T, ml *ManifestLayout) error{
	"WriteToDisk": func(t *testing.T, ml *ManifestLayout) error {
		return ml.WriteToDisk(filepath.Join(t.TempDir(), "out"))
	},
	"WriteManifest": func(t *testing.T, ml *ManifestLayout) error {
		return WriteManifest(filepath.Join(t.TempDir(), "out"), Config{}, ml)
	},
	"WriteToTar": func(t *testing.T, ml *ManifestLayout) error {
		var buf bytes.Buffer
		return ml.WriteToTar(&buf)
	},
}

func TestExtraFiles_RefusesInvalidLayoutIdentity(t *testing.T) {
	// A ".." segment in a layout's own Namespace or Name is refused when the
	// layout has extra files. The extra collides with nothing, so the refusal
	// comes from the identity alone.
	for _, tc := range []struct{ desc, namespace, name string }{
		{"namespace", "../escape", "app"},
		{"name", "default", "../escape"},
	} {
		for wname, write := range nestedBaseWriters {
			t.Run(tc.desc+"/"+wname, func(t *testing.T) {
				ml := extraFileLayout(ExtraFile{Name: "values.yaml", Content: []byte("x: 1\n")})
				ml.Namespace, ml.Name = tc.namespace, tc.name
				err := write(t, ml)
				if err == nil || !strings.Contains(err.Error(), "must not contain") {
					t.Fatalf("expected a traversal refusal, got %v", err)
				}
			})
		}
	}
}

func TestExtraFiles_RefusesChildIdentityTraversal(t *testing.T) {
	// A direct child's Namespace or Name with a ".." segment is refused by the
	// parent's check, before the parent writes anything.
	for _, tc := range []struct {
		desc  string
		child *ManifestLayout
	}{
		{"child namespace", &ManifestLayout{
			Name: "web", Namespace: "../clusters", FilePer: FilePerResource,
			Resources: []client.Object{testObject("v1", "ConfigMap", "bar", "default")},
		}},
		{"single-file child name", &ManifestLayout{
			Name: "../../app/single", Namespace: "default/app/nested", ApplicationFileMode: AppFileSingle,
			Resources: []client.Object{testObject("v1", "ConfigMap", "bar", "default")},
		}},
	} {
		for wname, write := range nestedBaseWriters {
			t.Run(tc.desc+"/"+wname, func(t *testing.T) {
				parent := extraFileLayout(ExtraFile{Name: "values.yaml", Content: []byte("x: 1\n")})
				parent.Children = []*ManifestLayout{tc.child}
				err := write(t, parent)
				if err == nil || !strings.Contains(err.Error(), "must not contain") {
					t.Fatalf("expected a traversal refusal, got %v", err)
				}
			})
		}
	}
}

func TestWriteManifest_RefusesTraversalInGeneratedFilenames(t *testing.T) {
	// A generated file name with a ".." segment is refused, including one
	// that would stay inside the layout ("a/../x.yaml"): the rule is about
	// the name, not where it resolves.
	for _, name := range []string{
		"../app/x.yaml",
		"/../app/x.yaml",
		"../../../clusters/default/app/x.yaml",
		"a/../x.yaml",
	} {
		t.Run(name, func(t *testing.T) {
			cfg := Config{ManifestFileName: func(_, _, _ string, _ FileExportMode) string { return name }}
			err := WriteManifest(filepath.Join(t.TempDir(), "out"), cfg, extraFileLayout(ExtraFile{Name: "values.yaml", Content: []byte("x: 1\n")}))
			if err == nil || !strings.Contains(err.Error(), "must not contain") {
				t.Fatalf("expected a traversal refusal, got %v", err)
			}
		})
	}
}

func TestDiskWriters_RefusedTraversalCreatesNoDirectory(t *testing.T) {
	// A layout refused for a ".." in its Namespace leaves nothing behind: the
	// check runs before the writer creates the layout's directory, which
	// would otherwise land outside the base.
	for wname, write := range map[string]func(base string, ml *ManifestLayout) error{
		"WriteToDisk":   func(base string, ml *ManifestLayout) error { return ml.WriteToDisk(base) },
		"WriteManifest": func(base string, ml *ManifestLayout) error { return WriteManifest(base, Config{ManifestsDir: "."}, ml) },
	} {
		t.Run(wname, func(t *testing.T) {
			root := t.TempDir()
			base := filepath.Join(root, "out")
			ml := extraFileLayout(ExtraFile{Name: "values.yaml", Content: []byte("x: 1\n")})
			ml.Namespace = "../escape"
			if err := write(base, ml); err == nil || !strings.Contains(err.Error(), "must not contain") {
				t.Fatalf("expected a traversal refusal, got %v", err)
			}
			if _, err := os.Stat(filepath.Join(root, "escape")); err == nil {
				t.Errorf("the refused layout created %s outside the base", filepath.Join(root, "escape"))
			}
		})
	}
}

func TestWriteManifest_ConfiguredChildSingleMode(t *testing.T) {
	// A child with no ApplicationFileMode of its own takes Config's
	// AppFileSingle in WriteManifest, so it writes single.yaml into
	// default/app, the parent's directory: the parent's extra of that name
	// is refused.
	parent := extraFileLayout(ExtraFile{Name: "single.yaml", Content: []byte("x: 1\n")})
	parent.ApplicationFileMode = AppFilePerResource
	parent.Children = []*ManifestLayout{{
		Name: "single", Namespace: "default/app",
		Resources: []client.Object{testObject("v1", "ConfigMap", "bar", "default")},
	}}
	err := WriteManifest(t.TempDir(), Config{ApplicationFileMode: AppFileSingle}, parent)
	if err == nil || !strings.Contains(err.Error(), "would replace the child file") {
		t.Fatalf("expected a child-file refusal, got %v", err)
	}
}

func TestWriteToTar_RefusedTraversalWritesNothing(t *testing.T) {
	// The tar writer streams to an io.Writer and cannot take an entry back,
	// so a layout refused for a ".." writes no entry at all, not even its
	// directory entry (only the end-of-archive trailer the writer's Close
	// always emits).
	ml := extraFileLayout(ExtraFile{Name: "values.yaml", Content: []byte("x: 1\n")})
	ml.Namespace = "../escape"
	var buf bytes.Buffer
	if err := ml.WriteToTar(&buf); err == nil || !strings.Contains(err.Error(), "must not contain") {
		t.Fatalf("expected a traversal refusal, got %v", err)
	}
	tr := tar.NewReader(&buf)
	if hdr, err := tr.Next(); err != io.EOF {
		t.Errorf("WriteToTar emitted an entry before refusing: %v (err %v)", hdr, err)
	}
}
