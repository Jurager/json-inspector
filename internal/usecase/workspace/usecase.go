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

// Start points the app at the space it opens in: the one it was left in, or the oldest one when the
// user has turned "reopen the last workspace" off.
//
// It writes the pointer rather than answering a different one, and it runs once at launch. A
// preference read on every resolve would follow the whole session: everything that asks which
// space is on screen — the list, the history, the pruning, the imports — would be answered with
// the oldest one, and a window that switched away from it would be switched back.
func (u *UseCase) Start(ctx context.Context) error {
	reopen, err := u.store.ReopenLast(ctx)
	if err != nil {
		return err
	}
	if reopen {
		return nil
	}

	list, err := u.store.Workspaces(ctx)
	if err != nil || len(list) == 0 {
		return err
	}
	active, err := u.store.ActiveWorkspace(ctx)
	if err != nil || active == list[0].ID {
		return err
	}
	// The event is not published: at this hour nothing is listening yet, and the window reads the
	// snapshot on its way up rather than waiting to be told.
	return u.store.SetActiveWorkspace(ctx, list[0].ID)
}

// Snapshot is every workspace and the pointer to the one on screen — the whole of what the switcher
// draws, in one answer — with what each of them holds, because the window draws the counts in the
// same rows as the names.
func (u *UseCase) Snapshot(ctx context.Context) (domain.WorkspaceState, error) {
	list, err := u.store.Workspaces(ctx)
	if err != nil {
		return domain.WorkspaceState{}, err
	}
	active, err := u.store.ActiveWorkspace(ctx)
	if err != nil {
		return domain.WorkspaceState{}, err
	}
	counts, err := u.store.Counts(ctx)
	if err != nil {
		return domain.WorkspaceState{}, err
	}
	return domain.WorkspaceState{Workspaces: list, ActiveID: active, Counts: counts}, nil
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

func (u *UseCase) Update(
	ctx context.Context,
	id string,
	patch Patch,
) (domain.WorkspaceState, error) {
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

// Delete removes a workspace and everything in it. The last one is refused: every other answer the
// app gives leans on there being a space to keep things in, and which one it is does not matter —
// the row the app is born with goes like any other. Deleting the one on screen moves the pointer to
// the oldest that is left, so the window is never left pointing at a row that is gone.
func (u *UseCase) Delete(ctx context.Context, id string) (domain.WorkspaceState, error) {
	target, err := u.store.Workspace(ctx, id)
	if err != nil {
		return domain.WorkspaceState{}, err
	}

	list, err := u.store.Workspaces(ctx)
	if err != nil {
		return domain.WorkspaceState{}, err
	}
	if len(list) <= 1 {
		return domain.WorkspaceState{}, domain.Refuse(domain.CodeLastWorkspace,
			domain.ErrNotAllowed, nil)
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

	// The successor is picked here rather than left to the store's fallback: a reader of this call
	// should see which space the window lands on, and the fallback keeps its one job — a pointer that
	// went stale behind the app's back.
	if active == target.ID {
		remaining, err := u.store.Workspaces(ctx)
		if err != nil {
			return domain.WorkspaceState{}, err
		}
		if len(remaining) > 0 {
			if err := u.store.SetActiveWorkspace(ctx, remaining[0].ID); err != nil {
				return domain.WorkspaceState{}, err
			}
			u.notifier.Publish(TopicChanged, Changed{Workspace: remaining[0]})
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
