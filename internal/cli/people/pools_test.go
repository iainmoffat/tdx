package people

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/iainmoffat/tdx/internal/svc/peoplesvc"
)

type stubPeoplesvc struct {
	pools []peoplesvc.ResourcePool
	err   error
}

func (s *stubPeoplesvc) SearchPools(_ context.Context, _ string) ([]peoplesvc.ResourcePool, error) {
	if s.err != nil {
		return nil, s.err
	}
	return s.pools, nil
}

func runWith(t *testing.T, svc peoplesvcAPI, args ...string) (stdout string, err error) {
	t.Helper()
	cmd := newCmdWith(svc)
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SilenceUsage = true
	cmd.SetArgs(args)
	err = cmd.Execute()
	return out.String(), err
}

func TestPoolsList_TextOutput(t *testing.T) {
	stub := &stubPeoplesvc{pools: []peoplesvc.ResourcePool{
		{ID: 46, Name: "ICT - DBP - LPS", IsActive: true, RequiresApproval: true, ManagerFullName: "Iain Moffat"},
		{ID: 2, Name: "ADI - Business", IsActive: true, RequiresApproval: false, ManagerFullName: ""},
	}}
	out, err := runWith(t, stub, "pools", "list")
	require.NoError(t, err)
	// Sorted by name: ADI first, then ICT.
	idxADI := bytesIndex(out, "ADI - Business")
	idxICT := bytesIndex(out, "ICT - DBP - LPS")
	require.True(t, idxADI > 0 && idxICT > idxADI, "expected sorted-by-name output")
	require.Contains(t, out, "Iain Moffat")
	require.Contains(t, out, "yes")
	require.Contains(t, out, "no")
}

func TestPoolsList_JSONEnvelope(t *testing.T) {
	stub := &stubPeoplesvc{pools: []peoplesvc.ResourcePool{
		{ID: 46, Name: "ICT - DBP - LPS", IsActive: true, RequiresApproval: true, ManagerUID: "u1", ManagerFullName: "Iain"},
	}}
	out, err := runWith(t, stub, "pools", "list", "--json")
	require.NoError(t, err)

	var parsed struct {
		Schema string `json:"schema"`
		Pools  []struct {
			ID   int    `json:"id"`
			Name string `json:"name"`
		} `json:"pools"`
	}
	require.NoError(t, json.Unmarshal([]byte(out), &parsed))
	require.Equal(t, "tdx.v1.resourcePoolList", parsed.Schema)
	require.Len(t, parsed.Pools, 1)
	require.Equal(t, 46, parsed.Pools[0].ID)
	require.Equal(t, "ICT - DBP - LPS", parsed.Pools[0].Name)
}

func TestPoolsList_EmptyResult(t *testing.T) {
	stub := &stubPeoplesvc{pools: nil}
	out, err := runWith(t, stub, "pools", "list")
	require.NoError(t, err)
	require.Contains(t, out, "no resource pools")
}

func TestPoolsList_PropagatesError(t *testing.T) {
	stub := &stubPeoplesvc{err: errors.New("boom")}
	_, err := runWith(t, stub, "pools", "list")
	require.Error(t, err)
	require.Contains(t, err.Error(), "boom")
}

func bytesIndex(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
