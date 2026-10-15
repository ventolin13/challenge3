package request_test

import (
	"testing"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"

	"github.com/ventolin13/challenge3/internal/handler/request"
)

func TestParseAmount(t *testing.T) {
	t.Parallel()
	cases := []struct {
		raw  string
		ok   bool
		want string
	}{
		{raw: "1", ok: true, want: "1.00"},
		{raw: "1.2", ok: true, want: "1.20"},
		{raw: "1.20", ok: true, want: "1.20"},
		{raw: "100", ok: true, want: "100.00"},
		{raw: "0", ok: false, want: ""},
		{raw: "-1", ok: false, want: ""},
		{raw: "1.234", ok: false, want: ""},
		{raw: "1e2", ok: false, want: ""},
		{raw: "", ok: false, want: ""},
	}
	for _, tc := range cases {
		t.Run(tc.raw, func(t *testing.T) {
			t.Parallel()
			got, ok := request.ParseAmount(tc.raw)
			require.Equal(t, tc.ok, ok)
			if !tc.ok {
				return
			}
			require.True(t, got.Equal(decimal.RequireFromString(tc.want)))
		})
	}
}
