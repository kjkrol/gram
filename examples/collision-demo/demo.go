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
	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/control"
	"github.com/kjkrol/gram/entity/kind"
	"github.com/kjkrol/gram/entity/kind/comp"
	"github.com/kjkrol/gram/game"
	"github.com/kjkrol/gram/game/stage"
	"github.com/kjkrol/gram/plugin"
	"github.com/kjkrol/gram/plugins/collision"
	"github.com/kjkrol/gram/plugins/collision/hooks"
	"github.com/kjkrol/gram/plugins/players"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/render"
	"github.com/kjkrol/gram/rule/effect"
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

// Demo is the collision demo — exactly one Stage (mainStage below).
type Demo struct{ stage *mainStage }

var _ game.Game = (*Demo)(nil)

func NewDemo() *Demo { return &Demo{stage: newStage()} }

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

// State  persisting arbitrary game-owned state across a save/load cycle.
type State struct{ Saves int }

const (
	entityColors = 7
	entityShapes = 4
)

// body is the row every entity kind spawns from: its starting position and velocity.
type body struct {
	pos world.Position
	vel world.Velocity
}

// entityKindName names the kind drawn with color ci and shape si.
func entityKindName(ci, si int) string { return fmt.Sprintf("entity-%d-%d", ci, si) }

type mainStage struct {
	game.Stage // defined a section at a time: newStage

	world     *world.Plugin
	collision *collision.Plugin

	// kinds is one kind per color and shape; hitSprite is the overlay's atlas slot, no kind's.
	kinds     [entityColors][entityShapes]kind.Of[body]
	hitSprite render.SpriteID
	hit       effect.Effect

	state          *State
	collisionStats collision.ContactStats

	players *players.Plugin
}

// newStage defines the game a section at a time, each building on those before it.
func newStage() *mainStage {
	s := &mainStage{}
	s.Stage = stage.New("collision-demo").
		Plugins(s.usePlugins).
		Players(s.definePlayer).
		Effects(s.defineEffects).
		Rules(s.defineRules).
		Kinds(s.defineKinds).
		Looks(s.defineLooks).
		Scenes(s.defineScenes).
		Restore(s.restore).
		Units(s.placeUnits).
		Update(s.update)
	return s
}

func (s *mainStage) usePlugins(ctx game.Initializer) error {
	s.world = ctx.UseWorld(world.Config{
		Space:    world.SpaceCfg{Width: ScreenWidth, Height: ScreenHeight, Edges: aabbworld.Torus},
		Entities: world.EntitiesCfg{MaxCount: EntityCount, MinSize: RectSize, MaxSize: RectSize},
	})
	s.collision = collision.NewPlugin(s.world).WithStats(&s.collisionStats)
	s.players = players.NewPlugin(s.world)
	for _, p := range []plugin.Plugin{s.collision, s.players} {
		if err := ctx.Use(p); err != nil {
			return err
		}
	}
	s.state = &State{}
	return nil
}

// definePlayer makes the player and its camera's keys: drag with the middle button, scroll with
// the wheel, push an edge.
func (s *mainStage) definePlayer() error {
	return s.players.Local("player").Bind(s.players.Defaults()...)
}

// defineEffects says the one state: hit, for a moment after an entity struck another.
func (s *mainStage) defineEffects() { s.hit = hooks.Hit(s.world, hitDuration) }

// defineRules says the one rule: whoever strikes something is hit.
func (s *mainStage) defineRules(ctx game.Initializer) error { return ctx.Hook(hooks.ShowHits(s.hit)) }

// defineLooks has whoever is hit drawn under the hit's overlay.
func (s *mainStage) defineLooks() error {
	return s.world.Draw(hooks.HitOverlay(s.hit, world.Appearance{SpriteID: s.hitSprite}))
}

func (s *mainStage) defineScenes(ctx game.Initializer) []game.Scene {
	return []game.Scene{&mainScene{stage: s, tps: ctx.TPS()}}
}

func (s *mainStage) restore(p game.Persistence) (bool, error) {
	saves, err := p.List(saveBasePath)
	if err != nil {
		return false, err
	}
	if !slices.Contains(saves, "") {
		return false, nil
	}
	if err := p.Load(saveBasePath, "", s.state); err != nil {
		return false, err
	}
	log.Printf("loaded saved world (save #%d)", s.state.Saves)
	return true, nil
}

// defineKinds says what this game's entities are, fresh or restored.
func (s *mainStage) defineKinds() {
	kinds := s.world.Kinds()
	for ci := range entityColors {
		for si := range entityShapes {
			s.kinds[ci][si] = kind.Define[body](kinds, entityKindName(ci, si), kind.Spec{
				comp.Load(func(b body) world.Position { return b.pos }),
				comp.Load(func(b body) world.Velocity { return b.vel }),
				comp.Const(collision.Collider{}),
				comp.Const(collision.Physics{Restitution: 1}),
			})
		}
	}
	s.hitSprite = s.world.Kinds().NewSprite() // the overlay's atlas slot, no kind's
}

// placeUnits says who is there when the game starts fresh.
func (s *mainStage) placeUnits() {
	placement := world.NewGridPlacement(ScreenWidth, ScreenHeight, RectSize)
	motion := newRandomVelocity(200, 50, 10)
	entries := make([]kind.Entry, EntityCount)
	for i := range entries {
		entries[i] = s.kinds[rng.IntN(entityColors)][rng.IntN(entityShapes)].Entry(
			body{pos: placement.Place(i, EntityCount), vel: motion.initialVelocity(i)})
	}
	s.world.Seed(entries...)
}

func (s *mainStage) update(ctx goke.RunCtx, d time.Duration) {
	s.world.RunPlan(ctx, d)
	s.collision.RunPlan(ctx, d)
	s.players.RunPlan(ctx, d)
	ctx.Sync()
}

// =========================== Scene ===========================

type mainScene struct {
	stage *mainStage
	tps   *game.TPS
}

var _ game.Scene = (*mainScene)(nil)

func (m *mainScene) Name() string { return "main" }

func (m *mainScene) Layers() []render.Layer {
	s := m.stage

	palette := [8]color.RGBA{
		{R: 80, G: 120, B: 220, A: 255},
		{R: 90, G: 200, B: 110, A: 255},
		{R: 80, G: 200, B: 210, A: 255},
		{R: 150, G: 100, B: 220, A: 255},
		{R: 220, G: 210, B: 80, A: 255},
		{R: 230, G: 160, B: 60, A: 255},
		{R: 60, G: 160, B: 150, A: 255},
		{R: 220, G: 40, B: 40, A: 255},
	}
	atlas := render.NewAtlas()
	shapes := [entityShapes]func(color.RGBA) render.SpriteDrawer{render.Solid, render.Border, render.Diamond, render.Cross}
	for ci, c := range palette[:entityColors] {
		for si, shape := range shapes {
			atlas.RegisterAt(s.kinds[ci][si].SpriteID(), int(RectSize), shape(c))
		}
	}

	atlas.RegisterAt(s.hitSprite, int(RectSize), render.Solid(palette[entityColors]))
	atlas.Close()
	s.world.WithRenderer(atlas)

	entityCount := func() int { return s.world.Res.Telemetry.Count }
	return []render.Layer{
		render.NewCachedRenderer(
			render.SolidBackground{Color: color.RGBA{R: 50, G: 50, B: 50, A: 255}},
			ScreenWidth, ScreenHeight,
		),
		render.NewComposer(s.world.Renderer()),
		render.NewTelemetryRenderer(&m.tps.Ticks, entityCount).With(s.collisionStats.Reporter(&m.tps.Ticks)),
	}
}

// Viewports are where the world is shown: the local players' views.
func (m *mainScene) Viewports(screen geom.AABB) []render.Viewport {
	return m.stage.players.Viewports(screen)
}

func (m *mainScene) HandleEvents(events *control.InputEvents, runtime game.Runtime, composition game.Composition) {
	m.stage.players.EventHandler().HandleEvents(events)
	s := m.stage
	for _, k := range events.KeyEvents {
		if k.Action != control.ActionPress {
			continue
		}
		switch k.Key {
		case control.KeyEscape:
			runtime.Quit()
		case control.KeyF5:
			s.state.Saves++
			if err := runtime.Persistence().Save(saveBasePath, "", s.state); err != nil {
				log.Printf("save: %v", err)
				continue
			}
			log.Printf("saved (save #%d)", s.state.Saves)
		}
	}
}

func (m *mainScene) Focusable() bool { return true }
