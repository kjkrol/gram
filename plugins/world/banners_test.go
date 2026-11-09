package world_test

import (
	"image/color"
	"testing"
	"time"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/aabbworld/plane"
	"github.com/kjkrol/ceremony/command"
	"github.com/kjkrol/ceremony/comp"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/entity/kind"
	"github.com/kjkrol/gram/game"
	"github.com/kjkrol/gram/game/stage"
	"github.com/kjkrol/gram/internal/engine"
	"github.com/kjkrol/gram/lod"
	"github.com/kjkrol/gram/plugins/cameras/cameratest"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/plugins/world/sector"
	"github.com/kjkrol/gram/render"
)

const flagKind = "flag"

type flagRow struct{ x float32 }

// flags are loose units red, green from L4 on
type flags struct{}

var _ world.Figure = flags{}

func (flags) Define(w *world.Plugin) {
	kind.Define[flagRow](w.Kinds(), flagKind, kind.Spec{
		comp.Load(func(r flagRow) world.Position {
			return world.Position{AABB: plane.NewAABB(geom.NewVec(r.x, 250), 10, 10)}
		}),
		comp.Const(world.Velocity{}),
	})
}

func (flags) Look(a *world.Atlas, w *world.Plugin) {
	a.Add(kind.Named[flagRow](w.Kinds(), flagKind), 8, render.Solid(color.RGBA{R: 255, A: 255})).
		At(lod.L4, render.Solid(color.RGBA{G: 255, A: 255}))
}

// flagged is a world 8x2 sectors 512 a side of ten flags in the first, which its one camera sees
// from above at L4, a quarter of a pixel a world unit
type flagged struct {
	w   *world.Plugin
	ecs *goke.ECS
	cam camera.Camera
}

var _ sector.Observers = (*flagged)(nil)

func (f *flagged) Observers(each func(cam camera.Camera)) { each(f.cam) }

func newFlagged(t *testing.T, far world.FarLook) *flagged {
	t.Helper()
	f := &flagged{cam: cameratest.NewFromSpace(4096, 1024, 0)}
	f.cam.SetViewport(100, 100)
	f.cam.ZoomOut(4, 0, 0)
	f.cam.CenterOn(256, 256, 0)
	st := stage.New("flags").
		World(world.Config{
			Space:    world.SpaceCfg{Width: 4096, Height: 1024},
			Entities: world.EntitiesCfg{MaxCount: 64, MinSize: 5, MaxSize: 20},
			Sectors:  world.SectorsCfg{Buckets: 2},
			LOD:      lod.Ladder{Scales: [lod.Levels - 1]float32{0, 0, 0.8, 0.4}},
			Far:      far,
		}).
		Plugins(func(w *world.Plugin) {
			f.w = w
			w.Figures(flags{})
			w.Write().Observe(f)
		}).
		Effects(func(ctx game.Initializer) { f.ecs = ctx.ECS() }).
		Spawn(func() {
			flag := kind.Named[flagRow](f.w.Kinds(), flagKind)
			for i := range 10 {
				f.w.Write().Seed(flag.Entry(flagRow{x: 200 + float32(i)*12}))
			}
		}).
		Update(func(ctx goke.RunCtx, dt time.Duration) { f.w.RunPlan(ctx, dt) })
	if err := engine.NewEngine(oneStageGame{stage: st}).Init(); err != nil {
		t.Fatal(err)
	}
	f.w.Write().NewView(f.cam)
	f.ecs.Tick(frame)
	return f
}

func TestBanners_AreAKindsLooseUnitsDeclaringALookAtL4ASectorFarOff(t *testing.T) {
	f := newFlagged(t, world.Banners)
	if got := f.w.Published(); len(got) != 0 {
		t.Errorf("recorded %d flags drawn as a banner, want none", len(got))
	}
	b := f.w.Read(0).Banners(f.cam, nil)
	if len(b) != 1 || b[0].Count != 10 || len(b[0].IDs) != 10 || b[0].Sector != 0 ||
		b[0].Center != geom.NewVec(259, 255) {
		t.Fatalf("banners %+v, want one of the ten flags in sector 0 at (259, 255)", b)
	}
	f.w.Write().Carrier().Put(command.Nobody, world.ShowFar{Look: world.Points})
	f.ecs.Tick(frame)
	if got := f.w.Published(); len(got) != 10 || len(f.w.Read(0).Banners(f.cam, nil)) != 0 {
		t.Errorf("recorded %d flags as points, %d banners; want ten, none", len(got),
			len(f.w.Read(0).Banners(f.cam, nil)))
	}
}

type crowdRow struct{ x, y float32 }

// crowds are loose units of two kinds, red, green or blue from L4 on
type crowds struct{}

var _ world.Figure = crowds{}

func (crowds) Define(w *world.Plugin) {
	for _, name := range []string{flagKind, otherKind} {
		kind.Define[crowdRow](w.Kinds(), name, kind.Spec{
			comp.Load(func(r crowdRow) world.Position {
				return world.Position{AABB: plane.NewAABB(geom.NewVec(r.x, r.y), 10, 10)}
			}),
			comp.Const(world.Velocity{}),
		})
	}
}

func (crowds) Look(a *world.Atlas, w *world.Plugin) {
	red := render.Solid(color.RGBA{R: 255, A: 255})
	a.Add(kind.Named[crowdRow](w.Kinds(), flagKind), 8, red).
		At(lod.L4, render.Solid(color.RGBA{G: 255, A: 255}))
	a.Add(kind.Named[crowdRow](w.Kinds(), otherKind), 8, red).
		At(lod.L4, render.Solid(color.RGBA{B: 255, A: 255}))
}

const otherKind = "other"

// newCrowded is the world of newFlagged, its camera at (512, 512) over the corner of sectors 0,
// 1, 8 and 9, with 30 units of each kind given in six files 12 apart from (x, y)
func newCrowded(t *testing.T, x, y float32, kinds ...string) *flagged {
	t.Helper()
	f := &flagged{cam: cameratest.NewFromSpace(4096, 1024, 0)}
	f.cam.SetViewport(100, 100)
	f.cam.ZoomOut(4, 0, 0)
	f.cam.CenterOn(512, 512, 0)
	st := stage.New("crowds").
		World(world.Config{
			Space:    world.SpaceCfg{Width: 4096, Height: 1024},
			Entities: world.EntitiesCfg{MaxCount: 64, MinSize: 5, MaxSize: 20},
			Sectors:  world.SectorsCfg{Buckets: 2},
			LOD:      lod.Ladder{Scales: [lod.Levels - 1]float32{0, 0, 0.8, 0.4}},
			Far:      world.Banners,
		}).
		Plugins(func(w *world.Plugin) {
			f.w = w
			w.Figures(crowds{})
			w.Write().Observe(f)
		}).
		Effects(func(ctx game.Initializer) { f.ecs = ctx.ECS() }).
		Spawn(func() {
			for _, name := range kinds {
				k := kind.Named[crowdRow](f.w.Kinds(), name)
				for i := range 30 {
					f.w.Write().Seed(k.Entry(crowdRow{x: x + float32(i%6)*12,
						y: y + float32(i/6)*12}))
				}
			}
		}).
		Update(func(ctx goke.RunCtx, dt time.Duration) { f.w.RunPlan(ctx, dt) })
	if err := engine.NewEngine(oneStageGame{stage: st}).Init(); err != nil {
		t.Fatal(err)
	}
	f.w.Write().NewView(f.cam)
	f.ecs.Tick(frame)
	f.w.Published()
	return f
}

func TestBanners_AreOneOfACrowdStandingAcrossTheSeamsOfSectors(t *testing.T) {
	f := newCrowded(t, 482, 488, flagKind)
	b := f.w.Read(0).Banners(f.cam, nil)
	if len(b) != 1 || b[0].Count != 30 || len(b[0].IDs) != 30 {
		t.Fatalf("banners %+v of 30 flags in four sectors, want one of the 30", b)
	}
	if want := geom.NewVec(482+2.5*12+5, 488+2*12+5); b[0].Center != want {
		t.Errorf("the banner at %v, want at the 30's centroid %v", b[0].Center, want)
	}
	two := newCrowded(t, 482, 488, flagKind, otherKind)
	if b := two.w.Read(0).Banners(two.cam, nil); len(b) != 2 || b[0].Count != 30 ||
		b[1].Count != 30 || b[0].Kind == b[1].Kind {
		t.Errorf("banners %+v of 30 of each of two kinds in four sectors, want one of each", b)
	}
}

func TestBanners_DrawnAsTheConfigSaysWhereTheSettingsDoNot(t *testing.T) {
	if got := newFlagged(t, world.AsConfig).w.Published(); len(got) != 10 {
		t.Errorf("recorded %d flags with nothing said, want ten points", len(got))
	}
}

func TestFar_SettingsAreSavedWithTheGameOverTheConfig(t *testing.T) {
	cfg := testWorldConfig()
	cfg.Far, cfg.Counts = world.Banners, true
	// the settings w saved, loaded into a world of cfg
	loaded := func(w *world.Plugin) *world.Read {
		blob := *w.Serializable().Persisted()[0].(*[]byte)
		l := world.NewPlugin(cfg)
		*l.Serializable().Persisted()[0].(*[]byte) = blob
		l.Restore()
		return l.Read(0)
	}
	saved := world.NewPlugin(testWorldConfig())
	saved.Write().ShowFar(world.Points)
	saved.Write().ShowCounts(false)
	if r := loaded(saved); r.FarLook() != world.Points || r.Counts() {
		t.Errorf("loaded far %v, counts %v; want points, none, as saved", r.FarLook(), r.Counts())
	}
	if r := loaded(world.NewPlugin(testWorldConfig())); r.FarLook() != world.Banners ||
		!r.Counts() {
		t.Errorf("loaded far %v, counts %v of a save saying neither; want the Config's banners "+
			"and counts", r.FarLook(), r.Counts())
	}
}
