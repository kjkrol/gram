package selection_test

import (
	"slices"
	"testing"
	"time"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/aabbworld/plane"
	"github.com/kjkrol/ceremony"
	"github.com/kjkrol/ceremony/act"
	"github.com/kjkrol/ceremony/command"
	"github.com/kjkrol/ceremony/comp"
	"github.com/kjkrol/ceremony/cond"
	"github.com/kjkrol/ceremony/tag"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/control"
	"github.com/kjkrol/gram/entity/kind"
	"github.com/kjkrol/gram/plugin"
	"github.com/kjkrol/gram/plugins/cameras/cameratest"
	"github.com/kjkrol/gram/plugins/players"
	"github.com/kjkrol/gram/plugins/selection"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/render"
	"github.com/kjkrol/uid"
)

const (
	caravanKind = "caravan"
	porterKind  = "porter"
	caravanRole = "caravan"
)

// a caravan at (500, 100), an aggregate unfolding at the first step in a world of no sectors into
// three porters in a row at (100, 100), (130, 100) and (200, 100), each told what the case says,
// the caravan too, which then stands at their middle, where none of them does
type caravan struct {
	t       *testing.T
	w       *world.Plugin
	sel     *selection.Plugin
	players *players.Plugin
	me      *players.Player
	cam     camera.Camera
	ecs     *goke.ECS
	// what the selection draws
	renderer *selection.Renderer

	q      *goke.Query
	base   goke.Comp[world.Base]
	marks  goke.OptComp[tag.Tags[selection.Family]]
	partOf goke.OptComp[world.PartOf]

	whole uid.UID64
	// by x
	parts []uid.UID64
}

// told gives what the caravan and each of its porters are told as they are made, me and rival
// the players'
type told func(me, rival command.Issuer) (whole, part []command.Command)

func newCaravan(t *testing.T, tell told) *caravan {
	t.Helper()
	c := &caravan{t: t, cam: cameratest.NewFromSpace(1000, 1000, 0)}
	c.w = world.NewPlugin(world.Config{
		Space:    world.SpaceCfg{Width: 1000, Height: 1000},
		Entities: world.EntitiesCfg{MaxCount: 8, MinSize: 10, MaxSize: 10},
	})
	c.sel = selection.NewPlugin(c.w)
	c.players = players.NewPlugin(c.w, c.sel)
	c.me = c.players.Local("me")
	rival := c.players.Add("rival")
	c.players.Through(c.me).Over(geom.AABB{}, render.NewFeed(c.cam, nil))
	var bindings []control.Binding
	for _, b := range c.sel.DefaultBindings() {
		// the keys following the chosen unit want the cameras, which the case has none of
		if _, key := b.Trigger.(control.KeyPress); !key {
			bindings = append(bindings, b)
		}
	}
	if err := c.me.Bind(bindings...); err != nil {
		t.Fatal(err)
	}
	whole, part := tell(c.me.ID, rival.ID)
	unpack := make([]act.Step, 0, 3)
	for _, x := range []float32{-400, -370, -300} {
		at := geom.NewVec(x, 0)
		e := kind.EntryOf(porterKind, float32(0)).Told(part...)
		unpack = append(unpack, act.Self.Do(world.SpawnPart{Entry: e, At: at}))
	}
	c.w.Roles().Define(caravanRole, act.Branch[world.Unfold]{
		Name: "unpack",
		When: cond.Self.If(cond.Always),
		Then: act.Steps(unpack...),
	})
	kind.Define[float32](c.w.Kinds(), caravanKind, kind.Spec{
		comp.Load(func(x float32) world.Position {
			return world.Position{AABB: plane.NewAABB(geom.NewVec(x, 100), 10, 10)}
		}),
		comp.Const(world.Velocity{}),
		comp.Const(world.Aggregate{}),
		ceremony.Plays(c.w.Roles().Named(caravanRole)),
	})
	kind.Define[float32](c.w.Kinds(), porterKind, kind.Spec{
		comp.Const(world.Position{AABB: plane.NewAABB(geom.NewVec(-5, -5), 10, 10)}),
		comp.Const(world.Velocity{}),
	})
	c.w.Write().Seed(kind.Named[float32](c.w.Kinds(), caravanKind).Entry(500).Told(whole...))
	if err := c.w.Populate(); err != nil {
		t.Fatal(err)
	}
	ctx := &installCtx{ecs: goke.New()}
	for _, p := range []plugin.Plugin{c.w, c.sel, c.players} {
		if err := p.Install(ctx); err != nil {
			t.Fatal(err)
		}
	}
	if err := ctx.Deliver(c.w.Kinds().Played()...); err != nil {
		t.Fatal(err)
	}
	var systems []goke.System
	for _, produce := range ctx.pending {
		systems = append(systems, produce()...)
	}
	for _, s := range ctx.drawn {
		if r, ok := s.(*selection.Renderer); ok {
			c.renderer = r
			systems = append(systems, goke.SystemFn{OnInit: r.Init})
		}
	}
	systems = append(systems, goke.SystemFn{OnInit: func(si *goke.SysInit) {
		c.q = si.NewQueryBuilder(&c.base).Optional(&c.marks, &c.partOf).Build()
	}})
	c.ecs = ctx.ecs
	c.ecs.Setup(systems...)
	c.ecs.SetPlan(func(rc goke.RunCtx, d time.Duration) {
		c.sel.RunPlan(rc, d)
		c.w.RunPlan(rc, d)
		c.w.Write().Clock().Replay(rc, d)
		rc.Sync()
		c.players.RunPlan(rc, d)
	})
	for range 6 {
		c.ecs.Tick(time.Second / 10)
	}
	for c.q.All(); c.q.Next(); {
		cur := c.q.Cursor()
		of := c.partOf.Slice(cur)
		for _, id := range cur.IDs {
			if of == nil {
				c.whole = id
			} else {
				c.parts = append(c.parts, id)
			}
		}
	}
	slices.SortFunc(c.parts, func(a, b uid.UID64) int {
		return int(c.middle(a).X - c.middle(b).X)
	})
	if len(c.parts) != 3 {
		t.Fatalf("%d porters stand, want the three the caravan unfolded into", len(c.parts))
	}
	return c
}

func (c *caravan) middle(id uid.UID64) geom.Vec {
	c.t.Helper()
	if !c.q.Seek(id) {
		c.t.Fatalf("entity %d is gone", id)
	}
	return c.base.At(c.q.Cursor()).Pos.Center()
}

func (c *caravan) has(id uid.UID64, t tag.Tag[selection.Family]) bool {
	c.t.Helper()
	if !c.q.Seek(id) {
		c.t.Fatalf("entity %d is gone", id)
	}
	m := c.marks.At(c.q.Cursor())
	return m != nil && m.Has(t)
}

// selected is every entity Selected
func (c *caravan) selected() []uid.UID64 {
	var out []uid.UID64
	for c.q.All(); c.q.Next(); {
		cur := c.q.Cursor()
		marks := c.marks.Slice(cur)
		for i, id := range cur.IDs {
			if marks != nil && marks[i].Has(c.sel.Tags().Selected) {
				out = append(out, id)
			}
		}
	}
	return out
}

func (c *caravan) drag(from, to geom.Vec) {
	events := &control.InputEvents{}
	events.AddClickEvent(int(from.X), int(from.Y), control.MouseButtonLeft, control.ActionPress)
	events.AddClickEvent(int(to.X), int(to.Y), control.MouseButtonLeft, control.ActionRelease)
	c.players.EventHandler().HandleEvents(events)
	c.ecs.Tick(time.Second / 10)
}

func (c *caravan) click(at geom.Vec) { c.drag(at, at) }

func mine(me, _ command.Issuer) (whole, part []command.Command) {
	return []command.Command{players.Give{To: me}, selection.Allow{}},
		[]command.Command{players.Give{To: me}}
}

func TestParts_AClickOnAPartSelectsItsWholeAlone(t *testing.T) {
	c := newCaravan(t, mine)
	c.click(c.middle(c.parts[1]))
	if got := c.selected(); !slices.Equal(got, []uid.UID64{c.whole}) {
		t.Errorf("a click on a porter selected %v, want the caravan %d alone", got, c.whole)
	}
}

func TestParts_ABoxOverTwoPartsSelectsTheirWholeOnce(t *testing.T) {
	c := newCaravan(t, mine)
	from, to := c.middle(c.parts[0]), c.middle(c.parts[1])
	c.drag(geom.NewVec(from.X-8, from.Y-8), geom.NewVec(to.X+8, to.Y+8))
	if got, ok := c.sel.Read(0).Chosen(c.me.ID); !ok || got != c.whole {
		t.Errorf("a box over two porters chose %d (one %v), want the caravan %d alone", got, ok,
			c.whole)
	}
}

func TestParts_APartSelectableItselfSelectsItself(t *testing.T) {
	c := newCaravan(t, func(me, _ command.Issuer) (whole, part []command.Command) {
		return []command.Command{players.Give{To: me}, selection.Allow{}},
			[]command.Command{players.Give{To: me}, selection.Allow{}}
	})
	c.click(c.middle(c.parts[2]))
	if got := c.selected(); !slices.Equal(got, []uid.UID64{c.parts[2]}) {
		t.Errorf("a click on a porter selectable itself selected %v, want the porter %d alone",
			got, c.parts[2])
	}
}

func TestParts_APartSelectsNothingOfAWholeThePlayerMayNotSelect(t *testing.T) {
	for name, tell := range map[string]told{
		"not selectable": func(me, _ command.Issuer) (whole, part []command.Command) {
			return []command.Command{players.Give{To: me}}, []command.Command{players.Give{To: me}}
		},
		"another's": func(me, rival command.Issuer) (whole, part []command.Command) {
			return []command.Command{players.Give{To: rival}, selection.Allow{}},
				[]command.Command{players.Give{To: me}}
		},
	} {
		t.Run(name, func(t *testing.T) {
			c := newCaravan(t, tell)
			c.click(c.middle(c.parts[0]))
			if got := c.selected(); len(got) > 0 {
				t.Errorf("a click on a porter selected %v, want none", got)
			}
		})
	}
}

func TestParts_AWholeUnfoldedIsNoHitWhereItStands(t *testing.T) {
	c := newCaravan(t, mine)
	c.click(c.middle(c.whole))
	if got := c.selected(); len(got) > 0 {
		t.Errorf("a click where the unfolded caravan stands selected %v, want none", got)
	}
}

func TestParts_TheCursorOverAPartHoversItsWhole(t *testing.T) {
	c := newCaravan(t, mine)
	at := c.middle(c.parts[0])
	c.players.EventHandler().HandleEvents(&control.InputEvents{MousePos: at,
		CursorInWindow: true})
	// the players point at the end of a tick, the selection takes it at the next
	for range 2 {
		c.ecs.Tick(time.Second / 10)
	}
	hovered := c.sel.Tags().Hovered
	if !c.has(c.whole, hovered) || c.has(c.parts[0], hovered) {
		t.Errorf("the cursor over a porter: the caravan hovered %v, the porter %v; want the "+
			"caravan alone", c.has(c.whole, hovered), c.has(c.parts[0], hovered))
	}
}

func TestRenderer_OutlinesEveryPartOfAWholeUnfoldedSelectedNotTheWhole(t *testing.T) {
	c := newCaravan(t, mine)
	c.click(c.middle(c.parts[1]))
	r := c.renderer
	if r == nil {
		t.Fatal("the selection draws no Renderer")
	}
	r.Publish()
	r.Swap()
	var f render.Frame
	f.Reset(c.cam)
	r.Compose(&f, c.cam)
	sides := map[uid.UID64]int{}
	f.Each(func(_ render.At, v []render.Vertex) {
		var x, y float32
		for _, p := range v {
			x += p.DstX / float32(len(v))
			y += p.DstY / float32(len(v))
		}
		for _, id := range append([]uid.UID64{c.whole}, c.parts...) {
			m := c.middle(id)
			sx, sy := c.cam.ToScreen(m.X, m.Y)
			if x > sx-8 && x < sx+8 && y > sy-8 && y < sy+8 {
				sides[id]++
			}
		}
	})
	if f.Len() != 12 || sides[c.whole] > 0 {
		t.Errorf("%d sides, %d round the caravan, want the four of each of its three porters "+
			"alone", f.Len(), sides[c.whole])
	}
	for k, id := range c.parts {
		if sides[id] != 4 {
			t.Errorf("porter %d outlined by %d sides, want 4", k, sides[id])
		}
	}
}
