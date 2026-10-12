package unit

import "github.com/kjkrol/gram/plugins/board/cell"

// At is the cell an entity on the board stands in, kept by whoever moves it.
type At struct{ Cell cell.ID }

// Mover says which domains an entity moves in and, in a world with heights, how far above the
// ground it keeps (Lift), the least it keeps over it (Clearance) and the highest over sea level it
// climbs to (Ceiling; 0 none). An entity on the board without one moves on Land.
type Mover struct {
	Domain    cell.Domain
	Lift      float64
	Clearance float64
	Ceiling   float64
}

// DomainAt is the domain of the i-th entity of a chunk whose Mover column may be absent.
func DomainAt(movers []Mover, i int) cell.Domain {
	if movers == nil {
		return cell.Land
	}
	return movers[i].Domain
}
