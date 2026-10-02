package weathering

import (
	"fmt"
	"math"
	"time"

	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/clock"
	"github.com/kjkrol/gram/plugins/atmosphere/air"
	"github.com/kjkrol/gram/plugins/atmosphere/calendar"
	"github.com/kjkrol/gram/plugins/board"
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/rule/effect"
)

// Config is what the weather does to the board: Snowy names, for each kind snow may lie on, the
// kind it is under snow — both kinds the game's, already in the board's dictionary, the snowy one
// the same ground to cross and stand on with another look; Ice the kind Water freezes into
// ("water" when Water is empty); Sway the kinds that sway in the wind, by Swaying (0.6 when zero);
// High where snow lies first and longest — high ground, nil for nowhere in particular; Seed the
// dice the weathering throws (1 when zero).
type Config struct {
	Snowy   map[string]string
	Ice     string
	Water   string
	Sway    []string
	Swaying float64
	High    func(c cell.ID) bool
	Seed    uint64
}

func (c Config) withDefaults() Config {
	if c.Water == "" {
		c.Water = "water"
	}
	if c.Swaying == 0 {
		c.Swaying = 0.6
	}
	if c.Seed == 0 {
		c.Seed = 1
	}
	return c
}

// How the weathering goes, a second at a time: the share of the board's cells snow settles on at a
// full fall, and melts off each degree above freezing; the share of the water that freezes each
// degree below iceBelow, and thaws each degree above freezing; the winds above which what sways
// sways and below which it stops.
const (
	snowSettles = 0.02
	snowMelts   = 0.004
	iceBelow    = -3
	iceSets     = 0.002
	iceThaws    = 0.003
	swayAbove   = 15
	swayBelow   = 10
	// snow first lies round the loosest few spots of the drifts' pattern (driftSeeds of it), and
	// next to snow already lying; a winter begun has it on driftWinter
	driftSeeds  = 0.12
	driftWinter = 0.7
	driftSize   = 5 // cells across one of the drifts' patches
)

// Weathering is the weather on the board: three effects on the board's cells — snow turns a
// cell's kind into its snowy one, ice turns water into ice, sway has a kind bend in the wind —
// cast as the world's weather says and taken off again, once a second of game time from the
// schedule. Snow settles on cells here and there while it snows in the frost and melts off them
// once it is warm; water freezes from the shore out in a hard frost and thaws; what sways sways
// while the wind blows. A winter begun has its snow and its shores' ice at once.
type Weathering struct {
	cfg      Config
	board    *board.Plugin
	air      func() air.Weather
	effects  *effect.Effects
	calendar *calendar.Calendar

	snow, ice, sway effect.Effect
	snowy           map[cell.Name]cell.Kind
	frozen          cell.Kind
	water           cell.Name
	swaying         map[cell.Name]bool
	cells           []cell.ID // every cell of the board, to pick from
	dice            uint64
	laid            bool // the first second has laid what a winter begun lies under
	still           bool // the weather works nothing on the board (SetRunning)
}

// SetRunning has the weather work on the board, or not: off, the ground stays as it is, the snow
// and the ice as they lie, what sways as it stands. It is not saved.
func (w *Weathering) SetRunning(on bool) { w.still = !on }

// Running reports whether the weather works on the board.
func (w *Weathering) Running() bool { return !w.still }

// New is the weathering of cfg on brd under the weather weather gives, in cal's seasons, its
// effects fx's. Call it once the kinds cfg names are in brd's dictionary, before the game is set
// up; it defines the effects at once.
func New(brd *board.Plugin, weather func() air.Weather, fx *effect.Effects, cal *calendar.Calendar, cfg Config) (*Weathering, error) {
	cfg = cfg.withDefaults()
	kinds := brd.CellKinds()
	ww := &Weathering{cfg: cfg, board: brd, air: weather, effects: fx, calendar: cal, dice: cfg.Seed,
		snowy: map[cell.Name]cell.Kind{}, water: cell.Named(cfg.Water), swaying: map[cell.Name]bool{}}
	for name, under := range cfg.Snowy {
		k, ok := kinds.Get(under)
		if !ok {
			return nil, fmt.Errorf("weathering: no kind %q for %q under snow", under, name)
		}
		ww.snowy[cell.Named(name)] = k
	}
	if cfg.Ice != "" {
		k, ok := kinds.Get(cfg.Ice)
		if !ok {
			return nil, fmt.Errorf("weathering: no kind %q for ice", cfg.Ice)
		}
		ww.frozen = k
	}
	for _, name := range cfg.Sway {
		ww.swaying[cell.Named(name)] = true
	}
	ww.snow = fx.Define("snow", effect.Spec{effect.Alter(func(g *cell.Ground) {
		if under, ok := ww.snowy[g.Kind.Name]; ok {
			under.Sway = g.Kind.Sway // what sways goes on swaying under snow
			g.Kind = under
		}
	})})
	ww.ice = fx.Define("ice", effect.Spec{effect.Alter(func(g *cell.Ground) {
		if g.Kind.Name == ww.water && ww.cfg.Ice != "" {
			g.Kind = ww.frozen
		}
	})})
	ww.sway = fx.Define("sway", effect.Spec{effect.Alter(func(g *cell.Ground) { g.Kind.Sway = ww.cfg.Swaying })})
	return ww, nil
}

// Effects are the weathering's: snow, ice and sway, for a game asking whether a cell lies under
// one (effect.Effects.Has).
func (w *Weathering) Effects() (snow, ice, sway effect.Effect) { return w.snow, w.ice, w.sway }

// System is the weathering's system, run in every step of the simulation on clk, the world's
// clock: once every second of game time it works the weather on the board.
func (w *Weathering) System(clk *clock.Clock) goke.System {
	second := clock.Every(time.Second, 0)
	var last time.Duration
	begun := false
	return goke.SystemFn{OnUpdate: func(cb *goke.CmdBuf, d time.Duration) {
		now := clk.Time() + d
		if !begun {
			last, begun = clk.Time(), true
		}
		if second(clock.Moment{Last: last, Now: now}) {
			w.second(cb)
		}
		last = now
	}}
}

// second is a second of the weather on the board.
func (w *Weathering) second(cb *goke.CmdBuf) {
	if w.still {
		return
	}
	air := w.air()
	if !w.laid {
		w.laid = true
		if w.calendar.Now().Season() == calendar.Winter {
			w.winter(cb)
		}
	}
	switch {
	case air.Temperature < 0 && air.Snow > 0.05:
		w.scatter(cb, snowSettles*air.Snow, w.snow, w.snowLies)
	case air.Temperature > 0:
		w.clear(snowMelts*air.Temperature, w.snow, w.snowEdge)
	}
	if w.cfg.Ice != "" {
		switch {
		case air.Temperature < iceBelow:
			w.scatter(cb, iceSets*(iceBelow-air.Temperature), w.ice, w.freezes)
		case air.Temperature > 0:
			w.clear(iceThaws*air.Temperature, w.ice, func(cell.ID) bool { return true })
		}
	}
	w.blow(cb, float32(math.Hypot(float64(air.Wind[0]), float64(air.Wind[1]))))
}

// roll throws the dice: a number from 0 up to 1 (xorshift).
func (w *Weathering) roll() float32 {
	x := w.dice
	x ^= x << 13
	x ^= x >> 7
	x ^= x << 17
	w.dice = x
	return float32(x>>40) / float32(1<<24)
}

// pick is a cell of the board thrown at random, on any grid.
func (w *Weathering) pick() cell.ID {
	if w.cells == nil {
		brd := w.board.Res.Logic.Board
		w.cells = make([]cell.ID, 0, brd.CellCount())
		brd.EachCell(func(c cell.ID) { w.cells = append(w.cells, c) })
	}
	return w.cells[min(int(w.roll()*float32(len(w.cells))), len(w.cells)-1)]
}

// scatter casts fx on about share of the board's cells picked at random, those may takes.
func (w *Weathering) scatter(cb *goke.CmdBuf, share float32, fx effect.Effect, may func(c cell.ID) bool) {
	for range int(share*float32(w.board.Res.Logic.Board.CellCount()) + w.roll()) {
		c := w.pick()
		if id, ok := w.board.CellEntity(c); ok && may(c) {
			w.effects.Cast(cb, id, fx)
		}
	}
}

// clear takes fx off about share of the board's cells picked at random, those may lets go.
func (w *Weathering) clear(share float32, fx effect.Effect, may func(c cell.ID) bool) {
	for range int(share*float32(w.board.Res.Logic.Board.CellCount()) + w.roll()) {
		c := w.pick()
		if id, ok := w.board.CellEntity(c); ok && fx.On(id) && may(c) {
			w.effects.Dispel(id, fx)
		}
	}
}

// winter lays what a winter begun lies under: snow in drifts over most of the board, ice along
// its shores.
func (w *Weathering) winter(cb *goke.CmdBuf) {
	w.board.Res.Logic.Board.EachCell(func(c cell.ID) {
		id, ok := w.board.CellEntity(c)
		if !ok {
			return
		}
		if w.takesSnow(c) && w.drift(c) < driftWinter {
			w.effects.Cast(cb, id, w.snow)
		}
		if w.cfg.Ice != "" && w.freezes(c) {
			w.effects.Cast(cb, id, w.ice)
		}
	})
}

// takesSnow reports whether snow may lie on c at all: a kind that has a snowy one.
func (w *Weathering) takesSnow(c cell.ID) bool {
	_, ok := w.snowy[w.board.Res.Logic.Board.Kind(c).Name]
	return ok
}

// snowLies reports whether falling snow settles on c now: ground that takes it, high, or next to
// snow lying already, or one of the drifts' seeds — so snow lies in patches that grow.
func (w *Weathering) snowLies(c cell.ID) bool {
	if !w.takesSnow(c) {
		return false
	}
	return w.cfg.High != nil && w.cfg.High(c) || w.drift(c) < driftSeeds || w.nextTo(c)
}

// snowEdge reports whether the snow on c melts now: the lonelier it lies — the fewer of its
// neighbours under snow — and the later it lay in the drifts' pattern, the likelier, so the
// patches shrink whole and what lay first lies longest.
func (w *Weathering) snowEdge(c cell.ID) bool {
	neighbours := w.board.Res.Logic.Board.Neighbors(c)
	under := 0
	for _, n := range neighbours {
		if id, ok := w.board.CellEntity(n); ok && w.effects.Has(id, w.snow) {
			under++
		}
	}
	lonely := 1 - float32(under)/float32(max(len(neighbours), 1))
	return w.roll() < lonely+w.drift(c)*0.5
}

// nextTo reports whether a neighbour of c lies under snow.
func (w *Weathering) nextTo(c cell.ID) bool {
	for _, n := range w.board.Res.Logic.Board.Neighbors(c) {
		if id, ok := w.board.CellEntity(n); ok && w.effects.Has(id, w.snow) {
			return true
		}
	}
	return false
}

// drift is the drifts' pattern at c, 0 to 1, smooth over driftSize cells: where it is low, snow
// lies first and longest.
func (w *Weathering) drift(c cell.ID) float32 {
	x, y, _ := w.board.Res.Logic.Board.Coords(c)
	fx, fy := float64(x)/driftSize, float64(y)/driftSize
	ix, iy := math.Floor(fx), math.Floor(fy)
	u, v := smooth(fx-ix), smooth(fy-iy)
	at := func(dx, dy float64) float64 { return cellHash(int(ix+dx), int(iy+dy)) }
	top := at(0, 0) + (at(1, 0)-at(0, 0))*u
	bottom := at(0, 1) + (at(1, 1)-at(0, 1))*u
	return float32(top + (bottom-top)*v)
}

func smooth(t float64) float64 { return t * t * (3 - 2*t) }

// cellHash is a number 0 to 1 fixed for the whole point (x, y).
func cellHash(x, y int) float64 {
	h := uint32(x)*374761393 + uint32(y)*668265263
	h = (h ^ h>>13) * 1274126177
	return float64(h^h>>16) / float64(math.MaxUint32)
}

// freezes reports whether c is water that may freeze now: next to land or to ice already, so the
// ice grows from the shore out.
func (w *Weathering) freezes(c cell.ID) bool {
	brd := w.board.Res.Logic.Board
	if brd.Kind(c).Name != w.water {
		return false
	}
	for _, n := range brd.Neighbors(c) {
		if brd.Kind(n).Name != w.water {
			return true
		}
	}
	return false
}

// blow has what sways sway once the wind blows harder than swayAbove and stop once it falls below
// swayBelow.
func (w *Weathering) blow(cb *goke.CmdBuf, wind float32) {
	if len(w.swaying) == 0 || wind > swayBelow && wind < swayAbove {
		return
	}
	brd := w.board.Res.Logic.Board
	brd.EachCell(func(c cell.ID) {
		if !w.swaying[brd.Kind(c).Name] {
			return
		}
		id, ok := w.board.CellEntity(c)
		if !ok {
			return
		}
		switch swaying := w.effects.Has(id, w.sway); {
		case wind > swayAbove && !swaying:
			w.effects.Cast(cb, id, w.sway)
		case wind < swayBelow && swaying:
			w.effects.Dispel(id, w.sway)
		}
	})
}
