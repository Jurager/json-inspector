// Command manifest puts the app's identity into the files the packagers read.
//
// The app's name, bundle identifier, description, publisher and version each have to appear as
// text in files that no Go program reads: the Wails build config, the macOS bundle's Info.plist,
// the Windows version resource, the control file of a .deb. Only a person or a tool can write
// those, and until now nobody did — which is why the same word ended up in six files with four
// different spellings of the description and a stale name in five of them after any rename.
//
// So the values are declared once, in internal/platform, and this is the one place that knows how
// each of them reaches each reader. It writes them, and it can print one for the Taskfile, which
// needs two of them for the .desktop file it generates itself.
//
// The staleness that used to be possible is now impossible: manifest_test.go runs `differences` on
// every `go test` and fails when a file holds an identity value that Go no longer declares. The
// version is the one thing that test skips: in a committed tree it legitimately lags until a
// release.
package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"strings"

	"json-inspector/internal/platform"
)

func main() {
	version := flag.String("version", "", "release version, e.g. 0.1.3")
	print := flag.String("print", "", "print one identity value and exit: name, slug, description, "+
		"scheme")
	flag.Parse()

	// Printing is how the build asks rather than keeping a second copy: the Taskfile needs the
	// product's name and description for the .desktop file it writes, and it has no way to read Go.
	if *print != "" {
		value, err := identityValue(*print)
		if err != nil {
			log.Fatalf("manifest: %v", err)
		}
		fmt.Println(value)
		return
	}

	if err := checkVersion(*version); err != nil {
		log.Fatalf("manifest: %v", err)
	}
	for _, file := range files(".", *version) {
		if err := patchFile(file.Path, file.Fields); err != nil {
			log.Fatalf("manifest: %v", err)
		}
		fmt.Println(file.Path)
	}
}

// repoRoot is where the test reads from: it runs in this package, three directories below the files
// the table names.
const repoRoot = "../../.."

func identityValue(field string) (string, error) {
	switch field {
	case "name":
		return platform.Name, nil
	case "slug":
		return platform.Slug, nil
	case "description":
		return platform.Description, nil
	case "scheme":
		return platform.Scheme, nil
	default:
		return "", fmt.Errorf("no identity value named %q", field)
	}
}

func checkVersion(version string) error {
	switch {
	case version == "":
		return fmt.Errorf("-version is required, e.g. -version 0.1.3")
	case version == "dev":
		return fmt.Errorf("-version got the dev default; pass the release tag without its leading v")
	case strings.HasPrefix(version, "v"):
		return fmt.Errorf("-version got %q; drop the leading v from the tag", version)
	}
	return nil
}

// patchFile replaces every field's line in one file, and nothing else.
//
// The file is edited in place rather than regenerated: it is mostly things this tool has no opinion
// about — dev-mode settings, packaging dependencies, entitlements — and regenerating would mean
// carrying all of that in Go and silently dropping whatever a person adds there next.
func patchFile(path string, fields []Field) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	// The separator is whatever the file already uses: rewriting a Unix-ending file with CRLF would
	// show up as a change to every line.
	sep := "\n"
	if strings.Contains(string(data), "\r\n") {
		sep = "\r\n"
	}
	lines := strings.Split(string(data), sep)

	for _, field := range fields {
		i, err := locate(lines, field)
		if err != nil {
			return fmt.Errorf("%s: %w", path, err)
		}
		lines[i] = field.render()
	}
	if err := os.WriteFile(path, []byte(strings.Join(lines, sep)), 0o644); err != nil {
		return err
	}
	return nil
}

// differences reports every identity value that is written in a file and not declared in Go.
//
// Only the identity: the version fields are skipped, because a committed tree carries the version
// of the last release until the next one stamps over it, and demanding otherwise would fail every
// build between two releases.
func differences() ([]string, error) {
	var found []string
	// The version is not part of the question, so the fields that carry it are built with an empty
	// value. They are skipped below and never render.
	for _, file := range files(repoRoot, "") {
		data, err := os.ReadFile(file.Path)
		if err != nil {
			return nil, err
		}
		lines := strings.Split(strings.ReplaceAll(string(data), "\r\n", "\n"), "\n")

		for _, field := range file.Fields {
			if field.Version {
				continue
			}
			i, err := locate(lines, field)
			if err != nil {
				return nil, fmt.Errorf("%s: %w", file.Path, err)
			}
			if want := field.render(); lines[i] != want {
				found = append(found, fmt.Sprintf("%s:%d\n  want %s\n  have %s",
					file.Path, i+1, want, lines[i]))
			}
		}
	}
	return found, nil
}
