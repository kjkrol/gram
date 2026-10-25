package weathering

import (
	"math"
	"time"

	"github.com/kjkrol/ceremony/cond"
	cgoke "github.com/kjkrol/ceremony/goke"
	"github.com/kjkrol/ceremony/host"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/clock"
	"github.com/kjkrol/gram/plugins/atmosphere/air"
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/uid"
)

var _ goke.System = (*system)(nil)

type system struct {
	w      *Weathering
	clk    *clock.Clock
	tick   host.TickSource
	second cond.Word[clock.Moment]
	last   time.Duration
	begun  bool

	cells   *goke.Query
	cols    *cgoke.Columns
	plot    goke.Comp[cell.Plot]
	wring   goke.Comp[Wring]
	bare    *goke.Query // the cells without their knob yet
	wringID goke.CompID
	walked  int       // of the cells, this second
	walks   int       // seconds walked
	heights []float32 // the cells', in the order walked, each worked out anew every few seconds
	bases   []float32 // the low clouds' over them, as seldom

	// the block of cells walked now, for the rules' moments
	ids     []uid.UID64
	plots   []cell.Plot
	from    int // of the block, the first walked now
	to      int
	at      int // where the first walked now lies in heights
	air     air.Weather
	hour    time.Duration
	about   func(i int) Overhead // bound once
	walking func(i int) bool
}

func (s *system) Init(si *goke.SysInit) {
	s.wringID = si.RegComp[Wring]()
	var plot goke.Comp[cell.Plot]
	s.bare = si.NewQueryBuilder(&plot).Exclude(goke.Exclude[Wring]()).Build()
	qb := si.NewQueryBuilder(&s.plot, &s.wring)
	s.cols = cgoke.NewColumns(qb)
	s.w.overheads.Bind(s.cols)
	s.cells = qb.Build()
	s.about, s.walking = s.overhead, func(i int) bool { return i >= s.from && i < s.to }
}

func (s *system) Update(cb *goke.CmdBuf, d time.Duration) {
	now := s.clk.Read().Time() + d
	if !s.begun {
		s.last, s.begun = s.clk.Read().Time(), true
	}
	if s.second.Holds(clock.Moment{Last: s.last, Now: now}) {
		s.w.second(cb)
	}
	s.last = now
	for s.bare.All(); s.bare.Next(); {
		for _, e := range s.bare.Cursor().IDs {
			cb.AddOne(e, s.wringID, Wring{})
		}
	}
	s.walk(d)
}

// as many of the board's cells as d's share of a second of game time: their rules of an
// Overhead heard, and noted on the land, which turns them in once all are walked
func (s *system) walk(d time.Duration) {
	brd := s.w.board.Read(0).Board()
	n := int(math.Ceil(float64(brd.CellCount()) * d.Seconds()))
	heights := s.w.board.Read(0).Heights()
	rules := !s.w.overheads.Empty()
	s.air = s.w.air()
	if rules {
		s.hour = time.Duration(float64(s.w.calendar.Now().Time) * float64(24*time.Hour))
	}
	tick := s.tick.Of(d)
	row := 0
	for s.cells.All(); s.cells.Next(); {
		cursor := s.cells.Cursor()
		plots, wrings := s.plot.Slice(cursor), s.wring.Slice(cursor)
		s.from = max(s.walked-row, 0)
		s.to, s.at = s.from, s.walked
		for i := s.from; i < len(plots) && n > 0; i++ {
			p := brd.CellCenter(plots[i].Cell)
			if s.walked == len(s.heights) {
				s.heights, s.bases = append(s.heights, 0), append(s.bases, 0)
			}
			if (s.walked+s.walks)%anew == 0 || s.walks == 0 {
				s.heights[s.walked] = 0
				if heights != nil {
					s.heights[s.walked] = heights.At(p)
				}
				s.bases[s.walked], _ = s.air.LowBand(p.X, p.Y)
			}
			s.w.land.Note(p.X, p.Y, s.heights[s.walked], wrings[i].Falls,
				wrings[i].Gathers)
			s.walked++
			s.to++
			n--
		}
		if rules && s.to > s.from {
			s.ids, s.plots = cursor.IDs, plots
			s.w.overheads.RunWhere(tick, s.cols.Row(cursor), s.to, s.walking, s.about)
		}
		row += len(plots)
	}
	if s.walked >= row {
		s.w.land.Turn()
		s.walked = 0
		s.walks++
	}
}

// the moment of the i-th cell of the block walked now
func (s *system) overhead(i int) Overhead {
	c := s.plots[i].Cell
	p := s.w.board.Read(0).Board().CellCenter(c)
	m := s.air.At(p.X, p.Y)
	o := Overhead{ID: s.ids[i], Cell: c, Air: m, Low: s.air.Share(m, air.Low).Cover, Hour: s.hour}
	if k := s.at + i - s.from; s.bases[k] > 0 {
		o.Reach = s.heights[k] / s.bases[k]
	}
	return o
}

// seconds a cell's height and the low clouds' base over it are kept before they are worked out
// anew: the ground reshaped tells so late, and the bands drift slowly
const anew = 8
