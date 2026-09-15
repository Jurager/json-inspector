package search

import (
	"context"

	"json-inspector/internal/domain"
)

// Index is what the search reads. One method per area, and every method answers the same shape, so
// that an area is a line beside the others rather than a branch inside the query.
//
// A search is a projection for reading across the whole database, which is why it is one port here
// instead of a question asked of each feature: a feature that had to answer one would have to grow a
// search of its own, and adding an area would mean touching three packages.
//
// The names start with Find because the same *sqlite.Store implements every port in the app, and it
// already has a method called Collections. One type cannot carry two methods with one name, so before
// an area is added here its name is checked against internal/infra/sqlite.
//
// An area answers with its whole set and is told nothing about the words: matching happens above,
// because SQLite folds case for ASCII alone and the names in this app are Russian. What keeps that
// affordable is the schema, which holds a level as a document and the history to what retention
// leaves.
type Index interface {
	FindRequests(ctx context.Context, workspaceID string) ([]domain.SearchHit, error)
	FindCollections(ctx context.Context, workspaceID string) ([]domain.SearchHit, error)
	FindEnvironments(ctx context.Context, workspaceID string) ([]domain.SearchHit, error)
	FindHistory(ctx context.Context, workspaceID string) ([]domain.SearchHit, error)
}

// Scope answers which workspace the window is showing. It is a port of this feature's own rather than
// a call into the workspace use case: features never import each other, and this one only needs the
// name of the space it is searching in.
type Scope interface {
	ActiveWorkspace(ctx context.Context) (string, error)
}
