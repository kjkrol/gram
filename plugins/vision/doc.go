// Package vision gives entities a narrowed view of the world: a Sight cone sees what falls
// inside it, within range and not hidden behind something nearer. Each step fills the observer's
// Sighted and runs the rules of a Sighting.
//
// # Sight and Sighted
//
// [Sight] is where an entity looks and how far, a knob vision only reads and an effect may Alter;
// how wide it sees and from how high is its world.Eye, without which it is never scanned. What
// the last scan found is its [Sighted]: at most [MaxSeen] entities, nearest first. Every observer
// is scanned every step, or [Plugin.WithFrequency] times a second of game time, the scan shared
// among goroutines ([Plugin.WithWorkers]). Only the first band of the world's sectors looks
// (sector.Near): farther off an observer sees nothing.
//
// # Transparency
//
// Every entity cuts sight unless it carries a [Transparency]: 1 as if absent, 0 cutting, between
// them dimming. The world's cover (the board's veiled cells) dims or cuts a ray the same way and
// is never seen. Sight.Blockers are the world.Layers that cut or dim a sight at all; an entity
// on none of them is looked over and still seen.
//
// # Heights
//
// In a world with heights ([Plugin.WithBoard]; [Plugin.WithHeights] and [Plugin.WithCover] for a
// ground and a cover of one's own) sight follows geometry: the eye is at the observer's
// world.Eye, every entity spans its world.Z, and the ground is read along every ray, exactly off
// a ground.Profile or sampled every [Plugin.WithGroundStep]. With a world.Scale the ground sinks
// under the observer's level as far off as it lies. An Eye with a Vertical field sees up and
// down within it alone, turned up or down by its Tilt (a rider's head): a sentinel on a wall
// looking down sees the wall's foot, not the distance; the view drawn is the ground the field
// meets. Eye.Height and Eye.Vertical are refused in a flat world.
//
// # Sighting
//
// A rule of a [Sighting] (act.Branch[vision.Sighting]{When: cond.Other.If(role), …}) runs once a
// step per observer playing the rule's role, on everything in view playing the role of Other as
// [Seen] values nearest first, even when nothing is. Its Subject is the nearest seen, so an Aimed
// command is about it and fails with none in view. Its words: [SeesNobody] of the observer,
// [Closing] of the other — the nearest seen on a collision course with the observer.
//
// # SightOutline and Renderer
//
// The [Renderer] draws the views. Through a camera with Rays and in the default style it draws
// every observer's view on the GPU on [ViewTier], the ground out of sight veiled in a [Shadow]
// ([Plugin.WithShadow]). Otherwise an observer carrying [SightOutline] has its view's shape
// computed ([Band]s of hidden ground per angle in a world with heights) and drawn as a ring of
// [ConePoint]s in a [ConeStyle] ([Plugin.WithStyle]). Which views are drawn is the scene's,
// through render.Show rules ([Plugin.WithViews]); none, every one.
//
// The views start hidden: Shift+C ([Cones]) shows and hides them, [Plugin.ShowCones],
// [Plugin.HideCones] and [Plugin.ConesShown] do the same from code. A look, not saved; the scan
// goes on either way.
//
// # Vocabulary
//
// Moments, for a role's rules: [Sighting] (a pair grouped by observer, host.Met, its subject the
// nearest seen; its words [SeesNobody], [Closing] of the other). Knobs, for an effect's Alter:
// [Sight]. Commands: [Cones] (a player's, a look). Styles, for a figure's Look on the world's
// atlas (Slot.Styled): [Cone], the colour a kind's view is edged in, [DefaultCone] without;
// [Plugin.WithStyle] replaces every kind's with a composer of its own, [Plugin.WithShadow] the
// veil over the ground out of sight.
package vision
