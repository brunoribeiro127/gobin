package model_test

import (
	"testing"

	"github.com/brunoribeiro127/gobin/internal/model"
	"github.com/stretchr/testify/assert"
)

func TestBinaryDiagnostic_HasIssues(t *testing.T) {
	cases := map[string]struct {
		binaryDiagnostic model.BinaryDiagnostic
		expected         bool
	}{
		"no-issues": {
			binaryDiagnostic: model.BinaryDiagnostic{
				Name: "mockproj",
			},
			expected: false,
		},
		"all-issues": {
			binaryDiagnostic: model.BinaryDiagnostic{
				Name:      "mockproj",
				NotInPath: true,
				DuplicatesInPath: []string{
					"/usr/bin/mockproj",
					"/usr/local/bin/mockproj",
				},
				IsNotManaged:          true,
				IsPseudoVersion:       true,
				NotBuiltWithGoModules: true,
				IsOrphaned:            true,
				GoVersion: struct {
					Actual   string
					Expected string
				}{
					Actual:   "1.24.5",
					Expected: "1.24.6",
				},
				Platform: struct {
					Actual   string
					Expected string
				}{
					Actual:   "linux/amd64",
					Expected: "darwin/arm64",
				},
				Retracted:  "mock-retracted",
				Deprecated: "mock-deprecated",
				Vulnerabilities: []model.Vulnerability{
					{ID: "GO-2025-3770", URL: "https://pkg.go.dev/vuln/GO-2025-3770"},
				},
			},
			expected: true,
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			assert.Equal(t, tc.expected, tc.binaryDiagnostic.HasIssues())
		})
	}
}
