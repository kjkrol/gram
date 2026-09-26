package sky

import (
	"math"
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
		s.spawn.Slice(&f.Cursor)[0] = Day{Time: s.cfg.Start, Pace: 1}
	}
}

func (s *skySystem) Update(_ *goke.CmdBuf, d time.Duration) {
	for s.query.All(); s.query.Next(); {
		day := &s.day.Slice(s.query.Cursor())[0]
		s.pause.Drain(func(control.Issued[Pause]) { day.Stopped = !day.Stopped })
		s.forward.Drain(func(control.Issued[Forward]) {
			if day.Stopped {
				day.Time = wrap(day.Time + hour/2)
			} else {
				day.Pace *= 2
			}
		})
		s.back.Drain(func(control.Issued[Back]) {
			if day.Stopped {
				day.Time = wrap(day.Time - hour/2)
			} else {
				day.Pace /= 2
			}
		})
		if !day.Stopped {
			day.Time = wrap(day.Time + float32(d.Seconds()/s.cfg.Length.Seconds()*float64(day.Pace)))
		}
		// a hair over, so a time moved onto a step by halves is on it, not a rounding short of it
		if step := int(day.Time*float32(s.cfg.Steps)+1e-3) % s.cfg.Steps; step != s.step {
			s.step = step
			s.world.SetSun(s.cfg.SunAt(float32(step) / float32(s.cfg.Steps)))
		}
		return
	}
}

// wrap is t within one day, 0 up to 1.
func wrap(t float32) float32 {
	t = float32(math.Mod(float64(t), 1))
	if t < 0 {
		t++
	}
	return t
}
