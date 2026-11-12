package bridges_test

import (
	"bytes"
	"encoding/gob"
	"math"
	"reflect"
	"slices"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/aabbworld/plane"
	"github.com/kjkrol/ceremony/command"
	"github.com/kjkrol/ceremony/host"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/internal/f32"
	"github.com/kjkrol/gram/internal/parallel"
	"github.com/kjkrol/gram/plugin"
	"github.com/kjkrol/gram/plugins/board"
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/board/grid"
	"github.com/kjkrol/gram/plugins/bridges"
	"github.com/kjkrol/gram/plugins/bridges/site"
	"github.com/kjkrol/gram/plugins/cameras/cameratest"
	"github.com/kjkrol/gram/plugins/collision"
	"github.com/kjkrol/gram/plugins/light"
	"github.com/kjkrol/gram/plugins/motion"
	"github.com/kjkrol/gram/plugins/topography"
	"github.com/kjkrol/gram/plugins/vision"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/render"
	"github.com/kjkrol/uid"
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
func (*installer) Pool() *parallel.Pool                        { return nil }

// dale is a board 10 x 6 of cells 10 wide, the ground 20 up but for a valley at 0 from x 30 to
// 60.
type dale struct {
	t       *testing.T
	g       grid.Grid
	w       *world.Plugin
	m       *motion.Plugin
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
	cast    goke.Comp[light.Shadow]
	fixed   *goke.Query
	fixedB  goke.Comp[world.Base]
	all     *goke.Query
	allB    goke.Comp[world.Base]
	allZ    goke.OptComp[world.Z]
	allLie  goke.OptComp[world.Lie]
	allCast goke.OptComp[light.Shadow]
	allOf   goke.OptComp[world.PieceOf]
	allDeck goke.OptComp[cell.Floor]
	allPier goke.OptComp[collision.Collider]
	allWho  goke.OptComp[world.Whole]
	allKept goke.OptComp[world.Fixed]
	allSeen goke.OptComp[vision.Transparency]
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
	d.m = motion.NewPlugin(d.w)
	d.col = collision.NewPlugin(d.w, d.m)
	d.board = board.NewPlugin(d.g, &cell.MultipleOccupancy{}, d.w).
		WithMotion(d.m, cell.DefaultHiking).
		WithCollision(d.col)
	d.topo = topography.NewPlugin(d.w, d.board, nil, topography.Config{})
	d.bridges = bridges.NewPlugin(d.w, d.board,
		bridges.Config{Rules: rules}).WithShaper(d.topo.Write())
	d.board.Write().Board().SetAll(cell.Kind{Pace: 1, Allows: cell.Land})
	d.bridges.Types().Define("bridge", bridges.Spec{
		Allows:    cell.Land,
		Width:     6,
		Thickness: 1,
		Pace:      1,
		Rail:      cell.Rail{Keeps: cell.Land, Height: 1},
		Pier:      2,
	})
	if err := d.w.Carry(d.bridges); err != nil {
		t.Fatal(err)
	}
	return d
}

func valley(p geom.Vec) float32 {
	if p.X > 30 && p.X < 60 {
		return 0
	}
	return 20
}

func (d *dale) start(heights func(geom.Vec) float32) {
	d.t.Helper()
	d.topo.Write().Seed(heights)
	ctx := &installer{ecs: goke.New()}
	for _, p := range []plugin.Plugin{d.w, d.m, d.col, d.board, d.topo, d.bridges} {
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
	for _, s := range d.drawn {
		if b, ok := s.(plugin.Bound); ok {
			systems = append(systems, goke.SystemFn{OnInit: b.Init})
		}
	}
	systems = append(systems, goke.SystemFn{OnInit: func(si *goke.SysInit) {
		d.floors = si.NewQueryBuilder(&d.floor, &d.on).Build()
		d.piers = si.NewQueryBuilder(&d.pierB, &d.pierZ).
			Include(goke.Include[collision.Collider]()).Build()
		d.blocks = si.NewQueryBuilder(&d.blockZ, &d.cast).Build()
		d.fixed = si.NewQueryBuilder(&d.fixedB).Include(goke.Include[world.Fixed]()).Build()
		d.all = si.NewQueryBuilder(&d.allB).Optional(&d.allZ, &d.allLie, &d.allCast, &d.allOf,
			&d.allDeck, &d.allPier, &d.allWho, &d.allKept, &d.allSeen).Build()
	}})
	ctx.ecs.Setup(systems...)
	ctx.ecs.SetPlan(func(rc goke.RunCtx, dt time.Duration) {
		d.bridges.RunPlan(rc, dt)
		d.w.RunPlan(rc, dt)
		d.m.RunPlan(rc, dt)
		d.col.RunPlan(rc, dt)
		d.board.RunPlan(rc, dt)
		d.topo.RunPlan(rc, dt)
		rc.Sync()
		d.w.Write().Clock().Replay(rc, dt)
	})
	d.ecs = ctx.ecs
}

func (d *dale) give(cmd command.Command) {
	d.t.Helper()
	if !d.w.Write().Carrier().Put(1, cmd) {
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
	d.start(func(p geom.Vec) float32 {
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
	if len(piers) != 2 || piers[0].Y != 35 || f32.Abs(piers[0].X+piers[1].X-90) > f32.Eps(90) {
		t.Errorf("piers stand at %v, want at (40, 35) and (50, 35)", piers)
	}
	if h := d.topo.Write().Relief().At(geom.NewVec(70, 30)); h != 20 {
		t.Errorf("the far head's corner stands at %v, want levelled at the deck, 20", h)
	}
	n := 0
	for d.blocks.All(); d.blocks.Next(); {
		for _, s := range d.cast.Slice(d.blocks.Cursor()) {
			if s.Cast == light.Baked {
				n++
			}
		}
	}
	if n != 1+len(piers) {
		t.Errorf("%d blocks cast their shadows with the ground's, want the bridge's handle and "+
			"the %d piers", n, len(piers))
	}
	n = 0
	for d.fixed.All(); d.fixed.Next(); {
		n += len(d.fixed.Cursor().IDs)
	}
	if n != 1+len(decks)+len(piers) {
		t.Errorf("%d Fixed, want the handle, the %d pieces of deck and the %d piers", n,
			len(decks), len(piers))
	}
	d.wholeOver(at(2, 3), at(6, 3), 20, 5, 2)
}

// standing is an entity standing as the dale's query reads it
type standing struct {
	id           uid.UID64
	base         world.Base
	z            world.Z
	lie          world.Lie
	cast         light.Casting
	of           uid.UID64 // the whole it is a piece of, 0 for none
	deck, pier   bool
	whole, fixed bool
	seen         float32 // its Transparency, 0 for none
}

func (d *dale) standings() []standing {
	var out []standing
	for d.all.All(); d.all.Next(); {
		cur := d.all.Cursor()
		zs, lies, casts := d.allZ.Slice(cur), d.allLie.Slice(cur), d.allCast.Slice(cur)
		ofs, seens := d.allOf.Slice(cur), d.allSeen.Slice(cur)
		for i, id := range cur.IDs {
			s := standing{id: id, base: d.allB.Slice(cur)[i], deck: d.allDeck.Present(cur),
				pier: d.allPier.Present(cur), whole: d.allWho.Present(cur),
				fixed: d.allKept.Present(cur)}
			if zs != nil {
				s.z = zs[i]
			}
			if lies != nil {
				s.lie = lies[i]
			}
			if casts != nil {
				s.cast = casts[i].Cast
			}
			if ofs != nil {
				s.of = ofs[i].Whole
			}
			if seens != nil {
				s.seen = seens[i].Value
			}
			out = append(out, s)
		}
	}
	return out
}

// wholeOver checks the one bridge standing from the middle from to the middle to, its deck at
// deck on decks pieces and piers piers, is a handle over its pieces: the handle Fixed, lying
// from head to head, seen through, its shadow Baked and the block it casts the one its pieces of
// deck would cast joined; every piece under it, the decks casting no shadow, the piers theirs
func (d *dale) wholeOver(from, to geom.Vec, deck float32, decks, piers int) {
	d.t.Helper()
	var handles, pieces []standing
	for _, s := range d.standings() {
		switch {
		case s.whole:
			handles = append(handles, s)
		case s.deck || s.pier:
			pieces = append(pieces, s)
		}
	}
	if len(handles) != 1 {
		d.t.Fatalf("%d handles stand, want the bridge's one", len(handles))
	}
	h := handles[0]
	box := h.base.Pos.AABB
	wantLie := world.Lie{Length: to.X - from.X, Width: 6}
	if !h.fixed || h.cast != light.Baked || h.lie != wantLie ||
		box.TopLeft != geom.NewVec(from.X, from.Y-3) ||
		box.BottomRight != geom.NewVec(to.X, to.Y+3) ||
		h.base.Vel.Dir != geom.NewVec(1, 0) || h.z != (world.Z{Altitude: deck, Height: 1}) ||
		h.seen != 1 {
		d.t.Errorf("the handle is Fixed %v, casts %v, lies %+v over %v headed %v at %+v, seen "+
			"through %v; want Fixed, Baked, lying %+v from %v to %v 6 wide, east, from %v down "+
			"1, seen through 1", h.fixed, h.cast, h.lie, box, h.base.Vel.Dir, h.z, h.seen,
			wantLie, from, to, deck)
	}
	var lain []render.Block
	nDecks, nPiers := 0, 0
	for _, p := range pieces {
		if p.of != h.id {
			d.t.Errorf("a piece of the bridge is a piece of %d, want of the handle %d", p.of, h.id)
		}
		switch {
		case p.deck:
			nDecks++
			if p.cast != light.NoShadow {
				d.t.Errorf("a piece of deck casts %v, want no shadow: the handle casts it", p.cast)
			}
			lain = append(lain, lying(p.base.Pos.Center(), p.base.Vel.Dir, p.lie,
				p.z.Altitude-p.z.Height, p.z.Altitude))
		case p.pier:
			nPiers++
			if p.cast != light.Baked {
				d.t.Errorf("a pier casts %v, want its own Baked shadow", p.cast)
			}
		}
	}
	if nDecks != decks || nPiers != piers {
		d.t.Errorf("%d pieces of deck and %d piers under the handle, want %d and %d", nDecks,
			nPiers, decks, piers)
	}
	joint := joined(lain)
	got := lying(h.base.Pos.Center(), h.base.Vel.Dir, h.lie, h.z.Altitude-h.z.Height,
		h.z.Altitude)
	if len(joint) != 1 || got != joint[0] {
		d.t.Errorf("the handle casts the block %+v, want the one its pieces of deck joined "+
			"cast, %v", got, joint)
	}
}

// lying is the block the light makes of what lies along: its Lie round m headed dir, from bottom
// up to top
func lying(m, dir geom.Vec, lie world.Lie, bottom, top float32) render.Block {
	c := lie.Corners(m, dir)
	length := f32.Hypot(c[1].X-c[0].X, c[1].Y-c[0].Y)
	width := f32.Hypot(c[2].X-c[0].X, c[2].Y-c[0].Y)
	b := render.Block{
		X:      (c[0].X + c[3].X) / 2,
		Y:      (c[0].Y + c[3].Y) / 2,
		UX:     1,
		Along:  length / 2,
		Wide:   width / 2,
		Bottom: bottom,
		Top:    top,
	}
	if length > 0 {
		b.UX, b.UY = (c[1].X-c[0].X)/length, (c[1].Y-c[0].Y)/length
	}
	return b
}

// joined is the blocks of a deck laid in pieces joined into one, as the ground's shade joined
// them before the handle cast the deck's shadow
func joined(bs []render.Block) []render.Block {
	for i := range bs {
		if bs[i].UX < 0 || bs[i].UX == 0 && bs[i].UY < 0 {
			bs[i].UX, bs[i].UY = -bs[i].UX, -bs[i].UY
		}
	}
	for again := true; again; {
		again = false
		for i := 0; i < len(bs); i++ {
			for j := i + 1; j < len(bs); j++ {
				if b, ok := join(bs[i], bs[j]); ok {
					bs[i], again = b, true
					bs = slices.Delete(bs, j, j+1)
					j--
				}
			}
		}
	}
	return bs
}

func join(a, b render.Block) (render.Block, bool) {
	const near = 0.01
	apart := func(x, y float32) bool { return math.Abs(float64(x-y)) > near }
	if apart(a.UX, b.UX) || apart(a.UY, b.UY) || apart(a.Wide, b.Wide) ||
		apart(a.Bottom, b.Bottom) || apart(a.Top, b.Top) {
		return a, false
	}
	dx, dy := b.X-a.X, b.Y-a.Y
	if apart(dx*a.UY-dy*a.UX, 0) {
		return a, false
	}
	s := dx*a.UX + dy*a.UY
	if s-b.Along > a.Along+near || s+b.Along < -a.Along-near {
		return a, false
	}
	lo, hi := min(-a.Along, s-b.Along), max(a.Along, s+b.Along)
	mid := (lo + hi) / 2
	a.X, a.Y, a.Along = a.X+a.UX*mid, a.Y+a.UY*mid, (hi-lo)/2
	return a, true
}

func TestTool_TheBridgeInHandLiesOverItsCells(t *testing.T) {
	d := newDale(t)
	d.start(valley)
	at := func(x, y uint32) geom.Vec { return d.g.CellCenter(d.g.At(x, y)) }
	d.give(bridges.Mark{At: at(2, 3)})
	d.give(bridges.Aim{At: at(5, 3)})
	hand := d.drawn[len(d.drawn)-1] // the bridges' own, installed last
	cam := cameratest.NewFromSpace(100, 60, 0)
	hand.(plugin.Publisher).Publish()
	hand.(plugin.Publisher).Swap()
	var f render.Frame
	f.Reset(cam)
	hand.Compose(&f, cam)
	n := 0
	f.Each(func(at render.At, _ []render.Vertex) {
		n++
		if at.Top() || f32.Abs(float32(at.Z())-20.9) > 1e-4 {
			t.Errorf("the bridge in hand lies at %v, want over its head's ground at 20", at)
		}
	})
	if n == 0 {
		t.Fatal("the bridge in hand is not drawn")
	}
}

// sprites is a look noting the sprite of every entity it is handed to draw
type sprites struct {
	world.Look
	drawn map[render.SpriteID]bool
}

func (s sprites) Sprite(f *render.Frame, cam camera.Camera, box plane.AABB, z world.Z,
	atlas render.AtlasSource, a render.Appearance, lit render.Illumination) {
	s.drawn[a.SpriteID] = true
}

type sheet struct{}

func (sheet) Atlas() *render.Image                            { return nil }
func (sheet) UV(render.SpriteID) (sx0, sy0, sx1, sy1 float32) { return 0, 0, 1, 1 }
func (sheet) White() (u, v float32)                           { return 0, 0 }

func TestTool_TheWorldDrawsABridgesPiecesNeverItsHandle(t *testing.T) {
	d := newDale(t)
	d.start(valley)
	at := func(x, y uint32) geom.Vec { return d.g.CellCenter(d.g.At(x, y)) }
	d.give(bridges.Mark{At: at(2, 3)})
	d.give(bridges.Aim{At: at(6, 3)})
	d.give(bridges.Mark{At: at(6, 3)})
	d.give(bridges.Build{})
	cam := cameratest.NewFromSpace(100, 60, 0)
	d.w.Write().NewView(cam)
	look := sprites{Look: d.w.Read(0).Look(), drawn: map[render.SpriteID]bool{}}
	d.w.Write().SetLook(look)
	d.w.Write().SetAtlas(sheet{})
	d.ticks(5)
	r := d.drawn[0] // the world's own, installed first
	r.(plugin.Publisher).Publish()
	r.(plugin.Publisher).Swap()
	var f render.Frame
	f.Reset(cam)
	r.Compose(&f, cam)
	_, handle, _, ok := d.w.Kinds().Lookup("bridge.whole")
	bridge := d.bridges.Types().Named("bridge")
	if !ok || look.drawn[handle] || !look.drawn[bridge.Deck()] || !look.drawn[bridge.Pier()] {
		t.Errorf("the world draws the handle %v, the decks %v, the piers %v; want the pieces "+
			"alone", look.drawn[handle], look.drawn[bridge.Deck()], look.drawn[bridge.Pier()])
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

// load decodes into d's bridges what from's give to be saved, as the engine does with every
// Serializable, and has d's take it (Restore)
func (d *dale) load(from *dale) {
	d.t.Helper()
	var buf bytes.Buffer
	enc := gob.NewEncoder(&buf)
	for _, v := range from.bridges.Serializable().Persisted() {
		if err := enc.Encode(v); err != nil {
			d.t.Fatal(err)
		}
	}
	dec := gob.NewDecoder(&buf)
	for _, v := range d.bridges.Serializable().Persisted() {
		if err := dec.Decode(v); err != nil {
			d.t.Fatal(err)
		}
	}
	d.bridges.Restore()
}

func TestSave_ABridgeOrderedInThePauseIsBuiltAfterALoad(t *testing.T) {
	saved := newDale(t)
	saved.start(valley)
	at := func(x, y uint32) geom.Vec { return saved.g.CellCenter(saved.g.At(x, y)) }
	saved.give(world.Pause{})
	saved.give(bridges.Mark{At: at(2, 3)})
	saved.give(bridges.Aim{At: at(6, 3)})
	saved.give(bridges.Mark{At: at(6, 3)})
	saved.give(bridges.Build{})
	saved.ticks(5)
	if got := saved.decks(); len(got) != 0 {
		t.Fatalf("built %d pieces of deck in the tactical pause, want the bridge waiting",
			len(got))
	}
	loaded := newDale(t)
	loaded.load(saved)
	orders := loaded.bridges.Res.Orders
	if len(orders) != 1 {
		t.Fatalf("loaded %d bridges ordered, want the one confirmed in the pause", len(orders))
	}
	o := orders[0]
	if o.Kind != "bridge" || o.Plan.From != saved.g.At(2, 3) || o.Plan.To != saved.g.At(6, 3) ||
		o.Plan.Deck != 20 || len(o.Plan.Spans) != 3 || len(o.Plan.Piers) != 2 ||
		o.Plan.Piers[0] != geom.NewVec(40, 35) {
		t.Errorf("loaded the order %+v, want a bridge from [2, 3] to [6, 3], its deck at 20, "+
			"over 3 spans on piers from (40, 35)", o)
	}
	loaded.start(valley)
	loaded.give(world.Pause{})
	loaded.ticks(5)
	if got := loaded.decks(); len(got) != 0 {
		t.Fatalf("built %d pieces of deck in the tactical pause after the load, want none yet",
			len(got))
	}
	loaded.give(world.Pause{})
	loaded.ticks(5)
	if got := loaded.decks(); len(got) != 5 || len(loaded.bridges.Res.Orders) != 0 {
		t.Errorf("after the pause, built %d pieces of deck, %d bridges still ordered; want 5 "+
			"pieces and none", len(got), len(loaded.bridges.Res.Orders))
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
			d.board.Read(0).CellKinds().Define("earth", cell.Kind{Pace: 1, Allows: cell.Land})
			level, err := d.board.Lay(file(tc.entity), "map.yaml")
			if err != nil {
				t.Fatal(err)
			}
			err = d.bridges.Write().Spawn(level)
			if err == nil {
				d.topo.Write().Seed(level.Heights())
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
			at := func(x, y uint32) geom.Vec { return d.g.CellCenter(d.g.At(x, y)) }
			d.wholeOver(at(2, 3), at(6, 3), 20, 5, 2)
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

func TestSpawn_ABridgeAlongAPathTurnsAndClimbs(t *testing.T) {
	file := func(entity string) fstest.MapFS {
		var corners strings.Builder
		for y := 0; y <= 6; y++ {
			for x := 0; x <= 10; x++ {
				if x > 0 {
					corners.WriteString(" ")
				}
				switch {
				case x <= 3:
					corners.WriteString("20")
				case x >= 6:
					corners.WriteString("26")
				default:
					corners.WriteString("0")
				}
			}
			corners.WriteString("\n")
		}
		return fstest.MapFS{"map.yaml": {Data: []byte("board:\n  default: earth\n" +
			"  entities:\n    - " + entity + "\nrelief:\n  corners: |\n" +
			indent(corners.String()))}}
	}
	lay := func(t *testing.T, entity string) (*dale, *board.Level, error) {
		d := newDale(t)
		d.board.Read(0).CellKinds().Define("earth", cell.Kind{Pace: 1, Allows: cell.Land})
		level, err := d.board.Lay(file(entity), "map.yaml")
		if err != nil {
			t.Fatal(err)
		}
		return d, level, d.bridges.Write().Spawn(level)
	}
	if _, _, err := lay(t, "{kind: bridge, path: [[2, 2, 0, 0], [3, 2, 0, 0]]}"); err == nil {
		t.Error("a path of two cells is taken for a bridge")
	}
	if _, _, err := lay(t,
		"{kind: bridge, path: [[2, 2, 0, 0], [4, 2, 0, 0], [6, 2, 0, 0]]}"); err == nil {
		t.Error("a path leaping a cell is taken for a bridge")
	}
	d, level, err := lay(t, "{kind: bridge, lanes: 2, width: 7, path: "+
		"[[2, 2, 0, 0], [3, 2, 0, 0], [4, 3, 0, 0], [5, 3, 0, 0], [6, 3, 0, 0]]}")
	if err != nil {
		t.Fatal(err)
	}
	wound := d.bridges.Res.Wound
	d.bridges.Serializable().Persisted()
	d.bridges.Res = bridges.Resources{}
	d.bridges.Restore()
	if !reflect.DeepEqual(d.bridges.Res.Wound, wound) || len(wound) != 1 {
		t.Errorf("a load brought back %v, saved %v", d.bridges.Res.Wound, wound)
	}
	d.topo.Write().Seed(level.Heights())
	if err := d.board.Populate(); err != nil {
		t.Fatal(err)
	}
	d.start(level.Heights())
	d.ticks(2)
	decks := d.decks()
	if len(decks) != 5 {
		t.Fatalf("built %d pieces of deck, want 5", len(decks))
	}
	heights := level.Heights()
	level4 := func(c cell.ID) float32 {
		sum := float32(0)
		for _, p := range d.g.CellOutline(c, nil) {
			sum += heights(p)
		}
		return sum / 4
	}
	run := []float32{0, 10, 10 + 10*f32.Sqrt(2), 20 + 10*f32.Sqrt(2), 30 + 10*f32.Sqrt(2)}
	for i, xy := range [][2]uint32{{2, 2}, {3, 2}, {4, 3}, {5, 3}, {6, 3}} {
		c := d.g.At(xy[0], xy[1])
		f := decks[c]
		if f.Graded || f.Width != 7 || f.Lanes != 2 {
			t.Errorf("the deck at %v: %+v, want one winding, 7 wide, of two lanes", xy, f)
		}
		if i == 0 || i == 4 {
			if f.Height != 0 {
				t.Errorf("the head at %v stands %v over its ground", xy, f.Height)
			}
			continue
		}
		want := 20 + 6*run[i]/run[4]
		if got := level4(c) + f.Height; f32.Abs(got-want) > 1e-3 {
			t.Errorf("the deck at %v lies at %v, want %v: even from head to head", xy, got, want)
		}
	}
	turn := decks[d.g.At(4, 3)].Links
	back, _ := grid.Link(d.g, d.g.At(4, 3), d.g.At(3, 2))
	on, _ := grid.Link(d.g, d.g.At(4, 3), d.g.At(5, 3))
	if turn != back|on {
		t.Errorf("where the bridge turns its deck runs %08b, want back north-west and on east",
			turn)
	}
	// a step along the deck and the one off it onto its head are paced by the deck's own
	// slope, 6 up in 44: the same either way, whatever the ground does under it
	walked := cell.Kind{Pace: 1, Allows: cell.Land}
	read := d.board.Read(0)
	along := read.Pace(d.g.At(4, 3), d.g.At(5, 3), walked, cell.OnDeck, cell.Land)
	off := read.Pace(d.g.At(5, 3), d.g.At(6, 3), walked, cell.OnGround, cell.Land)
	want := cell.PaceOf(cell.DefaultHiking, cell.Step{Kind: walked, Domain: cell.Land,
		Slope: 6 / float64(run[4])})
	if math.Abs(along-want) > 1e-3 || math.Abs(off-want) > 1e-3 {
		t.Errorf("a step along the deck keeps %.3f of the pace and the one off it onto its "+
			"head %.3f, want %.3f for the deck's slope", along, off, want)
	}
	// whoever walks it walks a ramp: at the edge between two pieces as high as half way from
	// one to the next, a step up to neither
	top := func(i int) float32 { return 20 + 6*run[i]/run[4] }
	edge := geom.NewVec(50, 35)
	half := (top(2) + top(3)) / 2
	if floor, _ := read.Footing().Surface(edge, cell.Land, half, 0.1); f32.Abs(floor-half) > 1e-3 {
		t.Errorf("at the edge between two pieces the deck lies at %v, want %v", floor, half)
	}
	c, lv, ok := read.Board().LevelFor(geom.NewVec(50.5, 35), half, 0.1)
	if !ok || c != d.g.At(5, 3) || lv != cell.OnDeck {
		t.Errorf("at the low edge of a piece, feet at %v: on %v of %v, want its deck", half, lv,
			c)
	}
}
