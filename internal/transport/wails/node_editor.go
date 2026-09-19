package wails

import (
	"context"

	"json-inspector/internal/domain"
	"json-inspector/internal/usecase/draft"
)

// NodeEditor is assembled in this layer because this is the only one that knows both features —
// collections own the tree, the draft owns the editing — and neither should know the other.
type NodeEditor struct {
	Tree  []domain.Collection   `json:"tree"`
	Node  domain.CollectionNode `json:"node"`
	State draft.State           `json:"state"`
}

// OpenNode puts a saved request into the draft its card edits, and answers with everything the card
// draws. Opening is what a save starts over from as well: a draft that has just been opened has
// nothing unsaved in it, which is where the window's "Unsaved" comes from and goes.
func (s *CollectionsService) OpenNode(ctx context.Context, id string) (NodeEditor, error) {
	node, err := s.collections.Node(ctx, id)
	if err != nil {
		return NodeEditor{}, err
	}
	return s.editor(ctx, node)
}

// SaveNode writes what the card is editing back into the tree. The draft is where the request is —
// its method, its address, its rows, its body — and the node keeps what the card does not own: what
// it is called, where it sits, when it was made.
func (s *CollectionsService) SaveNode(ctx context.Context, id string) (NodeEditor, error) {
	node, err := s.collections.Node(ctx, id)
	if err != nil {
		return NodeEditor{}, err
	}
	edited, err := s.drafts.Current(ctx, domain.DraftID(id))
	if err != nil {
		return NodeEditor{}, err
	}
	if _, err := s.collections.SaveNode(ctx, nodeFromDraft(node, edited)); err != nil {
		return NodeEditor{}, err
	}

	// The node is read again and the draft opened on it, so the answer is the saved request rather
	// than the one that was being edited — and the next edit counts from there.
	saved, err := s.collections.Node(ctx, id)
	if err != nil {
		return NodeEditor{}, err
	}
	return s.editor(ctx, saved)
}

// editor is the answer both calls give: the node, the draft opened on it, and the tree. Opening
// does not go through the draft service, so the state is completed here as well — a card is drawn
// from this one answer, and would otherwise show a request that inherits nothing until it is
// edited.
func (s *CollectionsService) editor(
	ctx context.Context,
	node domain.CollectionNode,
) (NodeEditor, error) {
	state, err := s.drafts.Open(ctx, draftOfNode(node))
	if err != nil {
		return NodeEditor{}, err
	}
	tree, err := s.collections.Tree(ctx)
	if err != nil {
		return NodeEditor{}, err
	}
	return NodeEditor{Tree: tree, Node: node,
		State: completed(ctx, s.drafts, s.collections, state)}, nil
}

// draftOfNode is a saved request seen as something to edit. Rows keep their ids, so an editor that
// is open on one of them stays on it.
func draftOfNode(node domain.CollectionNode) domain.Draft {
	// A node with no auth of its own inherits one, and «Inherit» is how the draft says that: the
	// card draws the choice, and the tree is asked for the answer when the request goes out.
	auth := domain.Auth{Type: domain.AuthInherit}
	if node.Auth != nil {
		auth = *node.Auth
	}
	return domain.Draft{
		ID:            domain.DraftID(node.ID),
		Method:        node.Method,
		URL:           node.URL,
		Params:        domain.OrEmpty(node.Params),
		Headers:       domain.OrEmpty(node.Headers),
		Body:          node.Body,
		BodyKind:      domain.KindOf(node.BodyKind),
		Form:          domain.OrEmpty(node.Form),
		BodyFile:      node.BodyFile,
		Cookies:       domain.OrEmpty(node.Cookies),
		Auth:          auth,
		EnvironmentID: node.EnvironmentID,
	}
}

// nodeFromDraft is the other direction. The two answers that are not credentials are stored as the
// absence of an answer, so a card that never touched the chip cannot turn «Inherit» into «None» —
// the two are different answers, and only one of them was given. What a card was holding
// when it gave one of them stays with it, which is domain.Auth.Stored's job, not this one's.
func nodeFromDraft(node domain.CollectionNode, d domain.Draft) domain.CollectionNode {
	node.Method = d.Method
	node.URL = d.URL
	node.Params = d.Params
	node.Headers = d.Headers
	node.Body = d.Body
	node.BodyKind = domain.KindOf(d.BodyKind)
	node.Form = d.Form
	node.BodyFile = d.BodyFile
	node.Cookies = d.Cookies
	node.Auth = d.Auth.Stored()
	node.EnvironmentID = d.EnvironmentID
	return node
}

