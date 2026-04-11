package domain

import (
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestErrEntryNotFound_Wrappable(t *testing.T) {
	wrapped := fmt.Errorf("lookup failed: %w", ErrEntryNotFound)
	require.ErrorIs(t, wrapped, ErrEntryNotFound)
}

func TestErrUnsupportedTargetKind_Wrappable(t *testing.T) {
	wrapped := fmt.Errorf("for target: %w", ErrUnsupportedTargetKind)
	require.ErrorIs(t, wrapped, ErrUnsupportedTargetKind)
}

func TestSentinelErrors_AreDistinct(t *testing.T) {
	// Ensure we don't accidentally alias the new errors to existing ones.
	require.False(t, errors.Is(ErrEntryNotFound, ErrUnsupportedTargetKind))
	require.False(t, errors.Is(ErrUnsupportedTargetKind, ErrEntryNotFound))
	require.False(t, errors.Is(ErrEntryNotFound, ErrProfileNotFound))
}
