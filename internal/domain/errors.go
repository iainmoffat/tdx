package domain

import "errors"

var (
	// ErrInvalidProfile indicates a profile failed structural validation.
	ErrInvalidProfile = errors.New("invalid profile")

	// ErrProfileNotFound indicates a lookup by name failed.
	ErrProfileNotFound = errors.New("profile not found")

	// ErrProfileExists indicates a name collision.
	ErrProfileExists = errors.New("profile already exists")

	// ErrNoCredentials indicates no stored token for a profile.
	ErrNoCredentials = errors.New("no credentials for profile")

	// ErrInvalidToken indicates a token failed server-side validation.
	ErrInvalidToken = errors.New("invalid token")

	// ErrEntryNotFound indicates a GET /time/{id} returned 404.
	ErrEntryNotFound = errors.New("time entry not found")

	// ErrUnsupportedTargetKind indicates a TargetKind has no component-lookup
	// endpoint, so `tdx time type for` cannot handle it.
	ErrUnsupportedTargetKind = errors.New("unsupported target kind")
)
