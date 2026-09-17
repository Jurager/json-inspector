package domain

// WorkspaceKind is what a space is for: the one a single person keeps, or the one a team shares.
// The kind is a property of the row rather than a second table because everything a team adds —
// members, invitations, roles — hangs off the same workspace the personal one is.
type WorkspaceKind string

const (
	WorkspacePersonal WorkspaceKind = "personal"
	WorkspaceTeam     WorkspaceKind = "team"
)

func (k WorkspaceKind) Valid() bool {
	return k == WorkspacePersonal || k == WorkspaceTeam
}

// Workspace is the container everything the user makes belongs to: the history, the collections,
// the environments and the draft the composer is holding. Which one is shown is the window's own
// pointer (SettingActiveWorkspace); what a workspace holds never mixes with another.
//
// Color is the palette's word — "blue", "purple" — and not a value: the tint is drawn from a
// token, and a word the window does not know is drawn in the default tint rather than refused.
type Workspace struct {
	ID        string        `json:"id"`
	Name      string        `json:"name"`
	Kind      WorkspaceKind `json:"kind"`
	Color     string        `json:"color"`
	CreatedAt int64         `json:"createdAt"`
	UpdatedAt int64         `json:"updatedAt"`
}

// WorkspacePersonalID is the workspace the app is born with: the schema creates it, and it is the
// one a window that has never chosen anything opens on. It is a fixed word rather than a generated
// id for the same reason DraftCommandLine is — a migration has to be able to name it.
//
// It is not a workspace that cannot be deleted, and it is not what a fallback leans on: the last
// space left is the one that may not go, whichever it is, and a stale pointer falls back to the
// first row. What the word still names is the row the schema makes — and the row whose name is
// empty, which the window draws as «Личное» in whatever language it is in.
const WorkspacePersonalID = "personal"

// SettingActiveWorkspace names the workspace the window is showing. It is a preference of the
// installation — like the theme and the language beside it in the settings table, and like them it
// belongs to no workspace: one database has one answer to "which space is on screen", and switching
// writes it.
const SettingActiveWorkspace = "workspace.activeId"

// WorkspaceState is every workspace and the pointer to the one on screen — what the switcher draws
// and what every change to the set answers with. The shape follows EnvState, and for the same
// reason: the list and the choice are read together and are never interesting apart.
type WorkspaceState struct {
	Workspaces []Workspace `json:"workspaces"`
	ActiveID   string      `json:"activeId"`
	// Counts is what each space holds, by its id: three numbers for the manager window to draw.
	// Beside the list and not on every Workspace, because a count is a reading of the space and not
	// a part of it — a writer handed one would only have to ignore it.
	Counts map[string]WorkspaceCounts `json:"counts,omitempty"`
}

// WorkspaceCounts is what a space holds: the collections in it, the environments, and the runs its
// collections have been through. Three numbers and nothing else — what the design draws, and never
// what a rule is made of.
type WorkspaceCounts struct {
	Collections  int `json:"collections"`
	Environments int `json:"environments"`
	Runs         int `json:"runs"`
}

// NewWorkspace is a workspace on its way in.
func NewWorkspace(id string, name string, kind WorkspaceKind, color string, now int64) Workspace {
	return Workspace{
		ID:        id,
		Name:      name,
		Kind:      kind,
		Color:     color,
		CreatedAt: now,
		UpdatedAt: now,
	}
}
