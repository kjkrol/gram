package buildings

import (
	"slices"

	"embed"
	"image/color"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/lod"
	"github.com/kjkrol/gram/plugin"
	"github.com/kjkrol/gram/render"
)

//go:embed shaders
var shaders embed.FS

var solidShader = render.NewMeshShaderWith("buildings solids",
	render.Files(shaders, "shaders/solid.wgsl"),
	[]render.Uniform{
		{Name: "SolidViewProj", Size: 16},
		{Name: "SolidEye", Size: 3},
		{Name: "SolidBend", Size: 1},
		{Name: "SolidHaze", Size: 4},
		{Name: "SolidHazeWay", Size: 3},
		{Name: "SolidSight", Size: 4},
		{Name: "SolidFar", Size: 1},
	}).Instanced(5)

// a part's floats and its vertices: the two walls seen, two slopes and two ends, and far off
// the two walls and a lid
const (
	solidFloats   = 20
	solidVertices = 30
	farVertices   = 18
)

// solids draws the buildings of the level's file, a part an instance, each sector's as its
// level in the camera's view says: whole at the close and standard scales, their bodies alone
// and without windows at the strategic one
type solids struct {
	p *Plugin

	// the buildings laid once the ground is known, and those laid since, drawn from the swap
	shown *stock
	next  *stock
	stale bool

	// the sectors the camera composed last sees, and the ranges of those in the layer drawn
	sectors []seen
	ranges  []render.InstanceRange
	far     []render.InstanceRange
	opts    render.DrawMeshOptions
	own     map[string][]float32
	boxed   map[string]any
}

// stock is the parts of every sector's buildings kept on the GPU: where each sector's lie,
// whole and coarse, and how low and how high its buildings reach
type stock struct {
	kept   *render.Instances
	fine   map[int]render.InstanceRange
	coarse map[int]render.InstanceRange
	spans  map[int][2]float32
}

// a sector's parts as a camera sees them, and the box they stand in
type seen struct {
	box      geom.AABB
	low, top float32
	parts    render.InstanceRange
	far      bool
}

// how far past its sector's edge a building may stand, world units: half the widest
const overhang = 128

var (
	_ render.Direct    = (*solids)(nil)
	_ plugin.Publisher = (*solids)(nil)
)

// Publish lays the parts anew where the level's buildings changed.
func (r *solids) Publish() {
	if !r.stale {
		return
	}
	heights := r.p.board.Read(0).Heights()
	if heights == nil {
		return
	}
	r.stale = false
	fine, coarse := map[int][]float32{}, map[int][]float32{}
	spans := map[int][2]float32{}
	for k, all := range r.p.Res.Stood {
		for n, b := range all {
			spec := r.p.types.specs[b.Kind]
			seed := float32((k*7919+n*104729)%1000) / 1000
			fine[k] = lay(fine[k], spec, spec.of(b.Row), heights.At, seed, true)
			coarse[k] = lay(coarse[k], spec, spec.of(b.Row), heights.At, seed, false)
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
		r.next.kept.Release()
	}
	kept, at := render.PackInstances(solidFloats, fine, coarse)
	r.next = &stock{kept: kept, fine: at[0], coarse: at[1], spans: spans}
}

// Swap has the buildings laid last drawn.
func (r *solids) Swap() {
	if r.next == nil {
		return
	}
	if r.shown != nil {
		r.shown.kept.Release()
	}
	r.shown, r.next = r.next, nil
}

// lay appends to dst the parts of the building b of spec standing on the ground at: its foot at
// the lowest of the ground under its body's corners, its storeys counted from the highest;
// whole, with its windows and its fine parts
func lay(dst []float32, spec Spec, b standing, at func(geom.Vec) float32, seed float32,
	whole bool) []float32 {
	side := geom.NewVec(-b.dir.Y, b.dir.X)
	foot, ground := at(b.mid), at(b.mid)
	for _, sx := range [2]float32{-1, 1} {
		for _, sy := range [2]float32{-1, 1} {
			h := at(off(b.mid, b.dir, side, sx*b.size.X/2, sy*b.size.Y/2))
			foot, ground = min(foot, h), max(ground, h)
		}
	}
	for _, part := range spec.Parts {
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
		dst = append(dst,
			mid.X, mid.Y, foot, ground+walls,
			size.X/2, size.Y/2, b.dir.X, b.dir.Y,
			rise, ridge, spec.Storey, bay,
			wr, wg, wb, seed,
			tr, tg, tb, ground)
	}
	return dst
}

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

// Compose gathers the sectors cam sees with their parts, each at its level.
func (r *solids) Compose(f *render.Frame, cam camera.Camera) {
	r.sectors = r.sectors[:0]
	st := r.shown
	if st == nil || len(st.fine) == 0 {
		return
	}
	view := r.p.worldPlugin.Read(0).ViewFor(cam)
	if view == nil || view.Grid() == nil {
		for k, parts := range st.fine {
			r.sectors = append(r.sectors, seen{low: st.spans[k][0], top: st.spans[k][1],
				parts: parts})
		}
		slices.SortFunc(r.sectors, func(a, b seen) int { return a.parts.First - b.parts.First })
	} else {
		view.Sectors(func(i int, box geom.AABB, level lod.Level) {
			parts, far := st.fine[i], level >= lod.Strategic
			if far {
				parts = st.coarse[i]
			}
			if parts.Count > 0 {
				r.sectors = append(r.sectors, seen{box: box, low: st.spans[i][0],
					top: st.spans[i][1], parts: parts, far: far})
			}
		})
	}
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
	rays, ok := cam.(camera.Rays)
	if !ok {
		return
	}
	field, ok := rays.Rays()
	if !ok {
		return
	}
	w, h := cam.Viewport()
	tr, ok := camera.SceneTransform(field, w, h)
	if !ok {
		return
	}
	hole, holed := camera.Inner(cam)
	r.ranges, r.far = r.ranges[:0], r.far[:0]
	for _, s := range r.sectors {
		if s.box.BottomRight != s.box.TopLeft &&
			!tr.Shows(s.box, overhang, s.low, s.top, w, h, hole, holed) {
			continue
		}
		if s.far {
			r.far = render.JoinSpan(r.far, s.parts)
		} else {
			r.ranges = render.JoinSpan(r.ranges, s.parts)
		}
	}
	if len(r.ranges) == 0 && len(r.far) == 0 {
		return
	}
	if r.own == nil {
		r.own, r.boxed, r.opts.Uniforms = map[string][]float32{}, map[string]any{}, map[string]any{}
	}
	u.Into(r.opts.Uniforms)
	r.set("SolidViewProj", tr.M[:]...)
	r.set("SolidEye", tr.Eye[:]...)
	r.set("SolidBend", tr.Bend)
	var seeing, up float32
	var way [3]float32
	if r.p.sky != nil {
		seeing, up, way = r.p.sky.Air().Seen(cam, true)
	}
	r.set("SolidHaze", seeing, up, 0, 0)
	r.set("SolidHazeWay", way[:]...)
	var fog [4]float32
	if view := r.p.worldPlugin.Read(0).ViewFor(cam); view != nil {
		fog = view.Fogging()
	}
	r.set("SolidSight", fog[:]...)
	r.opts.Depth, r.opts.WriteDepth = t.Depth, true
	r.opts.Kept = r.shown.kept
	if len(r.ranges) > 0 {
		r.set("SolidFar", 0)
		r.opts.Vertices, r.opts.Spans = solidVertices, r.ranges
		t.Screen.DrawMesh(nil, solidShader, &r.opts)
	}
	if len(r.far) > 0 {
		r.set("SolidFar", 1)
		r.opts.Vertices, r.opts.Spans = farVertices, r.far
		t.Screen.DrawMesh(nil, solidShader, &r.opts)
	}
}

func (r *solids) set(name string, v ...float32) {
	u, ok := r.own[name]
	if !ok || len(u) != len(v) {
		u = make([]float32, len(v))
		r.own[name] = u
		r.boxed[name] = u
	}
	copy(u, v)
	r.opts.Uniforms[name] = r.boxed[name]
}
