package domain

import (
	"fmt"
	"regexp"
	"strings"
)

// artifactNamePattern is the authoritative allowlist for template/draft/profile
// names. First character must be ASCII letter, digit, or underscore. Total
// length 1–64. Subsequent characters allow `.` and `-` additionally.
var artifactNamePattern = regexp.MustCompile(`^[A-Za-z0-9_][A-Za-z0-9._-]{0,63}$`)

// windowsReservedNames lists case-insensitive base names that Windows refuses
// to open even with an extension (CON.yaml is treated as CON). Match is on the
// substring before the first `.` so CON.txt also rejects; COM10 doesn't.
var windowsReservedNames = map[string]struct{}{
	"CON": {}, "PRN": {}, "AUX": {}, "NUL": {},
	"COM1": {}, "COM2": {}, "COM3": {}, "COM4": {}, "COM5": {},
	"COM6": {}, "COM7": {}, "COM8": {}, "COM9": {},
	"LPT1": {}, "LPT2": {}, "LPT3": {}, "LPT4": {}, "LPT5": {},
	"LPT6": {}, "LPT7": {}, "LPT8": {}, "LPT9": {},
}

// isAllowedNameRune reports whether r is an ASCII rune the allowlist permits
// in a non-leading position: letters, digits, `.`, `_`, `-`.
func isAllowedNameRune(r rune) bool {
	switch {
	case r >= 'A' && r <= 'Z',
		r >= 'a' && r <= 'z',
		r >= '0' && r <= '9',
		r == '.', r == '_', r == '-':
		return true
	}
	return false
}

// ValidateArtifactName returns nil if name is a safe filesystem component for
// use as a template, draft, or profile name. See
// docs/specs/2026-05-16-artifact-name-validation.md for the rule and threat
// model. On reject, returns an error wrapping ErrInvalidArtifactName with a
// specific reason.
func ValidateArtifactName(name string) error {
	if name == "" {
		return fmt.Errorf("%w: name is required", ErrInvalidArtifactName)
	}
	if len(name) > 64 {
		return fmt.Errorf("%w: name exceeds 64 characters (got %d)", ErrInvalidArtifactName, len(name))
	}
	if !artifactNamePattern.MatchString(name) {
		// Prefer reporting a non-leading invalid character over a leading-dot/hyphen
		// reason — slashes and other unsafe characters are the security signal.
		invalidPos := -1
		var invalidRune rune
		for i, r := range name {
			if i > 0 && !isAllowedNameRune(r) {
				invalidPos = i
				invalidRune = r
				break
			}
		}
		if invalidPos >= 0 {
			return fmt.Errorf("%w: name contains invalid character %q at position %d",
				ErrInvalidArtifactName, invalidRune, invalidPos)
		}
		if name[0] == '.' || name[0] == '-' {
			return fmt.Errorf("%w: name may not start with %q", ErrInvalidArtifactName, string(name[0]))
		}
		for _, r := range name[:1] {
			return fmt.Errorf("%w: name contains invalid character %q at position %d",
				ErrInvalidArtifactName, r, 0)
		}
	}
	// Reserved-name check: match the substring before the first `.`.
	head := name
	if i := strings.IndexByte(name, '.'); i >= 0 {
		head = name[:i]
	}
	if _, reserved := windowsReservedNames[strings.ToUpper(head)]; reserved {
		return fmt.Errorf("%w: %q is a reserved name", ErrInvalidArtifactName, name)
	}
	return nil
}
