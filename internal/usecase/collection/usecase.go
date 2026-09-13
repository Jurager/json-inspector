// Package collection owns the saved requests: the tree they live in, what inherits from what, and
// what happens when a whole collection is run.
package collection

import (
	"context"
	"fmt"
	"strings"
	"sync/atomic"

	"json-inspector/internal/domain"
	"json-inspector/internal/platform"
)

// copySuffix marks a duplicate for what it is. It is the word the window shows, so it lives here
// rather than in a view that would have to invent its own.
const copySuffix = " (копия)"

// maxNameLength is the same ceiling the environments screen uses, so a name that is too long means
// the same thing wherever it is typed.
const maxNameLength = 120

type UseCase struct {
	store    Store
	sender   Sender
	notifier Notifier
	ids      platform.IDGen

	// A run holds the tree for as long as it lasts, and only one runs at a time: two of them would
	// write their rows into the same overview.
	running atomic.Bool
	stopped atomic.Bool
}

func NewUseCase(store Store, sender Sender, notifier Notifier, ids platform.IDGen) *UseCase {
	return &UseCase{store: store, sender: sender, notifier: notifier, ids: ids}
}

// Tree is every collection with its nodes, which is what the list draws and what a run walks.
func (u *UseCase) Tree(ctx context.Context) ([]domain.Collection, error) {
	return u.store.Collections(ctx)
}

// Node reads one node whole: everything opening a request needs, and the tree deliberately left out.
func (u *UseCase) Node(ctx context.Context, id string) (domain.CollectionNode, error) {
	return u.store.Node(ctx, id)
}

// CreateCollection adds an empty collection at the end of the list. The name is given rather than
// invented: the window asks for it in the tree, in the row the user is looking at.
func (u *UseCase) CreateCollection(ctx context.Context, name string, description string) ([]domain.Collection, error) {
	name, err := validName(name)
	if err != nil {
		return nil, err
	}

	tree, err := u.store.Collections(ctx)
	if err != nil {
		return nil, err
	}
	if err := u.store.SaveCollection(ctx, domain.Collection{
		ID:          u.ids(),
		Name:        name,
		Description: strings.TrimSpace(description),
		Position:    int64(len(tree)),
		Items:       []domain.CollectionNode{},
	}); err != nil {
		return nil, err
	}
	return u.Tree(ctx)
}

// NewNode is what the tree asks for when a row is created: where it goes, what it is, and what is
// already known about the request. A request made by hand arrives with its method and nothing else —
// the rest is filled in by the card that opens next — while one saved from the command line is a
// whole request, and building it empty first would be a node with no address if the second step
// failed.
type NewNode struct {
	CollectionID string          `json:"collectionId"`
	ParentID     string          `json:"parentId,omitempty"`
	Kind         domain.NodeKind `json:"kind"`
	Name         string          `json:"name"`

	Method  string             `json:"method,omitempty"`
	URL     string             `json:"url,omitempty"`
	Params  []domain.Row       `json:"params,omitempty"`
	Headers []domain.Row       `json:"headers,omitempty"`
	Body    string             `json:"body,omitempty"`
	Cookies []domain.CookieRow `json:"cookies,omitempty"`
	Auth    *domain.Auth       `json:"auth,omitempty"`
}

// CreateNode adds a folder or a request to the end of its group. An empty parent means the node
// lives in the collection's own root.
//
// It answers with the row that appeared and the tree it appeared in: the window needs the id Go
// minted, and looking it up by name afterwards would find the older row of the same name.
func (u *UseCase) CreateNode(ctx context.Context, in NewNode) (domain.CollectionNode, []domain.Collection, error) {
	name, err := validName(in.Name)
	if err != nil {
		return domain.CollectionNode{}, nil, err
	}
	if in.Kind != domain.NodeFolder && in.Kind != domain.NodeRequest {
		return domain.CollectionNode{}, nil, fmt.Errorf("вид узла %q: %w", in.Kind, domain.ErrNotAllowed)
	}
	if _, ok, err := u.collection(ctx, in.CollectionID); err != nil {
		return domain.CollectionNode{}, nil, err
	} else if !ok {
		return domain.CollectionNode{}, nil, fmt.Errorf("коллекция %s: %w", in.CollectionID, domain.ErrNotFound)
	}
	// A request can hold nothing, so only a folder is a place: without this a request dropped into
	// another one would be a node the tree can never draw.
	if in.ParentID != "" {
		parent, err := u.store.Node(ctx, in.ParentID)
		if err != nil {
			return domain.CollectionNode{}, nil, err
		}
		if parent.Kind != domain.NodeFolder || parent.CollectionID != in.CollectionID {
			return domain.CollectionNode{}, nil, fmt.Errorf("родитель %s: %w", in.ParentID, domain.ErrNotAllowed)
		}
	}

	position, err := u.store.NextPosition(ctx, in.CollectionID, in.ParentID)
	if err != nil {
		return domain.CollectionNode{}, nil, err
	}

	node := domain.CollectionNode{
		ID:           u.ids(),
		CollectionID: in.CollectionID,
		ParentID:     in.ParentID,
		Kind:         in.Kind,
		Name:         name,
		Position:     position,
	}
	if in.Kind == domain.NodeRequest {
		node.Method = defaultMethod(in.Method)
		node.URL = strings.TrimSpace(in.URL)
		node.Params = orEmptyRows(in.Params)
		node.Headers = orEmptyRows(in.Headers)
		node.Body = in.Body
		node.Cookies = orEmptyCookies(in.Cookies)
		node.Auth = in.Auth
	}
	if err := u.store.SaveNode(ctx, node); err != nil {
		return domain.CollectionNode{}, nil, err
	}
	tree, err := u.Tree(ctx)
	if err != nil {
		return domain.CollectionNode{}, nil, err
	}
	return node, tree, nil
}

// Rename is the one edit a tree row takes: the name. What a request is made of is edited in its own
// card and saved from there, so both a collection and a node answer to the same gesture.
func (u *UseCase) Rename(ctx context.Context, id string, name string) ([]domain.Collection, error) {
	name, err := validName(name)
	if err != nil {
		return nil, err
	}

	collection, ok, err := u.collection(ctx, id)
	if err != nil {
		return nil, err
	}
	if ok {
		collection.Name = name
		if err := u.store.SaveCollection(ctx, collection); err != nil {
			return nil, err
		}
		return u.Tree(ctx)
	}

	node, err := u.store.Node(ctx, id)
	if err != nil {
		return nil, err
	}
	node.Name = name
	if err := u.store.SaveNode(ctx, node); err != nil {
		return nil, err
	}
	return u.Tree(ctx)
}

// Duplicate copies a collection or a node into the same place, under a name that says what it is.
// Ids are minted anew: the copy is a second thing, not the same thing twice.
func (u *UseCase) Duplicate(ctx context.Context, id string) ([]domain.Collection, error) {
	if collection, ok, err := u.collection(ctx, id); err != nil {
		return nil, err
	} else if ok {
		return u.duplicateCollection(ctx, collection)
	}

	// The copy starts from the tree row, which is the only place a node's children are: copyNode
	// reads each node whole on the way, so a copy is the request and not just its name.
	tree, err := u.store.Collections(ctx)
	if err != nil {
		return nil, err
	}
	node, ok := findNode(tree, id)
	if !ok {
		return nil, fmt.Errorf("узел %s: %w", id, domain.ErrNotFound)
	}
	position, err := u.store.NextPosition(ctx, node.CollectionID, node.ParentID)
	if err != nil {
		return nil, err
	}

	copied, err := u.copyNode(ctx, node, node.CollectionID, node.ParentID, node.Name+copySuffix, position)
	if err != nil {
		return nil, err
	}
	if err := u.saveTree(ctx, copied); err != nil {
		return nil, err
	}
	return u.Tree(ctx)
}

// Delete removes a collection or a node. What was inside goes with it through the schema's cascade
// — one statement, so a half-deleted tree is not a state that exists.
func (u *UseCase) Delete(ctx context.Context, id string) ([]domain.Collection, error) {
	if _, ok, err := u.collection(ctx, id); err != nil {
		return nil, err
	} else if ok {
		if err := u.store.DeleteCollection(ctx, id); err != nil {
			return nil, err
		}
		return u.Tree(ctx)
	}

	if _, err := u.store.Node(ctx, id); err != nil {
		return nil, err
	}
	if err := u.store.DeleteNode(ctx, id); err != nil {
		return nil, err
	}
	return u.Tree(ctx)
}

// Import writes a collection that was made elsewhere — a Postman file — into the tree, at the end of
// the list. Ids are minted here because a file has none, and the collection keeps the name, the
// order and the requests it came with.
func (u *UseCase) Import(ctx context.Context, collection domain.Collection) ([]domain.Collection, error) {
	name, err := validName(collection.Name)
	if err != nil {
		return nil, err
	}
	tree, err := u.store.Collections(ctx)
	if err != nil {
		return nil, err
	}

	imported := domain.Collection{
		ID:          u.ids(),
		Name:        name,
		Description: strings.TrimSpace(collection.Description),
		Position:    int64(len(tree)),
	}

	// The subtree is built before any of it is written: an import that failed halfway would leave a
	// collection with half a file in it.
	roots := make([]domain.CollectionNode, 0, len(collection.Items))
	for i, item := range collection.Items {
		roots = append(roots, u.adopt(item, imported.ID, "", int64(i)))
	}

	if err := u.store.SaveCollection(ctx, imported); err != nil {
		return nil, err
	}
	for _, root := range roots {
		if err := u.saveTree(ctx, root); err != nil {
			return nil, err
		}
	}
	return u.Tree(ctx)
}

// adopt gives an imported subtree what a file does not write: ids, and where it lives. Everything
// else — the names, the addresses, the rows — is the file's.
func (u *UseCase) adopt(node domain.CollectionNode, collectionID string, parentID string, position int64) domain.CollectionNode {
	adopted := node
	adopted.ID = u.ids()
	adopted.CollectionID = collectionID
	adopted.ParentID = parentID
	adopted.Position = position
	adopted.Items = nil
	adopted.Params = withIDs(u.ids, node.Params)
	adopted.Headers = withIDs(u.ids, node.Headers)
	for i := range adopted.Cookies {
		if adopted.Cookies[i].ID == "" {
			adopted.Cookies[i].ID = u.ids()
		}
	}

	for i, child := range node.Items {
		adopted.Items = append(adopted.Items, u.adopt(child, collectionID, adopted.ID, int64(i)))
	}
	return adopted
}

// Full is a collection with every node read whole, or a single request when the id names one: what an
// export writes down. The tree the window draws carries the method and nothing else — a list of two
// hundred rows has no business carrying two hundred bodies — so an export reads them again.
func (u *UseCase) Full(ctx context.Context, id string) (domain.Collection, error) {
	collection, ok, err := u.collection(ctx, id)
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

	items, err := u.fullNodes(ctx, collection.Items)
	if err != nil {
		return domain.Collection{}, err
	}
	return domain.Collection{Name: collection.Name, Description: collection.Description, Items: items}, nil
}

func (u *UseCase) fullNodes(ctx context.Context, rows []domain.CollectionNode) ([]domain.CollectionNode, error) {
	out := make([]domain.CollectionNode, 0, len(rows))
	for _, row := range rows {
		node, err := u.store.Node(ctx, row.ID)
		if err != nil {
			return nil, err
		}
		node.Position = row.Position
		if row.Kind == domain.NodeFolder {
			node.Items, err = u.fullNodes(ctx, row.Items)
			if err != nil {
				return nil, err
			}
		}
		out = append(out, node)
	}
	return out, nil
}

// SaveNode writes what the card was editing back into the tree. The node is re-read first: the
// parts the card does not own — where it sits, what kind it is, when it was made — belong to the
// tree and stay as they are.
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
	if stored.Kind == domain.NodeRequest {
		stored.Method = defaultMethod(edited.Method)
		stored.URL = edited.URL
		stored.Params = orEmptyRows(edited.Params)
		stored.Headers = orEmptyRows(edited.Headers)
		stored.Body = edited.Body
		stored.Cookies = orEmptyCookies(edited.Cookies)
		stored.Auth = edited.Auth
	}
	if err := u.store.SaveNode(ctx, stored); err != nil {
		return nil, err
	}
	return u.Tree(ctx)
}

// duplicateCollection copies a whole collection into a new one at the end of the list. The tree is
// read with its request fields left out, so every node is read again on the way in — a copy of a
// request that lost its headers would be worse than no copy at all.
func (u *UseCase) duplicateCollection(ctx context.Context, collection domain.Collection) ([]domain.Collection, error) {
	tree, err := u.store.Collections(ctx)
	if err != nil {
		return nil, err
	}

	copied := domain.Collection{
		ID:          u.ids(),
		Name:        clip(collection.Name + copySuffix),
		Description: collection.Description,
		Position:    int64(len(tree)),
		Items:       []domain.CollectionNode{},
	}

	roots := make([]domain.CollectionNode, 0, len(collection.Items))
	for i, node := range collection.Items {
		child, err := u.copyNode(ctx, node, copied.ID, "", node.Name, int64(i))
		if err != nil {
			return nil, err
		}
		roots = append(roots, child)
	}

	if err := u.store.SaveCollection(ctx, copied); err != nil {
		return nil, err
	}
	// A copy behaves the way the original did, so what the collection runs around its requests goes
	// with it. The tree carries no scripts — they belong to the level, not to the row — so they are
	// read from the original and written to the copy rather than travelling in the struct.
	if scripts, err := u.store.Scripts(ctx, collection.ID); err != nil {
		return nil, err
	} else if scripts != nil {
		if err := u.store.SaveScripts(ctx, copied.ID, scripts); err != nil {
			return nil, err
		}
	}
	for _, root := range roots {
		if err := u.saveTree(ctx, root); err != nil {
			return nil, err
		}
	}
	return u.Tree(ctx)
}

// copyNode builds the copy of a subtree in memory before any of it is written: a duplicate that
// failed halfway would leave a folder with half its requests in it.
//
// What it is given is a tree row — a name, a method and what is under it — and what it copies is the
// node the store holds. The two are not the same thing, and taking the row for the content is how a
// duplicate once came out as an empty request with the right name.
func (u *UseCase) copyNode(ctx context.Context, row domain.CollectionNode, collectionID string, parentID string, name string, position int64) (domain.CollectionNode, error) {
	node, err := u.store.Node(ctx, row.ID)
	if err != nil {
		return domain.CollectionNode{}, err
	}
	copied := node
	copied.ID = u.ids()
	copied.CollectionID = collectionID
	copied.ParentID = parentID
	copied.Name = name
	copied.Position = position
	copied.Items = nil
	// The rows are the copy's own: a row is addressed by its id, and one id naming a row in two
	// requests is one row in two places. The copy is a second thing, rows and all.
	copied.Params = copyRows(u.ids, node.Params)
	copied.Headers = copyRows(u.ids, node.Headers)
	copied.Cookies = copyCookies(u.ids, node.Cookies)

	for i, child := range row.Items {
		child, err := u.copyNode(ctx, child, collectionID, copied.ID, child.Name, int64(i))
		if err != nil {
			return domain.CollectionNode{}, err
		}
		copied.Items = append(copied.Items, child)
	}
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

// saveTree writes a copied subtree depth first: a child's parent has to exist before it does.
func (u *UseCase) saveTree(ctx context.Context, node domain.CollectionNode) error {
	children := node.Items
	node.Items = nil
	if err := u.store.SaveNode(ctx, node); err != nil {
		return err
	}
	for _, child := range children {
		if err := u.saveTree(ctx, child); err != nil {
			return err
		}
	}
	return nil
}

// findNode looks a node up in the tree, which is where its children are. The tree is small enough
// to walk, and walking it is what says whether the id names a node at all.
func findNode(tree []domain.Collection, id string) (domain.CollectionNode, bool) {
	var walk func([]domain.CollectionNode) (domain.CollectionNode, bool)
	walk = func(nodes []domain.CollectionNode) (domain.CollectionNode, bool) {
		for _, node := range nodes {
			if node.ID == id {
				return node, true
			}
			if found, ok := walk(node.Items); ok {
				return found, true
			}
		}
		return domain.CollectionNode{}, false
	}
	for _, collection := range tree {
		if found, ok := walk(collection.Items); ok {
			return found, true
		}
	}
	return domain.CollectionNode{}, false
}

// collection finds a collection by id and says whether it found one rather than failing: nodes are
// addressed in the same id space, and a caller naming an id does not say which of the two it named.
func (u *UseCase) collection(ctx context.Context, id string) (domain.Collection, bool, error) {
	tree, err := u.store.Collections(ctx)
	if err != nil {
		return domain.Collection{}, false, err
	}
	collection, ok := findCollection(tree, id)
	return collection, ok, nil
}

func validName(name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return "", fmt.Errorf("имя не может быть пустым: %w", domain.ErrNotAllowed)
	}
	if len([]rune(name)) > maxNameLength {
		return "", fmt.Errorf("имя длиннее %d символов: %w", maxNameLength, domain.ErrNotAllowed)
	}
	return name, nil
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
