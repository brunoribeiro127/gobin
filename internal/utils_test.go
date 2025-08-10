package internal_test

import (
	"errors"
	"testing"

	"github.com/brunoribeiro127/gobin/internal"
	"github.com/stretchr/testify/assert"
)

func TestMust(t *testing.T) {
	cases := map[string]struct {
		value any
		err   error
	}{
		"success": {
			value: "test",
		},
		"error": {
			value: nil,
			err:   errors.New("expected error"),
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			if tc.err != nil {
				assert.Panics(t, func() {
					internal.Must(tc.value, tc.err)
				})
			} else {
				assert.Equal(t, tc.value, internal.Must(tc.value, tc.err))
			}
		})
	}
}
