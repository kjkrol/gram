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
played, for a menu of the game's own — the engine shows it nowhere since the controls screen
dropped it (2026-10-26); `Saves` the directory) and a named
collection of `game.Stage`s plus which starts (`Stages() (map[string]Stage, string)`);
`game.New(props, stages...)` is that `Game`, the first stage starting unless
`Defined.Starting(name)` says. A `Stage` has `Init`/`Spawn`/`Update` (`Loaded` in place of
`Spawn` from a save), each with its **own** `*goke.ECS`, built fresh (plus a fresh `world.Plugin`
if its chain begins with `World`) the moment `Runtime.SwitchStage` enters it, so a menu Stage
sits idle until the player starts the game. A `Stage` is never a `plugin.Plugin`: it installs
plugins. Its `Init` gets a `game.Initializer` (`Use`, `UseWorld`, `Track`, `TPS`, the embedded
`plugin.Installer`, `Commands`). **A `Stage` has no `HandleEvents`**: input is a `Scene`'s.

Within one active `Stage`, a `game.Scene` is `Name`, `Drawers() []render.Renderer` (built once on
entering the Stage; a picture or a source is no Renderer — the world is shown through a
`ui.Image` of a `render.Feed` of `world.Read(0).Picture()`; a drawer that reads the world is a
`plugin.Bound`, readied once by the engine), `HandleEvents`,
`Focusable`. `Stage.Stack()` is the
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
.Spawn(f...).Scenes(f).Shows(names...).Update(f)` hands back a `game.Stage`. Each link returns a
type holding the links after it, so a chain out of order does not compile, a link may be left out,
and `Update` alone must come. A section takes a function of whatever shape it needs (`stage.Step`);
`Kinds` and `Spawn` take several, run in order. `World(cfg)` makes the world anew each time the
Stage is entered (`Heights: true` wherever a board, a topography or an atmosphere stands; a side
view too, gravity pulling along Y); `Plugins(func(w *world.Plugin))` has the arena make the plugins
over it, used in the order made — `motion.NewPlugin(w)` first wherever anything moves, handed to
whatever steers or pushes (collision, bullet, driving, navigation, gravity; the board's units by
`WithMotion`), its `RunPlan` right after the world's in `Update`; `Plugins(func(made *plugin.Made))`
the same for a Stage without a world. **A plugin may have a section of its own**:
`section.Define(name, after, before)`, `section.Of[P](part, f)`, written with `With(sections...)` on
any link from `World` to `Controls` (out of place it panics as the chain is built; a Stage using no
`P` fails `Init`): `With(dialog.Conversations(f))` after the commands, before the kinds. The Stage
so built makes its scenes once the world is there — in `Loaded` from a save, else at the end of
`Spawn`, before `Populate` (nothing in them reads the entities spawned): the stack, the first scene
shown or those `Shows` names, the Composition and the ui scenes tracked. The order is what builds on
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
`mainScene{arena: a, …}` builds it — its colours and `screen()`: first the plugins' look
options (`vision.WithViews`, `board.ShowGrid(false)`, `WithAtmosphere`,
`a.atmosphere.CastClouds(atmosphere.CloudShadows{Ground, Figures, High})`, `a.light.WithDistant(…)`
where there is no sky (a sun of the game's own moves by `WithDistant` in `update` before
`a.light.RunPlan`, look/light's `sun.go`), `world.WithFog` where there is no atmosphere, the painter's styles,
`ShowHolds`, `ShowRoutes`, `ShowZones`), then the ui tree, the world shown as
`feed := render.NewFeed(cam, a.world.Read(0).Picture())` (2026-10-15: no `picture()`, no composer named
in a demo; every plugin made draws what it gave `ctx.Draws`, the atlases closed by the Stage
before the scenes) — and `defineScenes` only makes it. A minimap is the one picture of its own:
`ui.TopRight(a.minimap.For(cam, a.player.ID).Masked(ui.Circle).Border(rim, 2)).Size(220, 220)`
over the feed of the same `cam` (made once in `screen()`), `.Fill` where there is no ground
(motion/expanse); a press on it moves `cam`; its compass (`a.minimap.Compass(cam)`) is laid over
it in a box `minimap.Overhang` bigger, `Passing`, so the press and the corner zone under reach
through. See
`examples/start/scenes` for a menu Stage with no world, a ui scene of the engine's commands and a
togglable modal panel over the still-ticking world.

## UI: a scene's screen composed of elements (`ui`)

A Stage has many scenes, each a composition of a view built out of elements. Two levels of
composing, never mixed: **the world** (2.5D/3D) is `render`'s (a Composer, order by Z in a depth
buffer, `Direct`),
**the screen** (2D) is `ui`'s tree, the order its containers' names say. **ui knows no camera** and
no `render` rule: the world through a camera is a `render.Feed` (`render.NewFeed(cam, picture)`,
drawn every frame into an image of the size it is shown at; `ToWorld`/`ToPixels`), which
`ui.Image(feed)` shows. `ui.NewScene(name, screen)` is a `game.Scene`: `Input(players.Handle)`,
`Issue(players.IssueAs(pl))` for its buttons and keys, `Keys(bindings...)`, `Theme(th)`,
`Show`/`Hide`/`Toggle`/`Shown` by name, `Replace(screen)` (a tree put in after the frame's
publish published as the barrier lays the scene out), `Lay(screen)` (tests call it),
`Laid()` (the screen it was last laid over: a menu built for a size rebuilds itself when the
next tick finds it laid over another, as the players' settings and controls do); it saves
which named elements are shown (`Persisted`/`Restore`). **The notation says what it does**:
`ui.Layers(bottom, …, top)` covers, `ui.Columns`/`ui.Rows` split and never cover, each part
`ui.Share(n, el)`, `ui.Fixed(px, el)` (counted in what its split needs), `ui.Fit(el)`; an anchor
wraps its element — `ui.BottomMiddle(el).Size(w, h).Margin(px)`, nine of them. Elements: `Panel`,
`Label` (the theme's `Font`: `render.Font`, Go Regular, glyphs drawn on demand), `Heading` (the
theme's `Bold`), `Title` (its `Large`, Go Bold at 20), `Image` (a picture; a source with
`DrawGlass(dst)`, as a `render.Feed` with a `render.Glass` source, draws on it last of all,
over whatever lies over the picture), `Window`, `LabelOf`/`ButtonOf`/
`WindowOf`/`HeadingOf` (their words read off a `ui.Text` once a frame, as the scene's drawer — a
`plugin.Publisher` — publishes after the ticks, only while the scene is shown, a scene shown
after the frame's publish on the barrier, the frame laid, drawn and clicked as published; `ui.Of[T]` reads a component of the pinned
entity, `ui.While(on, text)` says text while `on()`; a Text saying nothing leaves the element out), `Button(label, cmds...)` (the scene's own
`ui.Show`/`Hide`/`Toggle{Name}`, or any command through `Issue`; no callbacks into the game),
`Blank`, `Drawer(renderer)`, `Scroll(el)` (an element taller than its box, scrolled by its bar or
the wheel, its content clipped to the box); the controls `Checkbox(label, get, set)`, `Slider(min, max, step,
get, set)` (dragged till the button is let go, the wheel over it a step; `Size` its length),
`Dropdown(options, get, set)` (its options opened over everything, in the scene's overlay; a
click outside or Esc closes them, `Scene.Unfolded`) — each `get` read once a frame as the scene publishes, each `set`
building the command for the new value, given like a button's; they read no entity. Any
element takes `Fill`, `Border`, `Padding`, `Named`, `Hidden`,
`Modal`, `Masked(ui.Circle | ui.Polygon(...))`, `Fraction(w, h)` (an axis 0 following the
proportions of the world a feed shows whole), `Steady` (never shrinks: a box round words that
change). `ui.Theme{Font, Bold, Large, Text, Shadow, Panel, Border, Title, Button, Accent}` is how
elements look where they say nothing of their own. A feed filling the screen is drawn straight
onto it. Input: a click goes to the topmost element hit — a button or a control takes it
(`pressable` in `input.go`), an
`Image(feed).Input(players.Through(pl))` lets it through to the bindings, any other element keeps
it — a label, a picture and a Blank over their whole box, so a HUD over the picture covers it
(no edge scroll, no wheel there) unless it is `Passing()` (2026-10-26): the pointer passes
through a passing element and all in it to what lies under, a button or a control inside aside;
the plugins' HUD words (the atmosphere's panels, the bridges' `Reason`, the minimap's compass)
are passing by the plugin, a game's own words over the picture (PAUSED) by the scene.
**Elements from the game are pinned to entities**, written as `Under` on the atlas:
`ui.Label("frozen").Under(effects.Named(FrozenEf))` is shown once for every entity the effect is on,
`.On(ceremony.Named/Group(...))` for those called so, `Where(ui.Tagged(t))` for those carrying a
tag; `Above`/`Below`/`Beside`/`Offset` say how it stands by its entity in every picture that has it
in sight (`ui.Owner`: a player's picture shows its own entities and nobody's);
`OffScreen(ui.PointAtIt | ui.ShowIt | ui.GoToIt)` what it does out of sight (`ui.Looker`:
`cameras.CenterOn`). A pinned element's button gives every `ceremony.Command` for `ui.It` for the
entity it is shown for (`ceremony.ID(id)`); a plugin's own command that is a `ui.About`
(`dialog.Choose`) gets the entity the same way. Every demo shows its world through a ui scene: the
feed through a camera made beside it, a backdrop `Blank`, the clock's HUD as a `ui.Drawer`, and
under an atmosphere its HUD elements (2026-10-20: `a.atmosphere.Bar()` at an edge, or
`TimeOfDay()`, `Date()` and `Weather()` apart by the corners; board/topography's `-hud
bar|widgets` flag beside `-look`). A demo with a big screen may compose it in a subpackage of
its own (board/topography's `hud`: `hud.Screen(hud.Deps{...})`), `scene.go` keeping the looks
and the camera; the telemetry panel is the plugin's scene, not the demo's.

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
noted (the lever before the hero whose rules name `LeverRole`); the looks are declared by the
Stage once it is set up (the world's and the board's `ctx.Looks`), so a scene names no atlas. The
arena's `Effects`/`Rules` keep what several kinds share; `Kinds` is gone from every demo. A figure
takes nothing in its constructor but its sheet: what a part needs (the board,
the world's registers through `b.World()`) comes in the call; the arena keeps the rows several
figures read (`unitRow`), the commands and the spawn; a row one figure reads goes with it
(`fordRow`). Kinds alike are one type of values noted several times (topography's `ground`
table of tiles, its `walker{kind, color, ordered}`). A plugin with a register of kinds of its
own gives its own `Figure` and `Figures`, the plugin coming in both calls (`bridges.Figure`:
`Define(p)`, `Look(a *world.Atlas, p)`, the bridge's Spec on `p.Types()`, its deck's and pier's
looks from `p.Types().Named(name)`; `bullet.Figure` the same over `p.Shots()`; `p.World()` the
registers), noted with `a.bridges.Figures(bridge{})`, `a.bullet.Figures(round{}, grenade{})`
after the board's. A game's own sky is the same kind of thing: a type in `sky.go`
(`var _ atmosphere.Sky = dusk{}`, `Look(d *backdrop.Dome, p *atmosphere.Plugin)` setting the
parts it draws its own way, its WGSL beside it, `//go:embed vault.wgsl`), noted last in the link,
`a.atmosphere.Skies(dusk{})`; a demo under the Earth's sky notes `earth{}` assigning the four
`backdrop.Earth*` parts. The board's surface likewise: every demo with a board has a
`surface.go` (`var _ board.Surface = surface{}`, `Look(a *board.Atlas, b *board.Plugin)` setting
`a.Grid(board.Line{...})` and `a.Holds(width)`; the defaults spelt out as
`board.DefaultGridLine(b.Board().Grid)` and `board.DefaultHoldWidth`), noted right after the
tiles, `a.board.Surfaces(surface{})`. In a perspective the sky shows only above the
cameras' `MinPitch` (`cameras.Config`; 30° by default hides it; look/sky sets 2). Q and E turn
about the ground in the middle of the screen, far off once the stick lies flat: turn first, then
lay it down (F). The rule: `Look`
takes the atlas it draws on and the plugin whose register the kind is in. A figure whose kind is
built on another plugin's method takes that plugin in its constructor (dialog's
`host{talks: a.dialog}` for `Script`), nothing else. A figure's numbers — its size in pixels,
its speed, its timings, its sheet's columns — are constants beside it, lowercase; `demo.go`
keeps the game's (TPS, the grid, the screen, the entity count, a plugin's setting like the
driving's tap). Coloured balls that are nothing but a kind (collision's `ball{color, shape}`,
minimal's `box{}`, scenes' `mover{}`) are a figure in `demo.go`. A `Put` in a plan lands at
the step's end, a `Hold` in a plan runs for its
loop's life: see the `ceremony` skill.

**A pattern introduced is applied to every demo in the same pass** (the user's word, 2026-10-14,
finding topography still a "museum" after three demos were converted): the user reads all the demos
(20 programs in 2026-10); convert them all, with forks by group (board, look, navigation, rules,
motion/start) when the pass is big, then review the forks' output for one style (shared sizes
lowercase, comments only where the code cannot say it). Mechanical changes across the demos are a
`go/ast` tool in the scratchpad (parse, edit, `printer.Fprint`, then `gofmt -w`): the `s`→`a`
receiver rename and the `var _ board.Figure = …` assertions were made so, one tool each; a shell
loop strips unused imports from `go build`'s lines. Check all demos at once: build each to the
scratchpad and run it from its directory under `timeout 5s`, exit 124 alive, grep the logs for
`panic`.

## A plugin's own scene, input and keys (facts found in the code)

- A plugin's scene (`game.Scenic`, `Scenes()`) is gathered by `PluginScenes()` into the stack
  of a Stage built with `stage.New`, **hidden**; `.Shows(names...)` shows it from the start; the
  Composition's order is saved, so a loaded game shows what was shown. The model is
  `players.ControlsScene`: a `game.Scene` wrapping a `ui.Scene`, because `ui.Scene.Focusable()` is
  always true and a focusable scene on top takes all input. Only the active scene's
  `HandleEvents` (`players.Handle`) gets the `Composition` (`Show`/`Hide`/`Active`), so a key
  that shows another scene is carried there (K, F10: `p.carry` in `Handle`), and works while
  the engine is paused (input runs before the pause check) but not while another scene is
  active. A plugin's queue drained in `RunPlan` (the board's `Grid`, vision's `Cones`) does nothing
  under the engine's pause — which is why the settings screen (`players.SettingsScene`, F10)
  holds the game in the clock's tactical pause instead: `RunPlan` runs, the controls' commands
  are carried out, the picture redraws. A plugin binds no key to what its settings set (the
  user's word, 2026-10-20): the screen is where a player sets them; a game may bind a key of
  its own to the command. It forwards its input to its `ui.Scene` (the controls screen too,
  for its scroll) and calls `p.carry` itself, so the players' own commands (`KeepRunning`,
  `telemetry.Panel`) work while it is up. Both screens are one layout, `menu.go`'s `menu(name,
  title, blocks, w, h)` (the user's word, 2026-10-26: one aesthetic): the title in a framed
  panel at the top middle (`<name>.title`), the blocks (`block`, `framed`) in columns
  (`<name>.column.N`) in a `ui.Scroll` (`<name>.scroll`), in the default theme — its `Large`
  is `render.DefaultLargeFont`, Go Bold 28, the settings' former own theme made the default.
- The controls screen lists "General" = bindings whose handler is players or nil
  (`controls.go` `groups`);
  `handlerOf` searches only the handlers given to `players.NewPlugin`: a CommandHandler merely
  `Use`d is still carried (`initializer.go`), but listed under "General" and left out of
  `Defaults()`. `control.Mods` has `Ctrl` (the cameras' sight) and `Alt` (no default
  key; `withMods` writes "Ctrl+Alt+Shift+"); `SceneKey` compares Shift alone.
- A game lets the right button grab the world and drag it with `cameras.Config{Grab: true}` in
  `makePlugins` (`cameras.Keys{Grab: true}` for keys of its own); off by default (2026-10-27).
  The grab yields (`control.Binding.Yielding`) to navigation's right drag, which turns the
  selected units only while there are any: with nothing selected the drag grabs the world, with
  a selection it turns the units; a right click sends them either way. A click's slop is
  `control.DragSlop` (`Context.Dragged`): the grab takes the whole way on its first pass past it.
- Every scene's `Drawers()`, hidden ones included, is built and `Init`ed once on entering the
  Stage (`stage_runtime.go`), the world's picture once however many scenes show it;
  `ui.Scene.Replace` does not re-init drawers. A `ui.LabelOf(Text)`
  reads its words several times a frame (`needs`, `absent`, `draw`): cache per frame.
  `ui.Drawer(renderer)` ignores its box and draws on the whole screen; `render.DebugPrint` starts
  at the corner, which is why the old telemetry sat top-left.
- Telemetry numbers (the panel's basic mode shows FPS, TPS, Frame and Entities, the extended
  one every line; a many-valued line is a value of lines under its label): `render.ActualFPS`
  (set by the engine once a second), `render.Spent()` (where a frame's time went, set beside them),
  `game.TPS.Ticks` (`ctx.TPS()`, stale under pause), `world.Res.Telemetry.Count` (spawned
  entities), `view.View.In.Len()` per camera when culled. A `render.Reporter` that reads
  the world is a `plugin.Bound` too (climate's builds a query), readied through the telemetry
  panel's scene. A demo's screen has no telemetry layer: the
  panel is the plugin's scene, shown with `.Shows(MainScene, telemetry.SceneName)`; reporters
  are added in `makePlugins` or the Scenes section (`s.telemetry.With(...)`).
- The one place to attach a plugin to every Stage with a world is the `World(cfg).Plugins(f)`
  link (`game/stage/sections.go`): it runs `f(w)` and uses `w.Made().All()`.

## Game / persistence

`internal/engine.Engine` owns the window's loop (every frame `Update`, `Draw` into an image the
size of the screen, `Present`; since 2026-10-24 the next frame's ticks and publishing run on a
goroutine of their own while a frame is drawn, the barrier between taking the input — shown a
frame later — and laying the scenes out; a stage's first frame, a switch's and every frame with
`Props.Serial` or `GRAM_SERIAL` set run serially; the game's pool is then one short of the CPUs unless
`Props.Workers` says) and drives one active `Stage` at a time, calling `game.Stages()`
whenever it resolves a name. Its `Persistence()` (`Save`/`Load`/`List`) is bound to the active
Stage's ECS; a tracked `plugin.Serializable`/`PostLoader` is included automatically; saved
resources are matched by name (`Name()`, or the Go type name), so a save survives plugins being
added, removed or reordered (`internal/engine/persist.go`). **Saving and loading are the
engine's**: commands like any other — `game.Save{Slot}`, `game.Load{Stage, Slot}` (a save not
there is an error and nothing changes), `game.Enter{Stage}`, `game.Quit{}` — bound to keys
(`GameBindings`) or ui buttons (through `Runtime.Issue`); the engine's queues are in every
Stage's carrier and are carried out after the input, before the ticks, so a rule's `act.Do`
saves too. A save is a Stage's: `filepath.Join(Props.Saves, stage)`, a file a slot. `enterStage`
loads after Init, before Setup, and calls `Stage.Loaded()` in place of `Spawn`, so the scenes are made after the load — a start look set
in `screen()` would clobber a loaded setting, which is why a plugin's start (`ShowGrid(false)`,
`ShowCones()`) is set in `makePlugins`. A plugin's settings are `plugin.Keep(&p.Res.Settings)`
(`gram-plugin`); the tests' way:
`Props.Saves` a temp dir, `eng.Save("")`, a second engine `Load("", "")`, `Init`. A plan's shape
changed, its saves do not load; goke registers 512 component types at most, every fact one.

**The window** (`internal/engine/desktop_adapter.go`, 2026-10-26): gogpu runs `OnUpdate` (the
adapter's `Collect`) on the main thread and `OnDraw` (the engine's `frame`, with the input) on a
render thread; AppKit takes a window's calls on the main thread alone, so `ToggleFullscreen` (F11,
`Runtime`) only asks and `Collect` switches at the window's next pass, as it asks for
`GRAM_FULLSCREEN` on a Mac, where the window's config carries no full screen. The `pointer` folds
a pass's events: free, the cursor's place; caught (riding), the window's deltas — or, where the
window never gives one (macOS: gogpu's lock is a stub, its events carry no deltas, and
`CursorNone` is never unhidden, so the cursor stays visible), the movement of its place, the look
stopping at the screen's edge. Where the window fills the screen a place is pinned to the last
row and column (Cocoa's flipped origin puts the bottom row at h). The full-screen key the
controls screen names is `control.FullscreenKey()`, the engine setting Ctrl+Cmd+F on darwin.
`GRAM_INPUT_LOG=1` logs the cursor's place, the window's sizes and flags once a second and every
click: the first thing to read when zones or clicks misbehave on another platform.
