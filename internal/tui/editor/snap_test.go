package editor

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSnapToHalf(t *testing.T) {
	tests := []struct {
		input    float64
		expected float64
	}{
		{0.0, 0.0},
		{0.3, 0.5},
		{0.7, 0.5},
		{0.8, 1.0},
		{1.0, 1.0},
		{1.25, 1.5},
		{1.333, 1.5},
		{2.7, 2.5},
		{2.9, 3.0},
		{8.0, 8.0},
		{24.5, 24.0},
		{-1.0, 0.0},
		{100.0, 24.0},
	}
	for _, tt := range tests {
		got := snapToHalf(tt.input)
		require.InDelta(t, tt.expected, got, 0.001, "snapToHalf(%v)", tt.input)
	}
}

func TestNudge(t *testing.T) {
	require.InDelta(t, 1.0, nudge(0.5, 1), 0.001)   // up
	require.InDelta(t, 0.0, nudge(0.5, -1), 0.001)   // down
	require.InDelta(t, 0.0, nudge(0.0, -1), 0.001)   // clamp at 0
	require.InDelta(t, 24.0, nudge(24.0, 1), 0.001)  // clamp at 24
	require.InDelta(t, 23.5, nudge(24.0, -1), 0.001) // down from max
}
