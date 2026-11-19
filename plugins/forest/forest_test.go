package forest_test

import (
	"image/color"
	"math"
	"strings"
	"testing"
	"time"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/aabbworld/plane"
	"github.com/kjkrol/ceremony"
	"github.com/kjkrol/ceremony/act"
	"github.com/kjkrol/ceremony/comp"
	"github.com/kjkrol/ceremony/cond"
	"github.com/kjkrol/ceremony/effect"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/entity/kind"
	"github.com/kjkrol/gram/game"
	"github.com/kjkrol/gram/game/stage"
	"github.com/kjkrol/gram/internal/engine"
	"github.com/kjkrol/gram/internal/f32"
	"github.com/kjkrol/gram/lod"
	"github.com/kjkrol/gram/plugins/board"
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/board/grid"
	"github.com/kjkrol/gram/plugins/board/ground"
	"github.com/kjkrol/gram/plugins/board/look"
	"github.com/kjkrol/gram/plugins/cameras/cameratest"
	"github.com/kjkrol/gram/plugins/forest"
	"github.com/kjkrol/gram/plugins/motion"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/plugins/world/sector"
	"github.com/kjkrol/gram/render"
	"github.com/kjkrol/uid"
)

const (
	cellSize = 32
	// 4 by 2 sectors of 16 cells
	cols, rows = 64, 32
	width      = cols * cellSize
	height     = rows * cellSize
	sectorSide = 512
	frame      = time.Second / 60
	foldAfter  = 500 * time.Millisecond
	forestKind = "forest"
	groveCell  = "grove"
	thinnedEf  = "thinned"
)

type oneStageGame struct{ stage game.Stage }

func (oneStageGame) Props() game.Props { return game.Props{} }

func (g oneStageGame) Stages() (map[string]game.Stage, string) {
	return map[string]game.Stage{g.stage.Name(): g.stage}, g.stage.Name()
}

// ramp is a board whose ground rises a unit every four eastwards, flat to the eye
type ramp struct{ g grid.Grid }

func (ramp) Look() look.Look               { return look.Flat }
func (ramp) Heights() ground.Heights       { return rise{} }
func (ramp) Dressing() look.Dressing       { return nil }
func (ramp) Rise(from, to cell.ID) float64 { return 0 }

func (r ramp) Top(c cell.ID, height float32) ([4]float32, float32) {
	at := r.g.CellCenter(c)
	h := at.X/4 + height
	return [4]float32{h, h, h, h}, at.X / 4
}

type rise struct{}

func (rise) At(p geom.Vec) float32 { return p.X / 4 }
func (rise) Step() float32         { return cellSize }

type forestRow struct{}

type tree struct {
	name string
	spec forest.Spec
}

// woods is a kind of forest over the whole world: its trees, the roles it plays and its rules, the
// stands defining those roles among them
type woods struct {
	trees []tree
	plays []string
	rules func(p *forest.Plugin)
	// a kind uncovered declared beside the trees, for them to give way to
	walked bool
}

var (
	_ forest.Figure   = woods{}
	_ forest.Stateful = woods{}
	_ forest.Ruled    = woods{}
)

// planting is the woods of trees planting as stand says, in the role "woods"
func planting(stand forest.Stand, trees ...tree) woods {
	return woods{trees: trees, plays: []string{"woods"}, rules: func(p *forest.Plugin) {
		p.Stands().Define("woods", stand)
	}}
}

func (woods) Effects(p *forest.Plugin) {
	p.World().Write().Effects().Define(thinnedEf, effect.Spec{})
}

func (f woods) Rules(p *forest.Plugin) { f.rules(p) }

func (f woods) Define(p *forest.Plugin) {
	for _, t := range f.trees {
		p.Types().Define(t.name, t.spec)
	}
	w := p.World()
	var plays []*ceremony.Part
	for _, r := range f.plays {
		plays = append(plays, w.Roles().Named(r))
	}
	kind.Define[forestRow](w.Kinds(), forestKind, kind.Spec{
		comp.Const(world.Position{
			AABB: plane.NewAABB(geom.NewVec(width/2-8, height/2-8), 16, 16),
		}),
		comp.Const(world.Velocity{}),
		comp.Const(world.Aggregate{}),
		comp.Const(world.Field{Size: geom.NewVec(width, height)}),
		ceremony.Plays(plays...),
	})
}

func (f woods) Look(a *world.Atlas, p *forest.Plugin) {
	if f.walked {
		a.Add(p.World().Kinds().NewSprite(), 4, render.Solid(color.RGBA{R: 255, A: 255})).
			Uncovered()
	}
}

// grove is a world of 4 by 2 sectors over a board rising eastwards, a forest over all of it, and
// a camera seeing every sector at L3
type grove struct {
	t      *testing.T
	eng    *engine.Engine
	ecs    *goke.ECS
	w      *world.Plugin
	board  *board.Plugin
	p      *forest.Plugin
	kinds  []string
	cam    camera.Camera
	update func(goke.RunCtx, time.Duration)

	trees *goke.Query
	base  goke.Comp[world.Base]
	of    goke.Comp[world.PartOf]
	fixed goke.OptComp[world.Fixed]
	woods *goke.Query
	whole goke.Comp[world.Base]
}

var _ sector.Observers = (*grove)(nil)

func (g *grove) Observers(each func(cam camera.Camera)) { each(g.cam) }

func (g *grove) SetupSystems() []goke.System {
	return []goke.System{goke.SystemFn{OnInit: func(si *goke.SysInit) {
		g.trees = si.NewQueryBuilder(&g.base, &g.of).Optional(&g.fixed).Build()
		g.woods = si.NewQueryBuilder(&g.whole).Include(goke.Include[world.Field]()).Build()
	}}}
}

// newGrove is the grove of the forest f, its board's cells laid as lay says (all land where nil)
func newGrove(t *testing.T, f woods, lay func(b *board.Plugin)) *grove {
	t.Helper()
	g := &grove{t: t}
	for _, tr := range f.trees {
		g.kinds = append(g.kinds, tr.name)
	}
	// its picture shows the whole grove: the levels go by the zoom alone
	g.cam = cameratest.Showing(cameratest.NewFromSpace(width, height, 0))
	g.cam.SetViewport(100, 100)
	g.zoom(0.5)
	st := stage.New("grove").
		World(world.Config{
			Space:    world.SpaceCfg{Width: width, Height: height},
			Entities: world.EntitiesCfg{MaxCount: 8192, MinSize: 1, MaxSize: cellSize},
			Heights:  true,
			Sectors:  world.SectorsCfg{Buckets: 8},
			LOD: lod.Ladder{
				Scales: [lod.Levels - 1]float32{0, 0, 0.8, 0.4},
				Fold:   float32(foldAfter.Seconds()),
			},
		}).
		Plugins(func(w *world.Plugin) {
			g.w = w
			m := motion.NewPlugin(w)
			gr := grid.DefaultGrids{}.Square(cols, rows, cellSize)
			g.board = board.NewPlugin(gr, &cell.MultipleOccupancy{}, w).
				WithMotion(m, cell.DefaultHiking).
				WithMap(ramp{gr})
			g.board.Write().Board().SetAll(cell.Kind{Pace: 1, Allows: cell.Land})
			if lay != nil {
				lay(g.board)
			}
			g.p = forest.NewPlugin(w, g.board)
			w.Write().Observe(g)
			g.update = func(rc goke.RunCtx, d time.Duration) {
				w.RunPlan(rc, d)
				m.RunPlan(rc, d)
				g.board.RunPlan(rc, d)
				rc.Sync()
			}
		}).
		Figures(func() { g.p.Figures(f) }).
		Effects(func(ctx game.Initializer) {
			g.ecs = ctx.ECS()
			ctx.Setup(g)
		}).
		Spawn(func() {
			g.w.Write().Seed(kind.Named[forestRow](g.w.Kinds(), forestKind).Entry(forestRow{}))
		}).
		Update(func(rc goke.RunCtx, d time.Duration) { g.update(rc, d) })
	g.eng = engine.NewEngine(oneStageGame{stage: st})
	if err := g.eng.Init(); err != nil {
		t.Fatalf("Init: %v", err)
	}
	return g
}

func (g *grove) zoom(zoom float32) {
	g.cam.ZoomIn(zoom/g.cam.Zoom(), 0, 0)
	g.cam.CenterOn(width/2, height/2, 0)
}

func (g *grove) run(ticks int) {
	for range ticks {
		g.ecs.Tick(frame)
	}
}

// planted is where the trees of each kind stand, every one checked a part of the forest standing
// for good
func (g *grove) planted() map[string][]geom.Vec {
	g.t.Helper()
	byID := map[kind.ID]string{}
	for _, name := range g.kinds {
		byID[g.p.Types().Named(name).ID()] = name
	}
	var whole uid.UID64
	for g.woods.All(); g.woods.Next(); {
		whole = g.woods.Cursor().IDs[0]
	}
	got := map[string][]geom.Vec{}
	for g.trees.All(); g.trees.Next(); {
		cur := g.trees.Cursor()
		of := g.of.Slice(cur)
		for i, b := range g.base.Slice(cur) {
			name, ok := byID[b.TypeID]
			if !ok {
				continue
			}
			if !g.fixed.Present(cur) {
				g.t.Errorf("a %s does not stand for good", name)
			}
			if of[i].Whole != whole {
				g.t.Errorf("a %s is a part of %v, want of the forest %v", name, of[i].Whole, whole)
			}
			got[name] = append(got[name], b.Pos.Center())
		}
	}
	return got
}

func total(planted map[string][]geom.Vec) int {
	n := 0
	for _, at := range planted {
		n += len(at)
	}
	return n
}

var (
	oak     = forest.Spec{Height: 40, Crown: 20, Trunk: 0.3}
	pine    = forest.Spec{Height: 60, Crown: 14, Trunk: 0.25}
	heather = forest.Spec{Height: 12, Crown: 16}
)

func TestStands_AMixPlantsEveryKindByTheSeed(t *testing.T) {
	g := newGrove(t, planting(forest.Stand{Mix: []forest.Share{
		{Tree: "oak"},
		{Tree: "pine", Weight: 3},
	}}, tree{"oak", oak}, tree{"pine", pine}), nil)
	g.run(2)
	got := g.planted()
	oaks, pines := len(got["oak"]), len(got["pine"])
	if oaks == 0 || pines == 0 {
		t.Fatalf("%d oaks and %d pines planted, want both", oaks, pines)
	}
	if share := float64(pines) / float64(oaks+pines); math.Abs(share-0.75) > 0.05 {
		t.Errorf("pines are %.2f of the trees, want 0.75 by their weight", share)
	}
}

func TestStands_EachKindGrowsWithinItsBand(t *testing.T) {
	// the ground rises from 0 in the west to 512 in the east
	g := newGrove(t, planting(forest.Stand{Mix: []forest.Share{
		{Tree: "oak", To: 160},
		{Tree: "pine", From: 128, To: 320},
		{Tree: "heather", From: 400},
	}}, tree{"oak", oak}, tree{"pine", pine}, tree{"heather", heather}), nil)
	g.run(2)
	bands := map[string][2]float32{
		"oak":     {0, 160},
		"pine":    {128, 320},
		"heather": {400, 512},
	}
	for name, at := range g.planted() {
		if len(at) == 0 {
			t.Errorf("no %s planted", name)
		}
		band := bands[name]
		for _, p := range at {
			if h := p.X / 4; h < band[0] || h >= band[1] {
				t.Errorf("a %s at %v grows on ground %v high, want within %v", name, p, h, band)
				break
			}
		}
	}
	for _, at := range g.planted() {
		for _, p := range at {
			if h := p.X / 4; h >= 320 && h < 400 {
				t.Fatalf("a tree at %v on ground %v high, where no kind grows", p, h)
			}
		}
	}
}

func TestStands_PlantNoTreeOnARoadNorWithinItsMargin(t *testing.T) {
	// a road 6 wide along the middle row, west to east
	const row, wide = rows / 2, 6
	road := func(b *board.Plugin) {
		bd := b.Read(0).Board()
		for x := range uint32(cols) {
			c := bd.Grid.At(x, row)
			var links cell.Links
			for _, nx := range []uint32{x - 1, x + 1} {
				if nx >= cols {
					continue
				}
				if l, ok := grid.Link(bd.Grid, c, bd.Grid.At(nx, row)); ok {
					links |= l
				}
			}
			b.Write().Board().SetWay(c, cell.Way{
				Kind:  cell.Kind{Name: cell.Named("road"), Pace: 1, Allows: cell.Land},
				Width: wide,
				Links: links,
			})
		}
	}
	g := newGrove(t, planting(forest.Stand{
		Density: 3,
		Jitter:  0.25,
		Mix:     []forest.Share{{Tree: "oak"}},
	}, tree{"oak", oak}), road)
	g.run(2)
	axis := float32(row*cellSize + cellSize/2)
	margin := float32(wide)/2 + float32(wide)/4 + oak.Crown/2
	// the band runs from the first cell's middle to the last's: past them only its hubs
	beside := 0
	for _, p := range g.planted()["oak"] {
		d := f32.Abs(p.Y - axis)
		if d <= margin && p.X >= cellSize/2 && p.X <= width-cellSize/2 {
			t.Fatalf("an oak at %v, %v from the road's axis, within its margin %v", p, d, margin)
		}
		if d <= margin+cellSize {
			beside++
		}
	}
	if beside == 0 {
		t.Error("no oak within a cell beyond the road's margin: the margin is too wide")
	}
}

func TestStands_PlantTheirDensityOnTheirCells(t *testing.T) {
	// the grove's cells are the western half's
	grove := cell.Kind{Name: cell.Named(groveCell), Pace: 1, Allows: cell.Land}
	half := func(b *board.Plugin) {
		bd := b.Write().Board()
		for y := range uint32(rows) {
			for x := range uint32(cols / 2) {
				bd.Set(b.Read(0).Board().Grid.At(x, y), grove)
			}
		}
	}
	for _, density := range []float32{0, 0.5, 3} {
		want := density
		if want == 0 {
			want = forest.DefaultDensity
		}
		g := newGrove(t, planting(forest.Stand{
			Cells:   []string{groveCell},
			Density: density,
			Jitter:  0.25,
			Mix:     []forest.Share{{Tree: "oak"}},
		}, tree{"oak", oak}), half)
		g.run(2)
		at := g.planted()["oak"]
		for _, p := range at {
			if p.X >= width/2 {
				t.Fatalf("density %v: an oak at %v, off the grove's cells", density, p)
			}
		}
		cells := float32(rows * cols / 2)
		if got := float32(len(at)) / cells; got < want*0.9 || got > want*1.1 {
			t.Errorf("density %v: %.3f trees a cell of the grove, want %v within 10%%", density,
				got, want)
		}
	}
}

func TestStands_TheTreesFoldInWithTheirSectors(t *testing.T) {
	g := newGrove(t, planting(forest.Stand{Mix: []forest.Share{{Tree: "oak"}}}, tree{"oak", oak}),
		nil)
	g.run(2)
	if n := total(g.planted()); n == 0 {
		t.Fatal("no tree planted at L3")
	}
	g.zoom(0.25)
	g.run(int(foldAfter/frame) + 4)
	if n := total(g.planted()); n != 0 {
		t.Errorf("%d trees stand after every sector folded at L4, want none", n)
	}
	g.zoom(0.5)
	g.run(2)
	if n := total(g.planted()); n == 0 {
		t.Error("no tree planted again back at L3")
	}
}

// a forest whole plants densely; one folding short of its trees is thinned, and plants sparsely,
// a share of the dense plots, from then on
func TestStands_WhenChoosesTheStandAndFelledTellsAForestShortOfTrees(t *testing.T) {
	dense := forest.Stand{Density: 1, Mix: []forest.Share{{Tree: "oak"}}}
	g := newGrove(t, woods{
		trees: []tree{{"oak", oak}},
		plays: []string{"whole", "thinned"},
		rules: func(p *forest.Plugin) {
			thinned := p.World().Write().Effects().Named(thinnedEf)
			sparse := dense
			sparse.Density, sparse.When = 0.5, cond.Self.If(thinned)
			whole := dense
			whole.When = cond.Self.If(cond.Not(thinned))
			p.Stands().Define("whole", whole, act.Branch[world.Fold]{
				Name: "thin",
				When: cond.Self.If(cond.Not(thinned)).And(cond.Self.If(p.Stands().Felled(dense))),
				Then: act.Self.Put(thinned),
			})
			p.Stands().Define("thinned", sparse)
		},
	}, nil)
	g.run(2)
	first := g.planted()["oak"]
	if got := float32(len(first)) / (rows * cols); got < 0.9 || got > 1.1 {
		t.Fatalf("%.3f trees a cell of the forest whole, want 1 within 10%%", got)
	}
	g.zoom(0.25)
	g.run(int(foldAfter/frame) + 4)
	g.zoom(0.5)
	g.run(2)
	fx := g.w.Write().Effects()
	thinned := fx.Named(thinnedEf)
	if again := g.planted()["oak"]; len(again) != len(first) || fx.Has(g.forest(), thinned) {
		t.Fatalf("folded whole and unfolded again: %d trees, thinned %v; want %d, not thinned",
			len(again), fx.Has(g.forest(), thinned), len(first))
	}

	g.fell()
	g.zoom(0.25)
	g.run(int(foldAfter/frame) + 4)
	if !fx.Has(g.forest(), thinned) {
		t.Fatal("the forest folded short of a tree is not thinned")
	}
	g.zoom(0.5)
	g.run(2)
	sparse := g.planted()["oak"]
	if got := float32(len(sparse)) / (rows * cols); got < 0.45 || got > 0.55 {
		t.Errorf("%.3f trees a cell of the forest thinned, want 0.5 within 10%%", got)
	}
	dense0 := map[geom.Vec]bool{}
	for _, p := range first {
		dense0[p] = true
	}
	for _, p := range sparse {
		if !dense0[p] {
			t.Fatalf("the forest thinned plants an oak at %v, where it stood no oak whole", p)
		}
	}
}

// stood is a look noting the sprites it stands true as bodies and those it draws as billboards
type stood struct {
	world.Look
	bodies, boards map[render.SpriteID]bool
	sways          map[render.SpriteID]float32
}

var _ world.UprightLook = stood{}

func (s stood) Sprite(_ *render.Frame, _ camera.Camera, _ plane.AABB, _ world.Z,
	_ render.AtlasSource, a render.Appearance, _ render.Illumination) {
	s.boards[a.SpriteID] = true
	if s.sways != nil {
		s.sways[a.SpriteID] = a.Sway
	}
}

func (s stood) Upright(_ *render.Frame, _ camera.Camera, _ plane.AABB, _ world.Z,
	_ render.AtlasSource, a render.Appearance, _ render.Illumination) {
	s.bodies[a.SpriteID] = true
}

// bare is the grove's ground laying no tile, the board drawing nothing of an atlas it has not
type bare struct{ ramp }

func (bare) Look() look.Look { return look.Nothing }

func TestTypes_ATreeIsABodyAtL2OnlyWhereItsSpecSaysBody(t *testing.T) {
	body := pine
	body.Body = true
	g := newGrove(t, planting(forest.Stand{Mix: []forest.Share{{Tree: "oak"}, {Tree: "pine"}}},
		tree{"oak", oak}, tree{"pine", body}), func(b *board.Plugin) {
		b.WithMap(bare{ramp{b.Read(0).Board().Grid}})
	})
	look := stood{
		Look:   g.w.Read(0).Look(),
		bodies: map[render.SpriteID]bool{},
		boards: map[render.SpriteID]bool{},
	}
	g.w.Write().SetLook(look)
	g.w.Write().NewView(g.cam)
	g.zoom(1)
	// planted at the second tick, the trees are in the view from the third
	g.run(3)
	if err := g.eng.Frame(); err != nil {
		t.Fatal(err)
	}
	g.w.Read(0).Picture().DrawWorld(nil, g.cam)
	_, oaks, _, _ := g.w.Kinds().Lookup("oak")
	_, pines, _, _ := g.w.Kinds().Lookup("pine")
	if !look.bodies[pines] || look.boards[pines] {
		t.Errorf("at L2 a pine with Body stood as a body %v, as a billboard %v; want a body",
			look.bodies[pines], look.boards[pines])
	}
	if look.bodies[oaks] || !look.boards[oaks] {
		t.Errorf("at L2 an oak stood as a body %v, as a billboard %v; want a billboard",
			look.bodies[oaks], look.boards[oaks])
	}
}

// inRelief is a camera seeing as cam does, in relief
type inRelief struct{ camera.Camera }

func (inRelief) Projection() camera.Projection { return standingUp{} }

type standingUp struct{ camera.TopDown }

func (standingUp) Sorts() bool { return true }

// bodyStood is a look noting the kinds it stands as their bodies, whole and as cards, and those
// it draws as sprites
type bodyStood struct {
	world.Look
	whole, cards map[render.SpriteID]*world.Bodied
	sprites      map[render.SpriteID]bool
}

var _ world.BodyLook = bodyStood{}

func (s bodyStood) Sprite(_ *render.Frame, _ camera.Camera, _ plane.AABB, _ world.Z,
	_ render.AtlasSource, a render.Appearance, _ render.Illumination) {
	s.sprites[a.SpriteID] = true
}

func (s bodyStood) Body(_ *render.Frame, _ camera.Camera, _ plane.AABB, _ world.Z,
	_ render.AtlasSource, b *world.Bodied, _ float32, whole bool, a render.Appearance,
	_ render.Illumination) {
	if whole {
		s.whole[a.SpriteID] = b
		return
	}
	s.cards[a.SpriteID] = b
}

// a trunk under a ball of leaves, within an oak's crown and its height
var oakParts = []world.Part{
	{Shape: world.Cone, To: [3]float32{0, 0, 24}, Wide: 3, Tip: 2, Paint: forest.Wood},
	{Shape: world.Spheroid, From: [3]float32{0, 0, 20}, To: [3]float32{0, 0, 40}, Wide: 20},
}

func TestTypes_ATreeWithPartsStandsAsItsBodyInReliefAndAsItsSpriteFromAbove(t *testing.T) {
	bodied := oak
	bodied.Parts = oakParts
	g := newGrove(t, planting(forest.Stand{Mix: []forest.Share{{Tree: "oak"}, {Tree: "pine"}}},
		tree{"oak", bodied}, tree{"pine", pine}), func(b *board.Plugin) {
		b.WithMap(bare{ramp{b.Read(0).Board().Grid}})
	})
	_, oaks, _, _ := g.w.Kinds().Lookup("oak")
	_, pines, _, _ := g.w.Kinds().Lookup("pine")
	relief := inRelief{g.cam}
	g.w.Write().NewView(g.cam)
	g.w.Write().NewView(relief)
	g.zoom(1)
	// seen is the trees as a look with the body's seam is handed them through cam
	seen := func(cam camera.Camera) bodyStood {
		look := bodyStood{
			Look:    g.w.Read(0).Look(),
			whole:   map[render.SpriteID]*world.Bodied{},
			cards:   map[render.SpriteID]*world.Bodied{},
			sprites: map[render.SpriteID]bool{},
		}
		was := g.w.Read(0).Look()
		g.w.Write().SetLook(look)
		g.run(3)
		if err := g.eng.Frame(); err != nil {
			t.Fatal(err)
		}
		g.w.Read(0).Picture().DrawWorld(nil, cam)
		g.w.Write().SetLook(was)
		return look
	}
	look := seen(relief)
	b := look.whole[oaks]
	if b == nil || b.Parts != 2 || b.Wide != 20 || b.Tall != 40 {
		t.Fatalf("in relief an oak with Parts stood whole as %+v, want its body of 2 parts, "+
			"20 wide and 40 tall", b)
	}
	if look.sprites[oaks] || !look.sprites[pines] || look.whole[pines] != nil {
		t.Errorf("in relief the oak was drawn as a sprite %v, the pine as one %v and whole "+
			"%v; want the body alone of the oak and the sprite of the pine", look.sprites[oaks],
			look.sprites[pines], look.whole[pines] != nil)
	}
	// the bodies drawn in to the close scale, an oak at the standard one is a card of its body
	g.w.Write().SetBodies(lod.L1)
	if look := seen(relief); look.cards[oaks] != b || look.whole[oaks] != nil {
		t.Errorf("with the bodies whole close up alone the oak stood as a card %v and whole "+
			"%v, want the card of its body", look.cards[oaks] != nil, look.whole[oaks] != nil)
	}
	if look := seen(g.cam); !look.sprites[b.Above] || look.sprites[oaks] ||
		len(look.whole)+len(look.cards) != 0 {
		t.Errorf("from above the oak was drawn seen from above %v, from the side %v and as a "+
			"body %d times, want it seen from above alone", look.sprites[b.Above],
			look.sprites[oaks], len(look.whole)+len(look.cards))
	}
}

func TestTypes_ATreeWhosePartsStandPastItsCrownPanicsNamingIt(t *testing.T) {
	defer func() {
		msg, _ := recover().(string)
		if !strings.Contains(msg, `"oak"`) {
			t.Errorf("a tree with a part past its crown was taken with %q, want a panic "+
				"naming it", msg)
		}
	}()
	wide := oak
	wide.Parts = []world.Part{{Shape: world.Spheroid, From: [3]float32{30, 0, 10},
		To: [3]float32{30, 0, 20}, Wide: 10}}
	newGrove(t, planting(forest.Stand{Mix: []forest.Share{{Tree: "oak"}}}, tree{"oak", wide}),
		func(*board.Plugin) {})
}

func TestTypes_ATreeSwaysAsMuchAsItsSpecSays(t *testing.T) {
	supple := pine
	supple.Sway = 0.7
	g := newGrove(t, planting(forest.Stand{Mix: []forest.Share{{Tree: "oak"}, {Tree: "pine"}}},
		tree{"oak", oak}, tree{"pine", supple}), func(b *board.Plugin) {
		b.WithMap(bare{ramp{b.Read(0).Board().Grid}})
	})
	look := stood{
		Look:   g.w.Read(0).Look(),
		bodies: map[render.SpriteID]bool{},
		boards: map[render.SpriteID]bool{},
		sways:  map[render.SpriteID]float32{},
	}
	g.w.Write().SetLook(look)
	g.w.Write().NewView(g.cam)
	g.zoom(1)
	g.run(3)
	if err := g.eng.Frame(); err != nil {
		t.Fatal(err)
	}
	g.w.Read(0).Picture().DrawWorld(nil, g.cam)
	_, oaks, _, _ := g.w.Kinds().Lookup("oak")
	_, pines, _, _ := g.w.Kinds().Lookup("pine")
	if !look.boards[pines] || !look.boards[oaks] {
		t.Fatalf("pines drawn %v, oaks %v; want both to check", look.boards[pines],
			look.boards[oaks])
	}
	if look.sways[pines] != 0.7 || look.sways[oaks] != 0 {
		t.Errorf("a pine of Sway 0.7 is drawn swaying %v and an oak of none %v; want 0.7 and 0",
			look.sways[pines], look.sways[oaks])
	}
}

func TestTypes_ASwayPastOnePanics(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Error("a tree swaying 1.5 was defined, want a panic")
		}
	}()
	wild := pine
	wild.Sway = 1.5
	newGrove(t, planting(forest.Stand{Mix: []forest.Share{{Tree: "pine"}}}, tree{"pine", wild}),
		func(*board.Plugin) {})
}

func (g *grove) forest() uid.UID64 {
	for g.woods.All(); g.woods.Next(); {
		return g.woods.Cursor().IDs[0]
	}
	return 0
}

// fell has one tree of the forest go
func (g *grove) fell() {
	g.t.Helper()
	for g.trees.All(); g.trees.Next(); {
		g.w.Write().Carrier().PutFrom(g.trees.Cursor().IDs[0], world.Despawn{})
		g.run(2)
		return
	}
	g.t.Fatal("no tree to fell")
}

func TestTypes_EntryIsATreeOfAKindDefinedByTheTimeTheLooksAre(t *testing.T) {
	asked := func(name string) (refused bool) {
		defer func() { refused = recover() != nil }()
		f := planting(forest.Stand{Mix: []forest.Share{{Tree: "oak"}}}, tree{"oak", oak})
		stands := f.rules
		f.rules = func(p *forest.Plugin) {
			stands(p)
			if got := p.Types().Entry(name).Kind(); got != name {
				t.Errorf("the entry is of %q, want %q", got, name)
			}
		}
		newGrove(t, f, nil)
		return false
	}
	if asked("oak") {
		t.Error("an entry of a tree defined is refused")
	}
	if !asked("elm") {
		t.Error("an entry of a tree never defined is given")
	}
}

// giving is a look noting whether each sprite it laid was told to give way
type giving struct {
	world.Look
	on   *bool
	gave map[render.SpriteID]bool
}

var _ world.UncoveringLook = giving{}

func (giving) Uncover(bool) {}

func (g giving) GiveWay(on bool) { *g.on = on }

func (giving) Uncovers() bool { return false }

func (giving) GivesWay() bool { return false }

func (g giving) Sprite(_ *render.Frame, _ camera.Camera, _ plane.AABB, _ world.Z,
	_ render.AtlasSource, a render.Appearance, _ render.Illumination) {
	g.gave[a.SpriteID] = *g.on
}

func TestTypes_ATreeGivesWayToWhatIsUncovered(t *testing.T) {
	f := planting(forest.Stand{Mix: []forest.Share{{Tree: "oak"}}}, tree{"oak", oak})
	f.walked = true
	g := newGrove(t, f, func(b *board.Plugin) { b.WithMap(bare{ramp{b.Read(0).Board().Grid}}) })
	look := giving{Look: g.w.Read(0).Look(), on: new(bool), gave: map[render.SpriteID]bool{}}
	g.w.Write().SetLook(look)
	g.w.Write().NewView(g.cam)
	g.zoom(1)
	g.run(3)
	if err := g.eng.Frame(); err != nil {
		t.Fatal(err)
	}
	g.w.Read(0).Picture().DrawWorld(nil, g.cam)
	_, oaks, _, _ := g.w.Kinds().Lookup("oak")
	if gave, laid := look.gave[oaks]; !laid || !gave {
		t.Errorf("an oak was laid %v, told to give way %v; want it giving way to what the "+
			"game's looks uncover", laid, gave)
	}
}
