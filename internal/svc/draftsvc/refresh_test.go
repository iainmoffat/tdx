package draftsvc

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestStrategy_Validate(t *testing.T) {
	cases := []struct {
		in      Strategy
		wantErr bool
	}{
		{StrategyAbort, false},
		{StrategyOurs, false},
		{StrategyTheirs, false},
		{Strategy(""), true},
		{Strategy("merge"), true},
	}
	for _, tc := range cases {
		err := tc.in.Validate()
		if tc.wantErr {
			require.Error(t, err, "expected error for %q", tc.in)
		} else {
			require.NoError(t, err, "unexpected error for %q", tc.in)
		}
	}
}

func TestRefreshResult_ZeroValueIsAbortFalse(t *testing.T) {
	var r RefreshResult
	require.False(t, r.Aborted)
	require.Empty(t, r.Conflicts)
	require.Equal(t, 0, r.Adopted)
	require.Equal(t, 0, r.Preserved)
	require.Equal(t, 0, r.Resolved)
	require.Equal(t, 0, r.ResolvedByStrategy)
}
