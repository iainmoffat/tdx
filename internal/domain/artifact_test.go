package domain

import (
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestValidateArtifactName_Accepts(t *testing.T) {
	cases := []string{
		"default",
		"my-week",
		"my-week2",
		"My_Week.draft",
		"a",
		"COM10",   // not COM1-9
		"LPT10",   // not LPT1-9
		"CONsole", // not exact CON
		strings.Repeat("a", 64),
	}
	for _, name := range cases {
		t.Run(name, func(t *testing.T) {
			require.NoError(t, ValidateArtifactName(name))
		})
	}
}

func TestValidateArtifactName_Rejects(t *testing.T) {
	cases := []struct {
		name   string
		reason string
	}{
		{"", "required"},
		{"..", "may not start with"},
		{".", "may not start with"},
		{".hidden", "may not start with"},
		{"-flag", "may not start with"},
		{"../../credentials", "invalid character"},
		{"/etc/passwd", "invalid character"},
		{"foo/bar", "invalid character"},
		{"foo\\bar", "invalid character"},
		{"foo bar", "invalid character"},
		{"foo\tbar", "invalid character"},
		{"naïve", "invalid character"},
		{"foo\x00bar", "invalid character"},
		{strings.Repeat("a", 65), "exceeds 64 characters"},
		{"CON", "reserved name"},
		{"con", "reserved name"},
		{"COM1", "reserved name"},
		{"LPT9", "reserved name"},
		{"NUL", "reserved name"},
		{"PRN", "reserved name"},
		{"AUX", "reserved name"},
		{"CON.txt", "reserved name"},
		{"nul.foo", "reserved name"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := ValidateArtifactName(c.name)
			require.Error(t, err)
			require.True(t, errors.Is(err, ErrInvalidArtifactName), "must wrap ErrInvalidArtifactName, got: %v", err)
			require.Contains(t, err.Error(), c.reason, "expected error to contain %q, got: %v", c.reason, err)
		})
	}
}
