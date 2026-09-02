package mongo

import (
	"context"

	"go.mongodb.org/mongo-driver/v2/mongo/options"

	"github.com/shhac/agent-mongo/internal/config"
	"github.com/shhac/agent-mongo/internal/credential"
	"github.com/shhac/agent-mongo/internal/mongouri"
)

// The driver's MONGODB-OIDC vocabulary. Spelled out rather than imported: the
// constants live in x/mongo/driver/auth, which is not part of the driver's
// supported surface.
const (
	oidcMechanism         = "MONGODB-OIDC"
	oidcEnvironmentProp   = "ENVIRONMENT"
	oidcTokenResourceProp = "TOKEN_RESOURCE"
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

// oidcCredential hands the flow recipe to the driver, which does the rest: for
// the environment flows it reads the platform identity itself, so agent-mongo
// supplies configuration and never a token.
//
// The whole Credential is replaced rather than overlaid, unlike SCRAM. The flow
// is the source of truth for how an OIDC credential authenticates, and
// authSource is deliberately left empty — the driver requires it to be empty or
// "$external" for MONGODB-OIDC and fills it in itself.
//
// Only the platform-identity flows are named here. Every other flow is one
// where agent-mongo holds the token, and this package deliberately knows
// nothing about how: it asks the resolution, so files, keychains and refreshes
// stay in internal/credential where the clock and HTTP seams live.
func oidcCredential(conn config.Connection, res credential.Resolution) (options.Credential, error) {
	flow, err := res.OIDCFlow()
	if err != nil {
		return options.Credential{}, err
	}

	switch flow.Type {
	case config.FlowEnvironment:
		props := map[string]string{oidcEnvironmentProp: flow.Environment}
		if flow.TokenResource != "" {
			props[oidcTokenResourceProp] = flow.TokenResource
		}
		return options.Credential{
			AuthMechanism:           oidcMechanism,
			AuthMechanismProperties: props,
			// Azure reads this as the managed-identity client id; the gcp and
			// k8s providers ignore it, and it is empty for them anyway.
			Username: flow.ClientID,
		}, nil
	default:
		// Every other flow is one where agent-mongo holds the token. The
		// driver is handed a callback rather than the token itself, so it is
		// fetched when authentication happens: a rotated file is picked up,
		// and a reauth after the server expires a session re-reads it.
		return options.Credential{
			AuthMechanism: oidcMechanism,
			OIDCMachineCallback: func(ctx context.Context, _ *options.OIDCArgs) (*options.OIDCCredential, error) {
				token, err := res.AccessToken(ctx, mongouri.ParseHostFromURI(conn.ConnectionString))
				if err != nil {
					return nil, err
				}
				return &options.OIDCCredential{AccessToken: token}, nil
			},
		}, nil
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
