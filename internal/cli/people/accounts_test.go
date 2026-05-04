package people

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/iainmoffat/tdx/internal/svc/peoplesvc"
)

func accountsFixture() []peoplesvc.Account {
	return []peoplesvc.Account{
		{ID: 866, Name: "ICT - Infrastructure", IsActive: true, ManagerFullName: "Iain Moffat"},
		{ID: 1, Name: "Board of Trustees", IsActive: true},
		{ID: 47, Name: "ADI - Business Analysis", IsActive: false},
	}
}

func TestAccountsList_TextSortedByName(t *testing.T) {
	stub := &stubPeoplesvc{accounts: accountsFixture()}
	var out bytes.Buffer
	require.NoError(t, runAccountsList(context.Background(), &out, stub, "default", false))
	s := out.String()
	idxADI := strings.Index(s, "ADI - Business Analysis")
	idxBOT := strings.Index(s, "Board of Trustees")
	idxICT := strings.Index(s, "ICT - Infrastructure")
	require.True(t, idxADI >= 0 && idxBOT > idxADI && idxICT > idxBOT, "expected sorted-by-name output")
	require.Contains(t, s, "yes")
	require.Contains(t, s, "no", "inactive accounts render 'no'")
}

func TestAccountsList_JSONEnvelope(t *testing.T) {
	stub := &stubPeoplesvc{accounts: accountsFixture()}
	var out bytes.Buffer
	require.NoError(t, runAccountsList(context.Background(), &out, stub, "default", true))
	var got map[string]any
	require.NoError(t, json.Unmarshal(out.Bytes(), &got))
	require.Equal(t, "tdx.v1.accountList", got["schema"])
	arr, ok := got["accounts"].([]any)
	require.True(t, ok)
	require.Len(t, arr, 3)
}

func TestAccountsList_Empty(t *testing.T) {
	stub := &stubPeoplesvc{accounts: nil}
	var out bytes.Buffer
	require.NoError(t, runAccountsList(context.Background(), &out, stub, "default", false))
	require.Contains(t, out.String(), "no accounts")
}

func TestAccountsList_PropagatesError(t *testing.T) {
	stub := &stubPeoplesvc{accountsErr: errors.New("boom")}
	var out bytes.Buffer
	err := runAccountsList(context.Background(), &out, stub, "default", false)
	require.Error(t, err)
	require.Contains(t, err.Error(), "boom")
}

func TestAccountsCmd_FlagsRegistered(t *testing.T) {
	cmd := newCmdWith(nil)
	listCmd, _, err := cmd.Find([]string{"accounts", "list"})
	require.NoError(t, err)
	require.NotNil(t, listCmd.Flags().Lookup("json"))
	require.NotNil(t, listCmd.Flags().Lookup("profile"))
}
