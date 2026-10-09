// Command appearance-demo shows a game declaring how its entities are drawn in one place — the
// world's atlas in the scene's pictures: every walker is drawn by the twin of the way it goes
// (Facing), the leader turned smoothly with his crown on (Turning), the ghost as a ghost
// whatever happens, and the angry under the angry effect's own look (Under). They bounce off
// one another, so they turn, and the looks follow. R makes everyone angry for a while: an
// effect on the world, kept on each entity by a rule — anger has no direction, so an angry
// walker is a plain red square.
package main

import (
	"math/rand/v2"
	"time"

	"github.com/kjkrol/aabbworld"
	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/aabbworld/plane"
	"github.com/kjkrol/goke/v3"
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

	// Size is every entity's side; Walkers, Ghosts and Leaders how many of each there are.
	Size    = 16
	Walkers = 30
	Ghosts  = 6
	Leaders = 1

	// rageFor is how long R keeps everyone angry.
	rageFor = 3 * time.Second
)

// rng is where every random choice in this demo comes from; the test pins its seed.
var rng = rand.New(rand.NewPCG(rand.Uint64(), rand.Uint64()))

// =========================== Game ===========================

// =========================== Stage ===========================

// walker is the row every kind spawns from: where it starts and how it goes.
type walker struct {
	at  geom.Vec
	vel world.Velocity
}

type arena struct {
	world     *world.Plugin
	collision *collision.Plugin
	players   *players.Plugin
	cameras   *cameras.Plugin

	player *players.Player
}

// NewDemo is the appearance demo, defined on the arena s: its window and its one Stage, a section
// at a time.
func NewDemo(s *arena) game.Game {
	return game.New(game.Props{
		Title:       "gram appearance demo",
		Description: "R: everyone angry for a while.",
		ScreenWidth: ScreenWidth, ScreenHeight: ScreenHeight, Resizable: true,
		TargetTPS: TPS,
	},
		stage.New(AppearanceStage).
			World(world.Config{
				Space:    world.SpaceCfg{Width: ScreenWidth, Height: ScreenHeight, Edges: aabbworld.Torus},
				Entities: world.EntitiesCfg{MaxCount: Walkers + Ghosts + Leaders, MinSize: Size, MaxSize: Size},
			}).
			Plugins(s.makePlugins).
			Players(s.definePlayer).
			Effects(s.defineEffects).
			Rules(s.defineRules).
			Commands(s.defineCommands).
			Kinds(s.defineKinds).
			Controls(s.bindKeys).
			Spawn(s.spawnUnits).
			Scenes(s.defineScenes).
			Update(s.update))
}

// makePlugins makes the Stage's plugins over its world w — anew each time the Stage is entered —
// each telling the world of itself as it is made: the Stage uses them in that order.
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
	s.world.Effects().Define(AngryEf, effect.Spec{}) // its marker is its look: Under in the scene's pictures
}

func (s *arena) defineRules() {
	s.world.Roles().Define(MoodyRole,
		rule.Then[world.Moving]("rage spreads", rule.All, rule.During(s.world.Effects().Named(RageEf), rule.Keep(s.world.Effects().Named(AngryEf)))))
}

func (s *arena) defineKinds() {
	kinds := s.world.Kinds()
	spec := kind.Spec{
		comp.Load(func(w walker) world.Position { return world.Position{AABB: boxAt(w.at)} }),
		comp.Load(func(w walker) world.Velocity { return w.vel }),
		comp.Const(collision.Collider{}),
		comp.Const(collision.Physics{Restitution: 1}),
		rule.Plays(s.world.Roles().Named(MoodyRole)),
	}
	kind.Define[walker](kinds, WalkerKind, spec)
	kind.Define[walker](kinds, GhostKind, spec)
	kind.Define[walker](kinds, LeaderKind, spec)
}

func (s *arena) defineCommands() {
	s.world.Commands().Define(RageCmd, rule.Cast(s.world.Effects().Named(RageEf)).On(entity.World))
}

func (s *arena) bindKeys() error {
	return s.player.Bind(control.Give(control.KeyPress{Key: control.KeyR}, "Make everyone angry for a while",
		s.world.Commands().Named(RageCmd)))
}

func (s *arena) defineScenes() []game.Scene {
	m := &mainScene{arena: s}
	return []game.Scene{ui.NewScene(MainScene, m.screen()).Input(s.players.Handle)}
}

// boxAt is the box of side Size round at.
func boxAt(at geom.Vec) plane.AABB {
	return plane.NewAABB(geom.NewVec(at.X-Size/2, at.Y-Size/2), Size, Size)
}

func (s *arena) spawnUnits() {
	total := Walkers + Ghosts + Leaders
	placement := world.NewGridPlacement(ScreenWidth, ScreenHeight, Size)
	ways := [4]geom.Vec{geom.NewVec(1, 0), geom.NewVec(-1, 0), geom.NewVec(0, 1), geom.NewVec(0, -1)}
	leaderKind := kind.Named[walker](s.world.Kinds(), LeaderKind)
	ghostKind := kind.Named[walker](s.world.Kinds(), GhostKind)
	walkerKind := kind.Named[walker](s.world.Kinds(), WalkerKind)
	entries := make([]kind.Entry, total)
	for i := range entries {
		w := walker{at: placement.Place(i, total).Center(), vel: world.Velocity{Dir: ways[rng.IntN(4)], Value: 40 + 40*rng.Float64()}}
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

func (s *arena) update(ctx goke.RunCtx, d time.Duration) {
	s.world.RunPlan(ctx, d)
	s.collision.RunPlan(ctx, d)
	s.cameras.RunPlan(ctx, d)
	s.players.RunPlan(ctx, d)
	ctx.Sync()
}
