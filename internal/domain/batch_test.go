package domain

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestBatchResult_FullSuccess(t *testing.T) {
	r := BatchResult{Succeeded: []int{1, 2, 3}}
	require.True(t, r.FullSuccess())
	require.False(t, r.PartialSuccess())
	require.False(t, r.TotalFailure())
}

func TestBatchResult_PartialSuccess(t *testing.T) {
	r := BatchResult{
		Succeeded: []int{1},
		Failed:    []BatchFailure{{ID: 2, Message: "not found"}},
	}
	require.False(t, r.FullSuccess())
	require.True(t, r.PartialSuccess())
	require.False(t, r.TotalFailure())
}

func TestBatchResult_TotalFailure(t *testing.T) {
	r := BatchResult{
		Failed: []BatchFailure{{ID: 1, Message: "locked"}},
	}
	require.False(t, r.FullSuccess())
	require.False(t, r.PartialSuccess())
	require.True(t, r.TotalFailure())
}

func TestBatchResult_Empty(t *testing.T) {
	r := BatchResult{}
	require.False(t, r.FullSuccess())
	require.False(t, r.PartialSuccess())
	require.False(t, r.TotalFailure())
}
