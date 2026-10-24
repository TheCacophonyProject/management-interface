package api

import (
	"bufio"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"slices"
	"time"
)

// trapHistoryPath is where tc2-hat-comms saves the messages it sends to and gets from the
// trap, one JSON object per line, oldest first. It is read directly rather than over DBus so
// the history can still be seen when the trap isn't connected.
const trapHistoryPath = "/var/lib/tc2-hat-controller/trap-history.jsonl"

type trapHistoryEntry struct {
	Time time.Time `json:"time"`
	// Dir is "sent" or "received", "failed" for a sent message that got no response, or
	// "lost" for a line that couldn't be read. For failed and lost the payload is the error.
	Dir string `json:"dir"`
	// ID matches a response to the message it answers. 0 for a message the trap sent on
	// its own.
	ID      int    `json:"id,omitempty"`
	Type    string `json:"type"`
	Payload string `json:"payload,omitempty"`
}

// readTrapHistory returns the messages sent to and from the trap, newest first. A line in the
// file that can't be read, such as one cut short by a power cut, is returned as a "lost"
// entry with no time.
func readTrapHistory(path string) ([]trapHistoryEntry, error) {
	entries := []trapHistoryEntry{}
	f, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		return entries, nil
	}
	if err != nil {
		return nil, err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		var entry trapHistoryEntry
		if err := json.Unmarshal(scanner.Bytes(), &entry); err != nil {
			entry = trapHistoryEntry{Dir: "lost", Payload: "couldn't read this entry in the history file"}
		}
		entries = append(entries, entry)
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	slices.Reverse(entries)
	return entries, nil
}

// GetTrapHistory returns the messages sent between the camera and the trap, newest first.
func (api *ManagementAPI) GetTrapHistory(w http.ResponseWriter, r *http.Request) {
	entries, err := readTrapHistory(trapHistoryPath)
	if err != nil {
		log.Printf("failed to read trap history: %v", err)
		serverError(&w, errors.New("failed to read trap history"))
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(entries)
}
