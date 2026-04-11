package domain

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestProfile_Validate_AcceptsValidProfile(t *testing.T) {
	p := Profile{
		Name:          "default",
		TenantBaseURL: "https://ufl.teamdynamix.com/",
	}
	require.NoError(t, p.Validate())
}

func TestProfile_Validate_RejectsEmptyName(t *testing.T) {
	p := Profile{
		Name:          "",
		TenantBaseURL: "https://ufl.teamdynamix.com/",
	}
	require.ErrorIs(t, p.Validate(), ErrInvalidProfile)
}

func TestProfile_Validate_RejectsMissingURL(t *testing.T) {
	p := Profile{Name: "default"}
	require.ErrorIs(t, p.Validate(), ErrInvalidProfile)
}

func TestProfile_Validate_RejectsNonHTTPSURL(t *testing.T) {
	p := Profile{
		Name:          "default",
		TenantBaseURL: "http://ufl.teamdynamix.com/",
	}
	require.ErrorIs(t, p.Validate(), ErrInvalidProfile)
}

func TestProfile_Validate_RejectsNameWithSlash(t *testing.T) {
	p := Profile{
		Name:          "bad/name",
		TenantBaseURL: "https://ufl.teamdynamix.com/",
	}
	require.ErrorIs(t, p.Validate(), ErrInvalidProfile)
}

func TestProfile_Validate_RejectsUnparseableURL(t *testing.T) {
	p := Profile{
		Name:          "default",
		TenantBaseURL: "https://exa mple.com/",
	}
	require.ErrorIs(t, p.Validate(), ErrInvalidProfile)
}

func TestProfile_Validate_RejectsURLWithNoHost(t *testing.T) {
	p := Profile{
		Name:          "default",
		TenantBaseURL: "https://",
	}
	require.ErrorIs(t, p.Validate(), ErrInvalidProfile)
}
