// Package record owns the history: what was sent, what came back, and how long any of it is
// kept.
package record

import (
	"sync/atomic"

	"json-inspector/internal/platform"
)

// Event topics. A record appearing and a send finishing are different things: the first is one the
// app was told about, the second is an answer to something the window asked for. There is no
// "started" topic: the caller gets the id from Send itself, and it is the only caller there is.
const (
	TopicRecordAdded     = "record:added"
	TopicRequestFinished = "request:finished"
	TopicRequestFailed   = "request:failed"
)

type UseCase struct {
	store     Store
	scope     Scope
	executor  Executor
	notifier  Notifier
	retention RetentionSource
	screen    Screener
	mask      Masker
	ids       platform.IDGen

	// sincePrune counts saves between two looks at the retention rules. It is atomic because a
	// window's send and a collection run save from goroutines of their own, and two sends landing on
	// the same counter would be a data race on a number that decides when history is trimmed.
	sincePrune atomic.Int32
}

func NewUseCase(
	store Store,
	scope Scope,
	executor Executor,
	notifier Notifier,
	retention RetentionSource,
	screen Screener,
	mask Masker,
	ids platform.IDGen,
) *UseCase {
	return &UseCase{
		store: store, scope: scope, executor: executor, notifier: notifier, retention: retention,
		screen: screen, mask: mask, ids: ids,
	}
}
