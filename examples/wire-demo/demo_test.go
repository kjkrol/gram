package main

import (
	"github.com/kjkrol/gram/internal/engine"
	"testing"
	"time"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/aabbworld/plane"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/control"
	"github.com/kjkrol/gram/entity/tag"
	"github.com/kjkrol/gram/game"
	"github.com/kjkrol/gram/plugin"
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/board/grid"
	"github.com/kjkrol/gram/plugins/navigation"
	"github.com/kjkrol/gram/plugins/selection"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/rule"
	"github.com/kjkrol/uid"
)

// stageInit is a game.Initializer that drives the real Stage without a window;
// Scene.Layers() is left out.
type stageInit struct {
	ecs     *goke.ECS
	world   *world.Plugin
	tracked []any
	pending []func() []goke.System
	tps     game.TPS
}

var _ game.Initializer = (*stageInit)(nil)

func (c *stageInit) UseModule(m goke.Module) {
	regSys := goke.SystemFn{OnInit: func(*goke.SysInit) { m.RegSystems(c.ecs) }}
	c.tracked = append(c.tracked, m)
	c.pending = append(c.pending, func() []goke.System { return append(m.SetupSystems(), regSys) })
}

func (c *stageInit) Setup(providers ...goke.SetupProvider) {
	for _, p := range providers {
		c.tracked = append(c.tracked, p)
		c.pending = append(c.pending, p.SetupSystems)
	}
}

func (c *stageInit) RegSys(factory func() goke.System) goke.Runnable { return c.ecs.RegSys(factory()) }
func (c *stageInit) ECS() *goke.ECS                                  { return c.ecs }
func (c *stageInit) TPS() *game.TPS                                  { return &c.tps }

func (c *stageInit) Use(p plugin.Plugin) error {
	c.tracked = append(c.tracked, p)
	return p.Install(c)
}

func (c *stageInit) Hook(rules ...rule.Rule) error { return engine.HookOn(c.tracked, rules...) }

func (c *stageInit) Track(s plugin.Serializable) error {
	c.tracked = append(c.tracked, s)
	return nil
}

func (c *stageInit) UseWorld(cfg world.Config) *world.Plugin {
	cfg.Camera.ViewportWidth = ScreenWidth
	cfg.Camera.ViewportHeight = ScreenHeight
	c.world = world.NewPlugin(cfg)
	c.tracked = append(c.tracked, c.world)
	if err := c.world.Install(c); err != nil {
		panic(err)
	}
	return c.world
}

// stage is the demo built fresh, without a window, and a view of its units.
type stage struct {
	*mainStage
	ecs      *goke.ECS
	base     goke.Comp[world.Base]
	selected goke.OptComp[tag.Tags[selection.Family]]
	units    *goke.Query
}

func buildStage(t *testing.T) *stage {
	t.Helper()
	s := &stage{mainStage: &mainStage{}}
	ctx := &stageInit{ecs: goke.New()}
	if err := s.Init(ctx); err != nil {
		t.Fatalf("Init: %v", err)
	}
	if err := s.Spawn(); err != nil {
		t.Fatalf("Spawn: %v", err)
	}
	for _, v := range ctx.tracked {
		if p, ok := v.(plugin.Populator); ok {
			if err := p.Populate(); err != nil {
				t.Fatalf("Populate: %v", err)
			}
		}
	}
	ctx.ecs.SetPlan(func(rc goke.RunCtx, d time.Duration) { s.Update(rc, d); s.world.Clock().Replay(rc, d) })
	var systems []goke.System
	for _, produce := range ctx.pending {
		systems = append(systems, produce()...)
	}
	systems = append(systems, goke.SystemFn{OnInit: func(si *goke.SysInit) {
		s.units = si.NewQueryBuilder(&s.base).Optional(&s.selected).Build()
	}})
	ctx.ecs.Setup(systems...)
	s.ecs = ctx.ecs
	return s
}

func (s *stage) tick(n int) {
	for range n {
		s.ecs.Tick(time.Second / TPS)
	}
}

// press presses and lets go key at the keyboard, through the player's bindings.
func (s *stage) press(key control.Key) {
	ev := &control.InputEvents{}
	ev.AddKeyEvent(key, control.ActionPress)
	ev.AddKeyEvent(key, control.ActionRelease)
	s.players.EventHandler().HandleEvents(ev)
}

// each calls f for every unit, with the cell under its centre and whether it is selected.
func (s *stage) each(f func(id uid.UID64, x, y uint32, selected bool)) {
	sel := s.selection.Tags().Selected
	for s.units.All(); s.units.Next(); {
		cur := s.units.Cursor()
		tags := s.selected.Slice(cur)
		for k, id := range cur.IDs {
			c, ok := s.brd.CellAt(s.base.Slice(cur)[k].Pos.Center())
			if !ok {
				continue
			}
			x, y, _ := s.brd.Coords(c)
			f(id, x, y, tags != nil && tags[k].Has(sel))
		}
	}
}

// onStrip is every unit whose centre stands on a trapdoor of the strip from column left.
func (s *stage) onStrip(left uint32) map[uid.UID64]bool {
	out := map[uid.UID64]bool{}
	s.each(func(id uid.UID64, x, y uint32, _ bool) {
		if x >= left && x <= left+1 && y >= stripTop && y <= stripBottom {
			out[id] = true
		}
	})
	return out
}

// onRow is every unit whose centre stands on row y.
func (s *stage) onRow(row uint32) []uid.UID64 {
	var out []uid.UID64
	s.each(func(id uid.UID64, _, y uint32, _ bool) {
		if y == row {
			out = append(out, id)
		}
	})
	return out
}

// admits reports whether the cell at x, y holds a walker.
func (s *stage) admits(x, y uint32) bool {
	c, _ := s.brd.CellIndex(x, y)
	return s.brd.Kind(c).Admits(cell.Land)
}

// holds reports whether the top trapdoor of the strip from column left holds a walker.
func (s *stage) holds(left uint32) bool { return s.admits(left, stripTop) }

// gateOpen reports whether both cells of the gate let a walker through.
func (s *stage) gateOpen() bool {
	return s.admits(gateLeft, fenceRow) && s.admits(gateLeft+1, fenceRow)
}

func (s *stage) alive() map[uid.UID64]bool {
	out := map[uid.UID64]bool{}
	for s.units.All(); s.units.Next(); {
		for _, id := range s.units.Cursor().IDs {
			out[id] = true
		}
	}
	return out
}

// put moves the unit id onto cell c.
func (s *stage) put(id uid.UID64, c cell.ID) {
	to := cellBox(s.brd, c, EntitySize).TopLeft
	for s.units.All(); s.units.Next(); {
		cur := s.units.Cursor()
		for k, at := range cur.IDs {
			if at == id {
				s.world.Space().MoveTo(&s.base.Slice(cur)[k].Pos.AABB, to)
			}
		}
	}
}

// Key 1 puts the west wire on for a while: the west trapdoors open under whoever stands on them,
// the east ones and the gate stay as they are, and once the pulse is over the west ones hold again.
func TestKey1_OpensTheWestTrapdoorsAlone(t *testing.T) {
	s := buildStage(t)
	var caught map[uid.UID64]bool
	for range 20 * TPS {
		s.tick(1)
		if caught = s.onStrip(westLeft); len(caught) > 0 {
			break
		}
	}
	if len(caught) == 0 {
		t.Fatal("nobody walked onto the west strip in twenty seconds")
	}
	s.press(control.Key1)
	s.tick(TPS / 2)
	alive := s.alive()
	for id := range caught {
		if alive[id] {
			t.Errorf("unit %d stood on a west trapdoor as the west wire went on and is still here", id)
		}
	}
	if s.holds(westLeft) || !s.holds(eastLeft) || s.gateOpen() {
		t.Errorf("key 1: west holds %v, east holds %v, gate open %v; want the west open alone",
			s.holds(westLeft), s.holds(eastLeft), s.gateOpen())
	}
	s.tick(int((pulse + time.Second).Seconds() * TPS))
	if !s.holds(westLeft) {
		t.Error("a west trapdoor still open after the west wire's pulse")
	}
}

// A scout on the plate puts the east wire on: the east trapdoors open, the west ones stay shut;
// stepped off, the plate keeps its wire on for the pulse and the trapdoors then shut.
func TestPlate_OpensTheEastTrapdoorsAlone(t *testing.T) {
	s := buildStage(t)
	s.tick(1)
	if !s.holds(westLeft) || !s.holds(eastLeft) {
		t.Fatal("a trapdoor open before anyone stood on the plate")
	}
	scouts := s.onRow(yardRow)
	if len(scouts) != 3 {
		t.Fatalf("%d units on the yard's first row, want the three scouts", len(scouts))
	}
	plate, _ := s.brd.CellIndex(plateCol, yardRow)
	s.put(scouts[0], plate)
	s.tick(TPS / 2)
	if s.holds(eastLeft) || !s.holds(westLeft) {
		t.Errorf("plate stood on: east holds %v, west holds %v; want the east open alone", s.holds(eastLeft), s.holds(westLeft))
	}
	away, _ := s.brd.CellIndex(GridWidth/2, yardRow)
	s.put(scouts[0], away)
	s.tick(int(pulse.Seconds() * TPS / 2))
	if s.holds(eastLeft) {
		t.Error("the east trapdoors shut at once, want them open for the pulse after the scout stepped off")
	}
	s.tick(2 * TPS)
	if !s.holds(eastLeft) {
		t.Error("the east trapdoors still open well after the scout stepped off the plate")
	}
}

// G flips the gate's switch: the gate opens and stays open, long past a pulse, until G again shuts
// it, and it stays shut; the trapdoors are none of its business.
func TestGate_StaysAsTheSwitchLeftIt(t *testing.T) {
	s := buildStage(t)
	s.tick(1)
	if s.gateOpen() {
		t.Fatal("the gate open before its switch was flipped")
	}
	s.press(control.KeyG)
	s.tick(TPS / 2)
	if !s.gateOpen() {
		t.Fatal("G flipped the switch and the gate is still shut")
	}
	s.tick(int(5 * pulse.Seconds() * TPS))
	if !s.gateOpen() {
		t.Error("the gate shut by itself, want it open until G again")
	}
	if !s.holds(westLeft) || !s.holds(eastLeft) {
		t.Errorf("the gate's switch on: west holds %v, east holds %v; want both shut", s.holds(westLeft), s.holds(eastLeft))
	}
	s.press(control.KeyG)
	s.tick(TPS / 2)
	if s.gateOpen() {
		t.Fatal("G flipped the switch back and the gate is still open")
	}
	s.tick(int(5 * pulse.Seconds() * TPS))
	if s.gateOpen() {
		t.Error("the gate opened by itself, want it shut until G again")
	}
}

// A scout sent into the meadow stays in the yard while the gate is shut and walks out through it
// once G opens it.
func TestGate_LetsTheScoutsOutOnlyWhileOpen(t *testing.T) {
	s := buildStage(t)
	s.tick(1)
	scout := s.onRow(yardRow)[0]
	s.world.Commands().Put(s.player.ID, selection.Select{IDs: []uid.UID64{scout}})
	s.tick(1)
	meadow, _ := s.brd.CellIndex(GridWidth/2, 9) // between the strips, off the wanderers' rows
	send := func() {
		s.world.Commands().Put(s.player.ID, navigation.MoveTo{Cell: meadow, At: s.brd.CellCenter(meadow)})
	}
	row := func() (y uint32, alive bool) {
		s.each(func(id uid.UID64, _, at uint32, _ bool) {
			if id == scout {
				y, alive = at, true
			}
		})
		return y, alive
	}
	send()
	s.tick(10 * TPS)
	if y, alive := row(); !alive || y < yardRow {
		t.Fatalf("sent off with the gate shut, the scout is on row %d (alive %v); want it in the yard", y, alive)
	}
	s.press(control.KeyG)
	s.tick(TPS / 2)
	send()
	s.tick(10 * TPS)
	if y, alive := row(); !alive || y >= fenceRow {
		t.Errorf("sent off through the open gate, the scout is on row %d (alive %v); want it in the meadow", y, alive)
	}
}

// J hastens the selected units playing hasty: selecting over the whole meadow selects the
// player's three scouts and two porters, never the wanderers nobody owns, and the scouts alone are
// hastened.
func TestHaste_OnlyTheSelectedScoutsPlayingHasty(t *testing.T) {
	s := buildStage(t)
	s.tick(1)
	everywhere := geom.NewAABBAt(geom.NewVec(0, 0), ScreenWidth, ScreenHeight)
	s.world.Commands().Put(s.player.ID, selection.Select{Box: everywhere})
	s.tick(1)
	s.press(control.KeyJ)
	s.tick(2)
	var scouts, porters, selectedPorters, others int
	s.each(func(id uid.UID64, _, y uint32, selected bool) {
		hastened := s.haste.On(id)
		switch {
		case y == yardRow && hastened && selected:
			scouts++
		case y == yardRow+1:
			if hastened {
				porters++
			}
			if selected {
				selectedPorters++
			}
		case y < fenceRow && (hastened || selected):
			others++
		}
	})
	if scouts != 3 || porters != 0 || selectedPorters != 2 || others != 0 {
		t.Errorf("J: %d scouts selected and hastened, %d porters hastened, %d porters selected, %d wanderers touched; want 3, 0, 2, 0",
			scouts, porters, selectedPorters, others)
	}
}

// cellBox is the size x size box centred on cell c.
func cellBox(g grid.Grid, c cell.ID, size uint32) plane.AABB {
	at := g.CellCenter(c)
	half := float64(size) / 2
	return plane.NewAABB(geom.NewVec(at.X-half, at.Y-half), float64(size), float64(size))
}
