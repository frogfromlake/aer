package state

import (
	"encoding/json"
	"fmt"
	"os"
	"sync"
)

// Store tracks submitted GUIDs per feed to prevent re-ingestion on repeated runs.
// State is persisted to a local JSON file.
type Store struct {
	path string
	mu   sync.Mutex
	data map[string]map[string]bool // feed_name -> set of submitted GUIDs
}

// NewStore creates or loads a dedup state store from the given file path.
func NewStore(path string) (*Store, error) {
	s := &Store{
		path: path,
		data: make(map[string]map[string]bool),
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return s, nil
		}
		return nil, fmt.Errorf("failed to read state file %s: %w", path, err)
	}

	// Deserialize: JSON stores arrays, we convert to sets
	var fileData map[string][]string
	if err := json.Unmarshal(raw, &fileData); err != nil {
		return nil, fmt.Errorf("failed to parse state file %s: %w", path, err)
	}

	for feed, guids := range fileData {
		s.data[feed] = make(map[string]bool, len(guids))
		for _, g := range guids {
			s.data[feed][g] = true
		}
	}

	return s, nil
}

// HasSeen returns true if the GUID has already been submitted for the given feed.
func (s *Store) HasSeen(feedName, guid string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	if feedGUIDs, ok := s.data[feedName]; ok {
		return feedGUIDs[guid]
	}
	return false
}

// MarkSeen records a GUID as submitted for the given feed.
func (s *Store) MarkSeen(feedName, guid string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.data[feedName] == nil {
		s.data[feedName] = make(map[string]bool)
	}
	s.data[feedName][guid] = true
}

// Save persists the current state to disk.
func (s *Store) Save() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Convert sets to arrays for JSON serialization
	fileData := make(map[string][]string, len(s.data))
	for feed, guids := range s.data {
		arr := make([]string, 0, len(guids))
		for g := range guids {
			arr = append(arr, g)
		}
		fileData[feed] = arr
	}

	raw, err := json.MarshalIndent(fileData, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal state: %w", err)
	}

	if err := os.WriteFile(s.path, raw, 0644); err != nil {
		return fmt.Errorf("failed to write state file %s: %w", s.path, err)
	}

	return nil
}
