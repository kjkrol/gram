package sky

import (
	"time"

	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/control"
	"github.com/kjkrol/gram/plugins/world"
)

var _ goke.System = (*skySystem)(nil)

// skySystem keeps the day going: it finds the sky's entity, or makes one at Config.Start, moves its
// time on every tick at its pace, and puts the sun of the time into the world at every step.
type skySystem struct {
	cfg     Config
	world   *world.Plugin
	forward *control.Queue[Forward]
	back    *control.Queue[Back]
	pause   *control.Queue[Pause]

	query *goke.Query
	day   goke.Comp[Day]
	spawn goke.Comp[Day]
	step  int // the step whose sun the world has; -1 before the first
}

func newSkySystem(cfg Config, w *world.Plugin, forward *control.Queue[Forward], back *control.Queue[Back], pause *control.Queue[Pause]) *skySystem {
	return &skySystem{cfg: cfg, world: w, forward: forward, back: back, pause: pause, step: -1}
}

// hour is an hour of a day; a stopped day moves by half of one.
const hour = float32(1) / 24

func (s *skySystem) Init(si *goke.SysInit) {
	s.query = si.NewQueryBuilder(&s.day).Build()
	for s.query.All(); s.query.Next(); {
		return // a loaded game brought its sky
	}
	f := si.NewFactory(&s.spawn)
	f.Create(1)
	for f.Next() {
		s.spawn.Slice(&f.Cursor)[0] = Day{Time: s.cfg.Start, Pace: 1, Date: s.cfg.midst(s.cfg.Season), Calendar: s.cfg.Calendar}
	}
}

func (s *skySystem) Update(_ *goke.CmdBuf, d time.Duration) {
	for s.query.All(); s.query.Next(); {
		day := &s.day.Slice(s.query.Cursor())[0]
		day.Length = s.cfg.Length // the day is as long as the sky has it now
		s.pause.Drain(func(control.Issued[Pause]) { day.Stopped = !day.Stopped })
		s.forward.Drain(func(control.Issued[Forward]) {
			if day.Stopped {
				day.passing(hour / 2)
			} else {
				day.Pace *= 2
			}
		})
		s.back.Drain(func(control.Issued[Back]) {
			if day.Stopped {
				day.passing(-hour / 2)
			} else {
				day.Pace /= 2
			}
		})
		if !day.Stopped {
			day.passing(float32(d.Seconds() / s.cfg.Length.Seconds() * float64(day.Pace)))
		}
		// a hair over, so a time moved onto a step by halves is on it, not a rounding short of it
		step := int(day.Time*float32(s.cfg.Steps)+1e-3) % s.cfg.Steps
		if at := int(day.Date)*s.cfg.Steps + step; at != s.step {
			s.step = at
			then := *day
			then.Time = float32(step) / float32(s.cfg.Steps)
			s.world.SetSun(s.cfg.LightAt(then.OfYear(), then.Time, then.Moon()))
		}
		return
	}
}
