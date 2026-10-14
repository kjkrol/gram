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
// scenes are made. [Plugin.Show], [Plugin.Hide], [Plugin.Toggle] and [Plugin.Shown] work on the
// Stage's Composition, as the players' key does.
//
// # Vocabulary
//
// Commands: [Panel] (a player's, carried by the players: it needs the scene's Composition).
package telemetry
