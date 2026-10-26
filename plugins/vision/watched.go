package vision

import (
	"github.com/kjkrol/ceremony/store"
	"github.com/kjkrol/goke/v3"
)

// Watched marks an observer whose Sighted the game reads itself, in a system, a plan or a fact:
// vision scans it though no rule of a Sighting is for it.
type Watched struct{}

// whether something reads every observer of the block: the plugin scanning all, a rule for
// every one, SightOutline or Watched on the block
func (s *ScanSystem) watchedAll(cursor *goke.Cursor) bool {
	return s.all || s.anyone || s.outline.Present(cursor) || s.watch.Present(cursor)
}

// whether a hosted rule of a Sighting is for the i-th observer of the walk's row
func (s *ScanSystem) isFor(row store.Row, i int) bool {
	return s.anyone || s.host.For(s.host.Read(walked, row, i))
}
