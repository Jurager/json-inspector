package collection

// A collection going out as a file and coming back as one. The tree a list draws is shallow, so
// what leaves is read whole first — an export of what the window holds would be an export of names.

import (
	"context"
	"strings"

	"json-inspector/internal/domain"
)

// Import writes a collection that was made elsewhere — a Postman file — into the tree, at the end
// of the top level. Ids are minted here because a file has none, and the collection keeps the name,
// the order and everything it came with, nested collections included.
func (u *UseCase) Import(
	ctx context.Context,
	collection domain.Collection,
) ([]domain.Collection, error) {
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
func (u *UseCase) adopt(
	collection domain.Collection,
	items []domain.CollectionNode,
	children []domain.Collection,
) []pendingLevel {
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

// Full is a collection with every request read whole, or a single request when the id names one:
// what an export writes down. The tree the window draws carries the method and nothing else — a
// list of two hundred rows has no business carrying two hundred bodies — so an export reads them
// again.
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

func (u *UseCase) fullCollection(
	ctx context.Context,
	collection domain.Collection,
) (domain.Collection, error) {
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

func (u *UseCase) fullNodes(
	ctx context.Context,
	rows []domain.CollectionNode,
) ([]domain.CollectionNode, error) {
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
