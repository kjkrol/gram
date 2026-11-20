package buildings

import (
	"slices"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/internal/f32"
	"github.com/kjkrol/gram/plugins/board/cell"
)

// Read reads the buildings standing: where they hold the cameras.
type Read struct{ p *Plugin }

// Read reads the buildings standing.
func (p *Plugin) Read(int) *Read { return &p.read }

// Free reports whether no building's solid fills the point (x, y, z), every house shut: a part's
// walls on its rectangle from its foot and its roof over them, but for a house's room within its
// walls, from its floor to its ceiling, and its door.
func (r *Read) Free(x, y, z float32) bool { return r.p.cage.free(r.p, 0, x, y, z) }

// FreeTo is Free as a camera fastened how sees it: a house open before it, a unit standing in its
// room as the settings say, is free whole.
func (r *Read) FreeTo(how camera.How, x, y, z float32) bool {
	return r.p.cage.free(r.p, how, x, y, z)
}

// Span is the lowest foot and the highest roof of the buildings.
func (r *Read) Span() (low, high float32) { return r.p.cage.low, r.p.cage.high }

// Grain is how far a line of sight may step near a house with a room, and how thin its walls
// are; zero elsewhere.
func (r *Read) Grain(x, y float32) (step, thin float32) {
	c := &r.p.cage
	if !c.rooms {
		return 0, 0
	}
	brd := r.p.board.Read(0).Board()
	at, ok := brd.CellAt(geom.NewVec(x, y))
	if !ok {
		return 0, 0
	}
	for _, i := range c.cells[at] {
		if room := c.parts[i].room; room >= 0 {
			band := c.halls[room].band
			if thin == 0 || band < thin {
				step, thin = band/2, band
			}
		}
	}
	return step, thin
}

// opens reports whether a house a unit stands in opens round its room before a camera fastened
// how, as settings say: the tactical view's loose or centred on a unit, one behind a unit, never
// one inside a unit
func opens(settings Settings, how camera.How) bool {
	switch {
	case how&camera.Inside != 0:
		return false
	case how&camera.Behind != 0:
		return !settings.ShutBehind
	}
	return !settings.Shut
}

// cage is the buildings as they hold the cameras, laid anew as they are drawn where their lines
// changed: their parts by the board's cells they stand over, the rooms in them and how low and how
// high they reach
type cage struct {
	stale     bool
	parts     []held
	halls     []hall
	cells     map[cell.ID][]int32
	low, high float32
	rooms     bool
}

// held is a part as it holds the cameras: its middle, the way its length runs and half its
// length and width, its foot, its eaves, its roof's rise and half its ridge, and its building's
// room in the cage's halls (-1 none)
type held struct {
	mid, dir, half    geom.Vec
	foot, eaves, rise float32
	ridge             float32
	room              int32
}

// hall is a house's room as it holds the cameras: its floor and ceiling, how thick its walls and
// its ceiling are, its door's middle along the part and across it, half its width and its height,
// and the cell its handle is found by
type hall struct {
	floor, ceiling, band float32
	door                 geom.Vec
	doorHalf, doorTall   float32
	at                   cell.ID
}

// built is c laid anew where the buildings' lines changed, once the ground is known; as the
// buildings are laid to be drawn, so a camera before then finds them on the ground they are drawn
// on
func (c *cage) built(p *Plugin) *cage {
	if !c.stale {
		return c
	}
	heights := p.board.Read(0).Heights()
	if heights == nil {
		return c
	}
	c.stale = false
	c.parts, c.halls, c.rooms = c.parts[:0], c.halls[:0], false
	c.cells = map[cell.ID][]int32{}
	c.low, c.high = f32.Inf(1), f32.Inf(-1)
	brd := p.board.Read(0).Board()
	sectors := make([]int, 0, len(p.Res.Stood))
	for k := range p.Res.Stood {
		sectors = append(sectors, k)
	}
	slices.Sort(sectors)
	for _, k := range sectors {
		for _, b := range p.Res.Stood[k] {
			spec := p.types.specs[b.Kind]
			room := int32(-1)
			at := spec.of(b.Row)
			var h *house
			if b.Row.Door != [2]float32{} {
				if got, err := p.houseOf(spec, b.Row, heights.At); err == nil {
					h = &got
				}
			}
			if h != nil {
				room = int32(len(c.halls))
				c.halls = append(c.halls, hallOf(spec, at, *h))
				c.rooms = true
			}
			for _, part := range heldOf(spec, at, heights.At, h, room) {
				c.parts = append(c.parts, part)
				c.low, c.high = min(c.low, part.foot), max(c.high, part.eaves+part.rise)
				c.index(brd.CellAt, brd.Grid.CellSpan(), int32(len(c.parts)-1))
			}
		}
	}
	if len(c.parts) == 0 {
		c.low, c.high = 0, 0
	}
	return c
}

// index notes the part i under every cell its rectangle reaches over, found by cellAt, the
// cells span wide
func (c *cage) index(cellAt func(geom.Vec) (cell.ID, bool), span float32, i int32) {
	p := &c.parts[i]
	side := geom.NewVec(-p.dir.Y, p.dir.X)
	ux, uy := f32.Abs(p.dir.X), f32.Abs(p.dir.Y)
	hx := ux*p.half.X + uy*p.half.Y
	hy := uy*p.half.X + ux*p.half.Y
	step := span / 2
	var seen []cell.ID
	for x := -hx; ; x += step {
		x = min(x, hx)
		for y := -hy; ; y += step {
			y = min(y, hy)
			at := geom.NewVec(p.mid.X+x, p.mid.Y+y)
			local := geom.NewVec(x*p.dir.X+y*p.dir.Y, x*side.X+y*side.Y)
			if f32.Abs(local.X) <= p.half.X+step && f32.Abs(local.Y) <= p.half.Y+step {
				if id, ok := cellAt(at); ok && !slices.Contains(seen, id) {
					seen = append(seen, id)
					c.cells[id] = append(c.cells[id], i)
				}
			}
			if y >= hy {
				break
			}
		}
		if x >= hx {
			break
		}
	}
}

// heldOf are the parts of the building b of spec standing on the ground at as they hold the
// cameras, as solidsOf stands them; the body's room in the cage's halls room (-1 none)
func heldOf(spec Spec, b standing, at func(geom.Vec) float32, h *house, room int32) []held {
	solids := solidsOf(spec, b, at, h)
	out := make([]held, 0, len(solids))
	for k, s := range solids {
		p := held{
			mid:   s.mid,
			dir:   b.dir,
			half:  s.half,
			foot:  s.foot,
			eaves: s.eaves,
			rise:  s.rise,
			ridge: s.ridge,
			room:  room,
		}
		if k > 0 {
			// the room is the body's: another part holds the eye whole, open as its house is
			p.room = -2 - room
		}
		out = append(out, p)
	}
	return out
}

// hallOf is the room h of the building b of spec as it holds the cameras
func hallOf(spec Spec, b standing, h house) hall {
	door := h.opening(spec)
	q := geom.NewVec(door[0]-b.mid.X, door[1]-b.mid.Y)
	side := geom.NewVec(-b.dir.Y, b.dir.X)
	return hall{
		floor:    h.floor,
		ceiling:  h.floor + spec.Storey,
		band:     spec.wall(),
		door:     geom.NewVec(q.X*b.dir.X+q.Y*b.dir.Y, q.X*side.X+q.Y*side.Y),
		doorHalf: door[2],
		doorTall: door[3],
		at:       h.door,
	}
}

// free reports whether no building's solid fills (x, y, z) as a camera fastened how sees it, a
// zero how seeing every house shut
func (c *cage) free(p *Plugin, how camera.How, x, y, z float32) bool {
	if len(c.parts) == 0 || z >= c.high || z < c.low {
		return true
	}
	at, ok := p.board.Read(0).Board().CellAt(geom.NewVec(x, y))
	if !ok {
		return true
	}
	return c.freeIn(at, x, y, z, func(room int32) bool { return how != 0 && c.open(p, how, room) })
}

// freeIn reports whether no part noted under the cell at fills (x, y, z), the house of a room
// open says is open free whole
func (c *cage) freeIn(at cell.ID, x, y, z float32, open func(room int32) bool) bool {
	for _, i := range c.cells[at] {
		part := &c.parts[i]
		side := geom.NewVec(-part.dir.Y, part.dir.X)
		q := geom.NewVec(x-part.mid.X, y-part.mid.Y)
		along, across := q.X*part.dir.X+q.Y*part.dir.Y, q.X*side.X+q.Y*side.Y
		if f32.Abs(along) > part.half.X || f32.Abs(across) > part.half.Y || z < part.foot ||
			z >= part.top(along, across) {
			continue
		}
		room := part.room
		if room < -1 {
			room = -2 - room
		}
		if room < 0 {
			return false
		}
		if open(room) {
			continue
		}
		if part.room >= 0 && c.halls[room].holds(part, along, across, z) {
			continue
		}
		return false
	}
	return true
}

// open reports whether the house of the room room is open before a camera fastened how
func (c *cage) open(p *Plugin, how camera.How, room int32) bool {
	if p.rooms == nil || !opens(p.Res.Settings, how) {
		return false
	}
	read := p.rooms.Read(0)
	id, ok := read.At(c.halls[room].at, RoomStorey)
	return ok && read.Bared(id)
}

// top is how high the part's solid stands at along and across off its middle: its eaves and its
// roof's slopes over them, a hipped one's ends sloping too
func (p *held) top(along, across float32) float32 {
	if p.rise <= 0 {
		return p.eaves
	}
	f := 1 - f32.Abs(across)/p.half.Y
	if p.ridge < p.half.X-1e-3 {
		f = min(f, (p.half.X-f32.Abs(along))/(p.half.X-p.ridge))
	}
	return p.eaves + p.rise*max(f, 0)
}

// holds reports whether the room h lets an eye stand at along and across off the middle of its
// part, at z: within its walls, from its floor to under its ceiling, or in its door
func (h *hall) holds(part *held, along, across, z float32) bool {
	if z < h.floor {
		return false
	}
	in := f32.Abs(along) < part.half.X-h.band && f32.Abs(across) < part.half.Y-h.band
	if in && z < h.ceiling-h.band {
		return true
	}
	if z >= h.floor+h.doorTall {
		return false
	}
	// the door's wall: an end where the door lies at the part's end, a side otherwise
	if f32.Abs(h.door.X)/part.half.X > f32.Abs(h.door.Y)/part.half.Y {
		return f32.Abs(across-h.door.Y) < h.doorHalf &&
			f32.Abs(along) >= part.half.X-h.band-1e-3 && along*h.door.X > 0
	}
	return f32.Abs(along-h.door.X) < h.doorHalf &&
		f32.Abs(across) >= part.half.Y-h.band-1e-3 && across*h.door.Y > 0
}
