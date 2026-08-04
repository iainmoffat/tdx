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

	// ErrDayLocked indicates a pre-write check found the target day is locked.
	ErrDayLocked = errors.New("day is locked")

	// ErrWeekSubmitted indicates a pre-write check found the target week has
	// already been submitted for approval.
	ErrWeekSubmitted = errors.New("week already submitted")

	// ErrTimeOffIDUnknown indicates tdx could not determine the tenant's
	// time-off ItemID from the user's recent entries and no override was given.
	ErrTimeOffIDUnknown = errors.New("time-off id unknown")

	// ErrPermission indicates the API rejected the request because the
	// caller lacks the necessary role/app/approver relationship.
	ErrPermission = errors.New("permission denied")

	// ErrFanoutLimitExceeded indicates a per-user × per-week fan-out request
	// would exceed the hard caps defined in caps.go. Wrap with details about
	// which limit (weeks or users), the requested value, and the max.
	ErrFanoutLimitExceeded = errors.New("fanout_limit_exceeded")

	// ErrInvalidArtifactName indicates a template/draft/profile name failed
	// validation. Wrap with the specific reason for the user.
	ErrInvalidArtifactName = errors.New("invalid_artifact_name")
)
