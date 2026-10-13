package weathering

import (
	"fmt"
	cgoke "github.com/kjkrol/ceremony/goke"
	"math"
	"time"

	"github.com/kjkrol/ceremony/effect"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/clock"
	"github.com/kjkrol/gram/plugins/atmosphere/air"
	"github.com/kjkrol/gram/plugins/atmosphere/calendar"
	"github.com/kjkrol/gram/plugins/board"
	"github.com/kjkrol/gram/plugins/board/cell"
)

// Config is what the weather does to the board, in the game's own kinds: Snowy names, for each
// kind snow may lie on, the kind it is under snow; Ice the kind Water ("water" when empty)
// freezes into; Sway the kinds that sway in the wind, by Swaying (0.6 when zero); High where snow
// lies first and longest (nil: nowhere in particular); Seed the dice (1 when zero).
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

// shares of the board's cells a second; iceBelow in degrees Celsius, the winds in world units a
// second
const (
	snowSettles = 0.02
	snowMelts   = 0.004
	iceBelow    = -3
	iceSets     = 0.002
	iceThaws    = 0.003
	swayAbove   = 15
	swayBelow   = 10
	driftSeeds  = 0.12
	driftWinter = 0.7
	driftSize   = 5 // cells
)

// Weathering is the weather on the board: snow, ice and sway as effects on the board's cells,
// cast as the world's weather says and taken off again.
type Weathering struct {
	cfg      Config
	board    *board.Plugin
	air      func() air.Weather
	effects  *cgoke.Effects
	calendar *calendar.Calendar

	snow, ice, sway effect.Effect
	snowy           map[cell.Name]cell.Kind
	frozen          cell.Kind
	water           cell.Name
	swaying         map[cell.Name]bool
	cells           []cell.ID
	dice            uint64
	laid            bool
	still           bool
}

// SetRunning has the weather work on the board, or not; off, the ground stays as it is. It is
// not saved.
func (w *Weathering) SetRunning(on bool) { w.still = !on }

// Running reports whether the weather works on the board.
func (w *Weathering) Running() bool { return !w.still }

// New is the weathering of cfg on brd under the weather weather gives, its effects defined in
// fx at once. The kinds cfg names are looked up as the weather first touches the board; an
// unknown name panics then.
func New(brd *board.Plugin, weather func() air.Weather, fx *cgoke.Effects, cal *calendar.Calendar, cfg Config) (*Weathering, error) {
	cfg = cfg.withDefaults()
	ww := &Weathering{cfg: cfg, board: brd, air: weather, effects: fx, calendar: cal, dice: cfg.Seed,
		water: cell.Named(cfg.Water), swaying: map[cell.Name]bool{}}
	for _, name := range cfg.Sway {
		ww.swaying[cell.Named(name)] = true
	}
	fx.Define("snow", effect.Spec{effect.Alter(func(g *cell.Ground) {
		ww.ready()
		if under, ok := ww.snowy[g.Kind.Name]; ok {
			under.Sway = g.Kind.Sway
			g.Kind = under
		}
	})})
	ww.snow = fx.Named("snow")
	fx.Define("ice", effect.Spec{effect.Alter(func(g *cell.Ground) {
		ww.ready()
		if g.Kind.Name == ww.water && ww.cfg.Ice != "" {
			g.Kind = ww.frozen
		}
	})})
	ww.ice = fx.Named("ice")
	fx.Define("sway", effect.Spec{effect.Alter(func(g *cell.Ground) { g.Kind.Sway = ww.cfg.Swaying })})
	ww.sway = fx.Named("sway")
	return ww, nil
}

func (w *Weathering) ready() {
	if w.snowy != nil {
		return
	}
	kinds := w.board.CellKinds()
	snowy := map[cell.Name]cell.Kind{}
	for name, under := range w.cfg.Snowy {
		k, ok := kinds.Get(under)
		if !ok {
			panic(fmt.Sprintf("weathering: no kind %q for %q under snow", under, name))
		}
		snowy[cell.Named(name)] = k
	}
	if w.cfg.Ice != "" {
		k, ok := kinds.Get(w.cfg.Ice)
		if !ok {
			panic(fmt.Sprintf("weathering: no kind %q for ice", w.cfg.Ice))
		}
		w.frozen = k
	}
	w.snowy = snowy
}

// Effects are the weathering's three effects.
func (w *Weathering) Effects() (snow, ice, sway effect.Effect) { return w.snow, w.ice, w.sway }

// System is the weathering's system, run in every step of the simulation on clk, working once a
// second of game time.
func (w *Weathering) System(clk *clock.Clock) goke.System {
	second := clock.Every(time.Second, 0)
	var last time.Duration
	begun := false
	return goke.SystemFn{OnUpdate: func(cb *goke.CmdBuf, d time.Duration) {
		now := clk.Time() + d
		if !begun {
			last, begun = clk.Time(), true
		}
		if second.Holds(clock.Moment{Last: last, Now: now}) {
			w.second(cb)
		}
		last = now
	}}
}

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

// xorshift; 0 up to 1
func (w *Weathering) roll() float32 {
	x := w.dice
	x ^= x << 13
	x ^= x >> 7
	x ^= x << 17
	w.dice = x
	return float32(x>>40) / float32(1<<24)
}

func (w *Weathering) pick() cell.ID {
	if w.cells == nil {
		brd := w.board.Board()
		w.cells = make([]cell.ID, 0, brd.CellCount())
		brd.EachCell(func(c cell.ID) { w.cells = append(w.cells, c) })
	}
	return w.cells[min(int(w.roll()*float32(len(w.cells))), len(w.cells)-1)]
}

func (w *Weathering) scatter(cb *goke.CmdBuf, share float32, fx effect.Effect, may func(c cell.ID) bool) {
	for range int(share*float32(w.board.Board().CellCount()) + w.roll()) {
		c := w.pick()
		if id, ok := w.board.CellEntity(c); ok && may(c) {
			w.effects.Cast(id, fx)
		}
	}
}

func (w *Weathering) clear(share float32, fx effect.Effect, may func(c cell.ID) bool) {
	for range int(share*float32(w.board.Board().CellCount()) + w.roll()) {
		c := w.pick()
		if id, ok := w.board.CellEntity(c); ok && w.effects.Has(id, fx) && may(c) {
			w.effects.Dispel(id, fx)
		}
	}
}

func (w *Weathering) winter(cb *goke.CmdBuf) {
	w.board.Board().EachCell(func(c cell.ID) {
		id, ok := w.board.CellEntity(c)
		if !ok {
			return
		}
		if w.takesSnow(c) && w.drift(c) < driftWinter {
			w.effects.Cast(id, w.snow)
		}
		if w.cfg.Ice != "" && w.freezes(c) {
			w.effects.Cast(id, w.ice)
		}
	})
}

func (w *Weathering) takesSnow(c cell.ID) bool {
	w.ready()
	_, ok := w.snowy[w.board.Board().Kind(c).Name]
	return ok
}

// high ground, the drifts' seeds or next to snow lying: patches that grow
func (w *Weathering) snowLies(c cell.ID) bool {
	if !w.takesSnow(c) {
		return false
	}
	return w.cfg.High != nil && w.cfg.High(c) || w.drift(c) < driftSeeds || w.nextTo(c)
}

// the lonelier the snow and the later it lay, the likelier it melts: patches shrink whole
func (w *Weathering) snowEdge(c cell.ID) bool {
	neighbours := w.board.Board().Neighbors(c)
	under := 0
	for _, n := range neighbours {
		if id, ok := w.board.CellEntity(n); ok && w.effects.Has(id, w.snow) {
			under++
		}
	}
	lonely := 1 - float32(under)/float32(max(len(neighbours), 1))
	return w.roll() < lonely+w.drift(c)*0.5
}

func (w *Weathering) nextTo(c cell.ID) bool {
	for _, n := range w.board.Board().Neighbors(c) {
		if id, ok := w.board.CellEntity(n); ok && w.effects.Has(id, w.snow) {
			return true
		}
	}
	return false
}

// 0 to 1; where it is low, snow lies first and longest
func (w *Weathering) drift(c cell.ID) float32 {
	x, y, _ := w.board.Board().Coords(c)
	fx, fy := float64(x)/driftSize, float64(y)/driftSize
	ix, iy := math.Floor(fx), math.Floor(fy)
	u, v := smooth(fx-ix), smooth(fy-iy)
	at := func(dx, dy float64) float64 { return cellHash(int(ix+dx), int(iy+dy)) }
	top := at(0, 0) + (at(1, 0)-at(0, 0))*u
	bottom := at(0, 1) + (at(1, 1)-at(0, 1))*u
	return float32(top + (bottom-top)*v)
}

func smooth(t float64) float64 { return t * t * (3 - 2*t) }

func cellHash(x, y int) float64 {
	h := uint32(x)*374761393 + uint32(y)*668265263
	h = (h ^ h>>13) * 1274126177
	return float64(h^h>>16) / float64(math.MaxUint32)
}

// water next to land or ice: the ice grows from the shore out
func (w *Weathering) freezes(c cell.ID) bool {
	brd := w.board.Board()
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

func (w *Weathering) blow(cb *goke.CmdBuf, wind float32) {
	if len(w.swaying) == 0 || wind > swayBelow && wind < swayAbove {
		return
	}
	brd := w.board.Board()
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
			w.effects.Cast(id, w.sway)
		case wind < swayBelow && swaying:
			w.effects.Dispel(id, w.sway)
		}
	})
}
