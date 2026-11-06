package world

import (
	"embed"
	"image"
	"math"

	"github.com/kjkrol/gram/render"
)

//go:embed shaders/*.wgsl
var shaders embed.FS

var (
	// what stands in the sectors of a round, each in a slot of the darkroom for its albedo and
	// one for its relief
	photoShader = render.NewMeshShaderWith("world's sheet",
		render.Files(shaders, "shaders/sheet_photo.wgsl"), []render.Uniform{
			{Name: "SheetTarget", Size: 2}, {Name: "SheetPx", Size: 1}, {Name: "SheetSplit", Size: 1},
		}).Instanced(5)
	// the darkroom's slots copied into the sheets at L4, those halved into the sheets at L5, and
	// the rectangles drawn into filled before
	developShader = render.NewMeshShaderWith("world's sheet developed",
		render.Files(shaders, "shaders/sheet_half.wgsl"), []render.Uniform{
			{Name: "DevelopTarget", Size: 2}, {Name: "DevelopPass", Size: 1},
			{Name: "DevelopFill", Size: 1},
		}).Instanced(2)
)

// what developShader does: copy the first image, halve the albedo or the relief at L4, fill
const (
	developCopy = iota
	developAlbedo
	developRelief
	developFill
)

const (
	// the most texels a side of the darkroom
	darkroomSide = 8192
	// the most texels the darkroom holds
	darkroomTexels = 1 << 23
)

// the instance laid under every thing of a sector (sheet_photo.wgsl): it leaves the albedo clear
// and the relief facing up at no height where nothing stands, a photograph's blending leaving a
// cleared texel as it is drawn
var backdrop = shot{10: 2}

// darkroom develops the photographs a publish asked for, on the frame's side, a round of sectors
// at a time: into an image with a depth of its own, a slot a sector for the albedo on its left
// and one for the relief as far right, then into the sheets. A rectangle of the sheets is filled
// with ones and the values laid over it darkest, so a texel at L5 two sectors of a round share is
// laid alike by both.
type darkroom struct {
	room, albedo, relief *render.Image
	depth                *render.Depth
	// the albedo's slots' top-left corners, and how far right the relief's lie
	slots                      []image.Point
	split                      int
	opts, clearing, developing render.DrawMeshOptions
	uniforms                   map[string][]float32
	boxed                      map[string]any
	// the round's instances: the photographs' (the backdrop first in each slot), the sectors'
	// texels at L4 with where their slots lie, those at L5, and the room's whole
	inst, rects, halves, whole []float32
	// how many slots the round fills
	n int
}

// develop has the frame's photographs developed into the sheets, once a frame, the sprites drawn
// from atlas
func (s *sheets) develop(f *sheetFrame, atlas render.AtlasSource) {
	if f.developed {
		return
	}
	f.developed = true
	if atlas == nil || len(f.photos) == 0 {
		return
	}
	d := &s.room
	d.ready(s)
	sprites := atlas.Atlas()
	for _, p := range f.photos {
		if p.rect.Empty() {
			continue
		}
		d.lay(p, f.instances[p.from:p.to])
		if d.n == len(d.slots) {
			d.shoot(s, sprites)
		}
	}
	if d.n > 0 {
		d.shoot(s, sprites)
	}
}

// schedule has the frame's photographs developed as a draw first reads a sheet
func (s *sheets) schedule(fill func(*render.Image)) {
	for _, img := range [4]*render.Image{s.albedo, s.relief, s.albedoL5, s.reliefL5} {
		img.Defer(fill)
	}
}

// lay puts the photograph p of the things inst in the round's next slots
func (d *darkroom) lay(p photo, inst []float32) {
	at, from := d.slots[d.n], p.rect.Min
	d.n++
	for _, x := range [2]int{at.X, at.X + d.split} {
		place := [4]float32{float32(from.X - x), float32(from.Y - at.Y), p.span}
		slot := [4]float32{float32(x), float32(at.Y), float32(x + p.rect.Dx()),
			float32(at.Y + p.rect.Dy())}
		d.inst = appendShotIn(d.inst, backdrop[:], place, slot)
		for k := 0; k < len(inst); k += photoFloats {
			place[3]++
			d.inst = appendShotIn(d.inst, inst[k:k+photoFloats], place, slot)
		}
	}
	d.rects = appendRect(d.rects, p.rect, float32(at.X-from.X), float32(at.Y-from.Y))
	d.halves = appendRect(d.halves, halfOf(p.rect), 0, 0)
}

// appendShotIn appends to dst a thing as a photograph lays it, where its sector's slot is and
// the slot
func appendShotIn(dst, thing []float32, place, slot [4]float32) []float32 {
	dst = append(dst, thing...)
	dst = append(dst, place[:]...)
	return append(dst, slot[:]...)
}

// appendRect appends to dst the texels r as developShader draws them, read moved by (x, y)
func appendRect(dst []float32, r image.Rectangle, x, y float32) []float32 {
	return append(dst, float32(r.Min.X), float32(r.Min.Y), float32(r.Max.X), float32(r.Max.Y),
		x, y, 0, 0)
}

// shoot photographs the round's sectors, their sprites on atlas, into the sheets at L4, then
// halves the texels at L5 they lie in
func (d *darkroom) shoot(s *sheets, atlas *render.Image) {
	b := d.room.Bounds()
	// cleared by a draw, darkest with zero: a Clear would be a pass of its own
	d.set(&d.clearing, "DevelopTarget", float32(b.Dx()), float32(b.Dy()))
	d.set(&d.clearing, "DevelopPass", developFill)
	d.set(&d.clearing, "DevelopFill", 0)
	d.room.DrawMesh(nil, developShader, &d.clearing)
	d.set(&d.opts, "SheetTarget", float32(b.Dx()), float32(b.Dy()))
	d.set(&d.opts, "SheetPx", s.perUnit)
	d.set(&d.opts, "SheetSplit", float32(d.split))
	d.opts.Images[0] = atlas
	d.opts.Instances = d.inst
	d.room.DrawMesh(nil, photoShader, &d.opts)
	d.develop(s.albedo, d.rects, developCopy, d.albedo, nil)
	d.develop(s.relief, d.rects, developCopy, d.relief, nil)
	d.develop(s.albedoL5, d.halves, developAlbedo, s.albedo, s.relief)
	d.develop(s.reliefL5, d.halves, developRelief, s.albedo, s.relief)
	d.inst, d.rects, d.halves, d.n = d.inst[:0], d.rects[:0], d.halves[:0], 0
}

// develop fills the rectangles rects of dst with ones, then lays over them darkest what pass
// makes of the images a and b
func (d *darkroom) develop(dst *render.Image, rects []float32, pass int, a, b *render.Image) {
	o := &d.developing
	r := dst.Bounds()
	d.set(o, "DevelopTarget", float32(r.Dx()), float32(r.Dy()))
	d.set(o, "DevelopFill", 1)
	o.Instances, o.Images = rects, [4]*render.Image{a, b}
	for _, step := range [2]int{developFill, pass} {
		d.set(o, "DevelopPass", float32(step))
		o.Darkest = step != developFill
		dst.DrawMesh(nil, developShader, o)
	}
}

// ready makes the darkroom, slots as large as the largest sector of s for as many sectors as a
// frame photographs, as its sides and texels allow, and its depth
func (d *darkroom) ready(s *sheets) {
	if d.room != nil {
		return
	}
	w, h := 1, 1
	for i := range s.grid.Len() {
		r := s.rect(i)
		w, h = max(w, r.Dx()), max(h, r.Dy())
	}
	n := max(min(s.perFrame, s.grid.Len(), darkroomTexels/(2*w*h)), 1)
	cols := min(int(math.Ceil(math.Sqrt(float64(n)))), max(darkroomSide/(2*w), 1))
	rows := min((n+cols-1)/cols, max(darkroomSide/h, 1))
	for k := range min(n, cols*rows) {
		d.slots = append(d.slots, image.Pt(k%cols*w, k/cols*h))
	}
	d.split = cols * w
	d.room = render.NewImage(2*d.split, rows*h)
	d.albedo = d.room.SubImage(image.Rect(0, 0, d.split, rows*h))
	d.relief = d.room.SubImage(image.Rect(d.split, 0, 2*d.split, rows*h))
	d.depth = render.NewDepth()
	d.uniforms, d.boxed = map[string][]float32{}, map[string]any{}
	d.opts = render.DrawMeshOptions{Depth: d.depth, Primed: true, WriteDepth: true, Vertices: 6,
		Uniforms: map[string]any{}}
	d.whole = appendRect(nil, d.room.Bounds(), 0, 0)
	d.clearing = render.DrawMeshOptions{Depth: d.depth, ClearDepth: true, Untested: true,
		Darkest: true, Vertices: 6, Instances: d.whole, Uniforms: map[string]any{}}
	d.developing = render.DrawMeshOptions{Vertices: 6, Uniforms: map[string]any{}}
}

// set puts v in o's uniforms as name, boxed once
func (d *darkroom) set(o *render.DrawMeshOptions, name string, v ...float32) {
	u, ok := d.uniforms[name]
	if !ok || len(u) != len(v) {
		u = make([]float32, len(v))
		d.uniforms[name], d.boxed[name] = u, u
	}
	copy(u, v)
	o.Uniforms[name] = d.boxed[name]
}
