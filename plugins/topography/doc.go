// Package topography is a map in relief over a board: the ground's heights, the light and the
// water on them, drawn on the GPU, and the views of it — from above, isometric and in perspective.
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
// behavior), off a road and on one, unless its kind is Graded (board.CellKind.Graded): a road cut
// into the slope costs its own price alone. The ground changes as in Transport Tycoon: [Raise] and [Lower]
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
// leans what sways, lays the clouds' shadows and hazes the far off. The ground is lit on the GPU,
// every pixel by the sun on the slope under it; the relief and what stands on it cast shadows,
// baked on the GPU as the sun moves — a strip a frame while it goes on, all at once when it leaps
// ([Plugin.WithShadows] turns them off; [CoarseShadows], H, or [Plugin.WithCoarseShadows] bakes
// them half as fine a side, softer, for about half the GPU's work). The world's entities
// are lit by the sun on level ground, lean with the wind and cast their shadows on the relief away
// from the sun (sky.Sun.ShadowOf, laid over the ground on the GPU), from above as in relief. The
// relief is the board's Heights ([Plugin.Heights]), which sight and navigation read through the
// board.
//
// # Water
//
// A kind with a Shine glints ([Glint], the SeaGlint material of shaders/sea.wgsl): the shader ripples its
// surface with small waves and throws the sun back towards the eye, and within a few cells of a
// shore — the nearest cell that does not shine, worked out per corner of a square grid as the
// terrain changes ([Shore]) — the waves face it, roll in and break into foam. Water of a kind with a
// Flow runs instead ([Stream], the RunningWater material): down the slope of its cell, read off its
// corners, as fast as the Flow by the square root of the slope, averaged at each corner over the
// running cells meeting there, carried as a flow map — ripples and flecks of foam flowing on
// without a seam — and white where it runs fast: a rapid, a waterfall. The clouds' shadows lie over
// the water as over the ground.
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
// # Painted once
//
// Nothing of this is worked out per frame: a cell's read is kept while board.Board.CellVersion
// says it is as it was, a tile's blends and way (placed as if it stood at 0, 0) while the cells
// round it are. The tiles are dressed only to be painted: over a square grid the whole board is
// painted flat — every cell's base, the grounds running in, the ways and the crossings, 16 pixels
// a cell, its water beside it — a cell anew when it or a cell round it changes, the whole board
// when many do; over a hex grid the tiles are composed once from above (render.Still), anew when
// the board or the relief changes. The shader leaves out waves finer than a pixel or two, so the
// water far off calms instead of flickering.
//
// # Views
//
// The world is seen through the topography's camera, from above, isometrically — the 2:1 view of
// Transport Tycoon, a cell a diamond, heights lifting what stands — or, when the game says so
// (Config.Perspective), in perspective; [View] (Tab) goes round them at play, keeping the ground
// point in the middle of the screen, the heading, the pitch and how large a world unit is drawn
// there; the view is saved with the camera. Every view draws the same ground on the GPU; from above
// the world's entities lie over their boxes as the world draws them, in relief they stand as
// billboards upright on their centres at their altitudes, as wide as their boxes and as tall as
// their world.Z says (as the box is long without a height), hidden by the ground's depth where it
// stands before them; picking and the selection's outline follow, since they ask the world's Look. The isometric camera turns by any angle — the world clockwise on the screen as the
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
// Riding in a unit ([LookOut], first person) the eye is the unit's, where its world.Eye stands
// — Height over its bottom, its top without one, a unit 2 m tall looking from 2 m — seeing across
// the screen as wide as the Eye's Angle says, the height following the screen's shape (the
// camera's own field of view without one), with a near plane a thousandth of a cell: it goes with
// it, looking the way the unit faces; [Look] (the mouse,
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
// # The ground on the GPU
//
// [Plugin.Renderer] draws the ground, a render.Direct at the Ground tier for the scene's composer
// beside the board's and the world's, and the board's tiles lay nothing (board.Nothing). Over a
// square grid it is a mesh of the relief's lattice (topography/terrain): every corner a vertex at
// its height, coloured from the board painted flat, lit per pixel with its shadows, its water
// glinting and running, its waves breaking on the shore, the clouds' shadows and the grid over
// it, hazed far off, and round the world a skirt of level ground running on to the horizon. Over a
// hex grid every cell is a prism standing to its top, a face down to each lower neighbour,
// coloured from the tiles composed from above. The world's entities stand on it as billboards
// drawn on the GPU against its depth, hidden where the ground stands before them; from above they
// lie over their boxes.
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
// island's demo does). Riding, W walks the unit on — with Shift held it sprints, to its
// steering's Sprint — S brakes it and then backs it away facing on, A and D turn it, the view
// turning with it, the mouse looks round — across turning the unit to face where the eye looks; one
// that flies, flown from inside, holds its height over sea level, climbing and diving only along
// the look, at least its Mover's Clearance over the ground and under its Ceiling — Q, E, R and F
// do nothing, and V or Tab leave it — back to the view the camera
// was in, over the unit; K lists those keys then. [Follow] (V without the perspective)
// fastens the camera behind the one selected unit: every tick the camera is centred on it at its
// altitude and turned, eased, until the way it walks runs up the screen. It holds whatever else is
// done — other units selected and ordered, the camera panned or turned — until V again lets it go,
// or the unit is gone; the lower the eye, the lower on the screen the unit stands, over its
// shoulder. In perspective a fastened eye, as high as it flies, looks down more steeply where the
// ground between them would hide the unit and eases back as the way clears. [LookFrom] puts the
// eye at a point of the world and [LookAt] has it look at one, in perspective, a camera in another
// view going there first where the game reaches it. [Drive] (W, S, A and D riding; the arrows
// following; W and the up arrow with Shift sprint) steers the unit a camera is fastened to: the
// camera system keeps a steering.Driven on it while fastened, writes the keys into it every tick —
// riding, Flown and the look's rise too — and leaves it braking when let go; navigation carries it
// out along the ground, the topography's altitude system up and down. [Raise],
// [Lower] and [Level] shape the ground; [CoarseShadows] (H) switches the shadows' detail. Call
// [Plugin.RunPlan] after the world has moved and before
// the players' RunPlan.
package topography
