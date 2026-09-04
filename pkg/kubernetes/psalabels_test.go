package kubernetes

import (
	"testing"
)

func TestPSALabels_AllModes(t *testing.T) {
	got := PSALabels(PSARestricted, PSARestricted, PSARestricted, "v1.28")

	want := map[string]string{
		"pod-security.kubernetes.io/enforce":         "restricted",
		"pod-security.kubernetes.io/enforce-version": "v1.28",
		"pod-security.kubernetes.io/warn":            "restricted",
		"pod-security.kubernetes.io/warn-version":    "v1.28",
		"pod-security.kubernetes.io/audit":           "restricted",
		"pod-security.kubernetes.io/audit-version":   "v1.28",
	}
	if len(got) != len(want) {
		t.Fatalf("expected %d labels, got %d: %v", len(want), len(got), got)
	}
	for k, v := range want {
		if got[k] != v {
			t.Errorf("expected label %s=%s, got %q", k, v, got[k])
		}
	}
}

func TestPSALabels_NoVersion(t *testing.T) {
	got := PSALabels(PSARestricted, PSABaseline, PSAPrivileged, "")

	if got["pod-security.kubernetes.io/enforce"] != "restricted" {
		t.Errorf("expected enforce=restricted, got %q", got["pod-security.kubernetes.io/enforce"])
	}
	if got["pod-security.kubernetes.io/warn"] != "baseline" {
		t.Errorf("expected warn=baseline, got %q", got["pod-security.kubernetes.io/warn"])
	}
	if got["pod-security.kubernetes.io/audit"] != "privileged" {
		t.Errorf("expected audit=privileged, got %q", got["pod-security.kubernetes.io/audit"])
	}
	for _, k := range []string{
		"pod-security.kubernetes.io/enforce-version",
		"pod-security.kubernetes.io/warn-version",
		"pod-security.kubernetes.io/audit-version",
	} {
		if _, ok := got[k]; ok {
			t.Errorf("expected no version label %s when version is empty", k)
		}
	}
}

func TestPSALabels_SkipEmptyModes(t *testing.T) {
	got := PSALabels(PSARestricted, "", "", "latest")

	if got["pod-security.kubernetes.io/enforce"] != "restricted" {
		t.Errorf("expected enforce=restricted, got %q", got["pod-security.kubernetes.io/enforce"])
	}
	if got["pod-security.kubernetes.io/enforce-version"] != "latest" {
		t.Errorf("expected enforce-version=latest, got %q", got["pod-security.kubernetes.io/enforce-version"])
	}
	if len(got) != 2 {
		t.Errorf("expected only the enforce pair, got %v", got)
	}
}

func TestPSALabels_NoModes(t *testing.T) {
	got := PSALabels("", "", "", "v1.28")
	if len(got) != 0 {
		t.Errorf("expected an empty map, got %v", got)
	}
}

// TestPSALabels_MergeOntoNamespace is the documented call shape: the value
// helper builds the map, AddLabel applies it without touching other labels.
func TestPSALabels_MergeOntoNamespace(t *testing.T) {
	ns := CreateNamespace("myapp")
	AddLabel(ns, "team", "platform")
	for k, v := range PSALabels(PSARestricted, PSARestricted, PSARestricted, "") {
		AddLabel(ns, k, v)
	}

	if ns.Labels["team"] != "platform" {
		t.Errorf("expected pre-existing label to survive, got %q", ns.Labels["team"])
	}
	for _, k := range []string{
		"pod-security.kubernetes.io/enforce",
		"pod-security.kubernetes.io/warn",
		"pod-security.kubernetes.io/audit",
	} {
		if ns.Labels[k] != "restricted" {
			t.Errorf("expected %s=restricted, got %q", k, ns.Labels[k])
		}
	}
	if len(ns.Labels) != 4 {
		t.Errorf("expected 4 labels, got %v", ns.Labels)
	}
}
