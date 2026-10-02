package terrain

import (
	"fmt"

	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/render"
)

// Kinds is the board's cell.Kinds: the kinds a game created, by name, each given a sprite.
type Kinds struct {
	entries map[cell.Name]cell.Kind
	drawers map[cell.Name]render.SpriteDrawer
	next    render.SpriteID
	heights bool
}

// NewKinds is an empty dictionary; one of a flat world refuses a kind with a Height.
func NewKinds(heights bool) *Kinds {
	return &Kinds{entries: make(map[cell.Name]cell.Kind), drawers: make(map[cell.Name]render.SpriteDrawer), heights: heights}
}

func (d *Kinds) Draw(name string, draw render.SpriteDrawer) {
	d.drawers[cell.Named(name)] = draw
}

func (d *Kinds) Create(kinds ...cell.Kind) {
	for _, k := range kinds {
		if !d.heights && k.Height != 0 {
			panic(fmt.Sprintf("board: kind %q has a Height in a flat world; set world.Config.Heights", k.Name.String()))
		}
		k.SpriteID = d.next
		d.next++
		d.entries[k.Name] = k
	}
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

// Drawer is the sprite drawn for the kind named name, if one was (Draw).
func (d *Kinds) Drawer(name cell.Name) (render.SpriteDrawer, bool) {
	draw, ok := d.drawers[name]
	return draw, ok
}
