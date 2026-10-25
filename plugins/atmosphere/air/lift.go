package air

import (
	"math"

	"github.com/kjkrol/gram/internal/f32"
)

// Land is the ground under the air masses as the air feels it: in squares of BoardCell over the
// world, the highest ground in each, world units up, and what the places in it wring out of the
// air over them, the most of each: the share of the low clouds falling (Falls) and the share of
// the air's damp gathering into low clouds a second (Gathers). A walk over the ground notes its
// places (Note) and turns them in once it has gone all over (Turn).
type Land struct {
	across, down int
	now, next    []landSquare
}

type landSquare struct{ height, falls, gathers float32 }

// NewLand is the land under a world width by height world units, bare till its first Turn.
func NewLand(width, height float32) *Land {
	across := int(math.Ceil(float64(width/BoardCell))) + 1
	down := int(math.Ceil(float64(height/BoardCell))) + 1
	return &Land{across: across, down: down, now: make([]landSquare, across*down),
		next: make([]landSquare, across*down)}
}

// Note has the place at the world point (x, y) stand height world units up and wring falls and
// gathers out of the air over it, from the next Turn.
func (l *Land) Note(x, y, height, falls, gathers float32) {
	i, j := int(math.Floor(float64(x/BoardCell))), int(math.Floor(float64(y/BoardCell)))
	if i < 0 || j < 0 || i >= l.across || j >= l.down {
		return
	}
	s := &l.next[j*l.across+i]
	s.height = max(s.height, height)
	s.falls = max(s.falls, falls)
	s.gathers = max(s.gathers, gathers)
}

// Turn has the places noted since the last Turn stand for the land.
func (l *Land) Turn() {
	l.now, l.next = l.next, l.now
	clear(l.next)
}

// At is the land at the world point (x, y): how high it stands and what it wrings out of the air
// over it, blended between the squares round it; bare past the world.
func (l *Land) At(x, y float32) (height, falls, gathers float32) {
	gx, gy := x/BoardCell-0.5, y/BoardCell-0.5
	i, j := int(math.Floor(float64(gx))), int(math.Floor(float64(gy)))
	fx, fy := gx-float32(i), gy-float32(j)
	var s [4]landSquare
	for k, at := range [4][2]int{{i, j}, {i + 1, j}, {i, j + 1}, {i + 1, j + 1}} {
		if at[0] >= 0 && at[1] >= 0 && at[0] < l.across && at[1] < l.down {
			s[k] = l.now[at[1]*l.across+at[0]]
		}
	}
	return blend(s[0].height, s[1].height, s[2].height, s[3].height, fx, fy),
		blend(s[0].falls, s[1].falls, s[2].falls, s[3].falls, fx, fy),
		blend(s[0].gathers, s[1].gathers, s[2].gathers, s[3].gathers, fx, fy)
}

// Lift has the land work on the air masses over the world for dt seconds of the weather w: the
// low clouds held back and piling up before ground that stands up into them, rain wrung out and
// damp gathered into low clouds where the land says, the rain easing back down past it to
// falls(state), its weather's own.
func (d *Dome) Lift(w Weather, land *Land, falls func(state int16) float32, dt float32) {
	if len(d.levels) == 0 || dt <= 0 {
		return
	}
	l := &d.levels[0]
	h := &d.held
	h.read(d, l, w, land, dt)
	if w.Wind != ([2]float32{}) {
		h.hold(l, w)
	}
	wring := 1 - float32(math.Exp(-float64(dt)/wringTime))
	if cap(h.changed) < len(l.patches) {
		h.changed = make([]bool, len(l.patches))
	}
	h.changed = h.changed[:len(l.patches)]
	clear(h.changed)
	for k := range h.cells {
		e := &h.cells[k]
		c, was := &e.p.Cells[e.i], e.p.Cells[e.i]
		low := e.low
		if e.out != 0 || e.in != 0 {
			own := max(low-e.out, 0)
			if e.in > 0 {
				c.Damp = (c.Damp*own + e.damp) / (own + e.in)
			}
			low = own + e.in
			w.setLow(c, low)
		}
		if e.gather > 0 && c.Damp > 0 {
			into := min(e.gather*c.Damp*dt, c.Damp, 1-low)
			low += into
			c.Damp -= into
			w.setLow(c, low)
		}
		own := falls(e.p.States[e.i])
		if wrung := e.rain * low * smoothstep(0, wrungDamp, c.Damp); wrung > own && !w.Dry {
			c.Falls += (wrung - c.Falls) * wring
			c.Damp = max(c.Damp-c.Falls*dt/drainTime, 0)
		} else if c.Falls > own {
			c.Falls += (own - c.Falls) * wring
		}
		if *c != was {
			e.p.Version++
			h.changed[e.slot] = true
		}
	}
	measured := false
	for slot, changed := range h.changed {
		if changed {
			l.measure(slot)
			measured = true
		}
	}
	if measured {
		d.gather()
	}
}

// seconds the rain a place wrings out takes to set in and to stop; the damp under which it
// thins; seconds a mass raining as hard as it can takes to rain all its damp out; how far up to
// the low clouds' base ground begins to hold them back, as a share of it
const (
	wringTime = 2.0
	wrungDamp = 0.2
	drainTime = 30.0
	holdFrom  = 0.7
)

// the dome's finest level as the land works on it in a step, its cells in the order kept: where
// each is held, its low clouds, what the land under it wrings out, what of its low clouds it sends
// upwind (out), keeps of what comes in from downwind (in) and the damp that brings
type holding struct {
	w       int
	cells   []held
	room    []float32 // of what comes in, how much a cell has room for
	changed []bool    // by slot
	bases   []float32 // the low clouds' base every baseEvery cells each way, from the first kept
}

// how many cells apart the low clouds' base is worked out, read between: it changes over many
const baseEvery = 4

type held struct {
	p                 *Patch
	i, slot           int32
	low, rain, gather float32
	out, in, damp     float32
	upwind            [2]int32 // across and along, -1 for none
}

// the cells of l as they stand under the land, and as much of each one's low clouds as the wind
// carries past ground standing up into them in dt (all of it where it reaches their base)
func (h *holding) read(d *Dome, l *domeLevel, w Weather, land *Land, dt float32) {
	h.w = l.hi[0] - l.lo[0] + 1
	n := h.w * (l.hi[1] - l.lo[1] + 1)
	if cap(h.cells) < n {
		h.cells, h.room = make([]held, n), make([]float32, n)
	}
	h.cells, h.room = h.cells[:n], h.room[:n]
	speed := float32(math.Hypot(float64(w.Wind[0]), float64(w.Wind[1])))
	lowest := holdFrom * w.Base(Low)
	ux, uy := -int(sign(w.Wind[0])), -int(sign(w.Wind[1]))
	if speed > 0 {
		h.lay(d, l, w)
	}
	for slot, p := range l.patches {
		g0, g1, at, ok := l.cells(slot)
		for gy := g0[1]; ok && gy <= g1[1]; gy++ {
			row := (gy-at[1])*PatchCells - at[0]
			y := (float32(gy)+0.5)*l.cell + d.drift[1]
			for gx := g0[0]; gx <= g1[0]; gx++ {
				x := (float32(gx)+0.5)*l.cell + d.drift[0]
				k := (gy-l.lo[1])*h.w + gx - l.lo[0]
				e := &h.cells[k]
				*e = held{p: p, i: int32(row + gx), slot: int32(slot), upwind: [2]int32{-1, -1}}
				e.low = w.lowOf(p.Cells[e.i])
				var height float32
				height, e.rain, e.gather = land.At(x, y)
				if height > lowest && speed > 0 && e.low > 0 {
					base := h.base(l, gx, gy)
					e.out = min(smoothstep(holdFrom*base, base, height)*speed*dt/l.cell, 1) * e.low
				}
				if tx := gx + ux; ux != 0 && tx >= l.lo[0] && tx <= l.hi[0] {
					e.upwind[0] = int32(k + ux)
				}
				if ty := gy + uy; uy != 0 && ty >= l.lo[1] && ty <= l.hi[1] {
					e.upwind[1] = int32(k + uy*h.w)
				}
			}
		}
	}
}

// the low clouds' bases over l, every baseEvery cells
func (h *holding) lay(d *Dome, l *domeLevel, w Weather) {
	across, down := (l.hi[0]-l.lo[0])/baseEvery+2, (l.hi[1]-l.lo[1])/baseEvery+2
	if cap(h.bases) < across*down {
		h.bases = make([]float32, across*down)
	}
	h.bases = h.bases[:across*down]
	for j := range down {
		for i := range across {
			x, y := l.middle(l.lo[0]+i*baseEvery, l.lo[1]+j*baseEvery, d.drift)
			h.bases[j*across+i], _ = w.LowBand(x, y)
		}
	}
}

// the low clouds' base over the cell (gx, gy), read between the four worked out round it
func (h *holding) base(l *domeLevel, gx, gy int) float32 {
	across := (l.hi[0]-l.lo[0])/baseEvery + 2
	fx, fy := gx-l.lo[0], gy-l.lo[1]
	i, j := fx/baseEvery, fy/baseEvery
	tx, ty := float32(fx%baseEvery)/baseEvery, float32(fy%baseEvery)/baseEvery
	at := j*across + i
	return blend(h.bases[at], h.bases[at+1], h.bases[at+across], h.bases[at+across+1], tx, ty)
}

// The low clouds each cell sends go to the cells upwind, which keep them as far as they hold less
// than a whole sky of them: in the world the clouds over a ridge stand still, or slow down, and
// pile up before it.
func (h *holding) hold(l *domeLevel, w Weather) {
	across := f32.Abs(w.Wind[0]) / (f32.Abs(w.Wind[0]) + f32.Abs(w.Wind[1]))
	share := [2]float32{across, 1 - across}
	for k := range h.cells {
		e := &h.cells[k]
		for way, to := range e.upwind {
			if e.out > 0 && to >= 0 {
				h.cells[to].in += e.out * share[way]
			}
		}
	}
	for k := range h.cells {
		e := &h.cells[k]
		h.room[k] = 1
		if e.in > 0 {
			h.room[k] = min((1-e.low)/e.in, 1)
		}
		e.in = 0
	}
	for k := range h.cells {
		e := &h.cells[k]
		if e.out <= 0 {
			continue
		}
		damp, sent := e.p.Cells[e.i].Damp, float32(0)
		for way, to := range e.upwind {
			if to >= 0 {
				moved := e.out * share[way] * h.room[to]
				h.cells[to].in += moved
				h.cells[to].damp += moved * damp
				sent += moved
			}
		}
		e.out = sent
	}
}

func sign(v float32) float32 {
	switch {
	case v > 0:
		return 1
	case v < 0:
		return -1
	}
	return 0
}

func smoothstep(e0, e1, v float32) float32 {
	if e1 <= e0 {
		if v >= e1 {
			return 1
		}
		return 0
	}
	t := clamp01((v - e0) / (e1 - e0))
	return t * t * (3 - 2*t)
}

// the cover of the low clouds of the mass m: in one layer, all of it
func (w Weather) lowOf(m Mass) float32 {
	if w.OneLayer {
		return clamp01(m.Cover)
	}
	return m.Layer(Low).Cover
}

// the mass c with low the cover of its low clouds
func (w Weather) setLow(c *Mass, low float32) {
	if w.OneLayer {
		c.Cover = clamp01(low)
		return
	}
	*c = c.WithLow(low)
}
