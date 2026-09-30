package relief

import (
	"time"

	"github.com/kjkrol/goke/v3"
)

// HeightsSystem is the goke.System keeping r's heights on entities of their own (Heights), saved
// with the game: first of all, so a loaded game's heights are the relief's before anything reads
// them; once a tick.
func (r *Relief) HeightsSystem() goke.System { return &heightsSystem{relief: r} }

var _ goke.System = (*heightsSystem)(nil)

// heightsSystem keeps the ground's heights on entities of their own, a run of them each (Heights): found after a load — the relief takes them over — or made at Setup; written anew
// whenever the relief has changed.
type heightsSystem struct {
	relief  *Relief
	query   *goke.Query
	comp    goke.Comp[Heights]
	spawn   goke.Comp[Heights]
	written uint64 // one more than the relief's version the runs hold; 0 none
}

func (s *heightsSystem) Init(si *goke.SysInit) {
	s.query = si.NewQueryBuilder(&s.comp).Build()
	var loaded []Heights
	for s.query.All(); s.query.Next(); {
		loaded = append(loaded, s.comp.Slice(s.query.Cursor())...)
	}
	if len(loaded) > 0 {
		if s.relief.adopt(loaded) {
			s.written = s.relief.Version() + 1
		}
		return
	}
	f := si.NewFactory(&s.spawn)
	f.Create(s.relief.Runs())
	run := 0
	for f.Next() {
		for i := range f.Cursor.IDs {
			h := &s.spawn.Slice(&f.Cursor)[i]
			h.First = uint32(run * HeightsRun)
			s.relief.fill(h)
			run++
		}
	}
	s.written = s.relief.Version() + 1
}

func (s *heightsSystem) Update(*goke.CmdBuf, time.Duration) {
	if s.written == s.relief.Version()+1 {
		return
	}
	for s.query.All(); s.query.Next(); {
		runs := s.comp.Slice(s.query.Cursor())
		for i := range runs {
			s.relief.fill(&runs[i])
		}
	}
	s.written = s.relief.Version() + 1
}
