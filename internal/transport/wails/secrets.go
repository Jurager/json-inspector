package wails

import (
	"context"

	"json-inspector/internal/infra/keychain"
	"json-inspector/internal/usecase/environment"
)

// keychainSecrets adapts the OS keychain to what the import needs. It exists only because secrets
// used to live there: the adapter and the package under it go away one release after the import has
// run everywhere, and nothing else in the app reads the keychain.
type keychainSecrets struct{}

// The assertion is what keeps this adapter and the port it serves from drifting apart.
var _ environment.SecretSource = keychainSecrets{}

// Get reports the keychain's answer. On anything but macOS that is "unavailable", which the import
// records as a warning per secret rather than failing over.
func (keychainSecrets) Get(_ context.Context, scope, name string) (string, error) {
	return keychain.Get(scope, name)
}
