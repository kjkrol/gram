# gram

<p align="center">
  <img src=".github/docs/img/logo.png" alt="gram logo" width="300">
  <br>
  <a href="https://go.dev"><img src="https://img.shields.io/badge/Go-1.27+-00ADD8?style=flat-square&logo=go" alt="Go Version"></a>
  <a href="https://pkg.go.dev/github.com/kjkrol/gram"><img src="https://img.shields.io/badge/GoDoc-Reference-007d9c?style=flat-square&logo=go" alt="GoDoc"></a>
  <a href="https://opensource.org/licenses/MIT"><img src="https://img.shields.io/badge/License-MIT-yellow.svg?style=flat-square" alt="License"></a>
  <a href="https://app.codecov.io/gh/kjkrol/gram"><img src="https://img.shields.io/codecov/c/github/kjkrol/gram?style=flat-square&logo=codecov" alt="Codecov Coverage"></a>
  <a href="https://github.com/kjkrol/gram/actions"><img src="https://github.com/kjkrol/gram/actions/workflows/go.yml/badge.svg" alt="Go Quality Check"></a>
</p>

**gram** is a modular 2D game engine for Go. A game is a set of named **Stages**, each with its
own entity-component world and its own **Scenes**; an engine drives the active Stage through
[Ebitengine](https://ebitengine.org/)'s `Update`/`Draw`/`Layout` loop on the
[goke](https://github.com/kjkrol/goke) ECS. Everything beyond the tick loop is a **plugin**: the
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
- **Behaviors are plain functions.** Game logic reacting to what a plugin finds is registered on
  that plugin and run inside its own pass; the payload type says whose it is, and a plugin
  refuses another's, so registering in the wrong place is an error, never a silent no-op.
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

**Prerequisites:** Go 1.27+ and the
[Ebitengine system dependencies](https://ebitengine.org/en/documents/install.html) (a C compiler
and a few system libraries; Ebitengine uses cgo on most platforms).

<a id="features"></a>
# ✨ Key Features

| Capability | Package | What you get |
|:---|:---|:---|
| **Stages and Scenes** | `game` | Named Stages with their own ECS and lifecycle (`Init`/`Restore`/`Spawn`/`Update`); Scenes with layered renderers and input; a live Composition of what is shown and which Scene is active |
| **Plugins and behaviors** | `plugin` | The one extension contract; behaviors built by the hosting plugin (`Between`, `Each`, `Every`) and run in its own pass |
| **World** | `plugins/world` | Every entity's `Base` (position, velocity, kind, capabilities); movement under stop, wrap or open edges; the shared spatial index and camera; spawning from kinds |
| **Kinds** | `plugins/world/kind` | `Define` a kind from a `Spec` of `Const` and `Load` components; `Entry` rows onto the roster |
| **Collisions** | `plugins/collision` | A `CollisionSystem` over the world's space: `Collider` to take part, `Physics` to bounce and be pushed apart, a `ShapeTest` to refine, `Meeting`/`Struck` for behaviors |
| **Sight** | `plugins/vision` | A `Sight` cone scanned each tick into `Seen`, nearest first; `Sighting` behaviors per observer; drawn outlines; in a Quasi3D world the eye looks over walls, forests and hills by height |
| **Board and navigation** | `plugins/board`, `plugins/navigation` | Square or hex grid with terrain and occupancy; `MoveOrder` paths that re-route when terrain changes |
| **Selection** | `plugins/selection` | A `Select` command into a `Selected` tag, with default bindings (click, marquee, shift-add) and a highlight renderer |
| **Players** | `plugins/players` | Who acts: a camera and view per player, the plugins' default bindings gathered and bound, input translated into typed commands the defining plugins drain |
| **Persistence** | `game.Persistence` | Save, load and list the active Stage's ECS and every tracked value by name |
| **Camera and rendering** | `camera`, `render` | A wrap-aware camera with zoom and pan; an atlas baked at `Close`, quad batching, cached and telemetry renderers |

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

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/kjkrol/aabbworld"
	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram"
	"github.com/kjkrol/gram/control"
	"github.com/kjkrol/gram/game"
	"github.com/kjkrol/gram/plugin"
	"github.com/kjkrol/gram/plugins/collision"
	"github.com/kjkrol/gram/plugins/collision/behavior"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/plugins/world/kind"
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
	stats     behavior.ContactStats
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
	if err := a.collision.RegisterBehavior(
		collision.Between(plugin.Any, plugin.Any, behavior.CountContacts(&a.stats)),
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

func (v *view) Layers() []render.Renderer {
	atlas := render.NewAtlas()
	atlas.RegisterAt(v.arena.boxes.SpriteID(), boxSize, render.Solid(color.RGBA{R: 90, G: 200, B: 110, A: 255}))
	atlas.Close()
	v.arena.world.WithRenderer(atlas)

	count := func() int { return v.arena.world.Res.Telemetry.Count }
	return []render.Renderer{
		render.SolidBackground{Color: color.RGBA{R: 30, G: 30, B: 30, A: 255}},
		v.arena.world.Renderer(),
		render.NewTelemetryRenderer(&v.tps.Ticks, count, &v.arena.stats.Counter),
	}
}

func (v *view) HandleEvents(events *control.InputEvents, runtime game.Runtime, _ game.Composition) {
	for _, k := range events.KeyEvents {
		if k.Action == control.ActionPress && k.Key == ebiten.KeyEscape {
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
| [`scenes-demo`](examples/scenes-demo) | A menu Stage switching into a gameplay Stage, a modal Scene over the ticking world, a non-focusable HUD | `make demo-scenes` |
| [`navigation-demo`](examples/navigation-demo) | A board with terrain, units selected by click and marquee, right-click move orders along re-routing paths, holes the planner avoids and H opens under the units | `make demo-navigation` |
| [`navigation-hex-demo`](examples/navigation-hex-demo) | The same on a hex board: hex cells and route arrows at 60°, a wall of merged hex bodies | `make demo-navigation-hex` |
| [`navigation-vision-demo`](examples/navigation-vision-demo) | Navigated units with sight cones that stop at walls and fade in forests, and a hawk that flies over both and sees through the forest | `make demo-navigation-vision` |
| [`navigation-vision-hex-demo`](examples/navigation-vision-hex-demo) | The same sight cones and hawk on a hex board | `make demo-navigation-vision-hex` |
| [`island-demo`](examples/island-demo) | An island of fields, forests, slow hills and slower mountains in a sea that drowns whoever is pushed in, larger than the window, under a zooming, panning camera; units with sight cones and a hawk on the Air plane | `make demo-island` |
| [`island-isometric-demo`](examples/island-isometric-demo) | The same island in a Quasi3D world through an isometric camera: hills and mountains stand up with sloping sides, forests are blocks, units billboards, and the hawk 40 up looks over what a walker's cone climbs and stops at | `make demo-island-isometric` |
| [`effect-demo`](examples/effect-demo) | An ice witch under orders turns the ground round her into snow and the lake into ice, fast on her own snow; it thaws behind her, a walker follows her trail while it lasts and slips on it, a boat with weak brakes sails onto the ice it saw coming and is frozen still and pale until it melts — all of it effects | `make demo-effect` |
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

## Plugins and behaviors

A plugin's `Install` only queues ECS wiring; the engine flushes it all in one `ecs.Setup` after
the Stage's `Init`, which is what lets `Restore` decide fresh-spawn or restore before the ECS
commits to either. Game logic that reacts to what a plugin finds is a *behavior*, built with the
plugin's own constructors: `collision.Between(a, b, fn)` for every pair it meets where one entity
carries tag `a` and the other `b` (`plugin.Any` as the wildcard), `board.Each[T](fn)` for every
entity carrying `T`, `world.Every(fn)` for every entity the payload's host visits. A tag
is a bit of a family — one `plugin.Tags[F]` component per family, named through `Kinds.DefineTag`,
given to a kind with `comp.Tagged` — so markers cost no component types of their own. Ready-made
behaviors live in `plugins/collision/behavior` and `plugins/vision/behavior`; which tags they run
between is the registration's to say. What a player *wants* goes the other way, as a command:
the plugin that defines the type (`navigation.MoveTo`, `selection.Select`) is a `plugin.CommandHandler`
that keeps its `control.Queue` and drains it in its own pass; the `players` plugin is built over
the command handlers and carries what a player's bindings, an AI or a network issue.

## Kinds, spawning and saves

`kind.Define[Row](world.Kinds(), "name", kind.Spec{...})` says what an entity is: each component
`comp.Const(v)` (the same for all) or `comp.Load(func(row Row) T)` (read from that entity's row).
A unit over a board is defined through `board.NewUnits[Row](brd, size, at)`:
`units.Define(name, domain, steering, extra...)` derives `Position` and `Cell` from the one point
`at` reads off a row and `Mover` and `Layers` from the one domain, then runs the world's roster —
what the plugins in the game bring by default (a `Collider`, a `Physics`, a `Velocity`) and what
they require (`Cell`, `Mover`, `Steering`), a Spec missing one panicking by plugin and reason.
`Spawn` puts entries on the world's roster with `Seed`; the engine spawns them only when
`Restore` loaded nothing. `Attach` and `Detach` are the mid-game counterparts of `Const`. Kinds
tell save files every component type their entities carry, so a game's own tags and state
survive a save without being registered anywhere else.

<a id="architecture"></a>
# Architecture

The packages form a strict acyclic graph; each imports only the layers below it. Every package
has a `doc.go` describing what it brings.

Design notes sit in [`doc/`](doc): [`roadmap.md`](doc/roadmap.md) is the map of what is done and
what comes next, [`movement.md`](doc/movement.md) the reasoning behind movement, terrain and
effects, [`views.md`](doc/views.md) where players and networking are headed.

| Package | Responsibility |
|:---|:---|
| [`camera`](camera/doc.go) | The view onto a world: screen conversion, culling, move and zoom; wrap-aware |
| [`control`](control/doc.go) | The input vocabulary: `InputEvents`, `KeyEvent`, `ClickEvent`, `EventHandler`; commands and bindings: `Queue`, `Issued`, `Binding`, `Command`, the triggers |
| [`render`](render/doc.go) | Drawing primitives: `Renderer`, `Atlas` baked at `Close`, `QuadBatch`, sprite drawers, cached and telemetry renderers |
| [`plugin`](plugin/doc.go) | The extension contract: `Plugin`, `Installer`, `Tick`, `Behavior`, `Tag`/`Tags`/`Any`, `Marks`, `Serializable`, `PostLoader`, `Populator` |
| [`plugin/host`](plugin/host/doc.go) | A plugin author's package: `Pair`/`Each`/`Every` behind a plugin's typed constructors, `PairHost` and `EachHost` that run them |
| [`plugins/world/kind`](plugins/world/kind/doc.go) | What an entity is: `Spec`, `Const`/`Load`, `Define`, `Of`, `Registry` |
| [`plugins/world`](plugins/world/doc.go) | The foundation: `Base`, the shared `Space` and camera, movement under the edge rules, kinds, `Seed`/`Populate`, `Attach`/`Detach`, the entity renderer |
| [`game`](game/doc.go) | What a game implements and receives: `Game`, `Stage`, `Scene`, `Scenes`, `Composition`, `Initializer`, `Runtime`, `Persistence` |
| [`plugins/collision`](plugins/collision/doc.go) | The `CollisionSystem` over the world's space; `Collider`, `Physics`, `ShapeTest`, `Meeting`, `Struck` |
| [`plugins/collision/behavior`](plugins/collision/behavior/doc.go) | `CountContacts`, `ShowHits` with `HitOverlay`, `LogContacts` |
| [`plugins/vision`](plugins/vision/doc.go) | `Sight` cones into `Seen`; `Sighting` behaviors; `SightOutline` drawn |
| [`plugins/vision/behavior`](plugins/vision/behavior/doc.go) | `Flee`, `Chase`, and the `Predator`/`Prey`/`Skittish`/`Threat` tags |
| [`plugins/board`](plugins/board/doc.go) | A square or hex grid with terrain kinds and occupancy over the world |
| [`plugins/effects`](plugins/effects/doc.go) | Temporary changes to entities — tags granted, components altered and restored — cast from anywhere |
| [`plugins/navigation`](plugins/navigation/doc.go) | `MoveOrder` paths across a board, re-routing when terrain changes; right-click commands; route drawing |
| [`plugins/selection`](plugins/selection/doc.go) | `Select` into `Selected`; default bindings; highlight renderer |
| [`plugins/players`](plugins/players/doc.go) | A carrier over the command handlers: players and their bindings, `Pan` and `Zoom` |
| [`internal/engine`](internal/engine/doc.go) | The `Engine`: the Ebitengine loop, one active Stage, persistence, input capture |
| [`gram`](doc.go) (public) | `Run`; the package you import. The root `doc.go` carries the concepts and the full package graph |

```
camera ──► render ──► plugin ──► plugins/world/kind ──► plugins/world ──► game ──► internal/engine ──► gram
control ───┘ (→ camera)                                   │  ▲
                                                          ▼  │
                     plugins/{collision, selection, vision, effects} ──► plugins/board ──► plugins/navigation, plugins/*/behavior ──► plugins/players
```

Outside the module: [goke](https://github.com/kjkrol/goke) is the ECS every Stage runs on,
[aabbworld](https://github.com/kjkrol/aabbworld) the space, collisions and line of sight under the
world, [Ebitengine](https://ebitengine.org/) the loop and the drawing,
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
dependencies while this integration evolves and versions on its own. The world, collisions and
line of sight are `aabbworld`'s; gram is where they meet an ECS and a screen.

<a id="documentation"></a>
# 📖 Documentation

- **API reference** on [pkg.go.dev](https://pkg.go.dev/github.com/kjkrol/gram).
- **Concepts and package graph** in the root [`doc.go`](doc.go); each package's own `doc.go`
  explains what it brings (see [Architecture](#architecture)).
- **Benchmarks** in [BENCHMARKS.md](BENCHMARKS.md); **changes** in [CHANGELOG.md](CHANGELOG.md).

# License

[MIT](LICENSE)
