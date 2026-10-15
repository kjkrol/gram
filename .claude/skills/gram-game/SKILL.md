---
name: gram-game
description: How a gram game and a demo are written — game.Game, Props, Stage and Scene, the Stage defined a section at a time (stage.New…Update) and what each section may define, a plugin's own section, the arena and demo.go/scene.go/names.go, the ui tree (Layers, Columns, Rows, elements, pinned elements, feeds), players and keys, saving and loading. Use when writing or changing a demo under examples/, a Stage, a scene, a menu, keys, or anything in game/, ui/ or internal/engine.
---

# gram-game

## Stage / Scene

A `game.Game` supplies `Props()` (window and tick-rate config, read once by `gram.Run(g)`;
`Resizable` makes the screen the window, F11 full screen; `TargetTPS` the engine's fixed step —
one `Update` a frame stepping as many times as the time gone says, at most 5, so the game slows
instead of spiralling; `Title` the window's, `Description` what the game is and how it is
played, shown under the title of the shortcuts list; `Saves` the directory) and a named
collection of `game.Stage`s plus which starts (`Stages() (map[string]Stage, string)`);
`game.New(props, stages...)` is that `Game`, the first stage starting unless
`Defined.Starting(name)` says. A `Stage` has `Init`/`Spawn`/`Update` (`Loaded` in place of
`Spawn` from a save), each with its **own** `*goke.ECS`, built fresh (plus a fresh `world.Plugin`
if its chain begins with `World`) the moment `Runtime.SwitchStage` enters it, so a menu Stage
sits idle until the player starts the game. A `Stage` is never a `plugin.Plugin`: it installs
plugins. Its `Init` gets a `game.Initializer` (`Use`, `UseWorld`, `Track`, `TPS`, the embedded
`plugin.Installer`, `Commands`). **A `Stage` has no `HandleEvents`**: input is a `Scene`'s.

Within one active `Stage`, a `game.Scene` is `Name`, `Layers() []render.Layer` (built once on
entering the Stage; a world layer listed straight in a scene is refused — the world is shown
through a `ui.Image` of a `render.Feed`), `HandleEvents`, `Focusable`. `Stage.Stack()` is the
`Name()`-keyed registry of every `Scene` it can show (`game.NewStack(scenes...)`);
`Stack.Composition()` the live state — which scenes are visible, in what order, which is
`Active()` (the topmost `Focusable`; a HUD on top never steals input). Every tick the engine
calls `HandleEvents` on the active scene alone. `Composition` embeds `plugin.Serializable`, so a
Stage tracks it (`ctx.Track`) and visibility survives a save. `Runtime` (`Paused`/`Pause`/
`Resume`/`TogglePause`/`Quit`/`SwitchStage`/`Save`/`Load`/`Issue`/`Persistence`/`TPS`/
`ToggleFullscreen`/`Props`) is one undivided interface reaching a Stage and every Scene alike. A
plugin's capability reaches a Scene by constructor injection, never through `Runtime`.

**A Stage is defined a section at a time** (`game/stage`): `stage.New(name).World(cfg)
.Plugins(a.makePlugins).Players(f).Effects(f).Rules(f).Commands(f).Kinds(f...).Controls(f)
.Spawn(f...).Scenes(f).Shows(names...).Update(f)` hands back a `game.Stage`. Each link returns
a type holding the links after it, so a chain out of order does not compile, a link may be left
out, and `Update` alone must come. A section takes a function of whatever shape it needs
(`stage.Step`); `Kinds` and `Spawn` take several, run in order. `World(cfg)` makes the world
anew each time the Stage is entered (`Heights: true` wherever a board, a topography or an
atmosphere stands; a side view too, gravity pulling along Y); `Plugins(func(w *world.Plugin))` has the arena make the
plugins over it, used in the order made; `Plugins(func(made *plugin.Made))` the same for a Stage
without a world. **A plugin may have a section of its own**: `section.Define(name, after,
before)`, `section.Of[P](part, f)`, written with `With(sections...)` on any link from `World` to
`Controls` (out of place it panics as the chain is built; a Stage using no `P` fails `Init`):
`With(dialog.Conversations(f))` after the commands, before the kinds. The Stage so built makes
its scenes once the world is there — in `Loaded` from a save, else at the end of `Spawn`, before
`Populate` (nothing in them reads the entities spawned): the stack, the first scene shown or
those `Shows` names, the Composition and the ui scenes tracked. The order is what builds on
what: players before the kinds, effects before the rules, roles before the commands for their
players and the kinds playing them, everything before the keys, the world before the scenes.

**A thing in its section**: the Stage tells the engine which part begins (`plugin/section`:
`Part`, `Reader`, `Writer`, `Check`, `Must`), and what is defined in another is refused — by an
error where the call returns one, a panic elsewhere, through `world.Plugin.InSection(what,
want...)`: `ctx.Use`/`UseWorld` (Plugins), `players.Add`/`Local` (Players), `Effects.Define`
(Effects), `Roles().Define`/`Plays` (Rules), `Commands().Define` (Commands), `cell.Kinds.Define`/
`kind.Define` (Kinds), `Player.Bind` (Players for the plugins' keys, Controls for the game's
own), `board.Seed`/`Lay`, `topography.Seed`, `world.Seed` (Spawn), a dialog's `Load`/`Define`/
`Changes`/`Conditions` (its part). A Stage written by hand is in no section: nothing is refused,
so every test installer works.

A demo groups its plugins in an **arena**, the collector every section builds on: `main` in
`demo.go` defines the game where it runs it (`props := game.Props{…}`, `a := &arena{}`,
`mainStage := stage.New(name).World(cfg).Plugins(a.makePlugins)…Update(a.update)`,
`gram.Run(game.New(props, mainStage))`); the sections are its methods (`makePlugins`,
`definePlayer`, `defineEffects`, `defineRules`, `defineCommands`, `defineCells`, `defineKinds`,
`bindKeys`, `spawn`, `defineScenes`, `update`). The scene is apart in `scene.go`: a
`mainScene{arena: a, …}` builds it — its colours, `picture()` (the plugins' look options —
`vision.WithViews`, `board.ShowGrid(false)`, `WithAtmosphere`, the painter's styles — and the
composer of the plugins' `Renderer()`s, `a.cameras.Renderer()` among them for the scroll
zones; no atlas: `world.Renderer()` and `board.Renderer()`
dress themselves from the figures and tiles when first asked, the board's sprites the cell's
size across) and `screen()` (the ui tree) — and `defineScenes` only makes it. See
`examples/start/scenes` for a menu Stage with no world, a ui scene of the engine's commands and a
togglable modal panel over the still-ticking world.

## UI: a scene's screen composed of elements (`ui`)

A Stage has many scenes, each a composition of a view built out of elements. Two levels of
composing, never mixed: **the world** (2.5D/3D) is `render`'s (a Composer, tiers, depth, `Direct`),
**the screen** (2D) is `ui`'s tree, the order its containers' names say. **ui knows no camera** and
no `render` rule: the world through a camera is a `render.Feed` (`render.NewFeed(cam, picture)`,
drawn every frame into an image of the size it is shown at; `ToWorld`/`ToPixels`), which
`ui.Image(feed)` shows. `ui.NewScene(name, screen)` is a `game.Scene`: `Input(players.Handle)`,
`Issue(players.IssueAs(pl))` for its buttons and keys, `Keys(bindings...)`, `Theme(th)`,
`Show`/`Hide`/`Toggle`/`Shown` by name, `Replace(screen)`, `Lay(screen)` (tests call it); it saves
which named elements are shown (`Persisted`/`Restore`). **The notation says what it does**:
`ui.Layers(bottom, …, top)` covers, `ui.Columns`/`ui.Rows` split and never cover, each part
`ui.Share(n, el)`, `ui.Fixed(px, el)` (counted in what its split needs), `ui.Fit(el)`; an anchor
wraps its element — `ui.BottomMiddle(el).Size(w, h).Margin(px)`, nine of them. Elements: `Panel`,
`Label` (the theme's `Font`: `render.Font`, Go Regular, glyphs drawn on demand), `Heading` (the
theme's `Bold`), `Title` (its `Large`, Go Bold at 20), `Image`, `Window`, `LabelOf`/`ButtonOf`/
`WindowOf` (their words read off a `ui.Text` every frame; `ui.Of[T]` reads a component of the pinned
entity; a Text saying nothing leaves the element out), `Button(label, cmds...)` (the scene's own
`ui.Show`/`Hide`/`Toggle{Name}`, or any command through `Issue`; no callbacks into the game),
`Blank`, `Layer(renderer)`; any element takes `Fill`, `Border`, `Padding`, `Named`, `Hidden`,
`Modal`, `Masked(ui.Circle | ui.Polygon(...))`, `Fraction(w, h)` (an axis 0 following the
proportions of the world a feed shows whole), `Steady` (never shrinks: a box round words that
change). `ui.Theme{Font, Bold, Large, Text, Shadow, Panel, Border, Title, Button}` is how elements
look where they say nothing of their own. A feed filling the screen is drawn straight onto it.
Input: a click goes to the topmost element hit — a button gives its commands, an
`Image(feed).Input(players.Through(pl))` lets it through to the bindings, any other element keeps
it. **Elements from the game are pinned to entities**, written as `Under` on the atlas:
`ui.Label("frozen").Under(effects.Named(FrozenEf))` is shown once for every entity the effect is on,
`.On(ceremony.Named/Group(...))` for those called so, `Where(ui.Tagged(t))` for those carrying a
tag; `Above`/`Below`/`Beside`/`Offset` say how it stands by its entity in every picture that has it
in sight (`ui.Owner`: a player's picture shows its own entities and nobody's);
`OffScreen(ui.PointAtIt | ui.ShowIt | ui.GoToIt)` what it does out of sight (`ui.Looker`:
`cameras.LookAt`). A pinned element's button gives every `ceremony.Command` for `ui.It` for the
entity it is shown for (`ceremony.ID(id)`); a plugin's own command that is a `ui.About`
(`dialog.Choose`) gets the entity the same way. Every demo shows its world through a ui scene: the
feed through a camera made beside it, a backdrop `Blank`, the telemetry and the clock's HUD as
`ui.Layer`s.

## Figures and tiles

A demo's kinds are figures (`world.Figure`: `Define(w)`, `Look(a, w)`; on a board
`board.Figure`: `Define(b)`, `Look(a, w)`) and tiles (`board.Tile`: `Define(b)`, `Look(a, b)`),
each in its own file with its sheet, its frames and, as `Stateful`/`Ruled`, its own `Effects`
and `Rules`, every interface it implements asserted beside the type — `var _ board.Figure =
hero{}` for one, a `var (…)` block for several (`board.Figure`, `board.Stateful`,
`board.Ruled`; `(*hero)(nil)` for pointer receivers): `Stateful` and `Ruled` are found by
assertion at run time, so without the `var _` a misspelt `Effects` is silently skipped. The
chain's `Figures(a.defineFigures)` link, right after `Plugins`, notes them —
`a.board.Tiles(grass{}, pit{})`, `a.board.Figures(newLever(), newHero(), plate{})` — and the
Stage runs their parts in Effects, Rules and Kinds after the game's own functions, in the order
noted (the lever before the hero whose rules name `LeverRole`); the looks are gathered by
`world.Renderer()`/`board.Renderer()` themselves, so `picture()` names no atlas. The arena's
`Effects`/`Rules` keep what several kinds share; `Kinds` is gone from every demo. A figure
takes nothing in its constructor but its sheet: what a part needs (the board,
the world's registers through `b.World()`) comes in the call; the arena keeps the rows several
figures read (`unitRow`), the commands and the spawn; a row one figure reads goes with it
(`fordRow`). Kinds alike are one type of values noted several times (topography's `ground`
table of tiles, its `walker{kind, color, ordered}`). A plugin with a register of kinds of its
own gives its own `Figure` and `Figures`, the plugin coming in both calls (`bridges.Figure`:
`Define(p)`, `Look(a *world.Atlas, p)`, the bridge's Spec on `p.Types()`, its deck's and pier's
looks from `p.Types().Named(name)`; `bullet.Figure` the same over `p.Shots()`; `p.World()` the
registers), noted with `a.bridges.Figures(bridge{})`, `a.bullet.Figures(round{}, grenade{})`
after the board's. The rule: `Look` takes the
atlas it draws on and the plugin whose register the kind is in. A figure whose kind is
built on another plugin's method takes that plugin in its constructor (dialog's
`host{talks: a.dialog}` for `Script`), nothing else. A figure's numbers — its size in pixels,
its speed, its timings, its sheet's columns — are constants beside it, lowercase; `demo.go`
keeps the game's (TPS, the grid, the screen, the entity count, a plugin's setting like the
driving's tap). Coloured balls that are nothing but a kind (collision's `ball{color, shape}`,
minimal's `box{}`, scenes' `mover{}`) are a figure in `demo.go`. A `Put` in a plan lands at
the step's end, a `Hold` in a plan runs for its
loop's life: see the `ceremony` skill.

**A pattern introduced is applied to every demo in the same pass** (the user's word,
2026-10-14, finding topography still a "museum" after three demos were converted): the user
reads all 24 demos; convert them all, with forks by group (board, look, navigation, rules,
motion/start) when the pass is big, then review the forks' output for one style (shared
sizes lowercase, comments only where the code cannot say it). Mechanical changes across the
demos are a `go/ast` tool in the scratchpad (parse, edit, `printer.Fprint`, then `gofmt -w`):
the `s`→`a` receiver rename and the `var _ board.Figure = …` assertions were made so, one
tool each; a shell loop strips unused imports from `go build`'s lines. Check all demos at
once: build each to the scratchpad and run it from its directory under `timeout 5s`, exit 124
alive, grep the logs for `panic`.

## A plugin's own scene, input and keys (facts found in the code)

- A plugin's scene (`game.Scenic`, `Scenes()`) is gathered by `PluginScenes()` into the stack
  of a Stage built with `stage.New`, **hidden**; `.Shows(names...)` shows it from the start; the
  Composition's order is saved, so a loaded game shows what was shown. The model is
  `players.Shortcuts`: a `game.Scene` wrapping a `ui.Scene`, because `ui.Scene.Focusable()` is
  always true and a focusable scene on top takes all input. Only the active scene's
  `HandleEvents` (`players.Handle`) gets the `Composition` (`Show`/`Hide`/`Active`), so a key
  that shows another scene is carried there (K: `p.listings.Drain` in `Handle`), and works while
  the engine is paused (input runs before the pause check) but not while another scene is
  active. A plugin's queue drained in `RunPlan` (board's B, vision's Shift+C) does nothing
  under the engine's pause.
- K lists "General" = bindings whose handler is players or nil (`shortcuts.go` `groups`);
  `handlerOf` searches only the handlers given to `players.NewPlugin`: a CommandHandler merely
  `Use`d is still carried (`initializer.go`), but listed under "General" and left out of
  `Defaults()`. `control.Mods` has `Ctrl` and `Alt` (unused so far; `withMods` writes
  "Ctrl+"); `SceneKey` compares Shift alone.
- Every scene's `Layers()`, hidden ones included, is built and `Init`ed once on entering the
  Stage (`stage_runtime.go`); `ui.Scene.Replace` does not re-init layers. A `ui.LabelOf(Text)`
  reads its words several times a frame (`needs`, `absent`, `draw`): cache per frame.
  `ui.Layer(renderer)` ignores its box and draws on the whole screen; `render.DebugPrint` starts
  at the corner, which is why the old telemetry sat top-left.
- Telemetry numbers: `render.ActualFPS/ActualTPS` (set by the engine once a second with one
  rate for both), `render.Spent()` (where a frame's time went, set beside them),
  `game.TPS.Ticks` (`ctx.TPS()`, stale under pause), `world.Res.Telemetry.Count` (spawned
  entities), `view.View.In.Len()` per camera when culled. `render.Reporter` needs
  `Init(*goke.SysInit)` (climate's builds a query), given through a layer's `Init`: the
  telemetry panel's scene returns such a layer. A demo's screen has no telemetry layer: the
  panel is the plugin's scene, shown with `.Shows(MainScene, telemetry.SceneName)`; reporters
  are added in `makePlugins` or the Scenes section (`s.telemetry.With(...)`).
- The one place to attach a plugin to every Stage with a world is the `World(cfg).Plugins(f)`
  link (`game/stage/sections.go`): it runs `f(w)` and uses `w.Made().All()`.

## Game / persistence

`internal/engine.Engine` owns the window's loop (every frame `Update`, `Draw` into an image the
size of the screen, `Present`) and drives one active `Stage` at a time, calling `game.Stages()`
whenever it resolves a name. Its `Persistence()` (`Save`/`Load`/`List`) is bound to the active
Stage's ECS; a tracked `plugin.Serializable`/`PostLoader` is included automatically; saved
resources are matched by name (`Name()`, or the Go type name), so a save survives plugins being
added, removed or reordered (`internal/engine/persist.go`). **Saving and loading are the
engine's**: commands like any other — `game.Save{Slot}`, `game.Load{Stage, Slot}` (a save not
there is an error and nothing changes), `game.Enter{Stage}`, `game.Quit{}` — bound to keys
(`GameBindings`) or ui buttons (through `Runtime.Issue`); the engine's queues are in every
Stage's carrier and are carried out after the input, before the ticks, so a rule's `act.Do`
saves too. A save is a Stage's: `filepath.Join(Props.Saves, stage)`, a file a slot. `enterStage`
loads after Init, before Setup, and calls `Stage.Loaded()` in place of `Spawn`; the tests' way:
`Props.Saves` a temp dir, `eng.Save("")`, a second engine `Load("", "")`, `Init`. A plan's shape
changed, its saves do not load; goke registers 512 component types at most, every fact one.
