// Package interiors keeps the rooms of the board's cells and who stands in each: it tells the
// rules as one comes in and goes out, and has whoever is in a room drawn in the light that gets
// into it.
//
// [NewPlugin] takes the world and the board. Run its RunPlan after the board's.
//
// # A room
//
// A room is a thing standing in pieces (world.Whole): its pieces are the floors with a room of
// its cells (cell.Floor with Clear, dug or Standing), its handle marked [Room], its floor as high
// as the handle's world.Z. What of the sky's light gets into it is the handle's light.Shelter,
// laid by whoever builds the room: the sun by its openings, windows and doors on its walls, a
// share of the sky's light and of the sun's thrown round, and its lamp. A house's room is the
// buildings plugin's, a tunnel's and a chamber's the underground's.
//
// # Who is in it
//
// An entity on the board stands in the room whose floor its feet are on: it carries [Inside]
// while it does, the rules hosted on [Entered] and [Left] told once as it comes in and goes out,
// and is light.Sheltered under its handle's shelter, where it has one: drawn in the light that
// gets in there, worked out on the GPU pixel by pixel, casting no shadow. [Read] says which room
// lies on a cell's storey, whether an entity drawn uncovered stands in it (for whoever draws what
// covers the room) and its lamp's light.
//
// # Vocabulary
//
// Moments, for a role's rules: [Entered], [Left]. Components: [Room] (marking a room's handle),
// [Inside] (on whoever stands in one).
package interiors
