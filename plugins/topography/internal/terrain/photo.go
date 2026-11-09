package terrain

import (
	"fmt"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/internal/f32"
	"github.com/kjkrol/gram/lod"
	"github.com/kjkrol/gram/plugins/world/view"
	"github.com/kjkrol/gram/render"
)

// the sheets' shaders at L4 and L5, photo.wgsl's LEVEL, and those working out the tops their
// lattices' corners stand at, tops.wgsl's
var (
	photoShaders = [2]*render.Shader{photoLeveled(lod.Strategic), photoLeveled(lod.L5)}
	topsShaders  = [2]*render.Shader{topsLeveled(lod.Strategic), topsLeveled(lod.L5)}
)

func photoLeveled(level lod.Level) *render.Shader {
	src := render.Files(shaders, "shaders/lattice.wgsl", "shaders/light.wgsl", "shaders/photo.wgsl")
	src = append([]byte(fmt.Sprintf("const LEVEL: u32 = %du;\n", level)), src...)
	return render.NewMeshShaderWith(fmt.Sprintf("terrain-sheet-level%d", level), src,
		[]render.Uniform{
			{Name: "ViewProj", Size: 16}, {Name: "Eye", Size: 3}, {Name: "Bend", Size: 1},
			{Name: "Cell", Size: 1}, {Name: "Corners", Size: 2}, {Name: "Low", Size: 1},
			{Name: "Span", Size: 1}, {Name: "SheetPx", Size: 1}, {Name: "SheetQuads", Size: 1},
			{Name: "HazeUp", Size: 1}, {Name: "HazeWay", Size: 3}, {Name: "Sight", Size: 4},
			{Name: "Bands", Size: 1}, {Name: "ShadowTint", Size: 3}, {Name: "Warmth", Size: 1},
		}).Instanced(2)
}

func topsLeveled(level lod.Level) *render.Shader {
	src := render.Files(shaders, "shaders/tops.wgsl")
	src = append([]byte(fmt.Sprintf("const LEVEL: u32 = %du;\n", level)), src...)
	return render.NewMeshShaderWith(fmt.Sprintf("terrain-sheet-tops-level%d", level), src,
		[]render.Uniform{
			{Name: "SheetPx", Size: 1}, {Name: "SheetQuads", Size: 1},
			{Name: "SheetReach", Size: 1},
		}).Instanced(2)
}

// the most texels a side of a quad of the sheets' lattices, a sector's texels at L4 about the
// pixels it spans at its finest there: a coarser lattice has fewer corners but raises a wider
// margin round a small thing standing alone
const photoQuad = 8

// photoMesh draws the sheets of what stands over the ground at L4 and L5: a lattice over each
// sector seen there whose sheet holds a photograph of something, its corners at the tops it holds,
// worked out as a photograph changes
type photoMesh struct {
	opts, working render.DrawMeshOptions
	painted       func(i int) uint32
	// by sector the photograph its sheet held as last seen, and the sectors a row
	held []uint32
	cols int
	// by L4 and L5: the lattice's quads a side and the texels its tops are sought within, by
	// sector whether its corners' tops wait to be worked out, the tops by corner, the instances
	// drawn (the sectors seen there and those of them whose tops are worked out), and the
	// lattice's triangles over its corners shared with the quads a side they were laid for
	quads, reach [2]int
	stale        [2][]bool
	tops         [2]*render.Image
	boxes, redo  [2][]float32
	lattice      [2]*render.Indices
	laid         [2]int
	seen         func(i int, box geom.AABB, level lod.Level)
}

func (p *photoMesh) init(painted func(i int) uint32) {
	p.painted = painted
	p.opts = render.DrawMeshOptions{WriteDepth: true}
	p.working = render.DrawMeshOptions{Vertices: 6}
	p.seen = func(i int, box geom.AABB, level lod.Level) {
		if level < lod.Strategic || level > lod.L5 || p.held[i] == 0 {
			return
		}
		k := level - lod.Strategic
		n := p.quads[k]
		x, y := float32(i%p.cols*n), float32(i/p.cols*n)
		p.boxes[k] = append(p.boxes[k], box.TopLeft.X, box.TopLeft.Y, box.BottomRight.X,
			box.BottomRight.Y, x, y, 0, 0)
		if p.stale[k][i] {
			p.stale[k][i] = false
			p.redo[k] = append(p.redo[k], x, y, x+float32(n+1), y+float32(n+1), box.TopLeft.X,
				box.TopLeft.Y, box.BottomRight.X, box.BottomRight.Y)
		}
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
	box := r.grid.Box(0)
	side := max(box.BottomRight.X-box.TopLeft.X, box.BottomRight.Y-box.TopLeft.Y)
	sheets := [2]struct {
		albedo, relief *render.Image
		px             float32
	}{{albedo, relief, perUnit}, {albedoL5, reliefL5, perUnit / 2}}
	for k, at := range sheets {
		p.quads[k], p.reach[k] = photoLattice(side * at.px)
	}
	p.watch(r.grid)
	p.boxes[0], p.boxes[1] = p.boxes[0][:0], p.boxes[1][:0]
	p.redo[0], p.redo[1] = p.redo[0][:0], p.redo[1][:0]
	r.cfg.Seen(cam).Sectors(p.seen)
	p.opts.Depth, p.opts.Uniforms, p.working.Uniforms = t.Depth, r.opts.Uniforms, r.opts.Uniforms
	for k, at := range sheets {
		if len(p.boxes[k]) == 0 {
			continue
		}
		r.set("SheetPx", at.px)
		r.set("SheetQuads", float32(p.quads[k]))
		r.set("SheetReach", float32(p.reach[k]))
		if len(p.redo[k]) > 0 {
			p.working.Images = [4]*render.Image{nil, at.albedo, at.relief, albedoL5}
			p.working.Instances = p.redo[k]
			p.tops[k].DrawMesh(nil, topsShaders[k], &p.working)
		}
		p.opts.Images = [4]*render.Image{r.lattice, at.albedo, at.relief, p.tops[k]}
		p.opts.Instances = p.boxes[k]
		t.Screen.DrawMesh(p.latticeOf(k), photoShaders[k], &p.opts)
	}
}

// watch has the tops of the sectors round one whose photograph changed worked out anew, a corner's
// top reaching into its neighbours' photographs, and those of every sector where the tops are
// made anew for a lattice of another size
func (p *photoMesh) watch(grid view.Sectors) {
	n, cols := grid.Len(), grid.Cols()
	rows := n / cols
	if len(p.held) != n || p.cols != cols {
		p.held, p.cols = make([]uint32, n), cols
	}
	for k := range p.tops {
		w, h := cols*p.quads[k]+1, rows*p.quads[k]+1
		if t := p.tops[k]; t != nil && t.Bounds().Dx() == w && t.Bounds().Dy() == h &&
			len(p.stale[k]) == n {
			continue
		}
		if p.tops[k] != nil {
			p.tops[k].Deallocate()
		}
		p.tops[k], p.stale[k] = render.NewImage(w, h), make([]bool, n)
		for i := range p.stale[k] {
			p.stale[k][i] = true
		}
	}
	for i := range n {
		v := p.painted(i)
		if v == p.held[i] {
			continue
		}
		p.held[i] = v
		c, r := i%cols, i/cols
		for y := max(r-1, 0); y <= min(r+1, rows-1); y++ {
			for x := max(c-1, 0); x <= min(c+1, cols-1); x++ {
				p.stale[0][y*cols+x], p.stale[1][y*cols+x] = true, true
			}
		}
	}
}

// latticeOf is the triangles of the lattice at L4 (k 0) or L5 (k 1), its quads a side over their
// corners, row by row
func (p *photoMesh) latticeOf(k int) *render.Indices {
	n := p.quads[k]
	if p.lattice[k] != nil && p.laid[k] == n {
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
	p.lattice[k], p.laid[k] = render.NewIndices(idx), n
	return p.lattice[k]
}

// photoLattice is how many quads a side of a sector texels a side lays, photoQuad texels a side
// each at most, and how many texels round a corner its tops are sought within: a quad, so every
// quad over something stands at its tops
func photoLattice(texels float32) (n, reach int) {
	n = max(int(f32.Ceil(texels/photoQuad)), 1)
	return n, max(int(f32.Ceil(texels/float32(n))), 1)
}
