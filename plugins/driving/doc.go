// Package driving drives units by hand: a player's keys, or an entity's own commands, walk,
// turn, brake and back a unit away, and fly one that flies.
//
// # Commands and the hand
//
// [Ahead] walks the units on the way each faces (Sprint urging them to their Steering's Sprint),
// [Back] brakes them to a stop and backs them away, [Turn] turns them, [Toward] walks them the way
// of the screen. Each is given every tick its key is held (control.KeyHeld), and the commands of
// a tick add up into a hand — each way held within one, the ways of Toward summed. A player's
// command names the camera it came through (control.Context.Camera): the hand is on the unit that
// camera is fastened to (camera.Fastening), else on the units the player has selected
// (selection.Selected), each obeying the player (players/owner.Obeys). An entity's command for
// itself (rule.Order) is its own hand. The hand is written into the unit's steering.Driven — Ahead,
// Turn, Sprint and Face, the way of Toward — and the unit marked [Driving]; a tick with no hand
// on a Driving unit zeroes them, braking it. The fields an eye riding in the unit writes — Look,
// Flown, Climb (plugins/topography) — are left alone.
//
// # Keys
//
// [Tank] are four keys driving the unit as one sits in it — on (with Shift, sprinting), back,
// left, right — and [Compass] four keys walking it the way of the screen; each holds in the
// fastenings of the camera it says (In: camera.Inside riding in the unit, camera.Behind following
// it, camera.Outside from anywhere), so the same keys may pan a free camera and drive a ridden
// unit. [DefaultKeys] — the plugin's DefaultBindings — are W, S, A and D inside the unit and the
// arrows behind it. A game binds others for its players: a split screen gives each its own
// Compass.
//
// # Carrying it out
//
// Every step of the simulation a Driven unit turns — by Turn, or towards Look, before any Face —
// and walks the way it faces, or brakes and backs away; one that flies, flown by hand, goes along
// the ground only the run of the way it is steered along (steering.Driven.Slope). Given a board
// ([Plugin.WithGround]) it never walks onto ground its domain may not stand on, and its cell
// (unit.At) and its hold on the board's occupancy follow it cell by cell, with unit.Entered. A
// Driving unit is let go once no hand is on it and it stands.
//
// # Navigation
//
// The driving needs no navigation. A plugin that keeps units apart and steers them along routes
// hands the driving its [Keeping] as it is made (navigation.NewPlugin does): whether a driven unit
// may walk on into the cell ahead, as its spacing says, and whether a unit is steered along an
// order of its own, which the driving leaves be with no hand on it. Navigation, for its part, has
// a unit a hand takes give its order up; its RunPlan comes before the driving's.
package driving
