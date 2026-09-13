package collection

import (
	"context"

	"json-inspector/internal/domain"
)

// Store is the tree as this feature keeps it. The whole thing is read at once — a collection is a
// document, not a list to page through — and written a node at a time.
type Store interface {
	Collections(ctx context.Context) ([]domain.Collection, error)
	Node(ctx context.Context, id string) (domain.CollectionNode, error)

	SaveCollection(ctx context.Context, collection domain.Collection) error
	SaveNode(ctx context.Context, node domain.CollectionNode) error
	DeleteCollection(ctx context.Context, id string) error
	DeleteNode(ctx context.Context, id string) error

	NextPosition(ctx context.Context, collectionID string, parentID string) (int64, error)
}
