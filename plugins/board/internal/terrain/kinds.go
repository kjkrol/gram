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
	taken   int // slots issued: next wraps round past the last
	// Guard, when set, is called with each kind's name as it is defined; it refuses one defined
	// out of its section.
	Guard func(name string)
	// Played, when set, notes the roles a kind's cells play, so their rules are delivered.
	Played func(roles ...*ceremony.Part)
}

// NewKinds is an empty dictionary.
func NewKinds() *Kinds {
	return &Kinds{
		entries: make(map[cell.Name]cell.Kind),
		plays:   make(map[cell.Name]tag.Tags[cond.Roles]),
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
	k.SpriteID = d.NewSprite()
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

// NewSprite reserves a slot of the board's atlas that belongs to no kind: a cover's. It panics
// once every one of the render.MaxSprites is taken.
func (d *Kinds) NewSprite() render.SpriteID {
	if d.taken == render.MaxSprites {
		panic(fmt.Sprintf("board: every one of the %d atlas sprites is taken", render.MaxSprites))
	}
	id := d.next
	d.next++
	d.taken++
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
