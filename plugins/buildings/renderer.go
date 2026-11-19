package buildings

import (
	"embed"
	"image/color"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/lod"
	"github.com/kjkrol/gram/plugin"
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/render"
	"github.com/kjkrol/uid"
)

//go:embed shaders
var shaders embed.FS

var solidUniforms = []render.Uniform{
	{Name: "SolidViewProj", Size: 16},
	{Name: "SolidEye", Size: 3},
	{Name: "SolidBend", Size: 1},
	{Name: "SolidHaze", Size: 4},
	{Name: "SolidHazeWay", Size: 3},
	{Name: "SolidSight", Size: 4},
	{Name: "SolidFar", Size: 1},
	{Name: "SolidPane", Size: 4},
	{Name: "SolidUncovering", Size: 1},
	{Name: "SolidUncoveredIn", Size: 4},
	{Name: "SolidIndoor", Size: 4},
	{Name: "SolidLamp", Size: 3},
}

var (
	solidShader = render.NewMeshShaderWith("buildings solids",
		render.Files(shaders, "shaders/common.wgsl", "shaders/solid.wgsl"),
		solidUniforms).Instanced(6)
	innerShader = render.NewMeshShaderWith("buildings rooms",
		render.Files(shaders, "shaders/common.wgsl", "shaders/interior.wgsl"),
		solidUniforms).Instanced(6)
	baredShader = render.NewMeshShaderWith("buildings rooms bared",
		render.Files(shaders, "shaders/common.wgsl", "shaders/bared.wgsl"),
		solidUniforms).Instanced(6)
)

// a part's floats and its vertices: the two walls seen, two slopes and two ends, and far off
// the two walls and a lid; a room's floats and its vertices, its floor and two walls, bared
// alike
const (
	solidFloats   = 24
	solidVertices = 30
	farVertices   = 18
	innerFloats   = 24
	innerVertices = 18
)

// how much of a bay and of a storey a window's pane takes, from the storey's foot to its head,
// and how many of the windows glow after dusk
const (
	paneWide = 0.44
	paneFrom = 0.32
	paneTo   = 0.8
	litShare = 0.3
)

// how much of the sky's light gets into a room and how much of the sun's it throws round from
// where it falls in, and the light its lamp burns with after dusk (straight colour times
// strength)
const (
	indoorSky    = 1.1
	indoorBounce = 0.5
)

var roomLamp = [3]float32{0.9, 0.74, 0.45}

// solids draws the buildings of the level's file, a part an instance, each sector's as its
// level in the camera's view says: whole at the close and standard scales, their bodies alone
// and without windows at the strategic one
type solids struct {
	p *Plugin

	// the buildings laid once the ground is known, and those laid since, drawn from the swap
	shown *stock
	next  *stock
	stale bool

	// whether the ground is hazed, as the ticks left it and as the frame drawn has it
	hazes bool
	hazed bool
	// the rooms of the frame drawn, frames[frame], and of the one published, the other
	frames [2]rooms
	frame  int

	// the sectors the camera composed last sees, and the ranges of those in the layer drawn
	sectors []seen
	ranges  []render.InstanceRange
	d       drawer
}

// stock is the parts of every sector's buildings kept on the GPU: where each sector's lie,
// whole and coarse, and how low and how high its buildings reach; and the insides of the rooms,
// kept apart, each sector's, with the rooms in the order kept
type stock struct {
	kept   *render.Instances
	fine   map[int]render.InstanceRange
	coarse map[int]render.InstanceRange
	spans  map[int][2]float32
	inside *render.Instances
	inner  map[int]render.InstanceRange
	rooms  []laidRoom
}

func (s *stock) release() {
	s.kept.Release()
	if s.inside != nil {
		s.inside.Release()
	}
}

// a building's room as laid: the sector it stands in, the cell its door opens from, the instance
// of its inside, where it is kept among the insides and its lamp's strength as laid
type laidRoom struct {
	sector int
	cell   cell.ID
	inner  [innerFloats]float32
	at     int
	lamp   float32
}

// rooms is a frame's rooms: those bared, by their place among the stock's
type rooms struct{ bared []int }

// a sector's parts as a camera sees them and the insides of its rooms, the box they stand in and
// the sector
type seen struct {
	box      geom.AABB
	low, top float32
	parts    render.InstanceRange
	inner    render.InstanceRange
	sector   int
	far      bool
}

// how far past its sector's edge a building may stand, world units: half the widest
const overhang = 128

var (
	_ render.Direct    = (*solids)(nil)
	_ plugin.Publisher = (*solids)(nil)
)

// Publish lays the parts anew where the level's buildings changed or a room's lamp burns other
// than it was laid with, and finds the rooms an uncovered entity stands in.
func (r *solids) Publish() {
	r.hazes = r.p.ground != nil && r.p.ground.Hazes()
	r.lay()
	st := r.next
	if st == nil {
		st = r.shown
	}
	fr := &r.frames[r.frame^1]
	fr.bared = fr.bared[:0]
	if st == nil || r.p.rooms == nil {
		return
	}
	read := r.p.rooms.Read(0)
	for k, room := range st.rooms {
		id, ok := read.At(room.cell, RoomStorey)
		if r.lampOf(id, ok) != room.lamp {
			r.stale = true
		}
		if ok && read.Bared(id) {
			fr.bared = append(fr.bared, k)
		}
	}
}

// lampOf is how strongly the lamp of the room id burns, its shelter's against the plugin's own,
// 1 where the room is not known (ok)
func (r *solids) lampOf(id uid.UID64, ok bool) float32 {
	if !ok || r.p.rooms == nil {
		return 1
	}
	l := r.p.rooms.Read(0).Lamp(id)
	return (l[0] + l[1] + l[2]) / (roomLamp[0] + roomLamp[1] + roomLamp[2])
}

// lay lays the parts anew where the level's buildings changed, once the ground is known
func (r *solids) lay() {
	if !r.stale {
		return
	}
	heights := r.p.board.Read(0).Heights()
	if heights == nil {
		return
	}
	r.stale = false
	fine, coarse, insides := map[int][]float32{}, map[int][]float32{}, map[int][]float32{}
	spans := map[int][2]float32{}
	var rooms []laidRoom
	for k, all := range r.p.Res.Stood {
		for n, b := range all {
			spec := r.p.types.specs[b.Kind]
			seed := float32((k*7919+n*104729)%1000) / 1000
			var h *house
			if b.Row.Door != [2]float32{} {
				if got, err := r.p.houseOf(spec, b.Row, heights.At); err == nil {
					h = &got
				}
			}
			at := spec.of(b.Row)
			fine[k] = lay(fine[k], spec, at, heights.At, seed, true, h)
			coarse[k] = lay(coarse[k], spec, at, heights.At, seed, false, h)
			if h != nil {
				room := laidRoom{sector: k, cell: h.door, inner: inside(spec, at, *h),
					at: len(insides[k]) / innerFloats}
				if r.p.rooms != nil {
					id, ok := r.p.rooms.Read(0).At(h.door, RoomStorey)
					room.lamp = r.lampOf(id, ok)
				} else {
					room.lamp = 1
				}
				room.inner[15] = room.lamp
				rooms = append(rooms, room)
				insides[k] = append(insides[k], room.inner[:]...)
			}
		}
		parts := fine[k]
		span := [2]float32{parts[2], parts[3] + parts[8]}
		for i := 0; i < len(parts); i += solidFloats {
			span[0] = min(span[0], parts[i+2])
			span[1] = max(span[1], parts[i+3]+parts[i+8])
		}
		spans[k] = span
	}
	if r.next != nil {
		r.next.release()
	}
	kept, at := render.PackInstances(solidFloats, fine, coarse)
	r.next = &stock{kept: kept, fine: at[0], coarse: at[1], spans: spans, rooms: rooms}
	if len(rooms) > 0 {
		inside, in := render.PackInstances(innerFloats, insides)
		r.next.inside, r.next.inner = inside, in[0]
		for i := range rooms {
			rooms[i].at += in[0][rooms[i].sector].First
		}
	}
}

// Swap has the buildings laid last drawn.
func (r *solids) Swap() {
	r.hazed = r.hazes
	r.frame ^= 1
	if r.next == nil {
		return
	}
	if r.shown != nil {
		r.shown.release()
	}
	r.shown, r.next = r.next, nil
}

// lay appends to dst the parts of the building b of spec standing on the ground at: its foot at
// the lowest of the ground under its body's corners, its storeys counted from the highest, or
// from the floor of its room where it has one (h), its body's door open; whole, with its windows
// and its fine parts
func lay(dst []float32, spec Spec, b standing, at func(geom.Vec) float32, seed float32,
	whole bool, h *house) []float32 {
	side := geom.NewVec(-b.dir.Y, b.dir.X)
	foot, ground := at(b.mid), at(b.mid)
	for _, sx := range [2]float32{-1, 1} {
		for _, sy := range [2]float32{-1, 1} {
			h := at(off(b.mid, b.dir, side, sx*b.size.X/2, sy*b.size.Y/2))
			foot, ground = min(foot, h), max(ground, h)
		}
	}
	if h != nil {
		foot, ground = min(foot, h.floor), h.floor
	}
	for k, part := range spec.Parts {
		if part.Fine && !whole {
			continue
		}
		size := part.Size
		if size.X <= 0 || size.Y <= 0 {
			size = b.size
		}
		mid := off(b.mid, b.dir, side, part.Off.X*b.size.X/2, part.Off.Y*b.size.Y/2)
		walls := part.Height
		if walls <= 0 {
			walls = b.walls + part.Above
		}
		rise := part.Pitch * size.Y / 2
		ridge := float32(0)
		switch part.Roof {
		case Flat:
			rise = 0
		case Gable:
			ridge = size.X / 2
		case Hip:
			ridge = max(size.X-size.Y, 0) / 2
		}
		bay := spec.Bay
		if part.Blind || !whole {
			bay = 0
		}
		wr, wg, wb := straight(part.Walls, spec.Walls)
		tr, tg, tb := straight(part.Tiles, spec.Tiles)
		var door [4]float32
		if h != nil && k == 0 && whole {
			door = h.opening(spec)
		}
		dst = append(dst,
			mid.X, mid.Y, foot, ground+walls,
			size.X/2, size.Y/2, b.dir.X, b.dir.Y,
			rise, ridge, spec.Storey, bay,
			wr, wg, wb, seed,
			tr, tg, tb, ground,
			door[0], door[1], door[2], door[3])
	}
	return dst
}

// inside is the instance of the inside of the room h of the building b of spec: its floor and
// ceiling, its body's rectangle, its door and windows, the floor's colour and the walls'
func inside(spec Spec, b standing, h house) [innerFloats]float32 {
	body := spec.Parts[0]
	walls := body.Height
	if walls <= 0 {
		walls = b.walls + body.Above
	}
	door := h.opening(spec)
	fr, fg, fb := straight(spec.Floor, woodFloor)
	wr, wg, wb := straight(body.Walls, spec.Walls)
	// plastered: the walls' colour washed half to white inside
	wr, wg, wb = (wr+1)/2, (wg+1)/2, (wb+1)/2
	return [innerFloats]float32{
		b.mid.X, b.mid.Y, h.floor, h.floor + spec.Storey,
		b.size.X / 2, b.size.Y / 2, b.dir.X, b.dir.Y,
		door[0], door[1], door[2], door[3],
		spec.Storey, spec.Bay, walls, 0,
		fr, fg, fb, 0,
		wr, wg, wb, 0,
	}
}

// the floor of a room whose kind says none: boards
var woodFloor = color.RGBA{R: 140, G: 104, B: 72, A: 255}

// off is the point along units along and across units across off mid
func off(mid, along, across geom.Vec, a, c float32) geom.Vec {
	return geom.NewVec(mid.X+along.X*a+across.X*c, mid.Y+along.Y*a+across.Y*c)
}

// straight is c's colour, or other's where c is none, as a shader takes it
func straight(c, other color.RGBA) (r, g, b float32) {
	if c == (color.RGBA{}) {
		c = other
	}
	return float32(c.R) / 255, float32(c.G) / 255, float32(c.B) / 255
}

// Compose gathers the sectors cam sees at the strategic scale or coarser with their parts,
// drawn with the ground: their bodies alone, without windows.
func (r *solids) Compose(f *render.Frame, cam camera.Camera) {
	r.sectors = r.sectors[:0]
	st := r.shown
	if st == nil || len(st.coarse) == 0 {
		return
	}
	view := r.p.worldPlugin.Read(0).ViewFor(cam)
	if view == nil || view.Grid() == nil {
		return
	}
	view.Sectors(func(i int, box geom.AABB, level lod.Level) {
		if parts := st.coarse[i]; level >= lod.Strategic && parts.Count > 0 {
			r.sectors = append(r.sectors, seen{box: box, low: st.spans[i][0],
				top: st.spans[i][1], parts: parts, far: true})
		}
	})
	if len(r.sectors) > 0 {
		f.DrawOpaque()
	}
}

// Draw draws the parts gathered into the target through cam under the frame's uniforms u: of a
// picture drawn in layers, the sectors in the layer's window and not wholly in the next one's.
func (r *solids) Draw(t render.Target, cam camera.Camera, u render.Uniforms) {
	if t.Screen == nil || t.Depth == nil || len(r.sectors) == 0 {
		return
	}
	tr, w, h, ok := transformOf(cam)
	if !ok {
		return
	}
	r.ranges = shownOf(r.ranges[:0], r.sectors, cam, &tr, w, h, false)
	if len(r.ranges) == 0 {
		return
	}
	r.d.begin(u)
	r.p.scene(&r.d, cam, &tr, r.hazed)
	r.d.set("SolidFar", 1)
	r.d.opts.Depth, r.d.opts.WriteDepth = t.Depth, true
	r.d.opts.Kept, r.d.opts.Vertices, r.d.opts.Spans = r.shown.kept, farVertices, r.ranges
	t.Screen.DrawMesh(nil, solidShader, &r.d.opts)
}

// transformOf is how cam lays the world on its viewport w by h
func transformOf(cam camera.Camera) (tr camera.Transform, w, h float32, ok bool) {
	rays, ok := cam.(camera.Rays)
	if !ok {
		return tr, 0, 0, false
	}
	field, ok := rays.Rays()
	if !ok {
		return tr, 0, 0, false
	}
	w, h = cam.Viewport()
	tr, ok = camera.SceneTransform(field, w, h)
	return tr, w, h, ok
}

// shownOf appends to dst the parts of the sectors tr shows of a picture drawn in layers, or the
// insides of their rooms: in the layer's window and not wholly in the next one's
func shownOf(dst []render.InstanceRange, sectors []seen, cam camera.Camera,
	tr *camera.Transform, w, h float32, insides bool) []render.InstanceRange {
	hole, holed := camera.Inner(cam)
	for _, s := range sectors {
		if s.box.BottomRight != s.box.TopLeft &&
			!tr.Shows(s.box, overhang, s.low, s.top, w, h, hole, holed) {
			continue
		}
		if !insides {
			dst = render.JoinSpan(dst, s.parts)
		} else if s.inner.Count > 0 {
			dst = render.JoinSpan(dst, s.inner)
		}
	}
	return dst
}

// scene sets the uniforms every program of the plugin draws through cam by: its transform tr,
// the air and the sight, the panes and the rooms' light; nothing gives way
func (p *Plugin) scene(d *drawer, cam camera.Camera, tr *camera.Transform, hazed bool) {
	d.set("SolidViewProj", tr.M[:]...)
	d.set("SolidEye", tr.Eye[:]...)
	d.set("SolidBend", tr.Bend)
	var seeing, up float32
	var way [3]float32
	if p.sky != nil {
		seeing, up, way = p.sky.Air().Seen(cam, hazed)
	}
	d.set("SolidHaze", seeing, up, 0, 0)
	d.set("SolidHazeWay", way[:]...)
	var fog [4]float32
	if view := p.worldPlugin.Read(0).ViewFor(cam); view != nil {
		fog = view.Fogging()
	}
	d.set("SolidSight", fog[:]...)
	d.set("SolidPane", paneWide, paneFrom, paneTo, litShare)
	d.set("SolidIndoor", indoorSky, indoorBounce, 0, 0)
	d.set("SolidLamp", roomLamp[:]...)
	d.set("SolidUncovering", 0)
	d.set("SolidUncoveredIn", 0, 0, 0, 0)
	d.opts.ReadDepth = nil
}

// drawer is a Direct's options and uniforms, each boxed once
type drawer struct {
	opts  render.DrawMeshOptions
	own   map[string][]float32
	boxed map[string]any
}

// begin readies d for a draw under the frame's uniforms u
func (d *drawer) begin(u render.Uniforms) {
	if d.own == nil {
		d.own, d.boxed, d.opts.Uniforms = map[string][]float32{}, map[string]any{},
			map[string]any{}
	}
	u.Into(d.opts.Uniforms)
}

func (d *drawer) set(name string, v ...float32) {
	u, ok := d.own[name]
	if !ok || len(u) != len(v) {
		u = make([]float32, len(v))
		d.own[name] = u
		d.boxed[name] = u
	}
	copy(u, v)
	d.opts.Uniforms[name] = d.boxed[name]
}
