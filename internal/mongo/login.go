package mongo

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	driver "go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	"github.com/shhac/agent-mongo/internal/config"
	"github.com/shhac/agent-mongo/internal/credential"
	"github.com/shhac/agent-mongo/internal/mongouri"
)

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
	host := mongouri.ParseHostFromURI(conn.ConnectionString)

	var session credential.Session
	callback := func(ctx context.Context, args *options.OIDCArgs) (*options.OIDCCredential, error) {
		idp := credential.IDPInfo{}
		if args.IDPInfo != nil {
			idp = credential.IDPInfo{
				Issuer:   args.IDPInfo.Issuer,
				ClientID: args.IDPInfo.ClientID,
				Scopes:   args.IDPInfo.RequestScopes,
			}
		}

		completed, err := credential.RunDeviceLogin(ctx, idp, host, prompt)
		if err != nil {
			return nil, err
		}
		session = completed

		result := &options.OIDCCredential{
			AccessToken:  completed.AccessToken,
			RefreshToken: nonEmpty(completed.RefreshToken),
		}
		if !completed.ExpiresAt.IsZero() {
			expiry := completed.ExpiresAt
			result.ExpiresAt = &expiry
		}
		return result, nil
	}

	clientOpts := options.Client().
		ApplyURI(conn.ConnectionString).
		SetMaxPoolSize(1).
		SetMinPoolSize(0).
		SetServerSelectionTimeout(10 * time.Second).
		SetAuth(options.Credential{
			AuthMechanism:     oidcMechanism,
			OIDCHumanCallback: callback,
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
	if session.AccessToken == "" {
		return credential.Session{}, credential.LoginNotAttemptedError()
	}
	return session, nil
}

func nonEmpty(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
