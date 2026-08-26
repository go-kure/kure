module github.com/example/fixture

go 1.26.6

// Current pin: v0.36.4 (Kubernetes 1.36)

replace (
	k8s.io/api => k8s.io/api v0.36.4
)

require (
	k8s.io/api v0.36.4
	github.com/example/floor-dep v0.5.1
	github.com/example/floor-owner v1.0.0
	github.com/example/range-dep v2.0.3
	github.com/example/boundary-dep v1.6.0
	github.com/example/k8s-basis-dep v0.36.4
	github.com/example/pseudo-dep v0.0.0-20260101000000-000000000099
	// Deliberately form-1 (vX.0.0-<ts>-<hash>), not vX.Y.Z-0.<ts>-<hash>:
	// is_pseudo_version() (sync-versions.sh:104) only matches form 1 by
	// design (barman-cloud's real-world need -- see its own comment). A
	// form-3 pin here would skip the upstream_release live-check/
	// substitution branch entirely and pass the ordinary range check by
	// coincidence, which is exactly what an earlier draft of this fixture
	// did and rows 15-17 never actually exercised what they claimed to.
	github.com/example/upstream-dep v0.0.0-20260101000000-eeeeeeeeeeee
	github.com/example/net-dep v0.0.0-20260101000000-aaaaaaaaaaaa
)
