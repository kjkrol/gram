// Package collision detects overlaps between world entities each tick and records what each
// struck on its Collider. An entity takes part while it carries Collider; one also carrying
// Physics is pushed apart and bounces. Reactions are rules (act.Branch) of a Meeting, a pair,
// or of a Struck, obeyed by the roles its entities play (world.Roles).
//
// # Plugin and its system
//
// [Plugin] is built over a world plugin and motion ([NewPlugin]) and installed with Use; it runs
// one collision system a tick over the world's space. A Collider counts from the tick it is
// carried; one that loses its Collider meets nobody from the next. Whoever is pushed out through
// an open edge is motion's leaver (motion.Plugin.Leave). Only the first band of the world's
// sectors collides (sector.Near): farther off a box meets nothing and keeps no contacts.
//
// # Solid ground
//
// Given a [Field] (the board's Solid cells, board.Plugin.WithCollision or [Plugin.WithField]) the
// system also pushes every movable collider out of the solid ground on its world.Layers and, with
// heights, in its [Band]. A contact with the ground bounces off it as off an infinite mass and is
// recorded as a [Contact] with Terrain set and the Cell; a sensor is told and never pushed. A
// pair parted that still touches stays met, a [Contact] at rest (Resting) on both sides with no
// impulse and no strike, until the two come apart. A
// unit a hand drives (steering.Driven, the keys held or not) is never turned by a wall, only
// slowed.
//
// A push apart never puts a collider further over ground that does not take it (water to a
// walker, a hole; Field.Overhang): that side holds where it is and bounces as off the ground, the
// other goes the whole way. Ground turning to water under an entity is no push: it stays, fallen
// in.
//
// # Heights
//
// In a world with heights (world.Config.Heights) a pair meets only where the [Band]s the two span
// ([BandOf] a Z) share a stretch, and the ground stops an entity only in a solid cell whose own
// band meets the entity's. Whatever says no height spans [Everywhere] and meets everything, as
// Layers 0 meets every plane; two bands meeting only at an edge do not meet. A band carries the
// entity's step: a deck's slab strikes only what of the band lies over it ([Band.Above]), so a
// walker steps onto a deck within its reach and passes under one it fits under. A field may hold
// walls besides its boxes ([Field.Walls]): railings along a bridge, a band's slab. In a flat world
// every band is Everywhere.
//
// # Swept entities
//
// An entity that moves itself further in a step than the world's cap allows (a shot) carries a
// [Sweep]: where its centre was as the step began, its box where it ended. For the tick the space
// holds the whole stretch, every pair and solid box on it is refined to the segment of the step,
// and the nearest contact alone is kept: one hit a step, its Contacts saying where along the step
// it lies (Along) and that it was only detected (Sensed). A swept entity is a sensor whatever its
// Physics, two swept pass through each other, and a sweep passes through the one it Ignores. A
// wrapping world refuses a Sweep.
//
// # Collider and Physics
//
// [Collider] is all it takes to take part; it also holds what the entity struck the tick before
// ([Collider.Contacts], at most [MaxContacts]). Two colliders touch only where their
// world.Layers meet. [Physics] makes an entity pushed out of overlaps and bouncing, by Mass
// (non-positive weighs [DefaultMass], +Inf is a wall) and Restitution; one without Physics is
// only ever detected. Separation is always an even split. Collision brings a Collider and a
// Physics to every unit through the world's roster; a game drops the Physics of a unit nothing
// pushes with comp.Without.
//
// # Meeting and Struck
//
// A rule of a [Meeting] (When: cond.Other.If(role)) is handed one per confirmed contact between two
// entities, seen from Self; a rule of a [Struck] one per entity that struck something the tick
// before. [Plugin.WithStats] counts the contacts into a [ContactStats], [Plugin.WithLog] writes a
// line for each.
//
// # Vocabulary
//
// Moments, for a role's rules: [Meeting] (a pair, host.Met; its word [Facing]), [Struck] (one that
// struck something the tick before). Knobs, for an effect's Alter: [Physics]. Commands: [Boxes]
// (a player's, a look: every entity's box in red, its base under it, its solid over the figures
// in a world with heights, or none; [BoxesShown]; Shift+X in turn). Settings ([Settings], saved
// with the game, in [Resources]): the boxes shown.
package collision
