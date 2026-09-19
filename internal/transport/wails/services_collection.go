package wails

import (
	"context"

	"json-inspector/internal/domain"
	"json-inspector/internal/usecase/collection"
	"json-inspector/internal/usecase/draft"
)

// CollectionsService is the saved requests. Every call that changes the tree answers with the whole
// tree — a rename moves a row the list is already drawing. It holds the draft too, because a saved
// request is edited as one.
type CollectionsService struct {
	collections *collection.UseCase
	drafts      *draft.UseCase
	host        *Host
}

func NewCollectionsService(
	uc *collection.UseCase,
	drafts *draft.UseCase,
	host *Host,
) *CollectionsService {
	return &CollectionsService{collections: uc, drafts: drafts, host: host}
}

func (s *CollectionsService) Tree(ctx context.Context) ([]domain.Collection, error) {
	return s.collections.Tree(ctx)
}

// Node is what opening a saved request needs: a tree row carries its method and nothing else.
func (s *CollectionsService) Node(ctx context.Context, id string) (domain.CollectionNode, error) {
	return s.collections.Node(ctx, id)
}

// Contents is what the collection page's table draws: the requests inside the collection that is
// open, folders and all, with the address each of them goes to and the folder it sits in. The tree
// carries no request payload, so the page asks for its own rows rather than making every tree read
// heavier for one screen.
func (s *CollectionsService) Contents(ctx context.Context, id string) ([]domain.LevelRow, error) {
	return s.collections.Contents(ctx, id)
}

// CreatedCollection is what making a collection answers with, and it is CreatedNode's shape for
// CreatedNode's reason: the id has to come back from Go, because a lookup by name would find the
// older row of the same name — and a folder is created inside another one as often as at the top.
func (s *CollectionsService) CreateCollection(
	ctx context.Context,
	name string,
	description string,
	parentID string,
) (CreatedCollection, error) {
	collection, tree, err := s.collections.CreateCollection(ctx, name, description, parentID)
	if err != nil {
		return CreatedCollection{}, err
	}
	return CreatedCollection{Collection: collection, Tree: tree}, nil
}

// CreatedNode is what a creation answers with: the row that appeared and the tree it appeared in.
// The id has to come back from Go — a lookup by name would find the older row of the same name.
type CreatedNode struct {
	Node domain.CollectionNode `json:"node"`
	Tree []domain.Collection   `json:"tree"`
}

// CreatedCollection is CreatedNode for a collection rather than a request.
type CreatedCollection struct {
	Collection domain.Collection   `json:"collection"`
	Tree       []domain.Collection `json:"tree"`
}

func (s *CollectionsService) CreateNode(
	ctx context.Context,
	in collection.NodeDraft,
) (CreatedNode, error) {
	node, tree, err := s.collections.CreateNode(ctx, in)
	if err != nil {
		return CreatedNode{}, err
	}
	return CreatedNode{Node: node, Tree: tree}, nil
}

// SaveDraft copies what the command line is composing into a collection as a new request. The
// draft is not touched: saving a copy is not a move. The request arrives whole, because the node
// is written once — a second call filling in an empty one could leave it saved with no address.
func (s *CollectionsService) SaveDraft(
	ctx context.Context,
	collectionID string,
	name string,
) (CreatedNode, error) {
	draft, err := s.drafts.Current(ctx, domain.DraftCommandLine)
	if err != nil {
		return CreatedNode{}, err
	}

	node, tree, err := s.collections.CreateNode(ctx, collection.NodeDraft{
		CollectionID:  collectionID,
		Name:          name,
		Method:        draft.Method,
		URL:           draft.URL,
		Params:        draft.Params,
		Headers:       draft.Headers,
		Body:          draft.Body,
		BodyKind:      draft.BodyKind,
		Form:          draft.Form,
		BodyFile:      draft.BodyFile,
		Cookies:       draft.Cookies,
		Auth:          &draft.Auth,
		EnvironmentID: draft.EnvironmentID,
	})
	if err != nil {
		return CreatedNode{}, err
	}
	return CreatedNode{Node: node, Tree: tree}, nil
}

// Describe writes what a collection is for — the line the overview draws above its tabs. Empty is
// an answer there: the header then shows the placeholder that invites one.
func (s *CollectionsService) Describe(
	ctx context.Context,
	id string,
	description string,
) ([]domain.Collection, error) {
	return s.collections.Describe(ctx, id, description)
}

// SaveAuth writes what a collection authorizes its requests with. «None» is the same call with an
// empty auth: a level that has none is a level the ones below it inherit past, and the overview
// then draws the tab the way a collection without one looks.
func (s *CollectionsService) SaveAuth(
	ctx context.Context,
	id string,
	auth domain.Auth,
) ([]domain.Collection, error) {
	return s.collections.SaveAuth(ctx, id, auth)
}

// SaveVariables writes the `{{tokens}}` a collection answers for the requests inside it. The set
// arrives whole, as the editor holds it; the tree comes back because the panel draws these levels.
func (s *CollectionsService) SaveVariables(
	ctx context.Context,
	id string,
	variables []domain.Variable,
) ([]domain.Collection, error) {
	return s.collections.SaveVariables(ctx, id, variables)
}

func (s *CollectionsService) Rename(
	ctx context.Context,
	id string,
	name string,
) ([]domain.Collection, error) {
	return s.collections.Rename(ctx, id, name)
}

// Duplicate copies a node under a name the window composes: the suffix that says what the copy is
// is a word, and words belong to the side that knows the language.
func (s *CollectionsService) Duplicate(
	ctx context.Context,
	id string,
	suffix string,
) ([]domain.Collection, error) {
	return s.collections.Duplicate(ctx, id, suffix)
}

// MoveNode and MoveCollection are what a drop in the tree calls. The position is an index in the
// level the row was dropped into, counted the way the window drew it — the requests of a collection
// and the collections inside it are one list on screen and one number line here.
func (s *CollectionsService) MoveNode(
	ctx context.Context,
	id string,
	collectionID string,
	position int64,
) ([]domain.Collection, error) {
	return s.collections.MoveNode(ctx, id, collectionID, position)
}

func (s *CollectionsService) MoveCollection(
	ctx context.Context,
	id string,
	parentID string,
	position int64,
) ([]domain.Collection, error) {
	return s.collections.MoveCollection(ctx, id, parentID, position)
}

func (s *CollectionsService) Delete(ctx context.Context, id string) ([]domain.Collection, error) {
	return s.collections.Delete(ctx, id)
}

// Run starts a collection or a folder and answers with the id of the run. What happens next arrives
// as events: a run of fifty requests outlives the call that started it, and the window draws it as
// it goes.
func (s *CollectionsService) Run(
	ctx context.Context,
	collectionID string,
	nodeID string,
) (string, error) {
	return s.collections.Run(ctx, collectionID, nodeID)
}

// Stop ends the run after the request that is already in flight.
func (s *CollectionsService) Stop(ctx context.Context) (bool, error) {
	return s.collections.Stop(), nil
}

// LastRun is what the overview draws when it opens, and nil when nothing has been run here yet.
func (s *CollectionsService) LastRun(
	ctx context.Context,
	collectionID string,
	nodeID string,
) (*domain.CollectionRun, error) {
	return s.collections.LastRun(ctx, collectionID, nodeID)
}
