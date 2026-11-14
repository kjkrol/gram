package buildings_test

import (
	"reflect"
	"testing"
	"time"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/aabbworld/plane"
	"github.com/kjkrol/ceremony"
	"github.com/kjkrol/ceremony/act"
	"github.com/kjkrol/ceremony/comp"
	"github.com/kjkrol/ceremony/cond"
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
	"github.com/kjkrol/gram/plugins/buildings"
	"github.com/kjkrol/gram/plugins/cameras/cameratest"
	"github.com/kjkrol/gram/plugins/motion"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/plugins/world/sector"
)

const (
	cellSize = 32
	// 4 by 2 sectors of 16 cells
	cols, rows = 64, 32
	width      = cols * cellSize
	height     = rows * cellSize
	frame      = time.Second / 60
	foldAfter  = 500 * time.Millisecond
	townKind   = "town"
	townRole   = "town"
	houseKind  = "house"
)

type oneStageGame struct{ stage game.Stage }

func (oneStageGame) Props() game.Props { return game.Props{} }

func (g oneStageGame) Stages() (map[string]game.Stage, string) {
	return map[string]game.Stage{g.stage.Name(): g.stage}, g.stage.Name()
}

type townRow struct{}

// houses is a town over the whole world whose buildings stand as its sectors unfold
type houses struct{}

var (
	_ buildings.Figure = houses{}
	_ buildings.Ruled  = houses{}
)

func (houses) Rules(p *buildings.Plugin) {
	p.World().Roles().Define(townRole, act.Branch[world.Unfold]{
		Name: "its buildings stand",
		When: cond.Self.If(cond.Always),
		Then: act.Self.Do(buildings.Raise{}),
	})
}

func (houses) Define(p *buildings.Plugin) {
	p.Types().Define(houseKind, buildings.Spec{
		Storey: 10,
		Floors: 2,
		Parts:  []buildings.Part{{Roof: buildings.Gable, Pitch: 1}},
	})
	w := p.World()
	kind.Define[townRow](w.Kinds(), townKind, kind.Spec{
		comp.Const(world.Position{
			AABB: plane.NewAABB(geom.NewVec(width/2-8, height/2-8), 16, 16),
		}),
		comp.Const(world.Velocity{}),
		comp.Const(world.Aggregate{}),
		comp.Const(world.Field{Size: geom.NewVec(width, height)}),
		ceremony.Plays(w.Roles().Named(townRole)),
	})
}

// town is a world of 4 by 2 sectors, a town over all of it and a camera seeing every sector
type town struct {
	ecs *goke.ECS
	w   *world.Plugin
	p   *buildings.Plugin
	cam camera.Camera

	stood *goke.Query
	base  goke.Comp[world.Base]
	z     goke.Comp[world.Z]
	stand goke.Comp[world.Stand]
	of    goke.Comp[world.PartOf]
}

var _ sector.Observers = (*town)(nil)

func (g *town) Observers(each func(cam camera.Camera)) { each(g.cam) }

func (g *town) SetupSystems() []goke.System {
	return []goke.System{goke.SystemFn{OnInit: func(si *goke.SysInit) {
		g.stood = si.NewQueryBuilder(&g.base, &g.z, &g.stand, &g.of).Build()
	}}}
}

func newTown(t *testing.T, stood ...buildings.Row) *town {
	t.Helper()
	g := &town{}
	g.cam = cameratest.Showing(cameratest.NewFromSpace(width, height, 0))
	g.cam.SetViewport(100, 100)
	g.zoom(0.5)
	var update func(goke.RunCtx, time.Duration)
	st := stage.New("town").
		World(world.Config{
			Space:    world.SpaceCfg{Width: width, Height: height},
			Entities: world.EntitiesCfg{MaxCount: 1024, MinSize: 1, MaxSize: 64},
			Heights:  true,
			Sectors:  world.SectorsCfg{Buckets: 4},
			LOD: lod.Ladder{
				Scales: [lod.Levels - 1]float32{0, 0, 0.8, 0.4},
				Fold:   float32(foldAfter.Seconds()),
			},
		}).
		Plugins(func(w *world.Plugin) {
			g.w = w
			m := motion.NewPlugin(w)
			gr := grid.DefaultGrids{}.Square(cols, rows, cellSize)
			b := board.NewPlugin(gr, &cell.MultipleOccupancy{}, w).
				WithMotion(m, cell.DefaultHiking)
			b.Write().Board().SetAll(cell.Kind{Pace: 1, Allows: cell.Land})
			g.p = buildings.NewPlugin(w, b)
			w.Write().Observe(g)
			update = func(rc goke.RunCtx, d time.Duration) {
				w.RunPlan(rc, d)
				g.p.RunPlan(rc, d)
				m.RunPlan(rc, d)
				b.RunPlan(rc, d)
				rc.Sync()
			}
		}).
		Figures(func() { g.p.Figures(houses{}) }).
		Effects(func(ctx game.Initializer) {
			g.ecs = ctx.ECS()
			ctx.Setup(g)
		}).
		Spawn(func() {
			g.w.Write().Seed(kind.Named[townRow](g.w.Kinds(), townKind).Entry(townRow{}))
			for _, r := range stood {
				g.p.Write().Stand(houseKind, r)
			}
		}).
		Update(func(rc goke.RunCtx, d time.Duration) { update(rc, d) })
	eng := engine.NewEngine(oneStageGame{stage: st})
	if err := eng.Init(); err != nil {
		t.Fatalf("Init: %v", err)
	}
	return g
}

func (g *town) zoom(zoom float32) {
	g.cam.ZoomIn(zoom/g.cam.Zoom(), 0, 0)
	g.cam.CenterOn(width/2, height/2, 0)
}

func (g *town) run(d time.Duration) {
	for range int(d / frame) {
		g.ecs.Tick(frame)
	}
}

type standing struct {
	box   plane.AABB
	z     world.Z
	stand world.Stand
}

func (g *town) standing() []standing {
	var out []standing
	for g.stood.All(); g.stood.Next(); {
		cur := g.stood.Cursor()
		zs, stands := g.z.Slice(cur), g.stand.Slice(cur)
		for i, b := range g.base.Slice(cur) {
			out = append(out, standing{box: b.Pos.AABB, z: zs[i], stand: stands[i]})
		}
	}
	return out
}

func TestRaise_StandsTheBuildingsOfASectorAsItUnfoldsAndTheyGoAsItFolds(t *testing.T) {
	g := newTown(t,
		buildings.Row{Mid: [2]float32{300, 200}, Size: [2]float32{40, 20}, Turn: 90, Floors: 3},
		buildings.Row{Mid: [2]float32{1500, 800}, Size: [2]float32{30, 30}},
	)
	g.run(200 * time.Millisecond)
	got := g.standing()
	if len(got) != 2 {
		t.Fatalf("%d buildings stand in a town seen near, want its two", len(got))
	}
	for _, b := range got {
		mid := world.Position{AABB: b.box}.Center()
		switch {
		case f32.Abs(mid.X-300) < 1 && f32.Abs(mid.Y-200) < 1:
			// turned a quarter its box is 20 by 40; three storeys of 10 and half its roof of 10
			if f32.Abs(b.box.Size.X-20) > 1e-3 || f32.Abs(b.box.Size.Y-40) > 1e-3 ||
				b.z.Height != 35 || b.stand != (world.Stand{Length: 40, Width: 20}) {
				t.Errorf("the house turned stands %v, %v high over %v", b.box, b.z.Height, b.stand)
			}
		case f32.Abs(mid.X-1500) < 1 && f32.Abs(mid.Y-800) < 1:
			if b.z.Height != 27.5 {
				t.Errorf("a house of no word stands %v high, want the kind's two storeys and "+
					"half its roof", b.z.Height)
			}
		default:
			t.Errorf("a building stands at %v, where the town has none", mid)
		}
	}
	g.zoom(0.05)
	g.run(foldAfter + time.Second)
	if n := len(g.standing()); n != 0 {
		t.Errorf("%d buildings stand in a town seen from afar, want them folded", n)
	}
}

func TestResources_TheBuildingsStandingComeBackWithALoad(t *testing.T) {
	g := newTown(t,
		buildings.Row{Mid: [2]float32{300, 200}, Size: [2]float32{40, 20}, Turn: 90, Floors: 3},
		buildings.Row{Mid: [2]float32{1500, 800}, Size: [2]float32{30, 30}},
	)
	stood := g.p.Res.Stood
	g.p.Serializable().Persisted()
	g.p.Res = buildings.Resources{}
	g.p.Restore()
	if !reflect.DeepEqual(g.p.Res.Stood, stood) || len(stood) == 0 {
		t.Errorf("loaded %v, saved %v", g.p.Res.Stood, stood)
	}
}
