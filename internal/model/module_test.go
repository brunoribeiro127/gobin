package model_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/brunoribeiro127/gobin/internal/model"
)

func TestModule_GetBaseModule(t *testing.T) {
	cases := map[string]struct {
		module   model.Module
		expected string
	}{
		"no-version-suffix": {
			module:   model.NewLatestModule("example.com/mockorg/mockproj"),
			expected: "example.com/mockorg/mockproj",
		},
		"with-version-suffix": {
			module:   model.NewLatestModule("example.com/mockorg/mockproj/v2"),
			expected: "example.com/mockorg/mockproj",
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			result := tc.module.GetBaseModule()
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestModule_IsValid(t *testing.T) {
	cases := map[string]struct {
		module   model.Module
		expected bool
	}{
		"valid-latest": {
			module:   model.NewLatestModule("example.com/mockorg/mockproj"),
			expected: true,
		},
		"valid-version": {
			module:   model.NewModule("example.com/mockorg/mockproj", model.NewVersion("v1.0.0")),
			expected: true,
		},
		"invalid-empty-path": {
			module:   model.NewLatestModule(""),
			expected: false,
		},
		"invalid-version": {
			module:   model.NewModule("example.com/mockorg/mockproj", model.NewVersion("1.0.0")),
			expected: false,
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			result := tc.module.IsValid()
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestModule_NextMajorModule(t *testing.T) {
	cases := map[string]struct {
		module   model.Module
		expected model.Module
	}{
		"no-version-suffix": {
			module:   model.NewLatestModule("example.com/mockorg/mockproj"),
			expected: model.NewLatestModule("example.com/mockorg/mockproj/v2"),
		},
		"with-version-suffix": {
			module:   model.NewLatestModule("example.com/mockorg/mockproj/v2"),
			expected: model.NewLatestModule("example.com/mockorg/mockproj/v3"),
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			result := tc.module.NextMajorModule()
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestModule_String(t *testing.T) {
	cases := map[string]struct {
		module   model.Module
		expected string
	}{
		"latest": {
			module:   model.NewLatestModule("example.com/mockorg/mockproj"),
			expected: "example.com/mockorg/mockproj@latest",
		},
		"with-version": {
			module:   model.NewModule("example.com/mockorg/mockproj", model.NewVersion("v1.0.0")),
			expected: "example.com/mockorg/mockproj@v1.0.0",
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			result := tc.module.String()
			assert.Equal(t, tc.expected, result)
		})
	}
}
