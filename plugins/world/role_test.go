package world_test

import (
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/aabbworld/plane"
	"github.com/kjkrol/ceremony"
	"github.com/kjkrol/ceremony/comp"
	"github.com/kjkrol/ceremony/cond"
	"github.com/kjkrol/ceremony/tag"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/entity/kind"
	"github.com/kjkrol/gram/game"
	"github.com/kjkrol/gram/internal/engine"
	"github.com/kjkrol/gram/plugins/world"
)

type roleStage struct {
	roles, moods   []string
	plays, carries []string
	load           bool

	world *world.Plugin
	unit  kind.Of[struct{}]
	role  map[string]*ceremony.Part
	mood  map[string]tag.Tag[moods]
	probe *roleProbe
	stack game.Scenes
}

type roleProbe struct {
	plays goke.Comp[tag.Tags[cond.Roles]]
	marks goke.Comp[tag.Tags[moods]]
	query *goke.Query
}

func (p *roleProbe) SetupSystems() []goke.System {
	return []goke.System{goke.SystemFn{OnInit: func(si *goke.SysInit) {
		p.query = si.NewQueryBuilder(&p.plays, &p.marks).Build()
	}}}
}

func (g *roleStage) Name() string { return "stage" }

func (g *roleStage) Init(ctx game.Initializer) error {
	g.world = ctx.UseWorld(testWorldConfig())
	g.probe = &roleProbe{}
	ctx.Setup(g.probe)
	g.role = map[string]*ceremony.Part{}
	for _, name := range g.roles {
		g.world.Roles().Define(name)
		g.role[name] = g.world.Roles().Named(name)
	}
	g.mood = map[string]tag.Tag[moods]{}
	for _, name := range g.moods {
		g.mood[name] = g.world.Kinds().DefineTag[moods](name)
	}
	var plays []*ceremony.Part
	for _, name := range g.plays {
		plays = append(plays, g.role[name])
	}
	var carries []tag.Tag[moods]
	for _, name := range g.carries {
		carries = append(carries, g.mood[name])
	}
	kind.Define[struct{}](g.world.Kinds(), "unit", kind.Spec{
		comp.Const(world.Position{AABB: plane.NewAABB(geom.NewVec(100, 100), 10, 10)}),
		comp.Const(world.Velocity{}),
		ceremony.Plays(plays...),
		comp.Tagged(carries...),
	})
	g.unit = kind.Named[struct{}](g.world.Kinds(), "unit")
	return nil
}

func (g *roleStage) Loaded() error { return nil }

func (g *roleStage) Spawn() error {
	g.world.Seed(g.unit.Entry(struct{}{}))
	return nil
}

func (g *roleStage) Update(goke.RunCtx, time.Duration) {}

func (g *roleStage) Stack() game.Scenes {
	if g.stack == nil {
		g.stack, _ = game.NewStack()
	}
	return g.stack
}

func runRoleStage(t *testing.T, g *roleStage) *engine.Engine {
	t.Helper()
	eng := engine.NewEngine(oneStageGame{stage: g, props: game.Props{}})
	if g.load {
		if err := eng.Load("", ""); err != nil {
			t.Fatalf("Load: %v", err)
		}
	}
	if err := eng.Init(); err != nil {
		t.Fatalf("Init: %v", err)
	}
	return eng
}

func onlyRoles(t *testing.T, g *roleStage) (tag.Tags[cond.Roles], tag.Tags[moods]) {
	t.Helper()
	var plays tag.Tags[cond.Roles]
	var marks tag.Tags[moods]
	seen := 0
	g.probe.query.All()
	for g.probe.query.Next() {
		cursor := g.probe.query.Cursor()
		for i := range cursor.IDs {
			plays, marks = g.probe.plays.Slice(cursor)[i], g.probe.marks.Slice(cursor)[i]
			seen++
		}
	}
	if seen != 1 {
		t.Fatalf("found %d entities playing roles, want exactly 1", seen)
	}
	return plays, marks
}

func panicMessage(t *testing.T, f func()) (msg string) {
	t.Helper()
	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("returned quietly, want a panic")
		}
		msg, _ = r.(string)
	}()
	f()
	return ""
}

func TestRoles_PlayedByAKind(t *testing.T) {
	g := &roleStage{roles: []string{"mortal", "hasty", "lazy"}, plays: []string{"mortal", "lazy"}}
	runRoleStage(t, g)

	plays, _ := onlyRoles(t, g)
	if !plays.Has(g.role["mortal"].Tag()) || !plays.Has(g.role["lazy"].Tag()) || plays.Has(g.role["hasty"].Tag()) {
		t.Errorf("plays %b, want mortal and lazy, not hasty", plays)
	}
}

func TestRoles_PlaysTwiceInOneKindPanics(t *testing.T) {
	w := world.NewPlugin(testWorldConfig())
	w.Roles().Define("mortal")
	w.Roles().Define("hasty")
	mortal, hasty := w.Roles().Named("mortal"), w.Roles().Named("hasty")

	msg := panicMessage(t, func() {
		kind.Define[struct{}](w.Kinds(), "scout", kind.Spec{
			comp.Const(world.Position{AABB: plane.NewAABB(geom.NewVec(100, 100), 10, 10)}),
			comp.Const(world.Velocity{}),
			ceremony.Plays(mortal),
			ceremony.Plays(hasty),
		})
	})
	want := reflect.TypeFor[tag.Tags[cond.Roles]]().String() + " twice"
	if !strings.Contains(msg, `"scout"`) || !strings.Contains(msg, want) {
		t.Errorf("panic %q does not name the kind and %q", msg, want)
	}
}

func TestRoles_SurviveASaveTheWorldNamesThemIn(t *testing.T) {
	t.Chdir(t.TempDir())

	saver := &roleStage{
		roles: []string{"mortal", "hasty", "lazy"}, plays: []string{"mortal", "hasty"},
		moods: []string{"calm", "angry"}, carries: []string{"calm"},
	}
	if err := runRoleStage(t, saver).Save(""); err != nil {
		t.Fatalf("Save: %v", err)
	}

	loader := &roleStage{roles: []string{"lazy", "hasty", "mortal"}, moods: []string{"angry", "calm"}, load: true}
	runRoleStage(t, loader)

	plays, marks := onlyRoles(t, loader)
	mortal, hasty := loader.role["mortal"].Tag(), loader.role["hasty"].Tag()
	if plays != tag.Tags[cond.Roles](0).With(mortal, hasty) {
		t.Errorf("after a load, plays %b, want mortal (bit %d) and hasty (bit %d) alone", plays, mortal, hasty)
	}
	if marks != tag.Tags[moods](0).With(loader.mood["calm"]) {
		t.Errorf("after a load with the moods reordered too, marks %b, want calm alone (bit %d)", marks, loader.mood["calm"])
	}
}
