package board

import (
	"github.com/kjkrol/gram/internal/parallel"
	"github.com/kjkrol/gram/plugin/section"
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/board/ground"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/render"
)

// Read reads the board's state on one goroutine at a time: Plugin.Read(w), worker w's.
type Read struct {
	p     *Plugin
	board *BoardRead
}

var _ parallel.Forkable[*Read] = (*Read)(nil)

// Fork reads the same state through queries and scratch of its own, for another goroutine.
func (r *Read) Fork() *Read { return &Read{p: r.p, board: r.board.Fork()} }

// Board reads the board: its grid and its terrain.
func (r *Read) Board() *BoardRead { return r.board }

// Drawn is the board as the frame drawn reads it: its cells as last published, the live board
// before any was.
func (r *Read) Drawn() Drawn { return r.p.board.drawnBoard() }

// World is the world the board lies over: its registers, for a figure defined over the board.
func (r *Read) World() *world.Plugin { return r.p.worldPlugin }

// CellKinds are the kinds of cells registered.
func (r *Read) CellKinds() cell.Kinds { return r.p.kinds }

// Heights is the board's ground heights, its Map's; nil on a flat map.
func (r *Read) Heights() ground.Heights { return r.p.board.mapping.Heights() }

// Footing is where whoever moves over the board stands, the ground or a deck; nil on a flat map.
func (r *Read) Footing() ground.Surfaces {
	if r.Heights() == nil {
		return nil
	}
	return surfaces{r.board}
}

// Cover is what stands on the board and holds sight back.
func (r *Read) Cover() ground.Cover { return r.board.cover }

// Occupancy is who holds the cells: the occupancy the plugin was built with.
func (r *Read) Occupancy() cell.OccupancyRead { return r.p.occupancy }

// Climb is the Map's Climb.
func (r *Read) Climb(from, to cell.ID, d cell.Domain) float64 {
	return r.p.board.mapping.Climb(from, to, d)
}

// Least is the Map's Least.
func (r *Read) Least(d cell.Domain) float64 { return r.p.board.mapping.Least(d) }

// Atlas is the sprite sheet the board's renderer draws the cells from; nil before it is closed.
func (r *Read) Atlas() render.AtlasSource { return r.p.atlas }

// GridShown reports whether the grid is drawn.
func (r *Read) GridShown() bool { return r.p.Res.Settings.Grid }

// HoldsShown reports whether the cells held are drawn.
func (r *Read) HoldsShown() bool { return r.p.Res.Settings.Holds }

// Write changes the board's state, on the game's goroutine.
type Write struct{ p *Plugin }

// Board writes the board's terrain.
func (w *Write) Board() *BoardWrite { return &w.p.board.write }

// Occupancy is who holds the cells, to take and let go of them.
func (w *Write) Occupancy() cell.Occupancy { return w.p.occupancy }

// Seed sets the terrain applied when the Stage starts fresh.
func (w *Write) Seed(layout Layout) {
	if err := w.p.worldPlugin.InSection("the board's layout seeded", section.Spawn); err != nil {
		panic("board: " + err.Error())
	}
	w.p.seeded = &layout
}

// SetAtlas takes atlas as the board's looks for the board's renderer; a *Atlas's Close calls it,
// a foreign render.AtlasSource is given by hand. nil panics.
func (w *Write) SetAtlas(atlas render.AtlasSource) {
	p := w.p
	if atlas == nil {
		panic("board: SetAtlas needs the game's atlas of the cell kinds (board.Plugin.NewAtlas)")
	}
	p.atlas = atlas
	p.renderer.SetAtlas(atlas)
	p.renderer.Shade(nil)
	if ba, ok := atlas.(*Atlas); ok {
		p.renderer.Shade(ba.shaded)
	}
	line, holds := DefaultGridLine(p.board.Grid), float32(DefaultHoldWidth)
	if ba, ok := atlas.(*Atlas); ok {
		line, holds = ba.grid, ba.holds
	}
	p.Res.Render = p.renderer.State()
	p.Res.Render.ShowGridLines = p.Res.Settings.Grid
	p.Res.Render.GridLine = line.Color
	p.Res.Render.GridWidth = line.Width
	p.holdsView.Width(holds)
}

// ShowGrid draws the grid over the board, or not, as the Grid command switches it
// (Settings.Grid); shown from the start.
func (w *Write) ShowGrid(shown bool) {
	p := w.p
	p.Res.Settings.Grid = shown
	if p.Res.Render != nil {
		p.Res.Render.ShowGridLines = shown
	}
}

// ShowHolds shows the cells the occupancy holds, each an outline in its holder's colour, or
// hides them (Settings.Holds).
func (w *Write) ShowHolds(shown bool) {
	w.p.Res.Settings.Holds = shown
	w.p.holdsView.Show(shown)
}
