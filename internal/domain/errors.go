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
)
