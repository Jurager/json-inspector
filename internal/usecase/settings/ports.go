package settings

import "context"

// Store is the key/value table the preferences live in. The use case owns the meaning of each key;
// the store only has to keep them.
type Store interface {
	Settings(ctx context.Context) (map[string]string, error)
	SaveSetting(ctx context.Context, key, value string) error
}

// Notifier publishes a change to whoever is listening. It is declared here, next to its user: the
// features that own the events decide what they are called, and the transport decides how they
// reach a window.
type Notifier interface {
	Publish(topic string, payload any)
}
