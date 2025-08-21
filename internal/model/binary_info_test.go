package model_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/brunoribeiro127/gobin/internal/model"
)

func TestBinaryInfo_GetPinnedVersion(t *testing.T) {
	cases := map[string]struct {
		binaryInfo model.BinaryInfo
		expected   model.Version
	}{
		"latest-version": {
			binaryInfo: model.BinaryInfo{
				Name: "mockproj",
			},
			expected: model.NewLatestVersion(),
		},
		"latest-version-multiple-parts": {
			binaryInfo: model.BinaryInfo{
				Name: "mockproj-test",
			},
			expected: model.NewLatestVersion(),
		},
		"major-version": {
			binaryInfo: model.BinaryInfo{
				Name: "mockproj-v1",
			},
			expected: model.NewVersion("v1"),
		},
		"major-version-multiple-parts": {
			binaryInfo: model.BinaryInfo{
				Name: "mockproj-test-v1",
			},
			expected: model.NewVersion("v1"),
		},
		"minor-version": {
			binaryInfo: model.BinaryInfo{
				Name: "mockproj-v1.2",
			},
			expected: model.NewVersion("v1.2"),
		},
		"minor-version-multiple-parts": {
			binaryInfo: model.BinaryInfo{
				Name: "mockproj-test-v1.2",
			},
			expected: model.NewVersion("v1.2"),
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			result := tc.binaryInfo.GetPinnedVersion()
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestBinaryUpgradeInfo_GetUpgradePackage(t *testing.T) {
	cases := map[string]struct {
		binaryInfo model.BinaryUpgradeInfo
		expected   model.Package
	}{
		"same-module-package-paths": {
			binaryInfo: model.BinaryUpgradeInfo{
				BinaryInfo: model.BinaryInfo{
					PackagePath: "example.com/mockorg/mockproj",
					Module: model.Module{
						Path: "example.com/mockorg/mockproj",
					},
				},
				LatestModule: model.NewModule("example.com/mockorg/mockproj/v1", model.NewVersion("v1.0.0")),
			},
			expected: model.NewPackage("example.com/mockorg/mockproj@v1.0.0"),
		},
		"different-module-package-paths": {
			binaryInfo: model.BinaryUpgradeInfo{
				BinaryInfo: model.BinaryInfo{
					PackagePath: "example.com/mockorg/mockproj/cmd/mockproj",
					Module: model.Module{
						Path: "example.com/mockorg/mockproj",
					},
				},
				LatestModule: model.NewModule("example.com/mockorg/mockproj", model.NewVersion("v1.0.0")),
			},
			expected: model.NewPackage("example.com/mockorg/mockproj/cmd/mockproj@v1.0.0"),
		},
		"same-module-package-paths-major-version": {
			binaryInfo: model.BinaryUpgradeInfo{
				BinaryInfo: model.BinaryInfo{
					PackagePath: "example.com/mockorg/mockproj",
					Module: model.Module{
						Path: "example.com/mockorg/mockproj",
					},
				},
				LatestModule: model.NewModule("example.com/mockorg/mockproj/v2", model.NewVersion("v2.0.0")),
			},
			expected: model.NewPackage("example.com/mockorg/mockproj/v2@v2.0.0"),
		},
		"different-module-package-paths-major-version": {
			binaryInfo: model.BinaryUpgradeInfo{
				BinaryInfo: model.BinaryInfo{
					PackagePath: "example.com/mockorg/mockproj/cmd/mockproj",
					Module: model.Module{
						Path: "example.com/mockorg/mockproj",
					},
				},
				LatestModule: model.NewModule("example.com/mockorg/mockproj/v2", model.NewVersion("v2.0.0")),
			},
			expected: model.NewPackage("example.com/mockorg/mockproj/v2/cmd/mockproj@v2.0.0"),
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			result := tc.binaryInfo.GetUpgradePackage()
			assert.Equal(t, tc.expected, result)
		})
	}
}
