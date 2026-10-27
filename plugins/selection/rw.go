package selection

import (
	"github.com/kjkrol/ceremony/command"
	"github.com/kjkrol/ceremony/tag"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/internal/parallel"
	"github.com/kjkrol/gram/plugins/players/owner"
	"github.com/kjkrol/uid"
)

// Read reads the selection on one goroutine at a time: Plugin.Read(w), worker w's, through a
// query of its own.
type Read struct {
	p     *Plugin
	query *goke.Query // nil until the selection's system is set up
}

var _ parallel.Forkable[*Read] = (*Read)(nil)

// Read reads the selection on worker w of the game's pool, 0 the game's own goroutine; the other
// workers' reads come with Install.
func (p *Plugin) Read(w int) *Read { return p.reads[w] }

// Fork reads the same selection through a query of its own, for another goroutine; made before
// the selection's system is set up, it gets it then.
func (r *Read) Fork() *Read {
	f := &Read{p: r.p}
	if q := r.p.read.query; q != nil {
		f.query = q.Fork()
	}
	r.p.forks = append(r.p.forks, f)
	return f
}

// readied gives the reads the selection system's query, as it is set up
func (p *Plugin) readied() {
	q := p.module.sys.query
	p.read.query = q
	for _, f := range p.forks {
		f.query = q.Fork()
	}
}

// Chosen is the one Selected unit player by owns; false with none, or several. Nothing before
// the plugin is installed.
func (r *Read) Chosen(by command.Issuer) (uid.UID64, bool) {
	q := r.query
	if q == nil {
		return 0, false
	}
	s := r.p.module.sys
	var one uid.UID64
	n := 0
	for q.All(); q.Next(); {
		cur := q.Cursor()
		owners := s.owners.Slice(cur)
		for i, m := range s.marks.Slice(cur) {
			if m.Has(s.tags.Selected) && owner.Obeys(ownersAt(owners, i), by) {
				one, n = cur.IDs[i], n+1
			}
		}
	}
	return one, n == 1
}

// Any reports whether by has a Selected unit it commands; false before the plugin is installed.
func (r *Read) Any(by command.Issuer) bool {
	q := r.query
	if q == nil {
		return false
	}
	s := r.p.module.sys
	for q.All(); q.Next(); {
		cur := q.Cursor()
		owners := s.owners.Slice(cur)
		for i, m := range s.marks.Slice(cur) {
			if m.Has(s.tags.Selected) && owner.Obeys(ownersAt(owners, i), by) {
				return true
			}
		}
	}
	return false
}

// IsSelected reports whether an entity carrying marks is selected: a drawing rule's condition.
func (r *Read) IsSelected(marks tag.Tags[Family]) bool { return marks.Has(r.p.tags.Selected) }
