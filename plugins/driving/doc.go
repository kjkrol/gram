// Package driving drives units by hand: a player's keys, or an entity's own commands, walk,
// turn, strafe, brake and back a unit away, leap, and fly one that flies.
//
// # Commands and the hand
//
// [Ahead] walks the units on the way each faces (Sprint urging them on), [Back] brakes them and
// backs them away, [Turn] turns them, [Strafe] walks them across the way they face, facing on,
// [Toward] walks them the way of the screen, [Jump] has them leap ([JumpKey] binds it). Each is given every tick its key is held, and a tick's commands add
// up into a hand. A player's hand, through the camera its commands name, is on the unit that
// camera is fastened to, else on the units the player has selected and owns; an entity's command
// for itself (act.Self.Do) is its own hand. The hand is written into the unit's steering.Driven
// and the unit marked [Driving]; asked to go more than 45 degrees off its way, a walking unit
// brakes first and turns, then walks; a tick with no hand on it brakes it, and it is let go
// once it stands.
//
// # Keys
//
// [Tank] are four keys driving the unit as one sits in it (Left and Right turn), [Shooter] four
// driving it as a shooter does (Left and Right strafe, the mouse turns) and [Compass] four walking
// it the way of the screen; each holds in the fastenings of the camera it says (In), so the same
// keys may pan a free camera and drive a ridden unit. [DefaultKeys], the plugin's
// DefaultBindings, are a Shooter of W, S, A and D inside the unit, a Tank of the arrows behind
// it, and Space to jump inside it; a split screen gives each player its own Compass.
//
// # Ground and navigation
//
// Given a board ([Plugin.WithGround]) a driven unit never walks onto ground its domain may not
// stand on, and its cell and its hold on the occupancy follow it; one under gravity is held back
// by physics alone. A plugin that keeps units apart hands the driving its [Keeping]
// ([Plugin.WithKeeping], as navigation does): whether a unit may walk on into the cell ahead, and
// whether something else steers it, which the driving leaves be with no hand on it.
package driving
