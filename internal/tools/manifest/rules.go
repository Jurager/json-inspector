package main

import (
	"fmt"
	"path/filepath"
	"strings"

	"json-inspector/internal/platform"
)

// Field is one value in one file: how to find it, and what to write there.
type Field struct {
	Value string
	// Find is a substring of the line to replace, and it has to appear exactly once. Matching by
	// position instead would break the first time somebody adds a line above.
	Find string
	// Offset is how far below the marker the value sits: 0 when the marker is the value's own line,
	// 1 for the plist and JSON pairs, where a key stands on one line and its value on the next, and 2
	// for the deep-link scheme, which sits inside an array two lines under its own key.
	Offset int
	// Format is what the line becomes, with %s for the value. It carries the indentation and the
	// trailing comma — the line is replaced whole, and a format that got either wrong would reformat
	// the file rather than edit it.
	Format string
	// Render builds the line whole, for the one line that carries two of our values: the Windows
	// assembly identity names the app and states its version in the same element, and a format with a
	// single verb cannot say both. Such a line is marked Version, because its version is part of the
	// text the check would compare.
	Render func() string
	// Version marks the fields carrying the release version rather than the identity. They are the
	// ones the check skips: a committed tree carries the last release's version until the next one
	// stamps over it, so demanding agreement there would fail every build between two releases.
	Version bool
}

func (f Field) render() string {
	if f.Render != nil {
		return f.Render()
	}
	return fmt.Sprintf(f.Format, f.Value)
}

// File is one file a packager reads, and the values written in it.
type File struct {
	Path   string
	Fields []Field
}

// files is every such file.
//
// One place has to know all of them, because each reader — the Wails build config, the macOS
// bundle, the Windows version resource, nfpm, the Taskfile's own .desktop — has its own format,
// and no Go program can write any of them. This table is that place; the values are in platform.
//
// root is the repository: the command runs from it, and the test sits three directories down and
// passes the path instead of changing the working directory out from under everything else.
func files(root, version string) []File {
	plist := func(path string) File {
		return File{Path: filepath.Join(root, path), Fields: []Field{
			{Value: platform.Slug, Find: "<key>CFBundleExecutable</key>", Offset: 1,
				Format: "\t\t<string>%s</string>"},
			{Value: platform.Name, Find: "<key>CFBundleGetInfoString</key>", Offset: 1,
				Format: "\t\t<string>%s</string>"},
			{Value: platform.Identifier, Find: "<key>CFBundleIdentifier</key>", Offset: 1,
				Format: "\t\t<string>%s</string>"},
			{Value: platform.Name, Find: "<key>CFBundleName</key>", Offset: 1,
				Format: "\t\t<string>%s</string>"},
			{Value: version, Find: "<key>CFBundleShortVersionString</key>", Offset: 1,
				Format: "\t\t<string>%s</string>", Version: true},
			// The URL type carries the identifier twice: as the type's own name and as the scheme it
			// answers to. Both are the app's, and both were stale-able before this table existed.
			{Value: platform.Identifier, Find: "<key>CFBundleURLName</key>", Offset: 1,
				Format: "\t\t\t\t<string>%s</string>"},
			{Value: platform.Scheme, Find: "<key>CFBundleURLSchemes</key>", Offset: 2,
				Format: "\t\t\t\t\t<string>%s</string>"},
			{Value: version, Find: "<key>CFBundleVersion</key>", Offset: 1,
				Format: "\t\t<string>%s</string>", Version: true},
			{Value: platform.Copyright, Find: "<key>NSHumanReadableCopyright</key>", Offset: 1,
				Format: "\t\t<string>%s</string>"},
		}}
	}

	return []File{
		{
			Path: filepath.Join(root, "build/config.yml"),
			Fields: []Field{
				{Value: platform.Company, Find: `companyName: "`, Format: `  companyName: "%s"`},
				{Value: platform.Name, Find: `productName: "`, Format: `  productName: "%s"`},
				{Value: platform.Identifier, Find: `productIdentifier: "`,
					Format: `  productIdentifier: "%s"`},
				// The protocol block further down has a `description:` of its own, unquoted; the quote
				// in the marker is what keeps the two apart.
				{Value: platform.Description, Find: `description: "`,
					Format: `  description: "%s"`},
				{Value: platform.Copyright, Find: `copyright: "`, Format: `  copyright: "%s"`},
				{Value: platform.Name, Find: `comments: "`, Format: `  comments: "%s"`},
				{Value: version, Find: `version: "`, Format: `  version: "%s"`, Version: true},
				// The scheme the OS answers to, and the sentence it shows beside it. Both name the app,
				// and the second is why a rename used to leave the URL registration describing the old
				// one.
				{Value: platform.Scheme, Find: "- scheme: ", Format: "  - scheme: %s"},
				{Value: platform.Name + " Protocol", Find: "    description: ",
					Format: "    description: %s"},
			},
		},
		plist("build/darwin/Info.plist"),
		plist("build/darwin/Info.dev.plist"),
		{
			Path: filepath.Join(root, "build/windows/info.json"),
			Fields: []Field{
				{Value: version, Find: `"file_version": "`, Format: "\t\t\"file_version\": \"%s\"",
					Version: true},
				{Value: version, Find: `"ProductVersion": "`,
					Format: "\t\t\t\"ProductVersion\": \"%s\",", Version: true},
				{Value: platform.Company, Find: `"CompanyName": "`,
					Format: "\t\t\t\"CompanyName\": \"%s\","},
				{Value: platform.Description, Find: `"FileDescription": "`,
					Format: "\t\t\t\"FileDescription\": \"%s\","},
				{Value: platform.Copyright, Find: `"LegalCopyright": "`,
					Format: "\t\t\t\"LegalCopyright\": \"%s\","},
				{Value: platform.Name, Find: `"ProductName": "`,
					Format: "\t\t\t\"ProductName\": \"%s\","},
				// The last entry of the object, so it carries no trailing comma.
				{Value: platform.Name, Find: `"Comments": "`,
					Format: "\t\t\t\"Comments\": \"%s\""},
			},
		},
		{
			Path: filepath.Join(root, "build/linux/nfpm/nfpm.yaml"),
			Fields: []Field{
				{Value: platform.Slug, Find: `name: "`, Format: `name: "%s"`},
				{Value: version, Find: `version: "`, Format: `version: "%s"`, Version: true},
				{Value: platform.Description, Find: `description: "`, Format: `description: "%s"`},
				{Value: platform.Company, Find: `vendor: "`, Format: `vendor: "%s"`},
				{Value: platform.Homepage(), Find: `homepage: "`, Format: `homepage: "%s"`},
			},
		},
		{
			// The manifest Windows reads before the program runs, and the second file the reference
			// project patches by hand. Ours was never touched: every Windows release shipped an assembly
			// identity naming the previous version, because the only thing that wrote this file was
			// whoever last edited it.
			//
			// The marker is the end of the element, not `assemblyIdentity type="win32"`, which the
			// Common-Controls dependency below also carries.
			Path: filepath.Join(root, "build/windows/wails.exe.manifest"),
			Fields: []Field{
				{
					Find:    `processorArchitecture="*"/>`,
					Version: true,
					Render: func() string {
						return fmt.Sprintf(
							"    <assemblyIdentity type=\"win32\" name=\"%s\" version=\"%s\" "+
								"processorArchitecture=\"*\"/>",
							platform.Identifier, version)
					},
				},
			},
		},
	}
}

// locate is the index of the one line a field describes.
func locate(lines []string, field Field) (int, error) {
	var hits []int
	for i, line := range lines {
		if strings.Contains(line, field.Find) {
			hits = append(hits, i)
		}
	}

	switch len(hits) {
	case 0:
		// The usual cause is a regenerated file: the packagers' templates come from the framework, and
		// one of them coming back in its original shape is not an error to work around but news.
		return 0, fmt.Errorf("nothing matches %q — was the file regenerated", field.Find)
	case 1:
		// The one line, below.
	default:
		return 0, fmt.Errorf("%q matches %d lines, expected one", field.Find, len(hits))
	}

	i := hits[0] + field.Offset
	if i >= len(lines) {
		return 0, fmt.Errorf("%q is %d lines from the end, so its value is not there",
			field.Find, field.Offset)
	}
	return i, nil
}
