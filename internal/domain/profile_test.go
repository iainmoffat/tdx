package domain

import (
	"testing"

	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func TestProfile_Validate_AcceptsValidProfile(t *testing.T) {
	p := Profile{
		Name:          "default",
		TenantBaseURL: "https://demotemplate.teamdynamix.com/",
	}
	require.NoError(t, p.Validate())
}

func TestProfile_Validate_RejectsEmptyName(t *testing.T) {
	p := Profile{
		Name:          "",
		TenantBaseURL: "https://demotemplate.teamdynamix.com/",
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
		TenantBaseURL: "http://demotemplate.teamdynamix.com/",
	}
	require.ErrorIs(t, p.Validate(), ErrInvalidProfile)
}

func TestProfile_Validate_AllowsHTTPForLoopback(t *testing.T) {
	cases := []string{
		"http://localhost/",
		"http://127.0.0.1:8080/",
		"http://[::1]:9090/",
	}
	for _, url := range cases {
		t.Run(url, func(t *testing.T) {
			p := Profile{Name: "local", TenantBaseURL: url}
			require.NoError(t, p.Validate())
		})
	}
}

func TestProfile_Validate_RejectsNameWithSlash(t *testing.T) {
	p := Profile{
		Name:          "bad/name",
		TenantBaseURL: "https://demotemplate.teamdynamix.com/",
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

func TestProfileTicketAppIDRoundTrip(t *testing.T) {
	// Construct a Profile with TicketAppID set; round-trip through yaml.Marshal/Unmarshal;
	// verify the field survives.
	original := Profile{
		Name:          "myprofile",
		TenantBaseURL: "https://example.teamdynamix.com/",
		TicketAppID:   42,
	}

	// Marshal to YAML
	data, err := yaml.Marshal(original)
	require.NoError(t, err)

	// Unmarshal back
	var restored Profile
	err = yaml.Unmarshal(data, &restored)
	require.NoError(t, err)

	// Verify all fields match, including TicketAppID
	require.Equal(t, original.Name, restored.Name)
	require.Equal(t, original.TenantBaseURL, restored.TenantBaseURL)
	require.Equal(t, original.TicketAppID, restored.TicketAppID)
	require.Equal(t, 42, restored.TicketAppID)
}

func TestProfileTicketAppIDOmittedWhenZero(t *testing.T) {
	// Construct a Profile WITHOUT TicketAppID (zero value); marshal to yaml;
	// verify the output does NOT contain the string "ticketAppID" (omitempty should drop it).
	p := Profile{
		Name:          "myprofile",
		TenantBaseURL: "https://example.teamdynamix.com/",
		// TicketAppID is implicitly 0
	}

	data, err := yaml.Marshal(p)
	require.NoError(t, err)

	// Verify the YAML output does not contain "ticketAppID"
	yamlStr := string(data)
	require.NotContains(t, yamlStr, "ticketAppID")
}
