package protocol

import (
	"bytes"
	"testing"

	"qsync/internal/manifest"
)

func TestWriteReadMessageRoundTrip(t *testing.T) {
	var buf bytes.Buffer

	orig := NewManifestResponse(manifest.Manifest{
		Entries: []manifest.FileEntry{
			{Path: "a.txt", Size: 3, Hash: "abc"},
		},
	})

	if err := WriteMessage(&buf, orig); err != nil {
		t.Fatalf("write: %v", err)
	}

	got, err := ReadMessage(&buf)
	if err != nil {
		t.Fatalf("read: %v", err)
	}

	if got.Type != MsgManifestResponse {
		t.Fatalf("expected type %s, got %s", MsgManifestResponse, got.Type)
	}
	if got.Manifest == nil || len(got.Manifest.Entries) != 1 || got.Manifest.Entries[0].Path != "a.txt" {
		t.Fatalf("unexpected manifest payload: %+v", got.Manifest)
	}
}

func TestWriteReadMultipleMessages(t *testing.T) {
	var buf bytes.Buffer

	msgs := []Message{
		NewManifestRequest(),
		NewSyncPlan([]PlanItem{{Path: "x.txt", Action: manifest.ActionAdd, Size: 1, StreamID: 4}}),
		NewFileComplete("x.txt", true, ""),
	}

	for _, m := range msgs {
		if err := WriteMessage(&buf, m); err != nil {
			t.Fatalf("write: %v", err)
		}
	}

	for i, want := range msgs {
		got, err := ReadMessage(&buf)
		if err != nil {
			t.Fatalf("read %d: %v", i, err)
		}
		if got.Type != want.Type {
			t.Errorf("message %d: got type %s, want %s", i, got.Type, want.Type)
		}
	}
}

func TestReadMessageTruncated(t *testing.T) {
	var buf bytes.Buffer
	if err := WriteMessage(&buf, NewManifestRequest()); err != nil {
		t.Fatalf("write: %v", err)
	}

	truncated := bytes.NewReader(buf.Bytes()[:2])
	if _, err := ReadMessage(truncated); err == nil {
		t.Fatal("expected error reading truncated message")
	}
}
