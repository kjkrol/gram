package relief

import (
	"time"

	"github.com/kjkrol/goke/v3"
)

// HeightsSystem is the goke.System keeping r's heights on entities of their own (Heights), saved
// with the game. Run it first of all, so a loaded game's heights are the relief's before anything
// reads them.
func (r *Relief) HeightsSystem() *HeightsSystem { return &HeightsSystem{relief: r} }

var _ goke.System = (*HeightsSystem)(nil)

// HeightsSystem keeps a relief's heights on its entities as the relief changes: as it runs, and
// as Fill is called between its runs.
type HeightsSystem struct {
	relief  *Relief
	query   *goke.Query
	comp    goke.Comp[Heights]
	spawned goke.Comp[Heights]
	written uint64 // one more than the relief's version the runs hold; 0 none
}

func (s *HeightsSystem) Init(si *goke.SysInit) {
	s.query = si.NewQueryBuilder(&s.comp).Build()
	var loaded []Heights
	n := 0
	for s.query.All(); s.query.Next(); {
		runs := s.comp.Slice(s.query.Cursor())
		loaded = append(loaded, runs...)
		for _, h := range runs {
			n += int(h.Count)
		}
	}
	if len(loaded) == 0 {
		s.spawn(si, 0)
		s.written = s.relief.Version() + 1
		return
	}
	if !s.relief.adopt(loaded) {
		return
	}
	if n != len(s.relief.values) {
		// a hex relief saved by cell: its runs laid anew by corner, as many more as they take
		s.refill()
		s.spawn(si, len(loaded))
	}
	s.written = s.relief.Version() + 1
}

// the relief's runs from the from-th on
func (s *HeightsSystem) spawn(si *goke.SysInit, from int) {
	if from >= s.relief.Runs() {
		return
	}
	f := si.NewFactory(&s.spawned)
	f.Create(s.relief.Runs() - from)
	run := from
	for f.Next() {
		for i := range f.Cursor.IDs {
			h := &s.spawned.Slice(&f.Cursor)[i]
			h.First = uint32(run * HeightsRun)
			s.relief.fill(h)
			run++
		}
	}
}

func (s *HeightsSystem) Update(*goke.CmdBuf, time.Duration) { s.Fill() }

// Fill writes the relief's heights on the entities anew if it changed since they were last
// written, between the system's runs too; nothing before the system is set up.
func (s *HeightsSystem) Fill() {
	if s.query != nil && s.written != s.relief.Version()+1 {
		s.refill()
	}
}

func (s *HeightsSystem) refill() {
	for s.query.All(); s.query.Next(); {
		runs := s.comp.Slice(s.query.Cursor())
		for i := range runs {
			s.relief.fill(&runs[i])
		}
	}
	s.written = s.relief.Version() + 1
}
