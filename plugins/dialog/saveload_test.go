package dialog_test

import (
	"testing"
	"testing/fstest"
	"time"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/aabbworld/plane"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/entity/kind"
	"github.com/kjkrol/gram/entity/kind/comp"
	"github.com/kjkrol/gram/game"
	"github.com/kjkrol/gram/internal/engine"
	"github.com/kjkrol/gram/plugins/dialog"
	"github.com/kjkrol/gram/plugins/world"
)

type talkerStage struct {
	load bool // entered from the quicksave

	world  *world.Plugin
	dialog *dialog.Plugin
	talker kind.Of[struct{}]
	script goke.Comp[dialog.Script]
	talk   goke.Comp[dialog.Talk]
	query  *goke.Query
	stack  game.Scenes
}

func (s *talkerStage) Name() string { return "stage" }

func (s *talkerStage) Init(ctx game.Initializer) error {
	s.world = ctx.UseWorld(world.Config{
		Space:    world.SpaceCfg{Width: 1000, Height: 1000},
		Entities: world.EntitiesCfg{MaxCount: 1, MinSize: 1, MaxSize: 10},
	})
	s.dialog = dialog.NewPlugin(s.world, dialog.Config{})
	if err := ctx.Use(s.dialog); err != nil {
		return err
	}
	s.dialog.DefineEffects()
	if err := s.dialog.Load(fstest.MapFS{"a.yaml": {Data: []byte("greet:\n  say: [\"Hi\"]\n")}}, "a.yaml"); err != nil {
		return err
	}
	ctx.Setup(talkerProbe{s})
	script := s.dialog.Script("greet")
	kind.Define[struct{}](s.world.Kinds(), "talker", kind.Spec{
		comp.Const(world.Position{AABB: plane.NewAABB(geom.NewVec(100, 100), 10, 10)}),
		comp.Const(world.Velocity{}),
		comp.Const(script),
		comp.Const(dialog.Talk{Node: script.Start, With: 42}),
	})
	s.talker = kind.Named[struct{}](s.world.Kinds(), "talker")
	return nil
}

func (s *talkerStage) Loaded() error { return nil }

func (s *talkerStage) Spawn() error {
	s.world.Seed(s.talker.Entry(struct{}{}))
	return nil
}

func (s *talkerStage) Update(goke.RunCtx, time.Duration) {}

func (s *talkerStage) Stack() game.Scenes {
	if s.stack == nil {
		s.stack, _ = game.NewStack()
	}
	return s.stack
}

func (s *talkerStage) carried(t *testing.T) (dialog.Script, dialog.Talk) {
	t.Helper()
	s.query.All()
	if !s.query.Next() || len(s.query.Cursor().IDs) != 1 {
		t.Fatal("no talker, or more than one")
	}
	cur := s.query.Cursor()
	return s.script.Slice(cur)[0], s.talk.Slice(cur)[0]
}

type talkerProbe struct{ s *talkerStage }

func (p talkerProbe) SetupSystems() []goke.System {
	return []goke.System{goke.SystemFn{OnInit: func(si *goke.SysInit) {
		p.s.query = si.NewQueryBuilder(&p.s.script, &p.s.talk).Build()
	}}}
}

type talkerGame struct{ stage *talkerStage }

func (g talkerGame) Props() game.Props { return game.Props{} }
func (g talkerGame) Stages() (map[string]game.Stage, string) {
	return map[string]game.Stage{g.stage.Name(): g.stage}, g.stage.Name()
}

func TestDialog_SurvivesASaveAndALoad(t *testing.T) {
	t.Chdir(t.TempDir())
	saver := &talkerStage{}
	eng := engine.NewEngine(talkerGame{saver})
	if err := eng.Init(); err != nil {
		t.Fatal(err)
	}
	script, talk := saver.carried(t)
	if err := eng.Save(""); err != nil {
		t.Fatal(err)
	}
	loader := &talkerStage{load: true}
	eng2 := engine.NewEngine(talkerGame{loader})
	if err := eng2.Load("", ""); err != nil {
		t.Fatalf("Load: %v", err)
	}
	if err := eng2.Init(); err != nil {
		t.Fatal(err)
	}
	gotScript, gotTalk := loader.carried(t)
	if gotScript != script || gotTalk != talk {
		t.Errorf("after a load: %+v %+v, want %+v %+v", gotScript, gotTalk, script, talk)
	}
}
