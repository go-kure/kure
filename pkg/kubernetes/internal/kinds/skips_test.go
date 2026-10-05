package kinds

import (
	"strings"
	"testing"

	"k8s.io/apimachinery/pkg/runtime/schema"
)

func gvk(group, version, kind string) schema.GroupVersionKind {
	return schema.GroupVersionKind{Group: group, Version: version, Kind: kind}
}

// Two versions of one kind in one package, the shape the skip table exists for,
// beside a kind that has nothing to do with it.
func twoVersions() []Kind {
	return []Kind{
		{GVK: gvk("example.com", "v1", "Other"), Package: "p"},
		{GVK: gvk("example.com", "v1beta1", "Thing"), Package: "p"},
		{GVK: gvk("example.com", "v1beta2", "Thing"), Package: "p"},
	}
}

func TestApplySkipsMarksTheNamedKindOnly(t *testing.T) {
	all := twoVersions()
	skipped := map[schema.GroupVersionKind]string{gvk("example.com", "v1beta1", "Thing"): "deprecated"}
	if err := applySkips(all, skipped); err != nil {
		t.Fatal(err)
	}
	for _, k := range all {
		if want := k.GVK.Version == "v1beta1"; k.WrapperSkipped != want {
			t.Errorf("%s: WrapperSkipped = %v, want %v", k.GVK, k.WrapperSkipped, want)
		}
	}

	wrapped := Wrapped(all)
	if len(wrapped) != 2 || wrapped[0].GVK.Kind != "Other" || wrapped[1].GVK.Version != "v1beta2" {
		t.Errorf("Wrapped = %+v, want Other and the v1beta2 Thing, in order", wrapped)
	}
	if len(all) != 3 {
		t.Errorf("Wrapped must not shorten the slice it was given: %d kinds left", len(all))
	}
}

// The table resolves a wrapper-name clash and nothing else. Each of these
// entries would otherwise remove a constructor with nothing going red: the
// generator would simply not emit it.
func TestApplySkipsRefusesAnEntryThatResolvesNoClash(t *testing.T) {
	cases := []struct {
		name    string
		all     []Kind
		skipped map[schema.GroupVersionKind]string
		want    string
	}{
		{
			name:    "not registered",
			all:     twoVersions(),
			skipped: map[schema.GroupVersionKind]string{gvk("example.com", "v1alpha1", "Thing"): "gone"},
			want:    "not registered",
		},
		{
			name:    "no other version takes the name",
			all:     twoVersions(),
			skipped: map[schema.GroupVersionKind]string{gvk("example.com", "v1", "Other"): "unwanted"},
			want:    "no other registered kind",
		},
		{
			name: "every version skipped",
			all:  twoVersions(),
			skipped: map[schema.GroupVersionKind]string{
				gvk("example.com", "v1beta1", "Thing"): "deprecated",
				gvk("example.com", "v1beta2", "Thing"): "also unwanted",
			},
			want: "no other registered kind",
		},
		{
			name: "the other version lives in another package",
			all: []Kind{
				{GVK: gvk("example.com", "v1beta1", "Thing"), Package: "p"},
				{GVK: gvk("example.com", "v1beta2", "Thing"), Package: "q"},
			},
			skipped: map[schema.GroupVersionKind]string{gvk("example.com", "v1beta1", "Thing"): "deprecated"},
			want:    "no other registered kind",
		},
		{
			name:    "no reason given",
			all:     twoVersions(),
			skipped: map[schema.GroupVersionKind]string{gvk("example.com", "v1beta1", "Thing"): ""},
			want:    "states no reason",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := applySkips(c.all, c.skipped)
			if err == nil {
				t.Fatal("expected an error")
			}
			if !strings.Contains(err.Error(), c.want) {
				t.Errorf("error = %v, want it to contain %q", err, c.want)
			}
		})
	}
}

// The one real entry. Both versions of the MetalLB BGPPeer are registered, so
// both parse to their typed object; v1beta2 keeps the constructor and v1beta1,
// which upstream deprecates, is the one named as skipped.
func TestRegistered_BGPPeerIsRegisteredAtBothVersionsAndWrappedAtOne(t *testing.T) {
	all, err := Registered()
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]Kind{}
	skipped := 0
	for _, k := range all {
		if k.WrapperSkipped {
			skipped++
			if _, named := skippedWrappers[k.GVK]; !named {
				t.Errorf("%s is marked skipped but skippedWrappers does not name it", k.GVK)
			}
		}
		if k.GVK.Group == "metallb.io" && k.GVK.Kind == "BGPPeer" {
			got[k.GVK.Version] = k
		}
	}
	if skipped != len(skippedWrappers) {
		t.Errorf("%d kinds marked skipped, skippedWrappers names %d", skipped, len(skippedWrappers))
	}
	if len(got) != 2 {
		t.Fatalf("metallb.io BGPPeer registered at %d versions, want v1beta1 and v1beta2: %+v", len(got), got)
	}
	for version, wantSkipped := range map[string]bool{"v1beta1": true, "v1beta2": false} {
		k, ok := got[version]
		if !ok {
			t.Errorf("metallb.io/%s BGPPeer is not registered", version)
			continue
		}
		if k.WrapperSkipped != wantSkipped {
			t.Errorf("metallb.io/%s BGPPeer: WrapperSkipped = %v, want %v", version, k.WrapperSkipped, wantSkipped)
		}
		if k.Package != "metallb" || !k.Namespaced {
			t.Errorf("metallb.io/%s BGPPeer: package=%q namespaced=%v, want metallb/true", version, k.Package, k.Namespaced)
		}
		if want := "go.universe.tf/metallb/api/" + version; k.ImportPath != want {
			t.Errorf("metallb.io/%s BGPPeer: import path %q, want %q", version, k.ImportPath, want)
		}
	}
}
