// Command minimal is the smallest gram game that does something: one Stage with a world of
// bouncing boxes, a collision plugin counting their contacts, and one Scene drawing them.
// It is the README's example, kept here so it compiles and runs.
package main

import (
	"image/color"
	"math/rand/v2"
	"time"

	"github.com/kjkrol/aabbworld"
	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram"
	"github.com/kjkrol/gram/control"
	"github.com/kjkrol/gram/game"
	"github.com/kjkrol/gram/plugins/collision"
	"github.com/kjkrol/gram/plugins/collision/hooks"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/plugins/world/entity/kind"
	"github.com/kjkrol/gram/plugins/world/entity/kind/comp"
	"github.com/kjkrol/gram/render"
)

const (
	screenWidth, screenHeight = 640, 480
	boxSize                   = 12
	boxCount                  = 300
)

func main() { gram.Run(&Game{}) }

// Game is the game itself: window props and one Stage.
type Game struct{ stage arena }

func (g *Game) Props() game.Props {
	return game.Props{Title: "gram minimal", ScreenWidth: screenWidth, ScreenHeight: screenHeight, TargetTPS: 60}
}

func (g *Game) Stages() (map[string]game.Stage, string) {
	return map[string]game.Stage{g.stage.Name(): &g.stage}, g.stage.Name()
}

// box is the row every entity spawns from: where it starts and how it moves.
type box struct {
	pos world.Position
	vel world.Velocity
}

// arena is the one Stage: a torus of bouncing boxes.
type arena struct {
	world     *world.Plugin
	collision *collision.Plugin
	boxes     kind.Of[box]
	stats     hooks.ContactStats
	scenes    game.Scenes
}

func (a *arena) Name() string       { return "arena" }
func (a *arena) Stack() game.Scenes { return a.scenes }

// Init installs the plugins and defines what a box is.
func (a *arena) Init(ctx game.Initializer) error {
	a.world = ctx.UseWorld(world.Config{
		Space:    world.SpaceCfg{Width: screenWidth, Height: screenHeight, Edges: aabbworld.Torus},
		Entities: world.EntitiesCfg{MaxCount: boxCount, MinSize: boxSize, MaxSize: boxSize},
	})
	a.boxes = kind.Define[box](a.world.Kinds(), "box", kind.Spec{
		comp.Load(func(b box) world.Position { return b.pos }),
		comp.Load(func(b box) world.Velocity { return b.vel }),
		comp.Const(collision.Collider{}),
		comp.Const(collision.Physics{Restitution: 1}),
	})

	a.collision = collision.NewPlugin(a.world)
	if err := a.collision.Hook(
		hooks.CountContacts(&a.stats),
	); err != nil {
		return err
	}
	if err := ctx.Use(a.collision); err != nil {
		return err
	}

	scenes, err := game.NewStack(&view{arena: a, tps: ctx.TPS()})
	if err != nil {
		return err
	}
	a.scenes = scenes
	scenes.Composition().Show("view")
	return ctx.Track(scenes.Composition())
}

// Restore has nothing to restore from: this game keeps no saves.
func (a *arena) Restore(game.Persistence) (bool, error) { return false, nil }

// Spawn scatters the boxes on a grid, each heading somewhere at random.
func (a *arena) Spawn() error {
	rng := rand.New(rand.NewPCG(1, 2))
	placement := world.NewGridPlacement(screenWidth, screenHeight, boxSize)
	entries := make([]kind.Entry, boxCount)
	for i := range entries {
		var vel world.Velocity
		vel.SetDelta(geom.NewVec(rng.Float64()*200-100, rng.Float64()*200-100))
		entries[i] = a.boxes.Entry(box{pos: placement.Place(i, boxCount), vel: vel})
	}
	a.world.Seed(entries...)
	return nil
}

// Update is one tick: move, then collide.
func (a *arena) Update(ctx goke.RunCtx, d time.Duration) {
	a.world.RunPlan(ctx, d)
	a.collision.RunPlan(ctx, d)
	ctx.Sync()
}

// view is the one Scene: the boxes over a dark background, with a telemetry line.
type view struct {
	arena *arena
	tps   *game.TPS
}

func (v *view) Name() string    { return "view" }
func (v *view) Focusable() bool { return true }

func (v *view) Layers() []render.Layer {
	atlas := render.NewAtlas()
	atlas.RegisterAt(v.arena.boxes.SpriteID(), boxSize, render.Solid(color.RGBA{R: 90, G: 200, B: 110, A: 255}))
	atlas.Close()
	v.arena.world.WithRenderer(atlas)

	count := func() int { return v.arena.world.Res.Telemetry.Count }
	return []render.Layer{
		render.SolidBackground{Color: color.RGBA{R: 30, G: 30, B: 30, A: 255}},
		render.NewComposer(v.arena.world.Renderer()),
		render.NewTelemetryRenderer(&v.tps.Ticks, count).With(v.arena.stats.Reporter(&v.tps.Ticks)),
	}
}

// Viewports are where the world is shown: the camera over the whole screen.
func (v *view) Viewports(screen geom.AABB) []render.Viewport {
	return render.Whole(v.arena.world.Camera(), screen)
}

func (v *view) HandleEvents(events *control.InputEvents, runtime game.Runtime, _ game.Composition) {
	for _, k := range events.KeyEvents {
		if k.Action == control.ActionPress && k.Key == control.KeyEscape {
			runtime.Quit()
		}
	}
}
