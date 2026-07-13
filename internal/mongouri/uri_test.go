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
