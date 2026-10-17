package air

import (
	"image"
	"math"
	"slices"

	"github.com/kjkrol/gram/render"
)

// Mass is the weather over a piece of the world: how much of the sky its clouds cover, from torn
// shreds to big heaps (Billow), how much falls from them, rain or snow as the air's temperature
// has it (Falls), and how damp it is; each 0 to 1.
type Mass struct{ Cover, Billow, Falls, Damp float32 }

// PatchCells is how many cells a Patch has along each side.
const PatchCells = 16

// Patch is a patch of the air masses over a world and its sky, an entity of its own: PatchCells
// by PatchCells cells of one Level of the Dome, at its Slot in the level's ring, each with its
// Mass and the State of the weather it came from. Version grows with every change.
type Patch struct {
	Level, Slot int32
	Version     uint32
	Cells       [PatchCells * PatchCells]Mass
	States      [PatchCells * PatchCells]int16
}

// Dome is the air masses over a world and its sky out to the horizon, drifting with the clouds:
// level 0 over the world in cells of BoardCell, each level further out in cells at least twice
// as large, every level a ring of Patches; a mass comes in where the wind blows from.
type Dome struct {
	levels []domeLevel
	center [2]float32
	drift  [2]float32
	most   Mass
	laid   bool
	width  int // the picture's
	height int
	held   holding
}

// BoardCell is the size of level 0's cells, in world units.
const BoardCell = 128

// the levels a dome has at most, and the band of a level's cells along its edge where it gives
// way to the next
const (
	maxLevels = 6
	domeBand  = 2
)

type domeLevel struct {
	cell    float32
	tiles   [2]int     // patches across and down
	inside  [4]float32 // the rect it is read in: x0, y0, x1, y1
	block   int        // where its cells begin across the picture
	lo, hi  [2]int     // the cells kept, in the frame the clouds drift in
	patches []*Patch   // by slot
	most    []Mass     // by slot, of its cells kept
}

// NewDome is the dome over a world width by height world units, its sky reaching reach world
// units out from the world's edge, in as many levels as that takes.
func NewDome(width, height, reach float32) *Dome {
	d := &Dome{center: [2]float32{width / 2, height / 2}}
	c := float32(BoardCell)
	d.add(c, [4]float32{-c, -c, width + c, height + c})
	for len(d.levels) < maxLevels {
		was := d.levels[len(d.levels)-1]
		half := max(d.center[0]-was.inside[0], d.center[1]-was.inside[1])
		if half >= max(d.center[0], d.center[1])+reach {
			break
		}
		// coarser at least twice, its rect taking the finer's and a band of its own
		for c *= 2; 21*c < half; c *= 2 {
		}
		h := 23 * c
		d.add(c, [4]float32{d.center[0] - h, d.center[1] - h, d.center[0] + h, d.center[1] + h})
	}
	for _, l := range d.levels {
		d.height = max(d.height, l.tiles[1]*PatchCells)
	}
	return d
}

func (d *Dome) add(cell float32, inside [4]float32) {
	span := float32(PatchCells) * cell
	l := domeLevel{cell: cell, inside: inside, block: d.width}
	for k := range 2 {
		// the cells kept reach a cell past the rect, so the four round any point in it are there
		extent := inside[k+2] - inside[k] + 2*cell
		l.tiles[k] = int(math.Ceil(float64(extent/span))) + 1
	}
	l.patches = make([]*Patch, l.tiles[0]*l.tiles[1])
	l.most = make([]Mass, len(l.patches))
	d.width += l.tiles[0] * PatchCells
	d.levels = append(d.levels, l)
}

// Slots are the patches the dome is made of, a level and a slot each.
func (d *Dome) Slots() [][2]int32 {
	var s [][2]int32
	for k, l := range d.levels {
		for i := range l.patches {
			s = append(s, [2]int32{int32(k), int32(i)})
		}
	}
	return s
}

// Hold has the dome keep its cells in p, at p's level and slot; one out of the dome is refused.
func (d *Dome) Hold(p *Patch) bool {
	if int(p.Level) >= len(d.levels) || p.Level < 0 {
		return false
	}
	l := &d.levels[p.Level]
	if int(p.Slot) >= len(l.patches) || p.Slot < 0 {
		return false
	}
	l.patches[p.Slot] = p
	return true
}

// Own has the dome keep its cells in patches of its own, for a dome held by no world.
func (d *Dome) Own() {
	for _, s := range d.Slots() {
		d.Hold(&Patch{Level: s[0], Slot: s[1]})
	}
}

func floorDiv(a, b int) int {
	q := a / b
	if a%b != 0 && (a < 0) != (b < 0) {
		q--
	}
	return q
}

func (l *domeLevel) cellAt(gx, gy int) (*Patch, int) {
	tx, ty := floorDiv(gx, PatchCells), floorDiv(gy, PatchCells)
	p := l.patches[mod(ty, l.tiles[1])*l.tiles[0]+mod(tx, l.tiles[0])]
	return p, mod(gy, PatchCells)*PatchCells + mod(gx, PatchCells)
}

func mod(i, n int) int { return (i%n + n) % n }

// the slot of the patch the cell (gx, gy) is kept in
func (l *domeLevel) slotOf(gx, gy int) int {
	tx, ty := floorDiv(gx, PatchCells), floorDiv(gy, PatchCells)
	return mod(ty, l.tiles[1])*l.tiles[0] + mod(tx, l.tiles[0])
}

// the cells kept at drift: those whose middles lie within a cell of the rect
func (l *domeLevel) span(drift [2]float32) (lo, hi [2]int) {
	for k := range 2 {
		lo[k] = int(math.Ceil(float64((l.inside[k]-l.cell-drift[k])/l.cell - 0.5)))
		hi[k] = int(math.Floor(float64((l.inside[k+2]+l.cell-drift[k])/l.cell - 0.5)))
	}
	return lo, hi
}

// the middle of the cell (gx, gy) in the world
func (l *domeLevel) middle(gx, gy int, drift [2]float32) (float32, float32) {
	return (float32(gx)+0.5)*l.cell + drift[0], (float32(gy)+0.5)*l.cell + drift[1]
}

// the cells kept in the patch at slot, g0 to g1, and the cell it begins at; ok false when none
func (l *domeLevel) cells(slot int) (g0, g1, at [2]int, ok bool) {
	in := [2]int{slot % l.tiles[0], slot / l.tiles[0]}
	for k := range 2 {
		first := floorDiv(l.lo[k], PatchCells)
		at[k] = (first + mod(in[k]-first, l.tiles[k])) * PatchCells
		g0[k], g1[k] = max(l.lo[k], at[k]), min(l.hi[k], at[k]+PatchCells-1)
		if g0[k] > g1[k] {
			return g0, g1, at, false
		}
	}
	return g0, g1, at, true
}

func (l *domeLevel) measure(slot int) {
	m := Mass{}
	if g0, g1, at, ok := l.cells(slot); ok {
		p := l.patches[slot]
		for gy := g0[1]; gy <= g1[1]; gy++ {
			row := (gy-at[1])*PatchCells - at[0]
			for gx := g0[0]; gx <= g1[0]; gx++ {
				c := &p.Cells[row+gx]
				m = Mass{max(m.Cover, c.Cover), max(m.Billow, c.Billow), max(m.Falls, c.Falls),
					max(m.Damp, c.Damp)}
			}
		}
	}
	l.most[slot] = m
}

// Carry moves the dome with the clouds, drifted by drift as the wind carried them, the cells
// coming in on every level taking the mass born there, at the horizon, or the coarser level's;
// laid anew, every cell takes the mass born where it is. A dome taken back from a save is
// carried on with laid false.
func (d *Dome) Carry(drift [2]float32, born func(x, y float32) (Mass, int16), laid bool) {
	d.drift = drift
	changed := false
	for k := len(d.levels) - 1; k >= 0; k-- {
		if !d.carry(k, born, laid || !d.laid) {
			continue
		}
		changed = true
		for slot := range d.levels[k].patches {
			d.levels[k].measure(slot)
		}
	}
	d.laid = true
	if changed {
		d.gather()
	}
}

// whether level k's window moved, its cells coming in taking their masses
func (d *Dome) carry(k int, born func(x, y float32) (Mass, int16), fresh bool) bool {
	l := &d.levels[k]
	lo, hi := l.span(d.drift)
	was0, was1 := l.lo, l.hi
	l.lo, l.hi = lo, hi
	if !fresh && lo == was0 && hi == was1 {
		return false
	}
	for gy := lo[1]; gy <= hi[1]; gy++ {
		for gx := lo[0]; gx <= hi[0]; gx++ {
			if !fresh && gx >= was0[0] && gx <= was1[0] && gy >= was0[1] && gy <= was1[1] {
				continue
			}
			x, y := l.middle(gx, gy, d.drift)
			var m Mass
			var s int16
			if fresh || k == len(d.levels)-1 {
				m, s = born(x, y)
			} else {
				m, s = d.levels[k+1].at(x, y, d.drift)
			}
			p, i := l.cellAt(gx, gy)
			p.Cells[i], p.States[i] = m, s
			p.Version++
		}
	}
	return true
}

// Resume has the dome carry on from where a save left it, the clouds drifted by drift.
func (d *Dome) Resume(drift [2]float32) {
	d.drift = drift
	for k := range d.levels {
		l := &d.levels[k]
		l.lo, l.hi = l.span(drift)
		for slot := range l.patches {
			l.measure(slot)
		}
	}
	d.laid = true
	d.gather()
}

// Settle has the masses of a part of the dome's patches, part of parts, go the share of the
// way share says to the ones to says, taking their states unless the share is 0; parts 1 settles
// all of them.
func (d *Dome) Settle(to func(x, y float32) (Mass, int16), share func(x, y float32) float32,
	part, parts int) {
	n := 0
	for li := range d.levels {
		l := &d.levels[li]
		for slot, p := range l.patches {
			if n++; (n-1)%parts != part {
				continue
			}
			if l.settle(slot, p, d.drift, to, share) {
				p.Version++
				l.measure(slot)
			}
		}
	}
	d.gather()
}

// whether a mass of the patch at slot changed, settling
func (l *domeLevel) settle(slot int, p *Patch, drift [2]float32,
	to func(x, y float32) (Mass, int16), share func(x, y float32) float32) bool {
	g0, g1, at, ok := l.cells(slot)
	changed := false
	for gy := g0[1]; ok && gy <= g1[1]; gy++ {
		y := (float32(gy)+0.5)*l.cell + drift[1]
		row := (gy-at[1])*PatchCells - at[0]
		for gx := g0[0]; gx <= g1[0]; gx++ {
			x := (float32(gx)+0.5)*l.cell + drift[0]
			k := share(x, y)
			if k <= 0 {
				continue
			}
			m, s := to(x, y)
			i := row + gx
			c := &p.Cells[i]
			// within half a step of the picture's 8 bits it has settled
			if p.States[i] == s && max(abs(m.Cover-c.Cover), abs(m.Billow-c.Billow),
				abs(m.Falls-c.Falls), abs(m.Damp-c.Damp)) < 1.0/512 {
				continue
			}
			c.Cover += (m.Cover - c.Cover) * k
			c.Billow += (m.Billow - c.Billow) * k
			c.Falls += (m.Falls - c.Falls) * k
			c.Damp += (m.Damp - c.Damp) * k
			p.States[i] = s
			changed = true
		}
	}
	return changed
}

func abs(v float32) float32 { return max(v, -v) }

// the most of every level's patches
func (d *Dome) gather() {
	d.most = Mass{}
	for _, l := range d.levels {
		for _, m := range l.most {
			d.most = Mass{max(d.most.Cover, m.Cover), max(d.most.Billow, m.Billow),
				max(d.most.Falls, m.Falls), max(d.most.Damp, m.Damp)}
		}
	}
}

// Most is the most of each that any mass of the dome holds.
func (d *Dome) Most() Mass { return d.most }

// Reach is the rect of the world the dome holds masses over: x0, y0, x1, y1.
func (d *Dome) Reach() [4]float32 { return d.levels[len(d.levels)-1].inside }

// At is the mass over the world point (x, y) and the state of its weather: the finest level read
// there, given way to the next along its edge; past the dome, its edge.
func (d *Dome) At(x, y float32) (Mass, int16) {
	last := len(d.levels) - 1
	for k := range d.levels {
		l := &d.levels[k]
		in := min(min(x-l.inside[0], l.inside[2]-x), min(y-l.inside[1], l.inside[3]-y)) / l.cell
		if in < 0 {
			continue
		}
		m, s := l.at(x, y, d.drift)
		if in >= domeBand || k == last {
			return m, s
		}
		n, ns := d.levels[k+1].at(x, y, d.drift)
		t := in / domeBand
		if t < 0.5 {
			s = ns
		}
		return Mass{
			Cover:  n.Cover + (m.Cover-n.Cover)*t,
			Billow: n.Billow + (m.Billow-n.Billow)*t,
			Falls:  n.Falls + (m.Falls-n.Falls)*t,
			Damp:   n.Damp + (m.Damp-n.Damp)*t,
		}, s
	}
	l := &d.levels[last]
	x, y = min(max(x, l.inside[0]), l.inside[2]), min(max(y, l.inside[1]), l.inside[3])
	return l.at(x, y, d.drift)
}

// the level's four cells round (x, y) blended, the state the nearest's
func (l *domeLevel) at(x, y float32, drift [2]float32) (Mass, int16) {
	gx, gy := (x-drift[0])/l.cell-0.5, (y-drift[1])/l.cell-0.5
	i, j := int(math.Floor(float64(gx))), int(math.Floor(float64(gy)))
	fx, fy := gx-float32(i), gy-float32(j)
	p00, k00 := l.cellAt(i, j)
	p10, k10 := l.cellAt(i+1, j)
	p01, k01 := l.cellAt(i, j+1)
	p11, k11 := l.cellAt(i+1, j+1)
	m00, m10, m01, m11 := &p00.Cells[k00], &p10.Cells[k10], &p01.Cells[k01], &p11.Cells[k11]
	s := p00.States[k00]
	switch {
	case fx >= 0.5 && fy >= 0.5:
		s = p11.States[k11]
	case fx >= 0.5:
		s = p10.States[k10]
	case fy >= 0.5:
		s = p01.States[k01]
	}
	return Mass{
		Cover:  blend(m00.Cover, m10.Cover, m01.Cover, m11.Cover, fx, fy),
		Billow: blend(m00.Billow, m10.Billow, m01.Billow, m11.Billow, fx, fy),
		Falls:  blend(m00.Falls, m10.Falls, m01.Falls, m11.Falls, fx, fy),
		Damp:   blend(m00.Damp, m10.Damp, m01.Damp, m11.Damp, fx, fy),
	}, s
}

// the four corners' values blended at (fx, fy) between them
func blend(v00, v10, v01, v11, fx, fy float32) float32 {
	top := v00 + (v10-v00)*fx
	return top + (v01+(v11-v01)*fx-top)*fy
}

// the levels as the shaders read them (airAt): where each one's block begins in the world, the
// first cell it keeps, its cell and where its block begins across the picture; the rect it is
// read in
func (d *Dome) frame(level int) (a, c [4]float32) {
	if level >= len(d.levels) {
		return a, c
	}
	l := &d.levels[level]
	at := func(k int) float32 {
		return float32(float64(l.lo[k])*float64(l.cell) + float64(d.drift[k]))
	}
	return [4]float32{at(0), at(1), l.cell, float32(l.block)}, l.inside
}

// DomeImage is the dome's masses as the frame's image AirMasses, a block of pixels a level, its
// cells kept in order from its first, cover, billow, falls and damp in a pixel's channels; a
// level is written to the GPU as its window moves or a patch of it changes.
type DomeImage struct {
	img      *render.Image
	of       *Dome
	versions []uint32
	windows  [][2][2]int // a level's cells written: lo, hi
	levels   [][]byte    // a level's pixels
	pending  []int       // the levels to write
}

// Of is the picture of d, the levels changed since it was last written to it the next time a draw
// reads it.
func (i *DomeImage) Of(d *Dome) *render.Image {
	if i.of != d {
		if i.img != nil {
			i.img.Deallocate()
		}
		i.img, i.of = render.NewImage(d.width, d.height), d
		i.versions = i.versions[:0]
		i.windows = make([][2][2]int, len(d.levels))
		i.levels = make([][]byte, len(d.levels))
		i.pending = i.pending[:0]
		for k := range i.windows {
			i.windows[k][0][0] = 1 // none written: lo past hi
		}
	}
	queued := len(i.pending)
	n := 0
	for k := range d.levels {
		l := &d.levels[k]
		changed := i.windows[k] != [2][2]int{l.lo, l.hi}
		for _, p := range l.patches {
			if n == len(i.versions) {
				i.versions = append(i.versions, p.Version-1)
			}
			if p.Version != i.versions[n] {
				i.versions[n] = p.Version
				changed = true
			}
			n++
		}
		if changed {
			i.windows[k] = [2][2]int{l.lo, l.hi}
			i.put(k, l)
		}
	}
	if queued == 0 && len(i.pending) > 0 {
		i.img.Defer(i.write)
	}
	return i.img
}

// the level's pixels, copied out of the patches now: their memory is the world's
func (i *DomeImage) put(k int, l *domeLevel) {
	byte8 := func(v float32) byte { return byte(min(max(v, 0), 1)*255 + 0.5) }
	w, h := l.hi[0]-l.lo[0]+1, l.hi[1]-l.lo[1]+1
	if cap(i.levels[k]) < 4*w*h {
		i.levels[k] = make([]byte, 4*w*h)
	}
	pix := i.levels[k][:4*w*h]
	for gy := l.lo[1]; gy <= l.hi[1]; gy++ {
		for gx := l.lo[0]; gx <= l.hi[0]; gx++ {
			p, c := l.cellAt(gx, gy)
			m := &p.Cells[c]
			at := 4 * ((gy-l.lo[1])*w + gx - l.lo[0])
			pix[at], pix[at+1] = byte8(m.Cover), byte8(m.Billow)
			pix[at+2], pix[at+3] = byte8(m.Falls), byte8(m.Damp)
		}
	}
	i.levels[k] = pix
	if !slices.Contains(i.pending, k) {
		i.pending = append(i.pending, k)
	}
}

func (i *DomeImage) write(dst *render.Image) {
	for _, k := range i.pending {
		l := &i.of.levels[k]
		w := i.windows[k]
		r := image.Rect(l.block, 0, l.block+w[1][0]-w[0][0]+1, w[1][1]-w[0][1]+1)
		dst.SubImage(r).WritePixels(i.levels[k])
	}
	i.pending = i.pending[:0]
}
