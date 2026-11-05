package world_test

import (
	"testing"
	"time"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/aabbworld/plane"
	"github.com/kjkrol/ceremony/comp"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/entity/kind"
	"github.com/kjkrol/gram/game"
	"github.com/kjkrol/gram/internal/engine"
	"github.com/kjkrol/gram/plugins/world"
)

// fixedStage is a world of a lantern, Fixed, and a walker, not, from a Config of its far look
// and counts, its own settings changed as change says before a save
type fixedStage struct {
	far    world.FarLook
	counts bool
	change func(w *world.Write)

	world *world.Plugin
	base  goke.Comp[world.Base]
	fixed goke.OptComp[world.Fixed]
	query *goke.Query
	stack game.Scenes
}

func (g *fixedStage) SetupSystems() []goke.System {
	return []goke.System{goke.SystemFn{OnInit: func(si *goke.SysInit) {
		g.query = si.NewQueryBuilder(&g.base).Optional(&g.fixed).Build()
	}}}
}

func (g *fixedStage) Name() string { return "stage" }

func (g *fixedStage) Init(ctx game.Initializer) error {
	cfg := testWorldConfig()
	cfg.Entities.MaxCount = 2
	cfg.Far, cfg.Counts = g.far, g.counts
	g.world = ctx.UseWorld(cfg)
	ctx.Setup(g)
	at := comp.Const(world.Position{AABB: plane.NewAABB(geom.NewVec(100, 100), 10, 10)})
	kind.Define[struct{}](g.world.Kinds(), "lantern", kind.Spec{at, comp.Const(world.Velocity{}),
		comp.Const(world.Fixed{})})
	kind.Define[struct{}](g.world.Kinds(), "walker", kind.Spec{at, comp.Const(world.Velocity{})})
	return nil
}

func (g *fixedStage) Loaded() error { return nil }

func (g *fixedStage) Spawn() error {
	for _, name := range []string{"lantern", "walker"} {
		g.world.Write().Seed(kind.Named[struct{}](g.world.Kinds(), name).Entry(struct{}{}))
	}
	return nil
}

func (g *fixedStage) Update(goke.RunCtx, time.Duration) {}

func (g *fixedStage) Stack() game.Scenes {
	if g.stack == nil {
		g.stack, _ = game.NewStack()
	}
	return g.stack
}

// fixedKinds is the names of the kinds whose entities carry Fixed
func (g *fixedStage) fixedKinds(t *testing.T) []string {
	t.Helper()
	var names []string
	for g.query.All(); g.query.Next(); {
		cur := g.query.Cursor()
		if !g.fixed.Present(cur) {
			continue
		}
		for _, b := range g.base.Slice(cur) {
			for _, name := range []string{"lantern", "walker"} {
				if id, _, _, _ := g.world.Kinds().Lookup(name); id == b.TypeID {
					names = append(names, name)
				}
			}
		}
	}
	return names
}

// savedAndLoaded saves saver after its change and loads the save into loader
func savedAndLoaded(t *testing.T, saver, loader *fixedStage) {
	t.Helper()
	t.Chdir(t.TempDir())
	eng := engine.NewEngine(oneStageGame{stage: saver})
	if err := eng.Init(); err != nil {
		t.Fatalf("Init: %v", err)
	}
	if saver.change != nil {
		saver.change(saver.world.Write())
	}
	if err := eng.Save(""); err != nil {
		t.Fatalf("Save: %v", err)
	}
	eng2 := engine.NewEngine(oneStageGame{stage: loader})
	if err := eng2.Load("", ""); err != nil {
		t.Fatalf("Load: %v", err)
	}
	if err := eng2.Init(); err != nil {
		t.Fatalf("Init after the load: %v", err)
	}
}

func TestFixed_IsSavedOnTheEntitiesOfItsKindAlone(t *testing.T) {
	saver, loader := &fixedStage{}, &fixedStage{}
	savedAndLoaded(t, saver, loader)
	for _, g := range []*fixedStage{saver, loader} {
		if got := g.fixedKinds(t); len(got) != 1 || got[0] != "lantern" {
			t.Errorf("Fixed carried by %v, want the lantern alone", got)
		}
	}
}

func TestSettings_KeepTheFarLookAndTheCountsThroughASave(t *testing.T) {
	saver := &fixedStage{change: func(w *world.Write) {
		w.ShowFar(world.Banners)
		w.ShowCounts(false)
	}}
	loader := &fixedStage{far: world.Points, counts: true}
	savedAndLoaded(t, saver, loader)
	if far, counts := loader.world.Read(0).FarLook(), loader.world.Read(0).Counts(); far !=
		world.Banners || counts {
		t.Errorf("loaded far %v, counts %v; want those saved, banners and none", far, counts)
	}
}

func TestSettings_SavedAsConfigLoadAsTheConfigSays(t *testing.T) {
	loader := &fixedStage{far: world.Banners, counts: true}
	savedAndLoaded(t, &fixedStage{}, loader)
	if far, counts := loader.world.Read(0).FarLook(), loader.world.Read(0).Counts(); far !=
		world.Banners || !counts {
		t.Errorf("loaded far %v, counts %v over a save leaving them; want the Config's, "+
			"banners and counts", far, counts)
	}
}
