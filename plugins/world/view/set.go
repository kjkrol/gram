package view

import (
	"math/bits"

	"github.com/kjkrol/uid"
)

// EntitySet is a set of the world's entities by index — a View's answer to "who is in these
// bounds", a gesture's answer to "who was picked". It knows nothing of generations: fill it only
// with ids of entities alive at the time, from the Space or a query, never from a game's stale copy.
// The zero value is empty and ready.
type EntitySet struct{ words []uint64 }

// Clear empties the set, keeping its memory.
func (s *EntitySet) Clear() { clear(s.words) }

// Add puts id in the set, growing it as far as the index needs.
func (s *EntitySet) Add(id uid.UID64) {
	i := id.Index()
	w := int(i >> 6)
	for w >= len(s.words) {
		s.words = append(s.words, 0)
	}
	s.words[w] |= 1 << (i & 63)
}

// Has reports whether id is in the set; an index past its size is not.
func (s *EntitySet) Has(id uid.UID64) bool {
	i := id.Index()
	w := int(i >> 6)
	return w < len(s.words) && s.words[w]&(1<<(i&63)) != 0
}

// Len is how many entities the set holds.
func (s *EntitySet) Len() int {
	n := 0
	for _, w := range s.words {
		n += bits.OnesCount64(w)
	}
	return n
}
