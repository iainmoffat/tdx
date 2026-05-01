package authsvc

import (
	"encoding/base64"
	"encoding/json"
	"strings"
	"time"
)

// parseJWTExp decodes the unverified payload of a JWT bearer token and
// returns the `exp` claim as a UTC time. ok=false when the token isn't
// a parseable three-segment JWT or doesn't carry a numeric `exp` claim.
//
// The token signature is NOT verified — TD signs with its own secret
// that we don't possess. This is purely informational: showing the user
// when their stored bearer expires.
func parseJWTExp(token string) (time.Time, bool) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return time.Time{}, false
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return time.Time{}, false
	}
	var claims struct {
		Exp int64 `json:"exp"`
	}
	if err := json.Unmarshal(payload, &claims); err != nil {
		return time.Time{}, false
	}
	if claims.Exp == 0 {
		return time.Time{}, false
	}
	return time.Unix(claims.Exp, 0).UTC(), true
}
