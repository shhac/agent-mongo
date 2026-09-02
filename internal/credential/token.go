package credential

import (
	"encoding/base64"
	"encoding/json"
	"os"
	"strconv"
	"strings"
	"time"
)

// now is the clock. A package variable so an expiry test can pin an instant
// instead of sleeping or depending on wall time.
//
// Tests that swap it must not run in parallel. There is one mutator
// (fixedClock, in the tests) and that is deliberate: a mutex here would buy
// parallelism this package does not use and hide the constraint.
var now = time.Now

// ParseToken checks a raw token is a well-formed JWT that has not expired, and
// reports the expiry it found.
//
// source names where the token came from, for the error messages — a file path,
// or a credential alias for a token out of the keychain. The parsing is
// deliberately separate from fetching it: the same checks apply to a token read
// off disk and one read out of a stored session.
func ParseToken(source, raw string) (token string, expiry time.Time, err error) {
	// Trimmed because the usual ways this value is produced leave a trailing
	// newline, and the driver sends it verbatim.
	token = strings.TrimSpace(raw)

	segments := strings.Split(token, ".")
	if len(segments) != 3 || segments[0] == "" || segments[1] == "" || segments[2] == "" {
		return "", time.Time{}, MalformedTokenError(source)
	}

	expiry, ok := tokenExpiry(segments[1])
	if ok && !now().Before(expiry) {
		return "", expiry, ExpiredTokenError(source, expiry)
	}
	return token, expiry, nil
}

// ReadTokenFile reads a JWT from disk and checks it.
//
// The expiry check is agent-mongo's own. A server rejects a stale token with a
// generic authentication failure, which tells the caller nothing about what to
// do; knowing the file is out of date says exactly what has to happen next.
func ReadTokenFile(path string) (string, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return "", TokenFileError(path, err)
	}
	token, _, err := ParseToken("the token file "+strconv.Quote(path), string(raw))
	return token, err
}

// tokenExpiry reads the "exp" claim.
//
// A token whose payload will not decode, or carries no exp, is not rejected:
// the claim is optional and the server is the authority either way — this check
// exists to give a better error, not to be one.
//
// exp is read as a json.Number rather than an int64 because RFC 7519 says
// NumericDate, not integer: an issuer emitting 1756814400.0 or 1.7568144e9 is
// within spec, and decoding straight to int64 fails on both, which silently
// turned the expiry check off for exactly the issuers it was written for.
func tokenExpiry(payload string) (time.Time, bool) {
	decoded, err := decodeSegment(payload)
	if err != nil {
		return time.Time{}, false
	}
	var claims struct {
		Exp json.Number `json:"exp"`
	}
	if err := json.Unmarshal(decoded, &claims); err != nil || claims.Exp == "" {
		return time.Time{}, false
	}
	seconds, err := claims.Exp.Float64()
	if err != nil || seconds == 0 {
		return time.Time{}, false
	}
	return time.Unix(int64(seconds), 0), true
}

// decodeSegment accepts the unpadded base64url a JWT is specified to use, and
// falls back to the padded form rather than treating a padded token as having
// no expiry at all.
func decodeSegment(segment string) ([]byte, error) {
	if decoded, err := base64.RawURLEncoding.DecodeString(segment); err == nil {
		return decoded, nil
	}
	return base64.URLEncoding.DecodeString(segment)
}
