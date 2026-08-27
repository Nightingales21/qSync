package manifest

import (
	"sort"
	"testing"
)

func entry(path, hash string, size int64) FileEntry {
	return FileEntry{Path: path, Hash: hash, Size: size}
}

func sortedPlan(plan []DiffEntry) []DiffEntry {
	sort.Slice(plan, func(i, j int) bool { return plan[i].Path < plan[j].Path })
	return plan
}

func TestDiffAdd(t *testing.T) {
	client := Manifest{}
	server := Manifest{Entries: []FileEntry{entry("a.txt", "h1", 10)}}

	plan := Diff(client, server, false)

	if len(plan) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(plan))
	}
	if plan[0].Path != "a.txt" || plan[0].Action != ActionAdd || plan[0].Size != 10 {
		t.Errorf("unexpected entry: %+v", plan[0])
	}
}

func TestDiffUpdate(t *testing.T) {
	client := Manifest{Entries: []FileEntry{entry("a.txt", "old", 5)}}
	server := Manifest{Entries: []FileEntry{entry("a.txt", "new", 6)}}

	plan := Diff(client, server, false)

	if len(plan) != 1 || plan[0].Action != ActionUpdate {
		t.Fatalf("expected single update entry, got %+v", plan)
	}
}

func TestDiffUnchanged(t *testing.T) {
	client := Manifest{Entries: []FileEntry{entry("a.txt", "same", 5)}}
	server := Manifest{Entries: []FileEntry{entry("a.txt", "same", 5)}}

	plan := Diff(client, server, false)

	if len(plan) != 0 {
		t.Fatalf("expected no changes, got %+v", plan)
	}
}

func TestDiffDeleteFlagOff(t *testing.T) {
	client := Manifest{Entries: []FileEntry{entry("gone.txt", "h", 1)}}
	server := Manifest{}

	plan := Diff(client, server, false)

	if len(plan) != 0 {
		t.Fatalf("expected no changes when deletes disabled, got %+v", plan)
	}
}

func TestDiffDeleteFlagOn(t *testing.T) {
	client := Manifest{Entries: []FileEntry{entry("gone.txt", "h", 1)}}
	server := Manifest{}

	plan := Diff(client, server, true)

	if len(plan) != 1 || plan[0].Action != ActionDelete || plan[0].Path != "gone.txt" {
		t.Fatalf("expected delete entry, got %+v", plan)
	}
}

func TestDiffMixed(t *testing.T) {
	client := Manifest{Entries: []FileEntry{
		entry("keep.txt", "same", 1),
		entry("stale.txt", "old", 2),
		entry("removed.txt", "x", 3),
	}}
	server := Manifest{Entries: []FileEntry{
		entry("keep.txt", "same", 1),
		entry("stale.txt", "new", 2),
		entry("new.txt", "y", 4),
	}}

	plan := sortedPlan(Diff(client, server, true))

	want := []DiffEntry{
		{Path: "new.txt", Action: ActionAdd, Size: 4},
		{Path: "removed.txt", Action: ActionDelete},
		{Path: "stale.txt", Action: ActionUpdate, Size: 2},
	}

	if len(plan) != len(want) {
		t.Fatalf("expected %d entries, got %d: %+v", len(want), len(plan), plan)
	}
	for i := range want {
		if plan[i] != want[i] {
			t.Errorf("entry %d: got %+v, want %+v", i, plan[i], want[i])
		}
	}
}
