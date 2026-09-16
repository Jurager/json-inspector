package platform

// BuildInfo is the app describing itself to whoever asks at runtime: the windows, the extension
// bridge, the updater. It carries only the identity a running process needs — the rest of it (the
// publisher, the copyright, the data folder's name) exists for the packaging tools and is read from
// the constants beside it.
type BuildInfo struct {
	Version string
	// Build is the CI run number, empty for local builds.
	Build       string
	Name        string
	Slug        string
	Identifier  string
	Description string
}

// NewBuildInfo is the one construction, so the identity cannot be half-supplied from a composition
// root that forgot a field.
func NewBuildInfo() BuildInfo {
	return BuildInfo{
		Version:     version,
		Build:       build,
		Name:        Name,
		Slug:        Slug,
		Identifier:  Identifier,
		Description: Description,
	}
}
