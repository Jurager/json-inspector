package archtest

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"unicode/utf8"
)

// The rules here are the ones the style guide states about naming and imports, plus the one this
// project adds about how long a line may be. They are checked the way the layering rules are:
// mechanically, so a slip fails the build rather than a review. Each rule names where it is written
// down — docs/RECOMENDATIONS.md, or CLAUDE.md for the project's own.

// TestNoGetPrefix rejects `Get`-prefixed names. The guide's rule is that the noun carries the name
// — `JobName`, not `GetJobName` — and a name that says "get" says nothing that the return value or
// the receiver has not already said.
//
// The one honest exception is an interface this app implements but does not own: its method set is
// not ours to choose, and the day one arrives the rule gets a list of names with the reason beside
// each. None is here today — what the client implements is its own.
func TestNoGetPrefix(t *testing.T) {
	forEachDeclaration(t, func(name, pkg string, pos token.Position) {
		if !strings.HasPrefix(name, "Get") || len(name) == 3 || !isUpper(name[3]) {
			return
		}
		t.Errorf("%s: %s is named with a Get prefix — the noun says it better "+
			"(RECOMENDATIONS.md, \"Names of functions and methods\")", pos, name)
	})
}

// TestNoRepeatedPackageName rejects a name that spells its own package out again: `postmanType` in
// package postman reads as `postman.postmanType` at every call site.
//
// Only a name *longer* than the package is caught, and only an exported one: `record.Record` names
// the aggregate at the heart of the package, which is the guide's own exception — the noun is not a
// prefix that adds nothing, it is the name of the thing.
func TestNoRepeatedPackageName(t *testing.T) {
	forEachDeclaration(t, func(name, pkg string, pos token.Position) {
		if pkg == "" || len(name) <= len(pkg) || !isUpper(name[0]) {
			return
		}
		if !strings.EqualFold(name[:len(pkg)], pkg) || !isUpper(name[len(pkg)]) {
			return
		}
		t.Errorf("%s: %s repeats its package name — at the call site it reads as %s.%s "+
			"(RECOMENDATIONS.md, \"Avoid repetition\")", pos, name, pkg, name)
	})
}

// TestImportGroups keeps imports in the three groups the guide asks for, in this order: the
// standard library, everything else, and this module. A file with one group of one kind is fine; a
// group that mixes kinds is not, because which package an import came from is exactly what the
// grouping is for.
func TestImportGroups(t *testing.T) {
	walkGoFiles(t, func(path string, source string) {
		for _, group := range importGroups(source) {
			kinds := map[string]bool{}
			for _, line := range group {
				kinds[importKind(line)] = true
			}
			if len(kinds) > 1 {
				t.Errorf("%s: one import group holds %s — split it "+
					"(RECOMENDATIONS.md, \"Import order\")", path,
					strings.Join(sortedKeys(kinds), " and "))
			}
		}

		order := []string{}
		for _, group := range importGroups(source) {
			order = append(order, importKind(group[0]))
		}
		want := sortImports(order)
		if strings.Join(order, "|") != strings.Join(want, "|") {
			t.Errorf("%s: import groups are %v, want %v — standard library, then everything else, "+
				"then this module (RECOMENDATIONS.md, \"Import order\")",
				path, order, want)
		}

		for _, group := range importGroups(source) {
			if !sortedStrings(group) {
				t.Errorf("%s: imports are not sorted within their group: %v", path, group)
			}
		}
	})
}

// TestLineLength is the project's own rule rather than the guide's: a line is at most a hundred
// characters. Characters, not bytes — much of what is written here is Russian, and a rule counted
// in bytes would be twice as strict in a comment as in a line of Go.
func TestLineLength(t *testing.T) {
	const limit = 100

	walkGoFiles(t, func(path, source string) {
		for number, line := range strings.Split(source, "\n") {
			// The carriage return is dropped before counting: a file an editor saved with CRLF
			// endings would otherwise have every line reported one character too long, which is a
			// rule about line endings rather than about line length.
			if width := utf8.RuneCountInString(strings.TrimSuffix(line, "\r")); width > limit {
				t.Errorf("%s:%d is %d characters — the rule is %d (CLAUDE.md, \"Project rules\")",
					path, number+1, width, limit)
			}
		}
	})
}

// forEachDeclaration calls visit for every function, method and type at the top level of every Go
// file of ours, with the name, the package it was declared in, and where it was written.
func forEachDeclaration(t *testing.T, visit func(name, pkg string, pos token.Position)) {
	t.Helper()

	fset := token.NewFileSet()
	walkGoFiles(t, func(path string, source string) {
		file, err := parser.ParseFile(fset, path, source, parser.SkipObjectResolution)
		if err != nil {
			t.Fatalf("parsing %s: %v", path, err)
		}
		for _, decl := range file.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok {
				continue
			}
			visit(fn.Name.Name, file.Name.Name, fset.Position(fn.Pos()))
		}
	})
}

// walkGoFiles calls visit for every .go file that belongs to this module — the frontend's npm tree
// and the extension are not ours to check.
func walkGoFiles(t *testing.T, visit func(path, source string)) {
	t.Helper()

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
		data, readErr := os.ReadFile(path)
		if readErr != nil {
			return readErr
		}
		visit(filepath.ToSlash(path), string(data))
		return nil
	})
	if err != nil {
		t.Fatalf("walking the tree: %v", err)
	}
}

// importGroups is the import declarations of a file, one slice per blank-line-separated group, each
// holding the paths as written. A file with no parenthesised block answers with one group per plain
// `import "x"` line — and only that line: a quoted string anywhere else in the file (a fixture, a
// message key) is not an import.
func importGroups(source string) [][]string {
	start := strings.Index(source, "import (")
	if start < 0 {
		groups := [][]string{}
		for _, line := range strings.Split(source, "\n") {
			if !strings.HasPrefix(strings.TrimSpace(line), "import ") {
				continue
			}
			if path, ok := importPath(line); ok {
				groups = append(groups, []string{path})
			}
		}
		return groups
	}
	end := strings.Index(source[start:], "\n)")
	if end < 0 {
		return nil
	}

	groups, current := [][]string{}, []string{}
	for _, line := range strings.Split(source[start:start+end], "\n")[1:] {
		if strings.TrimSpace(line) == "" {
			if len(current) > 0 {
				groups = append(groups, current)
				current = []string{}
			}
			continue
		}
		if path, ok := importPath(line); ok {
			current = append(current, path)
		}
	}
	if len(current) > 0 {
		groups = append(groups, current)
	}
	return groups
}

// importPath reads the path out of one line of an import block, aliases and comments included. A
// line that is only a comment or a blank answers false.
func importPath(line string) (string, bool) {
	line = strings.TrimSpace(line)
	if idx := strings.Index(line, "//"); idx >= 0 {
		line = strings.TrimSpace(line[:idx])
	}
	quoted := regexp.MustCompile(`"([^"]+)"`).FindStringSubmatch(line)
	if quoted == nil {
		return "", false
	}
	return quoted[1], true
}

// importKind is where an import came from: this module, the standard library, or somebody else's
// code. A dot in the first segment is what tells the third apart from the first — the same rule
// goimports uses.
func importKind(path string) string {
	switch {
	case strings.HasPrefix(path, modulePath+"/") || path == modulePath:
		return "own"
	case strings.Contains(strings.SplitN(path, "/", 2)[0], "."):
		return "third"
	default:
		return "std"
	}
}

func sortImports(kinds []string) []string {
	rank := map[string]int{"std": 0, "third": 1, "own": 2}
	out := append([]string{}, kinds...)
	for i := 1; i < len(out); i++ {
		for j := i; j > 0 && rank[out[j]] < rank[out[j-1]]; j-- {
			out[j], out[j-1] = out[j-1], out[j]
		}
	}
	return out
}

func sortedStrings(list []string) bool {
	for i := 1; i < len(list); i++ {
		if list[i] < list[i-1] {
			return false
		}
	}
	return true
}

func sortedKeys(set map[string]bool) []string {
	out := make([]string, 0, len(set))
	for key := range set {
		out = append(out, key)
	}
	for i := 1; i < len(out); i++ {
		for j := i; j > 0 && out[j] < out[j-1]; j-- {
			out[j], out[j-1] = out[j-1], out[j]
		}
	}
	return out
}

func isUpper(c byte) bool { return c >= 'A' && c <= 'Z' }
