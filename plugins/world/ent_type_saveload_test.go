package world_test

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/aabbworld/plane"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/entity/kind"
	"github.com/kjkrol/gram/entity/kind/comp"
	"github.com/kjkrol/gram/game"
	"github.com/kjkrol/gram/internal/engine"
	"github.com/kjkrol/gram/plugins/world"
)

// typeStage defines its kinds in whatever order it is given, so two runs can
// disagree about which TypeID means what.
type typeStage struct {
	order    []string
	spawn    string
	loadFrom string

	world *world.Plugin
	kinds map[string]kind.Of[struct{}]
	probe *typeProbe
	stack game.Scenes
}

// typeProbe builds its query during the engine's one Setup call, so the test
// can read components back once Init (and any PostLoad) has finished.
type typeProbe struct {
	base  goke.Comp[world.Base]
	query *goke.Query
}

func (p *typeProbe) SetupSystems() []goke.System {
	return []goke.System{goke.SystemFn{OnInit: func(si *goke.SysInit) {
		p.query = si.NewQueryBuilder(&p.base).Build()
	}}}
}

func (g *typeStage) Name() string { return "stage" }

func (g *typeStage) Init(ctx game.Initializer) error {
	g.world = ctx.UseWorld(testWorldConfig())
	g.probe = &typeProbe{}
	ctx.Setup(g.probe)
	g.kinds = map[string]kind.Of[struct{}]{}
	for _, name := range g.order {
		kind.Define[struct{}](g.world.Kinds(), name, kind.Spec{
			comp.Const(world.Position{AABB: plane.NewAABB(geom.NewVec(100, 100), 10, 10)}),
			comp.Const(world.Velocity{}),
		})
		g.kinds[name] = kind.Named[struct{}](g.world.Kinds(), name)
	}
	return nil
}

func (g *typeStage) Restore(p game.Persistence) (bool, error) {
	if g.loadFrom == "" {
		return false, nil
	}
	if err := p.Load(g.loadFrom, ""); err != nil {
		return false, err
	}
	return true, nil
}

func (g *typeStage) Spawn() error {
	if g.spawn != "" {
		g.world.Seed(g.kinds[g.spawn].Entry(struct{}{}))
	}
	return nil
}

func (g *typeStage) Update(goke.RunCtx, time.Duration) {}

func (g *typeStage) Stack() game.Scenes {
	if g.stack == nil {
		g.stack, _ = game.NewStack()
	}
	return g.stack
}

// onlyType reads the Type of the single entity the stage holds.
func onlyType(t *testing.T, g *typeStage) kind.ID {
	t.Helper()
	var got kind.ID
	seen := 0
	g.probe.query.All()
	for g.probe.query.Next() {
		cursor := g.probe.query.Cursor()
		bases := g.probe.base.Slice(cursor)
		for i := range cursor.IDs {
			got = bases[i].TypeID
			seen++
		}
	}
	if seen != 1 {
		t.Fatalf("found %d entities carrying Type, want exactly 1", seen)
	}
	return got
}

func runStage(t *testing.T, g *typeStage) {
	t.Helper()
	eng := engine.NewEngine(oneStageGame{stage: g, props: game.Props{}})
	if err := eng.Init(); err != nil {
		t.Fatalf("Init: %v", err)
	}
	if g.loadFrom == "" {
		if err := eng.Persistence().Save(t.TempDir()+"/unused", ""); err != nil {
			t.Fatalf("Save: %v", err)
		}
	}
}

func TestType_IsAssignedByDefineOrder(t *testing.T) {
	g := &typeStage{order: []string{"rock", "wolf"}, spawn: "wolf"}
	runStage(t, g)

	if got := onlyType(t, g); got != 1 {
		t.Errorf("wolf has TypeID %d, want 1 — second Define call", got)
	}
	for name, want := range map[string]kind.ID{"rock": 0, "wolf": 1} {
		if got := g.kinds[name].ID(); got != want {
			t.Errorf("%q has ID %d, want %d", name, got, want)
		}
	}
}

func TestType_SurvivesAReorderedDictionary(t *testing.T) {
	path := t.TempDir() + "/save"

	saver := &typeStage{order: []string{"rock", "wolf"}, spawn: "wolf"}
	eng := engine.NewEngine(oneStageGame{stage: saver, props: game.Props{}})
	if err := eng.Init(); err != nil {
		t.Fatalf("Init: %v", err)
	}
	if got := onlyType(t, saver); got != 1 {
		t.Fatalf("before saving, wolf has TypeID %d, want 1", got)
	}
	if err := eng.Persistence().Save(path, ""); err != nil {
		t.Fatalf("Save: %v", err)
	}

	loader := &typeStage{order: []string{"wolf", "rock"}, loadFrom: path}
	eng2 := engine.NewEngine(oneStageGame{stage: loader, props: game.Props{}})
	if err := eng2.Init(); err != nil {
		t.Fatalf("Init after reorder: %v", err)
	}

	wolf := loader.kinds["wolf"]
	if wolf.ID() != 0 {
		t.Fatalf("wolf has ID %d in this build, want 0 — the fixture is not testing a reorder", wolf.ID())
	}
	if got := onlyType(t, loader); got != wolf.ID() {
		t.Errorf("loaded entity carries TypeID %d, want %d — the id wolf holds in this build", got, wolf.ID())
	}
}

func TestType_PanicsWhenTheSaveNamesAKindThisBuildDropped(t *testing.T) {
	path := t.TempDir() + "/save"

	saver := &typeStage{order: []string{"rock", "wolf"}, spawn: "wolf"}
	eng := engine.NewEngine(oneStageGame{stage: saver, props: game.Props{}})
	if err := eng.Init(); err != nil {
		t.Fatalf("Init: %v", err)
	}
	if err := eng.Persistence().Save(path, ""); err != nil {
		t.Fatalf("Save: %v", err)
	}

	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("loading a save that names a dropped kind did not panic")
		}
		if msg, _ := r.(string); !strings.Contains(msg, `"wolf"`) {
			t.Errorf("panic = %v, want it to name the missing kind", r)
		}
	}()

	loader := &typeStage{order: []string{"rock", "fox"}, loadFrom: path}
	engine.NewEngine(oneStageGame{stage: loader, props: game.Props{}}).Init()
}

func TestType_DictionaryRefusesMoreKindsThanTypeIDCanName(t *testing.T) {
	names := make([]string, 0, kind.MaxKinds+1)
	for i := range kind.MaxKinds + 1 {
		names = append(names, fmt.Sprintf("kind-%d", i))
	}

	defer func() {
		if r := recover(); r == nil {
			t.Fatalf("defining %d kinds did not panic", kind.MaxKinds+1)
		}
	}()

	g := &typeStage{order: names}
	engine.NewEngine(oneStageGame{stage: g, props: game.Props{}}).Init()
}
