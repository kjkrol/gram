package world

import (
	"image/color"
	"math"
	"time"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/gram/plugin/host"

	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/plugin"
	"github.com/kjkrol/gram/render"
	"github.com/kjkrol/uid"
)

var _ render.Source = (*Renderer)(nil)

// Renderer is the render.Source of the Position+Appearance entities in the View of the viewport's
// camera — what it sees this tick — each laid on the screen by the world's Look, running the Each
// behaviors of a Drawing over each chunk to settle their layers. A Stage that has not ticked yet
// sees everything.
type Renderer struct {
	renderQuery *goke.Query
	base        goke.Comp[Base]
	appearance  goke.Comp[Appearance]
	z           goke.OptComp[Z]
	host        *host.EachHost[Drawing]
	layers      [][]Appearance // one per entity of the chunk being drawn
	atlas       render.AtlasSource
	look        func() Look
	views       func(camera.Camera) *View
	view        *View // the one being drawn
	// sun and ground lay the shadows of what stands in a world with heights; nil sun, none
	sun    func() Sun
	ground func() Ground

	ids   []uid.UID64
	bases []Base
	about func(i int) Drawing // at, bound once so a frame allocates no method value
}

func newRenderer(atlas render.AtlasSource, views func(camera.Camera) *View, host *host.EachHost[Drawing], look func() Look) *Renderer {
	r := &Renderer{atlas: atlas, views: views, host: host, look: look}
	r.about = r.at
	return r
}

// shadowTier puts the shadows of what stands over the ground and its grid and under what stands.
const shadowTier = render.Ground + 20

// maxShadowReach caps how far a unit of height casts its shadow: a sun on the horizon would cast it
// for ever.
const maxShadowReach = 6

// shadowColor is the veil a shadow lays on the ground at its middle.
var shadowColor = color.RGBA{A: 110}

func (s *Renderer) Init(si *goke.SysInit) {
	qb := si.NewQueryBuilder(&s.base, &s.appearance).Optional(&s.z)
	s.host.Bind(qb)
	s.renderQuery = qb.Build()
}

// Compose hands f every drawn entity in sight of cam, as the world's Look lays it, and in a world
// with heights its shadow on the ground and the sun's light on it, as on level ground; a flat
// world's are drawn as they are.
func (s *Renderer) Compose(f *render.Frame, cam camera.Camera) {
	look := s.look()
	s.view = s.views(cam)
	var sun Sun
	var ground Ground
	light := render.Light{1, 1, 1}
	if s.sun != nil {
		sun, ground = s.sun(), s.ground()
		light = sun.Light(0, 0, 1)
	}
	s.each(func(i int, z *Z) {
		box := s.bases[i].Pos.AABB
		if !cam.Visible(box.AABB) {
			return
		}
		alt := float32(0)
		if z != nil {
			alt = float32(z.Altitude)
			if s.sun != nil {
				s.shadow(f, cam, box.AABB, *z, sun, ground)
			}
		}
		for _, l := range s.layers[i] {
			look.Sprite(f, cam, box, alt, s.atlas, l.SpriteID, light)
		}
	})
}

// shadow lays the shadow of an entity standing in box as z says on the ground away from the sun:
// a soft patch as wide as the box, stretched by its height and pushed off by how far above the
// ground it stands, at the depth of its nearest corner.
func (s *Renderer) shadow(f *render.Frame, cam camera.Camera, box geom.AABB, z Z, sun Sun, ground Ground) {
	sx, sy, sz := sun.Dir[0], sun.Dir[1], sun.Dir[2]
	if sz <= 0 {
		return // the sun is down
	}
	groundAt := func(x, y float32) float32 {
		if ground == nil {
			return 0
		}
		return float32(ground.At(geom.NewVec(float64(x), float64(y))))
	}
	cx, cy := float32(box.TopLeft.X+box.BottomRight.X)/2, float32(box.TopLeft.Y+box.BottomRight.Y)/2
	half := float32(max(box.BottomRight.X-box.TopLeft.X, box.BottomRight.Y-box.TopLeft.Y)) / 2
	// away from the sun, and how far a unit of height casts its shadow that way
	ux, uy := float32(1), float32(0)
	across := float32(math.Hypot(float64(sx), float64(sy)))
	if across > 0 {
		ux, uy = -sx/across, -sy/across
	}
	reach := min(across/sz, maxShadowReach)
	above := max(float32(z.Altitude)-groundAt(cx, cy), 0)
	start, length := above*reach, float32(z.Height)*reach
	mid := start + length/2
	mx, my := cx+ux*mid, cy+uy*mid
	along, wide := length/2+half, half
	var dst render.Corners
	depth := float32(math.Inf(-1))
	for k, c := range [4][2]float32{{-along, -wide}, {along, -wide}, {-along, wide}, {along, wide}} {
		x, y := mx+ux*c[0]-uy*c[1], my+uy*c[0]+ux*c[1]
		g := groundAt(x, y)
		dst[k][0], dst[k][1] = cam.Project(x, y, g)
		depth = max(depth, cam.Depth(x, y, g))
	}
	fade := half / 2 * cam.Zoom() // a solid core, soft for the outer half of each side
	f.Soft(shadowTier, depth, dst, shadowColor, render.Fade{Left: fade, Right: fade, Top: fade, Bottom: fade})
}

// each walks the drawn entities of the View, their Drawing behaviors run, calling visit once per
// entity with its index in the chunk and its Z, nil without one.
func (s *Renderer) each(visit func(i int, z *Z)) {
	tick := plugin.Tick{Now: time.Now()}
	s.renderQuery.All()
	for s.renderQuery.Next() {
		cursor := s.renderQuery.Cursor()
		s.ids, s.bases = cursor.IDs, s.base.Slice(cursor)
		appearances := s.appearance.Slice(cursor)
		zs := s.z.Slice(cursor)

		for len(s.layers) < len(s.ids) {
			s.layers = append(s.layers, nil)
		}
		for i := range s.ids {
			s.layers[i] = append(s.layers[i][:0], appearances[i])
		}
		if !s.host.Empty() {
			s.host.Run(tick, cursor, s.about)
		}
		for i, id := range s.ids {
			if !s.view.Contains(id) {
				continue
			}
			var z *Z
			if zs != nil {
				z = &zs[i]
			}
			visit(i, z)
		}
	}
}

// at describes the i-th entity of the chunk being drawn.
func (s *Renderer) at(i int) Drawing {
	return Drawing{ID: s.ids[i], Base: &s.bases[i], Layers: &s.layers[i]}
}
