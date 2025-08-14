package model_test

import (
	"testing"

	"github.com/brunoribeiro127/gobin/internal/model"
	"github.com/stretchr/testify/assert"
)

func TestNewPackage(t *testing.T) {
	cases := map[string]struct {
		pkg      string
		expected model.Package
	}{
		"latest": {
			pkg: "example.com/mockorg/mockproj",
			expected: model.NewPackageWithVersion(
				"example.com/mockorg/mockproj",
				model.NewVersion("latest"),
			),
		},
		"with-version": {
			pkg: "example.com/mockorg/mockproj@v1.2.3",
			expected: model.NewPackageWithVersion(
				"example.com/mockorg/mockproj",
				model.NewVersion("v1.2.3"),
			),
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			result := model.NewPackage(tc.pkg)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestPackage_GetBinaryName(t *testing.T) {
	cases := map[string]struct {
		pkg      string
		expected string
	}{
		"regular": {
			pkg:      "example.com/mockorg/mockproj",
			expected: "mockproj",
		},
		"with-version": {
			pkg:      "example.com/mockorg/mockproj@v1.2.3",
			expected: "mockproj",
		},
		"with-version-and-path-version": {
			pkg:      "example.com/mockorg/mockproj/v2@v2.3.1",
			expected: "mockproj",
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			result := model.NewPackage(tc.pkg).GetBinaryName()
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestPackage_IsValid(t *testing.T) {
	cases := map[string]struct {
		pkg      string
		expected bool
	}{
		"valid-regular": {
			pkg:      "example.com/mockorg/mockproj",
			expected: true,
		},
		"valid-with-version": {
			pkg:      "example.com/mockorg/mockproj@v1.2.3",
			expected: true,
		},
		"empty-path": {
			pkg:      "",
			expected: false,
		},
		"invalid-version": {
			pkg:      "example.com/mockorg/mockproj@1.2.3",
			expected: false,
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			result := model.NewPackage(tc.pkg).IsValid()
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestPackage_String(t *testing.T) {
	cases := map[string]struct {
		pkg      string
		expected string
	}{
		"regular": {
			pkg:      "example.com/mockorg/mockproj",
			expected: "example.com/mockorg/mockproj@latest",
		},
		"with-version": {
			pkg:      "example.com/mockorg/mockproj@v1.2.3",
			expected: "example.com/mockorg/mockproj@v1.2.3",
		},
		"with-latest-version": {
			pkg:      "example.com/mockorg/mockproj@v1.2.3",
			expected: "example.com/mockorg/mockproj@v1.2.3",
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			result := model.NewPackage(tc.pkg).String()
			assert.Equal(t, tc.expected, result)
		})
	}
}
