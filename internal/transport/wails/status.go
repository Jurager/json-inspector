package wails

import (
	"sync"
)

// Failure kinds, so the UI can phrase a broken data directory differently from a broken schema.
const (
	FailureDataDir   = "data-dir"
	FailureDatabase  = "database"
	FailureMigration = "migration"
)

// Failure is a startup problem the frontend has to explain. It cannot be a dialog: a dialog needs
// a running app, and the failures recorded here happen before one exists.
type Failure struct {
	Kind string `json:"kind"`
	// Detail is the error itself, for the screen to show under the sentence it writes for the kind.
	Detail string `json:"detail,omitempty"`
}

// Status is the outcome of everything that must work before the app has any data at all.
type Status struct {
	mu      sync.Mutex
	ready   bool
	failure *Failure
}

func NewStatus() *Status {
	return &Status{}
}

func (s *Status) Fail(kind string, detail string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ready = false
	s.failure = &Failure{Kind: kind, Detail: detail}
}

func (s *Status) Succeed() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ready = true
	s.failure = nil
}

func (s *Status) Ready() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.ready
}

func (s *Status) Failure() *Failure {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.failure
}

// StartupStatus is what the frontend reads on mount: whether the app has a database at all, and
// where that database lives either way.
type StartupStatus struct {
	Ready   bool     `json:"ready"`
	Failure *Failure `json:"failure,omitempty"`
	DataDir string   `json:"dataDir"`
	DBPath  string   `json:"dbPath"`
}
