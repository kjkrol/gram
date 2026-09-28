package weathering

import (
	"fmt"
	"math"
	"time"

	"github.com/kjkrol/gram/plugin"
	"github.com/kjkrol/gram/plugins/atmosphere/calendar"
	"github.com/kjkrol/gram/plugins/board"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/plugins/world/effects"
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
	High    func(c board.CellID) bool
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
	world    *world.Plugin
	effects  *effects.Effects
	calendar *calendar.Calendar

	snow, ice, sway effects.ID
	snowy           map[board.Name]board.CellKind
	frozen          board.CellKind
	water           board.Name
	swaying         map[board.Name]bool
	cells           []board.CellID // every cell of the board, to pick from
	dice            uint64
	laid            bool // the first second has laid what a winter begun lies under
}

// New is the weathering of cfg on brd under w's weather, in cal's seasons, its effects fx's. Call
// it once the kinds cfg names are in brd's dictionary, before the game is set up; it defines the
// effects at once.
func New(brd *board.Plugin, w *world.Plugin, fx *effects.Effects, cal *calendar.Calendar, cfg Config) (*Weathering, error) {
	cfg = cfg.withDefaults()
	kinds := brd.CellKindDict()
	ww := &Weathering{cfg: cfg, board: brd, world: w, effects: fx, calendar: cal, dice: cfg.Seed,
		snowy: map[board.Name]board.CellKind{}, water: board.Named(cfg.Water), swaying: map[board.Name]bool{}}
	for name, under := range cfg.Snowy {
		k, ok := kinds.Get(under)
		if !ok {
			return nil, fmt.Errorf("weathering: no kind %q for %q under snow", under, name)
		}
		ww.snowy[board.Named(name)] = k
	}
	if cfg.Ice != "" {
		k, ok := kinds.Get(cfg.Ice)
		if !ok {
			return nil, fmt.Errorf("weathering: no kind %q for ice", cfg.Ice)
		}
		ww.frozen = k
	}
	for _, name := range cfg.Sway {
		ww.swaying[board.Named(name)] = true
	}
	ww.snow = fx.Define("snow", effects.Spec{effects.Alter(func(g *board.Ground) {
		if under, ok := ww.snowy[g.Kind.Name]; ok {
			under.Sway = g.Kind.Sway // what sways goes on swaying under snow
			g.Kind = under
		}
	})})
	ww.ice = fx.Define("ice", effects.Spec{effects.Alter(func(g *board.Ground) {
		if g.Kind.Name == ww.water && ww.cfg.Ice != "" {
			g.Kind = ww.frozen
		}
	})})
	ww.sway = fx.Define("sway", effects.Spec{effects.Alter(func(g *board.Ground) { g.Kind.Sway = ww.cfg.Swaying })})
	return ww, nil
}

// Effects are the weathering's: snow, ice and sway, for a game asking whether a cell lies under
// one (effects.Effects.Has).
func (w *Weathering) Effects() (snow, ice, sway effects.ID) { return w.snow, w.ice, w.sway }

// Schedule lays the weathering on s: every second of game time.
func (w *Weathering) Schedule(s *effects.Schedule) { s.Every(time.Second, 0, w.second) }

// second is a second of the weather on the board.
func (w *Weathering) second(t plugin.Tick) {
	air := w.world.Weather()
	if !w.laid {
		w.laid = true
		if w.calendar.Now().Season() == calendar.Winter {
			w.winter(t)
		}
	}
	switch {
	case air.Temperature < 0 && air.Snow > 0.05:
		w.scatter(t, snowSettles*air.Snow, w.snow, w.snowLies)
	case air.Temperature > 0:
		w.clear(snowMelts*air.Temperature, w.snow, w.snowEdge)
	}
	if w.cfg.Ice != "" {
		switch {
		case air.Temperature < iceBelow:
			w.scatter(t, iceSets*(iceBelow-air.Temperature), w.ice, w.freezes)
		case air.Temperature > 0:
			w.clear(iceThaws*air.Temperature, w.ice, func(board.CellID) bool { return true })
		}
	}
	w.blow(t, float32(math.Hypot(float64(air.Wind[0]), float64(air.Wind[1]))))
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
func (w *Weathering) pick() board.CellID {
	if w.cells == nil {
		brd := w.board.Res.Logic.Board
		w.cells = make([]board.CellID, 0, brd.CellCount())
		brd.EachCell(func(c board.CellID) { w.cells = append(w.cells, c) })
	}
	return w.cells[min(int(w.roll()*float32(len(w.cells))), len(w.cells)-1)]
}

// scatter casts effect on about share of the board's cells picked at random, those may takes.
func (w *Weathering) scatter(t plugin.Tick, share float32, effect effects.ID, may func(c board.CellID) bool) {
	for range int(share*float32(w.board.Res.Logic.Board.CellCount()) + w.roll()) {
		c := w.pick()
		if id, ok := w.board.CellEntity(c); ok && may(c) {
			w.effects.Cast(t.CmdBuf, id, effect)
		}
	}
}

// clear takes effect off about share of the board's cells picked at random, those may lets go.
func (w *Weathering) clear(share float32, effect effects.ID, may func(c board.CellID) bool) {
	for range int(share*float32(w.board.Res.Logic.Board.CellCount()) + w.roll()) {
		c := w.pick()
		if id, ok := w.board.CellEntity(c); ok && w.effects.Has(id, effect) && may(c) {
			w.effects.Dispel(id, effect)
		}
	}
}

// winter lays what a winter begun lies under: snow in drifts over most of the board, ice along
// its shores.
func (w *Weathering) winter(t plugin.Tick) {
	w.board.Res.Logic.Board.EachCell(func(c board.CellID) {
		id, ok := w.board.CellEntity(c)
		if !ok {
			return
		}
		if w.takesSnow(c) && w.drift(c) < driftWinter {
			w.effects.Cast(t.CmdBuf, id, w.snow)
		}
		if w.cfg.Ice != "" && w.freezes(c) {
			w.effects.Cast(t.CmdBuf, id, w.ice)
		}
	})
}

// takesSnow reports whether snow may lie on c at all: a kind that has a snowy one.
func (w *Weathering) takesSnow(c board.CellID) bool {
	_, ok := w.snowy[w.board.Res.Logic.Board.Kind(c).Name]
	return ok
}

// snowLies reports whether falling snow settles on c now: ground that takes it, high, or next to
// snow lying already, or one of the drifts' seeds — so snow lies in patches that grow.
func (w *Weathering) snowLies(c board.CellID) bool {
	if !w.takesSnow(c) {
		return false
	}
	return w.cfg.High != nil && w.cfg.High(c) || w.drift(c) < driftSeeds || w.nextTo(c)
}

// snowEdge reports whether the snow on c melts now: the lonelier it lies — the fewer of its
// neighbours under snow — and the later it lay in the drifts' pattern, the likelier, so the
// patches shrink whole and what lay first lies longest.
func (w *Weathering) snowEdge(c board.CellID) bool {
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
func (w *Weathering) nextTo(c board.CellID) bool {
	for _, n := range w.board.Res.Logic.Board.Neighbors(c) {
		if id, ok := w.board.CellEntity(n); ok && w.effects.Has(id, w.snow) {
			return true
		}
	}
	return false
}

// drift is the drifts' pattern at c, 0 to 1, smooth over driftSize cells: where it is low, snow
// lies first and longest.
func (w *Weathering) drift(c board.CellID) float32 {
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
func (w *Weathering) freezes(c board.CellID) bool {
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
func (w *Weathering) blow(t plugin.Tick, wind float32) {
	if len(w.swaying) == 0 || wind > swayBelow && wind < swayAbove {
		return
	}
	brd := w.board.Res.Logic.Board
	brd.EachCell(func(c board.CellID) {
		if !w.swaying[brd.Kind(c).Name] {
			return
		}
		id, ok := w.board.CellEntity(c)
		if !ok {
			return
		}
		switch swaying := w.effects.Has(id, w.sway); {
		case wind > swayAbove && !swaying:
			w.effects.Cast(t.CmdBuf, id, w.sway)
		case wind < swayBelow && swaying:
			w.effects.Dispel(id, w.sway)
		}
	})
}
