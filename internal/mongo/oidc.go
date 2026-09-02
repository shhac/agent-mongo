package mongo

// The driver side of MONGODB-OIDC: how a flow recipe becomes driver auth
// options. SCRAM's mapping stays in auth.go, which the two share only through
// applyAuth's dispatch.

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
