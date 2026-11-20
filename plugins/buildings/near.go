package buildings

import (
	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/lod"
	"github.com/kjkrol/gram/render"
)

var (
	_ render.Direct    = (*near)(nil)
	_ render.Overlying = (*near)(nil)
)

// near draws the buildings of the sectors seen finer than the strategic scale among the
// figures, whole, their rooms inside: what of them lies over something uncovered gives way to
// it before the cameras the world's settings say, and a room an uncovered entity stands in is
// uncovered whole (its floor into the pass Uncovered) before those the buildings' settings say
type near struct {
	s *solids

	// what cam composed last sees: the sectors with their parts and their rooms' insides, the
	// rooms bared and those an uncovered entity stands in shut before it, by their place among
	// the stock's, and whether what gives way does so before it; the ranges drawn of the parts,
	// the insides and the bared
	sectors []seen
	bared   []int
	shut    []int
	gives   bool
	ranges  []render.InstanceRange
	insides []render.InstanceRange
	floors  []render.InstanceRange
	// the parts and the insides of the houses open, the parts of those shut, and the ranges drawn
	// without them
	open, openIn, whole []render.InstanceRange
	holes               []render.InstanceRange
	cutParts, cutIn     []render.InstanceRange
	d                   drawer
}

// Overlies: drawn among the figures over the ground's layers (render.Overlying).
func (n *near) Overlies() {}

// Compose gathers the sectors cam sees finer than the strategic scale with their parts and their
// rooms.
func (n *near) Compose(f *render.Frame, cam camera.Camera) {
	n.sectors, n.bared, n.shut = n.sectors[:0], n.bared[:0], n.shut[:0]
	s := n.s
	st := s.shown
	if st == nil || len(st.fine) == 0 {
		return
	}
	view := s.p.worldPlugin.Read(0).ViewFor(cam)
	if view == nil || view.Grid() == nil {
		for k, parts := range st.fine {
			n.sectors = append(n.sectors, seen{low: st.spans[k][0], top: st.spans[k][1],
				parts: parts, inner: st.inner[k], sector: k})
		}
	} else {
		view.Sectors(func(i int, box geom.AABB, level lod.Level) {
			if parts := st.fine[i]; level < lod.Strategic && parts.Count > 0 {
				n.sectors = append(n.sectors, seen{box: box, low: st.spans[i][0],
					top: st.spans[i][1], parts: parts, inner: st.inner[i], sector: i})
			}
		})
	}
	if len(n.sectors) == 0 {
		return
	}
	n.gives = s.p.worldPlugin.Read(0).GivesWayDrawn(cam)
	fr := &s.frames[s.frame]
	open := opens(fr.settings, camera.HowOf(cam))
	for _, k := range fr.bared {
		switch {
		case !n.sees(st.rooms[k].sector):
		case open:
			n.bared = append(n.bared, k)
		default:
			n.shut = append(n.shut, k)
		}
	}
	f.DrawCutout()
	if len(n.bared) > 0 {
		f.DrawUncovered()
	}
}

// sees reports whether the sector is among those composed
func (n *near) sees(sector int) bool {
	for i := range n.sectors {
		if n.sectors[i].sector == sector {
			return true
		}
	}
	return false
}

// Draw draws the parts and the rooms gathered among the figures in the cutout pass, giving way
// to what is uncovered, a house open round its room its walls cut low by it and no roof, and the
// rooms bared into the pass of the uncovered.
func (n *near) Draw(t render.Target, cam camera.Camera, u render.Uniforms) {
	if t.Screen == nil || t.Depth == nil || len(n.sectors) == 0 {
		return
	}
	tr, w, h, ok := transformOf(cam)
	if !ok {
		return
	}
	s := n.s
	n.d.begin(u)
	s.p.scene(&n.d, cam, &tr, s.hazed)
	n.d.set("SolidFar", 0)
	n.d.opts.Depth, n.d.opts.WriteDepth = t.Depth, true
	switch t.Pass {
	case render.Uncovered:
		n.drawBared(t, &tr, w, h)
		return
	case render.Cutout:
	default:
		return
	}
	st := s.shown
	n.ranges = shownOf(n.ranges[:0], n.sectors, cam, &tr, w, h, false)
	n.open, n.openIn, n.whole = n.open[:0], n.openIn[:0], n.whole[:0]
	for _, k := range n.bared {
		if room := &st.rooms[k]; inSpans(n.ranges, room.parts) {
			n.open = append(n.open, room.parts)
			n.openIn = append(n.openIn, render.InstanceRange{First: room.at, Count: 1})
		}
	}
	for _, k := range n.shut {
		if room := &st.rooms[k]; inSpans(n.ranges, room.parts) {
			n.whole = append(n.whole, room.parts)
		}
	}
	n.holes = append(append(n.holes[:0], n.open...), n.whole...)
	n.cutParts = cut(n.cutParts, n.ranges, n.holes)
	if len(n.cutParts) > 0 {
		if (n.gives || len(n.bared) > 0) && t.Uncovered != nil {
			x0, y0, x1, y1 := within(t, w, h)
			n.d.set("SolidUncovering", 1)
			n.d.set("SolidUncoveredIn", x0, y0, x1, y1)
			n.d.opts.ReadDepth = t.Uncovered
		}
		n.d.opts.Kept, n.d.opts.Instances = st.kept, nil
		n.d.opts.Vertices, n.d.opts.Spans = solidVertices, n.cutParts
		t.Screen.DrawMesh(nil, solidShader, &n.d.opts)
		n.d.opts.ReadDepth = nil
		n.d.set("SolidUncovering", 0)
	}
	if len(n.whole) > 0 {
		// a house shut round an uncovered entity in its room stands whole, giving way to none
		n.d.opts.Kept, n.d.opts.Instances = st.kept, nil
		n.d.opts.Vertices, n.d.opts.Spans = solidVertices, n.whole
		t.Screen.DrawMesh(nil, solidShader, &n.d.opts)
	}
	n.insides = shownOf(n.insides[:0], n.sectors, cam, &tr, w, h, true)
	n.cutIn = cut(n.cutIn, n.insides, n.openIn)
	if len(n.cutIn) > 0 {
		n.d.opts.Kept, n.d.opts.Instances = st.inside, nil
		n.d.opts.Vertices, n.d.opts.Spans = innerVertices(st.bays), n.cutIn
		t.Screen.DrawMesh(nil, innerShader, &n.d.opts)
	}
	if len(n.open) == 0 {
		return
	}
	// the houses open: their walls by the eye cut low, the far ones to their rooms' tops
	n.d.set("SolidOpen", 1, openWalls, 0, 0)
	n.d.opts.Kept, n.d.opts.Instances = st.kept, nil
	n.d.opts.Vertices, n.d.opts.Spans = solidVertices, n.open
	t.Screen.DrawMesh(nil, solidShader, &n.d.opts)
	n.d.opts.Kept = st.inside
	n.d.opts.Vertices, n.d.opts.Spans = innerVertices(st.bays), n.openIn
	t.Screen.DrawMesh(nil, innerShader, &n.d.opts)
	n.d.set("SolidOpen", 0, openWalls, 0, 0)
}

// inSpans reports whether r lies whole within one of spans
func inSpans(spans []render.InstanceRange, r render.InstanceRange) bool {
	for _, s := range spans {
		if r.First >= s.First && r.First+r.Count <= s.First+s.Count {
			return true
		}
	}
	return false
}

// cut appends to dst the instances of spans but those holes take
func cut(dst, spans, holes []render.InstanceRange) []render.InstanceRange {
	dst = dst[:0]
	for _, s := range spans {
		at, end := s.First, s.First+s.Count
		for at < end {
			// the hole reaching past at that begins first
			hole := render.InstanceRange{First: end}
			for _, h := range holes {
				if h.First+h.Count > at && h.First < hole.First {
					hole = h
				}
			}
			if hole.First > at {
				dst = append(dst, render.InstanceRange{First: at, Count: min(hole.First, end) - at})
			}
			at = max(at, hole.First+hole.Count)
		}
	}
	return dst
}

// drawBared draws the floors of the rooms bared into the target's depth, the uncovered's, and
// grows the rectangle of the viewport w by h they lie in by each, as tr lays it
func (n *near) drawBared(t render.Target, tr *camera.Transform, w, h float32) {
	st := n.s.shown
	if len(n.bared) == 0 || st == nil || st.inside == nil {
		return
	}
	n.floors = n.floors[:0]
	for _, k := range n.bared {
		n.floors = render.JoinSpan(n.floors, render.InstanceRange{First: st.rooms[k].at, Count: 1})
	}
	n.d.opts.Kept, n.d.opts.Instances = st.inside, nil
	n.d.opts.Vertices, n.d.opts.Spans = baredVertices, n.floors
	t.Screen.DrawMesh(nil, baredShader, &n.d.opts)
	in := t.UncoveredIn
	if in == nil {
		return
	}
	for _, k := range n.bared {
		b := st.rooms[k].inner[:]
		along, side := geom.NewVec(b[6], b[7]), geom.NewVec(-b[7], b[6])
		for _, c := range [8][3]float32{{-1, -1, 0}, {1, -1, 0}, {-1, 1, 0}, {1, 1, 0},
			{-1, -1, 1}, {1, -1, 1}, {-1, 1, 1}, {1, 1, 1}} {
			x := b[0] + along.X*c[0]*b[4] + side.X*c[1]*b[5]
			y := b[1] + along.Y*c[0]*b[4] + side.Y*c[1]*b[5]
			px, py, ok := onScreen(tr, x, y, b[2]+c[2]*(b[3]-b[2]), w, h)
			if !ok {
				continue
			}
			in[0], in[1] = min(in[0], px), min(in[1], py)
			in[2], in[3] = max(in[2], px), max(in[3], py)
		}
	}
}

// onScreen is where tr lays the world point (x, y, z) on a viewport w by h, sunk as far off as
// it lies (common.wgsl's clipOf); false behind the eye
func onScreen(tr *camera.Transform, x, y, z, w, h float32) (float32, float32, bool) {
	m := &tr.M
	dx, dy := x-tr.Eye[0], y-tr.Eye[1]
	z -= tr.Bend * (dx*dx + dy*dy)
	cw := m[3]*x + m[7]*y + m[11]*z + m[15]
	if cw <= 0 {
		return 0, 0, false
	}
	return ((m[0]*x+m[4]*y+m[8]*z+m[12])/cw + 1) / 2 * w,
		(1 - (m[1]*x+m[5]*y+m[9]*z+m[13])/cw) / 2 * h, true
}

// within is the rectangle the uncovered lie in (render.Target.UncoveredIn, the pixels of a
// viewport w by h), in the pixels of the target's texture; the whole of it where none is told
func within(t render.Target, w, h float32) (x0, y0, x1, y1 float32) {
	b := t.Screen.Bounds()
	x, y := float32(b.Min.X), float32(b.Min.Y)
	sx, sy := float32(b.Dx())/w, float32(b.Dy())/h
	in := [4]float32{0, 0, w, h}
	if t.UncoveredIn != nil {
		in = *t.UncoveredIn
	}
	return x + in[0]*sx, y + in[1]*sy, x + in[2]*sx, y + in[3]*sy
}
