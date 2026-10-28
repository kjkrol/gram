package unit

import "github.com/kjkrol/gram/plugins/board/cell"

// At is the cell an entity on the board stands in, kept by whoever moves it, and Next the cell
// it steps into next, set by whoever leads it: its own cell for a step within it, 0 while nobody
// leads it.
type At struct{ Cell, Next cell.ID }

// Mover says which domains an entity moves in and how far above the ground it keeps (Lift), the
// least it keeps over it (Clearance) and the highest over sea level it climbs to (Ceiling; 0 none).
// An entity on the board without one moves on Land.
type Mover struct {
	Domain    cell.Domain
	Lift      float32
	Clearance float32
	Ceiling   float32
}

// DomainAt is the domain of the i-th entity of a chunk whose Mover column may be absent.
func DomainAt(movers []Mover, i int) cell.Domain {
	if movers == nil {
		return cell.Land
	}
	return movers[i].Domain
}
