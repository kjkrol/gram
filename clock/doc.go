// Package clock is the tactical clock of a world: the one game time everything that simulates goes
// by, kept by the world and saved with the game.
//
// # Time and the tempo
//
// Game time is the sum of the simulation's steps ([Clock.Time]): it stands still in the tactical
// pause and goes at the clock's tempo, ½, 1, 2 or 4 by default ([Config.Tempos]). In a tick the
// clock replays the simulation as many times as the tempo says ([Clock.Replay]), each time over
// the same step, so the game runs the same at any tempo; a [Config.BiggerStep] clock replays once
// over a longer step instead. What is drawn goes by [Clock.Shown], game time run on past the last
// tick by the real time the engine holds toward the next, so animations move every frame.
//
// # Interface and simulation
//
// In its RunPlan a plugin runs at once whatever is interface (commands, selecting, planning) and
// hands the clock whatever simulates ([Clock.Simulate]): movement, collisions, effects, sight,
// the weather. The clock replays those pieces after the game's Update, in the order they came,
// so selecting and orders work in the tactical pause and never speed up.
//
// # Commands, phases and the display
//
// [Clock.TogglePause] is the tactical pause, [Clock.Faster] and [Clock.Slower] move the tempo,
// which the world's commands ask. The clock's entity carries a family of [Phase] tags that
// effects cast on it switch on and off; a rule asks [Clock.In] whether a phase holds. A rule of
// the clock's [Moment] fires once every step of the simulation; its words [At] and [Every]
// (cond.Self.If(clock.Every(period, offset))) hold in the step reaching a time. [Clock.Reporter] is
// a telemetry line, [Clock.HUD] a screen layer showing the game time.
package clock
