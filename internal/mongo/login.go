package mongo

import (
	"context"

	"go.mongodb.org/mongo-driver/v2/bson"
	driver "go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	"github.com/shhac/agent-mongo/internal/config"
	"github.com/shhac/agent-mongo/internal/credential"
	"github.com/shhac/agent-mongo/internal/mongouri"
)

// deviceLogin is the driver-facing half of an interactive login: it converts
// between the driver's OIDC types and agent-mongo's, and remembers the session
// the flow produced.
//
// A type rather than a closure inside DeviceLogin so the conversion can be
// exercised directly. Everything fiddly lives here — the server's IdP metadata
// may be absent, and the driver wants pointers for the optional fields — and
// none of it would otherwise run outside a live OIDC-configured deployment.
type deviceLogin struct {
	host    string
	prompt  func(credential.DevicePrompt)
	session credential.Session
}

func (d *deviceLogin) callback(
	ctx context.Context, args *options.OIDCArgs,
) (*options.OIDCCredential, error) {
	var idp credential.IDPInfo
	if args != nil && args.IDPInfo != nil {
		idp = credential.IDPInfo{
			Issuer:   args.IDPInfo.Issuer,
			ClientID: args.IDPInfo.ClientID,
			Scopes:   args.IDPInfo.RequestScopes,
		}
	}

	session, err := credential.RunDeviceLogin(ctx, idp, d.host, d.prompt)
	if err != nil {
		return nil, err
	}
	d.session = session

	result := &options.OIDCCredential{AccessToken: session.AccessToken}
	if session.RefreshToken != "" {
		refresh := session.RefreshToken
		result.RefreshToken = &refresh
	}
	if !session.ExpiresAt.IsZero() {
		expiry := session.ExpiresAt
		result.ExpiresAt = &expiry
	}
	return result, nil
}

// DeviceLogin performs an interactive OIDC login against a deployment and
// returns the session to store.
//
// It connects rather than talking to the identity provider directly, because
// the deployment is what says which provider guards it: the driver's human flow
// asks the server for its IdP metadata and hands it to the callback. That is
// why the device flow stores no issuer or client id of its own, and why a login
// needs a connection.
//
// No operation timeout is applied. A person has to read a code, open a link and
// approve on another device, which is not a query.
func DeviceLogin(
	ctx context.Context, conn config.Connection, prompt func(credential.DevicePrompt),
) (credential.Session, error) {
	login := &deviceLogin{
		host:   mongouri.ParseHostFromURI(conn.ConnectionString),
		prompt: prompt,
	}

	clientOpts := baseClientOptions(conn.ConnectionString).
		SetAuth(options.Credential{
			AuthMechanism:     oidcMechanism,
			OIDCHumanCallback: login.callback,
		})

	client, err := driver.Connect(clientOpts)
	if err != nil {
		return credential.Session{}, err
	}
	defer func() { _ = client.Disconnect(context.Background()) }()

	// The connection is lazy, so something has to actually run for the driver
	// to authenticate and the callback to fire.
	if err := client.Database("admin").
		RunCommand(ctx, bson.D{{Key: "ping", Value: 1}}).Err(); err != nil {
		return credential.Session{}, err
	}
	if login.session.AccessToken == "" {
		return credential.Session{}, credential.LoginNotAttemptedError()
	}
	return login.session, nil
}
