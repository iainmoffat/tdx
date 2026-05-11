package domain

// User is the identity information the TD whoami endpoint returns.
// Populated on Session and displayed by `tdx auth status`.
type User struct {
	ID             int    `json:"id,omitempty"             yaml:"id,omitempty"`
	UID            string `json:"uid,omitempty"            yaml:"uid,omitempty"`
	FullName       string `json:"fullName,omitempty"       yaml:"fullName,omitempty"`
	Email          string `json:"email,omitempty"          yaml:"email,omitempty"`
	Active         bool   `json:"active,omitempty"         yaml:"active,omitempty"`
	AccountName    string `json:"accountName,omitempty"    yaml:"accountName,omitempty"`
	ReportsToUID   string `json:"reportsToUID,omitempty"   yaml:"reportsToUID,omitempty"`
	ReportsToID    int    `json:"reportsToID,omitempty"    yaml:"reportsToID,omitempty"`
	ReportsToName  string `json:"reportsToName,omitempty"  yaml:"reportsToName,omitempty"`
	ReportsToEmail string `json:"reportsToEmail,omitempty" yaml:"reportsToEmail,omitempty"`

	ResourcePoolID   int    `json:"resourcePoolID,omitempty"   yaml:"resourcePoolID,omitempty"`
	ResourcePoolName string `json:"resourcePoolName,omitempty" yaml:"resourcePoolName,omitempty"`

	IsEmployee bool   `json:"isEmployee,omitempty" yaml:"isEmployee,omitempty"`
	Title      string `json:"title,omitempty"      yaml:"title,omitempty"`

	// WorkableHours is the user's expected weekly hours from TD. 0.0 means
	// unset/unknown; the time-report --incomplete filter falls back to a
	// global 40 default when computing per-user thresholds.
	WorkableHours float64 `json:"workableHours,omitempty" yaml:"workableHours,omitempty"`
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
