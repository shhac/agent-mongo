package mongo

import (
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	"github.com/shhac/agent-mongo/internal/config"
	"github.com/shhac/agent-mongo/internal/credential"
	"github.com/shhac/agent-mongo/internal/mongouri"
)

// applyAuth maps a resolved credential onto driver auth options. The default
// arm is the guard for a kind that credential.Resolve learns to resolve before
// this switch learns to drive it.
func applyAuth(
	clientOpts *options.ClientOptions, conn config.Connection, res credential.Resolution,
) (*options.ClientOptions, error) {
	switch res.Kind {
	case config.KindSCRAM:
		return clientOpts.SetAuth(scramCredential(clientOpts, conn, res)), nil
	case config.KindOIDC:
		cred, err := oidcCredential(conn, res)
		if err != nil {
			return nil, err
		}
		return clientOpts.SetAuth(cred), nil
	default:
		return nil, credential.UnsupportedKindError(res.Alias, res.Kind)
	}
}

// scramCredential overlays the stored username and password onto whatever the
// URI already asked for, rather than replacing it.
//
// SetAuth swaps out the whole Credential that ApplyURI derived, and once the
// userinfo has been moved into the credential store ApplyURI derives nothing at
// all: connstring.HasAuthParameters does not count authSource on its own, and a
// URI naming a mechanism without a username fails the driver's own validation,
// so ApplyURI leaves Auth nil in both cases. Both options are therefore read
// back out of the URI directly. Extracting credentials out of a URI must not
// change how that URI authenticates.
//
// authMechanismProperties is not carried across: it is meaningful only for
// GSSAPI, AWS and OIDC, none of which authenticate with a stored SCRAM
// username and password.
func scramCredential(
	clientOpts *options.ClientOptions, conn config.Connection, res credential.Resolution,
) options.Credential {
	cred := options.Credential{}
	if clientOpts.Auth != nil {
		cred = *clientOpts.Auth
	}
	cred.Username = res.Credential.Username
	cred.Password = res.Credential.Password
	cred.PasswordSet = true
	if cred.AuthSource == "" {
		cred.AuthSource = uriAuthSource(conn.ConnectionString)
	}
	if cred.AuthMechanism == "" {
		cred.AuthMechanism = mongouri.ParseAuthMechanismFromURI(conn.ConnectionString)
	}
	return cred
}

// uriAuthSource is the auth database the driver itself would have picked from a
// URI carrying inline credentials: the explicit authSource option, else the
// database in the path, else "" so the driver applies its own "admin" default
// (connstring.go:315-319).
func uriAuthSource(uri string) string {
	if source := mongouri.ParseAuthSourceFromURI(uri); source != "" {
		return source
	}
	return mongouri.ParseDBFromURI(uri)
}
