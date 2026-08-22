package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
)

// blankKey identifies one answerable blank within a proposal by its position in
// the document's Blanks slice. QuizIndex is retained for the standalone LLM
// questions, which are numbered separately from the document.
type blankKey struct {
	QuizIndex  int `json:"quiz_index"`
	BlankIndex int `json:"blank_index"`
}

// answer is what the user picked for one blank, kept alongside whether it was
// right so a restored session can repaint the result without re-deriving it.
type answer struct {
	Choice  string `json:"choice"`
	Correct bool   `json:"correct"`
}

// scoreStore persists answers per proposal to a single JSON file. It is small
// enough to rewrite whole on every answer, which keeps the on-disk state
// consistent without a write-ahead scheme.
type scoreStore struct {
	path string

	mu     sync.Mutex
	scores map[string]map[blankKey]answer
}

// scoresFile is the JSON shape on disk. blankKey cannot be a map key in JSON,
// so entries are stored as a flat list per proposal.
type scoresFile struct {
	Proposals map[string][]scoreEntry `json:"proposals"`
}

type scoreEntry struct {
	QuizIndex  int    `json:"quiz_index"`
	BlankIndex int    `json:"blank_index"`
	Choice     string `json:"choice"`
	Correct    bool   `json:"correct"`
}

func newScoreStore(dir string) *scoreStore {
	s := &scoreStore{scores: map[string]map[blankKey]answer{}}
	if dir == "" {
		return s
	}
	s.path = filepath.Join(dir, "scores.json")
	s.load()
	return s
}

func (s *scoreStore) load() {
	raw, err := os.ReadFile(s.path)
	if err != nil {
		return
	}
	var f scoresFile
	if err := json.Unmarshal(raw, &f); err != nil {
		return
	}
	for id, entries := range f.Proposals {
		m := make(map[blankKey]answer, len(entries))
		for _, e := range entries {
			m[blankKey{e.QuizIndex, e.BlankIndex}] = answer{Choice: e.Choice, Correct: e.Correct}
		}
		s.scores[id] = m
	}
}

// flush rewrites the whole file. The caller holds s.mu.
func (s *scoreStore) flush() {
	if s.path == "" {
		return
	}
	f := scoresFile{Proposals: map[string][]scoreEntry{}}
	for id, m := range s.scores {
		entries := make([]scoreEntry, 0, len(m))
		for k, a := range m {
			entries = append(entries, scoreEntry{k.QuizIndex, k.BlankIndex, a.Choice, a.Correct})
		}
		f.Proposals[id] = entries
	}
	raw, err := json.Marshal(f)
	if err != nil {
		return
	}
	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, raw, 0o644); err != nil {
		return
	}
	_ = os.Rename(tmp, s.path)
}

// answers returns the document blanks answered so far, keyed by blank index.
func (s *scoreStore) answers(proposalID string) map[int]answer {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make(map[int]answer, len(s.scores[proposalID]))
	for k, v := range s.scores[proposalID] {
		if k.QuizIndex == 0 {
			out[k.BlankIndex] = v
		}
	}
	return out
}

func (s *scoreStore) record(proposalID string, k blankKey, a answer) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.scores[proposalID] == nil {
		s.scores[proposalID] = map[blankKey]answer{}
	}
	s.scores[proposalID][k] = a
	s.flush()
}

func (s *scoreStore) reset(proposalID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.scores, proposalID)
	s.flush()
}

// progress reports how many blanks of a proposal are answered and how many of
// those were correct.
func (s *scoreStore) progress(proposalID string) (answered, correct int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, a := range s.scores[proposalID] {
		answered++
		if a.Correct {
			correct++
		}
	}
	return answered, correct
}
