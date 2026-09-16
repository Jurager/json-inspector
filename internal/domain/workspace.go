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
//
// Personal is derived from the id where the row is read and never written back: saying it here
// keeps the spelling of that id in one place, which is the one that also refuses the call.
type Workspace struct {
	ID        string        `json:"id"`
	Name      string        `json:"name"`
	Kind      WorkspaceKind `json:"kind"`
	Color     string        `json:"color"`
	Personal  bool          `json:"personal"`
	CreatedAt int64         `json:"createdAt"`
	UpdatedAt int64         `json:"updatedAt"`
}

// WorkspacePersonalID is the workspace the app is born with: the schema creates it, and it is the
// one every fallback points at. It is a fixed word rather than a generated id for the same reason
// DraftCommandLine is — it has to be nameable by a migration, by a fallback and by a rule.
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
}

// NewWorkspace is a workspace on its way in.
func NewWorkspace(id string, name string, kind WorkspaceKind, color string, now int64) Workspace {
	return Workspace{
		ID:        id,
		Name:      name,
		Kind:      kind,
		Color:     color,
		Personal:  id == WorkspacePersonalID,
		CreatedAt: now,
		UpdatedAt: now,
	}
}
