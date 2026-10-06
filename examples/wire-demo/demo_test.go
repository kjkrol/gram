package main

import (
	"maps"
	"testing"
	"time"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/aabbworld/plane"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/control"
	"github.com/kjkrol/gram/entity/tag"
	"github.com/kjkrol/gram/game"
	"github.com/kjkrol/gram/internal/hosts"
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
	hosts   []plugin.Host // of the rules of the moments the plugins installed catch
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

// testStage is the demo built fresh, without a window, and a view of its units.
type testStage struct {
	*arena
	stage    game.Stage
	ecs      *goke.ECS
	base     goke.Comp[world.Base]
	selected goke.OptComp[tag.Tags[selection.Family]]
	units    *goke.Query
}

func buildStage(t *testing.T) *testStage {
	t.Helper()
	s := &testStage{}
	s.arena, s.stage = newArena()
	ctx := &stageInit{ecs: goke.New()}
	if err := s.stage.Init(ctx); err != nil {
		t.Fatalf("Init: %v", err)
	}
	if err := ctx.Deliver(ctx.world.Kinds().Played()...); err != nil { // as the engine does once Init returns
		t.Fatalf("roles: %v", err)
	}
	if err := s.stage.Spawn(); err != nil {
		t.Fatalf("Spawn: %v", err)
	}
	for _, v := range ctx.tracked {
		if p, ok := v.(plugin.Populator); ok {
			if err := p.Populate(); err != nil {
				t.Fatalf("Populate: %v", err)
			}
		}
	}
	ctx.ecs.SetPlan(func(rc goke.RunCtx, d time.Duration) { s.stage.Update(rc, d); s.world.Clock().Replay(rc, d) })
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

func (s *testStage) tick(n int) {
	for range n {
		s.ecs.Tick(time.Second / TPS)
	}
}

// press presses and lets go key at the keyboard, through the player's bindings.
func (s *testStage) press(key control.Key) {
	ev := &control.InputEvents{}
	ev.AddKeyEvent(key, control.ActionPress)
	ev.AddKeyEvent(key, control.ActionRelease)
	s.players.EventHandler().HandleEvents(ev)
}

// each calls f for every unit, with the cell under its centre and whether it is selected.
func (s *testStage) each(f func(id uid.UID64, x, y uint32, selected bool)) {
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
func (s *testStage) onStrip(left uint32) map[uid.UID64]bool {
	out := map[uid.UID64]bool{}
	s.each(func(id uid.UID64, x, y uint32, _ bool) {
		if x >= left && x <= left+1 && y >= stripTop && y <= stripBottom {
			out[id] = true
		}
	})
	return out
}

// onRow is every unit whose centre stands on row y.
func (s *testStage) onRow(row uint32) []uid.UID64 {
	var out []uid.UID64
	s.each(func(id uid.UID64, _, y uint32, _ bool) {
		if y == row {
			out = append(out, id)
		}
	})
	return out
}

// opened is every cell open now — a trapdoor fallen open, the gate open — counted by the group it
// belongs to: "west", "east", "gate", "elsewhere" for none.
func (s *testStage) opened() map[string]int {
	out := map[string]int{}
	pit, gateway := cell.Named(PitCell), cell.Named(GatewayCell)
	for c := range cell.ID(s.brd.CellCount()) {
		if k := s.brd.Kind(c).Name; k != pit && k != gateway {
			continue
		}
		x, y, _ := s.brd.Coords(c)
		group := "elsewhere"
		switch {
		case y >= stripTop && y <= stripBottom && x >= westLeft && x <= westLeft+1:
			group = "west"
		case y >= stripTop && y <= stripBottom && x >= eastLeft && x <= eastLeft+1:
			group = "east"
		case y == fenceRow && x >= gateLeft && x <= gateLeft+1:
			group = "gate"
		}
		out[group]++
	}
	return out
}

// wantOpen checks that groups, every cell of them, are open now and nothing else is.
func (s *testStage) wantOpen(t *testing.T, when string, groups ...string) {
	t.Helper()
	size := map[string]int{"west": 2 * int(stripBottom-stripTop+1), "east": 2 * int(stripBottom-stripTop+1), "gate": 2}
	want := map[string]int{}
	for _, g := range groups {
		want[g] = size[g]
	}
	if got := s.opened(); !maps.Equal(got, want) {
		t.Errorf("%s: open cells by group %v, want %v", when, got, want)
	}
}

func (s *testStage) alive() map[uid.UID64]bool {
	out := map[uid.UID64]bool{}
	for s.units.All(); s.units.Next(); {
		for _, id := range s.units.Cursor().IDs {
			out[id] = true
		}
	}
	return out
}

// put moves the unit id onto cell c.
func (s *testStage) put(id uid.UID64, c cell.ID) {
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
	s.wantOpen(t, "key 1", "west")
	s.tick(int((pulse + time.Second).Seconds() * TPS))
	s.wantOpen(t, "after the west wire's pulse")
}

// A scout on the plate puts the east wire on: the east trapdoors open, the west ones stay shut;
// stepped off, the plate keeps its wire on for the pulse and the trapdoors then shut.
func TestPlate_OpensTheEastTrapdoorsAlone(t *testing.T) {
	s := buildStage(t)
	s.tick(1)
	s.wantOpen(t, "before anyone stood on the plate")
	scouts := s.onRow(yardRow)
	if len(scouts) != 3 {
		t.Fatalf("%d units on the yard's first row, want the three scouts", len(scouts))
	}
	plate, _ := s.brd.CellIndex(plateCol, yardRow)
	s.put(scouts[0], plate)
	s.tick(TPS / 2)
	s.wantOpen(t, "plate stood on", "east")
	away, _ := s.brd.CellIndex(GridWidth/2, yardRow)
	s.put(scouts[0], away)
	s.tick(int(pulse.Seconds() * TPS / 2))
	s.wantOpen(t, "stepped off the plate, within the pulse", "east")
	s.tick(2 * TPS)
	s.wantOpen(t, "well after the scout stepped off the plate")
}

// G flips the gate's switch: the gate opens and stays open, long past a pulse, until G again shuts
// it, and it stays shut; the trapdoors are none of its business.
func TestGate_StaysAsTheSwitchLeftIt(t *testing.T) {
	s := buildStage(t)
	s.tick(1)
	s.wantOpen(t, "before the gate's switch was flipped")
	s.press(control.KeyG)
	s.tick(TPS / 2)
	s.wantOpen(t, "G flipped the switch", "gate")
	s.tick(int(5 * pulse.Seconds() * TPS))
	s.wantOpen(t, "long after G flipped the switch", "gate")
	s.press(control.KeyG)
	s.tick(TPS / 2)
	s.wantOpen(t, "G flipped the switch back")
	s.tick(int(5 * pulse.Seconds() * TPS))
	s.wantOpen(t, "long after G flipped the switch back")
}

// A scout sent into the meadow stays in the yard while the gate is shut and walks out through it
// once G opens it.
func TestGate_LetsTheScoutsOutOnlyWhileOpen(t *testing.T) {
	s := buildStage(t)
	s.tick(1)
	scout := s.onRow(yardRow)[0]
	s.world.Carrier().Put(s.player.ID, selection.Select{IDs: []uid.UID64{scout}})
	s.tick(1)
	meadow, _ := s.brd.CellIndex(GridWidth/2, 9) // between the strips, off the wanderers' rows
	send := func() {
		s.world.Carrier().Put(s.player.ID, navigation.MoveTo{Cell: meadow, At: s.brd.CellCenter(meadow)})
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
	s.world.Carrier().Put(s.player.ID, selection.Select{Box: everywhere})
	s.tick(1)
	s.press(control.KeyJ)
	s.tick(2)
	var scouts, porters, selectedPorters, others int
	s.each(func(id uid.UID64, _, y uint32, selected bool) {
		hastened := s.world.Effects().Named(HasteEf).On(id)
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

// U has a selected scout beside the lever pull it: the west wire goes on and the west trapdoors
// open, the east ones stay shut; a scout far from the lever pulls nothing, and the trapdoors on the
// wire it stands beside are not levers.
func TestU_PullsTheLeverBesideTheSelectedScout(t *testing.T) {
	s := buildStage(t)
	s.tick(1)
	scouts := s.onRow(yardRow)
	if len(scouts) != 3 {
		t.Fatalf("%d units on the yard's first row, want the three scouts", len(scouts))
	}
	far, _ := s.brd.CellIndex(GridWidth/2, yardRow)
	s.put(scouts[0], far)
	s.tick(1)
	s.world.Carrier().Put(s.player.ID, selection.Select{IDs: []uid.UID64{scouts[0]}})
	s.tick(1)
	s.press(control.KeyU)
	s.tick(TPS / 4)
	s.wantOpen(t, "U far from the lever")
	beside, _ := s.brd.CellIndex(leverCol+1, yardRow)
	s.put(scouts[0], beside)
	s.tick(1)
	s.press(control.KeyU)
	s.tick(TPS / 4)
	s.wantOpen(t, "U beside the lever", "west")
}

// Hosts keeps the hosts of the rules of the moments a plugin catches.
func (c *stageInit) Hosts(h ...plugin.Host) { c.hosts = append(c.hosts, h...) }

// Deliver hands rules — a role's, each of its own — to the hosts of their moments, as the engine
// does with the roles played once a Stage's Init returns.
func (c *stageInit) Deliver(rules ...rule.Rule) error { return hosts.Deliver(c.hosts, rules...) }
