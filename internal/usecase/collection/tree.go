package collection

import (
	"context"
	"fmt"
	"strings"

	"json-inspector/internal/domain"
)

// CreateCollection adds an empty collection at the end of the top level. The name is given rather
// than invented: the window asks for it in the tree, in the row the user is looking at.
func (u *UseCase) CreateCollection(
	ctx context.Context,
	name string,
	description string,
) ([]domain.Collection, error) {
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

// NodeDraft is what the tree asks for when a row is created: where it goes and what is already
// known about the request. A request made by hand arrives with its method and nothing else — the
// rest is filled in by the card that opens next — while one saved from the command line is a whole
// request, and building it empty first would be a node with no address if the second step failed.
type NodeDraft struct {
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
func (u *UseCase) CreateNode(
	ctx context.Context,
	in NodeDraft,
) (domain.CollectionNode, []domain.Collection, error) {
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
		return domain.CollectionNode{}, nil, fmt.Errorf("collection %s: %w", in.CollectionID,
			domain.ErrNotFound)
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
		Params:       domain.OrEmpty(in.Params),
		Headers:      domain.OrEmpty(in.Headers),
		Body:         in.Body,
		BodyKind:     domain.KindOf(in.BodyKind),
		Form:         withFormIDs(u.ids, in.Form),
		BodyFile:     in.BodyFile,
		Cookies:      domain.OrEmpty(in.Cookies),
		// The auth is stored the way the tree keeps one: «None» with nothing behind it is a level
		// nobody has answered anything at, and storing it as a value would make a request saved from
		// the command line stop inheriting — which is not what «None» means there, where nothing is
		// above it and the chip offers no «Inherit» to pick instead.
		Auth: storedAuth(in.Auth),
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

// MoveNode puts a request where it was dropped: another collection, or another place in its own.
// The position counts the level the way the tree draws it — a collection's requests and the
// collections inside it are one list — so the window can name the row it was dropped after.
func (u *UseCase) MoveNode(
	ctx context.Context,
	id string,
	collectionID string,
	position int64,
) ([]domain.Collection, error) {
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

// MoveCollection puts a collection inside another, or back at the top level when the parent is
// empty.
//
// A collection dropped into itself or into one of its own children is refused rather than stored:
// the tree would be a ring, and a ring is a tree nothing can be drawn from.
func (u *UseCase) MoveCollection(
	ctx context.Context,
	id string,
	parentID string,
	position int64,
) ([]domain.Collection, error) {
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
// collection's description is what the overview draws above its tabs, and a folder has the same
// line in the same place — and both answer with the whole tree, because the tree is where the
// header reads it from.
func (u *UseCase) Describe(
	ctx context.Context,
	id string,
	description string,
) ([]domain.Collection, error) {
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

// SaveNode writes what the card was editing back into the tree. The node is re-read first: the
// parts the card does not own — where it sits, when it was made — belong to the tree and stay as
// they are.
func (u *UseCase) SaveNode(
	ctx context.Context,
	edited domain.CollectionNode,
) ([]domain.Collection, error) {
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
	stored.Params = domain.OrEmpty(edited.Params)
	stored.Headers = domain.OrEmpty(edited.Headers)
	stored.Body = edited.Body
	// What the body was composed as travels with the text: a form and a file are fields of their own
	// in the model, and a save that wrote the text without them would turn a multipart request back
	// into raw text the next time it was opened.
	stored.BodyKind = domain.KindOf(edited.BodyKind)
	stored.Form = domain.OrEmpty(edited.Form)
	stored.BodyFile = edited.BodyFile
	stored.Cookies = domain.OrEmpty(edited.Cookies)
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

// defaultMethod is what a request gets when the tree did not name one — the tree offers GET first,
// and a method is never empty on a request that has been sent.
func defaultMethod(method string) string {
	method = strings.TrimSpace(strings.ToUpper(method))
	if method == "" {
		return "GET"
	}
	return method
}
