// Command appearance-demo shows a game declaring how its entities are drawn in one place — the
// world's atlas in the scene's Layers: every walker is drawn by the twin of the way it goes
// (Facing), the leader turned smoothly with his crown on (Turning), the ghost as a ghost
// whatever happens, and the angry under the angry effect's own look (Under). They bounce off
// one another, so they turn, and the looks follow. R makes everyone angry for a while: an
// effect on the world, kept on each entity by a rule — anger has no direction, so an angry
// walker is a plain red square.
package main

import (
	"image/color"
	"math/rand/v2"
	"time"

	"github.com/kjkrol/aabbworld"
	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/aabbworld/plane"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/control"
	"github.com/kjkrol/gram/entity"
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

// Demo is the appearance demo — one Stage.
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
		Title:       "gram appearance demo — R: everyone angry for a while",
		ScreenWidth: ScreenWidth, ScreenHeight: ScreenHeight, Resizable: true,
		TargetTPS: TPS,
	}
}

func (d *Demo) Stages() (map[string]game.Stage, string) {
	return map[string]game.Stage{d.stage.Name(): d.stage}, d.stage.Name()
}

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

// newStage defines the game a section at a time, each building on those before it.
// newArena makes the arena — the collector of the stage's plugins, which every
// section builds on — and defines the stage on it, a section at a time.
func newArena() (*arena, game.Stage) {
	s := &arena{}
	return s, stage.New("appearance-demo").
		Plugins(s.usePlugins).
		Players(s.definePlayer).
		Effects(s.defineEffects).
		Rules(s.defineRules).
		Commands(s.defineCommands).
		Kinds(s.defineKinds).
		Controls(s.bindKeys).
		Scenes(s.defineScenes).
		Units(s.placeUnits).
		Update(s.update)
}

func (s *arena) usePlugins(ctx game.Initializer) error {
	s.world = ctx.UseWorld(world.Config{
		Space:    world.SpaceCfg{Width: ScreenWidth, Height: ScreenHeight, Edges: aabbworld.Torus},
		Entities: world.EntitiesCfg{MaxCount: Walkers + Ghosts + Leaders, MinSize: Size, MaxSize: Size},
	})
	s.collision = collision.NewPlugin(s.world)
	s.cameras = cameras.NewPlugin(s.world, cameras.TopDown(), camera.Config{})
	s.players = players.NewPlugin(s.world, s.cameras)
	for _, p := range []plugin.Plugin{s.collision, s.cameras, s.players} {
		if err := ctx.Use(p); err != nil {
			return err
		}
	}
	return nil
}

func (s *arena) definePlayer() error {
	s.player = s.players.Local("player", s.cameras.Main())
	return s.player.Bind(s.players.Defaults()...)
}

func (s *arena) defineEffects() {
	s.world.Effects().Define(RageEf, effect.Spec{effect.Lasts(rageFor)})
	s.world.Effects().Define(AngryEf, effect.Spec{}) // its marker is its look: Under in the Layers
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
	return []game.Scene{&mainScene{arena: s}}
}

// boxAt is the box of side Size round at.
func boxAt(at geom.Vec) plane.AABB {
	return plane.NewAABB(geom.NewVec(at.X-Size/2, at.Y-Size/2), Size, Size)
}

func (s *arena) placeUnits() {
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

// =========================== Scene ===========================

type mainScene struct{ arena *arena }

var _ game.Scene = (*mainScene)(nil)

func (m *mainScene) Name() string { return "main" }

// The scene's colours: the walkers calm and angry with their light noses, the ghost's pale and
// its eyes, the crown's gold, the backdrop.
var (
	calmColor       = color.RGBA{R: 70, G: 130, B: 220, A: 255}
	angryColor      = color.RGBA{R: 220, G: 60, B: 50, A: 255}
	noseColor       = color.RGBA{R: 245, G: 245, B: 230, A: 255}
	ghostColor      = color.RGBA{R: 225, G: 230, B: 245, A: 200}
	eyeColor        = color.RGBA{A: 255}
	goldColor       = color.RGBA{R: 240, G: 200, B: 40, A: 255}
	backgroundColor = color.RGBA{R: 40, G: 44, B: 52, A: 255}
)

func (m *mainScene) Layers() []render.Layer {
	s := m.arena
	s.world.WithRenderer(s.looks())
	return []render.Layer{
		render.NewCachedRenderer(render.SolidBackground{Color: backgroundColor}, ScreenWidth, ScreenHeight),
		render.NewComposer(s.world.Renderer()),
	}
}

// looks declares every look in one place, on the world's atlas: the walker by the twin of the
// way it goes (Facing) and a plain red square while angry — anger has no direction; the leader
// turned smoothly with his crown on (Turning), red and crowned while angry; the ghost a ghost
// whatever happens.
func (s *arena) looks() *world.Atlas {
	walkerKind := kind.Named[walker](s.world.Kinds(), WalkerKind)
	ghostKind := kind.Named[walker](s.world.Kinds(), GhostKind)
	leaderKind := kind.Named[walker](s.world.Kinds(), LeaderKind)
	angry := s.world.Effects().Named(AngryEf)

	atlas := s.world.NewAtlas()
	atlas.Add(walkerKind, Size, nosed(calmColor)).
		Facing(4, func(angleDeg float64) render.SpriteDrawer { return nosedAt(angleDeg, calmColor) }).
		Under(angry, render.Solid(angryColor))
	atlas.Add(leaderKind, Size, crowned(calmColor)).
		Under(angry, crowned(angryColor)).
		Turning() // the crown turns with him — and while he is angry too
	atlas.Add(ghostKind, Size, spook).
		Turning() // a ghost drifts face first; anger leaves it unmoved: no look under it
	atlas.Close()
	return atlas
}

// nosed is a square of body with a light nose eastwards — the way angle 0 points.
func nosed(body color.RGBA) render.SpriteDrawer { return nosedAt(0, body) }

// nosedAt is a square of body with its nose on the side angleDeg faces.
func nosedAt(angleDeg float64, body color.RGBA) render.SpriteDrawer {
	return func(dst *render.Canvas, size int) {
		render.Solid(body)(dst, size)
		render.Arrow(angleDeg, 4, noseColor)(dst, size)
	}
}

// crowned is a diamond of body under a gold band — drawn east first, within the circle inscribed
// in the box, so it turns whole.
func crowned(body color.RGBA) render.SpriteDrawer {
	return func(dst *render.Canvas, size int) {
		render.Diamond(body)(dst, size)
		render.Arrow(90, 3, goldColor)(dst, size)
	}
}

// spook is the ghost: a pale circle with two eyes.
func spook(dst *render.Canvas, size int) {
	r := float32(size) / 2
	dst.FillCircle(r, r, r-1, ghostColor)
	dst.FillRect(r-4, r-3, 2, 3, eyeColor)
	dst.FillRect(r+2, r-3, 2, 3, eyeColor)
}

// Viewports are where the world is shown: the local player's view.
func (m *mainScene) Viewports(screen geom.AABB) []render.Viewport {
	return m.arena.players.Viewports(screen)
}

func (m *mainScene) HandleEvents(events *control.InputEvents, runtime game.Runtime, composition game.Composition) {
	m.arena.players.Handle(events, runtime, composition)
}

func (m *mainScene) Focusable() bool { return true }
