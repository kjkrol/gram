package world

import (
	"fmt"

	"github.com/kjkrol/ceremony/comp"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/entity/kind"
	ikinds "github.com/kjkrol/gram/plugins/world/internal/kinds"
	"github.com/kjkrol/uid"
)

// Whole marks the kind of the handle of a thing standing in pieces, a bridge or a wall: the handle
// carries what is the whole's (its joint box as its Position, its Z, a Lie where it lies along,
// its Shadow) and no sprite, and its pieces go as it goes.
type Whole struct{}

// Unseen marks a kind none of the world's looks draw, its sprite left undeclared: what its
// plugin draws by a source of its own, a tunnel's piece under the ground.
type Unseen struct{}

// PieceOf is a piece's handle, the Whole it stands as a piece of; SpawnWhole and SeedWhole put it
// on every piece they spawn.
type PieceOf struct{ Whole uid.UID64 }

// takeWhole takes a SpawnWhole's handle and pieces into the step's groups, all of them or none
func (s *spawnSystem) takeWhole(cmd SpawnWhole) error {
	by, _, err := s.checked(cmd.Whole, placing{handle: true})
	if err != nil {
		return err
	}
	if !by.handle {
		return fmt.Errorf("kind %q: a handle whose kind carries no Whole", by.kind.Name)
	}
	s.pieces = s.pieces[:0]
	for _, e := range cmd.Pieces {
		piece, _, err := s.checked(e, placing{})
		if err != nil {
			return fmt.Errorf("a piece of %q: %w", by.kind.Name, err)
		}
		s.pieces = append(s.pieces, piece)
	}
	if err := s.room(by.kind.Name, 1+len(cmd.Pieces)); err != nil {
		return err
	}
	slot := len(s.made)
	s.made = append(s.made, 0)
	s.handles = s.group(s.handles, by, cmd.Whole, placing{handle: true, slot: slot})
	for i, e := range cmd.Pieces {
		s.groups = s.group(s.groups, s.pieces[i], e, placing{piece: true, slot: slot})
	}
	return nil
}

// spawns the groups of handles, noting each in made by its slot, then the groups, each piece
// under its handle
func (w *module) spawnHandled(handles, groups []spawnGroup, made []uid.UID64, cb *goke.CmdBuf,
	partOf, pieceOf goke.CompID) {
	for _, g := range handles {
		w.spawnRows(g.by, g.rows, g.entries, g.places, cb, partOf, pieceOf)
		for _, p := range g.places {
			made[p.slot] = p.id
		}
	}
	for _, g := range groups {
		for k := range g.places {
			if p := &g.places[k]; p.piece {
				p.whole = made[p.slot]
			}
		}
		w.spawnRows(g.by, g.rows, g.entries, g.places, cb, partOf, pieceOf)
	}
}

// wholeSeeded is why the world refuses a whole seeded, nil where it takes it
func (p *Plugin) wholeSeeded(sw SpawnWhole) error {
	k, err := p.seedKind(sw.Whole)
	if err != nil {
		return err
	}
	if !carries[Whole](k) {
		return fmt.Errorf("world: kind %q: a handle whose kind carries no Whole", k.Name)
	}
	for _, e := range sw.Pieces {
		piece, err := p.seedKind(e)
		if err != nil {
			return err
		}
		// the piece's factory gives it PieceOf, which a kind's own would give twice
		if carries[PieceOf](piece) {
			return fmt.Errorf("world: kind %q: a piece whose kind carries a PieceOf of its own",
				piece.Name)
		}
	}
	return nil
}

// populateWholes has the wholes seeded spawned as the Stage starts fresh, the handles first and
// every piece carrying PieceOf its handle from the start
func (w *module) populateWholes(wholes []SpawnWhole) {
	w.seeds = append(w.seeds, goke.SystemFn{OnInit: func(si *goke.SysInit) {
		spawners := map[spawnerKey]*spawner{}
		spawnerFor := func(e kind.Entry, pieces bool) (*spawner, ikinds.Kind) {
			k, _ := w.kinds.r.Kind(e.Kind())
			key := spawnerKey{k.TypeID, pieces}
			if spawners[key] == nil {
				spawners[key] = newSpawner(si, k, pieces)
			}
			return spawners[key], k
		}
		var handles, groups []spawnGroup
		count := 0
		for slot, sw := range wholes {
			by, k := spawnerFor(sw.Whole, false)
			if err := w.handleSizeOf(k.Position.Resolve(sw.Whole.Row())); err != nil {
				panic(fmt.Sprintf("world: kind %q, a whole's handle: %v", k.Name, err))
			}
			handles = grouped(handles, by, sw.Whole, placing{handle: true, slot: slot})
			for _, e := range sw.Pieces {
				by, k := spawnerFor(e, true)
				if err := w.sizeOf(k.Position.Resolve(e.Row())); err != nil {
					panic(fmt.Sprintf("world: kind %q, a piece of a whole: %v", k.Name, err))
				}
				groups = grouped(groups, by, e, placing{piece: true, slot: slot})
			}
			count += 1 + len(sw.Pieces)
		}
		w.reserve(count)
		w.telemetry.Count += count
		w.spawnHandled(handles, groups, make([]uid.UID64, len(wholes)), nil, 0, 0)
		w.reindex(si)
	}})
}

type spawnerKey struct {
	kind   kind.ID
	pieces bool
}

// carries reports whether k's template gives its entities a T
func carries[T any](k ikinds.Kind) bool {
	for _, c := range k.Comps {
		if _, ok := c.(comp.Template[T]); ok {
			return true
		}
	}
	return false
}
