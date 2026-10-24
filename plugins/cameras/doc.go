// Package cameras makes the cameras a game looks at its world through and moves them: from above
// over any world, and over a world in relief on a stick, turned and leaned, orthographically or
// in perspective, riding with an entity.
//
// # Making cameras
//
// [NewPlugin] takes the world and a [Config] and makes no camera: each is made where a scene
// shows it ([Write.New], render.NewFeed), beginning as its [Start] says, with a camera.Config of
// its own, and sized by whoever shows it. The zero Config is the plain camera from above, which
// wraps on a wrapping axis; one that turns, tilts, reaches the perspective or has a [Cage] sees
// the world in relief, a Cell of it drawn isometrically as a TileW x TileH diamond, and refuses a
// world that wraps. camera.Config.Follow names the entity a camera starts fastened Centred over.
// Every camera made is saved with the game, in the order made, and one made after a Load takes
// the state of the camera made in its place. The world's sectors are anchored at the point each
// camera observes (camera.Focused), a minimap's none. Every camera sees as Config.Detail says
// (camera.Detailed): how far, the fog before it, the scale below which it draws coarsely; [Sight]
// changes how far, saved with the camera.
//
// # Profiles
//
// Every picture but a minimap's is drawn by the [Knobs] of its camera's [Profile] ([ProfileOf]):
// Tactical for a camera free, over or behind an entity, in any view, Riding for one inside an
// entity (Config.Tactical and Riding to begin with, Settings.Tactical and Riding after); a camera
// whose state changes is dressed anew as the cameras' pass ends. A picture is drawn at its knobs'
// Resolution of the pixels it is shown at (camera.Resolved; the ui stays sharp), [Picture] sets
// it and [Resolution] steps a camera's, and [Adapt] has it follow Settings.FrameBudget as the
// knobs' [Strategy] says: coarser while the frames come slower, finer again while they keep up,
// the whole picture's (Uniform) or layer by layer (InLayers). A picture drawn whole below all its
// pixels draws its figures, all that lies over the ground, at every pixel while its ground is
// drawn at the resolution (camera.Figured; Knobs.SoftFigures and [SharpFigures] have them follow
// the ground's resolution instead, to compare). Drawn in layers (Knobs.Layers, [Layers],
// camera.Layered) a picture is the whole frame at OuterRes of its pixels, its central Middle at
// MiddleRes and its central Inner at the Resolution ([LayerShares], [LayerResolutions]; 0.6, 60%
// at 0.8, 35% at all to begin with), each layer inset in the frame but in a tactical perspective,
// where it stands on the frame's bottom edge, the near ground (camera.Footed); a step takes one
// layer at a time: coarser the outermost first, each down to a floor, finer the innermost first.
// A hand on the resolution or a layer's stops its profile's following (Still); the Settings show
// the first camera's picture in its profile's knobs, the settings screen a section a profile. The
// telemetry panel shows it, naming the profile ([Read.Reporter]); [ShowLayers] outlines a
// profile's layers.
//
// # Cage
//
// A [Cage] is where the cameras may be: a game hands the one of its map (the topography is one)
// or its own. A line of sight stops where it is not Free, so a camera picks the point under the
// cursor on the top of what is solid there; a [Grained] cage steps finer near what is thin.
//
// # Views
//
// In relief a camera hangs on a stick whose foot is the point in the middle of the screen: a pan
// slides the foot over the world at its height, [Turn] turns the stick about the upright through
// it, [Tilt] leans it from straight down to Config.MinPitch over the ground. Isometrically (the
// 2:1 view of Transport Tycoon, heights lifting what stands) is the stick turned an eighth and
// leaning at asin(TileH/TileW). The view is orthographic, Zoom screen pixels a world unit, or,
// where the Config reaches it, in perspective; [View] switches between the two keeping the foot,
// the turn, the pitch and the scale. Orthographically the whole screen stays over the world at
// sea level: a pan stops at the edge, and zooming out stops where the screen just fits the world.
// In perspective the eye stands at the stick's end, the wheel drawing it and the foot towards the
// cursor; where the cage puts something solid at the eye, it is lifted straight up and let down
// as the way clears. Behind an entity the eye is held on an arm, shortened where the cage puts
// something solid between; a camera stands as near as that arm in either projection there,
// nothing zooming it, and gives its tactical view back on letting go. On a world with a scale
// the ground far off sinks under the eye's level.
// Riding in an entity the eye is the entity's world.Eye, going with it and looking the way it
// faces. A camera in relief saved before the stick begins afresh, the log naming it.
//
// # Commands
//
// [Pan] moves a camera by screen pixels and lets go of whatever it was fastened to; [Zoom] scales
// it about a world point; [Follow] fastens it Centred over an entity, kept in the middle of the
// screen as it goes, or lets it go; [CenterOn] moves it once onto an entity, [CenterAt] onto the
// ground at a world point (a click on a minimap), fastened to nothing.
// In relief [View] switches the projection, [Turn] and [Tilt] move the stick, [Ride] takes it
// behind and inside the entity it is fastened to, [LookFrom] and [LookAt] place the eye in
// perspective and [Look] looks round riding, the cursor captured. [Resolution] draws a camera's
// picture at a factor of its pixels, a layer at a time in layers. Each names its camera, taken
// from the context by its binding, so the plugin never knows the players. The picture's
// commands name a [Profile] and no camera: [Adapt] has the resolution of the profile's pictures
// follow the frame budget as its Strategy says or stay, [Picture] draws them at a resolution,
// [SharpFigures] their figures at every pixel or at the ground's resolution, [Layers] in layers
// or whole, [LayerShares] and [LayerResolutions] shape the layers, [ShowLayers] outlines them, a
// profile each; [Budget] sets the frame budget for both and [Sampling] how long the rate is
// averaged over before it is followed. What else follows the budget is a [Lever] given in
// [Config.Levers], the topography's care: turned coarser before the pictures' resolution and
// finer after it. The budget is followed on the frame's barrier ([Plugin.Pace], told each
// frame's times): coarser only while the drawing's side holds the frames back, not the
// simulation, which no lever lightens. The plugin writes only the eye's part
// of a ridden entity's steering.Driven (Look, Flown, Climb).
//
// # Keys
//
// [Keys] are a player's keys to its camera: four keys held scrolling it, the wheel zooming about
// the cursor, the middle drag panning, the cursor at an edge scrolling and in a corner scrolling
// slantwise, or turning as Q and E do (Keys.CornersTurn), two with Ctrl seeing farther and
// nearer. [DefaultKeys] are W, S, A, D, = and -, Ctrl+= and Ctrl+-, with all three;
// [Plugin.DefaultBindings] are those, the corners turning where the Config turns, and the views'
// keys the Config gives: Q and E turning, R looking down more steeply and F flatter, Tab
// switching between the orthographic view and the perspective, V riding, the mouse looking round
// inside an entity. The scroll zones and the pictures' resolution have no key: the settings
// screen sets them.
//
// # Vocabulary
//
// Commands, a player's through its camera: [Pan], [Zoom], [CenterOn], [CenterAt], [View], [Turn],
// [Tilt],
// [LookFrom], [LookAt], [Ride], [Look], [Sight], [Resolution]; a player's about a [Profile]'s
// pictures: [Adapt] (the resolution following the frame budget as a [Strategy] says),
// [Picture] (the resolution), [SharpFigures] (the figures at every pixel), [Layers] (drawn in
// layers), [LayerShares] and [LayerResolutions] (the layers' shares of the frame and
// resolutions), [ShowLayers] (a look: the layers outlined); about every picture: [Budget] (the
// frame budget), [Zones] (a look: the scroll zones on the glass over every picture, over
// whatever a scene lays on it), [EdgeScroll] (the cursor at the edge scrolling, or not),
// [EdgeSpeed] (how fast at the very edge, a quarter of it at the zone's inner bound); [Follow]
// (a player's, or an entity's own fastening the camera over itself).
// Settings ([Settings], saved with the game, in [Resources]): the zones scrolling, shown and
// their speed, the frame budget, the layers shown and each profile's [Knobs]: the strategy, the
// resolution, the figures at every pixel, the picture drawn in layers, the layers' shares and
// resolutions.
package cameras
