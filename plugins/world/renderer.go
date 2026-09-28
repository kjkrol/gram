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
	// sun lights the sprites while sunlit says so; with ground it lays the shadows of what stands
	// in a world with heights. nil sun, neither
	sun    func() Sun
	sunlit func() bool
	ground func() Ground
	// weather is the air what sways bends in, handed to the frame; nil, a calm
	weather func() Weather
	// clock is the game time the frame's animations go by; nil, the composer's own
	clock func() time.Duration

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

// shadowVeil is how dark a shadow lays on the ground at its middle under a sun of shadowFull: a
// weaker light — the moon, the sun low at dawn — casts it paler.
const (
	shadowVeil = 110
	shadowFull = 0.6
)

func (s *Renderer) Init(si *goke.SysInit) {
	qb := si.NewQueryBuilder(&s.base, &s.appearance).Optional(&s.z)
	s.host.Bind(qb)
	s.renderQuery = qb.Build()
}

// Compose hands f every drawn entity in sight of cam, as the world's Look lays it, in the sun's
// light as on level ground where the world is sunlit, and in a world with heights its shadow on
// the ground; a flat world's without a sun are drawn as they are.
// Clock is the game time the frame's animations go by: the world's tactical clock's, so what sways
// and flows stands in the tactical pause and hurries with the tempo.
func (s *Renderer) Clock() (time.Duration, bool) {
	if s.clock == nil {
		return 0, false
	}
	return s.clock(), true
}

func (s *Renderer) Compose(f *render.Frame, cam camera.Camera) {
	look := s.look()
	s.view = s.views(cam)
	var sun Sun
	var ground Ground
	light := render.Light{1, 1, 1}
	if s.weather != nil {
		f.Weather(s.weather().Frame())
	}
	lit := s.sun != nil && (s.sunlit == nil || s.sunlit())
	if lit {
		sun = s.sun()
		light = sun.Light(0, 0, 1)
	}
	if s.ground != nil {
		ground = s.ground()
	}
	s.each(func(i int, z *Z) {
		box := s.bases[i].Pos.AABB
		if !cam.Visible(box.AABB) {
			return
		}
		var stands Z
		if z != nil {
			stands = *z
			if lit && s.ground != nil {
				s.shadow(f, cam, box.AABB, *z, sun, ground)
			}
		}
		for _, l := range s.layers[i] {
			look.Sprite(f, cam, box, stands, s.atlas, l.SpriteID, light, l.Sway)
		}
	})
}

// shadow lays the shadow of an entity standing in box as z says on the ground away from the sun:
// a soft patch as wide as the box, stretched by its height and pushed off by how far above the
// ground it stands, at the depth of its nearest corner.
func (s *Renderer) shadow(f *render.Frame, cam camera.Camera, box geom.AABB, z Z, sun Sun, ground Ground) {
	sx, sy, sz := sun.Dir[0], sun.Dir[1], sun.Dir[2]
	if sz <= 0 || sun.Strength <= 0 {
		return // the sun is down, or too faint to cast one
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
	scale := camera.ScaleAt(cam, cx, cy, groundAt(cx, cy))
	if scale == 0 {
		return // not in front of the eye
	}
	fade := half / 2 * scale // a solid core, soft for the outer half of each side
	veil := color.RGBA{A: uint8(shadowVeil * min(sun.Strength/shadowFull, 1))}
	f.Soft(shadowTier, depth, dst, veil, render.Fade{Left: fade, Right: fade, Top: fade, Bottom: fade})
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
