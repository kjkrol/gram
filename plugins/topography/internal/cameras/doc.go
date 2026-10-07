// Package cameras is the views of a map in relief: the cameras — from above, isometric and in
// perspective, an eye riding in a unit among them — how they project, pick and bound what they
// show, and their control, carrying out the commands that switch, turn and tilt them and keeping
// each one fastened behind or inside its unit as the camera says (camera.Fastening).
//
// [Maker] makes the world's cameras as a [Config] says, over the ground at any point and its
// extent; [Switch] turns a camera to the next view. A [Control] ([NewControl]) carries out the
// commands as its System runs, reading them as [Orders]: the topography keeps their queues and
// their keys (topography.View, Turn, Tilt, LookFrom, LookAt, Ride, Look) and puts the cameras
// its [Control.Maker] made to work over its relief.
//
// # Views
//
// The world is seen from above, isometrically — the 2:1 view of Transport Tycoon, a cell a
// diamond, heights lifting what stands — or, when the game says so (Config.Perspective), in
// perspective; View (Tab) goes round them at play, keeping the ground point in the middle of the
// screen, the heading, the pitch and how large a world unit is drawn there; the view is saved with
// the camera. The isometric camera turns by any angle — the world clockwise on the screen as the
// heading grows, keeping the ground point in the middle of the screen — and tilts from
// Config.MinPitch over the ground (30°, the 2:1 view's, unless the game lowers it: flatter, the
// near relief hides what lies behind it) to straight down. From above and in the isometric view
// the whole screen stays over the world at sea level, as the top-down camera keeps its window: a
// pan stops where a corner of the screen reaches the world's edge, and zooming out stops where the
// screen just fits over the world — so a rectangular screen never shows the ground beyond a
// diamond-shaped map, and never reaches the map's corners either. Zoom keeps the ground under the
// cursor where it is drawn, at its own height, as far as the screen stays over the world.
//
// The perspective is an eye flying over the world, never lower than two cells over its highest
// ground, seeing Config.FieldOfView from the top of the screen to the bottom (45° when zero): what
// is nearer the eye is larger. Pan (WASD, the cursor at an edge, a middle drag) moves it along the
// ground; Turn (Q and E) goes round the ground point in the middle of the screen, as high; Tilt
// raises the head (R: further off, no flatter than Config.MinPitch) or bows it (F: straight down at
// most), the eye where it is; Zoom comes in along the line to the ground under the cursor down to
// the ceiling and narrows the field of view from there — less of the ground, larger, the head
// turning so the ground under the cursor stays where it is drawn, the eye flying on towards it
// where the pitch floor holds the head — and zooming out widens it back, then lifts the eye, no
// higher than where the middle of the screen shows the world's diagonal across at the flattest
// pitch. The screen shows the horizon and what lies past the world's edge whatever the eye does, so
// the perspective keeps the ground point in the middle of the screen over the world instead: Pan
// stops where that point reaches the world's edge. LookFrom and LookAt put the eye and its look
// where the game wants them, over the edge too; the next Pan brings the middle back. On a world
// with a scale (world.Scale) the perspective shows the Earth: the ground far off sinks under the
// eye's level (world.Scale.Drop), level ground past the horizon out of sight, and fades to the
// sky's colour as far off as the air's Visibility says.
//
// Riding in a unit (fastened camera.Inside, first person) the eye is the unit's, where its world.Eye stands —
// Height over its bottom, its top without one, a unit 2 m tall looking from 2 m — seeing across the
// screen as wide as the Eye's Angle says, the height following the screen's shape (the camera's own
// field of view without one), with a near plane a thousandth of a cell: it goes with it, looking
// the way the unit faces; Look (the mouse, the cursor captured) turns the view at once and the
// unit to face it, and raises and lowers the head into the sky and down to the feet; Zoom narrows
// the view, Pan and Turn do nothing; the camera's fastening says Inside (camera.HowOf), its eye
// where camera.Eyed says, and the bindings holding there — the players' WASD driving the unit —
// fire instead of the free camera's. Through the perspective the sky's backdrop draws the sun, where the
// way towards it vanishes (camera.Vanisher). What a camera in relief hands its renderers — Bounds,
// the world rectangle they walk, and Visible — holds the ground the screen may show at any height
// from the relief's lowest ground to Headroom over its highest (relief.Relief.Extent), not only at
// sea level: high ground below the screen's lower edge at sea level is drawn on it, and the ground
// about an eye riding low is in view. Every view hands the GPU its transform
// (camera.SceneTransform), and a view in relief sorts what it draws by depth
// (camera.Projection.Sorts).
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
// The commands carry the camera of whoever gave them (control.Context.Camera), so the control
// turns a player's camera without knowing players; a camera of another view stays as it is. View
// (Tab) switches the view; Turn turns it (Q and E held, TurnStep a tick); Tilt bows the head
// or raises it (F and R, TiltStep a tick). What a camera is fastened to is the camera's own
// (camera.Fastening): the players' Follow fastens it Centred over the one selected unit, and Ride
// (V) takes it closer round: Behind the unit — every tick the camera is centred on it at its
// altitude and turned, eased, until the way it walks runs up the screen, whatever else is done,
// other units selected and ordered, the camera panned or turned; the lower the eye, the lower on
// the screen the unit stands, over its shoulder; in perspective a fastened eye, as high as it
// flies, looks down more steeply where the ground between them would hide the unit and eases back
// as the way clears — then, where the game reaches the perspective, Inside it, first person — the
// eye in the unit, kept there as it goes, pinned to the way it faces, world.Base's Vel.Dir, which is
// the axis of its sight where the game turns the sight with it (the island's demo does); the
// players' W walks the unit on — with Shift held it sprints, to its steering's Sprint — S brakes it
// and then backs it away facing on, A and D turn it, the view turning with it, the mouse looks
// round — across turning the unit to face where the eye looks; one that flies, flown from inside,
// holds its height over sea level, climbing and diving only along the look, at least its Mover's
// Clearance over the ground and under its Ceiling — Q, E, R and F do nothing, and V or Tab take
// the eye out, over the unit again; K lists those keys then — and over it again, Centred, the
// players' to keep. The system keeps every camera the Control's Maker made in line with its
// fastening: a fastening changed, let go (the players' Follow again, a Pan) or its unit gone ends
// the keeping, the eye coming out. LookFrom puts the eye at a point of the world and LookAt has it
// look at one, in perspective, a camera in another view going there first where the game reaches
// it; both let go of a unit the eye rides in. The unit is driven by the players' hand, which
// navigation carries out along the ground (steering.Driven); the control writes only the eye's
// part of it — the way to face while the look turns the unit, Flown and the look's rise — and
// clears it on letting go.
package cameras
