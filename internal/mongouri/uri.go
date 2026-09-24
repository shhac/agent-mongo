// Package mongouri parses MongoDB connection strings with plain string
// handling — no driver dependency — so config/display layers can extract and
// redact URI credentials without linking the driver.
package mongouri

import (
	"net/url"
	"slices"
	"strings"
)

// ParseDBFromURI extracts the database name from a MongoDB connection string's
// path segment, percent-decoded, or "" when absent/unparseable.
func ParseDBFromURI(uri string) string {
	a, ok := splitAuthority(uri)
	if !ok || !strings.HasPrefix(a.rest, "/") {
		return ""
	}
	path, _, _ := strings.Cut(a.rest[1:], "?")
	return unescape(path)
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

// authority is one parse of a connection string around its authority.
type authority struct {
	prefix      string // scheme plus "://"
	userinfo    string // raw (still percent-encoded); meaningful only when hasUserinfo
	hasUserinfo bool
	hosts       string // the comma-separated host list
	rest        string // path and query onward, from the "/" or "?"
}

// splitAuthority separates a connection string into scheme, userinfo, hosts
// and the rest. Parsed by hand because url.Parse rejects multi-host URIs
// (mongodb://a:1,b:2/db). The last "@" before the path delimits the userinfo,
// matching driver behaviour for passwords containing an unescaped "@".
func splitAuthority(uri string) (authority, bool) {
	schemeEnd := strings.Index(uri, "://")
	if schemeEnd < 0 {
		return authority{}, false
	}
	tail := uri[schemeEnd+3:]
	a := authority{prefix: uri[:schemeEnd+3], hosts: tail}
	if end := strings.IndexAny(tail, "/?"); end >= 0 {
		a.hosts, a.rest = tail[:end], tail[end:]
	}
	if at := strings.LastIndex(a.hosts, "@"); at >= 0 {
		a.userinfo, a.hasUserinfo, a.hosts = a.hosts[:at], true, a.hosts[at+1:]
	}
	return a, true
}

// userinfoParts is one parse of a connection string around its userinfo.
type userinfoParts struct {
	prefix  string // scheme plus "://"
	user    string // raw (still percent-encoded) username
	pass    string // raw password; meaningful only when hasPass
	hasPass bool   // userinfo contained a ":"; pass may still be empty
	rest    string // host onward, after the "@"
}

func splitUserinfo(uri string) (userinfoParts, bool) {
	a, ok := splitAuthority(uri)
	if !ok || !a.hasUserinfo {
		return userinfoParts{}, false
	}
	user, pass, hasPass := strings.Cut(a.userinfo, ":")
	return userinfoParts{
		prefix:  a.prefix,
		user:    user,
		pass:    pass,
		hasPass: hasPass,
		rest:    a.hosts + a.rest,
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

// ParseHostsFromURI returns every host in a connection string's seed list,
// without ports, or nil when unparseable. Every one matters to a check about
// where a credential may go: the driver connects and authenticates to each
// seed it is given, not only the first.
func ParseHostsFromURI(uri string) []string {
	a, ok := splitAuthority(uri)
	if !ok || a.hosts == "" {
		return nil
	}
	var hosts []string
	for _, seed := range strings.Split(a.hosts, ",") {
		host := hostWithoutPort(seed)
		if host == "" {
			return nil // one unreadable seed makes the whole list unknown
		}
		hosts = append(hosts, host)
	}
	return hosts
}

// HostKey is a connection string's seed list as one comparable value: sorted,
// lower-cased and comma-joined, so a session bound to a deployment matches that
// deployment however its hosts are ordered or cased, and nothing broader. ""
// when the hosts cannot be read.
func HostKey(uri string) string {
	hosts := ParseHostsFromURI(uri)
	for i, host := range hosts {
		hosts[i] = strings.ToLower(strings.TrimSuffix(host, "."))
	}
	slices.Sort(hosts)
	return strings.Join(slices.Compact(hosts), ",")
}

func hostWithoutPort(seed string) string {
	// An IPv6 literal is bracketed, and its own colons must not be read as a
	// port separator.
	if strings.HasPrefix(seed, "[") {
		literal, _, _ := strings.Cut(seed[1:], "]")
		return literal
	}
	host, _, _ := strings.Cut(seed, ":")
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
