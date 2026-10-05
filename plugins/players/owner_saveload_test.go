package players_test

import (
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
	"github.com/kjkrol/gram/plugins/players"
	"github.com/kjkrol/gram/plugins/players/owner"
	"github.com/kjkrol/gram/plugins/world"
)

// ownerStage is a world with two players and one unit the second owns; loadFrom, when set, is the
// save it restores.
type ownerStage struct {
	loadFrom string

	world   *world.Plugin
	players *players.Plugin
	unit    kind.Of[struct{}]
	owners  goke.Comp[tag.Tags[owner.Family]]
	query   *goke.Query
	stack   game.Scenes
}

func (s *ownerStage) Name() string { return "stage" }

func (s *ownerStage) Init(ctx game.Initializer) error {
	s.world = ctx.UseWorld(world.Config{
		Space:    world.SpaceCfg{Width: 1000, Height: 1000},
		Entities: world.EntitiesCfg{MaxCount: 1, MinSize: 1, MaxSize: 10},
	})
	s.players = players.NewPlugin(s.world)
	s.players.Local("first")
	second := s.players.Add("second")
	ctx.Setup(ownersProbe{s})
	kind.Define[struct{}](s.world.Kinds(), "unit", kind.Spec{
		comp.Const(world.Position{AABB: plane.NewAABB(geom.NewVec(100, 100), 10, 10)}),
		comp.Const(world.Velocity{}),
		comp.Tagged(owner.Of(second.ID)),
	})
	s.unit = kind.Named[struct{}](s.world.Kinds(), "unit")
	return ctx.Use(s.players)
}

func (s *ownerStage) Restore(p game.Persistence) (bool, error) {
	if s.loadFrom == "" {
		return false, nil
	}
	return true, p.Load(s.loadFrom, "")
}

func (s *ownerStage) Spawn() error {
	s.world.Seed(s.unit.Entry(struct{}{}))
	return nil
}

func (s *ownerStage) Update(goke.RunCtx, time.Duration) {}

func (s *ownerStage) Stack() game.Scenes {
	if s.stack == nil {
		s.stack, _ = game.NewStack()
	}
	return s.stack
}

// ownersOfTheUnit is what the stage's one unit is owned by.
func (s *ownerStage) ownersOfTheUnit(t *testing.T) tag.Tags[owner.Family] {
	t.Helper()
	var got tag.Tags[owner.Family]
	n := 0
	for s.query.All(); s.query.Next(); {
		for _, o := range s.owners.Slice(s.query.Cursor()) {
			got, n = o, n+1
		}
	}
	if n != 1 {
		t.Fatalf("%d entities carry owners, want the one unit", n)
	}
	return got
}

// ownersProbe builds the stage's query of the owners as the stage is set up.
type ownersProbe struct{ s *ownerStage }

func (p ownersProbe) SetupSystems() []goke.System {
	return []goke.System{goke.SystemFn{OnInit: func(si *goke.SysInit) { p.s.query = si.NewQueryBuilder(&p.s.owners).Build() }}}
}

type ownerGame struct{ stage *ownerStage }

func (g ownerGame) Props() game.Props { return game.Props{} }
func (g ownerGame) Stages() (map[string]game.Stage, string) {
	return map[string]game.Stage{g.stage.Name(): g.stage}, g.stage.Name()
}

// A unit a player owns is that player's still after the game is saved and loaded: the owners'
// tags are saved by name.
func TestOwner_SurvivesASaveAndALoad(t *testing.T) {
	path := t.TempDir() + "/save"
	saver := &ownerStage{}
	eng := engine.NewEngine(ownerGame{saver})
	if err := eng.Init(); err != nil {
		t.Fatal(err)
	}
	if got := saver.ownersOfTheUnit(t); !owner.Obeys(got, 2) || owner.Obeys(got, 1) {
		t.Fatalf("the unit is owned by %b, want the second player alone", got)
	}
	if err := eng.Persistence().Save(path, ""); err != nil {
		t.Fatal(err)
	}
	loader := &ownerStage{loadFrom: path}
	if err := engine.NewEngine(ownerGame{loader}).Init(); err != nil {
		t.Fatal(err)
	}
	if got := loader.ownersOfTheUnit(t); !owner.Obeys(got, 2) || owner.Obeys(got, 1) {
		t.Errorf("after a load the unit is owned by %b, want the second player alone", got)
	}
}
