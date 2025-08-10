package internal_test

import (
	"errors"
	"testing"

	"github.com/brunoribeiro127/gobin/internal"
	"github.com/stretchr/testify/assert"
)

func TestKind_IsValid(t *testing.T) {
	cases := map[string]struct {
		kind     internal.Kind
		expected bool
	}{
		"latest": {
			kind:     internal.KindLatest,
			expected: true,
		},
		"major": {
			kind:     internal.KindMajor,
			expected: true,
		},
		"minor": {
			kind:     internal.KindMinor,
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
		kind     internal.Kind
		expected string
	}{
		"latest": {
			kind:     internal.KindLatest,
			expected: "latest",
		},
		"major": {
			kind:     internal.KindMajor,
			expected: "major",
		},
		"minor": {
			kind:     internal.KindMinor,
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
		expected internal.Kind
		err      error
	}{
		"latest": {
			kind:     "latest",
			expected: internal.KindLatest,
		},
		"major": {
			kind:     "major",
			expected: internal.KindMajor,
		},
		"minor": {
			kind:     "minor",
			expected: internal.KindMinor,
		},
		"invalid": {
			kind: "invalid",
			err:  errors.New(`invalid kind "invalid", allowed values are: [latest major minor]`),
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			kind := internal.Kind("")
			err := kind.Set(tc.kind)
			assert.Equal(t, tc.expected, kind)
			assert.Equal(t, tc.err, err)
		})
	}
}

func TestKind_Type(t *testing.T) {
	kind := internal.Kind("")
	assert.Equal(t, "kind", kind.Type())
}
