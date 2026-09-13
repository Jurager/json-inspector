package domain

// Collection is a saved tree of requests with a name of its own. Folders and requests live inside it
// in one order, which is the order the tree draws them and the order a run walks them.
type Collection struct {
	ID          string           `json:"id"`
	Name        string           `json:"name"`
	Description string           `json:"description,omitempty"`
	Position    int64            `json:"position"`
	CreatedAt   int64            `json:"createdAt"`
	UpdatedAt   int64            `json:"updatedAt"`
	Items       []CollectionNode `json:"items"`

	// Auth is what everything inside inherits unless it says otherwise. It is a pointer for the same
	// reason a node's is: nil is "nothing here", and the walk stops at the first non-nil it meets.
	Auth *Auth `json:"auth,omitempty"`
}

// NodeKind is what a node is: a folder holds other nodes, a request is the thing that gets sent.
type NodeKind string

const (
	NodeFolder  NodeKind = "folder"
	NodeRequest NodeKind = "request"
)

// CollectionNode is one entry of a collection's tree. Folders and requests share the type because
// they share the tree: moving one is a parent and a position, not a different table.
//
// The request's own fields are absent until the node is opened: a tree of two hundred nodes has no
// business carrying two hundred bodies, and the method is all a tree row draws.
//
// Auth and Scripts are pointers for the reason the schema's NULL columns exist: nil means "not set
// here, take the parent's" and a value — even an empty one — means "this is the answer, stop
// looking". Without the difference a collection could not say "no auth for anything in here".
type CollectionNode struct {
	ID           string   `json:"id"`
	ParentID     string   `json:"parentId,omitempty"`
	CollectionID string   `json:"collectionId"`
	Kind         NodeKind `json:"kind"`
	Name         string   `json:"name"`
	Position     int64    `json:"position"`

	// A request's own fields. `omitempty` where a zero value and an absent one mean the same thing.
	Method  string      `json:"method,omitempty"`
	URL     string      `json:"url,omitempty"`
	Params  []Row       `json:"params,omitempty"`
	Headers []Row       `json:"headers,omitempty"`
	Body    string      `json:"body,omitempty"`
	Cookies []CookieRow `json:"cookies,omitempty"`
	Auth    *Auth       `json:"auth,omitempty"`

	Description string `json:"description,omitempty"`
	CreatedAt   int64  `json:"createdAt"`
	UpdatedAt   int64  `json:"updatedAt"`

	// Items are a folder's children, in order. A request has none.
	Items []CollectionNode `json:"items,omitempty"`
}
