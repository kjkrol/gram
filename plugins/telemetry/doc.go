// Package telemetry is the panel in the top-left corner saying how the game runs: the frame
// rate, the ticks a second, where a frame's time goes, the entities, the game time, and
// whatever the plugins and the game add. It is a scene of its own in every Stage with a world,
// hidden until ~ (the players' key) or the Stage's Shows shows it.
//
// # Plugin
//
// [NewPlugin] makes the panel over a world with the standard lines; a Stage built with
// game/stage makes one itself where the game made none ([Of] finds it). [Plugin.With] adds the
// lines of render.Reporters (the atmosphere's, collision's stats), [Plugin.Line] a line of the
// game's own, [Plugin.Clear] drops every line, the standard ones too; all before the Stage's
// scenes are made. A value of several lines stands under its label, a part a line. The lines are
// read after the ticks, four times a second and as the mode changes ([Plugin.Publish]); the
// panel draws the last reading. [Write.Show], [Write.Hide], [Write.Toggle] and [Read.Shown] work
// on the Stage's Composition, as the players' key does.
//
// # Mode
//
// The panel shows the basics ([Basic]: the frame rate, the ticks a second, where a frame's time
// goes and the entities) or every line ([Extended]: the game time, the reporters' and the game's
// own too); [Write.SetMode] and [Read.Mode] set and read it, [Settings] keeps it with the
// game.
//
// # Vocabulary
//
// Commands: [Panel] (a player's, carried by the players: it needs the scene's Composition).
// Settings ([Settings], in [Resources]): the panel showing every line.
package telemetry
