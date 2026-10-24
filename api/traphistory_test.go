package api

import (
	"os"
	"path/filepath"
	"testing"
)

func TestReadTrapHistory(t *testing.T) {
	dir := t.TempDir()

	entries, err := readTrapHistory(filepath.Join(dir, "missing.jsonl"))
	if err != nil || len(entries) != 0 {
		t.Fatalf("missing file: got %v, %v, want no entries", entries, err)
	}

	path := filepath.Join(dir, "trap-history.jsonl")
	data := `{"time":"2026-10-24T01:00:00Z","dir":"sent","id":4,"type":"CAGE_KILL"}
{"time":"2026-10-24T01:05:00Z","dir":"received","type":"CAGE_CAPTURED","payload":"{\"HoldHours\": 24}"}
{"time":"2026-10-24T01:06:00Z","ty
`
	if err := os.WriteFile(path, []byte(data), 0644); err != nil {
		t.Fatal(err)
	}
	entries, err = readTrapHistory(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 3 {
		t.Fatalf("got %d entries, want 3", len(entries))
	}
	if entries[0].Dir != "lost" || !entries[0].Time.IsZero() || entries[0].Type != "" {
		t.Errorf("newest entry is %+v, want a lost entry for the cut short line", entries[0])
	}
	entries = entries[1:]
	if entries[0].Dir != "received" || entries[0].Type != "CAGE_CAPTURED" || entries[0].Payload != `{"HoldHours": 24}` {
		t.Errorf("newest entry is %+v, want CAGE_CAPTURED with its payload", entries[0])
	}
	if entries[1].Dir != "sent" || entries[1].ID != 4 || entries[1].Type != "CAGE_KILL" || entries[1].Payload != "" {
		t.Errorf("oldest entry is %+v, want the sent CAGE_KILL", entries[1])
	}
}
