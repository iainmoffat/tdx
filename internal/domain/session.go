package domain

// Session is an authenticated pairing of a Profile and its bearer token.
// Phase 1 does not track expiry or identity; those arrive in later phases.
type Session struct {
	Profile Profile
	Token   string
}

// HasToken returns true if the session carries a non-empty token.
func (s Session) HasToken() bool {
	return s.Token != ""
}
