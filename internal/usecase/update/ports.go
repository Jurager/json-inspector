package update

import (
	"context"

	"json-inspector/internal/domain"
)

// Releases is the release source: what versions exist, and how one of them gets installed. Declared
// here because this is the side that needs it — the mechanism underneath knows nothing about
// throttling, skipped versions or channels, and that is what keeps it replaceable.
type Releases interface {
	Latest(ctx context.Context, channel domain.UpdateChannel) (domain.Release, error)
	Install(ctx context.Context, version string) error
}

// State keeps the one value the app remembers about updating. The preferences are read through
// Settings instead of from here, so that each key in the table has exactly one owner.
type State interface {
	LoadUpdateState(ctx context.Context) (domain.UpdateState, error)
	SaveUpdateState(ctx context.Context, state domain.UpdateState) error
}

// Settings is the two preferences this feature reads: whether to check on its own, and which
// channel to ask. A narrow interface satisfied by the settings use case as it stands — nothing
// adapts it, and this package never learns what else the app remembers.
type Settings interface {
	Snapshot(ctx context.Context) (domain.Settings, error)
}
