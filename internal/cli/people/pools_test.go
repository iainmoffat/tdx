package people

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/iainmoffat/tdx/internal/domain"
	"github.com/iainmoffat/tdx/internal/svc/peoplesvc"
)

type stubPeoplesvc struct {
	pools          []peoplesvc.ResourcePool
	accounts       []peoplesvc.Account
	lookupHits     []domain.User
	searchUsers    []domain.User
	users          map[string]domain.User
	err            error
	accountsErr    error
	lookupErr      error
	searchUsersErr error
	getErr         error

	lastLookupQuery  string
	lastLookupMax    int
	lastSearchFilter domain.UserFilter
}

func (s *stubPeoplesvc) SearchPools(_ context.Context, _ string) ([]peoplesvc.ResourcePool, error) {
	if s.err != nil {
		return nil, s.err
	}
	return s.pools, nil
}

func (s *stubPeoplesvc) SearchAccounts(_ context.Context, _ string) ([]peoplesvc.Account, error) {
	if s.accountsErr != nil {
		return nil, s.accountsErr
	}
	return s.accounts, nil
}

func (s *stubPeoplesvc) LookupPeople(_ context.Context, _, query string, max int) ([]domain.User, error) {
	s.lastLookupQuery = query
	s.lastLookupMax = max
	if s.lookupErr != nil {
		return nil, s.lookupErr
	}
	return s.lookupHits, nil
}

func (s *stubPeoplesvc) GetUser(_ context.Context, _, uid string) (domain.User, error) {
	if s.getErr != nil {
		return domain.User{}, s.getErr
	}
	if u, ok := s.users[uid]; ok {
		return u, nil
	}
	return domain.User{}, errors.New("not found")
}

func (s *stubPeoplesvc) SearchUsers(_ context.Context, _ string, filter domain.UserFilter) ([]domain.User, error) {
	s.lastSearchFilter = filter
	if s.searchUsersErr != nil {
		return nil, s.searchUsersErr
	}
	return s.searchUsers, nil
}

func TestPoolsList_TextOutput(t *testing.T) {
	stub := &stubPeoplesvc{pools: []peoplesvc.ResourcePool{
		{ID: 46, Name: "Sample Pool - PE", IsActive: true, RequiresApproval: true, ManagerFullName: "Sample User"},
		{ID: 2, Name: "ADI - Business", IsActive: true, RequiresApproval: false, ManagerFullName: ""},
	}}
	var out bytes.Buffer
	require.NoError(t, runPoolsList(context.Background(), &out, stub, "default", false))
	s := out.String()
	idxADI := strings.Index(s, "ADI - Business")
	idxICT := strings.Index(s, "Sample Pool - PE")
	require.True(t, idxADI >= 0 && idxICT > idxADI, "expected sorted-by-name output")
	require.Contains(t, s, "Sample User")
	require.Contains(t, s, "yes")
	require.Contains(t, s, "no")
}

func TestPoolsList_JSONEnvelope(t *testing.T) {
	stub := &stubPeoplesvc{pools: []peoplesvc.ResourcePool{
		{ID: 46, Name: "Sample Pool - PE", IsActive: true, RequiresApproval: true, ManagerUID: "u1", ManagerFullName: "Iain"},
	}}
	var out bytes.Buffer
	require.NoError(t, runPoolsList(context.Background(), &out, stub, "default", true))

	var parsed struct {
		Schema string `json:"schema"`
		Pools  []struct {
			ID   int    `json:"id"`
			Name string `json:"name"`
		} `json:"pools"`
	}
	require.NoError(t, json.Unmarshal(out.Bytes(), &parsed))
	require.Equal(t, "tdx.v1.resourcePoolList", parsed.Schema)
	require.Len(t, parsed.Pools, 1)
	require.Equal(t, 46, parsed.Pools[0].ID)
	require.Equal(t, "Sample Pool - PE", parsed.Pools[0].Name)
}

func TestPoolsList_EmptyResult(t *testing.T) {
	stub := &stubPeoplesvc{pools: nil}
	var out bytes.Buffer
	require.NoError(t, runPoolsList(context.Background(), &out, stub, "default", false))
	require.Contains(t, out.String(), "no resource pools")
}

func TestPoolsList_PropagatesError(t *testing.T) {
	stub := &stubPeoplesvc{err: errors.New("boom")}
	var out bytes.Buffer
	err := runPoolsList(context.Background(), &out, stub, "default", false)
	require.Error(t, err)
	require.Contains(t, err.Error(), "boom")
}

func TestPoolsCmd_FlagsRegistered(t *testing.T) {
	cmd := newCmdWith(nil)
	listCmd, _, err := cmd.Find([]string{"pools", "list"})
	require.NoError(t, err)
	require.NotNil(t, listCmd.Flags().Lookup("json"))
	require.NotNil(t, listCmd.Flags().Lookup("profile"))
}
