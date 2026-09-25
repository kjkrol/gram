package world

import (
	"github.com/kjkrol/gram/plugin/host"
	"time"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/aabbworld/plane"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/plugin"
	"github.com/kjkrol/gram/render"
	"github.com/kjkrol/uid"
)

var _ render.Source = (*Renderer)(nil)

// Renderer is the render.Source of the Position+Appearance entities in the View of the viewport's
// camera — what it sees this tick — on the Objects tier, running the Each behaviors of a Drawing
// over each chunk to settle their layers. A Stage that has not ticked yet sees everything.
type Renderer struct {
	renderQuery *goke.Query
	base        goke.Comp[Base]
	appearance  goke.Comp[Appearance]
	z           goke.OptComp[Z]
	host        *host.EachHost[Drawing]
	layers      [][]Appearance // one per entity of the chunk being drawn
	atlas       render.AtlasSource
	worldW      float32
	worldH      float32
	views       func(camera.Camera) *View
	view        *View // the one being drawn

	ids   []uid.UID64
	bases []Base
}

func newRenderer(atlas render.AtlasSource, views func(camera.Camera) *View, host *host.EachHost[Drawing], worldW, worldH uint32) *Renderer {
	return &Renderer{atlas: atlas, views: views, host: host, worldW: float32(worldW), worldH: float32(worldH)}
}

func (s *Renderer) Init(si *goke.SysInit) {
	qb := si.NewQueryBuilder(&s.base, &s.appearance).Optional(&s.z)
	s.host.Bind(qb)
	s.renderQuery = qb.Build()
}

// Compose hands f every drawn entity: from above its box, in a piece per image where it crosses a
// wrap seam; through an isometric camera a billboard the size of its box standing on its centre,
// at the depth of that centre, which ties with the tile it stands on.
func (s *Renderer) Compose(f *render.Frame, cam camera.Camera) {
	_, iso := cam.Projection().(camera.Isometric)
	s.view = s.views(cam)
	s.each(func(i int, alt float32, sprite render.SpriteID) {
		box := s.bases[i].Pos.AABB
		if !cam.Visible(box.AABB) {
			return
		}
		if !iso {
			s.flat(f, box, sprite)
			return
		}
		x0, y0 := float32(box.TopLeft.X), float32(box.TopLeft.Y)
		x1, y1 := float32(box.BottomRight.X), float32(box.BottomRight.Y)
		cx, cy := (x0+x1)/2, (y0+y1)/2
		f.Sprite(render.Objects, cam.Depth(cx, cy, alt), s.atlas, sprite, render.Billboard(cam, cx, cy, alt, x1-x0, y1-y0), 1)
	})
}

// flat draws box from above, each image of it where it crosses a wrap seam showing its own part of
// the sprite.
func (s *Renderer) flat(f *render.Frame, box plane.AABB, sprite render.SpriteID) {
	sizeX, sizeY := float32(box.Size.X), float32(box.Size.Y)
	render.VisitWrapImages(box, s.worldW, s.worldH, func(img geom.AABB, dx, dy float32) bool {
		x0, y0 := float32(img.TopLeft.X), float32(img.TopLeft.Y)
		x1, y1 := float32(img.BottomRight.X), float32(img.BottomRight.Y)
		u0, u1 := uvSpan(x1-x0, sizeX, dx)
		v0, v1 := uvSpan(y1-y0, sizeY, dy)
		f.SpriteRectUV(render.Objects, 0, s.atlas, sprite, x0, y0, x1, y1, u0, v0, u1, v1)
		return true
	})
}

// uvSpan is the slice of the sprite one image shows along one axis.
func uvSpan(imgSize, spriteSize, shift float32) (float32, float32) {
	visible := imgSize / spriteSize
	if shift == 0 {
		return 0, visible
	}
	return 1 - visible, 1
}

// each walks the drawn entities of the View, their Drawing behaviors run, calling draw once per
// layer with the entity's index in the chunk and its altitude.
func (s *Renderer) each(draw func(i int, alt float32, sprite render.SpriteID)) {
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
			s.host.Run(tick, cursor, s.at)
		}
		for i, id := range s.ids {
			if !s.view.Contains(id) {
				continue
			}
			alt := float32(0)
			if zs != nil {
				alt = float32(zs[i].Altitude)
			}
			for _, l := range s.layers[i] {
				draw(i, alt, l.SpriteID)
			}
		}
	}
}

// at describes the i-th entity of the chunk being drawn.
func (s *Renderer) at(i int) Drawing {
	return Drawing{ID: s.ids[i], Base: &s.bases[i], Layers: &s.layers[i]}
}
