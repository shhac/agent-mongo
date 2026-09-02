package config

// Kind names the MongoDB authentication mechanism a credential drives. An
// empty Kind reads as KindSCRAM, so configs written before kinds existed keep
// working and are never rewritten just to gain the field.
type Kind string

const (
	KindSCRAM Kind = "scram"
	KindOIDC  Kind = "oidc"
)

// FlowType names how an OIDC credential obtains a session.
type FlowType string

const (
	// FlowEnvironment: the driver reads the identity the platform already
	// gave this process — a projected Kubernetes service-account token, an
	// Azure managed identity, a GCE service account. Nothing is stored and
	// nothing expires from agent-mongo's point of view.
	FlowEnvironment FlowType = "environment"
	// FlowFile: a token some other tool already wrote to disk. Covers the
	// platforms the driver has no built-in provider for, and lets whatever
	// already holds an identity (az, gcloud, a sidecar) do the issuing.
	FlowFile FlowType = "file"
)

// Environment values the driver implements for FlowEnvironment.
const (
	EnvironmentK8s   = "k8s"
	EnvironmentAzure = "azure"
	EnvironmentGCP   = "gcp"
)

// Flow is how an OIDC credential gets a session, and how it gets a new one
// when that expires. It is the whole of an OIDC credential's configuration:
// there is no secret to store for the flows that read an ambient identity.
type Flow struct {
	Type FlowType `json:"type"`
	// Environment selects the platform identity provider for FlowEnvironment.
	Environment string `json:"environment,omitempty"`
	// TokenResource is the audience the token is minted for. Required by the
	// driver for the azure and gcp environments.
	TokenResource string `json:"token_resource,omitempty"`
	// ClientID identifies which managed identity to use on azure. Ignored by
	// the gcp and k8s providers.
	ClientID string `json:"client_id,omitempty"`
	// Path is the file holding the token, for FlowFile. Absolute, because a
	// CLI is run from wherever the caller happens to be.
	Path string `json:"path,omitempty"`
	// AllowedHosts limits the hosts this credential's token may be sent to.
	// Empty means the built-in default; see credential.DefaultAllowedHosts.
	AllowedHosts []string `json:"allowed_hosts,omitempty"`
}

type Credential struct {
	Kind     Kind   `json:"kind,omitempty"`
	Username string `json:"username,omitempty"`
	Password string `json:"password,omitempty"`
	Flow     *Flow  `json:"flow,omitempty"`
}

// ResolvedKind is the only reader of Kind: it supplies the SCRAM default so no
// caller has to remember that an absent kind is not an invalid one.
func (c Credential) ResolvedKind() Kind {
	if c.Kind == "" {
		return KindSCRAM
	}
	return c.Kind
}

// SetCredential stores an entry, creating the map when this is the first one.
//
// A mutator on *Config rather than a top-level helper, because a caller that
// keeps secrets in the OS keychain has to do this inside the same critical
// section as its keychain writes.
func (c *Config) SetCredential(alias string, cred Credential) {
	if c.Credentials == nil {
		c.Credentials = map[string]Credential{}
	}
	c.Credentials[alias] = cred
}
