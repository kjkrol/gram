package order

import "github.com/kjkrol/gram/plugins/board/cell"

// MaxPathLength bounds Path.Steps; a longer route is fetched in chunks.
const MaxPathLength = 64

// Path is the cached route toward MoveOrder.Target, consumed step by step. Length 0 is a route
// not yet planned.
type Path struct {
	Steps  [MaxPathLength]cell.ID
	Decks  uint64  // which level each of Steps lies on, its low bit by step
	From   cell.ID // the cell the route begins in
	Length uint16
	Index  uint16
	Upper  uint64 // with Decks the level of each of Steps, its high bit by step
}

// At is the cell p's route stands at: the last step taken, or From.
func At(p *Path) cell.ID {
	if p.Index == 0 {
		return p.From
	}
	return p.Steps[p.Index-1]
}

// LevelAt is the level of its cell p's step k lies on.
func LevelAt(p *Path, k uint16) cell.Level {
	if k >= 64 {
		return cell.OnGround
	}
	return cell.Level(p.Decks>>k&1 | p.Upper>>k&1<<1)
}

// SetLevel has p's step k lie on level l.
func SetLevel(p *Path, k uint16, l cell.Level) {
	p.Decks = p.Decks&^(1<<k) | uint64(l&1)<<k
	p.Upper = p.Upper&^(1<<k) | uint64(l>>1)<<k
}

// SetLevels has p's steps lie on the levels lo and hi say, as a finder gives them.
func SetLevels(p *Path, lo, hi uint64) { p.Decks, p.Upper = lo, hi }

// Through reports whether p's route goes on through c, its next step: a route to the ground
// under a deck may cross the deck over its goal first.
func Through(p *Path, c cell.ID) bool {
	return p.Index+1 < p.Length && p.Steps[p.Index] == c
}
