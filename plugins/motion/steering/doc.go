// Package steering turns what an entity wants into motion, gradually.
//
// # Steering, Course and Helm
//
// A [Steering] is how an entity may be steered: how fast it turns and answers, and its motion
// profile. It is a knob an effect may Alter. Its [Course] is what it is asked and how far it has
// come; whoever steers holds both as a [Helm] and asks for a heading or a speed through it. The
// [System], run by motion before it moves, turns the heading by at most TurnRate a tick and
// writes the base speed from the profile.
//
// # Commands
//
// [Away], [Toward] and [Turn] are the commands an entity gives itself in a rule; the System
// carries them out, one a step.
//
// # Driven and Pace
//
// [Driven] is an entity steered by hand, written every tick by whoever steers it and carried out
// by the driving plugin; its Held is how long the hand has lain on it. [Pace] is how fast the
// ground lets an entity go, written by the board.
//
// # Vocabulary
//
// Knobs, for an effect's Alter: [Steering]. Commands, an entity's own: [Away] and [Toward] (aimed
// at the moment's subject, host.Aimed), [Turn].
package steering
