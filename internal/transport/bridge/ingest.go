package bridge

// Ingest handles data received from the extension.
type Ingest interface {
	Captured(req CapturedRequest)
	StateChanged(state CaptureState)
	ClientGone()
	FocusRequested(req FocusRequest)
}

type nopIngest struct{}

func (nopIngest) Captured(CapturedRequest)    {}
func (nopIngest) StateChanged(CaptureState)   {}
func (nopIngest) ClientGone()                 {}
func (nopIngest) FocusRequested(FocusRequest) {}
