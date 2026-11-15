package bridges

import (
	"slices"

	"embed"
	"image/color"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/internal/f32"
	"github.com/kjkrol/gram/lod"
	"github.com/kjkrol/gram/plugin"
	"github.com/kjkrol/gram/plugins/atmosphere/air"
	"github.com/kjkrol/gram/render"
)

//go:embed shaders
var shaders embed.FS

var structureShader = render.NewMeshShaderWith("bridges structure",
	render.Files(shaders, "shaders/structure.wgsl"),
	[]render.Uniform{
		{Name: "SpanViewProj", Size: 16},
		{Name: "SpanEye", Size: 3},
		{Name: "SpanBend", Size: 1},
		{Name: "SpanHaze", Size: 4},
		{Name: "SpanHazeWay", Size: 3},
		{Name: "SpanSight", Size: 4},
	}).Instanced(5)

// Model is how a bridge laid along a path is built, drawn as solids at every level: None draws
// its deck's and piers' looks alone.
type Model uint8

const (
	None Model = iota
	// Beam is a slab on wall piers.
	Beam
	// Arch is a stone bridge, an arch from pier to pier under a solid parapet.
	Arch
	// Viaduct is a girder on slender piers.
	Viaduct
)

// Sky is the air the bridges far off are hazed by: plugins/atmosphere's Read.
type Sky interface{ Air() air.Weather }

// WithAir has the bridges far off hazed by the air of sky, as the ground is.
func (p *Plugin) WithAir(sky Sky) *Plugin {
	p.sky = sky
	return p
}

const (
	pieceFloats = 20
	// the pieces a cell's bend is drawn in, as the ground's roads are
	bends = 4
	// a wall, a slab seen from above, a slab seen from below
	wallPiece  = 0
	slabPiece  = 1
	underPiece = 2
	// how far past its sector's edge a bridge's piece may lie, world units
	overhang = 64
)

// structure draws the bridges laid along paths: their girders, parapets, piers and arches
type structure struct {
	p *Plugin
	// the pieces being laid, a sector's together, and how low and how high they reach
	pieces map[int][]float32
	spans  map[int][2]float32
	laid   [][]station // every bridge's stations, as it is laid
	// the structures laid, kept on the GPU, and those laid since, drawn from the swap
	shown *stock
	next  *stock
	stale bool

	sectors []seenSpan
	ranges  []render.InstanceRange
	opts    render.DrawMeshOptions
	own     map[string][]float32
	boxed   map[string]any
}

// stock is the pieces of every sector kept on the GPU: where each sector's lie and how low and
// how high they reach
type stock struct {
	kept  *render.Instances
	at    map[int]render.InstanceRange
	spans map[int][2]float32
}

type seenSpan struct {
	box      geom.AABB
	low, top float32
	pieces   render.InstanceRange
}

var (
	_ render.Direct    = (*structure)(nil)
	_ plugin.Publisher = (*structure)(nil)
)

// Swap has the structures laid last drawn.
func (r *structure) Swap() {
	if r.next == nil {
		return
	}
	if r.shown != nil {
		r.shown.kept.Release()
	}
	r.shown, r.next = r.next, nil
}

// Publish lays the structures anew where the level's bridges changed.
func (r *structure) Publish() {
	if !r.stale {
		return
	}
	heights := r.p.board.Read(0).Heights()
	if heights == nil {
		return
	}
	r.stale = false
	r.pieces, r.spans = map[int][]float32{}, map[int][2]float32{}
	r.laid = r.laid[:0]
	for _, w := range r.p.Res.Wound {
		r.laid = append(r.laid, stations(w))
	}
	for i, w := range r.p.Res.Wound {
		r.lay(i, w, heights.At)
	}
	if r.next != nil {
		r.next.kept.Release()
	}
	kept, at := render.PackInstances(pieceFloats, r.pieces)
	r.next = &stock{kept: kept, at: at[0], spans: r.spans}
	r.pieces, r.spans = nil, nil
}

// a point of a bridge's axis: where, its deck's level, half its width across, how far along
type station struct {
	at   geom.Vec
	z    float32
	side geom.Vec
	far  float32
}

// stations are the points w's deck is drawn through, as the ground's roads bend through their
// cells' axes
func stations(w Wound) []station {
	n := len(w.At)
	mid := func(i, k int) (geom.Vec, float32) {
		return geom.NewVec((w.At[i].X+w.At[k].X)/2, (w.At[i].Y+w.At[k].Y)/2),
			(w.Deck[i] + w.Deck[k]) / 2
	}
	var pts []geom.Vec
	var zs []float32
	add := func(p geom.Vec, z float32) {
		pts, zs = append(pts, p), append(zs, z)
	}
	add(w.At[0], w.Deck[0])
	for i := 1; i < n-1; i++ {
		p0, z0 := mid(i-1, i)
		p2, z2 := mid(i, i+1)
		for k := 0; k <= bends; k++ {
			if k == 0 && i > 1 {
				continue
			}
			t := float32(k) / bends
			z := w.Deck[i] + (z2-w.Deck[i])*(2*t-1)
			if t <= 0.5 {
				z = z0 + (w.Deck[i]-z0)*2*t
			}
			add(geom.NewVec(
				(1-t)*(1-t)*p0.X+2*t*(1-t)*w.At[i].X+t*t*p2.X,
				(1-t)*(1-t)*p0.Y+2*t*(1-t)*w.At[i].Y+t*t*p2.Y), z)
		}
	}
	add(w.At[n-1], w.Deck[n-1])
	out := make([]station, len(pts))
	for i, p := range pts {
		a, b := pts[max(i-1, 0)], pts[min(i+1, len(pts)-1)]
		l := max(f32.Hypot(b.X-a.X, b.Y-a.Y), f32.Eps(1))
		out[i] = station{at: p, z: zs[i],
			side: geom.NewVec(-(b.Y-a.Y)/l*w.Width/2, (b.X-a.X)/l*w.Width/2)}
		if i > 0 {
			out[i].far = out[i-1].far + f32.Hypot(p.X-pts[i-1].X, p.Y-pts[i-1].Y)
		}
	}
	return out
}

// beside says another bridge's deck lies at p, about as high as z: two decks side by side are
// one roadway, no parapet between them
func (r *structure) beside(own int, p geom.Vec, z, within float32) bool {
	for i, st := range r.laid {
		if i == own {
			continue
		}
		for _, s := range st {
			half := f32.Hypot(s.side.X, s.side.Y)
			if f32.Hypot(p.X-s.at.X, p.Y-s.at.Y) < half && f32.Abs(z-s.z) < within {
				return true
			}
		}
	}
	return false
}

func (r *structure) lay(own int, w Wound, ground func(geom.Vec) float32) {
	st := r.laid[own]
	total := st[len(st)-1].far
	spec := r.p.types.specs[w.Kind]
	every := spec.Span
	if every <= 0 {
		every = total
	}
	// the piers stand evenly between the heads
	bays := max(int(total/every+0.5), 1)
	bay := total / float32(bays)
	body, rail := straight(spec.Body), straight(spec.Parapet)
	depth := func(far, void float32) float32 {
		if spec.Model != Arch {
			return spec.Girder
		}
		u := far/bay - f32.Floor(far/bay)
		rise := min(bay/2, max(void-spec.Girder, 0))
		return spec.Girder + rise*(2*u-1)*(2*u-1)
	}
	void := func(s station) float32 { return s.z - ground(s.at) }
	for i := 1; i < len(st); i++ {
		a, b := st[i-1], st[i]
		sector := r.p.worldPlugin.Read(0).SectorOf(a.at.X, a.at.Y)
		da, db := depth(a.far, void(a)), depth(b.far, void(b))
		for _, sign := range [2]float32{-1, 1} {
			pa := geom.NewVec(a.at.X+a.side.X*sign, a.at.Y+a.side.Y*sign)
			pb := geom.NewVec(b.at.X+b.side.X*sign, b.at.Y+b.side.Y*sign)
			out := geom.NewVec(a.side.X*sign, a.side.Y*sign)
			r.wall(sector, pa, pb, a.z, b.z, da, db, out, body)
			mid := geom.NewVec((pa.X+pb.X)/2, (pa.Y+pb.Y)/2)
			if h := spec.Rail.Height; h > 0 && !r.beside(own, mid, (a.z+b.z)/2, 1.5*h) {
				r.wall(sector, pa, pb, a.z+h, b.z+h, h, h, out, rail)
				in := geom.NewVec(-out.X, -out.Y)
				r.wall(sector, pa, pb, a.z+h, b.z+h, h, h, in, rail)
			}
		}
		r.slab(sector, a, b, a.z-da, b.z-db, underPiece, body)
	}
	if spec.Model == None || spec.Pier <= 0 {
		return
	}
	k := 1
	for i := 1; i < len(st) && k < bays; i++ {
		at := float32(k) * bay
		if st[i].far < at {
			continue
		}
		a, b := st[i-1], st[i]
		t := (at - a.far) / max(b.far-a.far, f32.Eps(1))
		s := station{
			at:   geom.NewVec(a.at.X+(b.at.X-a.at.X)*t, a.at.Y+(b.at.Y-a.at.Y)*t),
			z:    a.z + (b.z-a.z)*t,
			side: b.side,
		}
		k++
		foot := ground(s.at)
		top := s.z - spec.Girder
		if top-foot <= 0 {
			continue
		}
		wide, thick := float32(0.9), spec.Pier
		switch spec.Model {
		case Arch:
			wide, thick = 1, 1.5*spec.Pier
		case Viaduct:
			wide = 0.3
		}
		r.pier(s, top, foot, wide, thick, body)
	}
}

// pier stands a pier under the deck at s, from top down into the ground at foot: wide of the
// deck's width across it, thick along it
func (r *structure) pier(s station, top, foot, wide, thick float32, rgb [3]float32) {
	sector := r.p.worldPlugin.Read(0).SectorOf(s.at.X, s.at.Y)
	half := max(f32.Hypot(s.side.X, s.side.Y), f32.Eps(1))
	across := geom.NewVec(s.side.X/half, s.side.Y/half)
	along := geom.NewVec(-across.Y, across.X)
	var c [4]geom.Vec
	for i, d := range [4][2]float32{{-1, -1}, {1, -1}, {1, 1}, {-1, 1}} {
		c[i] = geom.NewVec(s.at.X+across.X*half*wide*d[0]+along.X*thick/2*d[1],
			s.at.Y+across.Y*half*wide*d[0]+along.Y*thick/2*d[1])
	}
	outs := [4]geom.Vec{geom.NewVec(-along.X, -along.Y), across, along,
		geom.NewVec(-across.X, -across.Y)}
	// into the ground, so a slope shows no gap under it
	sunk := top - foot + thick
	for i := range 4 {
		r.wall(sector, c[i], c[(i+1)%4], top, top, sunk, sunk, outs[i], rgb)
	}
}

func (r *structure) wall(sector int, p0, p1 geom.Vec, z0, z1, d0, d1 float32, out geom.Vec,
	rgb [3]float32) {
	l := max(f32.Hypot(out.X, out.Y), f32.Eps(1))
	r.add(sector, min(z0-d0, z1-d1), max(z0, z1),
		p0.X, p0.Y, p1.X, p1.Y,
		z0, z1, d0, d1,
		out.X/l, out.Y/l, 0, 0,
		rgb[0], rgb[1], rgb[2], wallPiece,
		0, 0, 0, 0)
}

func (r *structure) slab(sector int, a, b station, z0, z1, kind float32, rgb [3]float32) {
	r.add(sector, min(z0, z1), max(z0, z1),
		a.at.X, a.at.Y, b.at.X, b.at.Y,
		z0, z1, 0, 0,
		a.side.X, a.side.Y, b.side.X, b.side.Y,
		rgb[0], rgb[1], rgb[2], kind,
		0, 0, 0, 0)
}

func (r *structure) add(sector int, low, top float32, piece ...float32) {
	r.pieces[sector] = append(r.pieces[sector], piece...)
	span, ok := r.spans[sector]
	if !ok {
		span = [2]float32{low, top}
	}
	r.spans[sector] = [2]float32{min(span[0], low), max(span[1], top)}
}

func straight(c color.RGBA) [3]float32 {
	return [3]float32{float32(c.R) / 255, float32(c.G) / 255, float32(c.B) / 255}
}

// Compose gathers the sectors cam sees with their bridges' pieces.
func (r *structure) Compose(f *render.Frame, cam camera.Camera) {
	r.sectors = r.sectors[:0]
	st := r.shown
	if st == nil || len(st.at) == 0 {
		return
	}
	view := r.p.worldPlugin.Read(0).ViewFor(cam)
	if view == nil || view.Grid() == nil {
		for k, pieces := range st.at {
			r.sectors = append(r.sectors, seenSpan{low: st.spans[k][0], top: st.spans[k][1],
				pieces: pieces})
		}
		slices.SortFunc(r.sectors, func(a, b seenSpan) int {
			return a.pieces.First - b.pieces.First
		})
	} else {
		view.Sectors(func(i int, box geom.AABB, _ lod.Level) {
			if pieces := st.at[i]; pieces.Count > 0 {
				r.sectors = append(r.sectors, seenSpan{box: box, low: st.spans[i][0],
					top: st.spans[i][1], pieces: pieces})
			}
		})
	}
	if len(r.sectors) > 0 {
		f.DrawOpaque()
	}
}

// Draw draws the pieces gathered into the target through cam under the frame's uniforms u: of
// a picture drawn in layers, the sectors in the layer's window and not wholly in the next
// one's.
func (r *structure) Draw(t render.Target, cam camera.Camera, u render.Uniforms) {
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
	r.ranges = r.ranges[:0]
	for _, s := range r.sectors {
		if s.box.BottomRight != s.box.TopLeft &&
			!tr.Shows(s.box, overhang, s.low, s.top, w, h, hole, holed) {
			continue
		}
		r.ranges = render.JoinSpan(r.ranges, s.pieces)
	}
	if len(r.ranges) == 0 {
		return
	}
	if r.own == nil {
		r.own, r.boxed, r.opts.Uniforms = map[string][]float32{}, map[string]any{}, map[string]any{}
	}
	u.Into(r.opts.Uniforms)
	r.set("SpanViewProj", tr.M[:]...)
	r.set("SpanEye", tr.Eye[:]...)
	r.set("SpanBend", tr.Bend)
	var seeing, up float32
	var way [3]float32
	if r.p.sky != nil {
		seeing, up, way = r.p.sky.Air().Seen(cam, true)
	}
	r.set("SpanHaze", seeing, up, 0, 0)
	r.set("SpanHazeWay", way[:]...)
	var fog [4]float32
	if view := r.p.worldPlugin.Read(0).ViewFor(cam); view != nil {
		fog = view.Fogging()
	}
	r.set("SpanSight", fog[:]...)
	r.opts.Depth, r.opts.WriteDepth = t.Depth, true
	r.opts.Vertices, r.opts.Kept, r.opts.Spans = 6, r.shown.kept, r.ranges
	t.Screen.DrawMesh(nil, structureShader, &r.opts)
}

func (r *structure) set(name string, v ...float32) {
	u, ok := r.own[name]
	if !ok || len(u) != len(v) {
		u = make([]float32, len(v))
		r.own[name] = u
		r.boxed[name] = u
	}
	copy(u, v)
	r.opts.Uniforms[name] = r.boxed[name]
}
