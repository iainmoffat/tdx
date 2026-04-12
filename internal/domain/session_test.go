package domain

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSession_HasToken(t *testing.T) {
	cases := []struct {
		name    string
		session Session
		want    bool
	}{
		{
			name:    "empty session",
			session: Session{},
			want:    false,
		},
		{
			name:    "session with token",
			session: Session{Token: "abc"},
			want:    true,
		},
		{
			name: "session with profile and token",
			session: Session{
				Profile: Profile{Name: "default", TenantBaseURL: "https://ufl.teamdynamix.com/"},
				Token:   "xyz",
			},
			want: true,
		},
		{
			name: "session with profile but no token",
			session: Session{
				Profile: Profile{Name: "default", TenantBaseURL: "https://ufl.teamdynamix.com/"},
			},
			want: false,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.want, tc.session.HasToken())
		})
	}
}
