# gram

<p align="center">
  <img src=".github/docs/img/gram_logo.png" alt="gram logo" width="300">
  <br>
  <a href="https://go.dev"><img src="https://img.shields.io/badge/Go-1.27+-00ADD8?style=flat-square&logo=go" alt="Go Version"></a>
  <a href="https://pkg.go.dev/github.com/kjkrol/gram"><img src="https://img.shields.io/badge/GoDoc-Reference-007d9c?style=flat-square&logo=go" alt="GoDoc"></a>
  <a href="https://opensource.org/licenses/MIT"><img src="https://img.shields.io/badge/License-MIT-yellow.svg?style=flat-square" alt="License"></a>
  <a href="https://app.codecov.io/gh/kjkrol/gram"><img src="https://img.shields.io/codecov/c/github/kjkrol/gram?style=flat-square&logo=codecov" alt="Codecov Coverage"></a>
  <a href="https://github.com/kjkrol/gram/actions"><img src="https://github.com/kjkrol/gram/actions/workflows/go.yml/badge.svg" alt="Go Quality Check"></a>
</p>

**gram** is a modular game engine for Go. A game is a set of named **Stages**, each with its
own entity-component world and its own **Scenes**; an engine drives the active Stage on the
[goke](https://github.com/kjkrol/goke) ECS, a tick and a picture a frame, and draws it on the GPU
through WebGPU ([gogpu](https://github.com/gogpu/gogpu), pure Go over Vulkan, Metal or DirectX). Everything beyond the tick loop is a **plugin**: the
built-in ones give a Stage a world of moving boxes, collisions, sight, a board with terrain,
pathfinding and mouse selection, and a game adds its own the same way. Formerly *gokebiten*.

<p align="center">
    <a href="#features">Features</a>
    &nbsp;&bull;&nbsp;
    <a href="#installation">Installation</a>
    &nbsp;&bull;&nbsp;
    <a href="#example">Example</a>
    &nbsp;&bull;&nbsp;
    <a href="#demos">Demos</a>
    &nbsp;&bull;&nbsp;
    <a href="#model">Model</a>
    &nbsp;&bull;&nbsp;
    <a href="#architecture">Architecture</a>
    &nbsp;&bull;&nbsp;
    <a href="BENCHMARKS.md">Benchmarks</a>
    &nbsp;&bull;&nbsp;
    <a href="#documentation">Documentation</a>
</p>

# Design Goals

- **One extension point.** Everything beyond the tick loop is a `plugin.Plugin`, built-in or
  yours, installed from a Stage's `Init` with `ctx.Use`. There is no registry, no lookup by name
  and no install-order retry: a plugin that needs another takes it as a constructor argument.
- **A Stage owns its ECS.** Each Stage gets a fresh world the moment it is entered, so a menu
  Stage sits idle with no gameplay entities until the player starts.
- **Behaviour is rules.** Game logic reacting to what a plugin finds is a rule run inside that
  plugin's own pass; the moment's type says whose it is, `ctx.Hook` hooks it there, and a rule no
  plugin in use hosts is an error, never a silent no-op. Roles say who obeys a rule, wires what a
  lever drives.
- **Kinds say what an entity is.** A kind is the list of components its entities carry, each
  constant or read from the entity's own row; it also tells save files what to expect.
- **Saves survive change.** Persisted resources are matched by name, never by position, so a
  save survives plugins being added, removed or reordered between versions.
- **Contract in the open, orchestration inside.** What a game implements (`game`, `plugin`,
  `render`) is public and documented; only the engine that drives it is `internal`.

<a id="installation"></a>
# 📦 Installation

```bash
go get github.com/kjkrol/gram
```

**Prerequisites:** Go 1.27+ and a GPU with a Vulkan, Metal or DirectX 12 driver; gogpu needs no
cgo. Without a GPU the tests that draw skip themselves.

<a id="features"></a>
# ✨ Key Features

| Capability | Package | What you get |
|:---|:---|:---|
| **Stages and Scenes** | `game` | Named Stages with their own ECS and lifecycle (`Init`/`Restore`/`Spawn`/`Update`); Scenes with layered renderers and input; a live Composition of what is shown and which Scene is active |
| **Plugins and rules** | `plugin` | The one extension contract; rules hooked on the plugin whose pass catches their moment, pairs too, run by its hosts (`Rules`, `PairRules`, `StepRules`) with a `Tick` |
| **Behaviour** | `rule` | One vocabulary: rules at a plugin's moments, hooked with `ctx.Hook` on whichever plugin hosts them; roles a kind or a cell plays — the rules they obey, the abilities a player casts; wires by name from levers, plates and switches to what they drive; plans a kind's entities follow, effects that hold, commands an entity gives itself as a player would, facts plugins tell it |
| **World** | `plugins/world` | Every entity's `Base` (position, velocity, kind, capabilities); movement under stop, wrap or open edges; the shared spatial index and camera; spawning from kinds; `Config.Heights` for a world with heights |
| **Steering and views** | `plugins/world/steering`, `plugins/world/view` | A `Steering` profile turned into heading and speed each tick; a `View` of what a camera sees |
| **Kinds** | `entity/kind` | `Define` a kind from a `Spec` of `Const` and `Load` components; `Entry` rows onto the roster |
| **Collisions** | `plugins/collision` | Collision over the world's space: `Collider` to take part, `Physics` to bounce and be pushed apart, `Meeting`/`Struck` for rules |
| **Sight** | `plugins/vision` | A `Sight` cone scanned each tick into `Sighted`, nearest first; `Sighting` rules per observer; outlines shown with Shift+C; in a world with heights the eye looks over walls, forests and hills by height |
| **Board and navigation** | `plugins/board`, `plugins/navigation` | Square or hex grid with terrain and occupancy; `MoveOrder` paths that re-route when terrain changes |
| **Topography** | `plugins/topography` | A map in relief over the board: the ground's heights shaped by the player and pricing every slope, the sun's light on the relief and the terrain's shadows, grounds blending, round coasts, water glinting and running, rivers and roads drawn across the cells, the clouds' shadows, less detail far off; each kind styled by name; seen from above, isometrically or in perspective, Tab goes round |
| **Selection** | `plugins/selection` | A `Select` command into a `Selected` tag, with default bindings (click, marquee, shift-add) and a highlight renderer |
| **Players** | `plugins/players` | Who acts: a camera and view per player, the plugins' default bindings gathered and bound, input translated into typed commands the defining plugins drain |
| **Persistence** | `game.Persistence` | Save, load and list the active Stage's ECS and every tracked value by name |
| **Camera and rendering** | `camera`, `render` | A wrap-aware camera with zoom and pan; everything drawn on the GPU (WebGPU, WGSL): sources composed by tier with a shared depth buffer, Direct sources with shaders of their own, a picture composed once and kept on the GPU (`Still`), instanced sprites; an atlas baked at `Close`, telemetry renderers |

<a id="example"></a>
# Example

The smallest game that does something: one Stage with a torus of bouncing boxes, a collision
plugin counting their contacts, and one Scene drawing them. This is
[`examples/minimal`](examples/minimal/main.go); run it with `make demo-minimal`.

```go
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
	"github.com/kjkrol/gram/entity/kind"
	"github.com/kjkrol/gram/entity/kind/comp"
	"github.com/kjkrol/gram/game"
	"github.com/kjkrol/gram/plugins/collision"
	"github.com/kjkrol/gram/plugins/world"
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
	stats     collision.ContactStats
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

	a.collision = collision.NewPlugin(a.world).WithStats(&a.stats)
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
```

## Demos

[`examples/collision-demo`](examples/collision-demo) is the same idea at scale: thousands of
colliding boxes at a fixed 120 TPS, with save and load on F5.

<table>
  <thead>
    <tr>
      <th style="text-align: left; vertical-align: top; width: 400px;">
        <video src="https://github.com/user-attachments/assets/2b921500-eb3e-49bf-98ee-ac741746e64d" width="400" autoplay loop muted playsinline></video>
        <br>
          <sub><strong>Stats:</strong> 2306 colliding AABBs | 120 TPS | 50 collisions/tick</sub>
      </th>
      <th style="text-align: left; vertical-align: top; width: 400px;">
        <video src="https://github.com/user-attachments/assets/50695c5a-4f77-4352-87da-1fa13168415b" width="400" autoplay loop muted playsinline></video>
        <br>
        <sub><strong>Stats:</strong> 524 colliding AABBs | 120 TPS | 15 collisions/tick</sub>
      </th>
    </tr>
  </thead>
</table>

<a id="demos"></a>

| Demo | Shows | Run |
|:---|:---|:---|
| [`minimal`](examples/minimal) | One Stage, one Scene, a world with collisions: the README example | `make demo-minimal` |
| [`collision-demo`](examples/collision-demo) | Thousands of bouncing boxes of many kinds, hit overlays, telemetry, save and load | `make demo-collision` |
| [`appearance-demo`](examples/appearance-demo) | Walkers bouncing off one another, drawn by the world's ready-made drawing rules: facing the way each goes, red while angry, a ghost as a ghost whatever it feels, the leader with a crown on; R makes everyone angry for a while — what is drawn follows, the Appearance is never touched | `make demo-appearance` |
| [`scenes-demo`](examples/scenes-demo) | A menu Stage switching into a gameplay Stage, a modal Scene over the ticking world, a non-focusable HUD | `make demo-scenes` |
| [`navigation-demo`](examples/navigation-demo) | A board with terrain, units selected by click and marquee, right-click move orders along re-routing paths, holes the planner avoids and H opens under the units | `make demo-navigation` |
| [`navigation-hex-demo`](examples/navigation-hex-demo) | The same on a hex board: hex cells and route arrows at 60°, a wall of solid hex cells | `make demo-navigation-hex` |
| [`navigation-vision-demo`](examples/navigation-vision-demo) | Navigated units with sight cones that stop at walls and fade in forests, and a hawk that flies over both and sees through the forest | `make demo-navigation-vision` |
| [`navigation-vision-hex-demo`](examples/navigation-vision-hex-demo) | The same sight cones and hawk on a hex board | `make demo-navigation-vision-hex` |
| [`board`](examples/board) | The island on the simple map: a flat world whose board draws itself from its kinds' colours, the streams, rivers, roads and bridges as plain bands; units walk from stop to stop over the roads with sight cones, a day goes by over the flat map — tiles and units tinted by the hour, clouds' shadows over the screen, rain and snow, snow lying and shores freezing in winter | `make demo-board` |
| [`board-topography`](examples/board-topography) | The same island in relief through the topography: a range of peaks and a plateau lit by the sun, sea cliffs, streams and rivers whose water runs and falls, roads over bridges, slower up the slopes and routed round them, the ground shaped under the cursor; seen isometrically, from above or in perspective, Tab goes round; units billboards, the hawk 40 up looking over what a walker's cone climbs and stops at; a day and the weather going by, snow and ice in winter | `make demo-board-topography` |
| [`board-atlas`](examples/board-atlas) | A small flat board drawn from the game's own atlas: striped grass, rippled water, a cobbled road, tree tops — sprites the game draws for its kinds — and a road laid as a way; units walk corner to corner | `make demo-board-atlas` |
| [`effect-demo`](examples/effect-demo) | An ice witch under orders turns the ground round her into snow and the lake into ice, fast on her own snow; it thaws behind her, a walker follows her trail while it lasts and slips on it, a boat with weak brakes sails onto the ice it saw coming and is frozen still and pale until it melts — all of it effects | `make demo-effect` |
| [`trapdoor-demo`](examples/trapdoor-demo) | Two levers and two strips of trapdoors across a meadow: wanderers walk to and fro over both, 1 and 2 pull a lever and its trapdoors open under whoever stands on them, the player's scouts too; J hastens the selected scouts to get clear — a lever a state of the game, the haste one of the scouts, the trapdoors cells tagged with their lever's group | `make demo-trapdoor` |
| [`pressure-plate-demo`](examples/pressure-plate-demo) | The same meadow with two pressure plates in place of the levers: walk a scout onto a plate and, while someone stands on it and a second after, its trapdoors are open under whoever is on them — whoever stands on a plate's cell presses it | `make demo-pressure-plate` |
| [`wire-demo`](examples/wire-demo) | The same meadow on three wires, each a connection by name whose own entity holds its state: 1 pulls the west lever for two seconds, a scout standing on the plate in the yard presses the east wire, G flips the gate's switch until G again, and U has a selected scout beside the lever in the yard pull it; the trapdoors, the plate, the gate and the lever are cells playing roles, each wired to its wire — one rule of a role for every cell playing it; everyone plays mortal and falls into an open trapdoor, and J hastens the selected scouts, which play hasty, never the porters | `make demo-wire` |
| [`split-screen-demo`](examples/split-screen-demo) | Two players at one keyboard: red drives its block with WSAD, blue with the arrows — each block its player's by the owner tag — each through a camera of its own in its half of the screen, and a minimap at the bottom shows the whole arena through a camera nobody drives | `make demo-split-screen` |
| [`vision-demo`](examples/vision-demo) | Entities keeping out of each other's way by sight, and a hunter living off the ones that fail | `make demo-vision` |

Every demo opens a window, so `go test` cannot exercise it; each ships its own tests of the
logic underneath.

<a id="model"></a>
# Model

## Stage and Scene

A `game.Game` supplies `Props` (window, tick rate) and its Stages by name. A Stage is one
self-contained context — a menu, the gameplay — with its own goke ECS, built fresh the moment
`Runtime.SwitchStage` enters it. Its lifecycle is `Init` (install plugins), `Restore` (resume
from a save, or report there is none), `Spawn` (seed the initial state, only when nothing was
restored) and `Update` (one tick, running the plugins' `RunPlan` in the order the game needs).

Within a Stage, a Scene is one thing it can show: its renderers, built once, and its input
handling. The Stage's `Scenes` registry is static; the `Composition` over it is live — which
Scenes are visible, in what order, and which is *active*, the topmost focusable one and the only
Scene whose `HandleEvents` runs. A HUD that is not focusable can sit on top and never steal
input. `Runtime` (pause, quit, switch Stage, persistence, camera) is one interface that reaches a
Stage and every Scene alike.

## Plugins, rules and plans

A plugin's `Install` only queues ECS wiring; the engine flushes it all in one `ecs.Setup` after
the Stage's `Init`, which is what lets `Restore` decide fresh-spawn or restore before the ECS
commits to either. Game logic that reacts to what a plugin finds is a *rule*, run in the pass of
the plugin that catches its moment:

```go
caught := rule.On("caught", rule.Between(predator, prey), func(m *rule.Moment[collision.Meeting]) rule.Step {
	return m.ForOther(m.Order(world.Despawn{}))
})
return ctx.Hook(caught)
```

fires for every pair it meets where one entity carries tag `predator` and the other `prey`, and
has the prey give itself a `Despawn`; `rule.All` would fire for every pair, `rule.Self(tag)` for
every entity carrying a tag. `ctx.Hook`, in `Init` once the plugins are used, hooks each rule on
the plugin in use that hosts its moment — a `Meeting` on collision — and a rule none hosts is an
error, never a silent no-op. A tag is a
bit of a family — one `tag.Tags[F]` component per family, named through `Kinds.DefineTag`, given
to a kind with `comp.Tagged` — so markers cost no component types of their own. What lasts over
ticks is a *plan* a kind gives its entities, `plan.New(name, func(a *plan.Actor) rule.Step {…})`,
of the same steps; both cast *effects* that hold for a while and give *commands* for their entity
(`a.Order(navigation.MoveTo{…})`), and a plan waits for the *facts* a plugin tells it
(`.Until[navigation.Arrived]()`) — the story is in [`doc/rule.md`](doc/rule.md). An effect turns
the knobs a plugin gives — components it only reads, like `steering.Steering` or a cell's
`cell.Ground`. A rule holds no Go code but its conditions; how entities are drawn is the one place
rules are Go (`render.Over`, `As`, `With`, `Show`, given to `world.Plugin.Draw`). Ready-made rules
live in `plugins/collision/hooks` and `plugins/vision/hooks`, whole, to `ctx.Hook`; navigation's
crowd is its own rules, StarCraft II's, over the moment `navigation.Touch`, which a game adds to
with `ctx.Hook` or replaces with `WithCrowd`. Behaviour is always written this way: a plugin perceives and carries
out, rules and plans say what to do when. What a player *wants* is a
command too: the plugin that defines the type (`navigation.MoveTo`, `selection.Select`) is a
`plugin.CommandHandler` that keeps its `control.Queue` and drains it in its own pass; the
`players` plugin is built over the command handlers and carries what a player's bindings, an AI
or a network issue, the world what the entities give themselves.

## Roles and wires

A *role* is a behaviour an entity plays — mortal, hasty, a trapdoor — not a group:
`rule.Role(name).Obeys(rules...)` fires the rules for those playing it alone, on top of their own
filters, and `Can(effect, trigger, label)` is an ability, the effect a player's key puts on its
selected units playing the role. A kind plays roles through one component, `rule.Plays(roles...)`,
a cell through its `cell.Entry.Roles`, and a role is hooked like a rule. A *wire* is a connection
by name — a lever and its trapdoors, a plate and its gate — with an entity of its own
(`world.Plugin.Wire`), its state an effect on that entity, so one effect serves any number of
wires. A key pulls it (`Wire.Key`) or flips it (`Wire.Switch`); a rule of what is wired to it
drives it (`OnWire`) or reads it (`WhileWire`):

```go
fx := s.world.Effects()
open := fx.Define("open", effect.Spec{effect.Alter(func(g *cell.Ground) { g.Kind = pit })})
on := fx.Define("on", effect.Spec{effect.Lasts(2 * time.Second)})
haste := fx.Define("haste", effect.Spec{effect.Lasts(3 * time.Second),
	effect.Alter(func(st *steering.Steering) { st.MaxSpeed *= 2 })})

mortal := rule.Role("mortal").Obeys(rule.On("fall in", rule.All, func(m *rule.Moment[unit.Standing]) rule.Step {
	return m.If(unit.Standing.Fallen, m.Order(world.Despawn{}))
}))
hasty := rule.Role("hasty").Can(haste, control.KeyPress{Key: control.KeyJ}, "Hasten the selected scouts")
trapdoor := rule.Role("trapdoor").Obeys(rule.On("open while on", rule.All, func(m *rule.Moment[cell.Now]) rule.Step {
	return m.WhileWire(on, m.Keep(open))
}))
plate := rule.Role("plate").Obeys(rule.On("press", rule.All, func(m *rule.Moment[cell.Now]) rule.Step {
	return m.If(cell.Now.Stood, m.OnWire(m.Apply(on)))
}))
west := s.world.Wire("west")

s.scout = units.Define("scout", land, profile, rule.Plays(mortal, hasty),
	comp.Tagged(s.selection.Tags().Selectable), comp.Tagged(s.player.Owner()))
s.player.Bind(west.Key(on, control.KeyPress{Key: control.Key1}, "Pull the west lever"))
s.player.Bind(s.selection.Abilities(hasty)...)
return ctx.Hook(mortal, hasty, trapdoor, plate)

// Spawn: a trapdoor on the west wire
cell.Entry{Kind: "boards", Cell: c, Roles: []*rule.Part{trapdoor}, Wired: west}
```

1 puts `on` on the west wire and every trapdoor wired to it opens for two seconds under whoever
stands there; a plate puts `on` on its own wire while it is stood on; J hastens the selected
scouts. A hundred levers are a hundred wires and these rules. A rule of the world as a whole, a
`clock.Moment`'s, takes no filter and obeys no role. [`examples/wire-demo`](examples/wire-demo) is
the whole program; the story is in [`doc/rule.md`](doc/rule.md).

## Kinds, spawning and saves

`kind.Define[Row](world.Kinds(), "name", kind.Spec{...})` says what an entity is: each component
`comp.Const(v)` (the same for all) or `comp.Load(func(row Row) T)` (read from that entity's row).
A unit over a board is defined through `board.NewUnits[Row](brd, size, at)`:
`units.Define(name, domain, steering, extra...)` derives `Position` and `At` (its cell) from the one point
`at` reads off a row and `Mover` and `Layers` from the one domain, then runs the world's roster —
what the plugins in the game bring by default (a `Collider`, a `Physics`, a `Velocity`) and what
they require (`At`, `Mover`, `Steering`), a Spec missing one panicking by plugin and reason.
`Spawn` puts entries on the world's roster with `Seed`; the engine spawns them only when
`Restore` loaded nothing; mid-game, components come and go through effects and the plugins' facts. Kinds
tell save files every component type their entities carry, so a game's own tags and state
survive a save without being registered anywhere else.

<a id="architecture"></a>
# Architecture

The packages form a strict acyclic graph; each imports only the layers below it. Every package
has a `doc.go` describing what it brings.

What is left to do is in [`doc/roadmap.md`](doc/roadmap.md).

| Package | Responsibility |
|:---|:---|
| [`camera`](camera/doc.go) | The contract of a view onto a world: screen conversion, culling, move and zoom, projections; the cameras live in `internal/camera` and come from the world |
| [`control`](control/doc.go) | The input vocabulary: `InputEvents`, `KeyEvent`, `ClickEvent`, `EventHandler`; commands and bindings: `Queue`, `Issued` (by a player or an entity), `Carrier`, `Binding`, `Command`, the triggers of bindings |
| [`render`](render/doc.go) | Drawing: `Renderer`, the `Composer` of a world view over `Source`s and its `Frame`, `Atlas` baked at `Close`, sprite drawers, cached and telemetry renderers; `Appearance` and the drawing rules (`Over`, `As`, `With`, `Show`) a renderer runs every frame through `Rules` |
| [`plugin`](plugin/doc.go) | The extension contract: `Plugin`, `Installer`, `CommandHandler`, `Serializable`, `PostLoader`, `Populator`, `Restorer`; the hosts a plugin runs the rules hooked on it with (`Rules`, `PairRules`, `StepRules`), the `Tick` they hand them, `Marks`, and what a moment is (`About`, `Met`, `Placed`, `Aimed`) |
| [`entity/tag`](entity/tag/doc.go) | Tag families: `Tags`, `Tag`, `Any`; a leaf |
| [`entity/kind`](entity/kind/doc.go) | What an entity is: `Spec`, `Const`/`Load` (`kind/comp`), `Define`, `Of`, `Registry` |
| [`entity`](entity/doc.go) | What every entity carries: `Base`, `Position`, `Velocity`, `Z`, `Layers`; the world re-exports them |
| [`clock`](clock/doc.go) | The tactical clock: game time as the sum of the simulation's steps, the tactical pause (Space), the tempo (] and [), `Simulate` for what a plugin's tick simulates, the phases, the `Moment` of a step with `At` and `Every` |
| [`rule/effect`](rule/effect/doc.go) | Temporary changes to entities — tags granted, components altered and restored — cast from anywhere, lasting in game time; the rules of the clock's moments |
| [`rule`](rule/doc.go) | Rules at a plugin's moments, in one vocabulary ([the story](doc/rule.md)): `On(name, filter, func(m *Moment[P]) Step)`, filters `All`, `Self`, `Between`, `Having`, the Moment's steps (`Apply`, `Keep`, `Unless`, `Order`, `OnWire`, `WhileWire`, `Playing`…); roles (`Role`, `Obeys`, `Can`, `Plays`) and wires (`Wire`, `Key`, `Switch`, `Wired`) |
| [`rule/plan`](rule/plan/doc.go) | What an entity does over time: `New(name, func(a *Actor) Step)` given to a kind (`OneOf`, `Steps`, `If`, `When`, `On`, `Until`, `Ask`), `Command`, the asks, `Mind`; run by the world |
| [`plugins/world`](plugins/world/doc.go) | The foundation: `Base`, the shared `Space` and camera, movement under the edge rules, kinds, `Seed`/`Populate`, `Despawn`, `Apply`/`Dispel` on the world itself, wires (`Wire`), the carrier of the commands entities give themselves, the entity renderer drawing as the rules given to `Draw` say (`Facing`); it runs the core's systems (the clock's, the plans', the effects'); its register of kinds and tags and its flat look in `plugins/world/internal` |
| [`plugins/world/steering`](plugins/world/steering/doc.go) | `Steering` profiles (knobs) and the `Course` asked of an entity through its `Helm`, carried out by the `System` each step; the commands an entity gives itself (`Away`, `Toward`, `Turn`); `Pace`, the ground's share of its speed; `Driven` for an entity steered by hand |
| [`plugins/world/view`](plugins/world/view/doc.go) | A `View` of the world with its `EntitySet`, refreshed by the `System` after movement |
| [`game`](game/doc.go) | What a game implements and receives: `Game`, `Stage`, `Scene`, `Scenes`, `Composition`, `Initializer`, `Runtime`, `Persistence` |
| [`plugins/collision`](plugins/collision/doc.go) | Collision over the world's space; `Collider`, `Physics`, `Meeting`, `Struck`; `Field`, the solid ground it asks of a board; the answer's arithmetic in `plugins/collision/internal/response` |
| [`plugins/collision/hooks`](plugins/collision/hooks/doc.go) | Ready-made rules: `ShowHits` with `HitOverlay` |
| [`plugins/vision`](plugins/vision/doc.go) | `Sight` cones (knobs) into `Sighted`; `Sighting` rules; `SightOutline` drawn |
| [`plugins/vision/hooks`](plugins/vision/hooks/doc.go) | Ready-made rules: `Flee`, `Chase`, `Search`, and the `Predator`/`Prey`/`Skittish`/`Threat` tags |
| [`plugins/board`](plugins/board/doc.go) | A square or hex grid with terrain kinds and occupancy over the world: the `Board` (the terrain, read and written), its `Layout` and `Map`, `NewUnits`; rules of `unit.Standing` and of `cell.Now`; its machinery in `plugins/board/internal`, nothing else imports it |
| [`plugins/board/cell`](plugins/board/cell/doc.go) | A cell as a place: `ID`, `Kind` and the `Kinds` a board holds, `Domain` (`Land`, `Water`, `Air`), the game's tags of places (`Family`, `Tag`, `Tags`), `Ground`, `Way`, `Crossing`, the moment `Now` (`Stood`); `TerrainMap`, the Layout's `Entry` (the roles a cell plays, the wire it is wired to), `Occupancy` (the board lets go of the gone every step) |
| [`plugins/board/unit`](plugins/board/unit/doc.go) | An entity on the board: the cell it is `At`, how it moves (`Mover`), where it stands at a step (`Standing`, `Fallen`) |
| [`plugins/board/grid`](plugins/board/grid/doc.go) | The topology: `Grid` (neighbours, `Toward`, cells under a box), `DefaultGrids` (square, hex), `Link`, `Shape` |
| [`plugins/board/look`](plugins/board/look/doc.go) | How a board is drawn: `Look`, `Dressing`, `Tile`, `FlatLook`, `Nothing`; the `Renderer` — composed every frame, in parallel, or once for a flat map, the grid over it |
| [`plugins/board/ground`](plugins/board/ground/doc.go) | What the board's ground is to the others: `Heights`, `Cover`, `Readied` |
| [`plugins/atmosphere`](plugins/atmosphere/doc.go) | The sky over a world on the world's clock: the calendar (`atmosphere/calendar` — days, seasons, the moon, the periods of the clock's rules), the light of the day (`atmosphere/sky` — the sun and the moon of the hour, the sky's colours, a frozen light: P, Shift+] and Shift+[), the celestial sphere (`atmosphere/celestial` — the sun's path, the moon's orbit and phase, the real stars turning round the pole), the climate (`atmosphere/climate` — zones from the equator to the pole, the weather going from one kind to the next: wind, clouds whose shadows drift over the ground, rain, snow; Shift+W changes it), what falls (`atmosphere/precipitation`), what the weather does to the board (`atmosphere/weathering` — snow lying, ice, what sways), the sky behind the world (`atmosphere/backdrop`) and the clouds' shadows over a flat world (`atmosphere/overcast`) |
| [`plugins/topography`](plugins/topography/doc.go) | A map in relief drawn on the GPU: the heights, the slopes' cost, the light and the shadows, the water and the ways on them, the sea to the horizon; the views — from above, isometric and in perspective, Tab goes round, V rides in a unit — with the cameras turned, tilted and fastened behind a unit. The commands (`View`, `Turn`, `LookOut`, `Raise`…) and `Relief` are its own; `relief` and `painter` are the vocabulary a game and the plugins share (`Climbing`, `MeanOfCells`, `Style`); the parts — relief, painter, water, terrain, hexes, billboards, cameras — are in `plugins/topography/internal` |
| [`plugins/navigation`](plugins/navigation/doc.go) | `MoveOrder` paths across a board, re-routing when terrain changes; right-click commands, and a unit's own (`MoveTo`, `Arrived`); route drawing; the crowd — rules over the moment `Touch` and the commands `StepAside`, `Detour`, `Pass`, `Hold`, `Settle`, `Stop`; its own crowd rules, as in StarCraft II: an ally standing makes way and stays aside, a group gathers round its point, strangers are gone round, nobody is stepped into water, off a cliff or into a wall |
| [`plugins/selection`](plugins/selection/doc.go) | `Select` into `Selected`; default bindings; the roles' `Abilities`; highlight renderer |
| [`plugins/players`](plugins/players/doc.go) | A carrier over the command handlers: players and their bindings, `Pan` and `Zoom`; whose a unit is (`players/owner`) — a player selects, orders and rides its own units alone |
| [`internal/steps`](internal/steps/doc.go) | The engine running the steps of rules and plans; `rule` and `rule/plan` are its faces |
| [`internal/engine`](internal/engine/doc.go) | The `Engine`: the window's loop (gogpu), one active Stage, persistence, input capture |
| [`gram`](doc.go) (public) | `Run`; the package you import. The root `doc.go` carries the concepts and the full package graph |

```
camera ──► render ──► plugin ──► rule ──► plugins/world ──► game ──► internal/engine ──► gram
control ───┘ (→ camera)                   │  ▲
                                          ▼  │
                     plugins/{collision, selection, vision} ──► plugins/board ──► plugins/navigation, plugins/*/hooks ──► plugins/players
```

Outside the module: [goke](https://github.com/kjkrol/goke) is the ECS every Stage runs on,
[aabbworld](https://github.com/kjkrol/aabbworld) the space, collisions and line of sight under the
world, [gogpu](https://github.com/gogpu/gogpu) the window, the loop and the GPU (WebGPU),
[astar](https://github.com/kjkrol/astar) the pathfinding.

<a id="performance"></a>
# ⏱️ Performance

One tick of the built-in plugins on an Intel i5-8265U (see [BENCHMARKS.md](BENCHMARKS.md#environment)),
0 allocs/op throughout once warm:

| Tick | Scene | Cost |
|:---|:---|---:|
| World: move every entity, rebuild the space | 5,000 boxes, 20×20 each, on a 4000×4000 torus | 199 µs |
| World + collisions | 524 boxes, 20×20 each, covering 20% of a 1024×1024 torus | 134 µs |
| World + collisions | 8,388 boxes, 5×5 each, covering 20% of the same torus | 2.9 ms |
| Vision: every observer scans its cone | 500 observers, 60° cones of radius 200 | 503 µs |

> **Deep dive**: every scene, what each benchmark measures, and how to reproduce, in
> [**BENCHMARKS.md**](BENCHMARKS.md).

```bash
make bench
```

# Relationship to goke and aabbworld

gram began as `goke`'s Ebitengine example and was extracted so the ECS stays free of GUI
dependencies while this integration evolves and versions on its own; it has since left Ebitengine
for WebGPU, drawing the whole picture on the GPU. The world, collisions and
line of sight are `aabbworld`'s; gram is where they meet an ECS and a screen.

<a id="documentation"></a>
# 📖 Documentation

- **API reference** on [pkg.go.dev](https://pkg.go.dev/github.com/kjkrol/gram).
- **Concepts and package graph** in the root [`doc.go`](doc.go); each package's own `doc.go`
  explains what it brings (see [Architecture](#architecture)).
- **Benchmarks** in [BENCHMARKS.md](BENCHMARKS.md); **changes** in [CHANGELOG.md](CHANGELOG.md).

# Credits

- The night sky's stars: the Yale Bright Star Catalogue, 5th revised edition (Hoffleit &
  Warren 1991), from the CDS, catalogue V/50 (`plugins/atmosphere/celestial/stars_gen.go`).
- The moon's face: NASA's Scientific Visualization Studio, CGI Moon Kit (Lunar Reconnaissance
  Orbiter LROC data) (`plugins/atmosphere/celestial/moon.png`).

# License

[MIT](LICENSE)
