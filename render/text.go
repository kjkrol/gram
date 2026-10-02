package render

import (
	"image"
	"image/color"

	"github.com/kjkrol/gram/render/gpu"
	"golang.org/x/image/font"
	"golang.org/x/image/font/basicfont"
	"golang.org/x/image/math/fixed"
)

// Text is printed as Ebitengine's debug text was: glyphs six pixels apart, lines sixteen, white
// over a black shadow a pixel down and right; the glyphs are x/image's basicfont 7x13.
const (
	GlyphWidth = 6
	LineHeight = 16
)

// glyphs is the sheet of every glyph printed, 7 by 16 a cell, by rune; built at first print.
var glyphs struct {
	sheet *Image
	cell  map[rune]int
}

const glyphCell = 7

// handGlyphs are glyphs basicfont lacks — it has ASCII alone — drawn here by hand from a cell's
// top-left, '#' a lit pixel; the rest of Latin-1 prints as its replacement box.
var handGlyphs = map[rune][]string{
	'°': {"", "", "", "  ##", " #  #", " #  #", "  ##"},
}

func glyphSheet() *Image {
	if glyphs.sheet != nil {
		return glyphs.sheet
	}
	var runes []rune
	for r := rune(0x20); r <= 0xff; r++ {
		if r < 0x7f || r >= 0xa1 {
			runes = append(runes, r)
		}
	}
	img := image.NewRGBA(image.Rect(0, 0, glyphCell*len(runes), LineHeight))
	d := font.Drawer{Dst: img, Src: image.NewUniform(color.White), Face: basicfont.Face7x13}
	glyphs.cell = map[rune]int{}
	for i, r := range runes {
		glyphs.cell[r] = i
		if rows, ok := handGlyphs[r]; ok {
			for y, row := range rows {
				for x, px := range row {
					if px == '#' {
						img.Set(i*glyphCell+x, y, color.White)
					}
				}
			}
			continue
		}
		d.Dot = fixed.P(i*glyphCell, 12)
		d.DrawString(string(r))
	}
	glyphs.sheet = NewImage(img.Bounds().Dx(), img.Bounds().Dy())
	glyphs.sheet.WritePixels(img.Pix)
	return glyphs.sheet
}

// DebugPrint prints s at the image's top-left corner.
func DebugPrint(dst *Image, s string) {
	DebugPrintAt(dst, s, dst.Bounds().Min.X, dst.Bounds().Min.Y)
}

// DebugPrintAt prints s with its top-left corner at (x, y), a line of it under another at each
// newline.
func DebugPrintAt(dst *Image, s string, x, y int) {
	sheet := glyphSheet()
	var verts []gpu.Vertex
	var indices []uint16
	for _, shade := range [2]struct {
		dx, dy int
		c      float32
	}{{1, 1, 0}, {0, 0, 1}} {
		col, line := 0, 0
		for _, r := range s {
			if r == '\n' {
				col, line = 0, line+1
				continue
			}
			i, ok := glyphs.cell[r]
			if !ok {
				i = glyphs.cell['?']
			}
			x0 := float32(x + col*GlyphWidth + shade.dx)
			y0 := float32(y + line*LineHeight + shade.dy)
			u0 := float32(i * glyphCell)
			v := func(dx, dy, su, sv float32) gpu.Vertex {
				return gpu.Vertex{DstX: x0 + dx, DstY: y0 + dy, SrcX: u0 + su, SrcY: sv, ColorR: shade.c, ColorG: shade.c, ColorB: shade.c, ColorA: 1}
			}
			base := uint16(len(verts))
			verts = append(verts, v(0, 0, 0, 0), v(glyphCell, 0, glyphCell, 0), v(0, LineHeight, 0, LineHeight), v(glyphCell, LineHeight, glyphCell, LineHeight))
			indices = append(indices, base, base+1, base+2, base+1, base+2, base+3)
			col++
			if len(verts) > 65000 {
				gpu.Sprites(dst.gpu(), sheet.gpu(), verts, indices, false, gpu.SourceOver)
				verts, indices = verts[:0], indices[:0]
			}
		}
	}
	gpu.Sprites(dst.gpu(), sheet.gpu(), verts, indices, false, gpu.SourceOver)
}

// TextWidth is how many pixels wide the longest line of s is printed.
func TextWidth(s string) int {
	longest, n := 0, 0
	for _, r := range s {
		if r == '\n' {
			n = 0
			continue
		}
		n++
		longest = max(longest, n)
	}
	return longest * GlyphWidth
}
