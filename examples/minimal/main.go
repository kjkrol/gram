// Command minimal is the smallest gram game that does something: one Stage with a world of
// bouncing boxes, a collision plugin counting their contacts, and one Scene showing them.
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
	"github.com/kjkrol/gram/ui"
)

const (
	screenWidth, screenHeight = 640, 480
	boxSize                   = 12
	boxCount                  = 300
)

func main() { gram.Run(&Game{stage: newArena()}) }

// Game is the game itself: window props and one Stage.
type Game struct{ stage game.Stage }

func (g *Game) Props() game.Props {
	return game.Props{Title: "gram minimal", ScreenWidth: screenWidth, ScreenHeight: screenHeight, TargetTPS: 60}
}

func (g *Game) Stages() (map[string]game.Stage, string) {
	return map[string]game.Stage{g.stage.Name(): g.stage}, g.stage.Name()
}

// box is the row every entity spawns from: where it starts and how it moves.
type box struct {
	pos world.Position
	vel world.Velocity
}

// BoxKind is the name the one kind of unit is defined by, and found by again.
const BoxKind = "box"

// arena is what the one Stage keeps: a torus of bouncing boxes.
type arena struct {
	world     *world.Plugin
	collision *collision.Plugin
	cameras   *cameras.Plugin
	players   *players.Plugin
	player    *players.Player // the one at the keyboard
	stats     collision.ContactStats
	picture   *render.Composer // the boxes, as the scene shows them
	tps       *game.TPS
}

// newArena defines the Stage a section at a time, in the order a Stage is always defined in; a
// section this game has no use for — cells, effects, rules — is left out.
func newArena() game.Stage {
	a := &arena{}
	return stage.New("arena").
		Plugins(a.usePlugins).
		Players(a.definePlayer).
		Kinds(a.defineKinds).
		Scenes(a.defineScenes).
		Units(a.placeUnits).
		Update(a.update)
}

func (a *arena) usePlugins(ctx game.Initializer) error {
	a.world = ctx.UseWorld(world.Config{
		Space:    world.SpaceCfg{Width: screenWidth, Height: screenHeight, Edges: aabbworld.Torus},
		Entities: world.EntitiesCfg{MaxCount: boxCount, MinSize: boxSize, MaxSize: boxSize},
	})
	a.collision = collision.NewPlugin(a.world).WithStats(&a.stats)
	a.cameras = cameras.NewPlugin(a.world)
	a.players = players.NewPlugin(a.world, a.cameras)
	for _, p := range []plugin.Plugin{a.collision, a.cameras, a.players} {
		if err := ctx.Use(p); err != nil {
			return err
		}
	}
	return nil
}

// definePlayer is whoever sits at the keyboard, with the keys the plugins give: Space pauses,
// K lists them all, Shift+Esc quits, the wheel and W, A, S, D move the camera.
func (a *arena) definePlayer() error {
	a.player = a.players.Local("player", a.cameras.New(cameras.TopDown(), camera.Config{}))
	return a.player.Bind(a.players.Defaults()...)
}

func (a *arena) defineKinds() {
	kind.Define[box](a.world.Kinds(), BoxKind, kind.Spec{
		comp.Load(func(b box) world.Position { return b.pos }),
		comp.Load(func(b box) world.Velocity { return b.vel }),
		comp.Const(collision.Collider{}),
		comp.Const(collision.Physics{Restitution: 1}),
	})
}

// defineScenes is the one scene: the world's picture and the screen it is shown on.
func (a *arena) defineScenes(ctx game.Initializer) []game.Scene {
	a.tps = ctx.TPS()
	return []game.Scene{ui.NewScene("view", a.pictures, a.screen).Input(a.players.Handle)}
}

func (a *arena) placeUnits() {
	boxKind := kind.Named[box](a.world.Kinds(), BoxKind)
	rng := rand.New(rand.NewPCG(1, 2))
	placement := world.NewGridPlacement(screenWidth, screenHeight, boxSize)
	entries := make([]kind.Entry, boxCount)
	for i := range entries {
		var vel world.Velocity
		vel.SetDelta(geom.NewVec(rng.Float64()*200-100, rng.Float64()*200-100))
		entries[i] = boxKind.Entry(box{pos: placement.Place(i, boxCount), vel: vel})
	}
	a.world.Seed(entries...)
}

func (a *arena) update(ctx goke.RunCtx, d time.Duration) {
	a.world.RunPlan(ctx, d)
	a.collision.RunPlan(ctx, d)
	a.cameras.RunPlan(ctx, d)
	a.players.RunPlan(ctx, d)
	ctx.Sync()
}

// The scene's colours: the boxes and the backdrop.
var (
	boxColor        = color.RGBA{R: 90, G: 200, B: 110, A: 255}
	backgroundColor = color.RGBA{R: 30, G: 30, B: 30, A: 255}
)

// pictures dresses the boxes and hands the world's picture.
func (a *arena) pictures() []render.WorldRenderer {
	boxKind := kind.Named[box](a.world.Kinds(), BoxKind)
	atlas := render.NewAtlas()
	atlas.Add(boxKind, boxSize, render.Solid(boxColor))
	atlas.Close()
	a.world.WithRenderer(atlas)
	a.picture = render.NewComposer(a.world.Renderer())
	return []render.WorldRenderer{a.picture}
}

// screen is the world through the player's camera on a dark backdrop, a telemetry line over it.
func (a *arena) screen() *ui.Element {
	count := func() int { return a.world.Res.Telemetry.Count }
	return ui.Layers( // from the bottom up: each covers those before it
		ui.Blank().Fill(backgroundColor),
		ui.Image(render.NewFeed(a.player.Camera, a.picture)).Input(a.players.Through(a.player)),
		ui.Layer(render.NewTelemetryRenderer(&a.tps.Ticks, count).With(a.stats.Reporter(&a.tps.Ticks))),
	)
}
