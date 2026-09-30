package world_test

import (
	"testing"
	"time"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/aabbworld/plane"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/game"
	"github.com/kjkrol/gram/internal/engine"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/plugins/world/entity/kind"
	"github.com/kjkrol/gram/plugins/world/entity/kind/comp"
	"github.com/kjkrol/gram/plugins/world/entity/tag"
)

// moods is a tag family of the test's own.
type moods struct{}

// tagStage defines the tags of one family in whatever order it is given and spawns one entity
// carrying the named ones, so two runs can disagree about which bit means what.
type tagStage struct {
	order    []string
	carries  []string
	loadFrom string

	world *world.Plugin
	unit  kind.Of[struct{}]
	tags  map[string]tag.Tag[moods]
	probe *tagProbe
	stack game.Scenes
}

type tagProbe struct {
	marks goke.Comp[tag.Tags[moods]]
	query *goke.Query
}

func (p *tagProbe) SetupSystems() []goke.System {
	return []goke.System{goke.SystemFn{OnInit: func(si *goke.SysInit) {
		p.query = si.NewQueryBuilder(&p.marks).Build()
	}}}
}

func (g *tagStage) Name() string { return "stage" }

func (g *tagStage) Init(ctx game.Initializer) error {
	g.world = ctx.UseWorld(testWorldConfig())
	g.probe = &tagProbe{}
	ctx.Setup(g.probe)
	g.tags = map[string]tag.Tag[moods]{}
	for _, name := range g.order {
		g.tags[name] = g.world.Kinds().DefineTag[moods](name)
	}
	var tags []tag.Tag[moods]
	for _, name := range g.carries {
		tags = append(tags, g.tags[name])
	}
	g.unit = kind.Define[struct{}](g.world.Kinds(), "unit", kind.Spec{
		comp.Const(world.Position{AABB: plane.NewAABB(geom.NewVec(100, 100), 10, 10)}),
		comp.Const(world.Velocity{}),
		comp.Tagged(tags...),
	})
	return nil
}

func (g *tagStage) Restore(p game.Persistence) (bool, error) {
	if g.loadFrom == "" {
		return false, nil
	}
	return true, p.Load(g.loadFrom, "")
}

func (g *tagStage) Spawn() error {
	g.world.Seed(g.unit.Entry(struct{}{}))
	return nil
}

func (g *tagStage) Update(goke.RunCtx, time.Duration) {}

func (g *tagStage) Stack() game.Scenes {
	if g.stack == nil {
		g.stack, _ = game.NewStack()
	}
	return g.stack
}

// onlyMarks reads the marks of the single entity the stage holds.
func onlyMarks(t *testing.T, g *tagStage) tag.Tags[moods] {
	t.Helper()
	var got tag.Tags[moods]
	seen := 0
	g.probe.query.All()
	for g.probe.query.Next() {
		for _, m := range g.probe.marks.Slice(g.probe.query.Cursor()) {
			got = m
			seen++
		}
	}
	if seen != 1 {
		t.Fatalf("found %d entities carrying the family, want exactly 1", seen)
	}
	return got
}

func runTagStage(t *testing.T, g *tagStage) *engine.Engine {
	t.Helper()
	eng := engine.NewEngine(oneStageGame{stage: g, props: game.Props{}})
	if err := eng.Init(); err != nil {
		t.Fatalf("Init: %v", err)
	}
	return eng
}

func TestTags_AreNumberedByDefineOrderWithinTheFamily(t *testing.T) {
	g := &tagStage{order: []string{"calm", "angry"}, carries: []string{"angry"}}
	runTagStage(t, g)

	if g.tags["calm"] != 0 || g.tags["angry"] != 1 {
		t.Errorf("tags = %v, want calm 0 and angry 1", g.tags)
	}
	if got := onlyMarks(t, g); !got.Has(g.tags["angry"]) || got.Has(g.tags["calm"]) {
		t.Errorf("marks %b, want angry alone", got)
	}
	defer func() {
		if recover() == nil {
			t.Error("defining a tag twice returned quietly, want a panic")
		}
	}()
	g.world.Kinds().DefineTag[moods]("angry")
}

func TestTags_SurviveAReorderedDefinition(t *testing.T) {
	path := t.TempDir() + "/save"

	saver := &tagStage{order: []string{"calm", "angry"}, carries: []string{"angry"}}
	eng := runTagStage(t, saver)
	if err := eng.Persistence().Save(path, ""); err != nil {
		t.Fatalf("Save: %v", err)
	}

	loader := &tagStage{order: []string{"angry", "calm"}, loadFrom: path}
	runTagStage(t, loader)

	got := onlyMarks(t, loader)
	if !got.Has(loader.tags["angry"]) || got.Has(loader.tags["calm"]) {
		t.Errorf("after a load with the bits swapped, marks %b: angry=%v calm=%v, want angry alone",
			got, got.Has(loader.tags["angry"]), got.Has(loader.tags["calm"]))
	}
}

func TestTags_PanicWhenTheSaveNamesATagThisBuildDropped(t *testing.T) {
	path := t.TempDir() + "/save"
	saver := &tagStage{order: []string{"calm", "angry"}, carries: []string{"angry"}}
	if err := runTagStage(t, saver).Persistence().Save(path, ""); err != nil {
		t.Fatalf("Save: %v", err)
	}

	defer func() {
		if recover() == nil {
			t.Error("loading a save naming a tag this build lacks returned quietly, want a panic")
		}
	}()
	runTagStage(t, &tagStage{order: []string{"calm"}, loadFrom: path})
}
