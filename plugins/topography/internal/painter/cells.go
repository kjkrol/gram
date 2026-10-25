package painter

import (
	"image"

	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/render"
)

// the bytes of a cell's block of 4×2 pixels in the cells' image, pixel by pixel along the top
// row, then the bottom: its kind's sprite (low, high), its flags, its spread in 255ths of a half;
// its shine in 255ths, its flow in 16ths (low, high), its kind's detail; its way's sprite (low,
// high), the way's Links, its detail; the way's width in 64ths (low, high), how far it has faded
// and turned into its mix in 255ths; the way's shine in 255ths, its flow in 16ths (low, high), its
// mix's shine; the mix's sprite (low, high), two spare; the ground at the cell's corners in
// 65535ths of the relief from its lowest to its highest (top-left, top-right, bottom-left,
// bottom-right, low byte first)
const (
	cellSprite    = 0
	cellFlags     = 2
	cellSpread    = 3
	cellShine     = 4
	cellFlow      = 5
	cellDetail    = 7
	cellWaySprite = 8
	cellLinks     = 10
	cellWayDetail = 11
	cellWidth     = 12
	cellFade      = 14
	cellMix       = 15
	cellWayShine  = 16
	cellWayFlow   = 17
	cellMixShine  = 19
	cellMixSprite = 20
	cellGround    = 24
	cellBytes     = 32
)

// a cell's flags
const (
	flagUnder  = 1 // it lies under the kinds round it
	flagRaised = 2 // its kind stands at a height
	flagRuns   = 4 // a way runs across it
	flagMixes  = 8 // its way turns into the kind it mixes with
)

// sprites a row of the rectangles' image holds, two pixels each
const rectsPerRow = 256

// cells is what the bake's shaders read of the board, as the Painter reads it: a block of a
// cell's (cellBytes), the sheet's rectangles of the sprites they name, two pixels a sprite; laid
// anew where cells changed and whole when the styles, the relief or the sheet did
type cells struct {
	img     *render.Image
	rects   *render.Image
	rected  []bool
	atlas   render.AtlasSource
	vers    []uint64 // one more than each cell's version the image holds, by ordinal
	at      uint64   // one more than the board's count of changes it holds; 0 none
	prev    uint64   // what at was before the last refresh that changed it
	styled  uint64
	low     float32 // the relief's lowest and its span the heights are held in
	span    float32
	block   [cellBytes]byte
	buf     []byte
	rect    [8]byte
	changed []int // the cells written anew by the last refresh, by ordinal; nil for all
}

// refresh brings the cells up to the board out of atlas; it reports whether anything changed.
func (l *Painter) refresh(atlas render.AtlasSource) bool {
	g := &l.cells
	cols, rows := int(l.sq.Cols), int(l.sq.Rows)
	n := cols * rows
	if g.at == l.board.Changes()+1 && g.styled == l.styleVersion && g.atlas == atlas {
		return false
	}
	low, high := l.ground().Extent()
	all := g.at == 0 || g.styled != l.styleVersion || g.atlas != atlas || len(g.vers) != n ||
		low != g.low || (high-low) != g.span
	if g.atlas != atlas {
		g.atlas, g.rects, g.rected = atlas, nil, g.rected[:0]
	}
	if g.img == nil || g.img.Bounds().Dx() != 4*cols || g.img.Bounds().Dy() != 2*rows {
		g.img, all = render.NewImage(4*cols, 2*rows), true
	}
	if len(g.vers) != n {
		g.vers = make([]uint64, n)
	}
	g.prev = g.at
	g.at, g.styled = l.board.Changes()+1, l.styleVersion
	g.low, g.span = low, high-low
	l.tables()
	if !all {
		g.changed = g.changed[:0]
		for i := range n {
			c := l.board.At(uint32(i%cols), uint32(i/cols))
			if l.board.CellVersion(c)+1 != g.vers[i] {
				g.changed = append(g.changed, i)
			}
		}
		if len(g.changed) <= n/8 {
			for _, i := range g.changed {
				x, y := i%cols, i/cols
				l.fillCell(l.board.At(uint32(x), uint32(y)), i)
				b := &g.block
				g.img.SubImage(image.Rect(4*x, 2*y, 4*x+4, 2*y+2)).WritePixels(b[:])
			}
			return true
		}
	}
	g.changed = nil
	stride := 16 * cols
	if need := stride * 2 * rows; cap(g.buf) < need {
		g.buf = make([]byte, need)
	} else {
		g.buf = g.buf[:need]
	}
	for y := range rows {
		for x := range cols {
			i := y*cols + x
			l.fillCell(l.board.At(uint32(x), uint32(y)), i)
			at := 2*y*stride + 16*x
			copy(g.buf[at:at+16], g.block[:16])
			copy(g.buf[at+stride:at+stride+16], g.block[16:])
		}
	}
	g.img.WritePixels(g.buf)
	return true
}

// fillCell lays what the bake reads of c, of ordinal i, into the block, its sprites into the
// rectangles
func (l *Painter) fillCell(c cell.ID, i int) {
	g := &l.cells
	g.vers[i] = l.board.CellVersion(c) + 1
	g.block = [cellBytes]byte{}
	b := &g.block
	top := l.topOf(c)
	g.sprite(cellSprite, top.sprite)
	if top.under {
		b[cellFlags] |= flagUnder
	}
	if top.raised {
		b[cellFlags] |= flagRaised
	}
	b[cellSpread] = unit255(top.spread * 2)
	b[cellShine] = unit255(top.shine)
	put16(b[cellFlow:], top.flow*16)
	b[cellDetail] = l.detailOf(top.sprite)
	if w := top.way; w.Runs() {
		b[cellFlags] |= flagRuns
		g.sprite(cellWaySprite, w.Kind.SpriteID)
		b[cellLinks] = byte(w.Links)
		b[cellWayDetail] = l.detailOf(w.Kind.SpriteID)
		put16(b[cellWidth:], w.Width*64)
		b[cellFade], b[cellMix] = unit255(w.Fade), unit255(w.Mix)
		b[cellWayShine], b[cellMixShine] = unit255(w.shine), unit255(w.mixShine)
		put16(b[cellWayFlow:], w.flow*16)
		if w.mixes {
			b[cellFlags] |= flagMixes
			g.sprite(cellMixSprite, w.mix)
		}
	}
	for k, h := range top.ground {
		v := float32(0)
		if g.span > 0 {
			v = (h - g.low) / g.span * 65535
		}
		put16(b[cellGround+2*k:], v)
	}
}

func put16(b []byte, v float32) {
	u := uint16(min(max(v+0.5, 0), 65535))
	b[0], b[1] = byte(u), byte(u>>8)
}

func unit255(v float32) byte { return byte(min(max(v, 0), 1)*255 + 0.5) }

// sprite lays id at the block's byte at, low then high, and its rectangle into the rectangles
// where there is a sheet
func (g *cells) sprite(at int, id render.SpriteID) {
	g.block[at], g.block[at+1] = byte(id), byte(id>>8)
	k := int(id)
	if g.atlas == nil || k < len(g.rected) && g.rected[k] {
		return
	}
	if rows := k/rectsPerRow + 1; g.rects == nil || g.rects.Bounds().Dy() < rows {
		old := g.rects
		g.rects = render.NewImage(2*rectsPerRow, max(rows, 2*heightOf(old)))
		for j, done := range g.rected {
			if done {
				g.writeRect(j)
			}
		}
	}
	for len(g.rected) <= k {
		g.rected = append(g.rected, false)
	}
	g.rected[k] = true
	g.writeRect(k)
}

func heightOf(img *render.Image) int {
	if img == nil {
		return 1
	}
	return img.Bounds().Dy()
}

func (g *cells) writeRect(k int) {
	x0, y0, x1, y1 := g.atlas.UV(render.SpriteID(k))
	r := &g.rect
	for j, v := range [4]float32{x0, y0, x1, y1} {
		u := uint16(v)
		r[2*j], r[2*j+1] = byte(u), byte(u>>8)
	}
	x, y := 2*(k%rectsPerRow), k/rectsPerRow
	g.rects.SubImage(image.Rect(x, y, x+2, y+1)).WritePixels(r[:])
}
