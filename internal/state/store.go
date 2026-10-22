// package state holds the job's environment. the runner owns one Store per job;
// it is the single source of truth that steps change through the job API and
// that every new step is spawned from
package state


import (
	"maps"
	"sync"
)

// Resolver fetches a secret's value on demand, state does not know about providers
// or config files, the config package supplies these.
type Resolver interface {
	Resolve()	(string, error)
	String()	string // the reference text, ex. "local:db/prod#password" safe to print
}

/*
	Entry is one variable. Exactly one of Value / Secret is meaningful,
	a literal is stored as is, a secret is stored as a reference and
	only fetched when a step is spawned (the "lazy" half of check-early-fetch-late)
*/
type Entry struct {
	Value		string
	Secret		Resolver
	Source		string // who put it here, for humans: `abc.config[staging] via step "load"`
}

func (e Entry) IsSecret() bool { return e.Secret != nil }

// Resolve returns the value a step should see
func (e Entry) Resolve() (string, error) {
	if e.Secret == nil {
		return e.Value, nil
	}
	return e.Secret.Resolve()
}

type Store struct {
	mu		sync.Mutex
	entries	map[string]Entry
}

func NewStore() *Store {
	return &Store{entries: map[string]Entry{}}
}

func (s *Store) Get(name string) (Entry, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	e, ok := s.entries[name]
	return e, ok
}

func (s *Store) Set(name string, e Entry) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.entries[name] = e
}

// SetMany applies a whole config load under one lock, so a step spawned 
// concurrently sees either none or all of it
func (s * Store) SetMany(m map[string]Entry) {
	s.mu.Lock()
	defer s.mu.Unlock()
	maps.Copy(s.entries, m)
}

func (s *Store) Delete(name string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.entries, name)
}

func (s *Store) Snapshot() map[string]Entry {
	s.mu.Lock()
	defer s.mu.Unlock()
	return maps.Clone(s.entries)
}