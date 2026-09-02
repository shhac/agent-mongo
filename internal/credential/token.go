package credential

import (
	"encoding/base64"
	"encoding/json"
	"os"
	"strings"
	"time"
)

// now is the clock. A package variable so an expiry test can set a fixed
// instant instead of sleeping or depending on wall time.
var now = time.Now

// ReadTokenFile reads a JWT from disk and checks it is well-formed and has not
// already expired.
//
// The expiry check is agent-mongo's own. A server rejects a stale token with a
// generic authentication failure, which tells the caller nothing about what to
// do; knowing the file is out of date says exactly what has to happen next.
func ReadTokenFile(path string) (string, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return "", TokenFileError(path, err)
	}

	// Trimmed because the usual way this file is produced writes a trailing
	// newline, and the driver sends the value verbatim.
	token := strings.TrimSpace(string(raw))
	segments := strings.Split(token, ".")
	if len(segments) != 3 || segments[0] == "" || segments[1] == "" || segments[2] == "" {
		return "", MalformedTokenError(path)
	}

	if expiry, ok := tokenExpiry(segments[1]); ok && !now().Before(expiry) {
		return "", ExpiredTokenError(path, expiry)
	}
	return token, nil
}

// tokenExpiry reads the "exp" claim. A token whose payload will not decode, or
// carries no exp, is not rejected here: the claim is optional and the server is
// the authority either way — this check exists to give a better error, not to
// be one.
func tokenExpiry(payload string) (time.Time, bool) {
	decoded, err := base64.RawURLEncoding.DecodeString(payload)
	if err != nil {
		return time.Time{}, false
	}
	var claims struct {
		Exp int64 `json:"exp"`
	}
	if err := json.Unmarshal(decoded, &claims); err != nil || claims.Exp == 0 {
		return time.Time{}, false
	}
	return time.Unix(claims.Exp, 0), true
}
