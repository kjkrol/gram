// Package occupancy keeps a board's cell.Occupancy true to the world: KeepSystem has every unit
// hold the cell it stands on, following it wherever it goes, and lets go of the holds of whoever
// left the world; PlaceSystem moves a part laid off its aggregate onto a cell taking it; View
// draws the cells held.
package occupancy
