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
make demo-collision                                                # examples/motion/collision
make demo-navigation                                               # examples/navigation/square (select, move, camera)
make demo-navigation-hex                                           # the same on a hex board
make demo-navigation-spawn                                         # walkers spawned under the cursor (N) hold their cells; Shift+O shows the cells held
make demo-guard                                                    # guards on a plan: patrol, step aside, rest, run to the gate while the alarm (A) is on
make demo-navigation-vision                                        # board + navigation + vision: walls cut sight, forests dim it, a hawk flies over
make demo-navigation-vision-hex                                    # the same on a hex board
make demo-effect                                                   # an ice witch: frost and frozen as effects
make demo-bullet                                                   # a soldier on WSAD shoots rounds (F) and throws grenades (G)
make demo-board                                                    # Madeira on the simple map: a flat board, bands for the ways, bridges and fords
make demo-board-topography                                         # Madeira in relief: light, water, roads, bridges, isometric or from above (Tab), the weather
go run ./cmd/gramap -recipe cmd/gramap/madeira.yaml -out <map.yaml> [-flat]   # a map of the Earth from the Terrain Tiles on AWS
make demo-board-atlas                                              # a small flat board from the game's own atlas, the water a material
make demo-wire                                                     # a lever, a plate and a switch driving trapdoors and a gate; one hero on WSAD, the lever pulled by touch (Space)
make demo-material                                                 # an entity drawn by a material instead of a sprite; C calms it into a sprite
make demo-animation                                                # a figure drawn frame after frame from a sheet; Shift+P freezes the gait
make demo-dialog                                                   # conversations from YAML: the miller and the smith
make demo-ember                                                    # procedural animation out of the entity's state; F douses into soot
make demo-platformer                                               # a hero in a side view under gravity: A/D walk, W jumps, G turns gravity over
make demo-scenes                                                  # examples/start/scenes: a menu Stage and a game Stage
make demo-vision                                                  # examples/rules/vision
make demo-minimal                                                 # the README example
make bench                                                        # every benchmark once, with allocations
make bench-save                                                   # 5 repeats into bench_results/ (ignored by git)
```

Only `render/gpu` and `internal/engine` import gogpu (and wgpu, gputypes); plugins see `render`
and `control`. Tests that draw ready a headless device (`gpu.Headless`; `GRAM_GPU=software` for
the software rasteriser, which draws wrong) and skip without a GPU. A running game logs its frame
rate and where a frame's time goes every second with `GRAM_FPS_LOG=1`, starts fullscreen with
`GRAM_FULLSCREEN=1` (F11 switches) and runs unpaced with `GRAM_VSYNC=off`; headless probes of a
demo's frame include about 10 ms of the GPU's readback at 2560x1440, so compare them with each
other, not with a frame rate.

The `examples/<group>/<name>` programs are real GUI apps — `go test` cannot exercise them. To
check one still runs after a change: build to a temp path, run under `timeout <n>s`, treat exit
124 (still running) as healthy. That is no substitute for looking at the demo when a visual
change needs confirmation; for what a demo does tick by tick without a window, the `gram-probe`
skill.

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
  pause, Shift+Esc quits, K lists the keys, ~ the telemetry panel, F5 saves, F9 loads, F11 full
  screen, ] and [ the tempo, Shift+X the boxes (collision's), Shift+O the holds, Shift+B the
  bridge tool, Shift+R the routes, Shift+C the cones, Shift+W the weather, P freezes the light,
  Space jumps in first person, = and - zoom, Shift+= and Shift+- shape the relief. A plugin's keys a
  game sets up (`driving.Compass`, `selection.FollowKey`, the cameras' zoom keys) are bound in the
  Players section with the defaults; the game's own go to the Controls section and are listed under
  "Game".
- **Everything a game defines is its Stage's, by name**, each register the world's, so each
  Stage has its own and nothing is the program's; a game's arena holds plugins and players alone.
- **The order a game makes its plugins in is the order they are used in**; no list, no `ctx` in
  a game's code. A plugin needing another's logic takes it as a constructor argument.
- **A thing in its section**: what is defined in another section is refused (`gram-game`).
- **How things look is the scene's**: `picture()` in a demo's `scene.go` sets what is drawing
  (atlases, `WithAtmosphere`, styles, views) and nothing else does; `makePlugins` configures what
  a plugin does. A sprite in a Stage's struct is a leak. A look is drawing (`Under`), never an
  effect's Spec.
- **A map is a file** (`map.yaml` beside a demo's code). Entities standing on the board are
  entities, listed one by one, never placed by a legend's character. Kinds stay in Go.
- **What stands on the board is an entity, not terrain**: a lever, a trapdoor, a gate, a plate.
- **Terrain is never an entity in the space**; a bridge is an entity a cell.
- **Never make a unit see the others ahead**: it learns by striking (navigation); lanes were
  tried and dropped.
- **Physics, not blocking**, for a unit under gravity: into the water and off an edge as it
  pleases, the rules deciding.
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
contract (`Plugin`/`Installer`/`Serializable`/`PostLoader`/`Populator`/`CommandHandler`/`Made`,
`plugin/section`); `game` is what a Stage implements and receives; `internal/engine` the
`Engine`. Built-in plugins (`plugins/*`) import `plugin`/`render` directly, as a third-party
plugin would. gram's core beside them — `ceremony`, `entity` (with `entity/kind`), `clock`,
`control`, `camera` — imports no plugin; the world makes and runs their systems. A plugin's root
holds its entries, its public subpackages the vocabulary and contracts, `internal` the machinery
(`gram-plugin`). **Layering: the world knows its entities and nothing else; the ground is the
board's, the sky the atmosphere's, `render` generic.**

Built-in plugins (`gram-plugins`): `world` (the foundation: space, entities, movement, the
registers, the clock), `board` (grid and terrain, cells as entities, what stands on the board,
bridges, maps as files; `cmd/gramap` makes maps of the Earth), `topography` (a map in relief
and its cameras), `atmosphere` (the time of day, the climate, the weather), `collision`,
`navigation`, `selection`, `dialog`, `players` (with `players/owner`), `cameras`, `driving`,
`gravity`, `vision`, `bridges`, `bullet`, `telemetry` (the panel in the corner, a scene of its
own in every Stage with a world). Each package has a `doc.go` describing what it brings, ending
with its `# Vocabulary`.

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
