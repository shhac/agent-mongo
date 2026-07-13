package mongo

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

// splitUserinfo separates a connection string around its userinfo section.
// Parsed by hand because url.Parse rejects multi-host URIs
// (mongodb://a:1,b:2/db). The last "@" before the path delimits the userinfo,
// matching driver behaviour for passwords containing an unescaped "@".
func splitUserinfo(uri string) (prefix, userinfo, hostAndAfter string, ok bool) {
	schemeEnd := strings.Index(uri, "://")
	if schemeEnd < 0 {
		return "", "", "", false
	}
	tail := uri[schemeEnd+3:]
	authority := tail
	if end := strings.IndexAny(tail, "/?"); end >= 0 {
		authority = tail[:end]
	}
	at := strings.LastIndex(authority, "@")
	if at < 0 {
		return "", "", "", false
	}
	return uri[:schemeEnd+3], authority[:at], tail[at+1:], true
}

// SplitURICredentials extracts a username/password embedded in a connection
// string's userinfo, percent-decoded, along with the URI with the userinfo
// removed. found is false when the URI carries no password (username-only
// userinfo, e.g. X.509 auth, is left alone).
func SplitURICredentials(uri string) (username, password, stripped string, found bool) {
	prefix, userinfo, rest, ok := splitUserinfo(uri)
	if !ok {
		return "", "", uri, false
	}
	rawUser, rawPass, hasPass := strings.Cut(userinfo, ":")
	if !hasPass || rawPass == "" {
		return "", "", uri, false
	}
	return unescape(rawUser), unescape(rawPass), prefix + rest, true
}

// RedactURI masks the password in a connection string's userinfo for display.
func RedactURI(uri string) string {
	prefix, userinfo, rest, ok := splitUserinfo(uri)
	if !ok {
		return uri
	}
	user, _, hasPass := strings.Cut(userinfo, ":")
	if !hasPass {
		return uri
	}
	return prefix + user + ":***@" + rest
}

func unescape(s string) string {
	decoded, err := url.PathUnescape(s)
	if err != nil {
		return s
	}
	return decoded
}
