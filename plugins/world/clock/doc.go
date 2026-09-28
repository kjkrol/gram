// Package clock is the tactical clock of a world: the one game time everything that simulates goes
// by, kept by the world and saved with the game.
//
// # Time and the tempo
//
// Game time is the sum of the simulation's steps ([Clock.Time]): it stands still in the tactical
// pause and goes at the clock's tempo — ½, 1, 2 or 4 by default ([Config.Tempos]). The engine
// ticks at a fixed step; in a tick the clock replays the simulation as many times as the tempo
// says ([Clock.Replay]), each time over the same step, so the game runs the same at any tempo —
// twice as many steps a second at 2, a step every other tick at ½, none in the pause. A
// [Config.BiggerStep] clock replays once over a longer step instead: cheaper, less exact. When
// the engine cannot keep up with a tempo for a while, the clock brings it down a notch and says so.
//
// # Interface and simulation
//
// A game calls its plugins' RunPlan once a tick, as ever. In it a plugin runs at once whatever is
// interface — commands, selecting, planning — and hands the clock whatever simulates
// ([Clock.Simulate]): movement, collisions, effects, sight, the weather. The clock replays those
// pieces after the game's Update, in the order they came, so the order the game laid out holds in
// every step. Selecting, the camera and orders therefore work in the tactical pause and never speed
// up; a behavior runs where the system hosting it runs, which for every host there is means the
// simulation.
//
// # Commands, phases and the display
//
// Space toggles the tactical pause, ] and [ move the tempo ([Clock.DefaultBindings]); the players
// plugin carries them through the world. The clock's entity carries a family of [Phase] tags the
// schedule (world/effects) switches on and off — a behavior asks [Clock.In] whether a phase holds.
// [Clock.Reporter] is a telemetry line, [Clock.HUD] a screen layer showing the game time.
package clock
