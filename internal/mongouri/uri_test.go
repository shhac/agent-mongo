package mongouri

import "testing"

func TestSplitURICredentials(t *testing.T) {
	tests := []struct {
		name     string
		uri      string
		username string
		password string
		stripped string
		found    bool
	}{
		{
			name:     "user and pass",
			uri:      "mongodb://deploy:s3cret@localhost:27017/myapp",
			username: "deploy",
			password: "s3cret",
			stripped: "mongodb://localhost:27017/myapp",
			found:    true,
		},
		{
			name:     "srv scheme",
			uri:      "mongodb+srv://deploy:s3cret@cluster.example.net/app?retryWrites=true",
			username: "deploy",
			password: "s3cret",
			stripped: "mongodb+srv://cluster.example.net/app?retryWrites=true",
			found:    true,
		},
		{
			name:     "multi-host",
			uri:      "mongodb://deploy:s3cret@host1:27017,host2:27018/app?replicaSet=rs0",
			username: "deploy",
			password: "s3cret",
			stripped: "mongodb://host1:27017,host2:27018/app?replicaSet=rs0",
			found:    true,
		},
		{
			name:     "percent-encoded",
			uri:      "mongodb://user%40corp:p%40ss%3Aword@localhost/db",
			username: "user@corp",
			password: "p@ss:word",
			stripped: "mongodb://localhost/db",
			found:    true,
		},
		{
			name:     "unescaped at in password",
			uri:      "mongodb://deploy:p@ss@localhost/db",
			username: "deploy",
			password: "p@ss",
			stripped: "mongodb://localhost/db",
			found:    true,
		},
		{
			name:     "query without path",
			uri:      "mongodb://deploy:s3cret@localhost?authSource=admin",
			username: "deploy",
			password: "s3cret",
			stripped: "mongodb://localhost?authSource=admin",
			found:    true,
		},
		{
			name:     "invalid percent escape falls back to raw",
			uri:      "mongodb://user:p%zzword@localhost/db",
			username: "user",
			password: "p%zzword",
			stripped: "mongodb://localhost/db",
			found:    true,
		},
		{
			name:     "trailing bare percent falls back to raw",
			uri:      "mongodb://u%zz:pa%@localhost/db",
			username: "u%zz",
			password: "pa%",
			stripped: "mongodb://localhost/db",
			found:    true,
		},
		{
			name:     "empty username with password",
			uri:      "mongodb://:pass@localhost/db",
			username: "",
			password: "pass",
			stripped: "mongodb://localhost/db",
			found:    true,
		},
		{
			name:     "no userinfo",
			uri:      "mongodb://localhost:27017/myapp",
			stripped: "mongodb://localhost:27017/myapp",
		},
		{
			name:     "username only",
			uri:      "mongodb://x509user@localhost/db?authMechanism=MONGODB-X509",
			stripped: "mongodb://x509user@localhost/db?authMechanism=MONGODB-X509",
		},
		{
			name:     "empty password",
			uri:      "mongodb://deploy:@localhost/db",
			stripped: "mongodb://deploy:@localhost/db",
		},
		{
			name:     "at sign in query only",
			uri:      "mongodb://localhost/db?appName=me@corp",
			stripped: "mongodb://localhost/db?appName=me@corp",
		},
		{
			name:     "not a uri",
			uri:      "localhost:27017",
			stripped: "localhost:27017",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			username, password, stripped, found := SplitURICredentials(tt.uri)
			if username != tt.username || password != tt.password ||
				stripped != tt.stripped || found != tt.found {
				t.Errorf("SplitURICredentials(%q) = (%q, %q, %q, %v), want (%q, %q, %q, %v)",
					tt.uri, username, password, stripped, found,
					tt.username, tt.password, tt.stripped, tt.found)
			}
		})
	}
}

func TestRedactURI(t *testing.T) {
	tests := []struct {
		name string
		uri  string
		want string
	}{
		{
			name: "user and pass",
			uri:  "mongodb://deploy:s3cret@localhost:27017/myapp",
			want: "mongodb://deploy:***@localhost:27017/myapp",
		},
		{
			name: "multi-host srv-style options",
			uri:  "mongodb://deploy:s3cret@host1:27017,host2:27018/app?replicaSet=rs0",
			want: "mongodb://deploy:***@host1:27017,host2:27018/app?replicaSet=rs0",
		},
		{
			name: "unescaped at in password",
			uri:  "mongodb://deploy:p@ss@localhost/db",
			want: "mongodb://deploy:***@localhost/db",
		},
		{
			name: "empty password still masked",
			uri:  "mongodb://deploy:@localhost/db",
			want: "mongodb://deploy:***@localhost/db",
		},
		{
			name: "empty username with password",
			uri:  "mongodb://:pass@localhost/db",
			want: "mongodb://:***@localhost/db",
		},
		{
			name: "no userinfo untouched",
			uri:  "mongodb://localhost:27017/myapp",
			want: "mongodb://localhost:27017/myapp",
		},
		{
			name: "username only untouched",
			uri:  "mongodb://x509user@localhost/db",
			want: "mongodb://x509user@localhost/db",
		},
		{
			name: "at sign in query untouched",
			uri:  "mongodb://localhost/db?appName=me@corp",
			want: "mongodb://localhost/db?appName=me@corp",
		},
		{
			name: "not a uri untouched",
			uri:  "localhost:27017",
			want: "localhost:27017",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := RedactURI(tt.uri); got != tt.want {
				t.Errorf("RedactURI(%q) = %q, want %q", tt.uri, got, tt.want)
			}
		})
	}
}

func TestParseAuthSourceFromURI(t *testing.T) {
	tests := []struct {
		name string
		uri  string
		want string
	}{
		{"absent", "mongodb://host:27017/app", ""},
		{"no query at all", "mongodb://host:27017", ""},
		{"explicit", "mongodb://host:27017/app?authSource=admin", "admin"},
		{"lowercase key", "mongodb://host:27017/app?authsource=admin", "admin"},
		{"among other options", "mongodb+srv://c.example.net/app?retryWrites=true&authSource=admin&w=1", "admin"},
		{"multi-host", "mongodb://a:27017,b:27017/app?authSource=admin", "admin"},
		{"percent-encoded", "mongodb://host/app?authSource=my%20db", "my db"},
		{"empty value", "mongodb://host/app?authSource=", ""},
		{"not confused by a similar key", "mongodb://host/app?authMechanism=SCRAM-SHA-1", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ParseAuthSourceFromURI(tt.uri); got != tt.want {
				t.Errorf("ParseAuthSourceFromURI(%q) = %q, want %q", tt.uri, got, tt.want)
			}
		})
	}
}

func TestParseAuthMechanismFromURI(t *testing.T) {
	tests := []struct {
		name string
		uri  string
		want string
	}{
		{"absent", "mongodb://host:27017/app", ""},
		{"explicit", "mongodb://host/app?authMechanism=SCRAM-SHA-1", "SCRAM-SHA-1"},
		{"lowercase key", "mongodb://host/app?authmechanism=MONGODB-OIDC", "MONGODB-OIDC"},
		{"not confused by authSource", "mongodb://host/app?authSource=admin", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ParseAuthMechanismFromURI(tt.uri); got != tt.want {
				t.Errorf("ParseAuthMechanismFromURI(%q) = %q, want %q", tt.uri, got, tt.want)
			}
		})
	}
}

func TestParseHostFromURI(t *testing.T) {
	tests := []struct {
		name string
		uri  string
		want string
	}{
		{"simple", "mongodb://host.example.net:27017/app", "host.example.net"},
		{"no port", "mongodb://host.example.net/app", "host.example.net"},
		{"no path", "mongodb://host.example.net", "host.example.net"},
		{"srv", "mongodb+srv://c0.abc.mongodb.net/app", "c0.abc.mongodb.net"},
		{"first of several hosts", "mongodb://a.example.net:27017,b.example.net:27017/app", "a.example.net"},
		{"userinfo stripped", "mongodb://user:pass@host.example.net:27017/app", "host.example.net"},
		{"password containing an @", "mongodb://user:p@ss@host.example.net/app", "host.example.net"},
		{"ipv4", "mongodb://127.0.0.1:27017", "127.0.0.1"},
		{"ipv6 literal keeps its colons", "mongodb://[::1]:27017/app", "::1"},
		{"query only", "mongodb://host.example.net?tls=true", "host.example.net"},
		{"not a connection string", "host.example.net:27017", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ParseHostFromURI(tt.uri); got != tt.want {
				t.Errorf("ParseHostFromURI(%q) = %q, want %q", tt.uri, got, tt.want)
			}
		})
	}
}

func TestIsTLS(t *testing.T) {
	tests := []struct {
		name string
		uri  string
		want bool
	}{
		{"srv implies tls", "mongodb+srv://c0.abc.mongodb.net/app", true},
		{"srv with tls=true", "mongodb+srv://c0.abc.mongodb.net/app?tls=true", true},
		{"srv turned off explicitly", "mongodb+srv://c0.abc.mongodb.net/app?tls=false", false},
		{"srv turned off via ssl", "mongodb+srv://c0.abc.mongodb.net/app?ssl=false", false},
		{"plain is not tls", "mongodb://host:27017/app", false},
		{"plain with tls=true", "mongodb://host:27017/app?tls=true", true},
		{"plain with ssl=true", "mongodb://host:27017/app?ssl=true", true},
		{"case-insensitive key and value", "mongodb://host:27017/app?TLS=TRUE", true},
		{"tls among other options", "mongodb://host:27017/app?retryWrites=true&tls=true", true},
		{"tls=false is not tls", "mongodb://host:27017/app?tls=false", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsTLS(tt.uri); got != tt.want {
				t.Errorf("IsTLS(%q) = %v, want %v", tt.uri, got, tt.want)
			}
		})
	}
}
