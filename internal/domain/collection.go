package domain

// Collection is a saved group of requests with a name of its own, and it may hold other
// collections. What used to be a folder is one of these with a parent: the two were never more than
// that apart, and keeping them separate is what stopped a collection from being put inside one.
type Collection struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Position    int64  `json:"position"`
	CreatedAt   int64  `json:"createdAt"`
	UpdatedAt   int64  `json:"updatedAt"`

	// ParentID is the collection this one sits in. Empty means the top level.
	ParentID string `json:"parentId,omitempty"`

	// Items are the requests this collection holds, and Children the collections inside it. The two
	// share one position space, which is what keeps a nested collection where it was put rather than
	// at the end of the group.
	Items    []CollectionNode `json:"items"`
	Children []Collection     `json:"children"`

	// Auth is what everything inside inherits unless it says otherwise. It is a pointer for the same
	// reason a node's is: nil is "nothing here", and the walk stops at the first non-nil it meets.
	Auth *Auth `json:"auth,omitempty"`

	// Variables are the `{{tokens}}` this collection answers for its own requests, and for everything
	// inside it. They are the environment's own kind of variable and stand over it: a level that
	// names `baseUrl` means that value for everything below, which is what makes a collection
	// portable between environments.
	Variables []Variable `json:"variables,omitempty"`
}

// LevelRow is one request as the collection page's table draws it: what it is called, the method it
// goes out with and the address it goes to. It is a read of its own because a tree row carries no
// request payload — what a list loads is names and methods — and the page is the one screen that
// shows a row's address.
type LevelRow struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Method string `json:"method,omitempty"`
	URL    string `json:"url,omitempty"`
}

// LevelEntry is one row of a collection's level: a request, or a collection inside it. Exactly one
// of the two is set, and both are pointers into the tree that was read.
type LevelEntry struct {
	Node       *CollectionNode
	Collection *Collection
}

// Level is what a collection holds in the order the tree draws it. Its requests and the collections
// inside it are one number line, so the two lists alternate by position — and each of them already
// comes in position order, which is what makes one pass over the pair enough.
func (c *Collection) Level() []LevelEntry {
	out := make([]LevelEntry, 0, len(c.Items)+len(c.Children))
	i, j := 0, 0
	for i < len(c.Items) || j < len(c.Children) {
		// A request and a collection never share a position: the number is written once, when the row
		// is created or dropped, and the two are numbered in one sequence.
		if j >= len(c.Children) || (i < len(c.Items) && c.Items[i].Position <= c.Children[j].Position) {
			out = append(out, LevelEntry{Node: &c.Items[i]})
			i++
			continue
		}
		out = append(out, LevelEntry{Collection: &c.Children[j]})
		j++
	}
	return out
}

// CollectionNode is one request of a collection. The fields beyond the name are absent until the
// node is opened: a collection of two hundred requests has no business carrying two hundred bodies,
// and the method is all a tree row draws.
//
// Auth and Scripts are pointers for the reason the schema's NULL columns exist: nil means "not set
// here, take the parent's" and a value — even an empty one — means "this is the answer, stop
// looking". Without the difference a collection could not say "no auth for anything in here".
type CollectionNode struct {
	ID           string `json:"id"`
	CollectionID string `json:"collectionId"`
	Name         string `json:"name"`
	Position     int64  `json:"position"`

	// A request's own fields. `omitempty` where a zero value and an absent one mean the same thing.
	Method   string      `json:"method,omitempty"`
	URL      string      `json:"url,omitempty"`
	Params   []Row       `json:"params,omitempty"`
	Headers  []Row       `json:"headers,omitempty"`
	Body     string      `json:"body,omitempty"`
	BodyKind BodyKind    `json:"bodyKind,omitempty"`
	Form     []FormRow   `json:"form,omitempty"`
	BodyFile string      `json:"bodyFile,omitempty"`
	Cookies  []CookieRow `json:"cookies,omitempty"`
	Auth     *Auth       `json:"auth,omitempty"`
	// Scripts are the code this request runs around itself. Nil means "not set here" and the levels
	// above are what runs; empty means this level has nothing to add.
	Scripts *Scripts `json:"scripts,omitempty"`

	Description string `json:"description,omitempty"`
	CreatedAt   int64  `json:"createdAt"`
	UpdatedAt   int64  `json:"updatedAt"`
}

// CollectionRun is one execution of a collection: when it happened and how it went. The requests it
// reached are beside it, one row each.
type CollectionRun struct {
	ID           string `json:"id"`
	CollectionID string `json:"collectionId"`
	// NodeID is the single request the run was started from. Empty means the collection as a whole,
	// nested collections included.
	NodeID string `json:"nodeId,omitempty"`

	// Environment is the name of the environment the run went out under, kept as it was called at
	// the time. It is a name rather than an id because a run is a thing that happened: the
	// environment on screen next week is not the one these requests were sent with, and neither is
	// the name it may have been renamed to since.
	Environment string `json:"environment,omitempty"`

	StartedAt  int64 `json:"startedAt"`
	FinishedAt int64 `json:"finishedAt,omitempty"`
	Passed     int   `json:"passed"`
	Failed     int   `json:"failed"`
	DurationUs int64 `json:"durationUs"`

	Results []CollectionRunResult `json:"results"`
}

// CollectionRunResult is one request of a run. It names the node it came from rather than carrying
// the node: a result says what happened, and it outlives the request being renamed or deleted. What
// that node is called now is the tree's answer, and the tree is what the overview has.
type CollectionRunResult struct {
	NodeID   string `json:"nodeId"`
	Position int64  `json:"position"`
	// Status is nil when nothing came back, which is also the only case where a run has an error.
	Status     *int   `json:"status,omitempty"`
	OK         bool   `json:"ok"`
	DurationUs int64  `json:"durationUs"`
	Error      string `json:"error,omitempty"`
	// Assertions is what the scripts around this request asserted and how many of those held. The
	// status says the request went through; this says what came back was what it asked for. They are
	// two counts rather than the reports themselves: the page draws one table of them, and a report
	// per row would be a call per request to draw a row.
	AssertionsPassed int `json:"assertionsPassed"`
	AssertionsTotal  int `json:"assertionsTotal"`
	// Skipped is a request a pre-request script kept from going out. It is neither a pass nor a
	// failure, which is why it is a flag of its own: the run counts it as neither.
	Skipped bool `json:"skipped,omitempty"`
	// RecordID is what this request produced, and the only link from a run to what was actually sent:
	// the row opens it the way a history row opens its own record. Empty for a request that never
	// went out — there is nothing to open.
	RecordID string `json:"recordId,omitempty"`
}
