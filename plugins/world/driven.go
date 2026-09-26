package world

// Driven is an entity steered by hand: Ahead 1 to walk on the way it faces, -1 to stop, 0 to let
// it go on as ordered or stand; Turn -1, 1 or 0 to turn anticlockwise, clockwise or not. Whoever
// steers it — a camera fastened behind it, say — writes it every tick; the plugin that moves
// entities over the ground carries it out, keeping them where they may go.
type Driven struct{ Ahead, Turn int8 }
