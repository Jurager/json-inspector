// Package collection owns the saved requests: the tree they live in, what inherits from what, and
// what happens when a whole collection is run.
package collection

import (
	"context"
	"fmt"
	"json-inspector/internal/domain"
	"json-inspector/internal/platform"
	"strconv"
	"strings"
	"sync/atomic"
)

// The suffix that marks a duplicate for what it is arrives from the window: a name is read by the
// user, and Go has no language to write it in.
// rather than in a view that would have to invent its own.

// maxNameLength is the same ceiling the environments screen uses, so a name that is too long means
// the same thing wherever it is typed.
const maxNameLength = 120

// maxDescriptionLength is what still reads as a description and not as a document pasted into a
// header. It is generous: a few sentences about what a collection is for.
const maxDescriptionLength = 500

type UseCase struct {
	store    Store
	scope    Scope
	sender   Sender
	notifier Notifier
	ids      platform.IDGen

	// A run holds the tree for as long as it lasts, and only one runs at a time: two of them would
	// write their rows into the same overview.
	running atomic.Bool
	stopped atomic.Bool
}

func NewUseCase(store Store, scope Scope, sender Sender, notifier Notifier, ids platform.IDGen) *UseCase {
	return &UseCase{store: store, scope: scope, sender: sender, notifier: notifier, ids: ids}
}

// Tree is every collection with its nodes, which is what the list draws and what a run walks.
func (u *UseCase) Tree(ctx context.Context) ([]domain.Collection, error) {
	workspace, err := u.scope.ActiveWorkspace(ctx)
	if err != nil {
		return nil, err
	}
	return u.store.Collections(ctx, workspace)
}

// Node reads one node whole: everything opening a request needs, and the tree deliberately left out.
func (u *UseCase) Node(ctx context.Context, id string) (domain.CollectionNode, error) {
	return u.store.Node(ctx, id)
}

// CreateCollection adds an empty collection at the end of the top level. The name is given rather
// than invented: the window asks for it in the tree, in the row the user is looking at.
func (u *UseCase) CreateCollection(ctx context.Context, name string, description string) ([]domain.Collection, error) {
	name, err := validName(name)
	if err != nil {
		return nil, err
	}

	workspace, err := u.scope.ActiveWorkspace(ctx)
	if err != nil {
		return nil, err
	}

	position, err := u.store.NextPosition(ctx, workspace, "")
	if err != nil {
		return nil, err
	}
	if err := u.store.SaveCollection(ctx, workspace, domain.Collection{
		ID:          u.ids(),
		Name:        name,
		Description: strings.TrimSpace(description),
		Position:    position,
		Items:       []domain.CollectionNode{},
		Children:    []domain.Collection{},
	}); err != nil {
		return nil, err
	}
	return u.Tree(ctx)
}

// NewNode is what the tree asks for when a row is created: where it goes and what is already known
// about the request. A request made by hand arrives with its method and nothing else — the rest is
// filled in by the card that opens next — while one saved from the command line is a whole request,
// and building it empty first would be a node with no address if the second step failed.
type NewNode struct {
	CollectionID string `json:"collectionId"`
	Name         string `json:"name"`

	Method   string             `json:"method,omitempty"`
	URL      string             `json:"url,omitempty"`
	Params   []domain.Row       `json:"params,omitempty"`
	Headers  []domain.Row       `json:"headers,omitempty"`
	Body     string             `json:"body,omitempty"`
	BodyKind domain.BodyKind    `json:"bodyKind,omitempty"`
	Form     []domain.FormRow   `json:"form,omitempty"`
	BodyFile string             `json:"bodyFile,omitempty"`
	Cookies  []domain.CookieRow `json:"cookies,omitempty"`
	Auth     *domain.Auth       `json:"auth,omitempty"`
}

// CreateNode adds a request to the end of a collection's level.
//
// It answers with the row that appeared and the tree it appeared in: the window needs the id Go
// minted, and looking it up by name afterwards would find the older row of the same name.
func (u *UseCase) CreateNode(ctx context.Context, in NewNode) (domain.CollectionNode, []domain.Collection, error) {
	name, err := validName(in.Name)
	if err != nil {
		return domain.CollectionNode{}, nil, err
	}
	workspace, err := u.scope.ActiveWorkspace(ctx)
	if err != nil {
		return domain.CollectionNode{}, nil, err
	}
	if _, ok, err := u.collection(ctx, workspace, in.CollectionID); err != nil {
		return domain.CollectionNode{}, nil, err
	} else if !ok {
		return domain.CollectionNode{}, nil, fmt.Errorf("collection %s: %w", in.CollectionID, domain.ErrNotFound)
	}

	position, err := u.store.NextPosition(ctx, workspace, in.CollectionID)
	if err != nil {
		return domain.CollectionNode{}, nil, err
	}

	node := domain.CollectionNode{
		ID:           u.ids(),
		CollectionID: in.CollectionID,
		Name:         name,
		Position:     position,
		Method:       defaultMethod(in.Method),
		URL:          strings.TrimSpace(in.URL),
		Params:       orEmptyRows(in.Params),
		Headers:      orEmptyRows(in.Headers),
		Body:         in.Body,
		BodyKind:     domain.KindOf(in.BodyKind),
		Form:         withFormIDs(u.ids, in.Form),
		BodyFile:     in.BodyFile,
		Cookies:      orEmptyCookies(in.Cookies),
		Auth:         in.Auth,
	}
	if err := u.store.SaveNode(ctx, node); err != nil {
		return domain.CollectionNode{}, nil, err
	}
	tree, err := u.store.Collections(ctx, workspace)
	if err != nil {
		return domain.CollectionNode{}, nil, err
	}
	return node, tree, nil
}

// MoveNode puts a request where it was dropped: another collection, or another place in its own. The
// position counts the level the way the tree draws it — a collection's requests and the collections
// inside it are one list — so the window can name the row it was dropped after.
func (u *UseCase) MoveNode(ctx context.Context, id string, collectionID string, position int64) ([]domain.Collection, error) {
	workspace, err := u.scope.ActiveWorkspace(ctx)
	if err != nil {
		return nil, err
	}
	if _, err := u.store.Node(ctx, id); err != nil {
		return nil, err
	}
	if _, ok, err := u.collection(ctx, workspace, collectionID); err != nil {
		return nil, err
	} else if !ok {
		return nil, fmt.Errorf("collection %s: %w", collectionID, domain.ErrNotFound)
	}

	if err := u.store.MoveNode(ctx, workspace, id, collectionID, position); err != nil {
		return nil, err
	}
	return u.store.Collections(ctx, workspace)
}

// MoveCollection puts a collection inside another, or back at the top level when the parent is empty.
//
// A collection dropped into itself or into one of its own children is refused rather than stored: the
// tree would be a ring, and a ring is a tree nothing can be drawn from.
func (u *UseCase) MoveCollection(ctx context.Context, id string, parentID string, position int64) ([]domain.Collection, error) {
	workspace, err := u.scope.ActiveWorkspace(ctx)
	if err != nil {
		return nil, err
	}
	tree, err := u.store.Collections(ctx, workspace)
	if err != nil {
		return nil, err
	}
	moved, ok := findCollection(tree, id)
	if !ok {
		return nil, fmt.Errorf("collection %s: %w", id, domain.ErrNotFound)
	}
	if parentID != "" {
		if _, ok := findCollection(tree, parentID); !ok {
			return nil, fmt.Errorf("collection %s: %w", parentID, domain.ErrNotFound)
		}
		if _, inside := findCollection(moved.Children, parentID); parentID == id || inside {
			return nil, domain.Refuse(domain.CodeIntoItself, domain.ErrNotAllowed, nil)
		}
	}

	if err := u.store.MoveCollection(ctx, workspace, id, parentID, position); err != nil {
		return nil, err
	}
	return u.store.Collections(ctx, workspace)
}

// Rename is the one edit a tree row takes: the name. What a request is made of is edited in its own
// card and saved from there, so both a collection and a node answer to the same gesture.
func (u *UseCase) Rename(ctx context.Context, id string, name string) ([]domain.Collection, error) {
	name, err := validName(name)
	if err != nil {
		return nil, err
	}

	workspace, err := u.scope.ActiveWorkspace(ctx)
	if err != nil {
		return nil, err
	}
	collection, ok, err := u.collection(ctx, workspace, id)
	if err != nil {
		return nil, err
	}
	if ok {
		collection.Name = name
		if err := u.store.SaveCollection(ctx, workspace, collection); err != nil {
			return nil, err
		}
		return u.store.Collections(ctx, workspace)
	}

	node, err := u.store.Node(ctx, id)
	if err != nil {
		return nil, err
	}
	node.Name = name
	if err := u.store.SaveNode(ctx, node); err != nil {
		return nil, err
	}
	return u.store.Collections(ctx, workspace)
}

// Describe is the other edit a tree row takes: what the level is for. A folder answers it too — a
// collection's description is what the overview draws above its tabs, and a folder has the same line
// in the same place — and both answer with the whole tree, because the tree is where the header reads
// it from.
func (u *UseCase) Describe(ctx context.Context, id string, description string) ([]domain.Collection, error) {
	description, err := validDescription(description)
	if err != nil {
		return nil, err
	}

	workspace, err := u.scope.ActiveWorkspace(ctx)
	if err != nil {
		return nil, err
	}
	if collection, ok, err := u.collection(ctx, workspace, id); err != nil {
		return nil, err
	} else if ok {
		collection.Description = description
		if err := u.store.SaveCollection(ctx, workspace, collection); err != nil {
			return nil, err
		}
		return u.store.Collections(ctx, workspace)
	}

	node, err := u.store.Node(ctx, id)
	if err != nil {
		return nil, err
	}
	node.Description = description
	if err := u.store.SaveNode(ctx, node); err != nil {
		return nil, err
	}
	return u.store.Collections(ctx, workspace)
}

// SaveAuth writes what a level authorizes its requests with — the collection's own tab, or a
// folder's, which everything inside it inherits. «Нет» is stored as nothing at all: a level that
// has no authorization of its own is a level the one below it inherits past, and an empty auth
// written down would stop that walk at the level that meant to say nothing.
func (u *UseCase) SaveAuth(ctx context.Context, id string, auth domain.Auth) ([]domain.Collection, error) {
	// Normalized for the same reason a draft's is: the answers travel whole, and the scheme they are
	// about may not be the one the previous answer was about.
	saved := authOrNil(auth.Normalized())

	workspace, err := u.scope.ActiveWorkspace(ctx)
	if err != nil {
		return nil, err
	}
	if collection, ok, err := u.collection(ctx, workspace, id); err != nil {
		return nil, err
	} else if ok {
		collection.Auth = saved
		if err := u.store.SaveCollection(ctx, workspace, collection); err != nil {
			return nil, err
		}
		return u.store.Collections(ctx, workspace)
	}

	node, err := u.store.Node(ctx, id)
	if err != nil {
		return nil, err
	}
	node.Auth = saved
	if err := u.store.SaveNode(ctx, node); err != nil {
		return nil, err
	}
	return u.store.Collections(ctx, workspace)
}

// AuthFor is what a request inherits: the answer of the nearest level above it that gave one, or
// nothing when none did. The command line's draft is in no tree, and a node that has been deleted
// is in none either — neither is a failure, because "there is nothing above this request" is what
// the answer is, not that the question was wrong.
func (u *UseCase) AuthFor(ctx context.Context, id domain.DraftID) (*domain.Auth, error) {
	workspace, err := u.scope.ActiveWorkspace(ctx)
	if err != nil {
		return nil, err
	}
	tree, err := u.store.Collections(ctx, workspace)
	if err != nil {
		return nil, err
	}
	for _, collection := range tree {
		if inherited, ok := inheritUnder([]domain.Collection{collection}, string(id), nil); ok {
			return authOrNil(actionable(inherited)), nil
		}
	}
	return nil, nil
}

// inheritUnder walks the tree looking for a request, carrying the answer of the levels above it: the
// nearest level that set one wins, and a level that set none passes down what it was given.
//
// A collection is a level like any other, which is why the walk descends into the collections inside
// one and not only into its requests.
func inheritUnder(collections []domain.Collection, id string, inherited *domain.Auth) (*domain.Auth, bool) {
	for _, collection := range collections {
		at := inherited
		if collection.Auth != nil {
			at = collection.Auth
		}
		for _, node := range collection.Items {
			nodeAt := at
			if node.Auth != nil {
				nodeAt = node.Auth
			}
			if node.ID == id {
				return nodeAt, true
			}
		}
		if found, ok := inheritUnder(collection.Children, id, at); ok {
			return found, true
		}
	}
	return nil, false
}

// authOrNil is an auth as the tree stores it: the two answers that are not credentials — «нет» and
// «наследовать» — are the absence of an answer.
func authOrNil(auth domain.Auth) *domain.Auth {
	if auth.Type == domain.AuthNone || auth.Type == domain.AuthInherit || auth.Type == "" {
		return nil
	}
	return &auth
}

// actionable is a stored auth as something to apply: nothing to inherit is «нет», which is what a
// request with nothing above it authorizes itself with.
func actionable(auth *domain.Auth) domain.Auth {
	if auth == nil {
		return domain.Auth{Type: domain.AuthNone}
	}
	return *auth
}

// Duplicate copies a collection or a node into the same place, under a name that says what it is.
// Ids are minted anew: the copy is a second thing, not the same thing twice.
func (u *UseCase) Duplicate(ctx context.Context, id string, suffix string) ([]domain.Collection, error) {
	workspace, err := u.scope.ActiveWorkspace(ctx)
	if err != nil {
		return nil, err
	}
	if collection, ok, err := u.collection(ctx, workspace, id); err != nil {
		return nil, err
	} else if ok {
		return u.duplicateCollection(ctx, workspace, collection, suffix)
	}

	// The copy starts from the tree row and reads the request whole on the way: a copy is the request
	// and not just its name.
	tree, err := u.store.Collections(ctx, workspace)
	if err != nil {
		return nil, err
	}
	node, ok := findNode(tree, id)
	if !ok {
		return nil, fmt.Errorf("node %s: %w", id, domain.ErrNotFound)
	}
	position, err := u.store.NextPosition(ctx, workspace, node.CollectionID)
	if err != nil {
		return nil, err
	}

	copied, err := u.copyNode(ctx, node, node.CollectionID, node.Name+suffix, position)
	if err != nil {
		return nil, err
	}
	if err := u.store.SaveNode(ctx, copied); err != nil {
		return nil, err
	}
	return u.store.Collections(ctx, workspace)
}

// Delete removes a collection or a node. What was inside goes with it through the schema's cascade
// — one statement, so a half-deleted tree is not a state that exists.
func (u *UseCase) Delete(ctx context.Context, id string) ([]domain.Collection, error) {
	workspace, err := u.scope.ActiveWorkspace(ctx)
	if err != nil {
		return nil, err
	}
	if _, ok, err := u.collection(ctx, workspace, id); err != nil {
		return nil, err
	} else if ok {
		if err := u.store.DeleteCollection(ctx, workspace, id); err != nil {
			return nil, err
		}
		return u.store.Collections(ctx, workspace)
	}

	if _, err := u.store.Node(ctx, id); err != nil {
		return nil, err
	}
	if err := u.store.DeleteNode(ctx, id); err != nil {
		return nil, err
	}
	return u.store.Collections(ctx, workspace)
}

// Import writes a collection that was made elsewhere — a Postman file — into the tree, at the end of
// the top level. Ids are minted here because a file has none, and the collection keeps the name, the
// order and everything it came with, nested collections included.
func (u *UseCase) Import(ctx context.Context, collection domain.Collection) ([]domain.Collection, error) {
	name, err := validName(collection.Name)
	if err != nil {
		return nil, err
	}
	workspace, err := u.scope.ActiveWorkspace(ctx)
	if err != nil {
		return nil, err
	}
	position, err := u.store.NextPosition(ctx, workspace, "")
	if err != nil {
		return nil, err
	}

	imported := domain.Collection{
		ID:          u.ids(),
		Name:        name,
		Description: strings.TrimSpace(collection.Description),
		Position:    position,
		Auth:        collection.Auth,
	}

	// The whole subtree is built before any of it is written: an import that failed halfway would
	// leave a collection with half a file in it.
	pending := u.adopt(imported, collection.Items, collection.Children)
	return u.saveTree(ctx, workspace, pending)
}

// adopt gives an imported subtree what a file does not write: ids, and where it lives. Everything
// else — the names, the addresses, the rows — is the file's.
//
// A collection is a level of the file like any other, so it is adopted by this same function one
// level down, with the level it came from carried along for its scripts.
func (u *UseCase) adopt(collection domain.Collection, items []domain.CollectionNode, children []domain.Collection) []pendingLevel {
	adopted := collection
	adopted.Items = []domain.CollectionNode{}
	adopted.Children = []domain.Collection{}

	for _, node := range items {
		node.ID = u.ids()
		node.CollectionID = adopted.ID
		node.Params = withIDs(u.ids, node.Params)
		node.Headers = withIDs(u.ids, node.Headers)
		for i := range node.Cookies {
			if node.Cookies[i].ID == "" {
				node.Cookies[i].ID = u.ids()
			}
		}
		adopted.Items = append(adopted.Items, node)
	}

	out := []pendingLevel{{collection: adopted}}
	for _, child := range children {
		nested := domain.Collection{
			ID:          u.ids(),
			Name:        child.Name,
			Description: child.Description,
			ParentID:    adopted.ID,
			Position:    child.Position,
			Auth:        child.Auth,
		}
		out = append(out, u.adopt(nested, child.Items, child.Children)...)
	}
	return out
}

// Full is a collection with every request read whole, or a single request when the id names one: what
// an export writes down. The tree the window draws carries the method and nothing else — a list of
// two hundred rows has no business carrying two hundred bodies — so an export reads them again.
func (u *UseCase) Full(ctx context.Context, id string) (domain.Collection, error) {
	workspace, err := u.scope.ActiveWorkspace(ctx)
	if err != nil {
		return domain.Collection{}, err
	}
	collection, ok, err := u.collection(ctx, workspace, id)
	if err != nil {
		return domain.Collection{}, err
	}
	if !ok {
		node, err := u.store.Node(ctx, id)
		if err != nil {
			return domain.Collection{}, err
		}
		return domain.Collection{Name: node.Name, Items: []domain.CollectionNode{node}}, nil
	}

	return u.fullCollection(ctx, collection)
}

func (u *UseCase) fullCollection(ctx context.Context, collection domain.Collection) (domain.Collection, error) {
	items, err := u.fullNodes(ctx, collection.Items)
	if err != nil {
		return domain.Collection{}, err
	}
	children := []domain.Collection{}
	for _, child := range collection.Children {
		full, err := u.fullCollection(ctx, child)
		if err != nil {
			return domain.Collection{}, err
		}
		children = append(children, full)
	}
	return domain.Collection{
		Name:        collection.Name,
		Description: collection.Description,
		Position:    collection.Position,
		Auth:        collection.Auth,
		Items:       items,
		Children:    children,
	}, nil
}

func (u *UseCase) fullNodes(ctx context.Context, rows []domain.CollectionNode) ([]domain.CollectionNode, error) {
	out := make([]domain.CollectionNode, 0, len(rows))
	for _, row := range rows {
		node, err := u.store.Node(ctx, row.ID)
		if err != nil {
			return nil, err
		}
		node.Position = row.Position
		out = append(out, node)
	}
	return out, nil
}

// SaveNode writes what the card was editing back into the tree. The node is re-read first: the parts
// the card does not own — where it sits, when it was made — belong to the tree and stay as they are.
func (u *UseCase) SaveNode(ctx context.Context, edited domain.CollectionNode) ([]domain.Collection, error) {
	stored, err := u.store.Node(ctx, edited.ID)
	if err != nil {
		return nil, err
	}

	stored.Name, err = validName(edited.Name)
	if err != nil {
		return nil, err
	}
	stored.Description = strings.TrimSpace(edited.Description)
	stored.Method = defaultMethod(edited.Method)
	stored.URL = edited.URL
	stored.Params = orEmptyRows(edited.Params)
	stored.Headers = orEmptyRows(edited.Headers)
	stored.Body = edited.Body
	stored.Cookies = orEmptyCookies(edited.Cookies)
	stored.Auth = edited.Auth
	if err := u.store.SaveNode(ctx, stored); err != nil {
		return nil, err
	}
	workspace, err := u.scope.ActiveWorkspace(ctx)
	if err != nil {
		return nil, err
	}
	return u.store.Collections(ctx, workspace)
}

// duplicateCollection copies a whole collection into a new one beside it, what is inside it included.
// The tree is read with its request fields left out, so every request is read again on the way in — a
// copy that lost its headers would be worse than no copy at all.
func (u *UseCase) duplicateCollection(ctx context.Context, workspace string, collection domain.Collection, suffix string) ([]domain.Collection, error) {
	position, err := u.store.NextPosition(ctx, workspace, collection.ParentID)
	if err != nil {
		return nil, err
	}

	copied, err := u.copyCollection(ctx, collection, collection.ParentID, position,
		clip(collection.Name+suffix))
	if err != nil {
		return nil, err
	}
	return u.saveTree(ctx, workspace, copied)
}

// copyCollection builds the copy of a whole collection in memory before any of it is written: a
// duplicate that failed halfway would leave a collection with half a tree in it.
//
// What it is given is a tree row — a name, and what is under it — and what it copies is what the
// store holds: the two are not the same thing, and taking the row for the content is how a duplicate
// once came out as an empty request with the right name.
func (u *UseCase) copyCollection(ctx context.Context, row domain.Collection, parentID string, position int64, name string) ([]pendingLevel, error) {
	copied := domain.Collection{
		ID:          u.ids(),
		Name:        name,
		Description: row.Description,
		ParentID:    parentID,
		Position:    position,
		Auth:        row.Auth,
		Items:       []domain.CollectionNode{},
		Children:    []domain.Collection{},
	}

	for _, node := range row.Items {
		child, err := u.copyNode(ctx, node, copied.ID, node.Name, node.Position)
		if err != nil {
			return nil, err
		}
		copied.Items = append(copied.Items, child)
	}

	// The copy is a second level, so what the original ran around its requests goes with it — read at
	// the end, because SaveScripts writes to the row the copy does not have yet.
	out := []pendingLevel{{collection: copied, from: row.ID}}
	for _, child := range row.Children {
		nested, err := u.copyCollection(ctx, child, copied.ID, child.Position, child.Name)
		if err != nil {
			return nil, err
		}
		out = append(out, nested...)
	}
	return out, nil
}

// copyNode builds the copy of one request: the row itself, read whole, with rows of its own.
func (u *UseCase) copyNode(ctx context.Context, row domain.CollectionNode, collectionID string, name string, position int64) (domain.CollectionNode, error) {
	node, err := u.store.Node(ctx, row.ID)
	if err != nil {
		return domain.CollectionNode{}, err
	}
	copied := node
	copied.ID = u.ids()
	copied.CollectionID = collectionID
	copied.Name = name
	copied.Position = position
	// The rows are the copy's own: a row is addressed by its id, and one id naming a row in two
	// requests is one row in two places. The copy is a second thing, rows and all.
	copied.Params = copyRows(u.ids, node.Params)
	copied.Headers = copyRows(u.ids, node.Headers)
	copied.Cookies = copyCookies(u.ids, node.Cookies)
	copied.Form = copyFormRows(u.ids, node.Form)
	return copied, nil
}

// withIDs gives rows ids they do not have. A file writes no ids at all, and a row the window cannot
// address is a row it cannot edit.
func withIDs(ids platform.IDGen, rows []domain.Row) []domain.Row {
	out := make([]domain.Row, 0, len(rows))
	for _, row := range rows {
		if row.ID == "" {
			row.ID = ids()
		}
		out = append(out, row)
	}
	return out
}

// withFormIDs is withIDs for a form body.
func withFormIDs(ids platform.IDGen, rows []domain.FormRow) []domain.FormRow {
	out := make([]domain.FormRow, 0, len(rows))
	for _, row := range rows {
		if row.ID == "" {
			row.ID = ids()
		}
		out = append(out, row)
	}
	return out
}

// copyFormRows is copyRows for a form body, for the same reason.
func copyFormRows(ids platform.IDGen, rows []domain.FormRow) []domain.FormRow {
	out := make([]domain.FormRow, 0, len(rows))
	for _, row := range rows {
		row.ID = ids()
		out = append(out, row)
	}
	return out
}

// copyRows and copyCookies give a copy rows of its own, for the reason the ids exist at all: the
// window edits a row by id, and a copy that kept them would be an edit away from changing both.
func copyRows(ids platform.IDGen, rows []domain.Row) []domain.Row {
	out := make([]domain.Row, 0, len(rows))
	for _, row := range rows {
		row.ID = ids()
		out = append(out, row)
	}
	return out
}

func copyCookies(ids platform.IDGen, cookies []domain.CookieRow) []domain.CookieRow {
	out := make([]domain.CookieRow, 0, len(cookies))
	for _, cookie := range cookies {
		cookie.ID = ids()
		out = append(out, cookie)
	}
	return out
}

// pendingLevel is a collection waiting to be written: its row, and the level it was copied from —
// empty for one that came from a file rather than from the tree.
//
// Scripts are not part of a collection: they belong to the level and are read by id, and SaveScripts
// can only write to a row that already exists. So they travel beside the row rather than inside it,
// and are written the moment the row is.
type pendingLevel struct {
	collection domain.Collection
	from       string
}

// saveTree writes built levels parents first: a row's parent has to exist before it does. The list
// comes pre-order, so the order it is given in is the order it writes in.
func (u *UseCase) saveTree(ctx context.Context, workspace string, pending []pendingLevel) ([]domain.Collection, error) {
	for _, level := range pending {
		if err := u.store.SaveCollection(ctx, workspace, level.collection); err != nil {
			return nil, err
		}
		if level.from != "" {
			if scripts, err := u.store.Scripts(ctx, workspace, level.from); err != nil {
				return nil, err
			} else if scripts != nil {
				if err := u.store.SaveScripts(ctx, workspace, level.collection.ID, scripts); err != nil {
					return nil, err
				}
			}
		}
		for _, node := range level.collection.Items {
			if err := u.store.SaveNode(ctx, node); err != nil {
				return nil, err
			}
		}
	}
	return u.store.Collections(ctx, workspace)
}

// findNode looks a request up in the tree, collections inside collections included: the tree is small
// enough to walk, and walking it is what says whether the id names a request at all.
func findNode(tree []domain.Collection, id string) (domain.CollectionNode, bool) {
	for _, collection := range tree {
		for _, node := range collection.Items {
			if node.ID == id {
				return node, true
			}
		}
		if node, ok := findNode(collection.Children, id); ok {
			return node, true
		}
	}
	return domain.CollectionNode{}, false
}

// collection finds a collection by id and says whether it found one rather than failing: nodes are
// addressed in the same id space, and a caller naming an id does not say which of the two it named.
func (u *UseCase) collection(ctx context.Context, workspace, id string) (domain.Collection, bool, error) {
	tree, err := u.store.Collections(ctx, workspace)
	if err != nil {
		return domain.Collection{}, false, err
	}
	collection, ok := findCollection(tree, id)
	return collection, ok, nil
}

func validName(name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return "", domain.Refuse(domain.CodeNameEmpty, domain.ErrNotAllowed, nil)
	}
	if len([]rune(name)) > maxNameLength {
		return "", domain.Refuse(domain.CodeNameTooLong, domain.ErrNotAllowed, domain.Args{"max": strconv.Itoa(maxNameLength)})
	}
	return name, nil
}

// validDescription trims what was typed and keeps it within the ceiling. An empty description is a
// real answer — the header shows a placeholder for it — so it is not refused the way an empty name is.
func validDescription(description string) (string, error) {
	description = strings.TrimSpace(description)
	if len([]rune(description)) > maxDescriptionLength {
		return "", domain.Refuse(domain.CodeDescriptionTooLong, domain.ErrNotAllowed,
			domain.Args{"max": strconv.Itoa(maxDescriptionLength)})
	}
	return description, nil
}

// clip keeps a name inside the ceiling. Duplicating at the limit is a thing a user does, and
// refusing it would be the app's problem, not theirs.
func clip(name string) string {
	runes := []rune(name)
	if len(runes) <= maxNameLength {
		return name
	}
	return string(runes[:maxNameLength])
}

// defaultMethod is what a request gets when the tree did not name one — the tree offers GET first,
// and a method is never empty on a request that has been sent.
func defaultMethod(method string) string {
	method = strings.TrimSpace(strings.ToUpper(method))
	if method == "" {
		return "GET"
	}
	return method
}

func orEmptyRows(rows []domain.Row) []domain.Row {
	if rows == nil {
		return []domain.Row{}
	}
	return rows
}

func orEmptyCookies(cookies []domain.CookieRow) []domain.CookieRow {
	if cookies == nil {
		return []domain.CookieRow{}
	}
	return cookies
}
