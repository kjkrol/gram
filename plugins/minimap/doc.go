// Package minimap brings minimaps: small pictures of the world round a player's camera, of the
// ground in miniature and the marks of the kinds that have one, the camera's view outlined on it;
// a click on one takes the camera there.
//
// # Plugin
//
// [NewPlugin] makes the minimaps over a world and its [Ground], as [Config] says: how far round
// the camera they reach, never past the world's last band, and whether they turn with the camera
// or keep north up. A game makes it after the cameras, hands it to the players with the other
// handlers and runs it after the cameras ([Plugin.RunPlan]).
//
// # A minimap in a scene
//
// [Plugin.For] is the minimap of a camera as a picture for a scene: the scene places it and
// dresses it, ui.TopRight(a.minimap.For(cam, a.player.ID)).Size(220, 220), round with
// .Masked(ui.Circle). A minimap draws a picture of its own, not the Stage's: the ground's
// miniature, the marks and the outline, nothing more, cheaply; north is a red triangle hung on
// the circle's edge ([Plugin.Compass], an element the scene lays over the minimap in a box
// [Overhang] bigger on every side), pointing north, at the top north up, turned round the circle
// and with it as the minimap turns. It is neither
// an anchor of the world's bands nor saved.
//
// # Ground and marks
//
// [Ground] is what lies under a minimap, the world's ground as one picture: the board's and the
// topography's give theirs. A kind shows on a minimap as its figure's Look declares with
// Styled([Mark]{...}); a kind without a Mark does not.
//
// # Vocabulary
//
// Commands: [Switch] shows the minimaps or hides them (Shift+M), [Turn] turns them with their
// cameras or keeps north up (Ctrl+M). Settings ([Settings], saved with the game, in
// [Resources]): both. A click on a minimap gives cameras.CenterAt. The red triangle on a minimap's
// edge points north.
package minimap
