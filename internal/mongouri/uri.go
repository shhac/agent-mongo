// Package mongouri parses MongoDB connection strings with plain string
// handling — no driver dependency — so config/display layers can extract and
// redact URI credentials without linking the driver.
package mongouri

import (
	"net/url"
	"strings"
)

// ParseDBFromURI extracts the database name from a MongoDB connection string's
// path segment, or "" when absent/unparseable.
func ParseDBFromURI(uri string) string {
	u, err := url.Parse(uri)
	if err != nil {
		return ""
	}
	return strings.TrimPrefix(u.Path, "/")
}

// ParseAuthSourceFromURI returns the connection string's authSource option, or
// "" when absent.
func ParseAuthSourceFromURI(uri string) string { return uriOption(uri, "authsource") }

// ParseAuthMechanismFromURI returns the connection string's authMechanism
// option, or "" when absent.
func ParseAuthMechanismFromURI(uri string) string { return uriOption(uri, "authmechanism") }

// uriOption reads one option out of a connection string's query.
//
// The key is matched case-insensitively because the driver treats URI option
// names that way: a connection string written "?authsource=admin"
// authenticates against the same database as "?authSource=admin".
func uriOption(uri, name string) string {
	_, query, found := strings.Cut(uri, "?")
	if !found {
		return ""
	}
	// A fragment is not meaningful in a connection string, but trim one anyway
	// so it cannot end up inside the returned value.
	query, _, _ = strings.Cut(query, "#")
	for _, pair := range strings.Split(query, "&") {
		key, value, _ := strings.Cut(pair, "=")
		if strings.EqualFold(key, name) {
			return unescape(value)
		}
	}
	return ""
}

// userinfoParts is one parse of a connection string around its userinfo.
type userinfoParts struct {
	prefix  string // scheme plus "://"
	user    string // raw (still percent-encoded) username
	pass    string // raw password; meaningful only when hasPass
	hasPass bool   // userinfo contained a ":"; pass may still be empty
	rest    string // host onward, after the "@"
}

// splitUserinfo separates a connection string around its userinfo section.
// Parsed by hand because url.Parse rejects multi-host URIs
// (mongodb://a:1,b:2/db). The last "@" before the path delimits the userinfo,
// matching driver behaviour for passwords containing an unescaped "@".
func splitUserinfo(uri string) (userinfoParts, bool) {
	schemeEnd := strings.Index(uri, "://")
	if schemeEnd < 0 {
		return userinfoParts{}, false
	}
	tail := uri[schemeEnd+3:]
	authority := tail
	if end := strings.IndexAny(tail, "/?"); end >= 0 {
		authority = tail[:end]
	}
	at := strings.LastIndex(authority, "@")
	if at < 0 {
		return userinfoParts{}, false
	}
	user, pass, hasPass := strings.Cut(authority[:at], ":")
	return userinfoParts{
		prefix:  uri[:schemeEnd+3],
		user:    user,
		pass:    pass,
		hasPass: hasPass,
		rest:    tail[at+1:],
	}, true
}

// SplitURICredentials extracts a username/password embedded in a connection
// string's userinfo, percent-decoded, along with the URI with the userinfo
// removed. found is false when the URI carries no password (username-only
// userinfo, e.g. X.509 auth, is left alone). An empty password ("user:@host")
// is deliberately not extractable even though RedactURI masks it — display
// errs on the safe side.
func SplitURICredentials(uri string) (username, password, stripped string, found bool) {
	p, ok := splitUserinfo(uri)
	if !ok || !p.hasPass || p.pass == "" {
		return "", "", uri, false
	}
	return unescape(p.user), unescape(p.pass), p.prefix + p.rest, true
}

// RedactURI masks the password in a connection string's userinfo for display.
func RedactURI(uri string) string {
	p, ok := splitUserinfo(uri)
	if !ok || !p.hasPass {
		return uri
	}
	return p.prefix + p.user + ":***@" + p.rest
}

func unescape(s string) string {
	decoded, err := url.PathUnescape(s)
	if err != nil {
		return s
	}
	return decoded
}

// ParseHostFromURI returns the first host in a connection string, without its
// port, or "" when unparseable. Hand-parsed for the same reason splitUserinfo
// is: url.Parse cannot cope with a multi-host URI, and the first host is enough
// to decide whether a deployment is one a credential may authenticate against.
func ParseHostFromURI(uri string) string {
	schemeEnd := strings.Index(uri, "://")
	if schemeEnd < 0 {
		return ""
	}
	authority := uri[schemeEnd+3:]
	if end := strings.IndexAny(authority, "/?"); end >= 0 {
		authority = authority[:end]
	}
	if at := strings.LastIndex(authority, "@"); at >= 0 {
		authority = authority[at+1:]
	}
	host, _, _ := strings.Cut(authority, ",")

	// An IPv6 literal is bracketed, and its own colons must not be read as a
	// port separator.
	if strings.HasPrefix(host, "[") {
		if literal, _, ok := strings.Cut(host[1:], "]"); ok {
			return literal
		}
	}
	host, _, _ = strings.Cut(host, ":")
	return host
}

// IsTLS reports whether a connection string will use TLS: mongodb+srv:// always
// does, and any other URI only when it says so explicitly.
func IsTLS(uri string) bool {
	if strings.HasPrefix(uri, "mongodb+srv://") {
		// srv implies TLS unless the URI turns it off.
		return !isFalse(uriOption(uri, "tls")) && !isFalse(uriOption(uri, "ssl"))
	}
	return isTrue(uriOption(uri, "tls")) || isTrue(uriOption(uri, "ssl"))
}

func isTrue(v string) bool  { return strings.EqualFold(v, "true") }
func isFalse(v string) bool { return strings.EqualFold(v, "false") }
