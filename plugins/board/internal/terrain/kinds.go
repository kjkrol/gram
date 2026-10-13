package terrain

import (
	"fmt"

	"github.com/kjkrol/ceremony"
	"github.com/kjkrol/ceremony/cond"
	"github.com/kjkrol/ceremony/tag"
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/render"
)

// Kinds is the board's cell.Kinds: the kinds a game defined, by name, each given a sprite and
// the roles its cells play.
type Kinds struct {
	entries map[cell.Name]cell.Kind
	plays   map[cell.Name]tag.Tags[cond.Roles]
	next    render.SpriteID
	heights bool
	// Guard, when set, is called with each kind's name as it is defined; it refuses one defined
	// out of its section.
	Guard func(name string)
	// Played, when set, notes the roles a kind's cells play, so their rules are delivered.
	Played func(roles ...*ceremony.Part)
}

// NewKinds is an empty dictionary; one of a flat world refuses a kind with a Height.
func NewKinds(heights bool) *Kinds {
	return &Kinds{
		entries: make(map[cell.Name]cell.Kind),
		plays:   make(map[cell.Name]tag.Tags[cond.Roles]),
		heights: heights,
	}
}

func (d *Kinds) Define(name string, k cell.Kind, plays ...*ceremony.Part) {
	if d.Guard != nil {
		d.Guard(name)
	}
	k.Name = cell.Named(name)
	if _, ok := d.entries[k.Name]; ok {
		panic(fmt.Sprintf("board: the cell kind %q is defined already", name))
	}
	if !d.heights && k.Height != 0 {
		panic(fmt.Sprintf("board: kind %q has a Height in a flat world; set world.Config.Heights", name))
	}
	k.SpriteID = d.next
	d.next++
	d.entries[k.Name] = k
	if len(plays) > 0 {
		var roles tag.Tags[cond.Roles]
		for _, r := range plays {
			roles = roles.With(r.Tag())
		}
		d.plays[k.Name] = roles
		if d.Played != nil {
			d.Played(plays...)
		}
	}
}

// Named is the kind defined as name, as the handle a game builds on; an unknown name panics.
func (d *Kinds) Named(name string) cell.Of {
	k, ok := d.Get(name)
	if !ok {
		panic(fmt.Sprintf("board: no cell kind is defined as %q", name))
	}
	return cell.OfKind(k)
}

// Plays are the roles the cells of the kind named name play; zero for none, "" for no kind.
func (d *Kinds) Plays(name string) tag.Tags[cond.Roles] {
	if len(name) > cell.MaxNameLen {
		return 0
	}
	return d.plays[cell.Named(name)]
}

// NewSprite reserves a slot of the board's atlas that belongs to no kind: a cover's.
func (d *Kinds) NewSprite() render.SpriteID {
	id := d.next
	d.next++
	return id
}

func (d *Kinds) Get(name string) (cell.Kind, bool) {
	if len(name) > cell.MaxNameLen {
		return cell.Kind{}, false
	}
	k, ok := d.entries[cell.Named(name)]
	return k, ok
}

func (d *Kinds) All() []cell.Kind {
	all := make([]cell.Kind, 0, len(d.entries))
	for _, k := range d.entries {
		all = append(all, k)
	}
	return all
}
