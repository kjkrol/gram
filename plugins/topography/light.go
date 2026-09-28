package topography

import (
	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/plugins/board"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/render"
	"math"
)

// Light is the light the world's sun casts on the tile's top at its corners, from the slope of
// the ground there — the tile's own corners and its neighbours', so slopes run on smoothly from
// tile to tile; what stands on the cell is lit as the ground under it. A flat world keeps its
// sprites' colours on level ground and shows the relief by its slopes alone.
func (t *tile) Light() render.Shade {
	if t.memo.has&memoLight == 0 {
		t.memo.light, t.memo.has = t.r.lightOf(t), t.memo.has|memoLight
	}
	return t.memo.light
}

// lightOf is t's Light, worked out anew only when the terrain or the sun has changed.
func (l *dresser) lightOf(t *tile) render.Shade {
	i, _ := l.ordinal(t.ID)
	if i >= len(l.sunlit) {
		return t.light()
	}
	s := &l.sunlit[i]
	if s.lit != l.sunStamp {
		s.light, s.lit = t.light(), l.sunStamp
	}
	return s.light
}

func (t *tile) light() render.Shade {
	r := t.r
	g := r.topOf(t.ID).ground
	left, lok := t.groundBeside(-1, 0)
	right, rok := t.groundBeside(1, 0)
	up, uok := t.groundBeside(0, -1)
	down, dok := t.groundBeside(0, 1)
	w, h := t.X1-t.X0, t.Y1-t.Y0
	// the ground's rise along x and y at each corner, from the corners either side of it
	slope := func(ahead, behind float32, aok, bok bool, own0, own1, step float32) float32 {
		switch {
		case aok && bok:
			return (ahead - behind) / (2 * step)
		case aok:
			return (ahead - own0) / step
		case bok:
			return (own1 - behind) / step
		}
		return (own1 - own0) / step
	}
	sun := r.lamp
	lit := t.sunlit()
	corner := func(k int, dx, dy float32) render.Light { return sun.Shaded(-dx, -dy, 1, lit[k]) }
	if !r.quasi3D {
		level := sun.Shaded(0, 0, 1, 1)
		corner = func(_ int, dx, dy float32) render.Light {
			l := sun.Shaded(-dx, -dy, 1, 1)
			return render.Light{l[0] / level[0], l[1] / level[1], l[2] / level[2]}
		}
	}
	return render.Shade{
		corner(0, slope(g[1], left[0], true, lok, g[0], g[1], w), slope(g[2], up[0], true, uok, g[0], g[2], h)),
		corner(1, slope(right[1], g[0], rok, true, g[0], g[1], w), slope(g[3], up[1], true, uok, g[1], g[3], h)),
		corner(2, slope(g[3], left[2], true, lok, g[2], g[3], w), slope(down[2], g[0], dok, true, g[0], g[2], h)),
		corner(3, slope(right[3], g[2], rok, true, g[2], g[3], w), slope(down[3], g[1], dok, true, g[1], g[3], h)),
	}
}

// sunlit is how much sun reaches each corner of the tile's top: all of it where the board casts no
// shadows.
func (t *tile) sunlit() [4]float32 {
	if t.memo.has&memoLit == 0 {
		t.memo.lit, t.memo.has = t.sunlitAnew(), t.memo.has|memoLit
	}
	return t.memo.lit
}

func (t *tile) sunlitAnew() [4]float32 {
	r := t.r
	if !r.shadows || !r.square {
		return [4]float32{1, 1, 1, 1}
	}
	return r.sunlitOf(t.ID, t.X0, t.Y0, t.X1, t.Y1)
}

// FaceLight is the light the world's sun casts on an upright face of the tile looking dx, dy
// cells away — towards a neighbour it stands above — as much in the sun as the top's edge over it.
func (t *tile) FaceLight(dx, dy int) render.Light {
	if !t.r.quasi3D {
		return render.Light{1, 1, 1}
	}
	lit := t.sunlit()
	var edge float32
	switch {
	case dx > 0:
		edge = (lit[1] + lit[3]) / 2
	case dx < 0:
		edge = (lit[0] + lit[2]) / 2
	case dy > 0:
		edge = (lit[2] + lit[3]) / 2
	default:
		edge = (lit[0] + lit[1]) / 2
	}
	return t.r.lamp.Shaded(float32(dx), float32(dy), 0, edge)
}

// groundBeside is the ground's corners of the cell dx, dy cells away; false off the board.
func (t *tile) groundBeside(dx, dy int) ([4]float32, bool) {
	w, h := t.X1-t.X0, t.Y1-t.Y0
	x, y := (t.X0+t.X1)/2+float32(dx)*w, (t.Y0+t.Y1)/2+float32(dy)*h
	c, ok := t.r.board.CellAt(geom.NewVec(float64(x), float64(y)))
	if !ok {
		return [4]float32{}, false
	}
	return t.r.topOf(c).ground, true
}

// cellSunlit is how much of the sun reaches each corner of a cell's top, 0 to 1.
type cellSunlit struct {
	corners [4]float32
	stamp   uint32
	light   render.Shade // the light on the cell's top, good while lit is sunStamp
	lit     uint32
}

// sunKey is what the shadows depend on: the terrain as it stands and the sun.
type sunKey struct {
	version uint64
	sun     world.Sun
}

// nextSunlit starts a Compose: when the terrain or the sun has changed since the last one, every
// shadow is worked out anew as it comes into sight.
func (l *dresser) nextSunlit() {
	key := sunKey{version: l.version(), sun: l.lighted}
	if n := l.board.CellCount(); len(l.sunlit) != n {
		l.sunlit, l.sunStamp = make([]cellSunlit, n), 0
	} else if key == l.sunFor && l.sunStamp != 0 {
		return
	}
	l.sunFor = key
	if l.sunStamp++; l.sunStamp == 0 { // wrapped round: old results would pass for new
		clear(l.sunlit)
		l.sunStamp = 1
	}
	l.stale = true
}

// sunlitOf is how much sun reaches each corner of c's top, the tile's box x0..x1, y0..y1: all of
// it unless the terrain between the corner and the sun — the ground and what stands on it — rises
// above the line towards the sun.
func (l *dresser) sunlitOf(c board.CellID, x0, y0, x1, y1 float32) [4]float32 {
	i, _ := l.ordinal(c)
	s := &l.sunlit[i]
	if s.stamp == l.sunStamp {
		return s.corners
	}
	if l.stale {
		l.measureHighest(l.camera)
		l.stale = false
	}
	top := l.topOf(c).z
	for k, p := range [4][2]float32{{x0, y0}, {x1, y0}, {x0, y1}, {x1, y1}} {
		s.corners[k] = l.sunReaches(p[0], p[1], top[k])
	}
	s.stamp = l.sunStamp
	return s.corners
}

// shadowReach is how many cells towards the sun the terrain may cast a shadow from.
const shadowReach = 16

// sunReaches is 1 when the sun reaches the point (x, y) at height z, 0 when the terrain hides it:
// walked towards the sun a quarter cell at a time, over the tops of the cells as this frame read
// them, until the line to the sun rises above the highest top about.
func (l *dresser) sunReaches(x, y, z float32) float32 {
	sun := l.lighted.Dir
	across := float32(math.Hypot(float64(sun[0]), float64(sun[1])))
	switch {
	case sun[2] <= 0:
		return 0 // the sun is down
	case across == 0:
		return 1 // straight overhead, nothing casts a shadow
	}
	size := float32(l.sq.Cell)
	step := size / 4
	dx, dy, rise := sun[0]/across*step, sun[1]/across*step, sun[2]/across*step
	const eps = 0.01
	for k := 1; k <= 4*shadowReach; k++ {
		px, py, pz := x+dx*float32(k), y+dy*float32(k), z+rise*float32(k)
		if pz > l.highest {
			return 1 // above everything that could stand in the way
		}
		fx, fy := px/size, py/size
		cx, cy := math.Floor(float64(fx)), math.Floor(float64(fy))
		c, ok := l.cellAt(int64(cx), int64(cy))
		if !ok {
			return 1 // off the board nothing stands
		}
		// the top over the point, between the cell's corners
		t := l.topOf(c).z
		u, v := fx-float32(cx), fy-float32(cy)
		top := (t[0]*(1-u)+t[1]*u)*(1-v) + (t[2]*(1-u)+t[3]*u)*v
		if top > pz+eps {
			return 0
		}
	}
	return 1
}

// measureHighest finds the highest top of the cells within shadowReach of cam's view towards the
// sun: nothing higher can cast a shadow into it.
func (l *dresser) measureHighest(cam camera.Camera) {
	b := cam.Bounds()
	reach := float64(shadowReach) * min(l.cellW, l.cellH)
	sun := l.lighted.Dir
	if sun[0] > 0 {
		b.BottomRight.X += reach
	} else if sun[0] < 0 {
		b.TopLeft.X -= reach
	}
	if sun[1] > 0 {
		b.BottomRight.Y += reach
	} else if sun[1] < 0 {
		b.TopLeft.Y -= reach
	}
	l.highest = float32(math.Inf(-1))
	l.board.CellsUnder(b, func(c board.CellID) {
		for _, z := range l.topOf(c).z {
			l.highest = max(l.highest, z)
		}
	})
}
