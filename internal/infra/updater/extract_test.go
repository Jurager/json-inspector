package updater

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"os"
	"path/filepath"
	"testing"
)

// zipWith writes a zip holding one entry of the given name and answers where it is.
func zipWith(t *testing.T, name, body string) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "update.zip")
	f, err := os.Create(path)
	if err != nil {
		t.Fatalf("creating the archive: %v", err)
	}
	defer f.Close()

	w := zip.NewWriter(f)
	entry, err := w.Create(name)
	if err != nil {
		t.Fatalf("creating the entry: %v", err)
	}
	if _, err := entry.Write([]byte(body)); err != nil {
		t.Fatalf("writing the entry: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("closing the archive: %v", err)
	}
	return path
}

// tarGzWith writes a tarball holding one entry of the given name and answers where it is.
func tarGzWith(t *testing.T, name, body string) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "update.tar.gz")
	f, err := os.Create(path)
	if err != nil {
		t.Fatalf("creating the archive: %v", err)
	}
	defer f.Close()

	gz := gzip.NewWriter(f)
	w := tar.NewWriter(gz)
	if err := w.WriteHeader(&tar.Header{
		Name: name, Mode: 0o644, Size: int64(len(body)), Typeflag: tar.TypeReg,
	}); err != nil {
		t.Fatalf("writing the header: %v", err)
	}
	if _, err := w.Write([]byte(body)); err != nil {
		t.Fatalf("writing the entry: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("closing the tarball: %v", err)
	}
	if err := gz.Close(); err != nil {
		t.Fatalf("closing the gzip stream: %v", err)
	}
	return path
}

// An entry named out of the archive is refused rather than written: the destination of an update is
// a temporary directory beside the running binary, and `../../x` in a downloaded file would put a
// file wherever it liked on the machine.
func TestAnEntryThatEscapesTheDestinationIsRefused(t *testing.T) {
	dest := t.TempDir()
	escape := filepath.Join(dest, "..", "escaped.txt")

	for _, tc := range []struct {
		what    string
		archive string
		extract func(string, string) error
	}{
		{"zip", zipWith(t, "../escaped.txt", "нет"), extractZip},
		{"targz", tarGzWith(t, "../escaped.txt", "нет"), extractTarGz},
	} {
		if err := tc.extract(tc.archive, dest); err == nil {
			t.Errorf("%s: extracting an entry named ../escaped.txt was allowed", tc.what)
		}
		if _, err := os.Stat(escape); err == nil {
			t.Errorf("%s: the entry was written outside the destination", tc.what)
		}
	}
}

// The ordinary case still works, folder entries included: what guards the update is a refusal of
// names that point out of the archive, not a refusal of archives.
func TestAnOrdinaryArchiveExtracts(t *testing.T) {
	dest := t.TempDir()

	if err := extractZip(zipWith(t, "json-inspector.exe", "binary"), dest); err != nil {
		t.Fatalf("extractZip: %v", err)
	}
	if err := extractTarGz(tarGzWith(t, "bin/json-inspector", "binary"), dest); err != nil {
		t.Fatalf("extractTarGz: %v", err)
	}

	for _, name := range []string{"json-inspector.exe", filepath.Join("bin", "json-inspector")} {
		body, err := os.ReadFile(filepath.Join(dest, name))
		if err != nil {
			t.Fatalf("reading %s: %v", name, err)
		}
		if !bytes.Equal(body, []byte("binary")) {
			t.Errorf("%s = %q, want the bytes the archive held", name, body)
		}
	}
}
