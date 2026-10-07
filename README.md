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
  plugin's own pass; the moment's type says whose it is, and nobody hands it over: a rule is a
  role's, a role is played, and the engine gives it to the plugin catching its moment — one no
  plugin in use catches is an error, never a silent no-op. A command says what a lever drives.
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
| **Plugins and rules** | `plugin` | The one extension contract; rules run in the pass of the plugin that catches their moment, pairs too, by its hosts (`Rules`, `PairRules`, `StepRules`) with a `Tick` |
| **Behaviour** | `rule` | One vocabulary: rules at a plugin's moments; roles a kind, a kind of cell or a plugin plays — the rules they obey, handed by the engine to whichever plugin catches their moments; commands written as sentences, for entities found by name or group — a lever, a plate, a switch and what they drive; plans a kind's entities follow, effects that hold, commands an entity gives itself as a player would, facts plugins tell it |
| **World** | `plugins/world` | Every entity's `Base` (position, velocity, kind, capabilities); movement under stop, wrap or open edges; the shared spatial index and camera; spawning from kinds, before the game and during it (`Spawn`); `Config.Heights` for a world with heights |
| **Steering and views** | `plugins/world/steering`, `plugins/world/view` | A `Steering` profile turned into heading and speed each tick; a `View` of what a camera sees |
| **Kinds** | `entity/kind` | `Define` a kind from a `Spec` of `Const` and `Load` components; `Entry` rows onto the roster |
| **Collisions** | `plugins/collision` | Collision over the world's space: `Collider` to take part, `Physics` to bounce and be pushed apart, `Meeting`/`Struck` for rules |
| **Shooting** | `plugins/bullet` | Shots as entities a `Shoot` fires from a unit's muzzle, flown past the step cap and swept by collision, or thrown in an arc; `Landing`, `Resting` and `Blast` for rules; a weapon is the game's rules and effects over them |
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
plugin counting their contacts, and one Scene showing them: a `ui` screen of the world's picture
through the player's camera, a backdrop under it and a telemetry line over it. This is
[`examples/minimal`](examples/minimal/main.go); run it with `make demo-minimal`.

```go
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
| [`scenes-demo`](examples/scenes-demo) | A menu Stage switching into a gameplay Stage whose `ui` screen has the world, a line of keys and a modal panel over the ticking world, open or closed as the game was saved | `make demo-scenes` |
| [`navigation-demo`](examples/navigation-demo) | A board with terrain, units selected by click and marquee, right-click move orders along re-routing paths, holes the planner avoids and H opens under the units | `make demo-navigation` |
| [`navigation-hex-demo`](examples/navigation-hex-demo) | The same on a hex board: hex cells and route arrows at 60°, a wall of solid hex cells | `make demo-navigation-hex` |
| [`navigation-vision-demo`](examples/navigation-vision-demo) | Navigated units with sight cones that stop at walls and fade in forests, and a hawk that flies over both and sees through the forest | `make demo-navigation-vision` |
| [`navigation-vision-hex-demo`](examples/navigation-vision-hex-demo) | The same sight cones and hawk on a hex board | `make demo-navigation-vision-hex` |
| [`board`](examples/board) | The island on the simple map: a flat world whose board draws itself from its kinds' colours, the streams, rivers, roads and bridges as plain bands; units walk from stop to stop over the roads with sight cones, a day goes by over the flat map — tiles and units tinted by the hour, clouds' shadows over the screen, rain and snow, snow lying and shores freezing in winter | `make demo-board` |
| [`board-topography`](examples/board-topography) | The same island in relief through the topography: a range of peaks and a plateau lit by the sun, sea cliffs, streams and rivers whose water runs and falls, roads over bridges, slower up the slopes and routed round them, the ground shaped under the cursor; seen isometrically, from above or in perspective, Tab goes round; units billboards, the hawk 40 up looking over what a walker's cone climbs and stops at; a day and the weather going by, snow and ice in winter | `make demo-board-topography` |
| [`board-atlas`](examples/board-atlas) | A small flat board drawn from the game's own atlas: striped grass, rippled water, a cobbled road, tree tops — sprites the game draws for its kinds — and a road laid as a way; units walk corner to corner | `make demo-board-atlas` |
| [`effect-demo`](examples/effect-demo) | An ice witch under orders turns the ground round her into snow and the lake into ice, fast on her own snow; it thaws behind her, a walker follows her trail while it lasts and slips on it, a boat with weak brakes sails onto the ice it saw coming and is frozen still until it melts, each kind in its own frozen look, and F freezes whoever the cursor points at — the states effects, snow and ice covers on cells that stay what they are | `make demo-effect` |
| [`dialog-demo`](examples/dialog-demo) | A conversation: the traveller walked up to the host is seen and greeted in a window above the host, answers one of three ways and the host shows what it made of it; walked away, the hello is gone — rules and effects for what happens, `ui` elements pinned to the host `Under` the effects, buttons giving commands | `make demo-dialog` |
| [`bullet-demo`](examples/bullet-demo) | A soldier on WSAD shoots: F fires a round the way it faces, over the low wall and into the high one, wounding the wanderer it strikes and taking a wounded one; G throws a grenade at the cursor in an arc over the high wall, which lies with a spark on it and bursts, wounding everyone within two cells and a half — the shots are the bullet plugin's, what they do is rules and effects | `make demo-bullet` |
| [`trapdoor-demo`](examples/trapdoor-demo) | Two levers and two strips of trapdoors across a meadow: wanderers walk to and fro over both, 1 and 2 pull a lever and its trapdoors open under whoever stands on them, the player's scouts too; J hastens the selected scouts to get clear — a lever a command for the group of cells that is its strip, the haste one for the selected | `make demo-trapdoor` |
| [`pressure-plate-demo`](examples/pressure-plate-demo) | The same meadow with two pressure plates in place of the levers: walk a scout onto a plate and, while someone stands on it and a second after, its trapdoors are open under whoever is on them — a plate a cell with a name, which stood on sets off the command that names it | `make demo-pressure-plate` |
| [`wire-demo`](examples/wire-demo) | The same meadow under three commands, each a sentence saying what it does, whom it is for and who sets it off: 1 opens the west trapdoors for two seconds, and so does a selected scout pulling the lever beside it (U); a scout standing on the plate in the yard opens the east ones; G flips the gate until G again — the trapdoors and the gate groups of cells, the lever and the plate cells with names, playing roles that only trigger; everyone plays mortal and falls into an open trapdoor, and J hastens the selected scouts, which play hasty, never the porters | `make demo-wire` |
| [`split-screen-demo`](examples/split-screen-demo) | Two players at one keyboard: red drives its block with WSAD, blue with the arrows — each block its player's by the owner tag — each through a camera of its own in its half of the screen, and a minimap at the bottom shows the whole arena through a camera nobody drives — one `ui` screen: two feeds in `Columns`, a line between, the minimap anchored over them | `make demo-split-screen` |
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

A game does not write those by hand: it defines its Stage a section at a time with
[`game/stage`](game/stage/doc.go), always in the same order — a chain out of order does not
compile, a section the game has no use for is left out, and what is defined in the wrong section
is refused:

```go
stage.New("meadow").
	Plugins(s.usePlugins).      // ctx.UseWorld, ctx.Use
	Players(s.definePlayer).    // the players, the plugins' default keys
	Effects(s.defineEffects).   // the states
	Rules(s.defineRules).       // the roles, the rules, the plans
	Commands(s.defineCommands). // what can be asked for
	Cells(s.defineCells).       // the kinds of cells, the roles their cells play
	Kinds(s.defineKinds).       // the kinds of units
	Controls(s.bindKeys).       // the game's own keys, each a command
	Scenes(s.defineScenes).     // the scenes
	Layout(s.layOut).           // a fresh game's board
	Units(s.placeUnits).        // a fresh game's units
	Update(s.update)            // the tick; hands back the game.Stage
```

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
roles := s.world.Roles()
roles.Define(PreyRole)
roles.Define(PredatorRole,
	rule.Then[collision.Meeting]("caught", rule.Other(roles.Named(PreyRole)), rule.ForOther(rule.Order(world.Despawn{}))))
kind.Define[body](kinds, HunterKind, kind.Spec{…, rule.Plays(roles.Named(PredatorRole))})
```

fires for every pair a predator meets whose other plays `prey`, and has the prey give itself a
`Despawn`; `rule.All` would fire for every pair, `rule.Self(tag)` for every entity carrying a tag.
Nobody hands the rule to collision: once `Init` returns, the engine gives the rules of every role
somebody plays to the plugin in use that catches their moment — a `Meeting` to collision — and a
rule none catches is an error, never a silent no-op. A tag is a
bit of a family — one `tag.Tags[F]` component per family, named through `Kinds.DefineTag`, given
to a kind with `comp.Tagged` — so markers cost no component types of their own. What lasts over
ticks is a *plan* a kind gives its entities, `s.world.Plans().Define(name, func(a *plan.Actor) rule.Step {…})`,
of the same steps; both cast *effects* that hold for a while and give *commands* for their entity
(`a.Order(navigation.MoveTo{…})`), and a plan waits for the *facts* a plugin tells it
(`.Until[navigation.Arrived]()`) — the story is in [`doc/rule.md`](doc/rule.md). An effect turns
the knobs a plugin gives — components it only reads, like `steering.Steering` or a cell's
`cell.Ground`. A rule holds no Go code but its conditions; how entities are drawn is the one place
rules are Go, declared on the world's atlas in a scene's Layers (`world.Plugin.NewAtlas`: a look
`Under` an effect, `Turning`, `Facing`). A plugin ships no
ready-made reactions — it gives moments and their conditions (`unit.Standing.Fallen`,
`vision.Sighting.Closing`), and the game says what follows; navigation's
crowd is its own rules, StarCraft II's, over the moment `navigation.Touch`, which a game adds to
with its roles or replaces with `WithCrowd`. Behaviour is always written this way: a plugin perceives and carries
out, rules and plans say what to do when. What a player *wants* is a
command too: the plugin that defines the type (`navigation.MoveTo`, `selection.Select`) is a
`plugin.CommandHandler` that keeps its `control.Queue` and drains it in its own pass; the
`players` plugin is built over the command handlers and carries what a player's bindings, an AI
or a network issue, the world what the entities give themselves.

## Roles and commands

A *role* is a behaviour an entity plays — mortal, hasty, a plate — not a group:
`s.world.Roles().Define(name, rules...)` fires the rules for those playing it alone, on top of their own
filters. **Defining registers and hands nothing back**: effects, roles, plans, commands and kinds are
each defined under a name in the Stage's world and taken back with `Named(name)` wherever they are
built on — a game keeps its names as constants, each with a suffix saying what it names (`Ef` an
effect, `Cmd` a command, `Role`, `Plan`, `Kind` a kind of unit, `Cell` a kind of cell), and a
Stage's struct holds its plugins and nothing else. Each Stage has its own registers. A kind plays roles through one component, `rule.Plays(roles...)`, a cell through its kind
(`cell.Kinds.Define(name, kind, roles...)`) or one cell of the Layout alone (`cell.Entry.Plays`),
the world and the atmosphere through their own `Plays` —
for the rules of a `clock.Moment` and of the weather. A plugin is an entity of the world too
(`world.Self`), called by its name: it carries the plugin's knobs and the effects it is under, so
a state of the sky is an effect on the atmosphere — `rule.Cast(bloodMoon).On(s.atmosphere)`,
read back by `rule.While(s.atmosphere, bloodMoon, step)`. What somebody asks for is a *command*, and
one about an effect is a sentence: put it on (`rule.Cast`), take it off (`Lift`) or switch it
(`Toggle`), for the entities bearing a name, those in a group, the world itself, the player's
selected units or the one pointed at (`On`), set off by the entity named (`By`):

```go
// the names, in one place: a name mistyped does not compile
const (
	OpenEf, HasteEf                   = "open", "haste"
	MortalRole, HastyRole, PlateRole  = "mortal", "hasty", "plate"
	OpenWestCmd, OpenEastCmd          = "open west", "open east"
	HastenCmd                         = "hasten"
	ScoutKind                         = "scout"
	PlateCell, BoardsCell             = "plate", "boards"
)

fx, roles, cmds := s.world.Effects(), s.world.Roles(), s.world.Commands()

// Effects
fx.Define(OpenEf, effect.Spec{effect.Lasts(2 * time.Second),
	effect.Alter(func(g *cell.Ground) { g.Kind = pit })})
fx.Define(HasteEf, effect.Spec{effect.Lasts(3 * time.Second),
	effect.Alter(func(st *steering.Steering) { st.MaxSpeed *= 2 })})

// Rules
roles.Define(MortalRole, rule.Then[unit.Standing]("fall in", rule.All,
	rule.If(unit.Standing.Fallen, rule.Order(world.Despawn{}))))
roles.Define(HastyRole)
roles.Define(PlateRole, rule.Then[cell.Now]("press", rule.All,
	rule.If(cell.Now.Stood, rule.Trigger())))
kinds.Define(PlateCell, cell.Kind{Cost: 1, Allows: cell.Land},
	roles.Named(PlateRole)) // every cell laid as a plate plays it

// Commands
cmds.Define(OpenWestCmd, rule.Cast(fx.Named(OpenEf)).On(entity.Group("west trapdoors")))
cmds.Define(OpenEastCmd, rule.Cast(fx.Named(OpenEf)).On(entity.Group("east trapdoors")).By(entity.Named("plate")))
cmds.Define(HastenCmd, rule.Cast(fx.Named(HasteEf)).On(s.selection.Selected(roles.Named(HastyRole))))

// Kinds
units.Define(ScoutKind, land, profile, rule.Plays(roles.Named(MortalRole), roles.Named(HastyRole)))

// Controls
s.player.Bind(
	control.Give(control.KeyPress{Key: control.Key1}, "Pull the west lever", cmds.Named(OpenWestCmd)),
	control.Give(control.KeyPress{Key: control.KeyJ}, "Hasten the selected scouts", cmds.Named(HastenCmd)))

// Units: a unit is told whose it is and that it may be selected, as it is made
s.world.Seed(units.Named(ScoutKind).Entry(row).Told(players.Give{To: s.player.ID}, selection.Allow{}))

// Layout: the cells are called what the commands call them
cell.Entry{Kind: BoardsCell, Cell: c, Group: "west trapdoors"}
cell.Entry{Kind: PlateCell, Cell: p, Name: "plate"}
```

1 opens every cell in the group "west trapdoors" for two seconds under whoever stands there; the
plate, stood on, sets off the command that names it; J hastens the selected scouts. A hundred
levers are a hundred commands and these rules, and an AI gives them as a key does
(`players.Issue`). A rule of the world as a whole, a `clock.Moment`'s, takes no filter and obeys
no role. [`examples/wire-demo`](examples/wire-demo) is the whole program; the story is in
[`doc/rule.md`](doc/rule.md).

## Kinds, spawning and saves

`kind.Define[Row](world.Kinds(), "name", kind.Spec{...})` says what an entity is: each component
`comp.Const(v)` (the same for all) or `comp.Load(func(row Row) T)` (read from that entity's row).
A unit over a board is defined through `board.NewUnits[Row](brd, size, at)`:
`units.Define(name, domain, steering, extra...)` derives `Position` and `At` (its cell) from the one point
`at` reads off a row and `Mover` and `Layers` from the one domain, then runs the world's roster —
what the plugins in the game bring by default (a `Collider`, a `Physics`, a `Velocity`) and what
they require (`At`, `Mover`, `Steering`), a Spec missing one panicking by plugin and reason.
`Spawn` puts entries on the world's roster with `Seed`; the engine spawns them only when
`Restore` loaded nothing; mid-game the command `world.Spawn{Entry}` adds an entity of a kind the
same way, and components come and go through effects and the plugins' facts. Kinds
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
| [`render`](render/doc.go) | Drawing: `Renderer`, the `Composer` of a world view over `Source`s and its `Frame`, a `Feed` of the world through a camera as a picture, `Atlas` baked at `Close`, sprite drawers, cached and telemetry renderers; `Appearance` and the drawing rules (`Over`, `As`, `With`, `Show`) a renderer runs every frame through `Rules` |
| [`plugin`](plugin/doc.go) | The extension contract: `Plugin`, `Installer`, `CommandHandler`, `Serializable`, `PostLoader`, `Populator`, `Restorer`; the hosts a plugin runs the rules of its moments with (`Rules`, `PairRules`, `StepRules`), the `Tick` they hand them, `Marks`, and what a moment is (`About`, `Met`, `Placed`, `Aimed`) |
| [`entity/tag`](entity/tag/doc.go) | Tag families: `Tags`, `Tag`, `Any`; a leaf |
| [`entity/kind`](entity/kind/doc.go) | What an entity is: `Spec`, `Const`/`Load` (`kind/comp`), `Define`, `Of`, `Registry` |
| [`entity`](entity/doc.go) | What every entity carries: `Base`, `Position`, `Velocity`, `Z`, `Layers`; the world re-exports them |
| [`clock`](clock/doc.go) | The tactical clock: game time as the sum of the simulation's steps, the tactical pause (Space), the tempo (] and [), `Simulate` for what a plugin's tick simulates, the phases, the `Moment` of a step with `At` and `Every` |
| [`rule/effect`](rule/effect/doc.go) | Temporary changes to entities — tags granted, components altered and restored — cast from anywhere, lasting in game time; the rules of the clock's moments |
| [`rule`](rule/doc.go) | Rules at a plugin's moments, in one vocabulary ([the story](doc/rule.md)): `Then[P](name, filter, step)` and `On`, filters `All`, `Self`, `Between`, `Having`, the steps (`Apply`, `Keep`, `Unless`, `Order`, `Trigger`, `Playing`…); roles (`Role`, `Obeys`, `Plays`) and commands about effects (`Cast`, `Lift`, `Toggle` with `On`, `By`, `For`) |
| [`rule/plan`](rule/plan/doc.go) | What an entity does over time: `New(name, func(a *Actor) Step)` given to a kind (`OneOf`, `Steps`, `If`, `When`, `On`, `Until`, `Ask`), `Command`, the asks, `Mind`; run by the world |
| [`plugins/world`](plugins/world/doc.go) | The foundation: `Base`, the shared `Space` and camera, movement under the edge rules, kinds, `Seed`/`Populate`, `Spawn`, `Despawn`, the commands about effects for entities named, grouped and the world itself, the carrier of the commands entities give themselves, the entity renderer drawing as the rules given to `Draw` say (`Facing`); it runs the core's systems (the clock's, the plans', the effects'); its register of kinds and tags and its flat look in `plugins/world/internal` |
| [`plugins/world/steering`](plugins/world/steering/doc.go) | `Steering` profiles (knobs) and the `Course` asked of an entity through its `Helm`, carried out by the `System` each step; the commands an entity gives itself (`Away`, `Toward`, `Turn`); `Pace`, the ground's share of its speed; `Driven` for an entity steered by hand |
| [`plugins/world/view`](plugins/world/view/doc.go) | A `View` of the world with its `EntitySet`, refreshed by the `System` after movement |
| [`game`](game/doc.go) | What a game implements and receives: `Game`, `Stage`, `Scene`, `Scenes`, `Composition`, `Initializer`, `Runtime`, `Persistence` |
| [`ui`](ui/doc.go) | A scene's screen composed of elements in a tree: `Layers` over one another, `Columns`/`Rows` split by `Share`, `Fixed`, `Fit`, anchors (`TopLeft`…`BottomRight`); `Panel`, `Label`, `Image` (a `render.Feed` of the world, any `render.Surface`), `Window`, `Button` (it gives commands), `Layer` (a screen renderer); shapes (`Circle`, `Polygon`); elements pinned to entities `Under` an effect or `On` a name, standing by them, pointing at them off the screen; the input routed to buttons, modal windows and the players |
| [`game/stage`](game/stage/doc.go) | A Stage defined a section at a time, in one order the compiler keeps: `New(name).Plugins(…).Players(…)…Update(…)`; [`plugin/section`](plugin/section/section.go) names the parts, for a plugin refusing what is defined out of its place |
| [`plugins/collision`](plugins/collision/doc.go) | Collision over the world's space; `Collider`, `Physics`, `Meeting`, `Struck`; `Field`, the solid ground it asks of a board; the answer's arithmetic in `plugins/collision/internal/response` |
| [`plugins/vision`](plugins/vision/doc.go) | `Sight` cones (knobs) into `Sighted`; `Sighting` rules; `SightOutline` drawn |
| [`plugins/board`](plugins/board/doc.go) | A square or hex grid with terrain kinds and occupancy over the world: the `Board` (the terrain, read and written), its `Layout` and `Map`, `NewUnits`; rules of `unit.Standing` and of `cell.Now`; its machinery in `plugins/board/internal`, nothing else imports it |
| [`plugins/board/cell`](plugins/board/cell/doc.go) | A cell as a place: `ID`, `Kind` and the `Kinds` a board holds, `Domain` (`Land`, `Water`, `Air`), `Ground`, `Way`, `Crossing`, the moment `Now` (`Stood`); `TerrainMap`, the Layout's `Entry` (the roles a cell plays, its name and its group), `Occupancy` (the board lets go of the gone every step) |
| [`plugins/board/unit`](plugins/board/unit/doc.go) | An entity on the board: the cell it is `At`, how it moves (`Mover`), where it stands at a step (`Standing`, `Fallen`) |
| [`plugins/board/grid`](plugins/board/grid/doc.go) | The topology: `Grid` (neighbours, `Toward`, cells under a box), `DefaultGrids` (square, hex), `Link`, `Shape` |
| [`plugins/board/look`](plugins/board/look/doc.go) | How a board is drawn: `Look`, `Dressing`, `Tile`, `FlatLook`, `Nothing`; the `Renderer` — composed every frame, in parallel, or once for a flat map, the grid over it |
| [`plugins/board/ground`](plugins/board/ground/doc.go) | What the board's ground is to the others: `Heights`, `Cover`, `Readied` |
| [`plugins/atmosphere`](plugins/atmosphere/doc.go) | The sky over a world on the world's clock: the calendar (`atmosphere/calendar` — days, seasons, the moon, the periods of the clock's rules), the light of the day (`atmosphere/sky` — the sun and the moon of the hour, the sky's colours, a frozen light: P, Shift+] and Shift+[), the celestial sphere (`atmosphere/celestial` — the sun's path, the moon's orbit and phase, the real stars turning round the pole), the climate (`atmosphere/climate` — zones from the equator to the pole, the weather going from one kind to the next: wind, clouds whose shadows drift over the ground, rain, snow; Shift+W changes it), what falls (`atmosphere/precipitation`), what the weather does to the board (`atmosphere/weathering` — snow lying, ice, what sways), the sky behind the world (`atmosphere/backdrop`) and the clouds' shadows over a flat world (`atmosphere/overcast`) |
| [`plugins/topography`](plugins/topography/doc.go) | A map in relief drawn on the GPU: the heights, the slopes' cost, the light and the shadows, the water and the ways on them, the sea to the horizon; the views — from above, isometric and in perspective, Tab goes round, V takes a camera that follows a unit behind it and inside it, first person — with the cameras turned and tilted. The commands (`View`, `Turn`, `Ride`, `Raise`…) and `Relief` are its own; `relief` and `painter` are the vocabulary a game and the plugins share (`Climbing`, `MeanOfCells`, `Style`); the parts — relief, painter, water, terrain, hexes, billboards, cameras — are in `plugins/topography/internal` |
| [`plugins/navigation`](plugins/navigation/doc.go) | `MoveOrder` paths across a board, re-routing when terrain changes; right-click commands, and a unit's own (`MoveTo`, `Arrived`); route drawing; the crowd — rules over the moment `Touch` and the commands `StepAside`, `Detour`, `Pass`, `Hold`, `Settle`, `Stop`; its own crowd rules, as in StarCraft II: an ally standing makes way and stays aside, a group gathers round its point, strangers are gone round, nobody is stepped into water, off a cliff or into a wall |
| [`plugins/bullet`](plugins/bullet/doc.go) | Shots fired and flown: `Shoot` (by a player from its selected units, by an entity, aimed), `Shots`/`Ammo`/`Body`, the `Flight`, a `Landing`, a `Resting` and a `Burst` into `Blast`s |
| [`plugins/selection`](plugins/selection/doc.go) | `Select` into `Selected`; default bindings; `Selected(roles...)` and `Pointed()`, whom a command is for; highlight renderer |
| [`plugins/players`](plugins/players/doc.go) | A carrier over the command handlers: players and their bindings, `Pan` and `Zoom`; `Follow`, a player's camera fastened over its unit (`camera.Fastening`), and `Drive`, its hand on its units, summed into a `Hand` navigation reads; whose a unit is (`players/owner`) — a player selects, orders, follows and drives its own units alone |
| [`internal/steps`](internal/steps/doc.go) | The engine running the steps of rules and plans; `rule` and `rule/plan` are its faces |
| [`internal/engine`](internal/engine/doc.go) | The `Engine`: the window's loop (gogpu), one active Stage, persistence, input capture |
| [`gram`](doc.go) (public) | `Run`; the package you import. The root `doc.go` carries the concepts and the full package graph |

```
camera ──► render ──► plugin ──► rule ──► plugins/world ──► game ──► internal/engine ──► gram
control ───┘ (→ camera)                   │  ▲
                                          ▼  │
                     plugins/{collision, selection, vision} ──► plugins/board ──► plugins/navigation, plugins/bullet ──► plugins/players
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
