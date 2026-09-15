package collection

// Duplicating a subtree. A copy is written as a whole level at a time, because a request is
// read whole on the way in: what the list draws is shallow, and a copy of that would have no
// address and no headers.

import (
	"context"
	"fmt"

	"json-inspector/internal/domain"
	"json-inspector/internal/platform"
)

// Duplicate copies a collection or a node into the same place, under a name that says what it is.
// Ids are minted anew: the copy is a second thing, not the same thing twice.
func (u *UseCase) Duplicate(
	ctx context.Context,
	id string,
	suffix string,
) ([]domain.Collection, error) {
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

// duplicateCollection copies a whole collection into a new one beside it, what is inside it
// included. The tree is read with its request fields left out, so every request is read again on
// the way in — a copy that lost its headers would be worse than no copy at all.
func (u *UseCase) duplicateCollection(
	ctx context.Context,
	workspace string,
	collection domain.Collection,
	suffix string,
) ([]domain.Collection, error) {
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
// store holds: the two are not the same thing, and taking the row for the content is how a
// duplicate once came out as an empty request with the right name.
func (u *UseCase) copyCollection(
	ctx context.Context,
	row domain.Collection,
	parentID string,
	position int64,
	name string,
) ([]pendingLevel, error) {
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
func (u *UseCase) copyNode(
	ctx context.Context,
	row domain.CollectionNode,
	collectionID string,
	name string,
	position int64,
) (domain.CollectionNode, error) {
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
// Scripts are not part of a collection: they belong to the level and are read by id, and
// SaveScripts can only write to a row that already exists. So they travel beside the row rather
// than inside it, and are written the moment the row is.
type pendingLevel struct {
	collection domain.Collection
	from       string
}

// saveTree writes built levels parents first: a row's parent has to exist before it does. The list
// comes pre-order, so the order it is given in is the order it writes in.
func (u *UseCase) saveTree(
	ctx context.Context,
	workspace string,
	pending []pendingLevel,
) ([]domain.Collection, error) {
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
