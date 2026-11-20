package buildings

import (
	"fmt"
	"math"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/aabbworld/plane"
	"github.com/kjkrol/ceremony/comp"
	"github.com/kjkrol/gram/entity/kind"
	"github.com/kjkrol/gram/internal/f32"
	"github.com/kjkrol/gram/plugins/board"
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/board/grid"
	"github.com/kjkrol/gram/plugins/interiors"
	"github.com/kjkrol/gram/plugins/light"
	"github.com/kjkrol/gram/plugins/vision"
	"github.com/kjkrol/gram/plugins/world"
)

// RoomStorey is the storey of the board's cells a building's room is laid on: the last, the
// first left to bridges and those between to what is dug under the ground.
const RoomStorey = cell.Storeys - 1

// a room's piece on a cell: its floor as high over the cell's level as the room's, its links
// into the room's other cells and out by its door
type roomRow struct {
	At     cell.ID
	Links  cell.Links
	Height float32
}

// a house's room whole: the box over its cells from its corner At, its floor, and the light that
// gets into it by its windows and door
type houseRow struct {
	At, Size geom.Vec
	Floor    float32
	Shelter  light.Shelter
}

func roomName(name string) string  { return name + ".room" }
func houseName(name string) string { return name + ".house" }

// roomsOf defines the kinds of the room of a building of the kind name with a door: its pieces,
// a floor a cell as high as a storey, and their handle
func (t *Types) roomsOf(name string, spec Spec) {
	w, brd := t.p.worldPlugin, t.p.board
	span := brd.Read(0).Board().Grid.CellSpan()
	door := spec.Door
	if door.Height <= 0 || door.Height > spec.Storey {
		door.Height = spec.Storey
	}
	pieces := board.NewEntities(brd, board.Shape{Size: span, Height: spec.Storey},
		func(r roomRow) cell.ID { return r.At })
	pieces.Define(roomName(name),
		comp.Load(func(r roomRow) cell.Floor { return spec.roomFloor(r, door) }),
		comp.Const(light.Shadow{Cast: light.NoShadow}),
		// the building cuts sight, its room nothing more
		comp.Const(vision.Transparency{Value: 1}),
		// the plugin draws it with its building
		comp.Const(world.Unseen{}),
	)
	kind.Define[houseRow](w.Kinds(), houseName(name), kind.Spec{
		comp.Load(func(r houseRow) world.Position {
			return world.Position{AABB: plane.NewAABB(r.At, r.Size.X, r.Size.Y)}
		}),
		comp.Const(world.Velocity{}),
		comp.Load(func(r houseRow) world.Z {
			return world.Z{Altitude: r.Floor, Height: spec.Storey}
		}),
		comp.Const(interiors.Room{}),
		comp.Load(func(r houseRow) light.Shelter { return r.Shelter }),
		comp.Const(light.Shadow{Cast: light.NoShadow}),
		comp.Const(vision.Transparency{Value: 1}),
		comp.Const(world.Fixed{}),
		comp.Const(world.Whole{}),
	})
}

// roomFloor is the floor of the piece r of the room of a building of the kind s, its door door:
// its walls as thick as the kind's, so they stop a body where they are drawn
func (s Spec) roomFloor(r roomRow, door cell.Gap) cell.Floor {
	return cell.Floor{
		Allows:   cell.Land,
		Height:   r.Height,
		Pace:     1,
		Graded:   true,
		Links:    r.Links,
		Storey:   RoomStorey,
		Clear:    s.Storey,
		Walls:    s.wall(),
		Standing: true,
		Door:     door,
	}
}

// house is the room of the building b of the kind name, its line row, on the board: its cells,
// the edge its door opens across and its floor, as high as the ground in the middle of that edge
type house struct {
	spec   Spec
	at     standing
	cells  []cell.ID
	door   cell.ID
	way    int
	doorAt geom.Vec // the middle of the edge it opens across
	floor  float32
}

// opening is the door of h of a building of spec, as the shaders take it: its middle, half its
// width and its height
func (h house) opening(spec Spec) [4]float32 {
	tall := spec.Door.Height
	if tall <= 0 || tall > spec.Storey {
		tall = spec.Storey
	}
	return [4]float32{h.doorAt.X, h.doorAt.Y, spec.Door.Width / 2, tall}
}

// houseOf is where the building of the kind spec on row stands its room on the ground ground
// gives, or why it may not: square to the board's square grid on whole cells, its door in the
// middle of one of its outer edges
func (p *Plugin) houseOf(spec Spec, row Row, ground func(geom.Vec) float32) (house, error) {
	var h house
	brd := p.board.Read(0).Board()
	g := brd.Grid
	shape, ok := grid.ShapeOf(g)
	switch {
	case spec.Door.Width <= 0:
		return h, fmt.Errorf("its kind has no door (Spec.Door)")
	case !ok || shape.Hex:
		return h, fmt.Errorf("a building with a door stands on a square grid")
	}
	turn := float32(math.Mod(float64(row.Turn), 90))
	if f32.Abs(turn) > 1e-3 && f32.Abs(f32.Abs(turn)-90) > 1e-3 {
		return h, fmt.Errorf("a building with a door stands square to the grid, turned %v",
			row.Turn)
	}
	b := spec.of(row)
	h.spec, h.at = spec, b
	hx, hy := b.reach()
	size := shape.Cell
	x0, y0 := (b.mid.X-hx)/size, (b.mid.Y-hy)/size
	x1, y1 := (b.mid.X+hx)/size, (b.mid.Y+hy)/size
	for _, v := range [4]float32{x0, y0, x1, y1} {
		if f32.Abs(v-f32.Round(v)) > 1e-3 {
			return h, fmt.Errorf("a building with a door stands on whole cells: it spans %v to "+
				"%v cells across", [2]float32{x0, y0}, [2]float32{x1, y1})
		}
	}
	at := geom.NewVec(row.Door[0], row.Door[1])
	best := f32.Inf(1)
	for y := int(f32.Round(y0)); y < int(f32.Round(y1)); y++ {
		for x := int(f32.Round(x0)); x < int(f32.Round(x1)); x++ {
			c := g.At(uint32(x), uint32(y))
			h.cells = append(h.cells, c)
			m := g.CellCenter(c)
			for way := range g.Directions() {
				dx, dy := g.Offset(way)
				out := geom.NewVec(float32(x+dx)+0.5, float32(y+dy)+0.5)
				if g.Corner(way) || out.X > x0 && out.X < x1 && out.Y > y0 && out.Y < y1 {
					continue
				}
				edge := geom.NewVec(m.X+float32(dx)*size/2, m.Y+float32(dy)*size/2)
				if d := f32.Hypot(edge.X-at.X, edge.Y-at.Y); d < best {
					best, h.door, h.way = d, c, way
				}
			}
		}
	}
	if best > size/2 {
		return h, fmt.Errorf("its door at %v lies on none of its walls", at)
	}
	m := g.CellCenter(h.door)
	dx, dy := g.Offset(h.way)
	h.doorAt = geom.NewVec(m.X+float32(dx)*size/2, m.Y+float32(dy)*size/2)
	h.floor = ground(h.doorAt)
	return h, nil
}

// openings are the room h of the building b of spec as the sky's light gets into it: by the
// windows of its storey where the ground outside lies under their sills, as its shaders draw
// them (paneOf), and by its door; as much of the sky's and of the sun's thrown round as they
// light it with (interior.wgsl), and its lamp
func openings(spec Spec, b standing, h house, ground func(geom.Vec) float32) light.Shelter {
	room := light.Shelter{Sky: indoorSky, Bounce: indoorBounce, Lamp: roomLamp}
	along := b.dir
	side := geom.NewVec(-b.dir.Y, b.dir.X)
	hx, hy := b.size.X/2, b.size.Y/2
	sill, head := paneFrom*spec.Storey, paneTo*spec.Storey
	door := h.opening(spec)
	if spec.Bay > 0 && !spec.Parts[0].Blind {
		for _, w := range [4]struct {
			mid, run, out geom.Vec
			wide          float32
		}{
			{off(b.mid, along, side, 0, hy), along, side, hx},
			{off(b.mid, along, side, 0, -hy), along, scaled(side, -1), hx},
			{off(b.mid, along, side, hx, 0), side, along, hy},
			{off(b.mid, along, side, -hx, 0), side, scaled(along, -1), hy},
		} {
			bays := max(f32.Floor(2*w.wide/spec.Bay), 1)
			step := 2 * w.wide / bays
			for k := float32(0); k < bays; k++ {
				at := -w.wide + (k+0.5)*step
				mid := off(w.mid, w.run, w.out, at, 0)
				half := paneWide * step / 2
				if ground(off(mid, w.run, w.out, 0, 0.5)) >= h.floor+sill ||
					byDoor(door, w.mid, w.run, w.out, at, half, step) {
					continue
				}
				room.Opens(light.Opening{A: off(mid, w.run, w.out, -half, 0),
					B: off(mid, w.run, w.out, half, 0), Bottom: sill, Top: head})
			}
		}
	}
	run := geom.NewVec(-(door[1] - b.mid.Y), door[0]-b.mid.X)
	if f32.Abs(run.X) > f32.Abs(run.Y) {
		run = geom.NewVec(f32.Copysign(1, run.X), 0)
	} else {
		run = geom.NewVec(0, f32.Copysign(1, run.Y))
	}
	at := geom.NewVec(door[0], door[1])
	room.Opens(light.Opening{A: at.Sub(scaled(run, door[2])), B: at.Add(scaled(run, door[2])),
		Top: door[3]})
	return room
}

// byDoor reports whether the pane at along the wall of middle mid, running run and facing out,
// half wide either side in a bay step long, comes nearer the door than half the pier the panes
// keep apart, as the shaders leave such a window out (byDoor)
func byDoor(door [4]float32, mid, run, out geom.Vec, at, half, step float32) bool {
	q := geom.NewVec(door[0]-mid.X, door[1]-mid.Y)
	if door[2] <= 0 || f32.Abs(q.X*out.X+q.Y*out.Y) > 0.5 {
		return false
	}
	return f32.Abs(at-(q.X*run.X+q.Y*run.Y))-half-door[2] < (1-paneWide)*step/2
}

func scaled(v geom.Vec, k float32) geom.Vec { return geom.NewVec(v.X*k, v.Y*k) }

// lay seeds the room h of the building of the kind name on the ground ground gives: a floor on
// each of its cells as high as its door's ground, over the cell's level, the mean of its
// corners', into its other cells along the grid and out by its door
func (p *Plugin) lay(name string, h house, ground func(geom.Vec) float32) {
	brd := p.board.Read(0).Board()
	g := brd.Grid
	in := make(map[cell.ID]bool, len(h.cells))
	for _, c := range h.cells {
		in[c] = true
	}
	var pieces []kind.Entry
	half := g.CellSpan() / 2
	lo, hi := g.CellCenter(h.cells[0]), g.CellCenter(h.cells[0])
	for _, c := range h.cells {
		var links cell.Links
		for way := range g.Directions() {
			if n, ok := g.Toward(c, way); ok && in[n] && !g.Corner(way) {
				links |= 1 << way
			}
		}
		if c == h.door {
			links |= 1 << h.way
		}
		m := g.CellCenter(c)
		level := float32(0)
		for _, corner := range [4][2]float32{{-1, -1}, {1, -1}, {-1, 1}, {1, 1}} {
			level += ground(geom.NewVec(m.X+corner[0]*half, m.Y+corner[1]*half)) / 4
		}
		pieces = append(pieces, kind.EntryOf(roomName(name),
			roomRow{At: c, Links: links, Height: h.floor - level}))
		lo = geom.NewVec(min(lo.X, m.X), min(lo.Y, m.Y))
		hi = geom.NewVec(max(hi.X, m.X), max(hi.Y, m.Y))
	}
	whole := houseRow{
		At:      geom.NewVec(lo.X-half, lo.Y-half),
		Size:    geom.NewVec(hi.X-lo.X+2*half, hi.Y-lo.Y+2*half),
		Floor:   h.floor,
		Shelter: openings(h.spec, h.at, h, ground),
	}
	p.worldPlugin.Write().SeedWhole(kind.EntryOf(houseName(name), whole), pieces...)
}
