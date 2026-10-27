package players

import (
	"fmt"
	"image/color"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/gram/game"
	"github.com/kjkrol/gram/ui"
)

// pixels
const (
	menuMargin     = 24 // round a menu's screen
	menuGap        = 16 // between a menu's parts
	sectionPadding = 8  // inside a section's frame
)

// the glass a menu dims the game behind it with
var menuDim = color.RGBA{A: 170}

// screenSize is the size a menu lays itself out for: the screen the scene was last laid over,
// the window as it is, else the Props' as the game starts
func screenSize(scene *ui.Scene, props game.Props) (w, h int) {
	if box := scene.Laid(); box.BottomRight.X > box.TopLeft.X {
		return int(box.BottomRight.X - box.TopLeft.X), int(box.BottomRight.Y - box.TopLeft.Y)
	}
	return props.ScreenWidth, props.ScreenHeight
}

// menu is the screen of the players' menus, the settings and the controls, laid for a screen w
// by h in the default theme: the title in a framed panel at the top middle (name.title) and,
// centred under it, the blocks in as many columns as the width takes (name.column.N), fewer and
// taller where they do not fit, scrolled then (name.scroll).
func menu(name, title string, blocks []block, w, h int) *ui.Element {
	width := float32(w) - 2*menuMargin
	height := float32(h) - 2*menuMargin
	_, titleHeight := ui.DefaultTheme().Large.Measure(title)
	height -= titleHeight + 2*sectionPadding + menuGap

	columns := columned(blocks, height)
	for n := len(columns) - 1; n >= 1 && widthOf(name, columns, width, height) > width; n-- {
		columns = columned(blocks, tallEnough(blocks, n))
	}
	return ui.Layers(
		ui.Blank().Fill(menuDim),
		ui.TopMiddle(ui.Rows(
			ui.Fit(ui.TopMiddle(ui.Panel(ui.Title(title).Named(name+".title")).
				Padding(sectionPadding).Border(ui.DefaultTheme().Border, 1))),
			ui.Fixed(menuGap, ui.Blank()),
			ui.Fixed(height, ui.Scroll(ui.Columns(spaced(name, columns)...)).Named(name+".scroll")),
		)).Padding(menuMargin),
	)
}

// a block never splits across columns: a top section framed whole, or within one a heading (a
// parent's without entries of its own joined to its first child's) and the lines right under it
type block struct {
	parts []ui.Part
	tall  float32
	bare  bool // headings alone so far
}

func (b *block) add(part ui.Part, tall float32) {
	b.parts = append(b.parts, part)
	b.tall += tall
}

// framed is blocks as one: their parts in a panel bordered a pixel, named "frame:" and the
// section's path, a gap under it
func framed(blocks []block, path string) block {
	var parts []ui.Part
	tall := float32(0)
	for _, b := range blocks {
		parts = append(parts, b.parts...)
		tall += b.tall
	}
	tall += 2 * sectionPadding
	frame := ui.Panel(ui.Rows(parts...)).Padding(sectionPadding).Named("frame:" + path)
	frame.Border(ui.DefaultTheme().Border, 1)
	return block{parts: []ui.Part{ui.Fixed(tall, frame), ui.Fixed(menuGap, ui.Blank())},
		tall: tall + menuGap}
}

// columned is the blocks in columns at most height tall, a block never split
func columned(blocks []block, height float32) []*ui.Element {
	var columns []*ui.Element
	var column []ui.Part
	used := float32(0)
	for _, b := range blocks {
		if used > 0 && used+b.tall > height {
			columns = append(columns, ui.Rows(column...))
			column, used = nil, 0
		}
		column = append(column, b.parts...)
		used += b.tall
	}
	if len(column) > 0 {
		columns = append(columns, ui.Rows(column...))
	}
	return columns
}

// tallEnough is the least height that holds the blocks in n columns
func tallEnough(blocks []block, n int) float32 {
	total, tallest := float32(0), float32(0)
	for _, b := range blocks {
		total += b.tall
		tallest = max(tallest, b.tall)
	}
	height := max(total/float32(n), tallest)
	for len(columned(blocks, height)) > n {
		height += menuGap
	}
	return height
}

// widthOf is the columns' width with the gaps between them, measured laid out once
func widthOf(name string, columns []*ui.Element, width, height float32) float32 {
	probe := ui.NewScene("", ui.Columns(spaced(name, columns)...))
	probe.Drawers()
	probe.Lay(geom.NewAABB(geom.NewVec(0, 0), geom.NewVec(width, height)))
	w := float32(0)
	for i := range columns {
		box := probe.Element(columnName(name, i)).Box()
		w += box.BottomRight.X - box.TopLeft.X
	}
	return w + float32(len(columns)-1)*menuGap*2
}

// spaced is the columns as parts, named by their place, a gap between them
func spaced(name string, columns []*ui.Element) []ui.Part {
	parts := make([]ui.Part, 0, 2*len(columns))
	for i, c := range columns {
		if i > 0 {
			parts = append(parts, ui.Fixed(menuGap*2, ui.Blank()))
		}
		parts = append(parts, ui.Fit(c.Named(columnName(name, i))))
	}
	return parts
}

func columnName(name string, i int) string { return fmt.Sprintf("%s.column.%d", name, i) }
