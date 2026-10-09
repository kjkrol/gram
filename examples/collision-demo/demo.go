package main

import (
	"fmt"
	"log"
	"math"
	"math/rand/v2"
	"slices"
	"time"

	"github.com/kjkrol/aabbworld"
	"github.com/kjkrol/goke/v3"
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
	TPS          = 60 * 2
	ScreenWidth  = 1024
	ScreenHeight = 1024

	saveBasePath = "collision-demo"

	// hitDuration is how long an entity keeps showing a collision.
	hitDuration = 50 * time.Millisecond
)

// rng is where every random choice in this demo comes from; a benchmark pins its seed.
var rng = rand.New(rand.NewPCG(rand.Uint64(), rand.Uint64()))

// RectSize, FillPercent and the EntityCount they imply are what the demo runs with;
// a benchmark sets them to tick this same Stage at another scale.
var (
	RectSize    uint32 = 5
	FillPercent        = 20.0
	EntityCount        = countFor(RectSize, FillPercent)
)

// countFor is how many entities of side rect fill percent of the screen.
func countFor(rect uint32, percent float64) int {
	return int(math.Floor(percent / 100.0 * float64(ScreenWidth*ScreenHeight) / float64(rect*rect)))
}

// =========================== Game ===========================

// =========================== Stage ===========================

const (
	entityColors = 7
	entityShapes = 4
)

// body is the row every entity kind spawns from: its starting position and velocity.
type body struct {
	pos world.Position
	vel world.Velocity
}

// bodyKind names the kind drawn with color ci and shape si.
func bodyKind(ci, si int) string { return fmt.Sprintf("entity-%d-%d", ci, si) }

type arena struct {
	world     *world.Plugin
	collision *collision.Plugin

	collisionStats collision.ContactStats

	players *players.Plugin
	cameras *cameras.Plugin
	player  *players.Player // the one at the keyboard
}

// NewDemo is the collision demo, defined on the arena s: its window and its one Stage, a section at
// a time.
func NewDemo(s *arena) game.Game {
	return game.New(game.Props{
		Title:       "gram collision demo",
		ScreenWidth: ScreenWidth, ScreenHeight: ScreenHeight, Resizable: true,
		TargetTPS: TPS,
	},
		stage.New(CollisionStage).
			World(world.Config{
				Space:    world.SpaceCfg{Width: ScreenWidth, Height: ScreenHeight, Edges: aabbworld.Torus},
				Entities: world.EntitiesCfg{MaxCount: EntityCount, MinSize: RectSize, MaxSize: RectSize},
			}).
			Plugins(s.makePlugins).
			Players(s.definePlayer).
			Effects(s.defineEffects).
			Rules(s.defineRules).
			Kinds(s.defineKinds).
			Restore(s.restore).
			Spawn(s.spawnUnits).
			Scenes(s.defineScenes).
			Update(s.update))
}

// makePlugins makes the Stage's plugins over its world w — anew each time the Stage is entered —
// each telling the world of itself as it is made: the Stage uses them in that order.
func (s *arena) makePlugins(w *world.Plugin) {
	s.world = w
	s.collision = collision.NewPlugin(s.world).WithStats(&s.collisionStats)
	s.cameras = cameras.NewPlugin(s.world)
	s.players = players.NewPlugin(s.world, s.cameras).WithSaves(saveBasePath)
}

func (s *arena) definePlayer() error {
	s.player = s.players.Local("player")
	return s.player.Bind(s.players.Defaults()...)
}

func (s *arena) defineEffects() {
	s.world.Effects().Define(HitEf, effect.Spec{effect.Lasts(hitDuration)})
}

func (s *arena) defineRules() {
	s.world.Roles().Define(BodyRole,
		rule.Then[collision.Struck]("hit", rule.All, rule.Apply(s.world.Effects().Named(HitEf))))
}

func (s *arena) defineScenes(ctx game.Initializer) []game.Scene {
	m := &mainScene{arena: s, tps: ctx.TPS()}
	return []game.Scene{ui.NewScene(MainScene, m.screen()).Input(s.players.Handle)}
}

func (s *arena) restore(p game.Persistence) (bool, error) {
	saves, err := p.List(saveBasePath)
	if err != nil {
		return false, err
	}
	if !slices.Contains(saves, "") {
		return false, nil
	}
	if err := p.Load(saveBasePath, ""); err != nil {
		return false, err
	}
	log.Print("loaded saved world")
	return true, nil
}

func (s *arena) defineKinds() {
	kinds := s.world.Kinds()
	for ci := range entityColors {
		for si := range entityShapes {
			kind.Define[body](kinds, bodyKind(ci, si), kind.Spec{
				comp.Load(func(b body) world.Position { return b.pos }),
				comp.Load(func(b body) world.Velocity { return b.vel }),
				comp.Const(collision.Collider{}),
				comp.Const(collision.Physics{Restitution: 1}),
				rule.Plays(s.world.Roles().Named(BodyRole)),
			})
		}
	}
}

func (s *arena) spawnUnits() {
	placement := world.NewGridPlacement(ScreenWidth, ScreenHeight, RectSize)
	motion := newRandomVelocity(200, 50, 10)
	bodyKindOf := func(c, sh int) kind.Of[body] { return kind.Named[body](s.world.Kinds(), bodyKind(c, sh)) }
	entries := make([]kind.Entry, EntityCount)
	for i := range entries {
		entries[i] = bodyKindOf(rng.IntN(entityColors), rng.IntN(entityShapes)).Entry(
			body{pos: placement.Place(i, EntityCount), vel: motion.initialVelocity(i)})
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
