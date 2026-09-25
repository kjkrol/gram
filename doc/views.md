# Views, players and commands: from a rectangle of the world to a network

[← Back to README](../README.md)

> A design sketch, not a contract: what the layers are, who owns what, and what each one leaves
> to the next. Code exists for the first two layers: `world.View` and `plugins/players` (one local
> player). Two things came out differently from the sketch below: the owning plugin keeps a
> queue and drains its commands in its own pass instead of a receiver run in players' pass, and the
> direction is the player's — the vocabulary (`control.Queue`, `control.Binding`) and the contract
> (`plugin.CommandHandler`) live below, and players is built over the command handlers, so a plugin with
> commands never knows players.

## 1. `world.View` — a rectangle and the entities in it

A `View` is what one pair of eyes sees: a rectangle of the world (`Bounds`) and the entities the
Space finds in it (`In`, an `EntitySet` — a set of the world's entities by index). The world keeps
any number of Views; each is made over a source of bounds (`world.Plugin.NewView(func() geom.AABB)`)
and refreshed by the `ViewSystem` once a tick, right after movement has rebuilt the Space. A View
whose bounds cover the whole world is not queried and simply sees everything; so does the zero
View a Stage has before its first tick.

The camera's View (`Plugin.View()`) is made by the world plugin itself, and the entity renderer
draws what it contains and nothing else. A View knows neither who is looking nor why.

Why a set and not a list: a set of entity indices masks a sequential walk over the ECS — one bit
test per entity, the real work only for the entities in view — and two Views combine as bit
operations (who is in both, who is in one and not the other) without any deduplication.

## 2. Players — a plugin: eyes, hands and a voice

A *player* is whoever acts in the game and may look at a part of it: a person at the keyboard, an
AI, a client somewhere on the network. A player has an identity, a **view** (a `world.View` over
its own `camera.Camera`, whose `State` is saved with the game — an AI gets a view of its part of
the board exactly as a person does), a rectangle of the screen when it looks locally (two players
on one screen is split screen: two cameras, two rectangles, one world), a **command queue**, and a
**translator** that fills the queue. Players are a plugin (`plugins/players`), because a game with
one player looking at the whole screen should not have to know about any of this.

### Three kinds of events, kept apart

- A **device event** is what `control` delivers today: a key, a click, a scroll. It stays as it is.
- A **command** is an intention in the game's vocabulary, as data: `MoveTo{Units, Cell}`,
  `Select{Box, Additive}`, `Pan{Delta}`, `Zoom{…}`, `Save{}`, and whatever the game adds. A command
  is typed, says which player issued it and in which tick, and does not know where it came from.
- A **world event** is what plugins already publish through behaviors — a `collision.Meeting`, a
  `vision.Sighting` — the world telling the game what happened. Commands go the other way.

Before the players plugin, gram had commands without the name, one private channel per plugin: `selection.Resources`
holds a `Pending` selection and `PendingIDs`, `navigation.Resources` a `PendingTarget`, and vision's
behaviors call `world.Steering.Request` directly. Each channel is fed by that plugin's own
`control.EventHandler`, so it is glued to the mouse: an AI cannot order a unit to move or select a
group, because there is no way to say so except by clicking.

### Translators

Whatever fills a player's queue is a translator, and there are three:

- **Bindings**, for a person: the player's definition says which device events become which
  commands. Screen coordinates pass through *this player's* camera, which is why the translator
  belongs to the player and not to a plugin.
- **A brain**, for an AI: a system or a behavior that looks through the player's `View` and its
  perception (vision) and issues commands into the same queue.
- **A frame decoder**, for a remote client (layer 3).

Plugins and the game never ask which of the three spoke.

### The vocabulary is the plugins'

Each plugin defines the command types of its own domain — `navigation.MoveTo`,
`selection.Select`, `world.Pan` and `world.Zoom` — and the game adds its own, exactly as the
payloads of behaviors work today (`Meeting` is collision's, `Sighting` is vision's). Whoever
defines a type also registers its receiver. Navigation's first word exists already:
`MoveCommand{Cell, Append}`, filled by its default event handler (now `MoveTo`); the player layer is where it
becomes a labelled binding. "What is under the cursor" — a hover — is the translator's too: a
`Space.Query` at a point, no collision involved.

### Default bindings are the plugins' too

`selection.DefaultEventHandler` is, in substance, a set of bindings: left click → `Select`,
drag → `Select{Box}`, shift → `Additive`. That pattern stays: a plugin ships a default set
(`selection.DefaultBindings()`, `navigation.DefaultBindings()`, world's camera controls), and the
game, defining a player, takes it whole, overrides single entries, or leaves it out. Two bindings
on one trigger for one player is an error when the set is assembled, never a silent last-one-wins.

### A binding carries a label

A binding is a trigger (key, button or gesture, with modifiers), the command it issues — or a
function building the command from context, such as the cursor's position through the player's
camera — and a **label** saying what it does: "Move selected units here", "Select", "Save game".
A player's set of bindings is therefore data: a help screen showing what a player can do is a
renderer over that set, per player; labels can be localised later; and the same list tells an AI
which moves exist, since the vocabulary is one.

### How plugins listen

`players.On[Cmd](func(t plugin.Tick, p *Player, c Cmd))`, the way behaviors are registered today,
by payload type: navigation registers `On[MoveTo]`, selection `On[Select]`, the world's camera
`On[Pan]` and `On[Zoom]`, the game `On[Save]`. A command type nobody listens to is an error at
registration, as `plugin.ErrUnhostedBehavior` is for a behavior in the wrong place, never a silent
no-op.

### Order within a tick

Input is captured → every player's translator runs and fills its queue → systems consume the
queues in `RunPlan` → the queues are cleared. `game.Scene.HandleEvents` stays for what is not a
game event: menus, pause, switching Stages.

### What it replaces

`selection.DefaultEventHandler`, `navigation.DefaultCommandEventHandler` and world's camera handler
become the default bindings producing commands; the `Pending*` fields of the plugins' `Resources`
go. `world.Steering.Request` stays as motion mechanics — how an entity carries out a heading — and
an AI's intention arrives as a command (`Steer{Entity, Dir}`, or higher, `MoveTo`); how those two
levels meet in one actuator, and how a unit follows a route through it, is [movement.md](movement.md).

### What it buys

Commands are data. A network carries commands, not keystrokes. Commands recorded per tick replay a
game deterministically, which also means tests of a whole game without a window. A person and an
AI speak one vocabulary and are treated alike by every plugin.

### Deliberately not

Not an event bus of strings and `any`: commands are types, so the compiler and `On[T]` say who
handles what. Not a player as an entity: a player in a strategy game commands many units; if it
has an avatar, that is an ordinary entity the player issues commands to.

## 3. Networking — a plugin over players

The server is one instance of the engine: Stages, ECS, plugins, ticking as always. A remote client
is a player whose translator is a frame decoder and whose view is shown elsewhere. The client sends
its commands and its `camera.State`; the server runs the commands — authority is the server's, so
the question of who owns the input does not arise — and sends back frames built from the player's
`View`:

- **Deltas from the View.** The View keeps last tick's set beside this tick's: entered (the client
  creates a sprite), left (it drops one), updated (positions). A client that just joined gets
  everything as entered.
- **One walk for all clients.** One pass over `Base`+`Appearance` serves every client: for up to 64
  clients a `uint64` mask per entity (bit = that client's View contains it), and the entity is
  appended to the frame of each client in its mask; more clients group into more masks. It is the
  renderer's principle again: a set masks a sequential walk, no list per client.
- **A frame** per tick per client: the tick number, the client's camera state echoed back,
  `entered: [id, kind, sprite, box]`, `updated: [id, box]`, `left: [id]`. Binary, little-endian;
  quantised positions later, if bandwidth asks.
- **The client** has no ECS and no Space: a camera, an atlas (kinds and their sprites agreed when
  it joins), a `render.QuadBatch`, frames coming in, commands going out. The transport sits behind
  an interface so tests run through memory.
- **Open.** The server's tick against the client's frame rate (interpolation); joining mid-game
  (a snapshot); trust (a LAN to begin with).

Networking is a plugin (`plugins/netview`), installed only by a game that wants it; the world and
the player layer know nothing about it.

## What each layer promises the next

| Layer | Owns | Promises upward |
|:---|:---|:---|
| `world.View` | the Space query, the set, when it is refreshed | "these entities are in these bounds, as of this tick" |
| players | cameras, screen rectangles, command queues, translators, bindings and their labels | "this player sees this View and speaks in these commands" |
| plugins, the game | the command types of their domain, their default bindings, the receivers | "these are the moves that exist, and what each does" |
| netview | frames, deltas, transport, the client | "this player is somewhere else; its commands and frames travel" |
