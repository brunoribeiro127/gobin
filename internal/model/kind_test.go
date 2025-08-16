package model_test

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/brunoribeiro127/gobin/internal/model"
)

func TestKind_GetTargetBinPath(t *testing.T) {
	cases := map[string]struct {
		kind     model.Kind
		basePath string
		name     string
		version  model.Version
		expected string
	}{
		"latest": {
			kind:     model.KindLatest,
			basePath: "/usr/local/bin",
			name:     "mockproj",
			version:  model.NewLatestVersion(),
			expected: "/usr/local/bin/mockproj",
		},
		"major": {
			kind:     model.KindMajor,
			basePath: "/usr/local/bin",
			name:     "mockproj",
			version:  model.NewVersion("v1.2.3"),
			expected: "/usr/local/bin/mockproj-v1",
		},
		"minor": {
			kind:     model.KindMinor,
			basePath: "/usr/local/bin",
			name:     "mockproj",
			version:  model.NewVersion("v1.2.3"),
			expected: "/usr/local/bin/mockproj-v1.2",
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			result := tc.kind.GetTargetBinPath(tc.basePath, tc.name, tc.version)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestKind_IsValid(t *testing.T) {
	cases := map[string]struct {
		kind     model.Kind
		expected bool
	}{
		"latest": {
			kind:     model.KindLatest,
			expected: true,
		},
		"major": {
			kind:     model.KindMajor,
			expected: true,
		},
		"minor": {
			kind:     model.KindMinor,
			expected: true,
		},
		"invalid": {
			kind:     "invalid",
			expected: false,
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			assert.Equal(t, tc.expected, tc.kind.IsValid())
		})
	}
}

func TestKind_String(t *testing.T) {
	cases := map[string]struct {
		kind     model.Kind
		expected string
	}{
		"latest": {
			kind:     model.KindLatest,
			expected: "latest",
		},
		"major": {
			kind:     model.KindMajor,
			expected: "major",
		},
		"minor": {
			kind:     model.KindMinor,
			expected: "minor",
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			assert.Equal(t, tc.expected, tc.kind.String())
		})
	}
}

func TestKind_Set(t *testing.T) {
	cases := map[string]struct {
		kind     string
		expected model.Kind
		err      error
	}{
		"latest": {
			kind:     "latest",
			expected: model.KindLatest,
		},
		"major": {
			kind:     "major",
			expected: model.KindMajor,
		},
		"minor": {
			kind:     "minor",
			expected: model.KindMinor,
		},
		"invalid": {
			kind: "invalid",
			err:  errors.New(`invalid kind "invalid", allowed values are: [latest major minor]`),
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			kind := model.Kind("")
			err := kind.Set(tc.kind)
			assert.Equal(t, tc.expected, kind)
			assert.Equal(t, tc.err, err)
		})
	}
}

func TestKind_Type(t *testing.T) {
	kind := model.Kind("")
	assert.Equal(t, "kind", kind.Type())
}
