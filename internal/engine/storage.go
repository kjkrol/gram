package engine

import (
	"bytes"
	"encoding/gob"
	"fmt"
	"sync"

	"github.com/kjkrol/gram/plugin"
)

// keyed by name, so a save survives plugins added, removed or reordered
type storage struct {
	mu    sync.Mutex
	items map[string]plugin.Serializable
}

func newStorage() *storage {
	return &storage{items: make(map[string]plugin.Serializable)}
}

func (s *storage) register(name string, v plugin.Serializable) {
	s.mu.Lock()
	defer s.mu.Unlock()
	verifyPersisted(name, v)
	s.items[name] = v
}

func verifyPersisted(name string, v plugin.Serializable) {
	var buf bytes.Buffer
	enc := gob.NewEncoder(&buf)
	for _, target := range v.Persisted() {
		if err := enc.Encode(target); err != nil {
			panic(fmt.Sprintf("gram: %q (%T).Persisted() returned an unencodable value %T: %v", name, v, target, err))
		}
	}
}

func (s *storage) persisted() map[string][]any {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make(map[string][]any, len(s.items))
	for name, v := range s.items {
		out[name] = v.Persisted()
	}
	return out
}
