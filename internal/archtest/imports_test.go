// Package archtest keeps the layering honest. Every rule in docs/ARCHITECTURE.md that can be
// checked mechanically is checked here, so a violation fails the build instead of being noticed
// months later in a review.
package archtest

import (
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

const modulePath = "json-inspector"

// wailsPkg is the desktop framework. Only the transport layer may see it: the moment a use case
// imports it, the logic stops being testable without a window.
const wailsPkg = "github.com/wailsapp/wails/v3"

// forbiddenNames are package names that say nothing about what they provide. A package that needs
// one of these names has more than one responsibility and wants splitting.
var forbiddenNames = map[string]bool{
	"util": true, "utils": true, "common": true, "helpers": true, "helper": true,
	"models": true, "types": true, "constants": true, "misc": true, "shared": true,
}

// treeRoot is where every rule walks from: the repository root, two levels above this package. The
// results are keyed by the path rules talk about ("internal/domain"), which is what this name is
// for.
//
// It is written with the path separator of the platform by filepath.Join below, so the comparison
// against it has to be made on the raw path rather than on the slash-separated form.
const treeRoot = "../.."

// isFixtures is a directory the rules do not look into: fixtures are not packages, and dot-folders
// are tooling.
func isFixtures(name string) bool {
	return name == "testdata" || strings.HasPrefix(name, ".")
}

// notOurCode are trees that hold no Go package of ours: the window and its dependencies, the
// extension's build output, and the release artefacts the Taskfile writes. Walking them is slow and
// they are not what any of these rules is about.
var notOurCode = map[string]bool{
	"frontend": true, "node_modules": true, "extension": true,
	"build": true, "bin": true, "dist": true,
}

// packageImports maps a package directory (relative to the repository root, slash-separated, using
// the module-relative form the rules talk about) to the module-internal packages it imports.
func packageImports(t *testing.T) map[string][]string {
	t.Helper()

	fset := token.NewFileSet()
	imports := map[string][]string{}

	err := filepath.WalkDir(treeRoot, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if path != treeRoot && (isFixtures(d.Name()) || notOurCode[d.Name()]) {
				return fs.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") {
			return nil
		}

		file, parseErr := parser.ParseFile(fset, path, nil, parser.ImportsOnly)
		if parseErr != nil {
			return parseErr
		}
		dir, relErr := filepath.Rel(treeRoot, filepath.Dir(path))
		if relErr != nil {
			return relErr
		}
		dir = filepath.ToSlash(dir)
		if dir == "." {
			// The root package has no directory of its own; the empty name is how the rules say it.
			dir = ""
		}

		for _, spec := range file.Imports {
			quoted, unquoteErr := strconv.Unquote(spec.Path.Value)
			if unquoteErr != nil {
				continue
			}
			imports[dir] = append(imports[dir], quoted)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walking the tree: %v", err)
	}
	return imports
}

// TestLayerDependencies is the rule table from docs/ARCHITECTURE.md, expressed as code.
func TestLayerDependencies(t *testing.T) {
	imports := packageImports(t)

	// forbid maps a package prefix to the internal packages it must not reach.
	forbid := []struct {
		owner   string
		against []string
		explain string
	}{
		{"internal/domain", []string{modulePath},
			"domain is entities and rules, with no dependency on the rest of the app"},
		{"internal/platform", []string{modulePath}, "platform is stdlib-only by design"},
		{"internal/infra", []string{modulePath + "/internal/usecase", modulePath + "/internal/transport"},
			"infrastructure is called by use cases, never the other way round"},
		{"internal/usecase", []string{modulePath + "/internal/infra", modulePath + "/internal/transport"},
			"use cases declare what they need; only the composition root wires implementations"},
	}

	for _, rule := range forbid {
		for dir, list := range imports {
			if dir != rule.owner && !strings.HasPrefix(dir, rule.owner+"/") {
				continue
			}
			for _, target := range list {
				for _, bad := range rule.against {
					if target == bad || strings.HasPrefix(target, bad+"/") {
						t.Errorf("%s imports %s — %s", dir, target, rule.explain)
					}
				}
			}
		}
	}

	// Use cases must not reach across to each other either: a feature that needs another declares
	// a small interface and lets the graph inject it.
	for dir, list := range imports {
		if !strings.HasPrefix(dir, "internal/usecase/") {
			continue
		}
		feature := strings.Split(strings.TrimPrefix(dir, "internal/usecase/"), "/")[0]
		for _, target := range list {
			other := strings.TrimPrefix(target, modulePath+"/internal/usecase/")
			if other == target {
				continue
			}
			if next := strings.Split(other, "/")[0]; next != feature {
				t.Errorf("%s imports %s — use cases must not import each other, inject an interface instead",
					dir, target)
			}
		}
	}

	// Wails belongs to the transport layer alone. The root package is exempt: main.go is the
	// composition root, and holding the *application.App it builds is its whole job.
	for dir, list := range imports {
		if dir == "" || strings.HasPrefix(dir, "internal/transport") {
			continue
		}
		for _, target := range list {
			if strings.HasPrefix(target, wailsPkg) {
				t.Errorf("%s imports %s — only internal/transport may depend on the desktop framework", dir,
					target)
			}
		}
	}
}

// TestPackageNames rejects the names that hide a package's job. Naming is checked because it is
// the cheapest signal that a package has grown a second responsibility.
func TestPackageNames(t *testing.T) {
	err := filepath.WalkDir(treeRoot, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() {
			return nil
		}
		name := d.Name()
		if path != treeRoot && (isFixtures(name) || notOurCode[name]) {
			return fs.SkipDir
		}
		if forbiddenNames[name] {
			t.Errorf("%s: package name %q says nothing about what it provides", path, name)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walking the tree: %v", err)
	}
}

// TestUsecaseFacade requires every feature to have the one file that is its entry point, so
// "where does this use case live" has a single answer.
func TestUsecaseFacade(t *testing.T) {
	entries, err := filepath.Glob("../internal/usecase/*")
	if err != nil {
		t.Fatalf("globbing use cases: %v", err)
	}
	for _, dir := range entries {
		info, statErr := fs.Stat(os.DirFS(dir), ".")
		if statErr != nil || !info.IsDir() {
			continue
		}
		if _, err := fs.Stat(os.DirFS(dir), "usecase.go"); err != nil {
			t.Errorf("%s: a use case package needs usecase.go as its entry point", dir)
		}
	}
}
