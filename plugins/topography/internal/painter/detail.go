package painter

import (
	"image"
	"image/draw"
	"sort"

	xdraw "golang.org/x/image/draw"

	"github.com/kjkrol/gram/plugins/board/cell"
	public "github.com/kjkrol/gram/plugins/topography/painter"
	"github.com/kjkrol/gram/render"
)

// Every detail's tile in the sheet Details bakes is DetailPx square, lies under a header row
// and is bordered by a pixel of its far side either way, so the GPU blends across its seam. The
// header's pixel k holds tile k's Across over AcrossScale in red: a texel, not a uniform, as the
// ground's uniforms are at the edge of what an Intel GPU pushes (64 bytes more cost 1.5 ms).
const (
	DetailPx    = public.DetailPx
	DetailPitch = DetailPx + 2
	AcrossScale = 16
	MaxDetails  = public.MaxDetails
)

type detailSheet struct {
	img    *render.Image
	of     map[render.SpriteID]byte // the detail a sprite's kind carries, 0 none
	styled uint64
}

// Details is every styled kind's Detail baked into one sheet, DetailPx pixels a tile in a grid
// under a header row, each tile's tone scaled to a mean of a half; nil while no kind has one.
// The count says how many times it was baked anew, 0 none.
func (l *Painter) Details() (sheet *render.Image, baked uint64) {
	l.bakeDetails()
	if l.details == nil || l.details.img == nil {
		return nil, l.detailsBaked
	}
	return l.details.img, l.detailsBaked
}

func (l *Painter) detailOf(sprite render.SpriteID) byte {
	l.bakeDetails()
	if l.details == nil {
		return 0
	}
	return l.details.of[sprite]
}

func (l *Painter) bakeDetails() {
	if l.details != nil && l.details.styled == l.styleVersion {
		return
	}
	d := &detailSheet{of: map[render.SpriteID]byte{}, styled: l.styleVersion}
	l.details = d
	if l.kinds == nil {
		return
	}
	names := make([]cell.Name, 0, len(l.styles))
	for name, s := range l.styles {
		if s.Detail.Image != nil {
			names = append(names, name)
		}
	}
	sort.Slice(names, func(i, j int) bool { return names[i].String() < names[j].String() })
	index := map[image.Image]byte{}
	var tiles []Detail
	for _, name := range names {
		k, ok := l.kinds.Get(name.String())
		if !ok {
			continue
		}
		detail := l.styles[name].Detail
		i, seen := index[detail.Image]
		if !seen {
			if len(tiles) == MaxDetails {
				panic("painter: more kinds with a Detail of their own than MaxDetails")
			}
			tiles = append(tiles, detail)
			i = byte(len(tiles))
			index[detail.Image] = i
		}
		d.of[k.SpriteID] = i
	}
	if len(tiles) == 0 {
		return
	}
	cols := 1
	for cols*cols < len(tiles) {
		cols++
	}
	rows := (len(tiles) + cols - 1) / cols
	w, h := cols*DetailPitch, 1+rows*DetailPitch
	pix := make([]byte, 4*w*h)
	for k, t := range tiles {
		x0, y0 := (k%cols)*DetailPitch+1, 1+(k/cols)*DetailPitch+1
		bakeTile(pix, w, x0, y0, t.Image)
		across := t.Across
		if across <= 0 {
			across = 1
		}
		o := 4 * k
		pix[o], pix[o+3] = byte(min(across/AcrossScale, 1)*255+0.5), 255
	}
	d.img = render.NewImage(w, h)
	d.img.WritePixels(pix)
	l.detailsBaked++
}

// how much of a tile's own hue is kept once its mean is grey: a patch far from the mean in
// hue comes out in the mean's complement, a purple on a reddish rock
const detailChroma = 0.35

// the tile DetailPx square at (x0, y0) with its border round it, its tone scaled so each
// channel's mean is 128, so the shader's doubling leaves the kind's colour as it is far off
func bakeTile(pix []byte, stride, x0, y0 int, src image.Image) {
	tile := image.NewRGBA(image.Rect(0, 0, DetailPx, DetailPx))
	if b := src.Bounds(); b.Dx() == DetailPx && b.Dy() == DetailPx {
		draw.Draw(tile, tile.Bounds(), src, b.Min, draw.Src)
	} else {
		xdraw.CatmullRom.Scale(tile, tile.Bounds(), src, b, xdraw.Src, nil)
	}
	var sum [3]float64
	for i := 0; i < len(tile.Pix); i += 4 {
		for c := range sum {
			sum[c] += float64(tile.Pix[i+c])
		}
	}
	var scale [3]float64
	for c := range scale {
		mean := sum[c] / float64(DetailPx*DetailPx)
		scale[c] = 0
		if mean > 0 {
			scale[c] = 128 / mean
		}
	}
	at := func(x, y int) int {
		x, y = (x+DetailPx)%DetailPx, (y+DetailPx)%DetailPx
		return 4 * (y*DetailPx + x)
	}
	for y := -1; y <= DetailPx; y++ {
		for x := -1; x <= DetailPx; x++ {
			s, o := at(x, y), 4*((y0+y)*stride+x0+x)
			var v [3]float64
			for c := range scale {
				v[c] = float64(tile.Pix[s+c]) * scale[c]
			}
			grey := (v[0] + v[1] + v[2]) / 3
			for c := range v {
				pix[o+c] = byte(min(grey+(v[c]-grey)*detailChroma, 255) + 0.5)
			}
			pix[o+3] = 255
		}
	}
}

// Kinds is the board's own sheet of the detail each pixel's kind carries (in red, the sheet
// Details bakes), as many pixels a cell as the albedo; nil before Surface painted one.
func (l *Painter) Kinds() *render.Image {
	if l.albedo == nil {
		return nil
	}
	return l.albedo.kinds
}

// CellDetails is, by cell row by row, the detail the cell's kind carries, or Dressed where its
// pixels in Kinds hold more than one kind; and a count of its changes. Nil before Surface.
func (l *Painter) CellDetails() ([]byte, uint64) {
	if l.albedo == nil {
		return nil, 0
	}
	return l.albedo.cells, l.albedo.cellsAt
}

func indexShade(k byte) render.Shade {
	return render.Lit(render.Light{float32(k) / 255, 0, 0})
}

func (l *Painter) layKindBase(s *groundSheet, i int) {
	cols := int(l.sq.Cols)
	c, ok := l.cellAt(int64(i%cols), int64(i/cols))
	if !ok {
		return
	}
	r := s.cell(uint32(i%cols), uint32(i/cols))
	x0, y0, x1, y1 := float32(r.Min.X), float32(r.Min.Y), float32(r.Max.X), float32(r.Max.Y)
	corners := render.Corners{{x0, y0}, {x1, y0}, {x0, y1}, {x1, y1}}
	s.kcanvas.Sprite(render.At{}, white, 0, corners, indexShade(l.detailOf(l.base(c).sprite)))
}

// the pieces as lay lays them, each in its kind's detail, edged hard where the albedo fades:
// a pixel holds one kind, the shader blending the details between the pixels
func (l *Painter) layKinds(s *groundSheet, i int, blends []BlendPiece, ways []WayPiece) {
	cols := int(l.sq.Cols)
	x0, y0 := float32((i%cols)*s.px), float32(s.top+(i/cols)*s.px)
	kx, ky := float32(s.px)/float32(l.cellW), float32(s.px)/float32(l.cellH)
	at := func(w render.World) render.Corners {
		var c render.Corners
		for k, p := range w {
			c[k] = [2]float32{x0 + p[0]*kx, y0 + p[1]*ky}
		}
		return c
	}
	for _, p := range blends {
		s.kcanvas.SpriteBlend(render.At{}, white, 0, at(p.World),
			indexShade(l.detailOf(p.Sprite)), p.Weight, 0)
	}
	for _, p := range ways {
		shade := indexShade(l.detailOf(p.Sprite))
		if p.Faded {
			s.kcanvas.SpriteBlend(render.At{}, white, 0, at(p.World), shade, p.Weight, 0)
			continue
		}
		s.kcanvas.Sprite(render.At{}, white, 0, at(p.World), shade)
	}
}
