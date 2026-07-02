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
