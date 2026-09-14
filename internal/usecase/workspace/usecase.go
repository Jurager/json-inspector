// Package workspace owns the spaces the app keeps its things in: which ones exist, which one is on
// screen, and what a new one starts as.
package workspace

import (
	"context"
	"strconv"
	"strings"
	"time"

	"json-inspector/internal/domain"
	"json-inspector/internal/platform"
)

// maxNameLength is the design's limit for a workspace's name: long enough for "Каталог внутренних
// API", short enough to stay one row in the switcher.
const maxNameLength = 40

// TopicChanged is published whenever the workspace on screen moves, so that every window redraws
// around the same one.
const TopicChanged = "workspace:changed"

type UseCase struct {
	store    Store
	ids      platform.IDGen
	notifier Notifier
}

func NewUseCase(store Store, ids platform.IDGen, notifier Notifier) *UseCase {
	return &UseCase{store: store, ids: ids, notifier: notifier}
}

// Snapshot is every workspace and the pointer to the one on screen — the whole of what the switcher
// draws, in one answer.
func (u *UseCase) Snapshot(ctx context.Context) (domain.WorkspaceState, error) {
	list, err := u.store.Workspaces(ctx)
	if err != nil {
		return domain.WorkspaceState{}, err
	}
	active, err := u.store.ActiveWorkspace(ctx)
	if err != nil {
		return domain.WorkspaceState{}, err
	}
	return domain.WorkspaceState{Workspaces: list, ActiveID: active}, nil
}

// Switch points the app at another workspace. Nothing is asked first and nothing is confirmed: the
// data below the window changes, which is what the switcher is for.
func (u *UseCase) Switch(ctx context.Context, id string) (domain.WorkspaceState, error) {
	target, err := u.store.Workspace(ctx, id)
	if err != nil {
		return domain.WorkspaceState{}, err
	}
	if err := u.store.SetActiveWorkspace(ctx, target.ID); err != nil {
		return domain.WorkspaceState{}, err
	}
	u.notifier.Publish(TopicChanged, Changed{Workspace: target})
	return u.Snapshot(ctx)
}

// CreateInput is what the create card sends. There is no kind: a workspace made here is a personal
// one — the schema has told the two apart from the start, and the form that makes a team is the one
// that will also have to invite its first members.
type CreateInput struct {
	Name  string `json:"name"`
	Color string `json:"color"`
}

// Create makes a workspace and leaves the app where it was: a new space is empty, and moving the
// window into it without being asked would look like the app had lost its data. The switcher's
// caller switches explicitly — which is what the design's menu does.
func (u *UseCase) Create(ctx context.Context, in CreateInput) (domain.WorkspaceState, error) {
	name, err := cleanName(in.Name)
	if err != nil {
		return domain.WorkspaceState{}, err
	}

	made := domain.NewWorkspace(u.ids(), name, domain.WorkspacePersonal,
		strings.TrimSpace(in.Color), time.Now().UnixMilli())
	if err := u.store.SaveWorkspace(ctx, made); err != nil {
		return domain.WorkspaceState{}, err
	}
	return u.Snapshot(ctx)
}

// Patch is a partial change: a nil field is left as it is.
type Patch struct {
	Name  *string `json:"name,omitempty"`
	Color *string `json:"color,omitempty"`
}

func (u *UseCase) Update(ctx context.Context, id string, patch Patch) (domain.WorkspaceState, error) {
	current, err := u.store.Workspace(ctx, id)
	if err != nil {
		return domain.WorkspaceState{}, err
	}

	if patch.Name != nil {
		name, err := cleanName(*patch.Name)
		if err != nil {
			return domain.WorkspaceState{}, err
		}
		current.Name = name
	}
	if patch.Color != nil {
		current.Color = strings.TrimSpace(*patch.Color)
	}
	current.UpdatedAt = time.Now().UnixMilli()

	if err := u.store.SaveWorkspace(ctx, current); err != nil {
		return domain.WorkspaceState{}, err
	}
	if active, err := u.store.ActiveWorkspace(ctx); err == nil && active == current.ID {
		u.notifier.Publish(TopicChanged, Changed{Workspace: current})
	}
	return u.Snapshot(ctx)
}

// Delete removes a workspace and everything in it. The one the app is born with is refused: it is
// the fallback every other answer leans on, and a database without it has no workspace to show.
// Deleting the one on screen moves the pointer back to that default, so the window is never left
// pointing at a row that is gone.
func (u *UseCase) Delete(ctx context.Context, id string) (domain.WorkspaceState, error) {
	target, err := u.store.Workspace(ctx, id)
	if err != nil {
		return domain.WorkspaceState{}, err
	}
	if target.Personal {
		return domain.WorkspaceState{}, domain.Refuse(domain.CodePersonalWorkspace,
			domain.ErrNotAllowed, domain.Args{"name": target.Name})
	}

	// Which workspace is on screen is read before the row goes: afterwards the pointer names something
	// that is no longer there, and the answer to "was this the one being shown" is gone with it.
	active, err := u.store.ActiveWorkspace(ctx)
	if err != nil {
		return domain.WorkspaceState{}, err
	}

	if err := u.store.DeleteWorkspace(ctx, target.ID); err != nil {
		return domain.WorkspaceState{}, err
	}

	if active == target.ID {
		if err := u.store.SetActiveWorkspace(ctx, domain.WorkspacePersonalID); err != nil {
			return domain.WorkspaceState{}, err
		}
		if fallback, err := u.store.Workspace(ctx, domain.WorkspacePersonalID); err == nil {
			u.notifier.Publish(TopicChanged, Changed{Workspace: fallback})
		}
	}
	return u.Snapshot(ctx)
}

// Changed is what a window receives when the workspace on screen moves.
type Changed struct {
	Workspace domain.Workspace `json:"workspace"`
}

func cleanName(raw string) (string, error) {
	name := strings.TrimSpace(raw)
	if name == "" {
		return "", domain.Refuse(domain.CodeNameEmpty, domain.ErrNotAllowed, nil)
	}
	if len([]rune(name)) > maxNameLength {
		return "", domain.Refuse(domain.CodeNameTooLong, domain.ErrNotAllowed,
			domain.Args{"max": strconv.Itoa(maxNameLength)})
	}
	return name, nil
}
