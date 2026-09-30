// Package collision detects overlaps between world entities each tick and records what each
// struck on its Collider. An entity takes part while it carries Collider; one also carrying
// Physics is pushed apart and bounces. Reactions are triggers (act.Trigger) of a Meeting, a
// pair, or of a Struck, hooked with Plugin.Hook.
//
// # Plugin and CollisionSystem
//
// [Plugin] is built over a world plugin ([NewPlugin]) and installed with Use; it runs one
// [CollisionSystem] system a tick. The CollisionSystem first settles every Collider's Base.Caps — CanCollide,
// plus Static for an immovable Physics, Sensor for none — and rebuilds the space when any changed,
// so a Collider counts from the tick it is carried. The tick is then one aabbworld collide.Engine
// Tick over the space's items, with the CollisionSystem as its Handler: the engine pairs up whoever may
// touch within a step, puts each overlapping pair to Touch, pushes the confirmed ones apart and
// reports each pushed box, which the CollisionSystem writes back to Base.Pos; whoever it pushed out
// through an open edge is marked world.Outside. A side that lost its Collider since the
// last rebuild vetoes the pair, is marked Plain, and the space is rebuilt after the tick.
//
// # Solid ground
//
// Given a [Field] (the board's Solid cells, board.Plugin.WithCollision or [Plugin.WithField]) the engine also
// pushes every movable collider out of the solid ground on its world.Layers, through the side of
// a cell facing open ground. A contact with the ground bounces off it as off an infinite mass and
// is recorded as a [Contact] with Terrain set and the Cell; a sensor is told and never pushed.
// pair triggers meet entities only.
//
// # Collider and Physics
//
// [Collider] is all it takes to take part; it also holds what the entity struck the tick before
// ([Collider.Contacts], at most [MaxContacts] recorded — extras are still separated, bounced and
// reported to triggers). Two colliders touch only where their world.Layers meet — a board game
// gives its units their Domain bits, so a flyer passes over a walker. [Physics] makes an entity take the physical side of a contact: pushed
// out of overlaps and bouncing, by Mass (non-positive weighs [DefaultMass], +Inf is a wall) and
// Restitution (the share of approach speed given back, 0 to 1; a pair uses the lower). An entity
// without Physics is only ever detected — a town, a trigger. Separation is always an even split.
//
// Collision brings a Collider and a Physics to every unit through the world's kind.Roster; a game
// drops the Physics of a unit nothing pushes with comp.Without.
//
// # ShapeTest
//
// A [ShapeTest] ([Plugin.WithShapeTest]) is asked once per overlapping pair, with both sides as
// [Contactee]s, whether the shapes inside the boxes really touch: it may refuse the contact (an
// alpha mask saying the pixels miss) or refine the penetration (a distance field). [BoxesTouch]
// is the default, at no cost.
//
// # Meeting and Struck
//
// A trigger of a [Meeting] is handed one per confirmed contact between its two tags,
// seen from Self: who it met, the impulse exchanged (zero when only detected) and the way Self
// left Other. A trigger of a [Struck] is handed one per entity per tick: which it is and what
// it struck the tick before. Ready-made ones are in plugins/collision/trigger; this package
// never imports it.
package collision
