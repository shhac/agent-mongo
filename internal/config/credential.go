package config

// Kind names the MongoDB authentication mechanism a credential drives. An
// empty Kind reads as KindSCRAM, so configs written before kinds existed keep
// working and are never rewritten just to gain the field.
type Kind string

const KindSCRAM Kind = "scram"

type Credential struct {
	Kind     Kind   `json:"kind,omitempty"`
	Username string `json:"username,omitempty"`
	Password string `json:"password,omitempty"`
}

// ResolvedKind is the only reader of Kind: it supplies the SCRAM default so no
// caller has to remember that an absent kind is not an invalid one.
func (c Credential) ResolvedKind() Kind {
	if c.Kind == "" {
		return KindSCRAM
	}
	return c.Kind
}
