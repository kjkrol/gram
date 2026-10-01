package board

import "github.com/kjkrol/gram/plugins/board/cell"

// Mover says which domains an entity moves in and, in a world with heights, how far above the ground
// it keeps (Lift: a hawk hundreds of metres, a walker 0), the least it keeps over the ground
// (Clearance) and the highest over sea level it climbs to (Ceiling; 0 none). One that flies (Air),
// flown by hand (steering.Driven.Flown), holds its height over sea level instead, climbing and
// diving as it is steered, its Lift following; an entity on the board without it moves on Land.
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
