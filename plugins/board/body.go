package board

import (
	"bytes"
	"cmp"
	"slices"

	"github.com/kjkrol/aabbworld/geom"
)

// Family is board's tag family; Plugin.Body lives in it.
type Family struct{}

// MaxBodyCells caps how many cells one terrain body spans along either axis.
const MaxBodyCells = 16

// bodyBox is one body in the making: its box, the terrain kind it is made of, whether it is solid
// or only veils sight, and how many pieces the current merge pass folded into it.
type bodyBox struct {
	box      geom.AABB
	kind     Name
	solid    bool
	veil     float64
	veils    Domain
	allows   Domain
	altitude float64
	height   float64
	count    int
}

// embodied reports whether cells of k become bodies: solid ones push, veiled ones dim sight.
func embodied(k CellKind) bool { return k.Solid || k.Veil > 0 }

// terrainBoxes lists every embodied cell's boxes merged into as few bodies as MaxBodyCells allows.
func terrainBoxes(brd *Board, dst []bodyBox) []bodyBox {
	dst = dst[:0]
	var boxes []geom.AABB
	brd.EachCell(func(c CellID) {
		k := brd.Kind(c)
		if !embodied(k) {
			return
		}
		alt := brd.Altitude(c)
		boxes = brd.CellBoxes(c, boxes[:0])
		for _, b := range boxes {
			dst = append(dst, bodyBox{box: b, kind: k.Name, solid: k.Solid, veil: k.Veil, veils: k.Veils, allows: k.Allows, altitude: alt, height: k.Height})
		}
	})
	dst = mergeAlong(dst, true)
	dst = mergeAlong(dst, false)
	slices.SortFunc(dst, func(a, b bodyBox) int {
		return cmp.Or(cmp.Compare(a.box.TopLeft.Y, b.box.TopLeft.Y), cmp.Compare(a.box.TopLeft.X, b.box.TopLeft.X))
	})
	return dst
}

// mergeAlong folds boxes of one kind at one altitude that share their span across the axis and
// touch along it, in place, MaxBodyCells at a time.
func mergeAlong(in []bodyBox, alongX bool) []bodyBox {
	lo := func(b bodyBox) (across1, across2, along float64) {
		if alongX {
			return b.box.TopLeft.Y, b.box.BottomRight.Y, b.box.TopLeft.X
		}
		return b.box.TopLeft.X, b.box.BottomRight.X, b.box.TopLeft.Y
	}
	slices.SortFunc(in, func(a, b bodyBox) int {
		a1, a2, a3 := lo(a)
		b1, b2, b3 := lo(b)
		return cmp.Or(bytes.Compare(a.kind[:], b.kind[:]), cmp.Compare(a.altitude, b.altitude), cmp.Compare(a1, b1), cmp.Compare(a2, b2), cmp.Compare(a3, b3))
	})
	out := in[:0]
	for i := range in {
		cur := in[i]
		if n := len(out); n > 0 && joins(&out[n-1], cur, alongX) {
			continue
		}
		cur.count = 1
		out = append(out, cur)
	}
	return out
}

// joins grows prev by next when they are one kind at one altitude, one lane and touching, within
// MaxBodyCells.
func joins(prev *bodyBox, next bodyBox, alongX bool) bool {
	const eps = 1e-9
	if prev.kind != next.kind || prev.altitude != next.altitude || prev.count >= MaxBodyCells {
		return false
	}
	p, n := prev.box, next.box
	if alongX {
		if p.TopLeft.Y != n.TopLeft.Y || p.BottomRight.Y != n.BottomRight.Y || n.TopLeft.X > p.BottomRight.X+eps {
			return false
		}
		prev.box.BottomRight.X = max(p.BottomRight.X, n.BottomRight.X)
	} else {
		if p.TopLeft.X != n.TopLeft.X || p.BottomRight.X != n.BottomRight.X || n.TopLeft.Y > p.BottomRight.Y+eps {
			return false
		}
		prev.box.BottomRight.Y = max(p.BottomRight.Y, n.BottomRight.Y)
	}
	prev.count++
	return true
}
