---
name: gram-game
description: How a gram game and a demo are written — the engine's loop and a Stage's entry, the Stage's sections and what each may define, the arena and a demo's files, figures, surfaces and skies, the ui, a plugin's own scene, the settings and controls screens, keys, saves, the window. Use when writing or changing a demo under examples/, a Stage, a scene, a menu, keys, or anything in game/, ui/ or internal/engine.
---

# gram-game

The API is `game/doc.go`, `game/stage/doc.go` and `ui/doc.go`; the house rules for a game and a
demo (sections, the arena, the scene's look, figures and tiles, maps, keys, settings, examples)
are CLAUDE.md's; a plugin's settings `gram-plugin`; each plugin `gram-plugins`. A menu Stage with
no world and a modal panel over the ticking world: `examples/start/scenes`.

## The engine and a Stage

- While a frame is drawn the next one ticks and publishes (`Props.Serial`, `Props.Workers`); the
  barrier between them takes the input and lays the scenes out. A stage's first frame and a
  switch run serially.
- Entering a Stage: Init, Load and Restore, `Loaded` or `Spawn`, the scenes, `Populate`. Every
  scene's `Drawers()`, hidden ones too, is built and `Init`ed once then; `ui.Scene.Replace`
  re-inits none.
- The engine's commands (`game.Save`, `Load`, `Enter`, `Quit`) are carried after the input,
  before the ticks, so a rule's `act.Do` saves too. A save test: `Props.Saves` a temp dir,
  `eng.Save("")`, a second engine `Load("", "")`, `Init`. A plan whose shape changed: its saves do
  not load. goke registers 512 component types at most, every fact one.
- A Stage has no `HandleEvents`. `Runtime` reaches a Stage and every Scene alike; a plugin's
  capability reaches a Scene by its constructor, never through `Runtime`.
- The `World(cfg).Plugins(f)` link (`game/stage/sections.go`) is the one place to attach a plugin
  to every Stage with a world: the telemetry is made there.

## Sections

- The chain: `stage.New(name).World(cfg).Plugins(a.makePlugins).Figures(a.defineFigures)
  .Players(…).Effects(…).Rules(…).Commands(…).Controls(…).Spawn(…).Scenes(…).Shows(…).Update(…)`.
  No demo uses `Kinds`: its kinds are figures. A plugin's own section is `With(...)` on a link
  (`With(dialog.Conversations(f))`); a Stage using no such plugin fails `Init`.
- **A thing in its section**: a plugin refuses what is defined in another through
  `world.Plugin.InSection(what, want...)`, by an error where the call returns one, else a panic:
  `ctx.Use`/`UseWorld` (Plugins), `players.Add`/`Local` (Players), effects (Effects), roles and
  `Plays` (Rules), commands (Commands), kinds and cell kinds (Kinds), `Player.Bind` (Players for
  the plugins' keys, Controls for the game's own), `world.Seed`, `board.Seed`/`Lay`,
  `topography.Seed` (Spawn), a dialog's `Load`/`Define`/`Changes`/`Conditions` (its part). A Stage
  written by hand is in no section, so every test installer works.

## A demo

- The sections are the arena's methods: `makePlugins`, `defineFigures`, `definePlayer`,
  `defineEffects`, `defineRules`, `defineCommands`, `bindKeys`, `spawn`, `defineScenes`,
  `update`. `defineScenes` only makes the `mainScene{arena: a, …}` of `scene.go`. A sun of the
  game's own moves by `WithDistant` in `update` before `a.light.RunPlan` (look/light's `sun.go`);
  a game's own `cell.Mobility`, handed to the board's `WithMotion`, lies in its `mobility.go`.
- The screen: the feed through a camera made beside it, a backdrop `Blank`, under an atmosphere
  its HUD (`a.atmosphere.Bar()` at an edge, or `TimeOfDay`, `Date`, `Weather` apart by the
  corners, 2026-10-20; board/topography's `-hud`). A big screen may go to a subpackage
  (board/topography's `hud`). The telemetry panel is the plugin's scene, never the demo's:
  `.Shows(MainScene, telemetry.SceneName)`.
- The `Figures` link notes the tiles, `a.board.Surfaces(surface{})`, the figures in the order
  their rules build on (the lever before the hero whose rules name `LeverRole`), a plugin's own
  kinds after the board's, the sky last (`a.atmosphere.Skies`; the Earth's is an `earth{}`
  assigning the four `backdrop.Earth*`). Every demo with a board has a `surface.go`, its defaults
  spelt out (`board.DefaultGridLine`, `board.DefaultHoldWidth`); a game's own sky's WGSL lies
  beside its `sky.go` (`//go:embed`). A pointer receiver is asserted as `(*hero)(nil)`.
- A figure's constructor takes its sheet alone; what a part needs (the board, `b.World()`) comes
  in the call. `Look` takes the atlas it draws on and the plugin whose register the kind is in. A
  figure built on another plugin's method takes that plugin (dialog's `host{talks: a.dialog}`).
- A figure's numbers (pixels, speed, timings, sheet columns) are lowercase constants beside it;
  `demo.go` keeps the game's (TPS, the grid, the screen, counts, a plugin's setting). Kinds alike
  are one type noted several times (topography's `walker{kind, color, ordered}`). The arena keeps
  a row several figures read (`unitRow`); a row one figure reads goes with it (`fordRow`).
- The sky shows only above the cameras' `MinPitch` (30° by default hides it; look/sky sets 2). Q
  and E turn about the ground mid-screen, far off once the stick lies flat: turn first, then lay
  it down (F).

## UI, screens and keys

- Two levels of composing, never mixed: the world is `render`'s (Z and the depth), the screen
  `ui`'s tree (the order its containers say); ui knows no camera.
- A menu built for a size rebuilds itself when the next tick finds it `Laid()` over another. A
  `LabelOf(Text)` reads its words several times a frame: cache per frame. `ui.Drawer` ignores its
  box and draws on the whole screen.
- A label, a picture and a `Blank` keep a click over their whole box, so a HUD over the picture
  covers it unless `Passing()` (2026-10-26): the plugins' HUD words (the atmosphere's panels, the
  bridges' `Reason`, the minimap's compass) are passing by the plugin, a game's own by the scene.
- A plugin's scene (`game.Scenic`) joins the stack hidden. The model is `players.ControlsScene`, a
  `game.Scene` wrapping a `ui.Scene`, since a `ui.Scene` is always focusable and a focusable scene
  on top takes all input.
- Only the active scene's `HandleEvents` gets the Composition, so a key showing another scene (K,
  F10) is carried there (`p.carry` in `players.Handle`): it works under the engine's pause, not
  while another scene is active. A screen forwards input to its `ui.Scene` and calls `p.carry`
  itself, so the players' own commands work while it is up.
- A queue drained in `RunPlan` does nothing under the engine's pause: hence the settings screen's
  tactical pause. What it sets has no key of the plugin's (the user's word, 2026-10-20).
- Both screens are one layout, `menu.go`'s `menu(...)` (the user's word, 2026-10-26: one
  aesthetic): the title framed at the top middle, framed blocks in columns in a `ui.Scroll`, the
  default theme. `Props.Description` is on no engine screen (2026-10-26): it is for a game's menu.
- The controls screen's "General" is the bindings whose handler is players or nil: a
  `CommandHandler` merely `Use`d, not handed to `players.NewPlugin`, is carried but listed there
  and left out of `Defaults()`. `SceneKey` compares Shift alone.
- V (2026-10-27) takes an orthographic camera into the perspective and letting go gives the view
  back. The arm behind a unit makes a Zelda, a racing or a Warcraft camera by three numbers, on
  F10 as switches with their sliders (`Range.Beside`, live while the switch is on). The zones,
  corners, drag and grab hold only where the hand is free (`camera.Free`), so none lets go of a
  camera kept behind a unit. The grab (off by default, 2026-10-27) yields to navigation's right
  drag while units are selected.
- Telemetry: `render.ActualFPS`, `render.Spent()`, `game.TPS.Ticks` (stale under the pause),
  `world.Res.Telemetry.Count`; a reporter reading the world is a `plugin.Bound`, readied through
  the panel's scene.

## The window (2026-10-26)

- gogpu runs `OnUpdate` (the adapter's `Collect`) on the main thread and `OnDraw` (the frame, with
  the input) on a render thread; AppKit takes a window's calls on the main thread alone, so
  `ToggleFullscreen` only asks and `Collect` switches at the window's next pass.
- Riding, the pointer takes the window's deltas; on macOS gogpu's lock is a stub (no deltas, the
  cursor never hidden), so the look follows the cursor's place and stops at the screen's edge.
  Where the window fills the screen a place is pinned to the last row and column (Cocoa's
  flipped origin puts the bottom row at h). F11 is `control.FullscreenKey()`: Ctrl+Cmd+F on darwin.
- `GRAM_INPUT_LOG=1` is the first thing to read when zones or clicks misbehave on a platform.

## Every demo in one pass

- A pattern introduced goes into every demo in the same pass (the user's word, 2026-10-14: a demo
  left in the old style is a "museum"), by forks per group when the pass is big (board, look,
  navigation, rules, motion/start), their output then reviewed for one style. A mechanical change
  is a `go/ast` tool in the scratchpad (parse, edit, `printer.Fprint`, `gofmt -w`). Check them
  all at once: each built to the scratchpad, run from its directory under `timeout 5s`, exit 124
  alive, the logs grepped for `panic`.
