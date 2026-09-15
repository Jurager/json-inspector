// Package search owns the one index over everything the app keeps, and answers what the palette
// draws when the user types into it.
package search

import (
	"context"
	"fmt"
	"strings"

	"json-inspector/internal/domain"
)

type UseCase struct {
	index Index
	scope Scope
}

func NewUseCase(index Index, scope Scope) *UseCase {
	return &UseCase{index: index, scope: scope}
}

// Query is what the palette asks: the words typed, and the one area the user narrowed to.
//
// No area is nil rather than an empty word, because that is what it is: a kind this build does not
// know is a kind a newer window asked for, and it should search nothing rather than everything.
//
// The rows a group draws are not a number here: how many fit under one heading is the answer's own
// business, and the window should not have to know it.
type Query struct {
	Text string             `json:"text"`
	Kind *domain.SearchKind `json:"kind"`
}

// Find searches every area at once or the one that was asked for. It is not called Query: the
// name would repeat the argument's own type, and the type is the better half of the two.
//
// The workspace is resolved once, here, and travels into every area as a parameter: an area that
// resolved it again could answer about a space the window has already left.
func (u *UseCase) Find(ctx context.Context, in Query) (domain.SearchResult, error) {
	workspace, err := u.scope.ActiveWorkspace(ctx)
	if err != nil {
		return domain.SearchResult{}, err
	}

	needle := strings.TrimSpace(in.Text)
	narrowed := in.Kind != nil
	limit := perGroupAll
	if narrowed {
		limit = perGroupOne
	}

	result := domain.SearchResult{Groups: []domain.SearchGroup{}}
	for _, area := range u.areas() {
		if narrowed && area.kind != *in.Kind {
			continue
		}
		// An empty field is not "everything": over every area the palette offers what the user was doing
		// last, and only the history keeps a time. Under a chosen chip the empty field means every row,
		// because the chip says "show me these".
		if needle == "" && !narrowed && area.kind != domain.SearchHistory {
			continue
		}

		hits, err := area.find(ctx, workspace)
		if err != nil {
			return domain.SearchResult{}, fmt.Errorf("searching %s: %w", area.kind, err)
		}
		rows := matched(needle, hits)
		if len(rows) == 0 {
			continue
		}
		ordered, total := rank(rows, limit)
		result.Groups = append(result.Groups,
			domain.SearchGroup{Kind: area.kind, Total: total, Hits: ordered})
	}
	return result, nil
}

// area is one entry of the index: the kind of row it answers with, and where to ask for it. It is a
// method value rather than a branch so that adding an area is a line beside the others.
type area struct {
	kind domain.SearchKind
	find func(ctx context.Context, workspaceID string) ([]domain.SearchHit, error)
}

// The areas, in the order the design draws them.
func (u *UseCase) areas() []area {
	return []area{
		{domain.SearchRequest, u.index.FindRequests},
		{domain.SearchCollection, u.index.FindCollections},
		{domain.SearchEnvironment, u.index.FindEnvironments},
		{domain.SearchHistory, u.index.FindHistory},
	}
}
