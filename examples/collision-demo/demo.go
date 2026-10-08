package main

import (
	"fmt"
	"image/color"
	"log"
	"math"
	"math/rand/v2"
	"slices"
	"time"

	"github.com/kjkrol/aabbworld"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/entity/kind"
	"github.com/kjkrol/gram/entity/kind/comp"
	"github.com/kjkrol/gram/game"
	"github.com/kjkrol/gram/game/stage"
	"github.com/kjkrol/gram/plugin"
	"github.com/kjkrol/gram/plugins/cameras"
	"github.com/kjkrol/gram/plugins/collision"
	"github.com/kjkrol/gram/plugins/players"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/render"
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

// Demo is the collision demo — exactly one Stage (arena below).
type Demo struct {
	a     *arena
	stage game.Stage
}

var _ game.Game = (*Demo)(nil)

func NewDemo() *Demo {
	a, st := newArena()
	return &Demo{a: a, stage: st}
}

func (d *Demo) Props() game.Props {
	return game.Props{
		Title:       "gram collision demo",
		ScreenWidth: ScreenWidth, ScreenHeight: ScreenHeight, Resizable: true,
		TargetTPS: TPS,
	}
}

func (d *Demo) Stages() (map[string]game.Stage, string) {
	return map[string]game.Stage{d.stage.Name(): d.stage}, d.stage.Name()
}

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

// newStage defines the game a section at a time, each building on those before it.
// newArena makes the arena — the collector of the stage's plugins, which every
// section builds on — and defines the stage on it, a section at a time.
func newArena() (*arena, game.Stage) {
	s := &arena{}
	return s, stage.New(CollisionStage).
		Plugins(s.usePlugins).
		Players(s.definePlayer).
		Effects(s.defineEffects).
		Rules(s.defineRules).
		Kinds(s.defineKinds).
		Restore(s.restore).
		Spawn(s.spawnUnits).
		Scenes(s.defineScenes).
		Update(s.update)
}

func (s *arena) usePlugins(ctx game.Initializer) error {
	s.world = ctx.UseWorld(world.Config{
		Space:    world.SpaceCfg{Width: ScreenWidth, Height: ScreenHeight, Edges: aabbworld.Torus},
		Entities: world.EntitiesCfg{MaxCount: EntityCount, MinSize: RectSize, MaxSize: RectSize},
	})
	s.collision = collision.NewPlugin(s.world).WithStats(&s.collisionStats)
	s.cameras = cameras.NewPlugin(s.world)
	s.players = players.NewPlugin(s.world, s.cameras).WithSaves(saveBasePath)
	for _, p := range []plugin.Plugin{s.collision, s.cameras, s.players} {
		if err := ctx.Use(p); err != nil {
			return err
		}
	}
	return nil
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

// =========================== Scene ===========================

type mainScene struct {
	arena *arena
	tps   *game.TPS
}

// The scene's colours: a palette of the bodies' colours — the one past entityColors is the
// hit's overlay — and the backdrop.
var (
	palette = [8]color.RGBA{
		{R: 80, G: 120, B: 220, A: 255},
		{R: 90, G: 200, B: 110, A: 255},
		{R: 80, G: 200, B: 210, A: 255},
		{R: 150, G: 100, B: 220, A: 255},
		{R: 220, G: 210, B: 80, A: 255},
		{R: 230, G: 160, B: 60, A: 255},
		{R: 60, G: 160, B: 150, A: 255},
		{R: 220, G: 40, B: 40, A: 255},
	}
	backgroundColor = color.RGBA{R: 50, G: 50, B: 50, A: 255}
)

// picture dresses the world and hands its picture.
func (m *mainScene) picture() render.Picture {
	s := m.arena

	atlas := s.world.NewAtlas()
	hit := s.world.Effects().Named(HitEf)
	flash := palette[entityColors]
	shapes := [entityShapes]func(color.RGBA) render.SpriteDrawer{render.Solid, render.Border, render.Diamond, render.Cross}
	bodyKindOf := func(c, sh int) kind.Of[body] { return kind.Named[body](s.world.Kinds(), bodyKind(c, sh)) }
	for ci, c := range palette[:entityColors] {
		for si, shape := range shapes {
			atlas.Add(bodyKindOf(ci, si), int(RectSize), shape(c)).
				Under(hit, func(dst *render.Canvas, size int) { // struck: its own shape, flashed
					shapes[si](flash)(dst, size)
				})
		}
	}
	atlas.Close()
	s.world.WithRenderer(atlas)

	return render.NewComposer(s.world.Renderer())
}

// screen is the world through a camera of its own, the player's view, on its backdrop, a telemetry line over it.
func (m *mainScene) screen() *ui.Element {
	s := m.arena
	entityCount := func() int { return s.world.Res.Telemetry.Count }
	return ui.Layers( // from the bottom up: each covers those before it
		ui.Blank().Fill(backgroundColor),
		ui.Image(render.NewFeed(s.cameras.New(cameras.TopDown(), camera.Config{}), m.picture())).Input(s.players.Through(s.player)),
		ui.Layer(render.NewTelemetryRenderer(&m.tps.Ticks, entityCount).With(s.collisionStats.Reporter(&m.tps.Ticks))),
	)
}
