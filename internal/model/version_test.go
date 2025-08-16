package model_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/brunoribeiro127/gobin/internal/model"
)

func TestNewVersion(t *testing.T) {
	cases := map[string]struct {
		version  string
		expected model.Version
	}{
		"regular": {
			version:  "v1.2.3",
			expected: model.Version("v1.2.3"),
		},
		"latest": {
			version:  "latest",
			expected: model.Version("latest"),
		},
		"empty": {
			version:  "",
			expected: model.Version(""),
		},
		"with-spaces": {
			version:  " v1.2.3 ",
			expected: model.Version("v1.2.3"),
		},
		"with-case": {
			version:  "v1.2.3-ALPHA",
			expected: model.Version("v1.2.3-alpha"),
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			result := model.NewVersion(tc.version)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestVersion_Compare(t *testing.T) {
	cases := map[string]struct {
		v1       model.Version
		v2       model.Version
		expected int
	}{
		"eq": {
			v1:       model.Version("v1.2.3"),
			v2:       model.Version("v1.2.3"),
			expected: 0,
		},
		"v1-major": {
			v1:       model.Version("v2.0.0"),
			v2:       model.Version("v1.2.3"),
			expected: 1,
		},
		"v1-minor": {
			v1:       model.Version("v1.3.0"),
			v2:       model.Version("v1.2.3"),
			expected: 1,
		},
		"v1-patch": {
			v1:       model.Version("v1.2.4"),
			v2:       model.Version("v1.2.3"),
			expected: 1,
		},
		"v2-major": {
			v1:       model.Version("v1.2.3"),
			v2:       model.Version("v2.0.0"),
			expected: -1,
		},
		"v2-minor": {
			v1:       model.Version("v1.2.3"),
			v2:       model.Version("v1.3.0"),
			expected: -1,
		},
		"v2-patch": {
			v1:       model.Version("v1.2.3"),
			v2:       model.Version("v1.2.4"),
			expected: -1,
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			result := tc.v1.Compare(tc.v2)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestVersion_IsLatest(t *testing.T) {
	cases := map[string]struct {
		version  model.Version
		expected bool
	}{
		"latest": {
			version:  model.Version("latest"),
			expected: true,
		},
		"semantic-version": {
			version:  model.Version("v1.2.3"),
			expected: false,
		},
		"empty": {
			version:  model.Version(""),
			expected: false,
		},
		"latest-with-case": {
			version:  model.Version("Latest"),
			expected: false,
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			result := tc.version.IsLatest()
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestVersion_IsPartOf(t *testing.T) {
	cases := map[string]struct {
		version     model.Version
		baseVersion model.Version
		expected    bool
	}{
		"major-version-match": {
			version:     model.Version("v1.2.3"),
			baseVersion: model.Version("v1"),
			expected:    true,
		},
		"major-minor-version-match": {
			version:     model.Version("v1.2.5"),
			baseVersion: model.Version("v1.2"),
			expected:    true,
		},
		"exact-match": {
			version:     model.Version("v1.2.3"),
			baseVersion: model.Version("v1.2.3"),
			expected:    true,
		},
		"major-version-no-match": {
			version:     model.Version("v2.0.0"),
			baseVersion: model.Version("v1"),
			expected:    false,
		},
		"major-minor-version-no-match": {
			version:     model.Version("v1.3.0"),
			baseVersion: model.Version("v1.2"),
			expected:    false,
		},
		"full-version-no-match": {
			version:     model.Version("v1.2.4"),
			baseVersion: model.Version("v1.2.3"),
			expected:    false,
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			result := tc.version.IsPartOf(tc.baseVersion)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestVersion_IsValid(t *testing.T) {
	cases := map[string]struct {
		version  model.Version
		expected bool
	}{
		"valid-latest": {
			version:  model.Version("latest"),
			expected: true,
		},
		"valid-semver-version": {
			version:  model.Version("v1.2.3"),
			expected: true,
		},
		"valid-pre-release": {
			version:  model.Version("v1.2.3-alpha"),
			expected: true,
		},
		"valid-pseudo-version": {
			version:  model.Version("v0.0.0-20250806180942-db69247c9fc7"),
			expected: true,
		},
		"invalid-without-v-prefix": {
			version:  model.Version("1.2.3"),
			expected: false,
		},
		"invalid-version-format": {
			version:  model.Version("invalid"),
			expected: false,
		},
		"empty": {
			version:  model.Version(""),
			expected: false,
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			result := tc.version.IsValid()
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestVersion_Major(t *testing.T) {
	cases := map[string]struct {
		version  model.Version
		expected string
	}{
		"regular": {
			version:  model.Version("v1.2.3"),
			expected: "v1",
		},
		"pre-release": {
			version:  model.Version("v2.1.0-alpha"),
			expected: "v2",
		},
		"pseudo-version": {
			version:  model.Version("v0.0.0-20250806180942-db69247c9fc7"),
			expected: "v0",
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			result := tc.version.Major()
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestVersion_MajorMinor(t *testing.T) {
	cases := map[string]struct {
		version  model.Version
		expected string
	}{
		"regular": {
			version:  model.Version("v1.2.3"),
			expected: "v1.2",
		},
		"pre-release": {
			version:  model.Version("v2.1.0-alpha"),
			expected: "v2.1",
		},
		"pseudo-version": {
			version:  model.Version("v0.0.0-20250806180942-db69247c9fc7"),
			expected: "v0.0",
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			result := tc.version.MajorMinor()
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestVersion_NextMajorVersion(t *testing.T) {
	cases := map[string]struct {
		version  model.Version
		expected model.Version
	}{
		"v0-to-v2": {
			version:  model.Version("v0.5.1"),
			expected: model.Version("v2"),
		},
		"v1-to-v2": {
			version:  model.Version("v1.2.3"),
			expected: model.Version("v2"),
		},
		"v2-to-v3": {
			version:  model.Version("v2.0.0"),
			expected: model.Version("v3"),
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			result := tc.version.NextMajorVersion()
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestVersion_String(t *testing.T) {
	cases := map[string]struct {
		version  model.Version
		expected string
	}{
		"latest": {
			version:  model.NewLatestVersion(),
			expected: "latest",
		},
		"semantic-version": {
			version:  model.NewVersion("v1.2.3"),
			expected: "v1.2.3",
		},
		"pre-release": {
			version:  model.NewVersion("v1.0.0-alpha"),
			expected: "v1.0.0-alpha",
		},
		"empty": {
			version:  model.NewVersion(""),
			expected: "",
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			result := tc.version.String()
			assert.Equal(t, tc.expected, result)
		})
	}
}
