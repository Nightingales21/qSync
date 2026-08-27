package manifest

import (
	"os"
	"path/filepath"
	"testing"
)

func TestScan(t *testing.T) {
	dir := t.TempDir()

	if err := os.WriteFile(filepath.Join(dir, "a.txt"), []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "sub"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "sub", "b.txt"), []byte("world"), 0o644); err != nil {
		t.Fatal(err)
	}

	m, err := Scan(dir)
	if err != nil {
		t.Fatal(err)
	}

	if len(m.Entries) != 2 {
		t.Fatalf("expected 2 entries, got %d: %+v", len(m.Entries), m.Entries)
	}
	if m.Entries[0].Path != "a.txt" {
		t.Errorf("expected first entry a.txt, got %s", m.Entries[0].Path)
	}
	if m.Entries[1].Path != "sub/b.txt" {
		t.Errorf("expected second entry sub/b.txt, got %s", m.Entries[1].Path)
	}
	if m.Entries[0].Hash == "" || m.Entries[0].Size != 5 {
		t.Errorf("unexpected entry: %+v", m.Entries[0])
	}
}

func TestScanIdenticalContentSameHash(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "a.txt"), []byte("same"), 0o644)
	os.WriteFile(filepath.Join(dir, "b.txt"), []byte("same"), 0o644)

	m, err := Scan(dir)
	if err != nil {
		t.Fatal(err)
	}

	byPath := m.ByPath()
	if byPath["a.txt"].Hash != byPath["b.txt"].Hash {
		t.Errorf("expected identical content to hash identically")
	}
}
