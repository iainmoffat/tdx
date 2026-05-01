package authsvc

import (
	"encoding/base64"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// makeFakeJWT builds a syntactically-valid three-segment JWT with the given
// `exp` claim. Signature segment is opaque junk; we don't verify here.
func makeFakeJWT(t *testing.T, exp int64) string {
	t.Helper()
	header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"HS256","typ":"JWT"}`))
	body, err := json.Marshal(map[string]any{"exp": exp})
	require.NoError(t, err)
	payload := base64.RawURLEncoding.EncodeToString(body)
	sig := base64.RawURLEncoding.EncodeToString([]byte("not-a-real-signature"))
	return strings.Join([]string{header, payload, sig}, ".")
}

func TestParseJWTExp_DecodesExp(t *testing.T) {
	want := int64(1777664918) // 2026-05-01T19:48:38Z
	tok := makeFakeJWT(t, want)
	got, ok := parseJWTExp(tok)
	require.True(t, ok)
	require.Equal(t, time.Unix(want, 0).UTC(), got)
}

func TestParseJWTExp_NotThreeSegments(t *testing.T) {
	_, ok := parseJWTExp("not.ajwt")
	require.False(t, ok)
}

func TestParseJWTExp_BadBase64(t *testing.T) {
	_, ok := parseJWTExp("aaa.???.bbb")
	require.False(t, ok)
}

func TestParseJWTExp_NoExp(t *testing.T) {
	header := base64.RawURLEncoding.EncodeToString([]byte(`{}`))
	payload := base64.RawURLEncoding.EncodeToString([]byte(`{"sub":"x"}`))
	sig := base64.RawURLEncoding.EncodeToString([]byte("x"))
	tok := strings.Join([]string{header, payload, sig}, ".")
	_, ok := parseJWTExp(tok)
	require.False(t, ok)
}

func TestParseJWTExp_RealUFLToken(t *testing.T) {
	// Real token from the user's credentials file, captured during
	// the v0.11.0 session. exp = 1777664918 = 2026-05-01T19:48:38Z.
	tok := "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9." +
		"eyJ1bmlxdWVfbmFtZSI6IjIyNzc4NzEwQHVmaWQuaXQudWZsLmVkdSIsInRkeF9lbnRpdHkiOiIyIiwidGR4X3BhcnRpdGlvbiI6IjMwOTUiLCJuYmYiOjE3Nzc1Nzg1MTgsImV4cCI6MTc3NzY2NDkxOCwiaWF0IjoxNzc3NTc4NTE4LCJpc3MiOiJURCIsImF1ZCI6Imh0dHBzOi8vd3d3LnRlYW1keW5hbWl4LmNvbS8ifQ." +
		"ff2J91oBBhfEXDwUpisvacfOtKClmtGlUtp9w2TWGZc"
	got, ok := parseJWTExp(tok)
	require.True(t, ok)
	require.Equal(t, int64(1777664918), got.Unix())
}
