package model_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/brunoribeiro127/gobin/internal/model"
)

func TestNewBinary(t *testing.T) {
	cases := map[string]struct {
		bin      string
		expected model.Binary
	}{
		"latest": {
			bin:      "mockproj",
			expected: model.NewBinaryWithVersion("mockproj", model.NewLatestVersion()),
		},
		"with-version": {
			bin:      "mockproj@v1.2.3",
			expected: model.NewBinaryWithVersion("mockproj", model.NewVersion("v1.2.3")),
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			result := model.NewBinary(tc.bin)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestBinary_IsPartOf(t *testing.T) {
	cases := map[string]struct {
		bin      model.Binary
		other    model.Binary
		expected bool
	}{
		"same-binary-latest": {
			bin:      model.NewBinary("mockproj"),
			other:    model.NewBinary("mockproj"),
			expected: true,
		},
		"same-binary-version": {
			bin:      model.NewBinary("mockproj@v1.2.3"),
			other:    model.NewBinary("mockproj@v1.2.3"),
			expected: true,
		},
		"part-of-version-latest": {
			bin:      model.NewBinary("mockproj@v1.2.3"),
			other:    model.NewBinary("mockproj@latest"),
			expected: true,
		},
		"part-of-version-major": {
			bin:      model.NewBinary("mockproj@v1.2.3"),
			other:    model.NewBinary("mockproj@v1"),
			expected: true,
		},
		"part-of-version-minor": {
			bin:      model.NewBinary("mockproj@v1.2.3"),
			other:    model.NewBinary("mockproj@v1.2"),
			expected: true,
		},
		"different-binary-name": {
			bin:      model.NewBinary("mockproj"),
			other:    model.NewBinary("mockproj2"),
			expected: false,
		},
		"different-binary-version": {
			bin:      model.NewBinary("mockproj@v1.2.3"),
			other:    model.NewBinary("mockproj@v1.2.4"),
			expected: false,
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			result := tc.bin.IsPartOf(tc.other)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestBinary_IsValid(t *testing.T) {
	cases := map[string]struct {
		bin      model.Binary
		expected bool
	}{
		"valid": {
			bin:      model.NewBinary("mockproj"),
			expected: true,
		},
		"invalid-empty-name": {
			bin:      model.NewBinary("@v1.2.3"),
			expected: false,
		},
		"invalid-version": {
			bin:      model.NewBinary("mockproj@1.2.3"),
			expected: false,
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			result := tc.bin.IsValid()
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestBinary_String(t *testing.T) {
	cases := map[string]struct {
		bin      model.Binary
		expected string
	}{
		"latest": {
			bin:      model.NewBinary("mockproj"),
			expected: "mockproj",
		},
		"with-version": {
			bin:      model.NewBinary("mockproj@v1.2.3"),
			expected: "mockproj@v1.2.3",
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			result := tc.bin.String()
			assert.Equal(t, tc.expected, result)
		})
	}
}
