package credential

import (
	"slices"

	"github.com/shhac/agent-mongo/internal/config"
)

// kindHandler is what a credential kind supplies. One entry in the kinds table
// is the single place a kind is registered.
type kindHandler struct {
	// fields are the kind's keychain-backed fields, in resolution order.
	fields []secretField
	// validate rejects a stored entry this kind cannot drive. Nil when any
	// entry of this kind is usable.
	validate func(alias string, entry config.Credential) error
	// checkConnection rejects an endpoint this kind must not be used with.
	// Nil when the kind places no constraints on where it authenticates.
	checkConnection func(entry config.Credential, uri string) error
}

var kinds = map[config.Kind]kindHandler{
	config.KindSCRAM: {fields: scramFields},
	config.KindOIDC: {
		fields:          oidcFields,
		validate:        validateOIDC,
		checkConnection: checkOIDCConnection,
	},
}

func handlerFor(kind config.Kind) (kindHandler, bool) {
	h, ok := kinds[kind]
	return h, ok
}

// SupportedKinds lists the kinds this build implements, derived from the
// dispatch table so it cannot advertise a kind nothing drives. A hand-written
// list fails by telling the reader to choose from a set that is not real.
func SupportedKinds() []string { return sortedNames(kinds) }

// requireHandler dispatches to a kind, or names the kinds this build has.
func requireHandler(alias string, kind config.Kind) (kindHandler, error) {
	h, ok := handlerFor(kind)
	if !ok {
		return kindHandler{}, UnsupportedKindError(alias, kind)
	}
	return h, nil
}

func (h kindHandler) validateEntry(alias string, entry config.Credential) error {
	if h.validate == nil {
		return nil
	}
	return h.validate(alias, entry)
}

func (h kindHandler) checkEndpoint(entry config.Credential, uri string) error {
	if h.checkConnection == nil {
		return nil
	}
	return h.checkConnection(entry, uri)
}

// sortedNames lists a string-keyed table's keys in order, for the
// self-correcting "valid values" part of an error.
func sortedNames[K ~string, V any](table map[K]V) []string {
	names := make([]string, 0, len(table))
	for key := range table {
		names = append(names, string(key))
	}
	slices.Sort(names)
	return names
}
