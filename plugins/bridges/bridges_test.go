package bridges_test

import (
	"math"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/ceremony/command"
	"github.com/kjkrol/ceremony/host"
	"github.com/kjkrol/goke/v3"
	icamera "github.com/kjkrol/gram/internal/camera"
	"github.com/kjkrol/gram/plugin"
	"github.com/kjkrol/gram/plugins/board"
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/board/grid"
	"github.com/kjkrol/gram/plugins/bridges"
	"github.com/kjkrol/gram/plugins/bridges/site"
	"github.com/kjkrol/gram/plugins/collision"
	"github.com/kjkrol/gram/plugins/topography"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/render"
)

type installer struct {
	ecs     *goke.ECS
	pending []func() []goke.System
	drawn   []render.Source
}

func (c *installer) UseModule(m goke.Module) {
	reg := goke.SystemFn{OnInit: func(*goke.SysInit) { m.RegSystems(c.ecs) }}
	c.pending = append(c.pending, func() []goke.System { return append(m.SetupSystems(), reg) })
}
func (c *installer) Setup(providers ...goke.SetupProvider) {
	for _, p := range providers {
		c.pending = append(c.pending, p.SetupSystems)
	}
}
func (c *installer) RegSys(f func() goke.System) goke.Runnable { return c.ecs.RegSys(f()) }
func (c *installer) ECS() *goke.ECS                            { return c.ecs }
func (c *installer) Hosts(...host.Host)                        {}
func (c *installer) Draws(s ...render.Source)                  { c.drawn = append(c.drawn, s...) }
func (c *installer) Looks(func())                              {}

// dale is a board 10 x 6 of cells 10 wide, the ground 20 up but for a valley at 0 from x 30 to
// 60.
type dale struct {
	t       *testing.T
	g       grid.Grid
	w       *world.Plugin
	board   *board.Plugin
	topo    *topography.Plugin
	bridges *bridges.Plugin
	col     *collision.Plugin
	ecs     *goke.ECS
	drawn   []render.Source
	floors  *goke.Query
	floor   goke.Comp[cell.Floor]
	on      goke.Comp[cell.On]
	piers   *goke.Query
	pierZ   goke.Comp[world.Z]
	pierB   goke.Comp[world.Base]
	blocks  *goke.Query
	blockZ  goke.Comp[world.Z]
}

var rules = site.Rules{Longest: 3, Under: 5, Clearance: 1, Reshape: 10}

func newDale(t *testing.T) *dale {
	t.Helper()
	d := &dale{t: t, g: grid.DefaultGrids{}.Square(10, 6, 10)}
	d.w = world.NewPlugin(world.Config{
		Space:    world.SpaceCfg{Width: 100, Height: 60},
		Entities: world.EntitiesCfg{MaxCount: 64, MinSize: 1, MaxSize: 10},
		Heights:  true,
	})
	d.col = collision.NewPlugin(d.w)
	d.board = board.NewPlugin(d.g, &cell.MultipleOccupancy{}, d.w).WithCollision(d.col)
	d.topo = topography.NewPlugin(d.w, d.board, topography.Config{Cell: 10})
	d.bridges = bridges.NewPlugin(d.w, d.board, bridges.Config{Rules: rules}).WithShaper(d.topo)
	d.board.Board().SetAll(cell.Kind{Cost: 1, Allows: cell.Land})
	d.bridges.Types().Define("bridge", bridges.Spec{
		Allows:    cell.Land,
		Width:     6,
		Thickness: 1,
		Cost:      1,
		Rail:      cell.Rail{Keeps: cell.Land, Height: 1},
		Pier:      2,
	})
	if err := d.w.Carry(d.bridges); err != nil {
		t.Fatal(err)
	}
	return d
}

func valley(p geom.Vec) float64 {
	if p.X > 30 && p.X < 60 {
		return 0
	}
	return 20
}

func (d *dale) start(heights func(geom.Vec) float64) {
	d.t.Helper()
	d.topo.Seed(heights)
	ctx := &installer{ecs: goke.New()}
	for _, p := range []plugin.Plugin{d.w, d.col, d.board, d.topo, d.bridges} {
		if err := p.Install(ctx); err != nil {
			d.t.Fatal(err)
		}
	}
	if err := d.w.Populate(); err != nil {
		d.t.Fatal(err)
	}
	if err := d.board.Populate(); err != nil {
		d.t.Fatal(err)
	}
	if err := d.topo.Populate(); err != nil {
		d.t.Fatal(err)
	}
	if err := d.bridges.Populate(); err != nil {
		d.t.Fatal(err)
	}
	d.drawn = ctx.drawn
	var systems []goke.System
	for _, produce := range ctx.pending {
		systems = append(systems, produce()...)
	}
	systems = append(systems, goke.SystemFn{OnInit: func(si *goke.SysInit) {
		d.floors = si.NewQueryBuilder(&d.floor, &d.on).Build()
		d.piers = si.NewQueryBuilder(&d.pierB, &d.pierZ).
			Include(goke.Include[collision.Collider]()).Build()
		d.blocks = si.NewQueryBuilder(&d.blockZ).Include(goke.Include[world.Block]()).Build()
	}})
	ctx.ecs.Setup(systems...)
	ctx.ecs.SetPlan(func(rc goke.RunCtx, dt time.Duration) {
		d.bridges.RunPlan(rc, dt)
		d.w.RunPlan(rc, dt)
		d.col.RunPlan(rc, dt)
		d.board.RunPlan(rc, dt)
		d.topo.RunPlan(rc, dt)
		rc.Sync()
		d.w.Clock().Replay(rc, dt)
	})
	d.ecs = ctx.ecs
}

func (d *dale) give(cmd command.Command) {
	d.t.Helper()
	if !d.w.Carrier().Put(1, cmd) {
		d.t.Fatalf("nobody takes %T", cmd)
	}
	d.ecs.Tick(time.Second / 60)
}

func (d *dale) ticks(n int) {
	for range n {
		d.ecs.Tick(time.Second / 60)
	}
}

func (d *dale) decks() map[cell.ID]cell.Floor {
	out := map[cell.ID]cell.Floor{}
	for d.floors.All(); d.floors.Next(); {
		cur := d.floors.Cursor()
		for i, f := range d.floor.Slice(cur) {
			out[d.on.Slice(cur)[i].Cell] = f
		}
	}
	return out
}

func (d *dale) pierPoints() []geom.Vec {
	var out []geom.Vec
	for d.piers.All(); d.piers.Next(); {
		for _, b := range d.pierB.Slice(d.piers.Cursor()) {
			out = append(out, b.Pos.Center())
		}
	}
	return out
}

func TestTool_APlayerLaysABridgeOutAndBuildsIt(t *testing.T) {
	d := newDale(t)
	d.start(func(p geom.Vec) float64 {
		if p.X >= 70 {
			return 23 // the far head's east side, 3 over the deck
		}
		return valley(p)
	})
	at := func(x, y uint32) geom.Vec { return d.g.CellCenter(d.g.At(x, y)) }
	d.give(bridges.Mark{At: at(2, 3)})
	d.give(bridges.Aim{At: at(7, 3)}) // four spans: too long
	d.give(bridges.Mark{At: at(7, 3)})
	d.give(bridges.Build{})
	d.ticks(5)
	if got := d.decks(); len(got) != 0 {
		t.Fatalf("a bridge too long was built: %v", got)
	}
	d.give(bridges.Aim{At: at(6, 3)})
	d.give(bridges.Mark{At: at(6, 3)})
	d.give(bridges.Build{})
	d.ticks(5)
	decks := d.decks()
	if len(decks) != 5 {
		t.Fatalf("built %d pieces of deck, want 2 heads and 3 spans: %v", len(decks), decks)
	}
	east, _ := grid.Direction(d.g, 1, 0)
	if f := decks[d.g.At(2, 3)]; f.Links != 1<<east || f.Height != 0 {
		t.Errorf("the first head carries %+v, want a band east alone at its ground", f)
	}
	if f := decks[d.g.At(4, 3)]; f.Height != 20 || !f.Graded {
		t.Errorf("the middle span carries %+v, want a graded deck 20 over the valley", f)
	}
	piers := d.pierPoints()
	if len(piers) != 2 || piers[0].Y != 35 || math.Abs(piers[0].X+piers[1].X-90) > 1e-9 {
		t.Errorf("piers stand at %v, want at (40, 35) and (50, 35)", piers)
	}
	if h := d.topo.Relief().At(geom.NewVec(70, 30)); h != 20 {
		t.Errorf("the far head's corner stands at %v, want levelled at the deck, 20", h)
	}
	n := 0
	for d.blocks.All(); d.blocks.Next(); {
		n += len(d.blockZ.Slice(d.blocks.Cursor()))
	}
	if n != len(decks)+len(piers) {
		t.Errorf("%d blocks cast their shadows with the ground's, want the %d pieces of deck "+
			"and the %d piers", n, len(decks), len(piers))
	}
}

func TestTool_TheBridgeInHandLiesOverItsCells(t *testing.T) {
	d := newDale(t)
	d.start(valley)
	at := func(x, y uint32) geom.Vec { return d.g.CellCenter(d.g.At(x, y)) }
	d.give(bridges.Mark{At: at(2, 3)})
	d.give(bridges.Aim{At: at(5, 3)})
	hand := d.drawn[len(d.drawn)-1] // the bridges' own, installed last
	cam := icamera.NewFromSpace(100, 60, 0)
	var f render.Frame
	f.Reset(cam)
	hand.Compose(&f, cam)
	n := 0
	f.Each(func(at render.At, _ []render.Vertex) {
		n++
		if at.Top() || math.Abs(float64(at.Z())-20.9) > 1e-4 {
			t.Errorf("the bridge in hand lies at %v, want over its head's ground at 20", at)
		}
	})
	if n == 0 {
		t.Fatal("the bridge in hand is not drawn")
	}
}

func TestTool_CancelBuildsNothing(t *testing.T) {
	d := newDale(t)
	d.start(valley)
	at := func(x, y uint32) geom.Vec { return d.g.CellCenter(d.g.At(x, y)) }
	d.give(bridges.Mark{At: at(2, 3)})
	d.give(bridges.Aim{At: at(6, 3)})
	d.give(bridges.Mark{At: at(6, 3)})
	d.give(bridges.Cancel{})
	d.give(bridges.Build{})
	d.ticks(5)
	if got := d.decks(); len(got) != 0 {
		t.Fatalf("a bridge cancelled was built: %v", got)
	}
	d.give(bridges.Mark{At: at(6, 3)})
	d.give(bridges.Build{})
	d.ticks(5)
	if got := d.decks(); len(got) != 5 {
		t.Errorf("marked again after Cancel, built %d pieces, want 5", len(got))
	}
}

func TestSpawn_ALevelsBridges(t *testing.T) {
	file := func(entity string) fstest.MapFS {
		var corners strings.Builder
		for y := 0; y <= 6; y++ {
			for x := 0; x <= 10; x++ {
				if x > 0 {
					corners.WriteString(" ")
				}
				if x > 3 && x < 6 {
					corners.WriteString("0")
				} else {
					corners.WriteString("20")
				}
			}
			corners.WriteString("\n")
		}
		return fstest.MapFS{"map.yaml": {Data: []byte("board:\n  default: earth\n" +
			"  entities:\n    - " + entity + "\nrelief:\n  corners: |\n" +
			indent(corners.String()))}}
	}
	for name, tc := range map[string]struct {
		entity string
		why    string
	}{
		"standing":    {"{kind: bridge, from: [2, 3], to: [6, 3]}", ""},
		"too long":    {"{kind: bridge, from: [1, 3], to: [6, 3]}", "map.yaml:4: the bridge at"},
		"too shallow": {"{kind: bridge, from: [1, 3], to: [3, 3]}", "map.yaml:4: the bridge at"},
	} {
		t.Run(name, func(t *testing.T) {
			d := newDale(t)
			d.board.CellKinds().Define("earth", cell.Kind{Cost: 1, Allows: cell.Land})
			level, err := d.board.Lay(file(tc.entity), "map.yaml")
			if err != nil {
				t.Fatal(err)
			}
			err = d.bridges.Spawn(level)
			if err == nil {
				d.topo.Seed(level.Heights())
				if err = d.board.Populate(); err == nil {
					err = d.bridges.Populate()
				}
			}
			if tc.why != "" {
				if err == nil || !strings.Contains(err.Error(), tc.why) {
					t.Fatalf("got %v, want an error saying %q", err, tc.why)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			d.start(level.Heights())
			d.ticks(2)
			if got := d.decks(); len(got) != 5 {
				t.Errorf("built %d pieces of deck, want 5", len(got))
			}
		})
	}
}

func indent(s string) string {
	var b strings.Builder
	for _, l := range strings.SplitAfter(s, "\n") {
		if l != "" {
			b.WriteString("    " + l)
		}
	}
	return b.String()
}
