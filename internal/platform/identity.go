package platform

// The app's identity, declared once.
//
// Every one of these also has to appear as text in files this program cannot reach — the Wails
// build config, the macOS bundle's Info.plist, the Windows version resource, the .deb's control
// file, the Taskfile that drives the packagers. That is why the same word used to live in six
// files: only a program can read a Go constant, and none of those readers is a Go program.
//
// So this is the one place a value is written, and `internal/tools/manifest` is the one place that
// knows how each value reaches each reader. A test over there fails when the two disagree, which is
// what keeps a rename from leaving five files describing the previous name.
const (
	// Name is what a person sees: window titles, application menus, the OS's own lists.
	Name = "JSON Inspector"

	// Slug names the artefacts on disk — the built binary, the release archives, and the name the
	// extension bridge introduces itself with. It has to agree with `APP_NAME` in the Taskfile and
	// with the name the updater looks for inside an archive (see infra/updater/swap_*.go).
	Slug = "json-inspector"

	// Identifier is the bundle's reverse-DNS name, and the OS keys the app's single instance off it:
	// two builds sharing it are one app to the platform, which is what makes the second launch focus
	// the first window instead of starting beside it.
	Identifier = "com.jurager.json-inspector"

	// Scheme is what a `json-inspector://…` link opens. It is deliberately its own constant rather
	// than the slug: the two are equal today by coincidence of naming, and a scheme has rules a file
	// name does not.
	Scheme = "json-inspector"

	// Company and Copyright are the publisher's, and appear only in the packaging metadata.
	Company   = "Yuri Gerasimov"
	Copyright = "© 2026, Yuri Gerasimov"

	// Description is the app in one line, for the places a person reads it before installing: the
	// Windows version resource, the .deb's description, the .desktop comment, the process list.
	//
	// It is a single field of a manifest written once at build time, so it cannot follow the language
	// the window is in — the name cannot either. Of the two languages it could be written in, English
	// is the one the manifest is read in.
	Description = "Inspect JSON with environment variables, schema mapping, " +
		"and browser request interception."

	// RepoOwner is where the releases live. The updater reaches GitHub with it, and the packagers
	// build the project's home page out of it and the slug.
	RepoOwner = "Jurager"

	// DataDirName is the folder in the user's config directory, and it is not the slug: renaming a
	// brand may change the slug, and it must never move a user's database. Changing this value
	// orphans every install that came before — a new folder is created, and the old one, with all the
	// history in it, is left behind.
	DataDirName = "json-inspector"
)

// Version and Build are stamped into the binary at build time rather than written in the source:
// `-ldflags -X json-inspector/internal/platform.version=…`. A release therefore leaves the tree
// untouched, and the working copy is never dirty after a build.
//
// They are variables for that reason alone, and the identity above is const for the opposite one:
// an empty identifier or an empty scheme is a broken bundle, and nothing that matters may depend
// on the build having been driven correctly.
var (
	version = "dev"
	build   = ""
)

// Homepage is the project's own address, built from the two parts rather than written a third time.
func Homepage() string {
	return "https://github.com/" + RepoOwner + "/" + Slug
}
