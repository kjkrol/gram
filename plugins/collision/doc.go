// Package collision detects overlaps between world entities each tick and records what each
// struck on its Collider. An entity takes part while it carries Collider; one also carrying
// Physics is pushed apart and bounces. Reactions are rules (rule.Then) of a Meeting, a pair, or of
// a Struck, obeyed by the roles its entities play (rule.Role).
//
// # Plugin and its system
//
// [Plugin] is built over a world plugin ([NewPlugin]) and installed with Use; it runs one
// collision system a tick. The system first settles every Collider's Base.Caps — CanCollide, plus
// Static for an immovable Physics, Sensor for none — and rebuilds the space when any changed, so a
// Collider counts from the tick it is carried. The tick is then one aabbworld collide.Engine Tick
// over the space's items, with the system as its Handler: the engine pairs up whoever may touch
// within a step, puts each overlapping pair to Touch, pushes the confirmed ones apart and reports
// each pushed box, which the system writes back to Base.Pos; whoever it pushed out through an open
// edge is marked world.Outside. A side that lost its Collider since the last rebuild vetoes the
// pair, is marked Plain, and the space is rebuilt after the tick.
//
// # Solid ground
//
// Given a [Field] (the board's Solid cells, board.Plugin.WithCollision or [Plugin.WithField]) the engine also
// pushes every movable collider out of the solid ground on its world.Layers and, with heights, in its [Band], through the side of
// a cell facing open ground. A contact with the ground bounces off it as off an infinite mass and
// is recorded as a [Contact] with Terrain set and the Cell; a sensor is told and never pushed.
// pair rules meet entities only.
//
// A push apart never puts a collider further over ground that does not take it — water to a
// walker, a hole ([Field].Overhang): the side it would put there holds where it is and bounces as
// off the ground, the other goes the whole way; a box the tick's later passes would leave further
// over such ground is not written back. Ground turning to water under an entity is no push: it
// stays there, fallen in.
//
// # Heights
//
// In a world with heights (world.Config.Heights) collision follows the entities' Z: a pair meets
// only where the heights the two span, their [Band]s ([BandOf] a Z: its bottom to its top), share
// a stretch, and the ground stops an entity only in a solid cell whose own band meets the entity's
// — the Field is asked for the entity's band beside its layers. A wall of a Height stands from
// below up to its top, so nothing passes under one on a slope and a shot over its top goes on; one
// of no Height stands at every height, as it does on the flat. Whatever says no height — an
// entity without a Z or with a Height of 0 — spans [Everywhere] and meets everything, as Layers 0
// meets every plane: the game does not use Z for it, so the test does not apply. Two bands meeting
// only at an edge do not meet (a crate on a platform). Layers and heights hold together, both must
// agree. In a flat world every band is Everywhere and nothing changes. What follows from it: two
// short units on stepped or sloped ground, their bands apart, pass each other — a game that wants
// them to meet declares their true height; in a topography world only entities carrying a
// unit.Mover get their Altitude written each step, any other collider keeps the Z it spawned with;
// and sight reads Z its own way (an entity without one is a point on the ground, a veiled cell a
// band from its level up), never through these bands. Overhang minds no height.
//
// # Swept entities
//
// An entity that moves itself further in a step than the world's cap allows — a shot — carries a
// [Sweep]: where its centre was as the step began, its box where it ended, written by whoever
// moves it, its Base.Vel zero. For the tick collision hands the space the whole stretch between
// the two, so the broad phase pairs it with everything on the path, refines each pair to the
// segment of its step against the other's box as it stands (a slanting path misses what lies in
// the stretch but off the segment; the other's own motion in the step is ignored), does the same
// with the solid ground box by box, and keeps the nearest contact alone — a pair or the ground —
// dropping the rest: one hit a step. Both sides' [Contact]s say where along the step it lies
// (Along, 0 to 1; 1 for a contact of no swept entity) and that it was only detected (Sensed). A
// swept entity is a sensor whatever its Physics: never pushed, never pushing, no bounce; two
// swept entities pass through each other; a sweep passes through the one it is told to Ignore
// (its shooter). After the tick the space holds the entity's own box again, so nothing but
// collision ever sees the stretch — at the cost of a rebuild of the space on either side of the
// tick while anything is swept. Its band in a world with heights is the one at the step's end. A
// wrapping world refuses a Sweep: a step must not cross a seam.
//
// # Collider and Physics
//
// [Collider] is all it takes to take part; it also holds what the entity struck the tick before
// ([Collider.Contacts], at most [MaxContacts] recorded — extras are still separated, bounced and
// reported to rules). Two colliders touch only where their world.Layers meet — a board game
// gives its units their Domain bits, so a flyer passes over a walker. [Physics] makes an entity take the physical side of a contact: pushed
// out of overlaps and bouncing, by Mass (non-positive weighs [DefaultMass], +Inf is a wall) and
// Restitution (the share of approach speed given back, 0 to 1; a pair uses the lower). An entity
// without Physics is only ever detected — a town, a rule. Separation is always an even split. The
// impulse and the footing are worked out by the plugin's own internal/response, not the game's to
// use.
//
// Collision brings a Collider and a Physics to every unit through the world's kind.Roster; a game
// drops the Physics of a unit nothing pushes with comp.Without.
//
// # Meeting and Struck
//
// A rule of a [Meeting] is handed one per confirmed contact between its two tags,
// seen from Self: who it met, the impulse exchanged (zero when only detected) and the way Self
// left Other. A rule of a [Struck] is handed one per entity that struck something the tick
// before: which it is and what it struck; an entity that struck nothing is not told. Ready-made
// rules are in plugins/collision/rules; this package never imports it. [Plugin.WithStats] counts the contacts into a [ContactStats], [Plugin.WithLog]
// writes a line for each — the plugin's own work in its pass.
package collision
