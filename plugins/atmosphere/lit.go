package atmosphere

import (
	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/aabbworld/plane"
	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/plugins/board"
	"github.com/kjkrol/gram/plugins/board/look"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/render"
)

// WithBoard puts a flat board under the sky: its tiles and the world's sprites take the sun's light
// on level ground, so the day tints the map — night dark, dawn warm — and what sways leans with the
// wind; brd's Map and the world's Look are wrapped, so call it after any other Map or Look is set
// and before Use. A board in relief (plugins/topography) lights itself under the atmosphere
// instead (topography.Plugin.WithAtmosphere), and the clouds' shadows over a flat board are the
// Clouds source's.
func (p *Plugin) WithBoard(brd *board.Plugin) *Plugin {
	brd.WithMap(&litMap{Map: brd.Map(), sky: p})
	p.worldPlugin.SetLook(litLook{Look: p.worldPlugin.Look(), sky: p})
	return p
}

// litMap is a board.Map whose tiles are lit by the sky and lean in its wind.
type litMap struct {
	board.Map
	sky *Plugin
}

func (m *litMap) Look() look.Look { return litCells{Look: m.Map.Look(), sky: m.sky} }

func (m *litMap) Dressing() look.Dressing {
	return &litDressing{Dressing: m.Map.Dressing(), sky: m.sky}
}

// litCells lays the cells as the map's Look does, what sways leaning with the wind: seen from above
// by its top, which moves as far as it stands high.
type litCells struct {
	look.Look
	sky *Plugin
}

func (l litCells) Cell(f *render.Frame, cam camera.Camera, t *look.Tile) {
	if amount, rise := t.Sway(); amount > 0 {
		leaning := *t
		lx, ly := l.sky.Air().Sway(f.Time(), (t.X0+t.X1)/2, (t.Y0+t.Y1)/2, amount)
		leaning.X0, leaning.X1 = t.X0+lx*rise, t.X1+lx*rise
		leaning.Y0, leaning.Y1 = t.Y0+ly*rise, t.Y1+ly*rise
		l.Look.Cell(f, cam, &leaning)
		return
	}
	l.Look.Cell(f, cam, t)
}

// litDressing is the map's Dressing with the tiles lit by the sun on level ground, the frame handed
// the sky's uniforms; without a dressing under it, that alone.
type litDressing struct {
	look.Dressing
	sky *Plugin
}

func (d *litDressing) Begin(f *render.Frame, cam camera.Camera) {
	sun := d.sky.Sun()
	sun.Frame(f)
	d.sky.Air().Frame(f, sun)
	if d.Dressing != nil {
		d.Dressing.Begin(f, cam)
	}
}

func (d *litDressing) Light(*look.Tile) render.Shade {
	return render.Lit(d.sky.Sun().Light(0, 0, 1))
}

// EvenLight is the sun's light on level ground, as every tile has it (Light), where the dressing
// under it dresses every tile the same frame after frame; false where it does not.
func (d *litDressing) EvenLight() (render.Light, bool) {
	if d.Dressing != nil {
		e, ok := d.Dressing.(look.EvenLit)
		if !ok {
			return render.Light{}, false
		}
		if _, ok := e.EvenLight(); !ok {
			return render.Light{}, false
		}
	}
	return d.sky.Sun().Light(0, 0, 1), true
}

func (d *litDressing) Sheet(atlas render.AtlasSource) render.AtlasSource {
	if d.Dressing == nil {
		return atlas
	}
	return d.Dressing.Sheet(atlas)
}

func (d *litDressing) Base(t *look.Tile) render.SpriteID {
	if d.Dressing == nil {
		return t.Sprite()
	}
	return d.Dressing.Base(t)
}

func (d *litDressing) FaceLight(t *look.Tile, dx, dy int) render.Light {
	if d.Dressing == nil {
		return render.Light{1, 1, 1}
	}
	return d.Dressing.FaceLight(t, dx, dy)
}

func (d *litDressing) Covers(t *look.Tile) bool { return d.Dressing != nil && d.Dressing.Covers(t) }

func (d *litDressing) Dress(f *render.Frame, cam camera.Camera, t *look.Tile, x0, y0, x1, y1, depth float32) {
	if d.Dressing != nil {
		d.Dressing.Dress(f, cam, t, x0, y0, x1, y1, depth)
	}
}

// litLook draws the world's entities as the world's Look does, in the sun's light on level ground
// and leaning with the wind as much as they sway.
type litLook struct {
	world.Look
	sky *Plugin
}

// Begin readies the world's Look under it where it draws on the GPU.
func (l litLook) Begin(cam camera.Camera) {
	if d, ok := l.Look.(world.DirectLook); ok {
		d.Begin(cam)
	}
}

// DrawSprites has the world's Look under it draw where it draws on the GPU.
func (l litLook) DrawSprites(t render.Target, cam camera.Camera, u render.Uniforms) {
	if d, ok := l.Look.(world.DirectLook); ok {
		d.DrawSprites(t, cam, u)
	}
}

func (l litLook) Sprite(f *render.Frame, cam camera.Camera, box plane.AABB, z world.Z, atlas render.AtlasSource, a render.Appearance, light render.Light) {
	sun := l.sky.Sun().Light(0, 0, 1)
	lit := render.Light{sun[0] * light[0], sun[1] * light[1], sun[2] * light[2]}
	if a.Sway > 0 { // seen from above by its top, as high as it is wide, leaning with the wind
		sizeX, sizeY := float32(box.Size.X), float32(box.Size.Y)
		cx, cy := float32(box.TopLeft.X)+sizeX/2, float32(box.TopLeft.Y)+sizeY/2
		lx, ly := l.sky.Air().Sway(f.Time(), cx, cy, a.Sway)
		rise := max(sizeX, sizeY)
		box = plane.NewAABB(geom.NewVec(box.TopLeft.X+float64(lx*rise), box.TopLeft.Y+float64(ly*rise)), box.Size.X, box.Size.Y)
	}
	a.Sway = 0 // leant here: the Look under takes the sprite as it stands
	l.Look.Sprite(f, cam, box, z, atlas, a, lit)
}
