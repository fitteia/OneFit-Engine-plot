package main

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func writeString(s string) func(io.Writer) error {
	return func(w io.Writer) error { _, err := io.WriteString(w, s); return err }
}

// TestWriteFileSpecial: a name that is not a regular file is written
// directly - there is no /dev/null.tmp to make.
func TestWriteFileSpecial(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("no /dev/null")
	}
	if err := writeFile(os.DevNull, writeString("x")); err != nil {
		t.Fatal(err)
	}
}

// TestWriteFileReplaces: an output is replaced whole, keeps its
// permissions, and no temporary file is left; a failed write leaves the
// old file as it was.
func TestWriteFileReplaces(t *testing.T) {
	dir := t.TempDir()
	out := filepath.Join(dir, "out.pdf")
	if err := os.WriteFile(out, []byte("old"), 0o640); err != nil {
		t.Fatal(err)
	}
	if err := writeFile(out, writeString("new")); err != nil {
		t.Fatal(err)
	}
	if b, _ := os.ReadFile(out); string(b) != "new" {
		t.Errorf("content %q", b)
	}
	if st, _ := os.Stat(out); runtime.GOOS != "windows" && st.Mode().Perm() != 0o640 {
		t.Errorf("mode %v, was 0640", st.Mode().Perm())
	}
	if err := writeFile(out, func(io.Writer) error { return errors.New("render failed") }); err == nil {
		t.Error("a failed write reported no error")
	}
	if b, _ := os.ReadFile(out); string(b) != "new" {
		t.Errorf("a failed write changed the file: %q", b)
	}
	if left, _ := filepath.Glob(filepath.Join(dir, "*.tmp")); len(left) > 0 {
		t.Errorf("temporary files left: %v", left)
	}
}

// TestWriteFileReadOnlyFolder: an existing, writable output in a folder
// where no temporary file can be made is written in place.
func TestWriteFileReadOnlyFolder(t *testing.T) {
	if runtime.GOOS == "windows" || os.Geteuid() == 0 {
		t.Skip("folder permissions do not bind here")
	}
	dir := t.TempDir()
	out := filepath.Join(dir, "out.svg")
	if err := os.WriteFile(out, []byte("old"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(dir, 0o555); err != nil {
		t.Fatal(err)
	}
	defer os.Chmod(dir, 0o755)
	if err := writeFile(out, writeString("new")); err != nil {
		t.Fatal(err)
	}
	if b, _ := os.ReadFile(out); string(b) != "new" {
		t.Errorf("content %q", b)
	}
	// a new file there still fails, with the folder's error
	if err := writeFile(filepath.Join(dir, "new.svg"), writeString("x")); err == nil {
		t.Error("made a new file in a read-only folder")
	}
}
