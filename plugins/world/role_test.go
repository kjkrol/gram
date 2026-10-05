package world_test

import (
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/aabbworld/plane"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/entity/kind"
	"github.com/kjkrol/gram/entity/kind/comp"
	"github.com/kjkrol/gram/entity/tag"
	"github.com/kjkrol/gram/game"
	"github.com/kjkrol/gram/internal/engine"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/rule"
)

// roleStage defines roles, and tags of moods after them, in whatever order it is given and spawns
// one entity playing and carrying the named ones, so two runs can disagree about which bit is which.
type roleStage struct {
	roles, moods   []string
	plays, carries []string
	loadFrom       string

	world *world.Plugin
	unit  kind.Of[struct{}]
	role  map[string]*rule.Part
	mood  map[string]tag.Tag[moods]
	probe *roleProbe
	stack game.Scenes
}

type roleProbe struct {
	plays goke.Comp[tag.Tags[rule.Roles]]
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
	g.role = map[string]*rule.Part{}
	for _, name := range g.roles {
		g.world.Roles().Define(name)
		g.role[name] = g.world.Roles().Named(name)
	}
	g.mood = map[string]tag.Tag[moods]{}
	for _, name := range g.moods {
		g.mood[name] = g.world.Kinds().DefineTag[moods](name)
	}
	var plays []*rule.Part
	for _, name := range g.plays {
		plays = append(plays, g.role[name])
	}
	var carries []tag.Tag[moods]
	for _, name := range g.carries {
		carries = append(carries, g.mood[name])
	}
	g.unit = kind.Define[struct{}](g.world.Kinds(), "unit", kind.Spec{
		comp.Const(world.Position{AABB: plane.NewAABB(geom.NewVec(100, 100), 10, 10)}),
		comp.Const(world.Velocity{}),
		rule.Plays(plays...),
		comp.Tagged(carries...),
	})
	return nil
}

func (g *roleStage) Restore(p game.Persistence) (bool, error) {
	if g.loadFrom == "" {
		return false, nil
	}
	return true, p.Load(g.loadFrom, "")
}

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

// runRoleStage builds the stage through the engine: Init, Restore or Spawn, Setup.
func runRoleStage(t *testing.T, g *roleStage) *engine.Engine {
	t.Helper()
	eng := engine.NewEngine(oneStageGame{stage: g, props: game.Props{}})
	if err := eng.Init(); err != nil {
		t.Fatalf("Init: %v", err)
	}
	return eng
}

// onlyRoles reads the roles and moods of the single entity the stage holds.
func onlyRoles(t *testing.T, g *roleStage) (tag.Tags[rule.Roles], tag.Tags[moods]) {
	t.Helper()
	var plays tag.Tags[rule.Roles]
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

// panicMessage runs f and returns what it panicked with, failing the test if it did not.
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
			rule.Plays(mortal),
			rule.Plays(hasty),
		})
	})
	want := reflect.TypeFor[tag.Tags[rule.Roles]]().String() + " twice"
	if !strings.Contains(msg, `"scout"`) || !strings.Contains(msg, want) {
		t.Errorf("panic %q does not name the kind and %q", msg, want)
	}
}

func TestRoles_SurviveASaveTheWorldNamesThemIn(t *testing.T) {
	path := t.TempDir() + "/save"

	saver := &roleStage{
		roles: []string{"mortal", "hasty", "lazy"}, plays: []string{"mortal", "hasty"},
		moods: []string{"calm", "angry"}, carries: []string{"calm"},
	}
	if err := runRoleStage(t, saver).Persistence().Save(path, ""); err != nil {
		t.Fatalf("Save: %v", err)
	}

	loader := &roleStage{roles: []string{"lazy", "hasty", "mortal"}, moods: []string{"angry", "calm"}, loadFrom: path}
	runRoleStage(t, loader)

	plays, marks := onlyRoles(t, loader)
	mortal, hasty := loader.role["mortal"].Tag(), loader.role["hasty"].Tag()
	if plays != tag.Tags[rule.Roles](0).With(mortal, hasty) {
		t.Errorf("after a load, plays %b, want mortal (bit %d) and hasty (bit %d) alone", plays, mortal, hasty)
	}
	if marks != tag.Tags[moods](0).With(loader.mood["calm"]) {
		t.Errorf("after a load with the moods reordered too, marks %b, want calm alone (bit %d)", marks, loader.mood["calm"])
	}
}
