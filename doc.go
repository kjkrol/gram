// Package gram is a modular game engine for Go: a [game.Game] — named [game.Stage] values, each
// with its own entity-component world and its own [game.Scene]s — driven by an engine that runs
// the goke ECS in a window's loop, a tick and a picture a frame, the picture drawn on the GPU
// through WebGPU (gogpu). Everything beyond the tick loop is a [plugin.Plugin]: the built-in ones
// give a Stage a world of moving boxes, collisions, sight, a board with terrain, pathfinding and
// mouse selection; a game adds its own the same way. [Run] is the whole public surface of this
// package.
//
// # Game, Stage and Scene
//
// A Game supplies [game.Props] (window, tick rate) and its Stages by name. A Stage is one
// context the game can be in — a menu, the gameplay — with its own ECS, built fresh as
// [game.Runtime.SwitchStage] enters it: Init installs its plugins through a [game.Initializer],
// Spawn seeds a fresh game, Loaded stands in for Spawn when the Stage is entered from a save,
// Update is one tick. A game defines a Stage a section at a time with package game/stage —
// World, Plugins, Players, Effects, Rules, Commands, Kinds, Controls, Spawn, Scenes, Update — in
// that order, which the compiler keeps. Saving and loading are commands like any other
// ([game.Save], [game.Load], [game.Enter], [game.Quit]), bound to keys or ui buttons.
//
// A Scene is one thing a Stage can show: its renderers and its input handling. The Stage's
// [game.Scenes] is the registry of them; the live [game.Composition] over it says which are
// visible, in what order, and which is active, the only one whose HandleEvents runs.
// [game.Runtime] — pause, quit, switch Stage, persistence, full screen — reaches a Stage and
// every Scene alike. A scene's screen is a tree of elements of package ui, the world among them
// as its picture, everything the Stage's plugins draw (world.Plugin.Picture), through a camera
// ([ui.Image] of a render.Feed); the cameras are the cameras plugin's, made beside the feed that
// shows them. A player owns no camera: it acts through
// the picture a scene wires it to.
//
// # Plugins and rules
//
// A [plugin.Plugin] is used from a Stage's Plugins section. Its Install only queues ECS wiring,
// flushed in one ecs.Setup after Init returns, and says what the plugin draws
// (plugin.Installer.Draws, the Stage's one picture of it all ordered by height) and how its kinds
// look (plugin.Installer.Looks, declared once Init returns, before the scenes are made). A plugin
// needing another takes it as a constructor argument: there is no registry and no lookup by
// name.
//
// What a game does about what a plugin finds is a rule of the library ceremony
// (github.com/kjkrol/ceremony), which gram builds on with goke as the store:
// act.Branch[P]{Name, When, Then}, run inside the pass of the plugin that catches its moment P —
// a collision.Meeting for a pair of entities, a unit.Standing for an entity on the board — when
// its When holds, clauses of ceremony/cond each saying whom they are about (cond.Self.If,
// cond.Other.If, cond.Around(n).If, cond.On(plugin).If), its Then doing steps said of somebody
// the same way (act.Self.Do, act.Other.Put, act.Around(n).Put): putting on and taking off
// effects, giving commands. A rule is a role's (world.Roles), a role is played by a kind, a cell
// or a plugin, and once Init returns the engine hands the rules of every role played to the
// plugin that catches their moment (ceremony.Deliver, to the hosts of plugin.Installer.Hosts);
// one none catches is an error. What an entity does over time is a plan, plan.New, of loops of
// the same words, plan.Loop[P]{Name, When, Then, Until, Timeout, Cooldown}, carried by its kind.
// What somebody asks for is a command (ceremony/command): a player's key, a rule's act.Self.Do,
// and about an effect a sentence —
// ceremony.Put(open).On(ceremony.Group("trapdoors")).By(ceremony.Named("lever")). The language
// is ceremony's own documentation; gram's plugins give the moments, facts, words and knobs it is
// written over, each marked beside its type, and the world keeps a Stage's registers of effects,
// roles and commands by name (doc/behaviour.md).
//
// # Kinds and spawning
//
// What an entity is comes from a kind (package entity/kind): a Spec of the components every
// entity of the kind carries, the same for all (Const) or read from that entity's own row
// (Load). A kind's Entry puts one entity on the world's roster, spawned for a Stage entered
// afresh; a map file (board.Plugin.Lay) does the same for a board. Kinds also tell the saves
// which component types to expect.
//
// # Tick
//
// Props.TargetTPS is the engine's one fixed step. The window's loop runs one Update per frame,
// stepping as many times as the time gone says, at most five, so a slow frame slows the game
// instead of spiralling. Each step calls the active Scene's HandleEvents, then Stage.Update,
// where the game runs its plugins' RunPlan in the order it needs: the world, then motion, before
// whatever reads its space. What is drawn goes by the clock's Shown time, which runs on between
// the steps.
//
// # Persistence
//
// [game.Persistence] saves and loads the active Stage's ECS with every tracked value's state: a
// plugin's Serializable, or anything a Stage handed to ctx.Track. Resources are matched by name,
// never by position, so a save survives plugins being added, removed or reordered.
//
// # Package dependencies
//
// The packages form a strict acyclic graph; each imports only layers below it:
//
//	0  camera               a camera over a world: conversion, culling, move, zoom
//	   ceremony             how entities behave (github.com/kjkrol/ceremony): tag, comp,
//	                        effect, fact, cond, act, plan, host, command, store, its root
//	                        and goke, its binding over the ECS
//	1  render               drawing: Composer, Frame, Atlas, Feed     → camera, ceremony/effect
//	   control              input events and bindings                 → camera, ceremony/command
//	2  entity/kind          what an entity is: Spec, Define, Of       → render, ceremony/comp
//	   clock                the tactical clock: time, pause, tempo    → control, render, ceremony
//	3  entity               what every entity carries: Base, Z        → kind
//	   plugin               the extension contract                    → control, render, ceremony
//	   plugins/players/owner whose a unit is; a leaf                  → ceremony/command, tag
//	4  plugins/world        the foundation: space, entities, kinds    → plugin, clock, ceremony
//	5  game                 Game, Stage, Scene, Composition, Runtime  → plugin, world, ceremony
//	   ui                   a scene's screen of elements              → game, render, control
//	   plugins/motion       entities steered and moved, the leavers   → world
//	   plugins/vision       what the units see                        → world, …
//	6  plugins/cameras, collision                                     → world, motion, …
//	7  plugins/selection    the units a player picks                  → cameras, motion, …
//	   plugins/board        a grid with terrain over the world        → motion, collision, …
//	   plugins/dialog       conversations as data                     → world, ui, …
//	8  plugins/driving      units driven by hand                      → motion, selection, board
//	9  plugins/navigation   MoveOrder paths across a board            → board, selection, driving
//	   plugins/bullet       shots fired, flown, landing and bursting  → motion, collision, board
//	   plugins/gravity      bodies falling, standing and leaping      → motion, board, collision
//	   plugins/bridges      bridges built by one set of rules         → world, board, …
//	   plugins/topography   a map in relief drawn on the GPU          → motion, board, atmosphere
//	   plugins/atmosphere   calendar, climate, weather, sky           → world, board, …
//	   plugins/players      players over the command handlers         → motion, cameras, ui, …
//	10 internal/engine      the Engine: the loop, one Stage, saves    → game, world, ceremony
//	11 gram                 Run; the package you import               → game, internal/engine
//
// Outside the module: ceremony is the behaviour — rules, plans, effects, roles, commands, facts
// — the layer under plugin and control, its store goke/v3 through its binding ceremony/goke
// (the only package of it importing goke, imported in gram as cgoke); goke/v3 is the ECS every
// Stage runs on, aabbworld the space, collisions and line of sight under the world, gogpu the
// window, the loop and the GPU, astar the pathfinding, and uid the entity identifiers.
package gram
