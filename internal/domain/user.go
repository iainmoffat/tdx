package domain

// User is the identity information the TD whoami endpoint returns.
// Populated on Session and displayed by `tdx auth status`.
type User struct {
	ID       int    `json:"id,omitempty" yaml:"id,omitempty"`
	UID      string `json:"uid,omitempty" yaml:"uid,omitempty"`
	FullName string `json:"fullName,omitempty" yaml:"fullName,omitempty"`
	Email    string `json:"email,omitempty" yaml:"email,omitempty"`
}

// DisplayName returns the most specific non-empty name available.
// Precedence: FullName > Email > UID > "(unknown user)".
func (u User) DisplayName() string {
	if u.FullName != "" {
		return u.FullName
	}
	if u.Email != "" {
		return u.Email
	}
	if u.UID != "" {
		return u.UID
	}
	return "(unknown user)"
}

// IsZero reports whether the user has no identifying fields.
func (u User) IsZero() bool {
	return u == User{}
}
