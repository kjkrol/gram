package main

import (
	"math/rand/v2"
	"time"

	"github.com/kjkrol/aabbworld"
	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/aabbworld/plane"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram"
	"github.com/kjkrol/gram/control"
	"github.com/kjkrol/gram/entity"
	"github.com/kjkrol/gram/entity/kind"
	"github.com/kjkrol/gram/entity/kind/comp"
	"github.com/kjkrol/gram/game"
	"github.com/kjkrol/gram/game/stage"
	"github.com/kjkrol/gram/plugins/cameras"
	"github.com/kjkrol/gram/plugins/collision"
	"github.com/kjkrol/gram/plugins/players"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/rule"
	"github.com/kjkrol/gram/rule/effect"
	"github.com/kjkrol/gram/ui"
)

const (
	TPS          = 60
	ScreenWidth  = 800
	ScreenHeight = 600

	Size    = 16
	Walkers = 30
	Ghosts  = 6
	Leaders = 1

	rageFor = 3 * time.Second
)

type arena struct {
	world     *world.Plugin
	collision *collision.Plugin
	players   *players.Plugin
	cameras   *cameras.Plugin
	player    *players.Player
}

func main() {
	props := game.Props{
		Title:       "gram appearance demo",
		Description: "R: everyone angry for a while.",
		ScreenWidth: ScreenWidth, ScreenHeight: ScreenHeight, Resizable: true,
		TargetTPS: TPS,
	}
	a := &arena{}
	mainStage := stage.New(AppearanceStage).
		World(world.Config{
			Space: world.SpaceCfg{
				Width:  ScreenWidth,
				Height: ScreenHeight,
				Edges:  aabbworld.Torus,
			},
			Entities: world.EntitiesCfg{
				MaxCount: Walkers + Ghosts + Leaders,
				MinSize:  Size,
				MaxSize:  Size,
			},
		}).
		Plugins(a.makePlugins).
		Players(a.definePlayer).
		Effects(a.defineEffects).
		Rules(a.defineRules).
		Commands(a.defineCommands).
		Kinds(a.defineKinds).
		Controls(a.bindKeys).
		Spawn(a.spawnUnits).
		Scenes(a.defineScenes).
		Update(a.update)
	gram.Run(game.New(props, mainStage))
}

func (s *arena) makePlugins(w *world.Plugin) {
	s.world = w
	s.collision = collision.NewPlugin(s.world)
	s.cameras = cameras.NewPlugin(s.world)
	s.players = players.NewPlugin(s.world, s.cameras)
}

func (s *arena) definePlayer() error {
	s.player = s.players.Local("player")
	return s.player.Bind(s.players.Defaults()...)
}

func (s *arena) defineEffects() {
	s.world.Effects().Define(RageEf, effect.Spec{effect.Lasts(rageFor)})
	s.world.Effects().Define(AngryEf, effect.Spec{})
}

func (s *arena) defineRules() {
	s.world.Roles().Define(MoodyRole, rule.Branch[world.Moving]{
		Name: "rage spreads",
		On:   rule.Self,
		When: rule.Under(s.world.Effects().Named(RageEf)).On(entity.World),
		Do:   rule.Hold(s.world.Effects().Named(AngryEf)),
	})
}

func (s *arena) defineCommands() {
	s.world.Commands().Define(RageCmd, rule.Put(s.world.Effects().Named(RageEf)).On(entity.World))
}

type walker struct {
	at  geom.Vec
	vel world.Velocity
}

func (s *arena) defineKinds() {
	kinds := s.world.Kinds()
	spec := kind.Spec{
		comp.Load(func(w walker) world.Position {
			return world.Position{
				AABB: plane.NewAABB(geom.NewVec(w.at.X-Size/2, w.at.Y-Size/2), Size, Size),
			}
		}),
		comp.Load(func(w walker) world.Velocity { return w.vel }),
		comp.Const(collision.Collider{}),
		comp.Const(collision.Physics{Restitution: 1}),
		rule.Plays(s.world.Roles().Named(MoodyRole)),
	}
	kind.Define[walker](kinds, WalkerKind, spec)
	kind.Define[walker](kinds, GhostKind, spec)
	kind.Define[walker](kinds, LeaderKind, spec)
}

func (s *arena) bindKeys() error {
	return s.player.Bind(
		control.Give(control.KeyPress{Key: control.KeyR}, "Make everyone angry for a while",
			s.world.Commands().Named(RageCmd)),
	)
}

func (s *arena) spawnUnits() {
	total := Walkers + Ghosts + Leaders
	placement := world.NewGridPlacement(ScreenWidth, ScreenHeight, Size)
	ways := [4]geom.Vec{
		geom.NewVec(1, 0),
		geom.NewVec(-1, 0),
		geom.NewVec(0, 1),
		geom.NewVec(0, -1),
	}
	leaderKind := kind.Named[walker](s.world.Kinds(), LeaderKind)
	ghostKind := kind.Named[walker](s.world.Kinds(), GhostKind)
	walkerKind := kind.Named[walker](s.world.Kinds(), WalkerKind)
	entries := make([]kind.Entry, total)
	for i := range entries {
		w := walker{
			at:  placement.Place(i, total).Center(),
			vel: world.Velocity{Dir: ways[rand.IntN(4)], Value: 40 + 40*rand.Float64()},
		}
		switch {
		case i < Leaders:
			entries[i] = leaderKind.Entry(w)
		case i < Leaders+Ghosts:
			entries[i] = ghostKind.Entry(w)
		default:
			entries[i] = walkerKind.Entry(w)
		}
	}
	s.world.Seed(entries...)
}

func (s *arena) defineScenes() []game.Scene {
	m := &mainScene{arena: s}
	return []game.Scene{ui.NewScene(MainScene, m.screen()).Input(s.players.Handle)}
}

func (s *arena) update(ctx goke.RunCtx, d time.Duration) {
	s.world.RunPlan(ctx, d)
	s.collision.RunPlan(ctx, d)
	s.cameras.RunPlan(ctx, d)
	s.players.RunPlan(ctx, d)
	ctx.Sync()
}
