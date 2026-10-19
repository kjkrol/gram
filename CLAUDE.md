# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## What this is

**gram** (module `github.com/kjkrol/gram`) is a public, modular Go game engine library: a
user-implemented `game.Game` — a named collection of `game.Stage`s, each with its own lifecycle
(`Init`/`Spawn` or `Loaded`/`Update`) and its own `game.Scene`s (`Stack`/`Composition`, the sole
entry point for input) — is driven by a `gram.Engine` that wraps
[goke](https://github.com/kjkrol/goke) (a type-safe, archetype-based ECS) in a window's loop — a
tick and a picture a frame — drawing everything on the GPU through WebGPU
([gogpu](https://github.com/gogpu/gogpu), pure Go over Vulkan). Everything beyond the tick loop is
a `plugin.Plugin`, made by the game's arena over the Stage's world and used by the Stage in the
order made (`stage.New(name).World(cfg).Plugins(a.makePlugins)`).

How entities behave — rules, plans, effects, roles, commands, facts — is the library
[ceremony](https://github.com/kjkrol/ceremony) (`github.com/kjkrol/ceremony`; locally
`/home/karol/CODE/ceremony`, joined to gram by `/home/karol/CODE/go.work` with goke at
`/home/karol/CODE/goke`), which gram uses through its binding over goke, `ceremony/goke`
(imported as `cgoke` wherever the ECS is imported too). gram's plugins perceive (moments, facts,
words, knobs) and carry commands out; the world keeps the registers and runs the binding's
systems. The grammar is documented there (`ceremony/CLAUDE.md`, `ceremony/doc/ceremony.md`),
never repeated here. Nothing that smells of the game goes into ceremony (an "obrzęd": prescribed
steps with roles — `command.Issuer`, never a player; no sprite, cell or unit).

The extension contract — `game.Game`/`Stage`/`Scene`/`Stack`/`Composition`/`Initializer`/
`Runtime`/`Persistence` and `plugin.Plugin`/`Installer`/`Serializable`/`PostLoader` — lives in the
public root packages `game` and `plugin` for godoc; only orchestration (`Engine`) lives in
`internal/engine`, re-exported by root `gram` as `Engine`/`Props`/`NewEngine`.

**Read the skill for the layer before working in it** (`.claude/skills`, loaded by their
descriptions): `gram-plugin` (how a plugin is built and written: the contract, the registers,
the delivery of rules, the store behind ceremony's contract, roles, `world.Self`, tags, kinds, a
plugin's package layout), `gram-plugins` (the reference of the built-in plugins, one entry each),
`gram-render` (the Composer, Direct, atlases and looks), `gram-game` (a game and a demo: Stage,
sections, arena, scene, ui, players, persistence), `gram-probe` (a demo driven headless tick by
tick), `goke` (the ECS: model, queries, release flow, `scripts/api.sh`), `ceremony` (the language
and its pitfalls).

## Commands

```bash
go build ./... && go vet ./... && gofmt -l . && go test ./...   # standard verification sequence
go test ./plugins/world/... -run TestName -v                     # a single test
make demo-start-minimal                                         # the README example
make demo-start-scenes                                          # a menu Stage and a game Stage
make demo-start-split-screen                                    # two players at one keyboard, each a block and a camera of its own; a minimap
make demo-look-animation                                        # a gallery of animations from sheets: walking, braking, bored, pulling, a lever swinging
make demo-look-material                                         # a ward's material pulsing, embers drawn out of their state circling it; G calms, F douses
make demo-look-sky                                              # a sky of the game's own over a valley: its vault, a big orange moon by day, made-up stars
make demo-look-light                                            # a flat board under a sun of the game's own circling in a day: a tower's shadow baked, walkers' patches, the hero's silhouette (WSAD), a lantern lighting the night, lava taking no shade; Shift+L the lamps' shadows
make demo-rules-effect                                          # an ice witch: frost and frozen as effects; J hastens the selected
make demo-rules-mechanisms                                      # a lever, a plate and a switch driving trapdoors and a gate; one hero on WSAD, the lever pulled by touch (Space)
make demo-rules-dialog                                          # conversations from YAML: the miller and the smith
make demo-rules-vision                                          # sight, avoidance and a hunter
make demo-rules-guard                                           # guards on a plan: patrol, step aside, rest, run to the gate while the alarm (A) is on
make demo-board-atlas                                           # a small flat board from the game's own atlas, the water a material
make demo-board-flat                                            # Madeira on the simple map: a flat board, bands for the ways, bridges and fords
make demo-board-topography                                      # Madeira in relief: light, water, roads, bridges, the camera on a stick (Q/E, R/F, Tab), the weather
go run ./cmd/gramap -recipe cmd/gramap/madeira.yaml -out <map.yaml> [-flat]   # a map of the Earth from the Terrain Tiles on AWS
make demo-navigation-square                                     # units sent round a wall, over a river by a bridge a wagon is too wide for; walls cut sight, forests dim it, a hawk flies over
make demo-navigation-hex                                        # the same on a hex board
make demo-navigation-spawn                                      # walkers spawned under the cursor (N) hold their cells; Shift+O shows the cells held
make demo-motion-collision                                      # thousands of bouncing boxes of many kinds, a flash on every hit
make demo-motion-expanse                                        # a hundred thousand boxes on a torus of ~98 km, alive in bands round the camera (Shift+G, Ctrl+G)
make demo-motion-bullet                                         # a soldier on WSAD shoots rounds (Space) and throws grenades (Shift held, the longer the further)
make demo-motion-platformer                                     # a hero in a side view under gravity: A/D walk, W jumps, G turns gravity over
make bench                                                      # every benchmark once, with allocations
make bench-save                                                 # 5 repeats into bench_results/ (ignored by git)
```

Only `render/gpu` and `internal/engine` import gogpu (and wgpu, gputypes); plugins see `render`
and `control`. Tests that draw ready a headless device (`gpu.Headless`; `GRAM_GPU=software` for
the software rasteriser, which draws wrong) and skip without a GPU. A running game logs its frame
rate and where a frame's time goes every second with `GRAM_FPS_LOG=1`, starts fullscreen with
`GRAM_FULLSCREEN=1` (F11 switches) and runs unpaced with `GRAM_VSYNC=off`; headless probes of a
demo's frame include about 10 ms of the GPU's readback at 2560x1440, so compare them with each
other, not with a frame rate. `GRAM_SHOT=<file.png>` saves frame `GRAM_SHOT_FRAME` (120) and
quits, the world taking one step a frame till then: shots of a baseline tree and of the working
one compare a change's pictures (the telemetry panel, random weather and random scenes are noise).

The `examples/<group>/<name>` programs are real GUI apps — `go test` cannot exercise them. To
check one still runs after a change: build to a temp path, run under `timeout <n>s`, treat exit
124 (still running) as healthy. That is no substitute for looking at the demo when a visual
change needs confirmation; for what a demo does tick by tick without a window, the `gram-probe`
skill.

**A change is checked on the demos that use what it changed**: `grep -rl
'"github.com/kjkrol/gram/plugins/<pkg>"' examples` lists them (the atmosphere: board/topography,
board/flat, look/sky); a demo without the package shows nothing of it. Every demo is the field
only for a change to `render`, `render/gpu`, the engine or the world. Pictures and frame rates
compare a copy of the tree from before the change with the working one, alternately, in each
regime the change has (steady, and the moment it works hardest): `gram-probe`, § Checking a
change.

CI (`.github/workflows/go.yml`) runs
`xvfb-run -a go test -v -race -coverprofile=coverage.txt -covermode=atomic ./...` on Go 1.27.0.

## Rules of the house

Decisions the user has made; they are not up for revisiting in passing.

- **Behaviour is written in ceremony, never as Go in a plugin's system.** A plugin perceives
  (moments, facts, words, knobs) and carries commands out; what to do when is rules and plans; a
  rule holds no Go code of its own. What a plugin does of its own — a pace, a log, a count, the
  weather — is its own pass's work, never a rule. The one place rules are Go is drawing
  (`render.Rule`). A step a rule cannot say is a missing moment, step or knob.
- **No hooking.** No `ctx.Hook`, no `Plugin.Hook`, no side entrance by another name: a rule is a
  role's, a role is played, the engine delivers the rules to the hosts. Tests of a plugin's own
  package may keep a `Hook` in `export_test.go`.
- **No plugin ships ready-made reactions**: an effect named by the library and one game's
  reaction are no reuse. A plugin gives moments and their words.
- **Defining registers and hands nothing back**; `Named(name)` is the handle. A game keeps its
  names as constants in its `names.go`, **every name with a suffix saying what it is**: `Ef` an
  effect, `Cmd` a command, `Role`, `Plan`, `Kind` a kind of unit, `Cell` a kind of cell; one
  literal used as two things is two constants (`PlateCell`, `PlateRole`); a name worked out is a
  function named alike (`pullCmd(lever)`); a name a package already gives is used from there
  (`weather.Clear`).
- **What a plugin gives is in its root package** (`commands.go`: its commands, `Queues()`,
  `DefaultBindings()`), as **real types, never aliases** — an alias gets no godoc. The one
  exception is the entity re-exports (`world.Base = entity.Base` and kin), where the component
  must be one type for goke and the saves.
- **A command is a `command.Command`** by its method `Command()`, confirmed where it is defined
  (`var _ command.Command = MoveTo{}` beside the type). Whatever takes commands takes a
  `command.Command`, never `any`.
- **The default keys are the same in every game and no demo repeats one**: Shift+P the tactical
  pause, Shift+Esc quits, K lists the keys, F10 the settings screen, ~ the telemetry panel, F5
  saves, F9 loads, F11 full screen, ] and [ the tempo, Shift+X the boxes (collision's: the bases, the solids, none),
  Shift+H the scroll zones (cameras'), Shift+O the holds, Shift+B the bridge tool, Shift+R the
  routes, Shift+C the cones, Shift+W the weather, Shift+V the clouds' look, Shift+S the high
  clouds' shadows (with the camera free: riding, Shift+S sprints back), P freezes the light,
  Space jumps in first person, = and - zoom, Shift+= and Shift+- shape the relief, Ctrl+= and
  Ctrl+- the camera's sight (farther, nearer), Alt+= and Alt+- the picture's resolution (finer,
  coarser), Alt+A its following the frame budget (following, in rings, off), Alt+Shift+= and
  Alt+Shift+- the ground's full care (farther, nearer), Ctrl+F the ground's haze, through any
  camera (off as a game starts), H the shadows' detail (coarse, fine), Shift+L the lamps' shadows
  (off, on); in a world cut into sectors Shift+G the sectors (by band, by what the camera
  sees, none) and Ctrl+G the bands (off, on); with a minimap Shift+M shows or hides it and Ctrl+M
  turns it with the camera or keeps north up.
  A key held, down or let go leaves the chords with Shift, Ctrl or Alt to these shortcuts unless
  its binding is `InChords()` (a sprint, a jump while sprinting). A plugin's keys a game sets up
  (`driving.Compass`, `selection.FollowKey`, the cameras' zoom keys) are bound in the
  Players section with the defaults; the game's own go to the Controls section and are listed under
  "Game".
- **Everything a game defines is its Stage's, by name**, each register the world's, so each
  Stage has its own and nothing is the program's; a game's arena holds plugins and players alone.
- **The order a game makes its plugins in is the order they are used in**; no list, no `ctx` in
  a game's code. A plugin needing another's logic takes it as a constructor argument.
- **A thing in its section**: what is defined in another section is refused (`gram-game`).
- **A plugin's settings are its own, saved with the game**: what a plugin shows or how hard it
  works at a player's word — the grid, the boxes, the cones, the routes, the zones, the clouds'
  look, the shadows' detail, the haze, the tempo — is a field of its `Res.Settings`, set by a
  command carrying the value (`Boxes{Shown}`, `Haze{On}`, `Tempo{X}`), announced in `Settings()`
  (`plugin.Tunable`: a `control.Switch`, `Choice` or `Range`, `Get` read every frame, `Set`
  building the command) and kept whole by `plugin.Keep(&p.Res.Settings)`, never a
  command that means "the next one": the key's binding builds the next value from the setting.
  Its start is the plugin's Config or a setter in `makePlugins` (`ShowGrid(false)`,
  `ShowCones()`), never the scene's, which is made after a load. F10 opens the players' settings
  screen, generated from every `Tunable` the players carry; while it is up the game stands in
  the tactical pause unless its own setting says it runs.
- **How things look is the scene's**: the start of `screen()` in a demo's `scene.go` sets how the
  plugins draw (`WithAtmosphere`, styles, views) and nothing else does; `makePlugins`
  configures what a plugin does and what it starts showing. A sprite in a Stage's struct is a leak. A look is
  drawing (`Under`), never an effect's Spec. What a plugin draws about a kind — its outline,
  cone, route — is styled in the figure's `Look` with `Styled` (`selection.Highlight`,
  `vision.Cone`, `navigation.RouteStyle`). What the board draws beyond its tiles — the grid's
  line, the width of the cells held — is a surface's `Look` (`surface.go`, `board.Surface`), as
  the sky is a `Sky`'s. A scene names nothing that draws and declares no atlas: every plugin
  draws what it gave `ctx.Draws` in `Install`, a feed shows the Stage's one picture
  (`render.NewFeed(cam, a.world.Picture())`) — a minimap alone shows a picture of its own
  (`minimap.Plugin.For(cam, player)`, placed and dressed by the scene) — and the Stage has the
  figures', tiles' and surfaces' looks declared and the atlases closed before its scenes
  (`ctx.Looks`).
- **A kind of entity is a figure, a kind of cell a tile**, each in a file of its own
  (`hero.go`, `lever.go`, `tiles.go`) with everything that is its alone: its effects
  (`Effects`), its roles and rules (`Rules`), what it is (`Define`), how it looks (`Look`),
  **asserted beside the type** (`var _ board.Figure = hero{}`, `board.Stateful`, `board.Ruled`,
  `board.Tile`; `world.Figure` without a board; a plugin with a register of kinds gives its own,
  `bridges.Figure`, `bullet.Figure`) — the optional parts are found by assertion, so
  only the `var _` says a misspelt `Effects` is noticed.
  The Stage's `Figures` link notes them and runs each part in its section; their looks are
  declared once the Stage is set up, before its scenes. The arena's `Effects` and `Rules` keep only
  what several kinds share (a role many play, a rule of the whole); the commands wiring the
  map and the spawn are the arena's. Coloured balls that are nothing but a kind are a figure
  in `demo.go`.
- **A map is a file** (`map.yaml` beside a demo's code). Entities standing on the board are
  entities, listed one by one, never placed by a legend's character. Kinds stay in Go.
- **What stands on the board is an entity, not terrain**: a lever, a trapdoor, a gate, a plate.
- **Terrain is never an entity in the space**; a bridge is an entity a cell.
- **Never make a unit see the others ahead**: it learns by striking (navigation); lanes were
  tried and dropped.
- **Physics, not blocking**, for a unit under gravity: into the water and off an edge as it
  pleases, the rules deciding.
- **The light plugin gives the instruments, never the sun**: a `light.Distant` light (way,
  strength, ambient, colours), lamps, and what each thing casts and takes (`light.Shadow`, a
  cell's `cell.Kind.Takes`); what shines is a `light.Source` on an entity. The sun is the
  atmosphere's: its sky writes the sun of the hour into the `light.Shining` beside its own
  entity's Source; a scene without a sky sets a `WithDistant` light. The light bakes the
  ground's shade and lays the live shadows; a ground draws itself in the frame's light, shade and
  lamps (`render.GroundShade`, `lightWay`, `lampLightAt`), never working light out of its own.
- **What is drawn over what is Z's alone**, never a layer, the order of adding or the order a
  plugin registers in: every piece of the world lies at a height (`render.At`; what lies on a
  surface at a small lift over it — a route's goal, a cell held, a bridge in hand — a figure at
  `world.FigureLift`, an entity's box at its base), the depth buffer deciding; the opaque ground
  first; the marks over the world at fixed levels (`render.OnTop(level)`: the selection's
  outline, then its box, the cameras' zones over all); a Direct asks where it draws
  (`Frame.DrawOpaque`, `DrawAt`, `ReadGround`).
- **A board stands in a world with heights** (`world.Config.Heights`), and so do the topography
  and the atmosphere: each panics in a flat one. A flat map is a board without relief; a side
  view (motion/platformer) is a world with heights, gravity pulling along Y, Z its depth. A
  world without a board (motion/collision, start/minimal, start/scenes, rules/vision) stays
  flat.
- **Examples are self-contained** (a demo keeps its `hero.go` and `hero.png`; no shared package
  under `examples/`), **not tested** (what a demo shows is tested in the library's package; no
  `_test.go` in `examples/`), **grouped by what they show** (`start`, `look`, `rules`, `board`,
  `navigation`, `motion`), with `main` in `demo.go`, the scene in `scene.go`, the names in
  `names.go`, no `NewDemo`, no `-demo` in a name.
- **Comments say only what the code cannot**; **lines stay within 100 columns**; **one value a
  line** (see "Docs, benchmarks and commits").
- **The user tags and publishes releases**, never Claude; a goke or ceremony fix reaches gram
  through a tag and a `go.mod` bump.

## The map

Dependency direction is one-way: `render` ← `plugin` ← `game` ← `internal/engine` ← `gram`.
`render` draws with no knowledge of Stages, Scenes or plugins; `plugin` is the extension
contract (`Plugin`/`Installer`/`Serializable`/`PostLoader`/`Populator`/`CommandHandler`/
`KindTemplater`/`Tunable`/`Keep`/`Made`,
`plugin/section`); `game` is what a Stage implements and receives; `internal/engine` the
`Engine`. Built-in plugins (`plugins/*`) import `plugin`/`render` directly, as a third-party
plugin would. gram's core beside them — `ceremony`, `entity` (with `entity/kind`), `clock`,
`control`, `camera` — imports no plugin; the world makes and runs their systems. A plugin's root
holds its entries, its public subpackages the vocabulary and contracts, `internal` the machinery
(`gram-plugin`). **Layering: the world knows its entities and nothing else; the ground is the
board's, the sky the atmosphere's, `render` generic.**

Built-in plugins (`gram-plugins`): `world` (the foundation: space, entities, the registers,
the clock), `motion` (steering, movement and the leavers past an open edge, made first and run
right after the world; whatever steers or pushes takes it), `light` (a distant light and the
lamps, the shadows baked and cast; made before the board and the topography, which draw by it),
`board` (grid and terrain, cells as entities, what stands on the board, bridges, maps as files;
`cmd/gramap` makes maps of the
Earth), `topography` (a map in relief, the cameras' cage), `atmosphere` (the time of day, the
climate, the weather), `collision`, `navigation`, `selection`, `dialog`, `players` (with
`players/owner`), `cameras`, `driving`, `gravity`, `vision`, `bridges`, `bullet`, `minimap`
(small pictures round the players' cameras), `telemetry`
(the panel in the corner, a scene of its own in every Stage with a world). Each package has a
`doc.go` describing what it brings, ending with its `# Vocabulary`.

## Testing conventions

Internal tests (`package world`) where a test needs unexported state; otherwise the `_test`
suffix. Prefer testing through a plugin's real `Install` → `ecs.Setup` → `RunPlan` path over
hand-built shortcuts when what is under test is install-order or wiring: regressions there have
only shown up that way. The headless installer in `bench/headless_test.go` is one more copy of
the helper the plugins' tests keep.

## Docs, benchmarks and commits

`doc/roadmap.md` lists what is left to do, and only that: take an item out when it lands. Every
package has a `doc.go` with `# Type` sections, each short: what the package brings and what a
game does with it, never how it does it inside, no history; the root `doc.go` carries the
concepts, the tick lifecycle and the layered package graph. README leads with what the library
is; its code example is `examples/start/minimal`, so change that program first and keep the README
in step; it lists the examples a table a group, each from the simplest to the richest. All
benchmarks live in `bench/` (package `bench_test`); `BENCHMARKS.md` records the results and the
method (this machine drifts ~10% between runs — compare a baseline copy and the working tree
alternately). `CHANGELOG.md` gets an entry per tag.

**A comment says only what the code cannot.** Write the code first, then add a comment only where
a reader would get something wrong or miss it without one. An exported type, function, method or
field gets **at most two sentences**: what it is and what it is for, never how it does it, never
its history. Unexported code, the bodies of functions and the tests get no comment unless it
carries a non-obvious why — a unit, a direction, a reason, a trap, a source. The sentence above a
test stays only where the test's name does not say what it checks. Never: a comment repeating the
name, the signature or the next line; section banners; a demo's package prose (what a demo is
and how it is played is its `Props.Description`); a constant made to carry a comment for a value
used once; a doc comment on an example's unexported name by habit.

**Lines stay within 100 columns**, a tab counted as 4. Break a call between its arguments or a
chain between its links, never right after an accessor, never by pushing a trailing comment past
a closing parenthesis (such a comment goes above the line). A struct literal of many fields is a
field a line. **One value a line**: a colour, a vector, a short literal stays whole on its own
line, never two assigned at once. `golines -m 100 -t 4 --no-reformat-tags` does most of it; check
what it made.

Commit messages use the conventional prefixes (`feat`, `fix`, `docs`, `refactor`, `perf`, `test`,
`chore`) and carry no `Co-Authored-By` trailer. Commit and push only when asked.
