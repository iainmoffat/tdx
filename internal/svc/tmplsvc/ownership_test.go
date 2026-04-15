package tmplsvc

import (
	"testing"

	"github.com/iainmoffat/tdx/internal/domain"
	"github.com/stretchr/testify/require"
)

func TestMarkerChecker_Mark(t *testing.T) {
	c := &MarkerChecker{}
	require.Equal(t, "original desc [tdx:default-week#row-01]", c.Mark("original desc", "default-week", "row-01"))
}

func TestMarkerChecker_Mark_EmptyDescription(t *testing.T) {
	c := &MarkerChecker{}
	require.Equal(t, "[tdx:default-week#row-01]", c.Mark("", "default-week", "row-01"))
}

func TestMarkerChecker_IsOwned(t *testing.T) {
	c := &MarkerChecker{}
	entry := domain.TimeEntry{Description: "work [tdx:default-week#row-01]"}
	require.True(t, c.IsOwned(entry, "default-week", "row-01"))
	require.False(t, c.IsOwned(entry, "other-template", "row-01"))
	require.False(t, c.IsOwned(entry, "default-week", "row-99"))
}

func TestMarkerChecker_IsOwned_NoMarker(t *testing.T) {
	c := &MarkerChecker{}
	entry := domain.TimeEntry{Description: "just a description"}
	require.False(t, c.IsOwned(entry, "default-week", "row-01"))
}

func TestMarkerChecker_Unmark(t *testing.T) {
	c := &MarkerChecker{}
	require.Equal(t, "original desc", c.Unmark("original desc [tdx:default-week#row-01]"))
	require.Equal(t, "", c.Unmark("[tdx:default-week#row-01]"))
	require.Equal(t, "no marker here", c.Unmark("no marker here"))
}

func TestMarkerChecker_RoundTrip(t *testing.T) {
	c := &MarkerChecker{}
	original := "my work description"
	marked := c.Mark(original, "test-tmpl", "row-03")
	require.Contains(t, marked, "[tdx:test-tmpl#row-03]")

	entry := domain.TimeEntry{Description: marked}
	require.True(t, c.IsOwned(entry, "test-tmpl", "row-03"))

	unmarked := c.Unmark(marked)
	require.Equal(t, original, unmarked)
}

func TestJournalChecker_Noop(t *testing.T) {
	c := &JournalChecker{}
	entry := domain.TimeEntry{Description: "anything"}
	require.False(t, c.IsOwned(entry, "tmpl", "row"))
	require.Equal(t, "test desc", c.Mark("test desc", "tmpl", "row"))
	require.Equal(t, "test desc", c.Unmark("test desc"))
}
