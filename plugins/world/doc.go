// Package world is the foundation for a Stage with moving, drawable entities: a Position and
// Velocity each, a shared spatial index other plugins query, and motion integrated each tick,
// never further than Position.MaxStep. What an entity carries is set by its kind — see kind.
//
// # Plugin
//
// [Plugin] is installed by a Stage's Init through ctx.UseWorld, once, from a [Config]: the
// [SpaceCfg] sizes the world and sets the edge rule per axis (aabbworld.Torus, WrapX or WrapY
// alone, OpenX or OpenY, a closed axis by default — a box stops whole at a closed edge, wraps at
// a wrapping one, may leave by an open one); the [EntitiesCfg] bounds how many entities the world
// holds and the sizes they spawn with; the camera.Config sizes the camera. An entity wholly past
// an open edge carries [Outside] — put on by whoever moved it there, the MoveSystem or collision's
// solver — and every tick it does, the [Each] behaviors of a [Leaving] registered with
// [Plugin.RegisterBehavior] hear of it; with none registered it is despawned. Put back inside, it
// loses the mark. [Plugin.Roster] is what the plugins in the game ask of a unit's kind — see
// package kind; world requires a Position and brings a Velocity. [Layers] are the planes an
// entity is on, one bit each, read by collision and
// sight: two entities meet only where they share a bit, and one carrying none is on every plane.
// Config.Quasi3D gives the world heights: entities carry a [Z] (bottom and rise), the board sets
// the world's [Ground] ([Plugin.SetGround]) and sight follows it; a flat world refuses a Z.
// The Plugin exposes the shared [aabbworld.Space] ([Plugin.Space]) and the shared camera
// ([Plugin.Camera]; the players plugin moves it through Pan and Zoom commands).
//
// # Base, Position and Velocity
//
// [Base] is the one component every entity carries: its [Position] (a plane.AABB), its
// [Velocity] (a unit heading and a speed in world units a second), the [kind.ID] it was spawned
// from, and the aabbworld.Capability bits the space indexes it under, which collision writes.
// A host hands Base to whatever it hosts instead of anyone binding it twice. No entity moves
// further in a tick than [StepReach] of its own shorter side ([Position.MaxStep],
// [Position.MaxSpeed]), so mixed sizes share a world without the smallest slowing the rest.
//
// # Kinds, Seed and Populate
//
// [Plugin.Kinds] is the registry kind.Define registers with; [Kinds] also issues atlas slots no
// kind owns ([Kinds.NewSprite]) and tells saves every component type its kinds carry. A Stage's
// Spawn puts entities on the roster with [Plugin.Seed]; the engine calls [Plugin.Populate] only
// when nothing was restored. [GridPlacement] arranges a population on a regular grid.
//
// # Tags
//
// A tag is a bit of a family: [plugin.Tags] is the family's component, an empty type of the
// plugin's or the game's names the family, and [Kinds.DefineTag] hands out the bits by name — saved by name, so a build defining them in another order still loads. A
// kind gives its entities tags with [comp.Tagged]; a query over the family's Tags narrows to
// the entities carrying any of them, and setting or clearing a bit is a value write, seen the
// same tick. Behaviors name tags in a plugin's Between; the marker components of old are gone.
//
// # Bodies
//
// [Bodies] spawns entities of a kind reserved with [Kinds.Reserve]: a Base and the caller's own
// columns, no Appearance and no size bounds, counted against MaxCount. It is how a plugin
// materializes geometry of its own from inside a system, at any tick. Terrain on a grid needs none:
// it is the world's Field and Cover (see [Plugin.SetField], [Plugin.SetCover]), which collision
// and sight read cell by cell.
//
// # Attach, Detach and Declare
//
// [Plugin.Attach] and [Plugin.Detach] are the mid-game counterparts of a kind's Const, for game
// logic that has a plugin.Tick and no component id; [Plugin.Despawn] removes an entity at the end
// of the tick. [Plugin.Declare] tells saves about a type only ever attached; call it in Init.
//
// # Systems
//
// [Plugin.RunPlan] runs the tick: every registered [Behavior] (a decision system, see
// [Plugin.RegisterBehavior]), then [SteeringSystem] carries out [Steering] requests (heading, and base
// speed for an entity with a motion profile), [VelocitySystem] runs the [Each] and [Every]
// behaviors of a [Moving] over every entity so they may scale that speed, then [MoveSystem] moves every box under the
// edge rules and hands the space every Base as an aabbworld.Item — Space.Rebuild. The space keeps
// no state of its own between ticks: Populate and PostLoad rebuild it too, so it is whole before
// the first tick, and a despawned entity is gone from it on the next. Anything reading the space
// in its own pass sees the boxes as they stand after the last rebuild — and, after a collision
// tick, as the engine pushed them.
//
// # Appearance and Renderer
//
// [Appearance] is the sprite an entity is drawn from; [Plugin.WithRenderer] builds the entity
// [Renderer] over an atlas, and the Each behaviors of a [Drawing] registered on the plugin settle
// each entity's layers in order — [Draw].Overlay, Draw.As, Draw.With and Draw.Facing are the
// ready-made ones. The Renderer draws the
// entities in the camera's [View] and nothing else.
//
// # View and EntitySet
//
// A [View] is what one pair of eyes sees: a rectangle of the world and the entities the Space finds
// in it, as an [EntitySet] — a set of the world's entities by index. The world keeps any number of
// Views ([Plugin.NewView] over a source of bounds, [Plugin.DropView]) and the [ViewSystem]
// refreshes each of them once a tick, right after movement has rebuilt the Space; a View whose
// bounds cover the whole world is not queried and simply sees everything, as does the zero View a
// Stage has before its first tick. [Plugin.View] is the camera's, made by the plugin itself; the
// entity renderer reads it. A View over another camera or a remote player's bounds is the same
// thing — see doc/views.md for where that leads.
//
// # Telemetry
//
// [Telemetry] counts the population; Resources publishes it for a renderer to show.
package world
