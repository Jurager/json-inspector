package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"strings"
)

type rule struct {
	find   string
	after  bool
	format string // %s is the version
}

type target struct {
	path  string
	rules []rule
}

var plistRules = []rule{
	{find: "<key>CFBundleShortVersionString</key>", after: true, format: "\t\t<string>%s</string>"},
	{find: "<key>CFBundleVersion</key>", after: true, format: "\t\t<string>%s</string>"},
}

var targets = []target{
	{
		path:  "build/config.yml",
		rules: []rule{{find: "  version: ", format: `  version: "%s"`}},
	},
	{path: "build/darwin/Info.plist", rules: plistRules},
	{path: "build/darwin/Info.dev.plist", rules: plistRules},
	{
		path:  "build/linux/nfpm/nfpm.yaml",
		rules: []rule{{find: "version: ", format: `version: "%s"`}},
	},
	{
		path: "build/windows/info.json",
		rules: []rule{
			{find: `"file_version": `, format: "\t\t\"file_version\": \"%s\""},
			{find: `"ProductVersion": `, format: "\t\t\t\"ProductVersion\": \"%s\","},
		},
	},
}

func main() {
	version := flag.String("version", "", "release version, e.g. 0.1.3")
	flag.Parse()

	if err := check(*version); err != nil {
		log.Fatalf("stamp: %v", err)
	}
	for _, t := range targets {
		if err := stamp(t, *version); err != nil {
			log.Fatalf("stamp: %v", err)
		}
	}
}

func check(version string) error {
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

func stamp(t target, version string) error {
	data, err := os.ReadFile(t.path)
	if err != nil {
		return err
	}
	sep := "\n"
	if strings.Contains(string(data), "\r\n") {
		sep = "\r\n"
	}
	lines := strings.Split(string(data), sep)

	for _, r := range t.rules {
		i, err := find(lines, r)
		if err != nil {
			return fmt.Errorf("%s: %w", t.path, err)
		}
		lines[i] = fmt.Sprintf(r.format, version)
	}
	if err := os.WriteFile(t.path, []byte(strings.Join(lines, sep)), 0o644); err != nil {
		return err
	}
	fmt.Printf("%s: %s\n", t.path, version)
	return nil
}

func find(lines []string, r rule) (int, error) {
	var hits []int
	for i, line := range lines {
		if strings.Contains(line, r.find) {
			hits = append(hits, i)
		}
	}
	switch len(hits) {
	case 1:
		i := hits[0]
		if !r.after {
			return i, nil
		}
		if i+1 >= len(lines) {
			return 0, fmt.Errorf("%q is the last line, so it has no value under it", r.find)
		}
		return i + 1, nil
	case 0:
		return 0, fmt.Errorf("nothing matches %q — was the file regenerated", r.find)
	default:
		return 0, fmt.Errorf("%q matches %d lines, expected one", r.find, len(hits))
	}
}
