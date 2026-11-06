package terrain

import (
	"fmt"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/internal/f32"
	"github.com/kjkrol/gram/lod"
	"github.com/kjkrol/gram/render"
)

// the sheets' shaders at L4 and L5, photo.wgsl's LEVEL
var photoShaders = [2]*render.Shader{photoLeveled(lod.Strategic), photoLeveled(lod.L5)}

func photoLeveled(level lod.Level) *render.Shader {
	src := render.Files(shaders, "shaders/lattice.wgsl", "shaders/light.wgsl", "shaders/photo.wgsl")
	src = append([]byte(fmt.Sprintf("const LEVEL: u32 = %du;\n", level)), src...)
	return render.NewMeshShaderWith(fmt.Sprintf("terrain-sheet-level%d", level), src,
		[]render.Uniform{
			{Name: "ViewProj", Size: 16}, {Name: "Eye", Size: 3}, {Name: "Bend", Size: 1},
			{Name: "Cell", Size: 1}, {Name: "Corners", Size: 2}, {Name: "Low", Size: 1},
			{Name: "Span", Size: 1}, {Name: "SheetPx", Size: 1}, {Name: "SheetQuads", Size: 1},
			{Name: "SheetReach", Size: 1}, {Name: "HazeUp", Size: 1}, {Name: "HazeWay", Size: 3},
			{Name: "Sight", Size: 4}, {Name: "Bands", Size: 1}, {Name: "ShadowTint", Size: 3},
			{Name: "Warmth", Size: 1},
		}).Instanced(1)
}

// the most quads a side of a sector's lattice at L4 and at L5, and the most texels round a corner
// the tops are sought within
const (
	photoQuadsL4 = 32
	photoQuadsL5 = 8
	photoReach   = 8
)

// photoMesh draws the sheets of what stands over the ground at L4 and L5: a lattice over each
// sector seen there whose sheet holds a photograph of something, its corners at the tops it holds
type photoMesh struct {
	opts    render.DrawMeshOptions
	painted func(i int) bool
	// by L4 and L5, the boxes of the sectors drawn there, an instance each, and the lattice's
	// triangles over its corners shared, quads a side
	boxes   [2][]float32
	lattice [2]*render.Indices
	quads   [2]int
	seen    func(i int, box geom.AABB, level lod.Level)
}

func (p *photoMesh) init(painted func(i int) bool) {
	p.painted = painted
	p.opts = render.DrawMeshOptions{WriteDepth: true}
	p.seen = func(i int, box geom.AABB, level lod.Level) {
		if level < lod.Strategic || level > lod.L5 || !p.painted(i) {
			return
		}
		k := level - lod.Strategic
		p.boxes[k] = append(p.boxes[k], box.TopLeft.X, box.TopLeft.Y, box.BottomRight.X,
			box.BottomRight.Y)
	}
}

// drawPhotos draws the sheets of what stands through cam over the ground drawn, each sector seen
// at L4 or L5 whose sheet holds a photograph of something, under the ground's uniforms
func (r *Renderer) drawPhotos(t render.Target, cam camera.Camera) {
	p := &r.photo
	if r.cfg.Sheet == nil || p.painted == nil || r.grid == nil {
		return
	}
	albedo, relief, albedoL5, reliefL5, perUnit := r.cfg.Sheet()
	if perUnit <= 0 || relief == nil || reliefL5 == nil {
		return
	}
	p.boxes[0], p.boxes[1] = p.boxes[0][:0], p.boxes[1][:0]
	r.cfg.Seen(cam).Sectors(p.seen)
	box := r.grid.Box(0)
	side := max(box.BottomRight.X-box.TopLeft.X, box.BottomRight.Y-box.TopLeft.Y)
	p.opts.Depth, p.opts.Uniforms = t.Depth, r.opts.Uniforms
	for k, at := range [2]struct {
		albedo, relief *render.Image
		px             float32
		most           int
	}{{albedo, relief, perUnit, photoQuadsL4}, {albedoL5, reliefL5, perUnit / 2, photoQuadsL5}} {
		if len(p.boxes[k]) == 0 {
			continue
		}
		n, reach := photoLattice(side*at.px, at.most)
		r.set("SheetPx", at.px)
		r.set("SheetQuads", float32(n))
		r.set("SheetReach", float32(reach))
		// at L4 the albedo at L5 too, where the tops are sought first
		p.opts.Images = [4]*render.Image{r.lattice, at.albedo, at.relief, albedoL5}
		p.opts.Instances = p.boxes[k]
		t.Screen.DrawMesh(p.latticeOf(k, n), photoShaders[k], &p.opts)
	}
}

// latticeOf is the triangles of a lattice of n quads a side over its (n+1)² corners, row by row,
// for the sheets at L4 (k 0) or L5 (k 1)
func (p *photoMesh) latticeOf(k, n int) *render.Indices {
	if p.lattice[k] != nil && p.quads[k] == n {
		return p.lattice[k]
	}
	if p.lattice[k] != nil {
		p.lattice[k].Release()
	}
	idx := make([]uint32, 0, 6*n*n)
	for q := range n * n {
		a := uint32(q/n*(n+1) + q%n)
		b, c := a+1, a+uint32(n)+1
		idx = append(idx, a, b, c, b, c+1, c)
	}
	p.lattice[k], p.quads[k] = render.NewIndices(idx), n
	return p.lattice[k]
}

// photoLattice is how many quads a side of a sector texels a side lays, most at most and a texel
// each at least, a power of two, and how many texels round a corner its tops are sought within:
// a quad, so every quad over something stands at its tops, photoReach at most
func photoLattice(texels float32, most int) (n, reach int) {
	n = most
	for n > 1 && float32(n) > texels {
		n /= 2
	}
	reach = min(max(int(f32.Ceil(texels/float32(n))), 1), photoReach)
	return n, reach
}
