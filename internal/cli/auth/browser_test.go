package auth

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestOpenBrowser_DefaultIsNonNil(t *testing.T) {
	require.NotNil(t, openBrowser, "package-level openBrowser must be initialized")
}

func TestOpenBrowser_Overridable(t *testing.T) {
	// Save and restore the package-level var so tests don't bleed.
	original := openBrowser
	defer func() { openBrowser = original }()

	var called string
	openBrowser = func(url string) error {
		called = url
		return nil
	}
	require.NoError(t, openBrowser("https://example.com/test"))
	require.Equal(t, "https://example.com/test", called)
}
