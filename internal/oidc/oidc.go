// Package oidc is a minimal OpenID Connect client: provider discovery, the
// RFC 8628 device authorization grant, and the refresh-token grant.
//
// It knows nothing about MongoDB, agent-mongo's config, or the driver. That is
// the point: the device flow is the one part of OIDC support with real protocol
// logic, and keeping it here lets it be tested against an ordinary httptest
// server rather than through a database connection.
package oidc

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// The failure modes a caller acts on differently.
var (
	// ErrDenied: the person declined the authorization request.
	ErrDenied = errors.New("authorization denied")
	// ErrCodeExpired: the user code was not entered before it expired.
	ErrCodeExpired = errors.New("device code expired")
	// ErrRefreshRejected: the refresh token is no longer accepted, so only a
	// fresh interactive login will do.
	ErrRefreshRejected = errors.New("refresh token rejected")
)

// Client talks to one identity provider.
//
// HTTP, Now and Sleep are injected rather than reached for directly so a test
// can point the client at an httptest server (which needs that server's own
// client for its certificate) and drive polling without spending real seconds.
type Client struct {
	HTTP  *http.Client
	Now   func() time.Time
	Sleep func(time.Duration)
}

func (c *Client) httpClient() *http.Client {
	if c.HTTP != nil {
		return c.HTTP
	}
	return http.DefaultClient
}

func (c *Client) now() time.Time {
	if c.Now != nil {
		return c.Now()
	}
	return time.Now()
}

func (c *Client) sleep(d time.Duration) {
	if c.Sleep != nil {
		c.Sleep(d)
		return
	}
	time.Sleep(d)
}

// Provider is the subset of an issuer's metadata this client needs.
type Provider struct {
	Issuer                      string
	DeviceAuthorizationEndpoint string
	TokenEndpoint               string
}

// Discover reads the issuer's OpenID configuration.
func (c *Client) Discover(ctx context.Context, issuer string) (Provider, error) {
	endpoint := strings.TrimSuffix(issuer, "/") + "/.well-known/openid-configuration"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return Provider{}, fmt.Errorf("building the discovery request for %q: %w", issuer, err)
	}

	var doc struct {
		Issuer                      string `json:"issuer"`
		DeviceAuthorizationEndpoint string `json:"device_authorization_endpoint"`
		TokenEndpoint               string `json:"token_endpoint"`
	}
	if err := c.do(req, &doc); err != nil {
		return Provider{}, fmt.Errorf("discovering %q: %w", issuer, err)
	}
	if doc.DeviceAuthorizationEndpoint == "" || doc.TokenEndpoint == "" {
		return Provider{}, fmt.Errorf(
			"identity provider %q does not advertise the device authorization grant", issuer)
	}
	return Provider{
		Issuer:                      doc.Issuer,
		DeviceAuthorizationEndpoint: doc.DeviceAuthorizationEndpoint,
		TokenEndpoint:               doc.TokenEndpoint,
	}, nil
}

// DeviceAuth is a started device authorization: what to show the person, and
// what to poll with.
type DeviceAuth struct {
	DeviceCode string
	// UserCode is the short code the person types.
	UserCode string
	// VerificationURI is where they type it. VerificationURIComplete, when the
	// provider supplies one, has the code already embedded.
	VerificationURI         string
	VerificationURIComplete string
	Interval                time.Duration
	ExpiresAt               time.Time
}

// StartDeviceAuth begins the grant, returning the code to show the person.
func (c *Client) StartDeviceAuth(
	ctx context.Context, p Provider, clientID string, scopes []string,
) (DeviceAuth, error) {
	form := url.Values{"client_id": {clientID}}
	if len(scopes) > 0 {
		form.Set("scope", strings.Join(scopes, " "))
	}

	var body struct {
		DeviceCode              string `json:"device_code"`
		UserCode                string `json:"user_code"`
		VerificationURI         string `json:"verification_uri"`
		VerificationURIComplete string `json:"verification_uri_complete"`
		ExpiresIn               int    `json:"expires_in"`
		Interval                int    `json:"interval"`
	}
	if err := c.postForm(ctx, p.DeviceAuthorizationEndpoint, form, &body); err != nil {
		return DeviceAuth{}, fmt.Errorf("starting device authorization: %w", err)
	}
	if body.DeviceCode == "" || body.UserCode == "" || body.VerificationURI == "" {
		return DeviceAuth{}, errors.New(
			"the identity provider's device authorization response was missing a code or verification URL")
	}

	// RFC 8628 makes interval optional and says 5 seconds when absent.
	interval := time.Duration(body.Interval) * time.Second
	if interval <= 0 {
		interval = 5 * time.Second
	}
	expiresIn := time.Duration(body.ExpiresIn) * time.Second
	if expiresIn <= 0 {
		expiresIn = 10 * time.Minute
	}

	return DeviceAuth{
		DeviceCode:              body.DeviceCode,
		UserCode:                body.UserCode,
		VerificationURI:         body.VerificationURI,
		VerificationURIComplete: body.VerificationURIComplete,
		Interval:                interval,
		ExpiresAt:               c.now().Add(expiresIn),
	}, nil
}

// PollForToken waits for the person to finish authorizing.
//
// It honours the provider's interval and the slow_down response, and stops when
// the user code expires rather than polling a dead code forever.
func (c *Client) PollForToken(
	ctx context.Context, p Provider, clientID string, auth DeviceAuth,
) (Token, error) {
	interval := auth.Interval
	for {
		if !c.now().Before(auth.ExpiresAt) {
			return Token{}, ErrCodeExpired
		}
		c.sleep(interval)
		if err := ctx.Err(); err != nil {
			return Token{}, err
		}

		token, err := c.exchange(ctx, p, url.Values{
			"grant_type":  {"urn:ietf:params:oauth:grant-type:device_code"},
			"device_code": {auth.DeviceCode},
			"client_id":   {clientID},
		})
		var oauthErr *Error
		switch {
		case err == nil:
			return token, nil
		case !errors.As(err, &oauthErr):
			return Token{}, err
		}

		switch oauthErr.Code {
		case "authorization_pending":
			// Nothing has happened yet; keep waiting.
		case "slow_down":
			// The provider is asking for more room between polls, and RFC 8628
			// says to add five seconds each time it does.
			interval += 5 * time.Second
		case "access_denied":
			return Token{}, ErrDenied
		case "expired_token":
			return Token{}, ErrCodeExpired
		default:
			return Token{}, err
		}
	}
}

// Token is a set of credentials from the provider.
type Token struct {
	AccessToken  string
	RefreshToken string
	ExpiresAt    time.Time
}

// Refresh exchanges a refresh token for a new access token.
//
// A provider that does not return a new refresh token is signalling that the
// old one still stands, so the caller keeps it.
func (c *Client) Refresh(
	ctx context.Context, p Provider, clientID, refreshToken string,
) (Token, error) {
	token, err := c.exchange(ctx, p, url.Values{
		"grant_type":    {"refresh_token"},
		"refresh_token": {refreshToken},
		"client_id":     {clientID},
	})
	var oauthErr *Error
	if errors.As(err, &oauthErr) && oauthErr.Code == "invalid_grant" {
		return Token{}, ErrRefreshRejected
	}
	return token, err
}

func (c *Client) exchange(ctx context.Context, p Provider, form url.Values) (Token, error) {
	var body struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		ExpiresIn    int    `json:"expires_in"`
	}
	if err := c.postForm(ctx, p.TokenEndpoint, form, &body); err != nil {
		return Token{}, err
	}
	if body.AccessToken == "" {
		return Token{}, errors.New("the identity provider returned no access token")
	}

	token := Token{AccessToken: body.AccessToken, RefreshToken: body.RefreshToken}
	if body.ExpiresIn > 0 {
		token.ExpiresAt = c.now().Add(time.Duration(body.ExpiresIn) * time.Second)
	}
	return token, nil
}

// Error is an OAuth 2.0 error response. The code is what a caller branches on;
// the description is what a person reads.
type Error struct {
	Code        string
	Description string
}

func (e *Error) Error() string {
	if e.Description == "" {
		return e.Code
	}
	return e.Code + ": " + e.Description
}

func (c *Client) postForm(ctx context.Context, endpoint string, form url.Values, out any) error {
	req, err := http.NewRequestWithContext(
		ctx, http.MethodPost, endpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return fmt.Errorf("building the request to %q: %w", endpoint, err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")
	return c.do(req, out)
}

// do sends the request and decodes the response, turning an OAuth error
// response into an *Error whichever status code carries it — providers use both
// 400 and 200 for authorization_pending in practice.
func (c *Client) do(req *http.Request, out any) error {
	resp, err := c.httpClient().Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	// Bounded so a misconfigured endpoint returning something enormous cannot
	// be read into memory in full.
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return fmt.Errorf("reading the response from %q: %w", req.URL, err)
	}

	var oauthErr struct {
		Error            string `json:"error"`
		ErrorDescription string `json:"error_description"`
	}
	if err := json.Unmarshal(body, &oauthErr); err == nil && oauthErr.Error != "" {
		return &Error{Code: oauthErr.Error, Description: oauthErr.ErrorDescription}
	}
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return fmt.Errorf("%s returned %s", req.URL, resp.Status)
	}
	if err := json.Unmarshal(body, out); err != nil {
		return fmt.Errorf("decoding the response from %q: %w", req.URL, err)
	}
	return nil
}
