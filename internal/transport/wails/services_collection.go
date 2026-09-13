package wails

import (
	"context"

	"json-inspector/internal/domain"
	"json-inspector/internal/usecase/collection"
)

// CollectionsService is the saved requests. Every call that changes the tree answers with the whole
// tree, because a rename moves a row the list is already drawing — a caller that had to splice the
// change in itself would be a second implementation of the tree's order.
type CollectionsService struct {
	collections *collection.UseCase
}

func NewCollectionsService(uc *collection.UseCase) *CollectionsService {
	return &CollectionsService{collections: uc}
}

func (s *CollectionsService) Tree(ctx context.Context) ([]domain.Collection, error) {
	return s.collections.Tree(ctx)
}

// Node is what opening a saved request needs: a tree row carries its method and nothing else.
func (s *CollectionsService) Node(ctx context.Context, id string) (domain.CollectionNode, error) {
	return s.collections.Node(ctx, id)
}

func (s *CollectionsService) CreateCollection(ctx context.Context, name string, description string) ([]domain.Collection, error) {
	return s.collections.CreateCollection(ctx, name, description)
}

func (s *CollectionsService) CreateNode(ctx context.Context, in collection.NewNode) ([]domain.Collection, error) {
	return s.collections.CreateNode(ctx, in)
}

func (s *CollectionsService) Rename(ctx context.Context, id string, name string) ([]domain.Collection, error) {
	return s.collections.Rename(ctx, id, name)
}

func (s *CollectionsService) Duplicate(ctx context.Context, id string) ([]domain.Collection, error) {
	return s.collections.Duplicate(ctx, id)
}

func (s *CollectionsService) Delete(ctx context.Context, id string) ([]domain.Collection, error) {
	return s.collections.Delete(ctx, id)
}

// SaveNode writes the request the card was editing back into the tree. Sending is a different
// gesture: it does not save, and nothing here is called by it.
func (s *CollectionsService) SaveNode(ctx context.Context, node domain.CollectionNode) ([]domain.Collection, error) {
	return s.collections.SaveNode(ctx, node)
}

// Run starts a collection or a folder and answers with the id of the run. What happens next arrives
// as events: a run of fifty requests outlives the call that started it, and the window draws it as
// it goes.
func (s *CollectionsService) Run(ctx context.Context, collectionID string, nodeID string) (string, error) {
	return s.collections.Run(ctx, collectionID, nodeID)
}

// Stop ends the run after the request that is already in flight.
func (s *CollectionsService) Stop(ctx context.Context) (bool, error) {
	return s.collections.Stop(), nil
}

// LastRun is what the overview draws when it opens, and nil when nothing has been run here yet.
func (s *CollectionsService) LastRun(ctx context.Context, collectionID string, nodeID string) (*domain.CollectionRun, error) {
	return s.collections.LastRun(ctx, collectionID, nodeID)
}
