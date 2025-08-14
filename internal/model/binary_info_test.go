package model_test

import (
	"testing"

	"github.com/brunoribeiro127/gobin/internal/model"
	"github.com/stretchr/testify/assert"
)

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
