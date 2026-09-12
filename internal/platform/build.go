package platform

// BuildInfo is the app's identity: the two values the linker stamps and the name it shows. It is
// the only thing the composition root knows that nothing else can discover for itself.
type BuildInfo struct {
	Version     string
	Build       string
	Name        string
	Description string
}
