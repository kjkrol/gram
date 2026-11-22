package mosaic

import (
	"math"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/gram/internal/f32"
	"github.com/kjkrol/mosaic/plane"
)

const (
	// the tolerances a road lies in full up to, with its own fine points up to detailed, and as
	// a ribbon from
	fullUpTo = 1
	detailed = 0.25
	plainly  = 1.5
	// how far apart a road's cells lie along it, in full and as a ribbon, how near its lines
	// keep to a bend's curve for the game and in the picture, and how long its last stretch
	// before a junction is
	fullApart   = 12
	ribbonApart = 24
	strays      = 0.5
	smooth      = 0.05
	lead        = 3.5
	// how far past a road's outer row the ground's own points give way
	margin = 6.5
	// the side of the plane's own buckets, a road's cell at the most
	bucket = 10
)

// originOf is the plane's origin of the n-th thing laid, of a kind: the kind in its low bits
func originOf(n int, k Kind) plane.Origin { return plane.Origin(n+1)<<3 | plane.Origin(k) }

func kindOf(tag uint64) Kind { return Kind(tag & 7) }

// build lays m over a ground width by height: its points as fine as its heights ask, every
// road in full for the fine tolerances and as a ribbon for the coarse
func build(m Map, width, height float32) *plane.Plane {
	if m.Apart == 0 {
		m.Apart = 2
	}
	if m.Rungs == 0 {
		m.Rungs = 6
	}
	edge := geom.NewAABB(geom.NewVec(0, 0), geom.NewVec(width, height))
	p := plane.New(edge, bucket, fullUpTo)
	p.Show(edge, float32(math.Inf(1)))
	ground := m.Ground
	if ground == nil {
		ground = func(geom.Vec) float32 { return 0 }
	}
	p.Put(plane.Terrain, plane.Relief(edge, m.Apart, m.Seed, ground, m.Rungs)...)
	b := builder{p: p, s: m.Section, grade: graded(ground), seed: m.Seed}
	meetings := make([]plane.Meeting, len(m.Junctions))
	for k, j := range m.Junctions {
		arms := make([]plane.Arm, len(j.Ways))
		for i, way := range j.Ways {
			arms[i] = plane.Arm{Away: unit(way), Half: b.half()}
		}
		meetings[k] = plane.Meet(j.At, arms, j.Radius, lead)
	}
	for n, r := range m.Roads {
		path, head, tail := trimmed(r.Path, m.Junctions, meetings)
		b.road(n, path, head, tail)
	}
	for k, j := range m.Junctions {
		b.junction(len(m.Roads)+k, j, meetings[k])
	}
	return p
}

type builder struct {
	p     *plane.Plane
	s     Section
	grade func(geom.Vec) float32
	seed  uint64
}

func (b builder) half() float32 { return float32(b.s.Lanes) * b.s.Lane / 2 }

// verge is how far from the kerb the verge's row lies: as far past the pavement's outer line
// as the pavement's own row lies short of it
func (b builder) verge() float32 { return 2*b.s.Walk - b.s.Lane/2 }

// bank is how far from the kerb the row closing the verge lies, the ground's own again: as far
// past the verge's outer line as the verge's row lies short of it
func (b builder) bank() float32 { return 2*(b.s.Walk+b.s.Verge) - b.verge() }

func (b builder) road(n int, path []geom.Vec, head, tail float32) {
	half := b.half()
	b.p.Mask(originOf(n, Carriageway), corridor(path, half+b.bank()+margin))
	full := plane.Line{
		Path: path, Spacing: fullApart, Fine: b.s.Lane, Within: strays, Smooth: smooth,
		Reach: half + b.bank(), Head: head, Tail: tail, Height: b.grade,
		Keep: fullUpTo, Detail: detailed,
	}
	full.Step = 1
	for i := 0; i < b.s.Lanes; i++ {
		full.Offsets = append(full.Offsets, (float32(i)+0.5)*b.s.Lane-half)
	}
	b.p.Lay(originOf(n, Carriageway), full)
	full.Step, full.Lift = 0, b.s.Kerb
	full.Offsets = []float32{-half - b.s.Lane/2, half + b.s.Lane/2}
	b.p.Lay(originOf(n, Pavement), full)
	full.Offsets = []float32{-half - b.verge(), half + b.verge()}
	b.p.Lay(originOf(n, Verge), full)
	full.Offsets, full.Lift, full.Drawn = []float32{-half - b.bank(), half + b.bank()}, 0, true
	b.p.Lay(originOf(n, Roadside), full)

	plain := plane.Line{
		Path: path, Spacing: ribbonApart, Within: strays, Reach: 1.5 * half,
		Head: head, Tail: tail, Height: b.grade, From: plainly,
	}
	plain.Offsets = []float32{-half / 2, half / 2}
	b.p.Lay(originOf(n, Carriageway), plain)
	plain.Offsets = []float32{-1.5 * half, 1.5 * half}
	b.p.Lay(originOf(n, Roadside), plain)
}

func (b builder) junction(n int, j Junction, m plane.Meeting) {
	half := b.half()
	hub := originOf(n, Crossing)
	// across each road's end from its lanes that run beside no kerb
	for k, way := range j.Ways {
		d := unit(way)
		for i := 1; i < b.s.Lanes-1; i++ {
			aside := (float32(i)+0.5)*b.s.Lane - half
			forth := m.Ends[k] - lead/2
			at := geom.NewVec(j.At.X+d.X*forth-d.Y*aside, j.At.Y+d.Y*forth+d.X*aside)
			b.p.Put(hub, plane.Point{At: at, Height: b.grade(at), Keep: fullUpTo, Step: 1})
		}
	}
	for k, path := range m.Corners {
		b.p.Mask(hub, m.Block(k, b.bank()+margin))
		full := plane.Line{
			Path: path, Spacing: fullApart, Fine: b.s.Lane, Within: strays, Smooth: smooth,
			Reach: b.bank(), Head: lead, Tail: lead, Height: b.grade,
			Keep: fullUpTo, Detail: detailed,
		}
		full.Offsets, full.Step = []float32{b.s.Lane / 2}, 1
		b.p.Lay(hub, full)
		full.Step, full.Lift = 0, b.s.Kerb
		full.Offsets = []float32{-b.s.Lane / 2}
		b.p.Lay(originOf(n, Pavement), full)
		full.Offsets = []float32{-b.verge()}
		b.p.Lay(originOf(n, Verge), full)
		full.Offsets, full.Lift, full.Drawn = []float32{-b.bank()}, 0, true
		b.p.Lay(originOf(n, Roadside), full)

		plain := plane.Line{
			Path: path, Spacing: ribbonApart, Within: strays, Reach: half / 2,
			Head: lead, Tail: lead, Height: b.grade, From: plainly,
		}
		plain.Offsets = []float32{half / 2}
		b.p.Lay(hub, plain)
		plain.Offsets = []float32{-half / 2}
		b.p.Lay(originOf(n, Roadside), plain)
	}
	outline := m.Outline()
	lo, hi := outline[0], outline[0]
	for _, v := range outline {
		lo = geom.NewVec(min(lo.X, v.X), min(lo.Y, v.Y))
		hi = geom.NewVec(max(hi.X, v.X), max(hi.Y, v.Y))
	}
	box := geom.NewAABB(lo, hi)
	fine := plane.Fill(box, 2.5, b.seed+11, b.grade, 0, fullUpTo)
	for i := range fine {
		fine[i].Step, fine[i].Keep = 1, min(fine[i].Keep, fullUpTo)
	}
	b.p.Put(hub, b.p.Inside(outline, fine, fullUpTo)...)
	coarse := plane.Fill(box, 6, b.seed+12, b.grade)
	for i := range coarse {
		coarse[i].From, coarse[i].Keep = plainly, float32(math.Inf(1))
	}
	b.p.Put(hub, b.p.Inside(outline, coarse, 2*plainly)...)
}

// graded is a road's height where it passes at: the ground's, evened over a stretch round it
func graded(ground func(geom.Vec) float32) func(geom.Vec) float32 {
	return func(at geom.Vec) float32 {
		var sum float32
		n := 0
		for dx := float32(-12); dx <= 12; dx += 6 {
			for dy := float32(-12); dy <= 12; dy += 6 {
				sum += ground(geom.NewVec(at.X+dx, at.Y+dy))
				n++
			}
		}
		return sum / float32(n)
	}
}

func unit(v geom.Vec) geom.Vec {
	l := f32.Sqrt(v.X*v.X + v.Y*v.Y)
	return geom.NewVec(v.X/l, v.Y/l)
}

// trimmed is a road's path short of the junctions its ends stand at, and how long its first
// and last stretch must be to stand across a junction's end
func trimmed(
	path []geom.Vec, js []Junction, ms []plane.Meeting,
) (out []geom.Vec, head, tail float32) {
	out = append(out, path...)
	cut := func(end, next geom.Vec) (geom.Vec, bool) {
		for k, j := range js {
			if f32.Abs(j.At.X-end.X) > 1 || f32.Abs(j.At.Y-end.Y) > 1 {
				continue
			}
			d := unit(next.Sub(end))
			for i, way := range j.Ways {
				w := unit(way)
				if w.X*d.X+w.Y*d.Y > 0.99 {
					by := ms[k].Ends[i]
					return geom.NewVec(j.At.X+w.X*by, j.At.Y+w.Y*by), true
				}
			}
		}
		return end, false
	}
	// the path's points the junction takes go, its end put where the junction's begins
	shorten := func(pts []geom.Vec) ([]geom.Vec, bool) {
		at, ok := cut(pts[0], pts[1])
		if !ok {
			return pts, false
		}
		reach := at.Sub(pts[0])
		far := reach.X*reach.X + reach.Y*reach.Y
		k := 1
		for k < len(pts)-1 {
			d := pts[k].Sub(pts[0])
			if d.X*d.X+d.Y*d.Y > far {
				break
			}
			k++
		}
		return append([]geom.Vec{at}, pts[k:]...), true
	}
	var ok bool
	if out, ok = shorten(out); ok {
		head = lead
	}
	reverse(out)
	if out, ok = shorten(out); ok {
		tail = lead
	}
	reverse(out)
	return out, head, tail
}

func reverse(pts []geom.Vec) {
	for i, j := 0, len(pts)-1; i < j; i, j = i+1, j-1 {
		pts[i], pts[j] = pts[j], pts[i]
	}
}

// corridor is the outline of the ground within half of a path
func corridor(path []geom.Vec, half float32) []geom.Vec {
	var left, right []geom.Vec
	for k := range path {
		a, b := path[max(k-1, 0)], path[min(k+1, len(path)-1)]
		d := unit(b.Sub(a))
		left = append(left, geom.NewVec(path[k].X-d.Y*half, path[k].Y+d.X*half))
		right = append(right, geom.NewVec(path[k].X+d.Y*half, path[k].Y-d.X*half))
	}
	reverse(right)
	return append(left, right...)
}
