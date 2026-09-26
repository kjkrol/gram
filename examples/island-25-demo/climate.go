package main

import (
	"image/color"
	"math"
	"math/rand/v2"
	"time"

	"github.com/kjkrol/gram/plugin"
	"github.com/kjkrol/gram/plugins/board"
	"github.com/kjkrol/gram/plugins/climate"
	"github.com/kjkrol/gram/plugins/effects"
	"github.com/kjkrol/gram/plugins/sky"
	"github.com/kjkrol/gram/render"
)

// ground is the island's weather on its ground, the game's own: three effects on the board's
// cells, cast as the weather says and taken off again. Snow turns a cell's kind into its snowy
// one, ice turns water into ice, sway has the forest bend in the wind.
type ground struct {
	snow, ice, sway effects.ID
	snowy           map[board.Name]board.CellKind // each kind snow may lie on, under snow
	frozen          board.CellKind                // water under ice
	pace            time.Duration                 // how much of the sky's time since the climate last worked
	laid            bool                          // the first tick has laid what a winter begun lies under
}

// How the climate goes, a second at a time: the share of the island's cells snow settles on at a
// full fall, and melts off each degree above freezing; the share of the water that freezes each
// degree below iceBelow, and thaws each degree above freezing; the winds above which the forest
// sways and below which it stops.
const (
	snowSettles = 0.02
	snowMelts   = 0.004
	iceBelow    = -3
	iceSets     = 0.002
	iceThaws    = 0.003
	swayAbove   = 15
	swayBelow   = 10
	// snow first lies on ground this high, round the loosest few spots of the drifts' pattern
	// (driftSeeds of it), and next to snow already lying; a winter begun has it on driftWinter
	highSnow    = 60
	driftSeeds  = 0.12
	driftWinter = 0.7
	driftSize   = 5 // cells across one of the drifts' patches
)

// snowyColors is how each kind snow may lie on looks under it; iceColor, water frozen.
var (
	snowyColors = map[string]color.RGBA{
		"earth":  {R: 232, G: 236, B: 235, A: 255},
		"sand":   {R: 238, G: 236, B: 225, A: 255},
		"rock":   {R: 205, G: 208, B: 212, A: 255},
		"forest": {R: 150, G: 185, B: 165, A: 255},
	}
	iceColor = color.RGBA{R: 175, G: 210, B: 230, A: 255}
)

// defineClimate adds the snowy kinds and ice to the board's dictionary and defines the effects;
// call it after the island's own kinds, before the board and the effects are in use.
func (s *mainStage) defineClimate() {
	kinds := s.board.CellKindDict()
	c := &s.ground
	c.snowy = map[board.Name]board.CellKind{}
	for name := range snowyColors {
		k, _ := kinds.Get(name)
		k.Name = board.Named("snowy " + name)
		kinds.Create(k) // the same ground to cross and stand on, another look
		c.snowy[board.Named(name)], _ = kinds.Get("snowy " + name)
	}
	kinds.Create(board.CellKind{Name: board.Named("ice"), Cost: 2, Allows: board.Land | board.Air, Shine: 0.3}.Costing(board.Air, 1))
	c.frozen, _ = kinds.Get("ice")

	c.snow = s.effects.Define("snow", effects.Spec{effects.Alter(func(g *board.Ground) {
		if under, ok := c.snowy[g.Kind.Name]; ok {
			under.Sway = g.Kind.Sway // a forest swaying goes on swaying under snow
			g.Kind = under
		}
	})})
	c.ice = s.effects.Define("ice", effects.Spec{effects.Alter(func(g *board.Ground) {
		if g.Kind.Name == board.Named("water") {
			g.Kind = c.frozen
		}
	})})
	c.sway = s.effects.Define("sway", effects.Spec{effects.Alter(func(g *board.Ground) { g.Kind.Sway = 0.6 })})
}

// climateSprites registers the snowy kinds' and ice's sprites in atlas.
func (s *mainStage) climateSprites(atlas *render.Atlas) {
	kinds := s.board.CellKindDict()
	for name, col := range snowyColors {
		k, _ := kinds.Get("snowy " + name)
		atlas.RegisterAt(k.SpriteID, CellSize, render.Solid(col))
	}
	atlas.RegisterAt(s.ground.frozen.SpriteID, CellSize, render.Solid(iceColor))
}

// weathering casts the island's effects and takes them off as the weather says, once a second of
// the sky's time — a day hurried on hurries them, a day stopped holds them:
// snow settles on cells here and there while it snows in the frost and melts off them once it is
// warm; water freezes from the shore out in a hard frost and thaws; the forest sways while the
// wind blows. A winter begun has its snow and its shores' ice at once.
func (s *mainStage) weathering(t plugin.Tick, w climate.Weathering) {
	c := &s.ground
	if !c.laid {
		c.laid = true
		if w.Season == sky.Winter {
			s.winter(t)
		}
	}
	if c.pace += t.Dt; c.pace < time.Second {
		return
	}
	seconds := float32(c.pace.Seconds()) // more than one when the day is hurried on
	c.pace = 0
	air := w.Weather
	switch {
	case air.Temperature < 0 && air.Snow > 0.05:
		s.scatter(t, snowSettles*air.Snow*seconds, c.snow, s.snowLies)
	case air.Temperature > 0:
		s.clear(snowMelts*air.Temperature*seconds, c.snow, s.snowEdge)
	}
	switch {
	case air.Temperature < iceBelow:
		s.scatter(t, iceSets*(iceBelow-air.Temperature)*seconds, c.ice, s.freezes)
	case air.Temperature > 0:
		s.clear(iceThaws*air.Temperature*seconds, c.ice, func(board.CellID) bool { return true })
	}
	blow := float32(math.Hypot(float64(air.Wind[0]), float64(air.Wind[1])))
	s.sway(t, blow)
}

// scatter casts effect on about share of the island's cells picked at random, those may takes.
func (s *mainStage) scatter(t plugin.Tick, share float32, effect effects.ID, may func(c board.CellID) bool) {
	brd := s.board.Res.Logic.Board
	for range int(share*GridWidth*GridHeight + rand.Float32()) {
		c, _ := brd.CellIndex(rand.Uint32N(GridWidth), rand.Uint32N(GridHeight))
		if id, ok := s.board.CellEntity(c); ok && may(c) {
			s.effects.Cast(t.CmdBuf, id, effect)
		}
	}
}

// clear takes effect off about share of the island's cells picked at random, those may lets go.
func (s *mainStage) clear(share float32, effect effects.ID, may func(c board.CellID) bool) {
	brd := s.board.Res.Logic.Board
	for range int(share*GridWidth*GridHeight + rand.Float32()) {
		c, _ := brd.CellIndex(rand.Uint32N(GridWidth), rand.Uint32N(GridHeight))
		if id, ok := s.board.CellEntity(c); ok && s.effects.Has(id, effect) && may(c) {
			s.effects.Dispel(id, effect)
		}
	}
}

// winter lays what a winter begun lies under: snow in drifts over most of the island, ice along
// its shores.
func (s *mainStage) winter(t plugin.Tick) {
	brd := s.board.Res.Logic.Board
	brd.EachCell(func(c board.CellID) {
		id, ok := s.board.CellEntity(c)
		if !ok {
			return
		}
		if s.takesSnow(c) && s.drift(c) < driftWinter {
			s.effects.Cast(t.CmdBuf, id, s.ground.snow)
		}
		if s.freezes(c) {
			s.effects.Cast(t.CmdBuf, id, s.ground.ice)
		}
	})
}

// takesSnow reports whether snow may lie on c at all: a kind that has a snowy one.
func (s *mainStage) takesSnow(c board.CellID) bool {
	_, ok := s.ground.snowy[s.board.Res.Logic.Board.Kind(c).Name]
	return ok
}

// snowLies reports whether falling snow settles on c now: ground that takes it, high, or next to
// snow lying already, or one of the drifts' seeds — so snow lies in patches that grow.
func (s *mainStage) snowLies(c board.CellID) bool {
	if !s.takesSnow(c) {
		return false
	}
	brd := s.board.Res.Logic.Board
	return brd.Altitude(c) >= highSnow || s.drift(c) < driftSeeds || s.nextTo(c)
}

// snowEdge reports whether the snow on c melts now: the lonelier it lies — the fewer of its
// neighbours under snow — and the later it lay in the drifts' pattern, the likelier, so the
// patches shrink whole and what lay first lies longest.
func (s *mainStage) snowEdge(c board.CellID) bool {
	neighbours := s.board.Res.Logic.Board.Neighbors(c)
	under := 0
	for _, n := range neighbours {
		if id, ok := s.board.CellEntity(n); ok && s.effects.Has(id, s.ground.snow) {
			under++
		}
	}
	lonely := 1 - float32(under)/float32(max(len(neighbours), 1))
	return rand.Float32() < lonely+s.drift(c)*0.5
}

// nextTo reports whether a neighbour of c lies under snow.
func (s *mainStage) nextTo(c board.CellID) bool {
	for _, n := range s.board.Res.Logic.Board.Neighbors(c) {
		if id, ok := s.board.CellEntity(n); ok && s.effects.Has(id, s.ground.snow) {
			return true
		}
	}
	return false
}

// drift is the drifts' pattern at c, 0 to 1, smooth over driftSize cells: where it is low, snow
// lies first and longest.
func (s *mainStage) drift(c board.CellID) float32 {
	x, y, _ := s.board.Res.Logic.Board.Coords(c)
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
func (s *mainStage) freezes(c board.CellID) bool {
	brd := s.board.Res.Logic.Board
	if brd.Kind(c).Name != board.Named("water") {
		return false
	}
	for _, n := range brd.Neighbors(c) {
		if brd.Kind(n).Name != board.Named("water") {
			return true
		}
	}
	return false
}

// sway has the forest sway once the wind blows harder than swayAbove and stop once it falls below
// swayBelow.
func (s *mainStage) sway(t plugin.Tick, blow float32) {
	brd := s.board.Res.Logic.Board
	brd.EachCell(func(c board.CellID) {
		name := brd.Kind(c).Name
		if name != board.Named("forest") && name != board.Named("snowy forest") {
			return
		}
		id, ok := s.board.CellEntity(c)
		if !ok {
			return
		}
		switch swaying := s.effects.Has(id, s.ground.sway); {
		case blow > swayAbove && !swaying:
			s.effects.Cast(t.CmdBuf, id, s.ground.sway)
		case blow < swayBelow && swaying:
			s.effects.Dispel(id, s.ground.sway)
		}
	})
}
