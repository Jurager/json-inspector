package bridge

// Ingest is what the server does with whatever the extension sends. It is declared here, next to
// the consumer, so this package never imports the layer that implements it — the wiring happens in
// the dependency graph, not in an import.
type Ingest interface {
	Captured(req CapturedRequest)
	StateChanged(state CaptureState)
	ClientGone()
	FocusRequested(req FocusRequest)
}

// nopIngest keeps a server usable without a listener; the app always supplies a real one.
type nopIngest struct{}

func (nopIngest) Captured(CapturedRequest)    {}
func (nopIngest) StateChanged(CaptureState)   {}
func (nopIngest) ClientGone()                 {}
func (nopIngest) FocusRequested(FocusRequest) {}
