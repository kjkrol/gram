package board

import (
	"image/color"
	"math"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/goke/v3"
	"github.com/kjkrol/gram/camera"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/gram/render"
)

var colorGridLine = color.RGBA{R: 20, G: 20, B: 20, A: 120}

// RenderState is the board renderer's live display toggles.
type RenderState struct {
	ShowGridLines bool
}

// ToggleShowGridLines flips whether grid lines are drawn.
func (r *RenderState) ToggleShowGridLines() { r.ShowGridLines = !r.ShowGridLines }

// Renderer is the render.Source of Board's cells: each visible cell laid on the screen by the
// board's Look, and the grid when it is on.
type Renderer struct {
	board   *Board
	look    func() Look
	sun     func() world.Sun
	weather func() world.Weather // the air over the world; nil, a calm clear day
	lighted world.Sun            // the sun of the frame being drawn
	lamp    world.Lamp           // lighted, ready to light every corner
	camera  camera.Camera        // the one of the frame being drawn
	atlas   render.AtlasSource
	cellW   float64
	cellH   float64
	state   *RenderState
	outline []geom.Vec
	ways    []WayPiece   // the pieces of the last tile's Way
	blends  []BlendPiece // the last tile's Blends
	tile    Tile
	// seen marks by ordinal the cells a frame has visited on a grid walked by sampling; stamp is the frame's mark.
	seen  []uint32
	stamp uint32
	// tops holds by ordinal what a Compose has read of a cell, good while its stamp is topStamp.
	tops     []cellTop
	topStamp uint32
	// sunlit holds by ordinal how much sun reaches each corner of a cell's top, good while its
	// stamp is sunStamp: as long as neither the terrain nor the sun changes.
	shadows  bool
	sunlit   []cellSunlit
	sunStamp uint32
	sunFor   sunKey
	highest  float32 // the highest top a shadow may come from, measured when the shadows go stale
	stale    bool    // the shadows went stale this frame: highest is still to be measured
	// shores holds by corner of a square grid, row by row, the way to the shore from it, good
	// while its stamp is shoreStamp: as long as the terrain does not change.
	shores     []cornerShore
	shoreStamp uint32
	shoreFor   uint64
}

// cornerShore is the shore as one corner of the grid sees it.
type cornerShore struct {
	corner render.ShoreCorner
	stamp  uint32
}

// cellSunlit is how much of the sun reaches each corner of a cell's top, 0 to 1.
type cellSunlit struct {
	corners [4]float32
	stamp   uint32
}

// sunKey is what the shadows depend on: the terrain as it stands and the sun.
type sunKey struct {
	version uint64
	sun     world.Sun
}

// cellTop is a cell as one Compose reads it once: its corners with its kind standing on them, the
// ground's corners under it, its ground level and its sprite.
type cellTop struct {
	z      [4]float32
	ground [4]float32
	alt    float32
	shine  float32
	flow   float32
	sway   float32
	way    Way
	spread float32
	raised bool // something stands a Height over the ground
	under  bool
	sprite render.SpriteID
	stamp  uint32
}

// minGridCell is how many pixels a cell must span on screen for the grid to be drawn over it.
const minGridCell = 6

func abs32(v float32) float32 {
	if v < 0 {
		return -v
	}
	return v
}

var _ render.Source = (*Renderer)(nil)

// gridTier puts the lines of a grid other than square over every tile and under whatever stands on
// them.
const gridTier = render.Ground + 10

func newRenderer(board *Board, atlas render.AtlasSource, state *RenderState, look func() Look, sun func() world.Sun) *Renderer {
	w, h := board.CellBounds()
	r := &Renderer{board: board, atlas: atlas, cellW: w, cellH: h, state: state, look: look, sun: sun, shadows: true}
	r.tile.r, r.tile.Atlas = r, atlas
	return r
}

func (l *Renderer) Init(*goke.SysInit) {}

// Compose hands the board's Look every cell under cam, and the grid when it is on: on a square
// grid the Look outlines each tile along its own edges, on any other the cells' outlines are
// drawn as lines.
func (l *Renderer) Compose(f *render.Frame, cam camera.Camera) {
	l.camera = cam
	l.lighted = l.sun()
	l.lamp = l.lighted.Lamp()
	f.Daylight(l.lighted.Daylight())
	if l.weather != nil {
		f.Weather(l.weather().Frame())
	}
	l.nextSunlit()
	l.nextShores()
	look := l.look()
	grid := l.gridShown()
	square := l.board.square != nil
	l.tile.Outlined = grid && square
	l.nextTops()
	l.eachVisible(func(c CellID) {
		center := l.board.CellCenter(c)
		t := &l.tile
		t.ID = c
		t.X0, t.Y0 = float32(center.X-l.cellW/2), float32(center.Y-l.cellH/2)
		t.X1, t.Y1 = float32(center.X+l.cellW/2), float32(center.Y+l.cellH/2)
		look.Cell(f, cam, t)
		if grid && !square {
			l.outlineLines(f, c)
		}
	})
}

// gridShown reports whether the grid is on and its cells are large enough on screen to read.
func (l *Renderer) gridShown() bool {
	return l.state.ShowGridLines && float32(min(l.cellW, l.cellH))*l.camera.Zoom() >= minGridCell
}

// outlineLines draws c's outline on the ground at its level, leaving out an edge that straddles a
// wrap seam — the cells either side draw its images — each edge at the depth of its nearer end.
func (l *Renderer) outlineLines(f *render.Frame, c CellID) {
	alt := float32(l.board.Altitude(c))
	l.outline = l.board.CellOutline(c, l.outline[:0])
	reach := float32(l.cellW+l.cellH) * l.camera.Zoom()
	for i, p := range l.outline {
		q := l.outline[(i+1)%len(l.outline)]
		px, py, qx, qy := float32(p.X), float32(p.Y), float32(q.X), float32(q.Y)
		ax, ay := l.camera.Project(px, py, alt)
		bx, by := l.camera.Project(qx, qy, alt)
		if abs32(bx-ax) > reach || abs32(by-ay) > reach {
			continue
		}
		f.Line(gridTier, max(l.camera.Depth(px, py, alt), l.camera.Depth(qx, qy, alt)), ax, ay, bx, by, 1, colorGridLine)
	}
}

// nextTops starts a Compose: every cell read before is read anew.
func (l *Renderer) nextTops() {
	if n := l.board.CellCount(); len(l.tops) != n {
		l.tops, l.topStamp = make([]cellTop, n), 0
	}
	if l.topStamp++; l.topStamp == 0 { // wrapped round: old reads would pass for new
		clear(l.tops)
		l.topStamp = 1
	}
}

// topOf is c as this Compose sees it, read from the board the first time it is asked for: the
// ground's corners on a sloped grid, its level everywhere on a flat one, raised by the kind's Height.
func (l *Renderer) topOf(c CellID) *cellTop {
	i, _ := l.board.ordinal(c)
	t := &l.tops[i]
	if t.stamp == l.topStamp {
		return t
	}
	kind := l.board.kindOf(c)
	r := l.board.Relief(c)
	rise := float32(kind.Height)
	t.alt, t.shine, t.flow, t.sway, t.sprite, t.stamp = float32(r.Level()), float32(kind.Shine), float32(kind.Flow), float32(kind.Sway), kind.SpriteID, l.topStamp
	t.way, t.spread, t.raised, t.under = l.board.Way(c), float32(kind.Spread), kind.Height > 0, kind.Under
	if l.board.sloped() {
		t.ground = r.Corners
	} else {
		t.ground = [4]float32{t.alt, t.alt, t.alt, t.alt}
	}
	for k := range t.z {
		t.z[k] = t.ground[k] + rise
	}
	return t
}

// eachVisible calls fn once for every cell under the camera's bounds: on a square grid straight
// from the rows and columns, on any other by sampling every half cell.
func (l *Renderer) eachVisible(fn func(c CellID)) {
	bounds := l.camera.Bounds()
	if l.board.square != nil {
		l.board.CellsUnder(bounds, fn)
		return
	}
	step := min(l.cellW, l.cellH) / 2
	if step <= 0 {
		step = 1
	}
	if n := l.board.CellCount(); len(l.seen) != n {
		l.seen, l.stamp = make([]uint32, n), 0
	}
	if l.stamp++; l.stamp == 0 { // wrapped round: old marks would pass for new
		clear(l.seen)
		l.stamp = 1
	}
	for y := bounds.TopLeft.Y; y < bounds.BottomRight.Y+step; y += step {
		for x := bounds.TopLeft.X; x < bounds.BottomRight.X+step; x += step {
			c, ok := l.board.CellAt(geom.NewVec(x, y))
			if !ok {
				continue
			}
			i, ok := l.board.ordinal(c)
			if !ok || l.seen[i] == l.stamp {
				continue
			}
			l.seen[i] = l.stamp
			fn(c)
		}
	}
}

// nextSunlit starts a Compose: when the terrain or the sun has changed since the last one, every
// shadow is worked out anew as it comes into sight.
func (l *Renderer) nextSunlit() {
	key := sunKey{version: l.board.Version(), sun: l.lighted}
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
func (l *Renderer) sunlitOf(c CellID, x0, y0, x1, y1 float32) [4]float32 {
	i, _ := l.board.ordinal(c)
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
func (l *Renderer) sunReaches(x, y, z float32) float32 {
	sun := l.lighted.Dir
	across := float32(math.Hypot(float64(sun[0]), float64(sun[1])))
	switch {
	case sun[2] <= 0:
		return 0 // the sun is down
	case across == 0:
		return 1 // straight overhead, nothing casts a shadow
	}
	sq := l.board.square
	size := float32(sq.CellSize)
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
		c, ok := l.board.squareCell(int64(cx), int64(cy))
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
func (l *Renderer) measureHighest(cam camera.Camera) {
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
	l.board.CellsUnder(b, func(c CellID) {
		for _, z := range l.topOf(c).z {
			l.highest = max(l.highest, z)
		}
	})
}

// nextShores starts a Compose: when the terrain has changed since the last one, every shore is
// worked out anew as it comes into sight.
func (l *Renderer) nextShores() {
	sq := l.board.square
	if sq == nil {
		return
	}
	version := l.board.Version()
	if n := int(sq.Width+1) * int(sq.Height+1); len(l.shores) != n {
		l.shores, l.shoreStamp = make([]cornerShore, n), 0
	} else if version == l.shoreFor && l.shoreStamp != 0 {
		return
	}
	l.shoreFor = version
	if l.shoreStamp++; l.shoreStamp == 0 { // wrapped round: old results would pass for new
		clear(l.shores)
		l.shoreStamp = 1
	}
}

// shoreReach is how many cells from a shore its waves turn to face it.
const shoreReach = 3

// shoreOf is the shore from each corner of the box x0..x1, y0..y1 of a cell: open water everywhere
// off a square grid.
func (l *Renderer) shoreOf(x0, y0, x1, y1 float32) render.Shore {
	if l.board.square == nil {
		return render.Shore{}
	}
	return render.Shore{l.shoreAt(x0, y0), l.shoreAt(x1, y0), l.shoreAt(x0, y1), l.shoreAt(x1, y1)}
}

// shoreAt is the shore from the grid's corner at (x, y), worked out the first time it is asked
// for since the terrain changed.
func (l *Renderer) shoreAt(x, y float32) render.ShoreCorner {
	sq := l.board.square
	size := float32(sq.CellSize)
	gx, gy := int64(math.Round(float64(x/size))), int64(math.Round(float64(y/size)))
	slot, ok := cornerSlot(gx, int64(sq.Width), sq.WrapX)
	row, okY := cornerSlot(gy, int64(sq.Height), sq.WrapY)
	if !ok || !okY {
		return l.workShore(gx, gy, size)
	}
	s := &l.shores[row*(int64(sq.Width)+1)+slot]
	if s.stamp != l.shoreStamp {
		s.corner, s.stamp = l.workShore(gx, gy, size), l.shoreStamp
	}
	return s.corner
}

// cornerSlot is corner g of n cells along an axis, folded onto 0..n-1 when it wraps; false off the
// grid.
func cornerSlot(g, n int64, wrap bool) (int64, bool) {
	if wrap {
		return (g%n + n) % n, true
	}
	return g, g >= 0 && g <= n
}

// workShore is the way from the grid's corner gx, gy to the nearest cell within shoreReach that
// does not shine, how far it is and how near; on the shore itself, the way into the land it
// touches.
func (l *Renderer) workShore(gx, gy int64, size float32) render.ShoreCorner {
	reach := shoreReach * size
	px, py := float32(gx)*size, float32(gy)*size
	best, bx, by := reach*reach, float32(0), float32(0)
	var ax, ay float32 // on the shore: towards the middles of the land cells touching the corner
	look := func(cx, cy int64) {
		c, ok := l.board.squareCell(cx, cy)
		if !ok || l.topOf(c).shine > 0 {
			return
		}
		x0, y0 := float32(cx)*size, float32(cy)*size
		dx, dy := min(max(px, x0), x0+size)-px, min(max(py, y0), y0+size)-py
		d := dx*dx + dy*dy
		if d == 0 {
			ax, ay = ax+x0+size/2-px, ay+y0+size/2-py
		}
		if d < best {
			best, bx, by = d, dx, dy
		}
	}
	// ring r is the cells r away from the four touching the corner, none of them nearer than r cells
	for r := int64(0); r <= shoreReach; r++ {
		if near := float32(r) * size; near*near >= best {
			break
		}
		lo, hi := -1-r, r
		for cx := lo; cx <= hi; cx++ {
			look(gx+cx, gy+lo)
			look(gx+cx, gy+hi)
		}
		for cy := lo + 1; cy < hi; cy++ {
			look(gx+lo, gy+cy)
			look(gx+hi, gy+cy)
		}
	}
	if best >= reach*reach {
		return render.ShoreCorner{Dist: reach}
	}
	d := float32(math.Sqrt(float64(best)))
	if d == 0 {
		bx, by = ax, ay
	}
	corner := render.ShoreCorner{Dist: d, Near: 1 - d/reach}
	if n := float32(math.Hypot(float64(bx), float64(by))); n > 0 {
		corner.X, corner.Y = bx/n, by/n
	}
	return corner
}
