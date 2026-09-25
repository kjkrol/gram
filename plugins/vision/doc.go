// Package vision gives entities a narrowed view of the world: a Sight cone sees what falls
// inside it, within range and not hidden behind something nearer. Each tick fills Sight.Seen
// and runs Between behaviors of a Sighting; SightOutline gets the view drawn.
//
// # Sight and Seen
//
// [Sight] is what an entity can take in — Facing, its own direction whichever way it moves;
// HalfAngle either side of it; Radius — and what the last scan found there: [Sighted], at most
// [MaxSeen] entities nearest first. The outline buffer is sized for [MaxSightRadius] and
// MaxHalfAngleMilli; a larger Sight still sees, its outline is only coarser. The [ScanSystem]
// scans every Sight against the world's space through aabbworld's line-of-sight scan, once a tick.
//
// # Transparency
//
// Every entity cuts sight unless it carries a [Transparency]: 1 as if absent, 0 cutting, in
// between dimming — a ray spends its Radius as a budget and a stretch through an entity at τ costs
// 1/τ per unit, so a forest at 0.4 is looked through at 0.4 of the reach. Terrain is no entity: the
// cone also walks the world's Cover (the board's veiled cells, τ = 1 - Veil, for the observer's
// Blockers), which ends or dims the reach the same way and is never seen. Whatever the ray
// reaches within its budget is seen, a
// forest looked into as much as a wall looked at. Sight.Blockers are the world.Layers that cut
// or dim a Sight at all: an entity on none of them is looked over as if it were not there, and
// still seen — a hawk with Blockers of Air looks over walls, forests and walkers, a walker with
// Blockers of Land under the hawk. Zero Blockers make every entity count.
//
// # Heights
//
// In a Quasi3D world (world.Config.Quasi3D) sight follows geometry instead of planes: the cone's
// eye is the observer's Z.Altitude plus Sight.Eye, every entity spans its world.Z, and the ground
// is the world's Ground sampled every [Plugin.WithGroundStep] along a ray (default: the board's
// cell). An entity is seen when the line from the eye to its top clears every nearer ground
// sample and every nearer blocking band within the budget, so a hawk 40 up looks over the wall, the
// forest and the hill a walker's cone stops at. Blockers are refused in a Quasi3D world, Eye in a
// flat one. The scan costs about three times the flat one; a longer ground step is cheaper.
//
// # Sighting
//
// A [Between] behavior registered here is run once a tick per observer carrying tag a,
// with a [Sighting]: the observer, its Base, Sight and Steering (nil for one that cannot be
// steered), and everything in view carrying b as [Seen] values nearest first — a directed pair,
// grouped by observer, run even when nothing is in view. A behavior tells its seen entities
// apart with Seen.Carries, and steers only through Steering.Request. Ready-made
// ones, and their tags, are in plugins/vision/behavior.
//
// # SightOutline and Renderer
//
// The Renderer drapes each fan over the world's Ground when it has one ([Renderer.WithGround]):
// the apex at the observer's altitude, the boundary on the ground under it, so through an
// isometric camera a cone climbs a hill.
//
// An entity also carrying [SightOutline] has its view's shape computed: a reach per evenly spaced
// angle across the cone. The [Renderer] draws it through the camera in a [ConeStyle]
// ([DefaultConeStyle] strokes the boundary; [Plugin.WithStyle] or [ConeStyleFn] for another).
//
// In a Quasi3D world the reach of sight is not the reach of the ground: past a cliff the plain is
// out of sight, a hawk above it or a higher hill beyond is not. There the outline reaches the full
// Radius at every angle and keeps, per angle, up to [MaxShadowsPerSample] [Band]s of ground out of
// sight (aabbworld's View.Shadows); the Renderer fills them over the ground as holes in the view,
// through a style's Shade when it is a [ConeShader], else [DefaultShadow]. A flat world keeps the
// reach cut where a wall stands, and no shadows.
package vision
