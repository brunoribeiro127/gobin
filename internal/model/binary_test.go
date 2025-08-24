package model_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/brunoribeiro127/gobin/internal/model"
)

func TestNewBinaryFromString(t *testing.T) {
	cases := map[string]struct {
		bin      string
		expected model.Binary
	}{
		"name-only": {
			bin: "mockproj",
			expected: model.NewBinary(
				"mockproj",
				model.NewLatestVersion(),
				"",
			),
		},
		"with-version": {
			bin: "mockproj@v1.2.3",
			expected: model.NewBinary(
				"mockproj",
				model.NewVersion("v1.2.3"),
				"",
			),
		},
		"with-extension": {
			bin: "mockproj.exe",
			expected: model.NewBinary(
				"mockproj",
				model.NewLatestVersion(),
				".exe",
			),
		},
		"with-version-and-extension": {
			bin: "mockproj@v1.2.3.exe",
			expected: model.NewBinary(
				"mockproj",
				model.NewVersion("v1.2.3"),
				".exe",
			),
		},
		"with-major-pinned-version": {
			bin: "mockproj-v1",
			expected: model.NewBinary(
				"mockproj-v1",
				model.NewLatestVersion(),
				"",
			),
		},
		"with-major-pinned-version-and-extension": {
			bin: "mockproj-v1.exe",
			expected: model.NewBinary(
				"mockproj-v1",
				model.NewLatestVersion(),
				".exe",
			),
		},
		"with-minor-pinned-version": {
			bin: "mockproj-v1.2",
			expected: model.NewBinary(
				"mockproj-v1.2",
				model.NewLatestVersion(),
				"",
			),
		},
		"with-minor-pinned-version-and-extension": {
			bin: "mockproj-v1.2.exe",
			expected: model.NewBinary(
				"mockproj-v1.2",
				model.NewLatestVersion(),
				".exe",
			),
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			result := model.NewBinaryFromString(tc.bin)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestBinary_GetPinKind(t *testing.T) {
	cases := map[string]struct {
		bin      model.Binary
		expected model.Kind
	}{
		"latest": {
			bin:      model.NewBinaryFromString("mockproj"),
			expected: model.KindLatest,
		},
		"latest-multiple-parts": {
			bin:      model.NewBinaryFromString("mockproj-test"),
			expected: model.KindLatest,
		},
		"major-pinned-version": {
			bin:      model.NewBinaryFromString("mockproj-v1"),
			expected: model.KindMajor,
		},
		"major-pinned-version-multiple-parts": {
			bin:      model.NewBinaryFromString("mockproj-test-v1"),
			expected: model.KindMajor,
		},
		"minor-pinned-version": {
			bin:      model.NewBinaryFromString("mockproj-v1.2"),
			expected: model.KindMinor,
		},
		"minor-pinned-version-multiple-parts": {
			bin:      model.NewBinaryFromString("mockproj-test-v1.2"),
			expected: model.KindMinor,
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			result := tc.bin.GetPinKind()
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestBinary_GetPinnedVersion(t *testing.T) {
	cases := map[string]struct {
		bin      model.Binary
		expected model.Version
	}{
		"latest": {
			bin:      model.NewBinaryFromString("mockproj"),
			expected: model.NewLatestVersion(),
		},
		"latest-multiple-parts": {
			bin:      model.NewBinaryFromString("mockproj-test"),
			expected: model.NewLatestVersion(),
		},
		"with-major-pinned-version": {
			bin:      model.NewBinaryFromString("mockproj-v1"),
			expected: model.NewVersion("v1"),
		},
		"with-major-pinned-version-multiple-parts": {
			bin:      model.NewBinaryFromString("mockproj-test-v1"),
			expected: model.NewVersion("v1"),
		},
		"with-minor-pinned-version": {
			bin:      model.NewBinaryFromString("mockproj-v1.2"),
			expected: model.NewVersion("v1.2"),
		},
		"with-minor-pinned-version-multiple-parts": {
			bin:      model.NewBinaryFromString("mockproj-test-v1.2"),
			expected: model.NewVersion("v1.2"),
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			result := tc.bin.GetPinnedVersion()
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestBinary_GetTargetBinName(t *testing.T) {
	cases := map[string]struct {
		bin      model.Binary
		kind     model.Kind
		expected string
	}{
		"latest": {
			bin:      model.NewBinary("mockproj", model.NewVersion("v1.2.3"), ""),
			kind:     model.KindLatest,
			expected: "mockproj",
		},
		"latest-with-extension": {
			bin:      model.NewBinary("mockproj", model.NewVersion("v1.2.3"), ".exe"),
			kind:     model.KindLatest,
			expected: "mockproj.exe",
		},
		"major": {
			bin:      model.NewBinary("mockproj", model.NewVersion("v1.2.3"), ""),
			kind:     model.KindMajor,
			expected: "mockproj-v1",
		},
		"major-with-extension": {
			bin:      model.NewBinary("mockproj", model.NewVersion("v1.2.3"), ".exe"),
			kind:     model.KindMajor,
			expected: "mockproj-v1.exe",
		},
		"minor": {
			bin:      model.NewBinary("mockproj", model.NewVersion("v1.2.3"), ""),
			kind:     model.KindMinor,
			expected: "mockproj-v1.2",
		},
		"minor-with-extension": {
			bin:      model.NewBinary("mockproj", model.NewVersion("v1.2.3"), ".exe"),
			kind:     model.KindMinor,
			expected: "mockproj-v1.2.exe",
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			result := tc.bin.GetTargetBinName(tc.kind)
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
			bin:      model.NewBinaryFromString("mockproj"),
			other:    model.NewBinaryFromString("mockproj"),
			expected: true,
		},
		"same-binary-version": {
			bin:      model.NewBinaryFromString("mockproj@v1.2.3"),
			other:    model.NewBinaryFromString("mockproj@v1.2.3"),
			expected: true,
		},
		"part-of-version-latest": {
			bin:      model.NewBinaryFromString("mockproj@v1.2.3"),
			other:    model.NewBinaryFromString("mockproj@latest"),
			expected: true,
		},
		"part-of-version-major": {
			bin:      model.NewBinaryFromString("mockproj@v1.2.3"),
			other:    model.NewBinaryFromString("mockproj@v1"),
			expected: true,
		},
		"part-of-version-minor": {
			bin:      model.NewBinaryFromString("mockproj@v1.2.3"),
			other:    model.NewBinaryFromString("mockproj@v1.2"),
			expected: true,
		},
		"different-binary-name": {
			bin:      model.NewBinaryFromString("mockproj"),
			other:    model.NewBinaryFromString("mockproj2"),
			expected: false,
		},
		"different-binary-version": {
			bin:      model.NewBinaryFromString("mockproj@v1.2.3"),
			other:    model.NewBinaryFromString("mockproj@v1.2.4"),
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
			bin:      model.NewBinaryFromString("mockproj"),
			expected: true,
		},
		"invalid-empty-name": {
			bin:      model.NewBinaryFromString("@v1.2.3"),
			expected: false,
		},
		"invalid-version": {
			bin:      model.NewBinaryFromString("mockproj@1.2.3"),
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
			bin:      model.NewBinaryFromString("mockproj"),
			expected: "mockproj",
		},
		"with-version": {
			bin:      model.NewBinaryFromString("mockproj@v1.2.3"),
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
