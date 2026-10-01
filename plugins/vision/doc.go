// Package vision gives entities a narrowed view of the world: a Sight cone sees what falls
// inside it, within range and not hidden behind something nearer. Each tick fills the entity's
// Sighted and runs pair rules of a Sighting; SightOutline gets the view drawn.
//
// # Sight and Sighted
//
// [Sight] is what an entity can take in — Facing, its own direction whichever way it moves unless
// Ahead has it look the way it goes, and Radius — a knob vision only reads, which an effect may
// Alter. What the last scan found is the entity's [Sighted], beside it: at most [MaxSeen] entities
// nearest first, given by vision to an observer without one. How wide it sees, and from how high, is its world.Eye: Angle the whole field across,
// Height over its bottom (zero: its top) — the one Eye a camera riding in the entity looks from
// too (plugins/topography), so the cone drawn is what the rider sees. A Sight without an Eye is
// never scanned. The outline buffer is sized for [MaxSightRadius] and MaxHalfAngleMilli; a larger
// Sight still sees, its outline is only coarser. The [ScanSystem] scans every Sight against the
// world's space through aabbworld's line-of-sight scan, once a tick — observers enough at a time
// on several goroutines at once, as many as there are CPUs unless told otherwise
// ([Plugin.WithWorkers]), each with a scanner of its own; the rules then run one observer at
// a time, in order.
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
// In a world with heights (world.Config.Heights) sight follows geometry instead of planes: the cone's
// eye is where the observer's Eye stands (Eye.Level: Height over its Z.Altitude, its top for
// none), every entity spans its world.Z, and the ground
// is the board's heights ([Plugin.WithBoard]; [Plugin.WithHeights] and [Plugin.WithCover] for a
// ground and a cover of one's own) sampled every [Plugin.WithGroundStep] along a ray (default:
// the board's cell). An entity is seen when the line from the eye to its top clears every nearer ground
// sample and every nearer blocking band within the budget, so a hawk 40 up looks over the wall, the
// forest and the hill a walker's cone stops at. On a world with a scale (world.Scale) the ground,
// the cover and the entities sink under the observer's level as far off as they lie
// (world.Scale.Drop): what lies past the observer's horizon is out of sight, a hawk's horizon far
// beyond a walker's. Blockers are refused in a world with heights, Eye.Height in a flat one. The
// scan costs about three times the flat one; a longer ground step is cheaper, and the Renderer
// drapes the shadows in the same step.
//
// # Sighting
//
// A rule of a [Sighting] hooked here (rule.On(name, rule.Between(a, b), …)) is
// run once a tick per observer carrying tag a, with a [Sighting]: the observer, its Base, Sight and
// Helm (the zero one for one that cannot be steered), and everything in view carrying b as [Seen]
// values nearest first — a directed pair, grouped by observer, run even when nothing is in view. A
// ready-made hook tells its seen entities apart with Seen.Carries, and steers only through
// Helm.Request. Ready-made
// ones, and their tags, are in plugins/vision/hooks.
//
// # SightOutline and Renderer
//
// The [Renderer] draws the views. Through a camera with Rays and in the default style it draws on
// the GPU the view of every observer — Sight and world.Eye, no SightOutline needed — on
// [ViewTier], after the ground and before what stands on it: the ground and its cover are copied
// into images every ground step as they change, round the world where it wraps, each observer's
// sight is baked every frame over them (shaders/sighted.wgsl) — hidden where the ground rises over
// the line from the eye, dimmed through cover as the scan dims it, sunk as far off as it lies — and
// laid over the ground the frame drew, read from its depth, or over level ground found along the
// camera's lines of sight in a world without heights, again past a wrapping world's seam
// (shaders/views.wgsl): the ground out of sight veiled in a [Shadow] ([DefaultShadow];
// [Plugin.WithShadow] for another), the cone's edge stroked. The views drawn follow the ground and
// the cover where the scan samples them, not the entities; what the scan found (Sighted) stays
// the truth of who sees whom.
//
// Otherwise — a world without heights whose observer carries a [SightOutline], its view cut by
// the entities as the scan found it, or a [ConeStyle] of one's own ([Plugin.WithStyle],
// [ConeStyleFn]) — an entity carrying [SightOutline] has its view's shape computed: a reach per evenly spaced angle across the cone, and in a world with heights up
// to [MaxShadowsPerSample] [Band]s of ground out of sight per angle (aabbworld's View.Shadows).
// The Renderer, a render.Source then, hands it to a scene's render.Composer on the
// render.Overlays tier as a ring of [ConePoint]s in a ConeStyle ([DefaultConeStyle] strokes it),
// draped over the world's Ground when it has one ([Renderer.WithGround]), the shadows veiled over
// it in pieces of the ground's step. A game that wants the shape on the CPU keeps SightOutline on
// its observers; it costs a scan that much more.
//
// The views start hidden. The plugin is a plugin.CommandHandler, its one key the players carry:
// Shift+C ([Cones]) shows every view drawn — the cones and the shadows — and hides them again;
// [Plugin.Hide] does the same from code, [Plugin.Hidden] reports it. Hidden views are a look at
// the world, like the camera's turn: they change at once, in the tactical pause too, and are not
// saved. The scan goes on either way.
package vision
