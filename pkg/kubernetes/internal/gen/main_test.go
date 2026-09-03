package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/go-kure/kure/pkg/kubernetes/internal/kinds"
)

func TestRun_GenerateThenCheckIsClean(t *testing.T) {
	root := t.TempDir()
	var stderr bytes.Buffer

	if rc := run([]string{"-root", root}, &stderr); rc != 0 {
		t.Fatalf("generate: rc=%d stderr=%s", rc, stderr.String())
	}
	if rc := run([]string{"-check", "-root", root}, &stderr); rc != 0 {
		t.Fatalf("check after generate: rc=%d stderr=%s", rc, stderr.String())
	}
	for _, f := range []string{createFile, createTestFile, registryFile, filepath.Join("fluxcd", createFile)} {
		if _, err := os.Stat(filepath.Join(root, f)); err != nil {
			t.Errorf("expected generated file %s: %v", f, err)
		}
	}
}

func TestRun_CheckFailsOnStaleAndMissingFiles(t *testing.T) {
	root := t.TempDir()
	var stderr bytes.Buffer
	if rc := run([]string{"-check", "-root", root}, &stderr); rc != 1 {
		t.Fatalf("check on empty root: rc=%d, want 1", rc)
	}
	if !strings.Contains(stderr.String(), "is stale (run scripts/gen-builders.sh generate)") {
		t.Errorf("stderr should name the stale file: %s", stderr.String())
	}

	stderr.Reset()
	if rc := run([]string{"-root", root}, &stderr); rc != 0 {
		t.Fatal(stderr.String())
	}
	path := filepath.Join(root, createFile)
	if err := os.WriteFile(path, []byte("// stale\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	stderr.Reset()
	if rc := run([]string{"-check", "-root", root}, &stderr); rc != 1 {
		t.Fatalf("check on stale file: rc=%d, want 1", rc)
	}
	if !strings.Contains(stderr.String(), path) {
		t.Errorf("stderr should name %s: %s", path, stderr.String())
	}
}

func TestRun_OrphanedPackageFilesFailCheckAndAreRemovedByGenerate(t *testing.T) {
	root := t.TempDir()
	var stderr bytes.Buffer
	if rc := run([]string{"-root", root}, &stderr); rc != 0 {
		t.Fatal(stderr.String())
	}
	// A package that once had kinds and lost them all leaves its generated
	// files behind; a hand-written file in the same directory is not touched.
	gone := filepath.Join(root, "gone")
	if err := os.MkdirAll(gone, 0o750); err != nil {
		t.Fatal(err)
	}
	orphan := filepath.Join(gone, createFile)
	orphanTest := filepath.Join(gone, createTestFile)
	keep := filepath.Join(gone, "sugar.go")
	for _, f := range []string{orphan, orphanTest, keep} {
		if err := os.WriteFile(f, []byte("package gone\n"), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	stderr.Reset()
	if rc := run([]string{"-check", "-root", root}, &stderr); rc != 1 {
		t.Fatalf("check with orphans: rc=%d, want 1; stderr=%s", rc, stderr.String())
	}
	for _, f := range []string{orphan, orphanTest} {
		if !strings.Contains(stderr.String(), f+" is orphaned") {
			t.Errorf("stderr should name orphan %s: %s", f, stderr.String())
		}
	}

	stderr.Reset()
	if rc := run([]string{"-root", root}, &stderr); rc != 0 {
		t.Fatalf("generate with orphans: rc=%d stderr=%s", rc, stderr.String())
	}
	for _, f := range []string{orphan, orphanTest} {
		if _, err := os.Stat(f); !os.IsNotExist(err) {
			t.Errorf("generate should remove orphan %s, stat err=%v", f, err)
		}
	}
	if _, err := os.Stat(keep); err != nil {
		t.Errorf("generate must not touch hand-written %s: %v", keep, err)
	}
	stderr.Reset()
	if rc := run([]string{"-check", "-root", root}, &stderr); rc != 0 {
		t.Fatalf("check after removing orphans: rc=%d stderr=%s", rc, stderr.String())
	}
}

func TestFindOrphans_MissingRootHasNone(t *testing.T) {
	orphans, err := findOrphans(filepath.Join(t.TempDir(), "absent"), map[string][]byte{})
	if err != nil || len(orphans) != 0 {
		t.Errorf("orphans=%v err=%v, want none", orphans, err)
	}
}

func TestFindOrphans_UnreadableDirIsAnError(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root ignores directory permissions")
	}
	root := t.TempDir()
	locked := filepath.Join(root, "locked")
	if err := os.MkdirAll(locked, 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(locked, 0o000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(locked, 0o750) })
	if _, err := findOrphans(root, map[string][]byte{}); err == nil {
		t.Error("expected a walk error for an unreadable directory")
	}
}

func TestRun_OrphanRemovalFailureIsReported(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root ignores directory permissions")
	}
	root := t.TempDir()
	var stderr bytes.Buffer
	if rc := run([]string{"-root", root}, &stderr); rc != 0 {
		t.Fatal(stderr.String())
	}
	gone := filepath.Join(root, "gone")
	if err := os.MkdirAll(gone, 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(gone, createFile), []byte("package gone\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(gone, 0o500); err != nil { // readable, not writable: Remove fails
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(gone, 0o750) })
	stderr.Reset()
	if rc := run([]string{"-root", root}, &stderr); rc != 1 {
		t.Errorf("rc=%d, want 1 when an orphan cannot be removed; stderr=%s", rc, stderr.String())
	}
}

func TestRun_BadFlag(t *testing.T) {
	var stderr bytes.Buffer
	if rc := run([]string{"-nope"}, &stderr); rc != 2 {
		t.Errorf("rc=%d, want 2", rc)
	}
}

func TestRun_WriteErrorIsReported(t *testing.T) {
	file := filepath.Join(t.TempDir(), "not-a-dir")
	if err := os.WriteFile(file, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	var stderr bytes.Buffer
	if rc := run([]string{"-root", file}, &stderr); rc != 1 {
		t.Errorf("rc=%d, want 1 when root is a file", rc)
	}
}

func TestRender_OneCreateFilePerPackageAndAllWrappersUnique(t *testing.T) {
	files, err := render("x")
	if err != nil {
		t.Fatal(err)
	}
	all, err := kinds.Registered()
	if err != nil {
		t.Fatal(err)
	}
	pkgs := map[string]bool{}
	for _, k := range all {
		pkgs[k.Package] = true
	}
	for pkg := range pkgs {
		if _, ok := files[filepath.Join("x", pkg, createFile)]; !ok {
			t.Errorf("no %s for package %q", createFile, pkg)
		}
		if _, ok := files[filepath.Join("x", pkg, createTestFile)]; !ok {
			t.Errorf("no %s for package %q", createTestFile, pkg)
		}
	}
	if _, ok := files[filepath.Join("x", registryFile)]; !ok {
		t.Errorf("no registry file")
	}
	for path, src := range files {
		if !bytes.HasPrefix(src, []byte("// Code generated by pkg/kubernetes/internal/gen; DO NOT EDIT.")) {
			t.Errorf("%s lacks the generated header", path)
		}
	}
	root := string(files[filepath.Join("x", createFile)])
	if !strings.Contains(root, "func CreateDeployment(name, namespace string) *appsv1.Deployment {\n\treturn Create[appsv1.Deployment](name, namespace)\n}") {
		t.Errorf("root create file lacks the Deployment wrapper:\n%s", root)
	}
	if !strings.Contains(root, "func CreateNamespace(name string) *corev1.Namespace {\n\treturn Create[corev1.Namespace](name, \"\")\n}") {
		t.Errorf("root create file lacks the cluster-scoped Namespace wrapper")
	}
	sub := string(files[filepath.Join("x", "fluxcd", createFile)])
	if !strings.Contains(sub, "kubernetes.Create[") || !strings.Contains(sub, "\"github.com/go-kure/kure/pkg/kubernetes\"") {
		t.Errorf("subpackage wrappers must call kubernetes.Create:\n%s", sub)
	}
}

func TestCheckUniqueWrappers(t *testing.T) {
	a := kinds.Kind{}
	a.GVK.Kind = "Thing"
	a.GVK.Group = "a"
	b := a
	b.GVK.Group = "b"
	if err := checkUniqueWrappers("p", []kinds.Kind{a, b}); err == nil {
		t.Error("expected a duplicate-wrapper error")
	}
	if err := checkUniqueWrappers("p", []kinds.Kind{a}); err != nil {
		t.Error(err)
	}
}

func TestImportAliases_Collision(t *testing.T) {
	a := kinds.Kind{ImportPath: "example.com/x/api/v1"}
	b := kinds.Kind{ImportPath: "example.com/y/x/v1"}
	if _, err := importAliases([]kinds.Kind{a, b}); err == nil {
		t.Error("expected an alias collision error for two paths aliasing to xv1")
	}
	got, err := importAliases([]kinds.Kind{a, a})
	if err != nil || got[a.ImportPath] != "xv1" {
		t.Errorf("aliases = %v, err = %v", got, err)
	}
}

func TestAliasFor(t *testing.T) {
	cases := map[string]string{
		"k8s.io/api/core/v1": "corev1",
		"k8s.io/api/apps/v1": "appsv1",
		"github.com/cert-manager/cert-manager/pkg/apis/certmanager/v1": "certmanagerv1",
		"github.com/fluxcd/source-watcher/api/v2/v1beta1":              "sourcewatcherv2v1beta1",
		"github.com/cloudnative-pg/cloudnative-pg/api/v1":              "cloudnativepgv1",
		"sigs.k8s.io/gateway-api/apis/v1":                              "gatewayapiv1",
	}
	for in, want := range cases {
		if got := aliasFor(in); got != want {
			t.Errorf("aliasFor(%q) = %q, want %q", in, got, want)
		}
	}
}
