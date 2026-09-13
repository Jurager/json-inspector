package wails

import (
	"context"

	"json-inspector/internal/domain"
	"json-inspector/internal/usecase/collection"
	"json-inspector/internal/usecase/draft"
)

// CollectionsService is the saved requests. Every call that changes the tree answers with the whole
// tree, because a rename moves a row the list is already drawing — a caller that had to splice the
// change in itself would be a second implementation of the tree's order.
// It holds the draft as well, and this is one of the two places that does: a saved request is
// edited as a draft, and the layer that knows both features is the one that can put them together.
type CollectionsService struct {
	collections *collection.UseCase
	drafts      *draft.UseCase
	host        *Host
}

func NewCollectionsService(uc *collection.UseCase, drafts *draft.UseCase, host *Host) *CollectionsService {
	return &CollectionsService{collections: uc, drafts: drafts, host: host}
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

// CreatedNode is what a creation answers with: the row that appeared and the tree it appeared in.
// The window needs the id Go minted — looking it up by name afterwards would find the older row of
// the same name — and the tree is what every other change to the tree answers with.
type CreatedNode struct {
	Node domain.CollectionNode `json:"node"`
	Tree []domain.Collection   `json:"tree"`
}

func (s *CollectionsService) CreateNode(ctx context.Context, in collection.NewNode) (CreatedNode, error) {
	node, tree, err := s.collections.CreateNode(ctx, in)
	if err != nil {
		return CreatedNode{}, err
	}
	return CreatedNode{Node: node, Tree: tree}, nil
}

// SaveDraft copies what the command line is composing into a collection as a new request. The draft
// is not touched: saving a copy is not a move, and what is being composed stays where it is.
//
// The request arrives whole — method, address, rows, body, and the auth the chip chose — because the
// node is written once: an empty request filled in by a second call would be a saved request with no
// address if that call failed.
func (s *CollectionsService) SaveDraft(ctx context.Context, collectionID string, parentID string, name string) (CreatedNode, error) {
	draft, err := s.drafts.Current(ctx, domain.DraftCommandLine)
	if err != nil {
		return CreatedNode{}, err
	}

	node, tree, err := s.collections.CreateNode(ctx, collection.NewNode{
		CollectionID: collectionID,
		ParentID:     parentID,
		Kind:         domain.NodeRequest,
		Name:         name,
		Method:       draft.Method,
		URL:          draft.URL,
		Params:       draft.Params,
		Headers:      draft.Headers,
		Body:         draft.Body,
		Cookies:      draft.Cookies,
		Auth:         &draft.Auth,
	})
	if err != nil {
		return CreatedNode{}, err
	}
	return CreatedNode{Node: node, Tree: tree}, nil
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
