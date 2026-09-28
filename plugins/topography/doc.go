// Package topography is a map in relief over a board: the ground's heights, the light and the
// water on them, and the two views of it — from above and isometric.
//
// [NewPlugin] takes the world, the board and the [Config] — the isometric view's cell and tile
// sizes, whether a fresh game begins isometric, the [Shaping] and the [Climbing] — and puts the
// board in relief at once: it is the board's Map (board.Plugin.WithMap) — its Look, its Dressing,
// its heights and its costs — the world's Ground, and the maker of the world's cameras. The world
// must have heights (world.Config.Heights) and may not wrap.
//
// # Relief
//
// The ground's heights are a [Relief]: on a square grid a lattice of corners the neighbouring
// cells share where they meet, so the ground runs on between the cells and has no vertical walls;
// on any other grid a level per cell. [Relief.Corners] is a cell's four heights ([Corners]),
// [Relief.Altitude] its level, [Relief.GroundAt] the ground under any point, read between the
// corners; [Relief.SetHeights] raises the ground to a function ([MeanOfCells] builds one from a
// height per cell), [Plugin.Seed] does so when a game starts fresh. The heights live on the
// topography's own entity ([Heights]), saved with the game. Every step the topography puts every
// unit at the ground under it plus its Mover's Lift, so a unit never declares where it stands in
// height and a hawk declares only how high it flies. A kind's Height is what stands on the cell.
//
// # Slopes and shaping
//
// [Climbing] says what a slope does to whoever goes over it: a climb slows by Up per unit of rise
// over run; a descent is quickest, by Down, at a fall of Ease and slows past it by Steep a unit;
// a Free domain (Air by default) flies over. The slope multiplies the kind's cost, in the planner
// ([Plugin.Climb], [Plugin.Least]) and on the move ([Plugin.Slope], through the board's Moving
// behavior), on a road and off it. The ground changes as in Transport Tycoon: [Raise] and [Lower]
// move the corner nearest a point (a hex cell on a hex grid) by a [Shaping] Step, [Level] brings
// an area to the height where it began, and the ground round about follows until no two corners
// along a cell's edge differ by more than MaxStep — = and - under the cursor, a left drag with L
// held; at once, in the tactical pause too. [Relief.Lift] and [Relief.Flatten] do the same from a
// game's code.
//
// # Styles
//
// How a kind looks in relief beyond its sprite is its [Style], set by its name ([Plugin.Style]):
// its Shine, its Flow, its Spread, whether it lies Under the others, what it MixWith. The board's
// kinds keep what play needs; an effect that turns a kind into another (snow, ice) turns it into
// that kind's Style too, and a way's kind is styled the same way.
//
// # Light and shadows
//
// The relief stands under an [Atmosphere] — plugins/atmosphere's Plugin ([Plugin.WithAtmosphere]),
// or, given none, sky.DefaultSun in still, clear air: its sun lights and shades it, its weather
// leans what sways, lays the clouds' shadows and hazes the far off. A tile is lit by the sun per
// corner, from the slope of the ground there and at the neighbours', so a slope runs on without a
// seam, and an upright face as much as the top's edge over it: a map in relief, from above as
// isometrically. The terrain casts shadows: a corner the ground or what stands on it hides from
// the sun, walked towards it up to 16 cells, gets the ambient light alone. Shadows and light are
// worked out as cells come into sight and kept until the terrain or the sun changes;
// [Plugin.WithShadows] turns the shadows off. The world's entities are lit by the sun on level
// ground, lean with the wind and cast their shadows on the relief away from the sun (sky.Sun.Shadow),
// from above as in relief. The relief is the board's Heights ([Plugin.Heights]), which sight and
// navigation read through the board.
//
// # Water
//
// A kind with a Shine glints ([Glint], the SeaGlint material of water.kage): the shader ripples its
// surface with small waves and throws the sun back towards the eye, and within a few cells of a
// shore — the nearest cell that does not shine, worked out per corner of a square grid as the
// terrain changes ([Shore]) — the waves face it, roll in and break into foam. Water of a kind with a
// Flow runs instead ([Stream], the RunningWater material): down the slope of its cell, read off its
// corners, as fast as the Flow by the square root of the slope, averaged at each corner over the
// running cells meeting there, carried as a flow map — ripples and flecks of foam flowing on
// without a seam — and white where it runs fast: a rapid, a waterfall. The clouds of the world's
// weather shadow every tile once, over all that lies on it (render.Frame.OvercastOn).
//
// # Blends and coasts
//
// Kinds with a Spread run into each other along a line their cells draw, not along the cells'
// edges: over each quarter of a tile a neighbour's kind is weighed at the quarter's corners by the
// share of the cells meeting there that are of it and shown where the weight is over a half
// (render.Frame.SpriteBlend), fading in over the mean of the two Spreads; the weights at a corner
// or a side are the same from every tile, so a staircase of cells becomes a slant and a cell alone
// a rounded diamond. A kind Under the others — water — keeps its glint: a tile that spreads next to
// it is drawn as it and its own kind laid over it by the share of cells not under, and the water's
// tile has the land round it laid over it the same way, so a coast runs round.
//
// # Ways
//
// A board.Way is drawn as a band through its cell: each way out ends halfway to its neighbour, as
// wide as the mean of the two ways there; the two out to the widest neighbours are one band curving
// round the cell's middle, any other joins it curving in, so a winding stream bends smoothly; a way
// out to one neighbour alone ends square across itself. A way whose kind shines is water running
// down its band; a way whose kind's Style names a kind to MixWith takes on that kind's look as far
// as its Mix, the other sprite glazed over its own and blended along the band (render.Frame.Glaze)
// — a river turning into the sea's colour towards its mouth; a way's Fade has it show the less the
// further it has faded, down to nothing where it ends, its water running on level ground the way
// it fades: a river running out into the sea. A board.Crossing — a bridge — is cut into bands as a
// way is, on a tier over the ways, each band meeting whichever of its neighbour's way and crossing
// runs back to it: a road meets its bridge, not the river under it. A way running out into water no
// way runs across runs on to its middle under it: the water lies over a way as it lies over the
// grounds round a coast, so a way shows only where the land does and a river's end follows the
// coast. Ways lie on a tier just over the tiles; a band running slantwise reaches into the cells
// either side of the corner it runs through, so its last stretch takes the depth of the nearest of
// the four cells meeting there.
//
// # Kept, and less far off
//
// Nothing of this is worked out per frame: a cell's read is kept while board.Board.CellVersion
// says it is as it was, a tile's blends and way (placed as if it stood at 0, 0) while the cells
// round it are, its light while the terrain and the sun are. Far off, less is drawn — by how many
// pixels a cell spans where the tile is drawn (camera.ScaleAt at its middle), so through a
// perspective a tile near the eye has all of it and one on the horizon none: a tile's
// detail eases its water's glint and running out between 12 and 6 pixels a cell and its shore
// below 16, a way's between 24 and 16, and the shader leaves out waves finer than a pixel or two.
// Where a cell spans fewer than 16 pixels on a square grid, a tile's blends and the ways over it
// are painted once on a ground sheet — the board's atlas with the board's cells below it, 16
// pixels a cell — and the tile draws its top and all that lies on it as one piece of the sheet, in
// its light; a cell is painted anew when it or a cell round it changes, the whole board at once
// when many do. So a far view hands the frame about as many pieces as the sprites alone.
//
// # Views
//
// The world is seen through the topography's camera, from above, isometrically — the 2:1 view of
// Transport Tycoon, a cell a diamond, heights lifting what stands — or, when the game says so
// (Config.Perspective), in perspective; [View] (Tab) goes round them at play, keeping the ground
// point in the middle of the screen, the heading, the pitch and how large a world unit is drawn
// there; the view is saved with the camera. From above the board's tiles lie flat (board.FlatLook)
// and the world's entities as the world draws them; in relief the cells stand as blocks — each top
// sloped between its corners and raised by its kind's Height, its top leant with the wind if its
// kind sways, the faces turned towards the viewer where it stands above its neighbour — and the
// entities as billboards upright on their centres at their altitudes, as wide as their boxes and as
// tall as their world.Z says (as the box is long without a height), at the depth of that centre,
// which a render.Composer sorts by; picking and the selection's outline follow, since they ask the
// world's Look. The isometric camera turns by any angle — the world clockwise on the screen as the
// heading grows, keeping the ground point in the middle of the screen — and tilts from
// Config.MinPitch over the ground (30°, the 2:1 view's, unless the game lowers it: flatter, the
// near relief hides what lies behind it) to straight down. From above and in the isometric view
// the whole screen stays over the world at sea level, as the top-down camera keeps its window:
// a pan stops where a corner of the screen reaches the world's edge, and zooming out stops where
// the screen just fits over the world — so a rectangular screen never shows the ground beyond a
// diamond-shaped map, and never reaches the map's corners either. Zoom keeps the ground under the
// cursor where it is drawn, at its own height, as far as the screen stays over the world.
//
// The perspective is an eye flying over the world, never lower than two cells over its highest
// ground, seeing Config.FieldOfView from the top of the screen to the bottom (45° when zero): what
// is nearer the eye is larger. Pan (WASD, the cursor at an edge, a middle drag) moves it along
// the ground; Turn (Q and E) goes round the ground point in the middle of the screen, as high;
// Tilt raises the head (R: further off, no flatter than Config.MinPitch) or bows it (F: straight
// down at most), the eye where it is; Zoom comes in along the line to the ground under the cursor
// down to the ceiling and narrows the field of view from there — less of the ground, larger, the
// head turning so the ground under the cursor stays where it is drawn, the eye flying on towards
// it where the pitch floor holds the head — and zooming out widens it back, then lifts the eye, no
// higher than where the middle of the screen shows the world's diagonal across at the flattest
// pitch. The screen shows the horizon and what lies past the world's edge whatever the eye does,
// so the perspective keeps the ground point in the middle of the screen over the world instead:
// Pan stops where that point reaches the world's edge. [LookFrom] and [LookAt] put the eye and its
// look where the game wants them, over the edge too; the next Pan brings the middle back. On a world with a scale (world.Scale) the perspective shows the
// Earth: the ground far off sinks under the eye's level (world.Scale.Drop), level ground past the
// horizon out of sight, and fades to the sky's colour as far off as the air's Visibility says —
// the tiles, their faces and the units, not what is laid over the tiles (render.Frame.Haze).
// Riding in a unit ([LookOut], first person) the eye is the unit's, on its top — its world.Z,
// Altitude plus Height, a unit 2 m tall looking from 2 m — with a near plane a thousandth of a
// cell: it goes with it, looking the way the unit faces; [Look] (the mouse,
// the cursor captured) turns the view at once and the unit to face it, and raises and lowers the
// head into the sky and down to the feet; Zoom narrows the view, Pan and Turn do nothing; the
// camera is then a camera.Rider in camera.FirstPerson, and the bindings holding in that mode — the
// unit's WASD — fire instead of the free camera's. Through the perspective the sky's backdrop draws the sun,
// where the way towards it vanishes (camera.Vanisher). What a camera in relief hands its renderers
// — Bounds, the world rectangle they walk, and Visible — holds the ground the screen may show at
// any height from the relief's lowest ground to Headroom over its highest (Relief.Extent), not
// only at sea level: high ground below the screen's lower edge at sea level is drawn on it, and
// the ground about an eye riding low is in view. A tile some of whose corners lie beside or behind
// the eye — the one the eye stands in, those round it — is drawn in pieces, those wholly in front
// alone, the farthest first (a projection has no point for what lies behind the eye; drawn whole,
// such a tile would cover the sky); what is laid over tiles is left out where it is not wholly in
// front, and a tile's detail goes by its nearest corner in front. Its depth is how far ahead of it
// the middle of a cell lies, so a cell's tile and what stands on it draw together as ever; the composer sorts
// by cell, without a depth buffer, and a tile's texture is drawn affinely across its quad — the
// limits of the first version, seen on tall relief looked at low and on big tiles near the eye. The sun
// glints towards one direction for the whole screen, the eye's from the middle.
//
// The camera is a camera.Picker: the ground under a screen point is found walking the line of
// sight — from the eye in perspective, the Earth's curve and all, isometrically from over the
// highest top down — half a cell at a time over the top of the ground as it is drawn, a kind's
// Height standing on its cell, to the first point it passes under, then halving down to a
// thousandth of a unit. A click on a slope however steep, on the top of a raised kind, or, riding
// in a unit, on a slope above the eye lands on the cell drawn there; the ground point in the middle
// of the screen, which Turn goes round, is found the same way.
//
// # Commands
//
// The plugin is a plugin.CommandHandler, its commands carrying the camera of whoever gave them
// (control.Context.Camera), so it turns a player's camera without knowing players; a camera of
// another view stays as it is. [View] (Tab) switches the view; [Turn] turns it (Q and E held,
// [TurnStep] a tick); [Tilt] bows the head or raises it (F and R, [TiltStep] a tick);
// [LookOut], given the selection ([Plugin.WithSelection], whose Selected tag it reads as
// navigation does) and the perspective (Config.Perspective), is V: it rides in the one selected
// unit, first person — the eye in the unit, kept there as it goes, pinned to the way it faces,
// world.Base's Vel.Dir, which is the axis of its sight where the game turns the sight with it (the
// island's demo does). Riding, W walks the unit on, S stops it, A and D turn it, the view turning
// with it, the mouse looks round — across turning the unit to face where the eye looks — Q, E, R
// and F do nothing, and V or Tab leave it — back to the view
// the camera was in, over the unit; K lists those keys then. [Follow] (V without the perspective)
// fastens the camera behind the one selected unit: every tick the camera is centred on it at its
// altitude and turned, eased, until the way it walks runs up the screen. It holds whatever else is
// done — other units selected and ordered, the camera panned or turned — until V again lets it go,
// or the unit is gone; the lower the eye, the lower on the screen the unit stands, over its
// shoulder. In perspective a fastened eye, as high as it flies, looks down more steeply where the
// ground between them would hide the unit and eases back as the way clears. [LookFrom] puts the
// eye at a point of the world and [LookAt] has it look at one, in perspective, a camera in another
// view going there first where the game reaches it. [Drive] (W, S, A and D riding; the arrows
// following) steers the unit a camera is fastened to: the camera system keeps a steering.Driven on it
// while fastened, writes the keys into it every tick and stops it when let go; navigation carries it out on the ground. [Raise],
// [Lower] and [Level] shape the ground. Call [Plugin.RunPlan] after the world has moved and before
// the players' RunPlan.
package topography
