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
through WebGPU ([gogpu](https://github.com/gogpu/gogpu), pure Go). Everything beyond the tick
loop is a **plugin**: the built-in ones give a Stage a world of moving boxes, collisions, sight,
a board with terrain, pathfinding and mouse selection, and a game adds its own the same way.

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
  yours. There is no registry and no lookup by name: a plugin that needs another takes it as a
  constructor argument.
- **A Stage owns its ECS.** Each Stage gets a fresh world the moment it is entered, so a menu
  Stage sits idle with no gameplay entities until the player starts.
- **Behaviour is rules.** What a game does about what a plugin finds is a rule, a sentence of
  who, when and what, run inside that plugin's own pass — written in
  [ceremony](https://github.com/kjkrol/ceremony), the library gram builds on. A rule is a role's, a role is played, and
  the engine hands it to the plugin catching its moment; one no plugin catches is an error.
- **Kinds say what an entity is.** A kind is the list of components its entities carry, each
  constant or read from the entity's own row; it also tells save files what to expect.
- **Saves survive change.** Persisted resources are matched by name, never by position.
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
| **Stages and Scenes** | `game`, `game/stage` | Named Stages with their own ECS, defined a section at a time; Scenes as `ui` screens showing the world through cameras; a live Composition of what is shown and which Scene takes input |
| **Plugins** | `plugin` | The one extension contract; `Installer.Hosts`, where a plugin hands the engine the hosts that run the rules of its moments inside its own pass |
| **Behaviour** | [`ceremony`](https://github.com/kjkrol/ceremony) | The library gram builds on, through its binding over goke (`ceremony/goke`): `act.Branch[P]{Name, When, Then}` a rule of a moment, `plan.Loop[P]{Name, When, Then, Until, Timeout, Cooldown}` a loop of a plan, every clause of either saying whom it is about (`cond.Self.If(x)`, `act.Other.Do(cmd)`); roles a kind, a cell or a plugin plays; effects that hold for a while and turn the plugins' knobs; messages between plans; commands written as sentences, for entities found by name or group |
| **World** | `plugins/world` | Every entity's `Base`; movement under stop, wrap or open edges; the spatial index; kinds and spawning before the game and during it; heights or planes |
| **Steering** | `plugins/world/steering` | A `Steering` profile turned into heading and speed each step; `Driven` for a unit steered by hand |
| **Collisions** | `plugins/collision` | `Collider` to take part, `Physics` to bounce and be pushed apart, the solid ground of a board, `Meeting` and `Struck` for rules |
| **Gravity** | `plugins/gravity` | Bodies falling, standing on what bears them and leaping, along Z in a world with heights or across a flat one; `Leap`, `Away` and `Landing` for rules |
| **Shooting** | `plugins/bullet` | Shots as entities fired from a unit's muzzle or thrown in an arc, swept by collision; `Landing`, `Resting` and `Blast` for rules |
| **Sight** | `plugins/vision` | A `Sight` cone scanned into `Sighted`, cut by walls and dimmed by forests, over them by height; `Sighting` rules per observer |
| **Conversations** | `plugins/dialog` | Nodes of lines and answers from YAML, every unit with lines of its own; what an answer changes is the game's own |
| **Board and navigation** | `plugins/board`, `plugins/navigation` | Square or hex grid with terrain, ways and what stands on it; a map laid out of a file; `MoveOrder` paths that re-route, onto a bridge's deck or under it |
| **Bridges** | `plugins/bridges` | Bridges built by one set of rules whoever builds them: a level's file, the map maker, a player with a bridge in hand |
| **Topography** | `plugins/topography` | A map in relief on the GPU: heights shaped and priced, the sun's light and shadows, water glinting and running, from above, isometrically or in perspective |
| **Atmosphere** | `plugins/atmosphere` | The calendar, the sky of the hour with the real stars, the climate and the weather, what falls and what it does to the board |
| **Selection, players, cameras** | `plugins/selection`, `plugins/players`, `plugins/cameras` | Click and marquee into `Selected`; players over the command handlers, their bindings and a list of keys (K); cameras made where a scene shows them, fastened over a unit, behind it or inside it |
| **Telemetry** | `plugins/telemetry` | The panel in the corner saying how the game runs — the frame rate, the ticks, where a frame's time goes, the entities, the game time, the plugins' and the game's own lines — in every Stage with a world, shown and hidden with ~ |
| **Persistence** | `game.Persistence` | Save, load and list the active Stage's ECS and every tracked value by name |
| **Rendering** | `render`, `camera` | Everything drawn on the GPU (WebGPU, WGSL): sources composed by tier with a shared depth buffer, `Direct` sources with shaders of their own, a picture kept on the GPU (`Still`), an atlas baked at `Close` |

<a id="example"></a>
# Example

The smallest game that does something: one Stage with a torus of bouncing boxes, a collision
plugin counting their contacts, and one Scene showing them through a camera from above, the
telemetry panel over it (~ hides it). This is
[`examples/start/minimal`](examples/start/minimal/main.go); run it with `make demo-minimal`.

```go
package main

import (
	"image/color"
	"math/rand/v2"
	"time"

	"github.com/kjkrol/aabbworld"
	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/ceremony/comp"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram"
	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/entity/kind"
	"github.com/kjkrol/gram/game"
	"github.com/kjkrol/gram/game/stage"
	"github.com/kjkrol/gram/plugins/cameras"
	"github.com/kjkrol/gram/plugins/collision"
	"github.com/kjkrol/gram/plugins/players"
	"github.com/kjkrol/gram/plugins/telemetry"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/render"
	"github.com/kjkrol/gram/ui"
)

const (
	screenWidth, screenHeight = 640, 480
	boxSize                   = 12
	boxCount                  = 300
)

type box struct {
	pos world.Position
	vel world.Velocity
}

const (
	BoxKind    = "box"
	ArenaStage = "arena"
	ViewScene  = "view"
)

type arena struct {
	world     *world.Plugin
	collision *collision.Plugin
	cameras   *cameras.Plugin
	players   *players.Plugin
	telemetry *telemetry.Plugin
	player    *players.Player
	stats     collision.ContactStats
}

func main() {
	props := game.Props{
		Title:        "gram minimal",
		ScreenWidth:  screenWidth,
		ScreenHeight: screenHeight,
		TargetTPS:    60,
	}
	a := &arena{}
	mainStage := stage.New(ArenaStage).
		World(world.Config{
			Space: world.SpaceCfg{
				Width:  screenWidth,
				Height: screenHeight,
				Edges:  aabbworld.Torus,
			},
			Entities: world.EntitiesCfg{MaxCount: boxCount, MinSize: boxSize, MaxSize: boxSize},
		}).
		Plugins(a.makePlugins).
		Figures(a.defineFigures).
		Players(a.definePlayer).
		Spawn(a.spawnUnits).
		Scenes(a.defineScenes).
		Shows(ViewScene, telemetry.SceneName).
		Update(a.update)
	gram.Run(game.New(props, mainStage))
}

func (a *arena) makePlugins(w *world.Plugin) {
	a.world = w
	a.collision = collision.NewPlugin(a.world).WithStats(&a.stats)
	a.cameras = cameras.NewPlugin(a.world)
	a.telemetry = telemetry.NewPlugin(a.world)
	a.players = players.NewPlugin(a.world, a.cameras)
}

func (a *arena) definePlayer() error {
	a.player = a.players.Local("player")
	return a.player.Bind(a.players.Defaults()...)
}

func (a *arena) defineFigures() { a.world.Figures(box{}) }

func (a *arena) spawnUnits() {
	boxKind := kind.Named[boxRow](a.world.Kinds(), BoxKind)
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

func (a *arena) defineScenes(ctx game.Initializer) []game.Scene {
	m := &mainScene{arena: a}
	a.telemetry.With(a.stats.Reporter(&ctx.TPS().Ticks))
	return []game.Scene{ui.NewScene(ViewScene, m.screen()).Input(a.players.Handle)}
}

func (a *arena) update(ctx goke.RunCtx, d time.Duration) {
	a.world.RunPlan(ctx, d)
	a.collision.RunPlan(ctx, d)
	a.cameras.RunPlan(ctx, d)
	a.players.RunPlan(ctx, d)
	ctx.Sync()
}

type mainScene struct {
	arena *arena
}

var (
	boxColor        = color.RGBA{R: 90, G: 200, B: 110, A: 255}
	backgroundColor = color.RGBA{R: 30, G: 30, B: 30, A: 255}
)

func (m *mainScene) screen() *ui.Element {
	a := m.arena
	view := render.NewFeed(a.cameras.New(cameras.TopDown(), camera.Config{}), m.picture())
	return ui.Layers(
		ui.Blank().Fill(backgroundColor),
		ui.Image(view).Input(a.players.Through(a.player)),
	)
}

func (m *mainScene) picture() render.Picture {
	a := m.arena
	return render.NewComposer(a.world.Renderer())
}
```

## Demos

[`examples/motion/collision`](examples/motion/collision) is the same idea at scale: thousands of
colliding boxes at a fixed 120 TPS, saved on F5 and loaded on F9.

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

The demos are grouped by what they show, each group from the simplest to the richest. Every
demo opens a window; what it shows is tested in the library's package that does it.

### Start: the frame of a game

| Demo | Shows | Run |
|:---|:---|:---|
| [`start/minimal`](examples/start/minimal) | One Stage, one Scene, a world with collisions: the README example | `make demo-minimal` |
| [`start/scenes`](examples/start/scenes) | A menu Stage switching into a gameplay Stage; a modal panel over the ticking world, open or closed as the game was saved | `make demo-scenes` |
| [`start/split-screen`](examples/start/split-screen) | Two players at one keyboard, each driving its own block through a camera of its own in its half of the screen; a minimap over them | `make demo-split-screen` |

### Look: how entities are drawn

| Demo | Shows | Run |
|:---|:---|:---|
| [`look/appearance`](examples/look/appearance) | Walkers drawn from a sheet facing the way they go, stepping on the move, stamping while angry (R), the leader turned with a crown on, a ghost a ghost whatever it feels | `make demo-appearance` |
| [`look/animation`](examples/look/animation) | A figure drawn frame after frame in game time (Shift+P freezes the gait), stepping the way it goes in eight directions, standing at rest | `make demo-animation` |
| [`look/material`](examples/look/material) | An entity drawn by a material worked out per pixel in place of a sprite; C calms it into a plain sprite for a while | `make demo-material` |
| [`look/ember`](examples/look/ember) | Procedural animation out of the entity's state: embers breathe standing and stream a tail driven (WSAD); F douses them | `make demo-ember` |

### Rules: states, rules and commands

| Demo | Shows | Run |
|:---|:---|:---|
| [`rules/effect`](examples/rules/effect) | An ice witch turns the ground round her into snow and the lake into ice; it thaws behind her, a walker slips on it, a boat is frozen in, each kind in its own frozen look; F freezes whoever the cursor points at | `make demo-effect` |
| [`rules/trapdoor`](examples/rules/trapdoor) | Two levers (1, 2) and two strips of trapdoors over a pit: a lever a command for its group of trapdoors, which open under whoever stands on them; J hastens the selected scouts | `make demo-trapdoor` |
| [`rules/pressure-plate`](examples/rules/pressure-plate) | The same meadow with pressure plates: a plate met sets off the command that names it | `make demo-pressure-plate` |
| [`rules/wire`](examples/rules/wire) | Three commands, each a sentence saying what it does, whom it is for and who sets it off: a hero walked on WSAD, a figure in eight directions standing, stepping, bored and pulling; a lever he walks up to, pulled and pushed back at hand (Space), its arm swinging frame by frame along the effect's course; a plate stood on, a gate flipped (G); zoom by the wheel, = and - | `make demo-wire` |
| [`rules/dialog`](examples/rules/dialog) | Conversations read from YAML: the traveller greeted by the miller and the smith in a window, the answers changing the host's `Regard`, shown under it as Friend, Neutral or Enemy | `make demo-dialog` |
| [`rules/vision`](examples/rules/vision) | Entities keeping out of each other's way by sight, and a hunter living off the ones that fail | `make demo-vision` |
| [`rules/guard`](examples/rules/guard) | Guards on a plan (`plan.New`): patrol the yard, step aside for whoever blocks them long enough, rest where they arrive, run to the gate while the alarm (A) is on | `make demo-guard` |

### Board: boards and maps

| Demo | Shows | Run |
|:---|:---|:---|
| [`board/atlas`](examples/board/atlas) | A small flat board drawn from the game's own atlas, the water a material shimmering per pixel, a road laid as a way | `make demo-board-atlas` |
| [`board/flat`](examples/board/flat) | Madeira on the simple map out of `cmd/gramap`: the island in its colours, rivers and roads as bands, bridges and fords standing on it, a day and the weather going by | `make demo-board` |
| [`board/topography`](examples/board/topography) | Madeira in relief, its true heights, a cell 333 m: lit by the sun, water running, roads laid as Transport Tycoon lays them, bridges on piers (Shift+B builds one), the ground shaped under the cursor; from above, isometrically or in perspective (Tab) | `make demo-board-topography` |

### Navigation: units sent across a board

| Demo | Shows | Run |
|:---|:---|:---|
| [`navigation/square`](examples/navigation/square) | Units selected by click and marquee, right-click move orders along re-routing paths, holes the planner avoids and H opens | `make demo-navigation` |
| [`navigation/hex`](examples/navigation/hex) | The same on a hex board | `make demo-navigation-hex` |
| [`navigation/spawn`](examples/navigation/spawn) | Walkers spawned under the cursor (N) hold their cells at once; Shift+O outlines the cells held | `make demo-navigation-spawn` |
| [`navigation/vision`](examples/navigation/vision) | Sight cones that stop at walls and fade in forests, and a hawk that flies over both | `make demo-navigation-vision` |
| [`navigation/vision-hex`](examples/navigation/vision-hex) | The same on a hex board | `make demo-navigation-vision-hex` |

### Motion: collisions, shots and gravity

| Demo | Shows | Run |
|:---|:---|:---|
| [`motion/collision`](examples/motion/collision) | Thousands of bouncing boxes of many kinds, hit overlays, telemetry, save and load | `make demo-collision` |
| [`motion/bullet`](examples/motion/bullet) | A soldier on WSAD shoots: F fires a round the way it faces, G throws a grenade in an arc that lies with a spark and bursts; the shots are the plugin's, the wounds and the fuse rules, effects and a plan | `make demo-bullet` |
| [`motion/platformer`](examples/motion/platformer) | A hero in a side view under gravity: A and D walk, W jumps, G turns gravity over, a hard landing dazes | `make demo-platformer` |

<a id="model"></a>
# Model

## Stage and Scene

A `game.Game` supplies `Props` (window, tick rate) and its Stages by name; `game.New(props,
stages...)` is one. A Stage is one self-contained context, a menu or the gameplay, with its own
goke ECS built fresh the moment `Runtime.SwitchStage` enters it: `Init` installs its plugins,
`Spawn` seeds a fresh game, `Loaded` stands in for `Spawn` when the Stage is entered from a save,
`Update` is one tick. A game defines a Stage a section at a time with
[`game/stage`](game/stage/doc.go), always in the same order; a chain out of order does not
compile, a section the game has no use for is left out, and what is defined in the wrong
section is refused:

```go
stage.New(MeadowStage).
	World(worldCfg).                                   // the world…
	Plugins(s.makePlugins).                            // …and the plugins the arena makes over it
	Players(s.definePlayer).                           // the players, the plugins' default keys
	Effects(s.defineEffects).                          // the states
	Rules(s.defineRules).                              // the roles and their rules
	Commands(s.defineCommands).                        // what can be asked for
	With(dialog.Conversations(s.defineConversations)). // a plugin's own section, in its place
	Kinds(s.defineCells, s.defineKinds).               // the kinds of cells and of units
	Controls(s.bindKeys).                              // the game's own keys, each a command
	Spawn(s.spawn).                                    // a fresh game, out of its map
	Scenes(s.defineScenes).                            // the scenes, their pictures and cameras
	Update(s.update)                                   // the tick; hands back the game.Stage
```

Saving and loading are the engine's: commands like any other (`game.Save`, `game.Load`,
`game.Enter`, `game.Quit`) bound to keys (F5, F9, Shift+Esc in every game) or to ui buttons, a
Stage entered from its save from any other, a menu's Continue.

A Scene is one thing a Stage can show. Its screen is a `ui` tree: the world as a picture through
a camera made beside it, `render.NewFeed(s.cameras.New(cameras.TopDown(), camera.Config{}),
picture)`, wired to the player who acts through it, `ui.Image(feed).Input(s.players.Through(
player))`. The Stage's `Scenes` registry is static; the `Composition` over it is live: which
Scenes are visible, in what order, and which is active, the topmost focusable one and the only
one whose `HandleEvents` runs. `Runtime` (pause, quit, switch Stage, persistence, full screen)
reaches a Stage and every Scene alike.

## Plugins, rules and plans

A plugin's `Install` only queues ECS wiring, flushed in one `ecs.Setup` after the Stage's
`Init`. What a game does about what a plugin finds is a *rule* of [ceremony](https://github.com/kjkrol/ceremony), the
library of behaviour gram builds on, run in the pass of the plugin that catches its moment:

```go
roles := s.world.Roles()
roles.Define(PreyRole)
roles.Define(PredatorRole, act.Branch[collision.Meeting]{
	Name: "caught",
	When: cond.Other.If(roles.Named(PreyRole)),
	Then: act.Other.Do(world.Despawn{}),
})
kind.Define[body](kinds, HunterKind, kind.Spec{…, ceremony.Plays(roles.Named(PredatorRole))})
```

reads as a sentence, who, when, what: a predator, meeting one playing prey, has it give itself a
`Despawn`. Every clause says whom it is about: `cond.Self` the entity, `cond.Other` the other of
the pair or the moment's subject, `cond.Around(n)` the places round, `cond.On(s.world)` the
world's own entity; and in what state, `.If(x)`: under an effect, playing a role, as a word of
the moment says (`unit.Fallen()`, `vision.SeesNobody()`), `cond.Chance(p)`, `cond.Always`, or
`cond.Not` of one; clauses join with `And` and `Or`. The steps say the same, `act.Self.Put(e)`,
`act.Other.Do(cmd)`, `act.Around(1).Put(e)`, in order (`act.Steps`) or the first that does
(`act.OneOf`).
Nobody hands the rule to collision: once `Init` returns, the engine gives the rules
of every role somebody plays to the plugin that catches their moment, and a rule none catches is
an error. What an entity does over time is a *plan* of loops, `plan.New(name, root)`, carried
by its kind: a `plan.Loop` is a branch with an end — `Until` the *fact* a plugin tells it
(`cond.Self.If(navigation.Arrived{})`), `Timeout`, `Cooldown` — choosing what to do
(`plan.OneOf`) or doing one thing after another (`plan.Steps`). Rules and plans cast *effects*
that hold for a while and give *commands* for their entity (`plan.Self.Do(navigation.MoveTo{…})`),
the same a player gives. An effect turns the *knobs* a plugin gives, components it only reads, like
`steering.Steering` or a cell's `cell.Ground`. A rule holds no Go code but its words; how
entities are drawn is the one place rules are Go (`render.Rule`), declared on the world's atlas
in a scene, where a kind's look under an effect is declared too (`Slot.Under(effect, look)`).
A plugin ships no ready-made reactions: it gives moments and their words, and the game says
what follows. The language is ceremony's [doc/ceremony.md](https://github.com/kjkrol/ceremony/blob/main/doc/ceremony.md);
how a game uses it over gram's plugins is [`doc/behaviour.md`](doc/behaviour.md).

What a player *wants* is a command too, a `command.Command` (ceremony's): the plugin that
defines its type
(`navigation.MoveTo`, `selection.Select`) is a `plugin.CommandHandler` that drains its queue in
its own pass; the `players` plugin carries what a player's bindings, an AI or a network issue,
the world what the entities give themselves.

## Roles and commands

A *role* is a behaviour an entity plays, mortal, hasty, a plate, not a group:
`s.world.Roles().Define(name, rules...)` fires the rules for those playing it alone. **Defining
registers and hands nothing back**: effects, roles, commands and kinds are defined under a name
in the Stage's world and taken back with `Named(name)` wherever they are built on; a game keeps
its names as constants, each with a suffix saying what it names (`Ef`, `Cmd`, `Role`, `Plan`,
`Kind`, `Cell`). A kind plays roles through `ceremony.Plays(roles...)`, a cell through its kind
or alone, a plugin through its own `Plays`: a plugin is an entity of the world too
(`world.Self`), so a state of the sky is an effect on the atmosphere,
`ceremony.Put(bloodMoon).On(s.atmosphere)`, read back by `cond.On(s.atmosphere).If(bloodMoon)`.
A command about an effect is a sentence: put it on (`ceremony.Put`), take it off (`Take`) or
switch it (`Toggle`), for the entities bearing a
name, those in a group, the world itself, the player's selected units or the one pointed at
(`On`), set off by the entity named (`By`):

```go
const (
	OpenEf       = "open"
	HasteEf      = "haste"
	MortalRole   = "mortal"
	HastyRole    = "hasty"
	PlateRole    = "plate"
	OpenWestCmd  = "open west"
	OpenEastCmd  = "open east"
	HastenCmd    = "hasten"
	ScoutKind    = "scout"
	TrapdoorKind = "trapdoor"
	PlateKind    = "plate"
)

fx, roles, cmds := s.world.Effects(), s.world.Roles(), s.world.Commands()

// Effects
fx.Define(OpenEf, effect.Spec{effect.Lasts(2 * time.Second),
	effect.Alter(func(f *cell.Floor) { f.Allows = 0 })})
fx.Define(HasteEf, effect.Spec{effect.Lasts(3 * time.Second),
	effect.Alter(func(st *steering.Steering) { st.MaxSpeed *= 2 })})

// Rules
roles.Define(MortalRole, act.Branch[unit.Standing]{
	Name: "fall in",
	When: cond.Self.If(unit.Fallen()),
	Then: act.Self.Do(world.Despawn{}),
})
roles.Define(HastyRole)
roles.Define(PlateRole, act.Branch[collision.Meeting]{
	Name: "press",
	When: cond.Self.If(cond.Always),
	Then: act.Self.Do(ceremony.Trigger{}),
})

// Commands
cmds.Define(OpenWestCmd, ceremony.Put(fx.Named(OpenEf)).On(ceremony.Group("west trapdoors")))
cmds.Define(OpenEastCmd, ceremony.Put(fx.Named(OpenEf)).On(ceremony.Group("east trapdoors")).
	By(ceremony.Named("plate")))
cmds.Define(HastenCmd, ceremony.Put(fx.Named(HasteEf)).
	On(s.selection.Selected(roles.Named(HastyRole))))

// Kinds
units.Define(ScoutKind, land, profile,
	ceremony.Plays(roles.Named(MortalRole), roles.Named(HastyRole)))
standing.Define(TrapdoorKind, comp.Const(cell.Floor{Allows: cell.Land}))
standing.Define(PlateKind, comp.Const(collision.Collider{}),
	ceremony.Plays(roles.Named(PlateRole)))

// Controls
s.player.Bind(
	control.Give(control.KeyPress{Key: control.Key1}, "Pull the west lever",
		cmds.Named(OpenWestCmd)),
	control.Give(control.KeyPress{Key: control.KeyJ}, "Hasten the selected scouts",
		cmds.Named(HastenCmd)))

// Spawn: the map; a row's yaml tags are the file's keys
type unitRow struct {
	Start cell.ID `yaml:"start"`
}
level, _ := s.board.Lay(mapFile, "map.yaml")
board.Spawn[standingRow](level, nil)
board.Spawn[unitRow](level, board.Sides{
	PlayerSide: {players.Give{To: s.player.ID}, selection.Allow{}}})
```

```yaml
board:
  entities:
    - {kind: trapdoor, group: west trapdoors, at: [7, 2]}
    - {kind: plate, name: plate, at: [20, 14]}
units:
  - {kind: scout, side: player, start: [3, 14]}
```

1 opens every trapdoor in the group "west trapdoors" for two seconds under whoever stands there;
the plate, met, sets off the command that names it; J hastens the selected scouts. An AI gives
them as a key does (`players.Issue`). [`examples/rules/wire`](examples/rules/wire) is the whole
program.

## Kinds, spawning and saves

`kind.Define[Row](world.Kinds(), "name", kind.Spec{...})` says what an entity is: each component
`comp.Const(v)` (the same for all) or `comp.Load(func(row Row) T)` (read from that entity's row).
A unit over a board is defined through `board.NewUnits[Row](brd, shape, at)`, which derives its
position and cell from the one point `at` reads off a row and runs the world's roster, what the
plugins in the game bring by default and what they require. What stands still on the board, a
lever, a trapdoor, a gate, is a kind of `board.NewEntities`. `Spawn` puts entries on the world's
roster, or lays the game's map out of a file (`board.Plugin.Lay`: the board a character a cell,
the relief in metres, roads and rivers along paths, the entities and the units, each a kind and
the fields of its row tagged `yaml`); a fresh Stage spawns them, a loaded one does not. Mid-game
the command `world.Spawn{Entry}` adds an entity the same way. Kinds tell save files every
component type their entities carry, so a game's own state survives a save without being
registered anywhere else.

## Maps of the Earth

`cmd/gramap` makes such a map of any stretch of the Earth at any scale out of a recipe
(`cmd/gramap/madeira.yaml` is Madeira's):

```bash
go run ./cmd/gramap -recipe cmd/gramap/madeira.yaml -out examples/board/topography/map.yaml
go run ./cmd/gramap -recipe cmd/gramap/madeira.yaml -out examples/board/flat/map.yaml -flat
```

The recipe says where, how many cells and how many metres a cell spans, what the ground is by
its slope and height, how much drained land makes a brook, a stream and a river, and the roads
as chains of named places. The heights come from the
[Terrain Tiles on AWS](https://registry.opendata.aws/terrain-tiles/), fetched once into the
user's cache. The rivers are carved into the ground, the roads routed and graded as Transport
Tycoon lays them, with bridges over water and down valleys and fords where a river is shallow;
with `-flat` the map has no heights.

<a id="architecture"></a>
# Architecture

The packages form a strict acyclic graph; each imports only the layers below it. Every package
has a `doc.go` describing what it brings. What is left to do is in
[`doc/roadmap.md`](doc/roadmap.md).

| Package | Responsibility |
|:---|:---|
| [`camera`](camera/doc.go) | The contract of a view onto a world: screen conversion, culling, move and zoom, projections, fastening over an entity |
| [`control`](control/doc.go) | The input vocabulary: `Binding`, the triggers, `Context`, `Keys`, a player's `Tool`; the commands a binding builds are `ceremony/command`'s |
| [`render`](render/doc.go) | Drawing: `Renderer`, the `Composer` over `Source`s and its `Frame`, `Direct` sources, `Feed`, `Still`, `Atlas`, `Font`, the drawing rules a renderer runs every frame |
| [`plugin`](plugin/doc.go) | The extension contract: `Plugin`, `Installer` (with `Hosts`, the hosts of a plugin's moments), `CommandHandler`, `Serializable`, `PostLoader`, `Populator`, `Made`; `plugin/section` the parts of a Stage |
| [`entity`](entity/doc.go) | What every entity carries: `Base`, `Position`, `Velocity`, `Z`, `Layers`, `Eye`, `Lie`; `entity/kind` what an entity is (`Spec`, `Define`, `Of`, the roster) |
| [`clock`](clock/doc.go) | The tactical clock: game time as the sum of the simulation's steps, the pause (Shift+P), the tempo (] and [), the `Moment` of a step with `At` and `Every` |
| [`ceremony`](https://github.com/kjkrol/ceremony) | The library gram builds on, outside the module: how entities behave — `cond` the conditions, `act` the steps and the branch of a rule, `plan` the loops of a plan, `effect` the states and knobs, `fact` the facts and messages, roles, commands about effects and labels in its root, `host` the hosts a plugin runs rules with, `command` the queues and the carrier, `tag` and `comp`; the language in its `doc/ceremony.md`, how a game uses it over gram's plugins in [`doc/behaviour.md`](doc/behaviour.md) |
| [`plugins/world`](plugins/world/doc.go) | The foundation: the space, movement under the edge rules, kinds and tags, `Seed`/`Populate`, `Spawn`, `Despawn`, the commands about effects, the carrier of commands, the entities' looks on the atlas; it runs the core's systems; `steering` and `view` beside it |
| [`game`](game/doc.go) | What a game implements and receives: `Game`, `Stage`, `Scene`, `Scenes`, `Composition`, `Initializer`, `Runtime`, `Persistence`; `game/stage` a Stage defined a section at a time |
| [`ui`](ui/doc.go) | A scene's screen as a tree of elements: `Layers`, `Columns`/`Rows`, anchors, `Panel`, `Label`, `Image` of a feed, `Window`, `Button` giving commands, elements pinned to entities `Under` an effect or `On` a name, a `Theme` |
| [`plugins/collision`](plugins/collision/doc.go) | Collision over the world's space: `Collider`, `Physics`, `Sweep`, the solid ground of a `Field`, `Meeting` and `Struck` |
| [`plugins/vision`](plugins/vision/doc.go) | `Sight` cones into `Sighted`, `Sighting` rules, the views drawn |
| [`plugins/board`](plugins/board/doc.go) | A square or hex grid with terrain over the world: the `Board`, `Layout` and `Map`, `NewUnits`, `NewEntities`, a level out of a file (`Lay`); `cell` the vocabulary of a cell, `unit` of a unit on it, `grid` the topology, `look` how it is drawn, `ground` what its ground is to the others, `water` and `network` rivers and roads |
| [`plugins/atmosphere`](plugins/atmosphere/doc.go) | The sky over a world on the world's clock: `calendar`, `sky`, `celestial`, `climate` and its `weather`, `air`, `precipitation`, `weathering`, `backdrop`, `overcast` |
| [`plugins/topography`](plugins/topography/doc.go) | A map in relief drawn on the GPU: the heights, the light and the shadows, the water, the views from above, isometric and in perspective, the commands shaping the ground; `relief` and `painter` its vocabulary |
| [`plugins/navigation`](plugins/navigation/doc.go) | `MoveOrder` paths across a board, re-routing as terrain changes, on a bridge's deck or under it; `MoveTo` a cell or an entity by name; a patrol as a plan over the unit's `Round` (`Onward`); the crowd as rules over `Touch`; routes drawn |
| [`plugins/driving`](plugins/driving/doc.go) | Units driven by hand: `Ahead`, `Back`, `Turn`, `Toward`, `Jump`, `Tank` and `Compass` keys |
| [`plugins/gravity`](plugins/gravity/doc.go) | Bodies falling, standing and leaping: `Pull`, `Body`, `Leap`, `Away`, `Landing` |
| [`plugins/bridges`](plugins/bridges/doc.go) | Bridges: `Types` of them, a level's, a player's with a `Tool` in hand; where one may stand in `bridges/site` |
| [`plugins/bullet`](plugins/bullet/doc.go) | Shots fired and flown: `Shoot`, `Shots`, `Flight`, `Landing`, `Resting`, `Burst` into `Blast`s |
| [`plugins/selection`](plugins/selection/doc.go) | `Select` into `Selected`, `Hovered`, `Selected(roles...)` and `Pointed()` as the targets of commands |
| [`plugins/dialog`](plugins/dialog/doc.go) | Conversations as data: `Node`s from YAML, a unit's `Script`, the game's `Changes` and `Conditions`, `Window` |
| [`plugins/cameras`](plugins/cameras/doc.go) | The cameras a game looks through: `New`, `Pan`, `Zoom`, `Follow`, `LookAt`, `Keys` |
| [`plugins/players`](plugins/players/doc.go) | Players over the command handlers: bindings into commands, `Through` and `IssueAs` for a ui scene, the list of keys; `players/owner` whose a unit is |
| [`plugins/telemetry`](plugins/telemetry/doc.go) | The telemetry panel: standard lines, the plugins' reporters and the game's own lines, a scene of its own in every Stage with a world, ~ |
| [`internal/engine`](internal/engine/doc.go) | The `Engine`: the window's loop, one active Stage, saves and loads |
| [`gram`](doc.go) | `Run`; the package you import. The root `doc.go` carries the concepts and the full package graph |

```
camera ───► render ──► plugin ──► plugins/world ──► game ──► internal/engine ──► gram
control ────┘  ▲                  │  ▲
ceremony ──────┘                  ▼  │
      plugins/{cameras, collision, selection, vision} ──► plugins/board ──► plugins/driving ──► plugins/navigation
                                                            └──► plugins/{bullet, gravity, bridges, topography, atmosphere} ──► plugins/players
```

Outside the module: [ceremony](https://github.com/kjkrol/ceremony) is the behaviour — rules, plans, effects, roles,
commands, facts — which gram's plugins perceive for and carry out, with goke as its store;
[goke](https://github.com/kjkrol/goke) is the ECS every Stage runs on,
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
dependencies; it has since left Ebitengine for WebGPU. The world, collisions and line of sight
are `aabbworld`'s; gram is where they meet an ECS and a screen.

<a id="documentation"></a>
# 📖 Documentation

- **API reference** on [pkg.go.dev](https://pkg.go.dev/github.com/kjkrol/gram).
- **Concepts and package graph** in the root [`doc.go`](doc.go); each package's own `doc.go`
  explains what it brings (see [Architecture](#architecture)).
- **The language of behaviour** in ceremony's [README](https://github.com/kjkrol/ceremony) and
  [doc/ceremony.md](https://github.com/kjkrol/ceremony/blob/main/doc/ceremony.md); [`doc/behaviour.md`](doc/behaviour.md) is how a
  game uses it over gram's plugins.
- **Benchmarks** in [BENCHMARKS.md](BENCHMARKS.md); **changes** in [CHANGELOG.md](CHANGELOG.md).

# Credits

- The night sky's stars: the Yale Bright Star Catalogue, 5th revised edition (Hoffleit &
  Warren 1991), from the CDS, catalogue V/50 (`plugins/atmosphere/celestial/stars_gen.go`).
- The moon's face: NASA's Scientific Visualization Studio, CGI Moon Kit (Lunar Reconnaissance
  Orbiter LROC data) (`plugins/atmosphere/celestial/moon.png`).
- The wire demo's lever: "Lever and Button" on OpenGameArt (CC0), its two positions, with two
  frames between drawn after them (`examples/rules/wire/lever.png`). The figure of the wire,
  animation and appearance demos (`hero.png` beside each) is drawn in-house.
- The heights of Madeira's maps and of every map `cmd/gramap` makes: the Terrain Tiles on AWS,
  Mapzen's Tilezen terrarium — SRTM, EU-DEM (Copernicus), GMTED2010, ETOPO1 and others; see their
  [attribution](https://github.com/tilezen/joerd/blob/master/docs/attribution.md).

# License

[MIT](LICENSE)
