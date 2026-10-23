package painter

import (
	"embed"
	"image"

	"github.com/kjkrol/gram/plugins/topography/internal/water"
	"github.com/kjkrol/gram/render"
)

//go:embed shaders/*.wgsl
var shaders embed.FS

var lanesShader = render.NewMeshShaderWith("painter's ways", render.Files(shaders,
	"shaders/lanes.wgsl"), []render.Uniform{
	{Name: "LaneTo", Size: 4}, {Name: "LaneGlint", Size: 1}, {Name: "LaneStream", Size: 1},
}).Instanced(laneVec4s)

// a piece's vec4s: its corners, its weights, its mix, its flow at its corners, its sprite's and
// its mix sprite's rectangles and its shine, its mix's shine, how soft it blends and its bits
const laneVec4s = 9

// a piece's bits
const (
	laneFaded  = 1
	laneMixes  = 2
	laneRuns   = 4
	laneGlints = 8
)

// lanes are the ways across the board's tiles as the GPU draws them from above, an instance a
// piece, laid anew as the board changes
type lanes struct {
	at     []float32
	ver    uint64 // one more than the board's count of changes they were laid at; 0 none
	atlas  render.AtlasSource
	moving bool // water runs or glints on some
	opts   render.DrawMeshOptions
	own    map[string]any
}

// drawWaysCPU draws the ways as DrawWays does out of the pieces wayAnew lays on the CPU: the
// reference the GPU's are checked against until the bake on the CPU goes
func (l *Painter) drawWaysCPU(dst *render.Image, atlas render.AtlasSource, px float32,
	u render.Uniforms) {
	g := &l.lanes
	if v := l.board.Changes() + 1; v != g.ver || atlas != g.atlas {
		g.ver, g.atlas = v, atlas
		l.layLanes(atlas)
	}
	if len(g.at) == 0 || px <= 0 {
		return
	}
	if g.own == nil {
		g.own, g.opts.Uniforms = map[string]any{}, map[string]any{}
	}
	u.Into(g.opts.Uniforms)
	b := dst.Bounds()
	g.set("LaneTo", 2*px/float32(b.Dx()), -2*px/float32(b.Dy()), -1, 1)
	g.set("LaneGlint", float32(water.SeaGlint()))
	g.set("LaneStream", float32(water.RunningWater()))
	g.opts.Images[0], g.opts.Instances, g.opts.Vertices = atlas.Atlas(), g.at, 6
	dst.DrawMesh(nil, lanesShader, &g.opts)
}

// DrawWays draws the ways across the tiles from above into dst, px pixels a world unit, out of
// atlas, under the frame's uniforms u, as Above readied the Painter: each way laid in pieces on
// the GPU as wayAnew lays them, its sprite blended and turned into its mix, the water running and
// glinting over it, row by row.
func (l *Painter) DrawWays(dst *render.Image, atlas render.AtlasSource, px float32,
	u render.Uniforms) {
	if px <= 0 {
		return
	}
	l.refresh(atlas)
	b, g := &l.baking, &l.lanes
	b.ways, g.moving = b.ways[:0], false
	for i := range l.board.CellCount() {
		x, y := uint32(i)%l.sq.Cols, uint32(i)/l.sq.Cols
		w := l.topOf(l.board.At(x, y)).way
		if !w.Runs() {
			continue
		}
		b.ways = append(b.ways, float32(x), float32(y), 0, 0)
		g.moving = g.moving || l.heights && (w.shine > 0 || w.mixes && w.mixShine > 0)
	}
	if len(b.ways) == 0 {
		return
	}
	if b.own == nil {
		b.own, b.opts.Uniforms = map[string]any{}, map[string]any{}
	}
	u.Into(b.opts.Uniforms)
	detail := min(max((l.cellPx-farCell)/(nearCell-farCell), 0), 1)
	l.paintSet(dst, layerAlbedo, image.Point{}, 0, px, detail, false)
	b.opts.Images = [4]*render.Image{atlas.Atlas(), l.cells.rects, l.cells.img, nil}
	b.opts.Instances, b.opts.Vertices = b.ways, wayVertices
	dst.DrawMesh(nil, topsShader, &b.opts)
}

// WaysMoving reports whether what DrawWays draws changes with the clock: water that may run or
// glint on some way.
func (l *Painter) WaysMoving() bool { return l.lanes.moving }

func (l *Painter) layLanes(atlas render.AtlasSource) {
	g := &l.lanes
	g.at, g.moving = g.at[:0], false
	for i := range l.board.CellCount() {
		c := l.board.At(uint32(i)%l.sq.Cols, uint32(i)/l.sq.Cols)
		if !l.topOf(c).way.Runs() {
			continue
		}
		for _, p := range l.cellTile(c).Way() {
			g.lay(&p, atlas)
		}
	}
}

func (g *lanes) lay(p *WayPiece, atlas render.AtlasSource) {
	var mix [4]float32 // as far as its look has turned, the water on it runs or glints
	if p.Mixes {
		for k, m := range p.Mix {
			mix[k] = min(max(m, 0), 1)
		}
	}
	var bits float32
	runs, glints := false, false
	for _, m := range mix {
		runs = runs || p.Shine*(1-m) > 0
		glints = glints || p.MixShine*m > 0
	}
	if p.Faded {
		bits += laneFaded
	}
	if p.Mixes && max(mix[0], mix[1], mix[2], mix[3]) > 0 {
		bits += laneMixes
	}
	if p.Shine > 0 && runs {
		bits, g.moving = bits+laneRuns, true
	}
	if !p.Faded && glints {
		bits, g.moving = bits+laneGlints, true
	}
	soft := p.Soft
	if soft <= 0 {
		soft = 0.5
	}
	w, f := p.World, p.Flow
	look, glaze := insetUV(atlas, p.Sprite), [4]float32{}
	if int(bits)&laneMixes != 0 {
		glaze = insetUV(atlas, p.MixSprite)
	}
	g.at = append(g.at,
		w[0][0], w[0][1], w[1][0], w[1][1], w[2][0], w[2][1], w[3][0], w[3][1],
		p.Weight[0], p.Weight[1], p.Weight[2], p.Weight[3], mix[0], mix[1], mix[2], mix[3],
		f[0][0], f[0][1], f[1][0], f[1][1], f[2][0], f[2][1], f[3][0], f[3][1],
		look[0], look[1], look[2], look[3], glaze[0], glaze[1], glaze[2], glaze[3],
		p.Shine, p.MixShine, min(max(soft, 0.01), 0.5), bits)
}

// insetUV is id's rectangle on atlas, half a texel in from its edges, as a sprite samples it
func insetUV(atlas render.AtlasSource, id render.SpriteID) [4]float32 {
	x0, y0, x1, y1 := atlas.UV(id)
	return [4]float32{x0 + 0.5, y0 + 0.5, x1 - 0.5, y1 - 0.5}
}

func (g *lanes) set(name string, v ...float32) {
	u, ok := g.own[name]
	if s, _ := u.([]float32); !ok || len(s) != len(v) {
		u = make([]float32, len(v))
		g.own[name] = u
	}
	copy(u.([]float32), v)
	g.opts.Uniforms[name] = u
}
