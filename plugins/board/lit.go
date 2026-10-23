package board

import (
	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/aabbworld/plane"
	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/plugins/atmosphere/air"
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/board/internal/draw"
	"github.com/kjkrol/gram/plugins/board/look"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/render"
)

// Atmosphere is the weather over a flat board: what leans what sways and whose clouds shade the
// ground; plugins/atmosphere's Plugin. The light is the light plugin's.
type Atmosphere interface {
	Air() air.Weather
}

// WithAtmosphere puts the board under a's weather: what sways leans with the wind and the clouds'
// shadows lie on its ground. Call it after any other Map or Look is set, before the first frame.
func (p *Plugin) WithAtmosphere(a Atmosphere) *Plugin {
	p.WithMap(&windyMap{Map: p.board.mapping, sky: a, clouds: &draw.Clouds{}})
	p.worldPlugin.Write().SetLook(windyLook{Look: p.worldPlugin.Read(0).Look(), sky: a})
	return p
}

type windyMap struct {
	Map
	sky    Atmosphere
	clouds *draw.Clouds
}

// Look leans what sways on a flat board with the wind.
func (m *windyMap) Look() look.Look {
	if l := m.Map.Look(); l != look.Flat {
		return l
	}
	return look.Swaying
}

func (m *windyMap) Dressing() look.Dressing {
	return &cloudyDressing{Dressing: m.Map.Dressing(), sky: m.sky, clouds: m.clouds}
}

// cloudyDressing lays the clouds' shadows on the ground, the dressing under it dressing the rest
type cloudyDressing struct {
	look.Dressing
	sky    Atmosphere
	clouds *draw.Clouds
}

var _ look.GroundDressing = (*cloudyDressing)(nil)
var _ look.Laid = (*cloudyDressing)(nil)

func (d *cloudyDressing) Way(c cell.ID) cell.Way {
	if l, ok := d.Dressing.(look.Laid); ok {
		return l.Way(c)
	}
	return cell.Way{}
}

func (d *cloudyDressing) Covers() []render.SpriteID {
	if l, ok := d.Dressing.(look.Laid); ok {
		return l.Covers()
	}
	return nil
}

func (d *cloudyDressing) Covered(c cell.ID) uint8 {
	if l, ok := d.Dressing.(look.Laid); ok {
		return l.Covered(c)
	}
	return 0
}

func (d *cloudyDressing) Begin(f *render.Frame, cam camera.Camera) {
	if d.Dressing != nil {
		d.Dressing.Begin(f, cam)
	}
}

// DressGround lays the clouds' shadows on the ground, where the atmosphere casts them.
func (d *cloudyDressing) DressGround(t render.Target, cam camera.Camera, u render.Uniforms) {
	if w := d.sky.Air(); w.ShadeGround {
		d.clouds.Draw(t, cam, u, w.Most().Cover)
	}
}

func (d *cloudyDressing) Base(c cell.ID, own render.SpriteID) render.SpriteID {
	if d.Dressing == nil {
		return own
	}
	return d.Dressing.Base(c, own)
}

// windyLook leans the swaying sprites with the wind, the Look under it drawing them
type windyLook struct {
	world.Look
	sky Atmosphere
}

var _ world.LyingLook = windyLook{}
var _ world.FoggyLook = windyLook{}

func (l windyLook) Begin(f *render.Frame, cam camera.Camera) {
	if d, ok := l.Look.(world.DirectLook); ok {
		d.Begin(f, cam)
	}
}

func (l windyLook) Fog(amount float32) {
	if f, ok := l.Look.(world.FoggyLook); ok {
		f.Fog(amount)
	}
}

func (l windyLook) DrawSprites(t render.Target, cam camera.Camera, u render.Uniforms) {
	if d, ok := l.Look.(world.DirectLook); ok {
		d.DrawSprites(t, cam, u)
	}
}

func (l windyLook) Sprite(
	f *render.Frame, cam camera.Camera, box plane.AABB, z world.Z, atlas render.AtlasSource,
	a render.Appearance, lit render.Illumination,
) {
	if a.Sway > 0 { // seen from above by its top, as high as it is wide
		sizeX, sizeY := float32(box.Size.X), float32(box.Size.Y)
		cx, cy := float32(box.TopLeft.X)+sizeX/2, float32(box.TopLeft.Y)+sizeY/2
		lx, ly := l.sky.Air().Sway(f.Time(), cx, cy, a.Sway)
		rise := max(sizeX, sizeY)
		box = plane.NewAABB(geom.NewVec(box.TopLeft.X+float64(lx*rise),
			box.TopLeft.Y+float64(ly*rise)), box.Size.X, box.Size.Y)
	}
	a.Sway = 0 // leant here: the Look under takes the sprite as it stands
	l.Look.Sprite(f, cam, box, z, atlas, a, lit)
}

func (l windyLook) Lay(f *render.Frame, cam camera.Camera, box plane.AABB, dir geom.Vec,
	lie world.Lie, z world.Z, atlas render.AtlasSource, a render.Appearance,
	edge render.SpriteID, lit render.Illumination) {
	if lies, ok := l.Look.(world.LyingLook); ok {
		lies.Lay(f, cam, box, dir, lie, z, atlas, a, edge, lit)
		return
	}
	l.Look.Sprite(f, cam, box, z, atlas, a, lit)
}

func (l windyLook) Lie(f *render.Frame, cam camera.Camera, box plane.AABB, z world.Z,
	atlas render.AtlasSource, a render.Appearance, lit render.Illumination) {
	if lies, ok := l.Look.(world.LyingLook); ok {
		lies.Lie(f, cam, box, z, atlas, a, lit)
		return
	}
	l.Look.Sprite(f, cam, box, z, atlas, a, lit)
}
