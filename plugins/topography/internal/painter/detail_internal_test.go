package painter

import (
	"image"
	"image/color"
	"testing"

	"github.com/kjkrol/ceremony"
	"github.com/kjkrol/gram/plugins/atmosphere/sky"
	"github.com/kjkrol/gram/plugins/board"
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/board/grid"
	"github.com/kjkrol/gram/render"
)

type testKinds map[string]cell.Kind

func (k testKinds) Define(name string, kind cell.Kind, _ ...*ceremony.Part) { k[name] = kind }
func (k testKinds) Named(name string) cell.Of {
	return cell.OfKind(k[name])
}
func (k testKinds) Get(name string) (cell.Kind, bool) {
	kind, ok := k[name]
	return kind, ok
}
func (k testKinds) All() []cell.Kind {
	var all []cell.Kind
	for _, kind := range k {
		all = append(all, kind)
	}
	return all
}

func solidImage(v uint8, size int) image.Image {
	img := image.NewRGBA(image.Rect(0, 0, size, size))
	for i := 0; i < len(img.Pix); i += 4 {
		img.Pix[i], img.Pix[i+1], img.Pix[i+2], img.Pix[i+3] = v, v, v, 255
	}
	return img
}

func TestBakeTile_ScalesTheToneToAMeanOfAHalfAndBordersTheTileWithItsFarSide(t *testing.T) {
	src := image.NewRGBA(image.Rect(0, 0, DetailPx, DetailPx))
	for y := range DetailPx {
		for x := range DetailPx {
			v := uint8(40)
			if x < DetailPx/2 {
				v = 120
			}
			src.SetRGBA(x, y, color.RGBA{v, v, v, 255})
		}
	}
	stride := DetailPitch
	pix := make([]byte, 4*stride*(DetailPitch+1))
	bakeTile(pix, stride, 1, 2, src)
	at := func(x, y int) byte { return pix[4*((2+y)*stride+1+x)] }
	if left, right := at(0, 0), at(DetailPx-1, 0); left != 192 || right != 64 {
		t.Errorf("the tile's left is %d and its right %d, want 192 and 64: a mean of 128",
			left, right)
	}
	if at(-1, 0) != at(DetailPx-1, 0) || at(DetailPx, 0) != at(0, 0) ||
		at(0, -1) != at(0, DetailPx-1) {
		t.Error("the border does not carry the far side across the seam")
	}
}

func TestPainter_BakesTheDetailsOnceAndPaintsEachPixelsKind(t *testing.T) {
	needGPU(t)
	st := map[cell.Name]Style{}
	kinds := testKinds{}
	grid := grid.DefaultGrids{}.Square(4, 4, 32)
	brd := board.NewBoard(grid)
	grass := solidImage(200, 8)
	land := styled(st, cell.Kind{Name: cell.Named("k50"), Allows: cell.Land, SpriteID: 1},
		Style{Spread: 0.3, Detail: Detail{Image: grass, Across: 2}})
	sand := styled(st, cell.Kind{Name: cell.Named("k51"), Allows: cell.Land, SpriteID: 2},
		Style{Spread: 0.3, Detail: Detail{Image: solidImage(90, 8), Across: 1}})
	bare := styled(st, cell.Kind{Name: cell.Named("k52"), Allows: cell.Land, SpriteID: 3}, Style{})
	kinds.Define("k50", land)
	kinds.Define("k51", sand)
	kinds.Define("k52", bare)
	brd.SetAll(land)
	at := func(x, y uint32) cell.ID { return grid.At(x, y) }
	for y := range uint32(4) {
		brd.Set(at(3, y), sand)
	}
	brd.Set(at(2, 0), sand)
	brd.Set(at(0, 3), bare)
	d := New(brd, reliefFor(brd), skyOf(sky.DefaultSun), false, st).WithKinds(kinds)
	sheet, baked := d.Details()
	if sheet == nil || baked != 1 || sheet.Bounds().Dx() != 2*DetailPitch ||
		sheet.Bounds().Dy() != 1+DetailPitch {
		t.Fatalf("the details are %v baked %d times, want two tiles in a row under a header, once",
			sheet, baked)
	}
	pix := make([]byte, 4*sheet.Bounds().Dx()*sheet.Bounds().Dy())
	sheet.ReadPixels(pix)
	if pix[0] != 32 || pix[4] != 16 {
		t.Errorf("the header holds %d and %d, want across 2 and 1 over 16 as 32 and 16",
			pix[0], pix[4])
	}
	if v := pix[4*((2+DetailPx/2)*sheet.Bounds().Dx()+1+DetailPx/2)]; v != 128 {
		t.Errorf("the first tile's middle is %d, want 128: a solid tile scaled to a half", v)
	}
	if d.detailOf(1) != 1 || d.detailOf(2) != 2 || d.detailOf(3) != 0 {
		t.Errorf("the details of the sprites are %d, %d and %d, want 1, 2 and none",
			d.detailOf(1), d.detailOf(2), d.detailOf(3))
	}
	if _, again := d.Details(); again != 1 {
		t.Error("the details were baked again with the styles as they were")
	}
	atlas := sheetAtlas{render.NewImage(32, 8)}
	if _, _, px, _ := d.Surface(atlas); px != 16 {
		t.Fatalf("the albedo is %d pixels a cell, want 16", px)
	}
	kindsSheet := d.Kinds()
	if kindsSheet == nil || kindsSheet.Bounds().Dx() != 64 || kindsSheet.Bounds().Dy() != 64 {
		t.Fatalf("the kinds are %v, want a 64 by 64 sheet", kindsSheet)
	}
	pix = make([]byte, 4*64*64)
	kindsSheet.ReadPixels(pix)
	kindAt := func(x, y int) byte { return pix[4*(y*64+x)] }
	if got := kindAt(40, 24); got != 1 {
		t.Errorf("the middle of a cell of the first kind holds %d, want 1", got)
	}
	if got := kindAt(56, 24); got != 2 {
		t.Errorf("the middle of the sand holds %d, want 2", got)
	}
	if got := kindAt(8, 56); got != 0 {
		t.Errorf("the middle of the bare cell holds %d, want none", got)
	}
	if got := kindAt(47, 17); got != 2 {
		t.Errorf("the land's corner among three cells of sand holds %d, want the sand in, 2",
			got)
	}
	for y := range 64 {
		for x := range 64 {
			if g, b := pix[4*(y*64+x)+1], pix[4*(y*64+x)+2]; g != 0 || b != 0 {
				t.Fatalf("pixel %d, %d holds %v, want its kind in red alone", x, y,
					pix[4*(y*64+x):4*(y*64+x)+4])
			}
		}
	}
	cells, _ := d.CellDetails()
	if cells[4] != 1 || cells[12] != 0 || cells[6] != Dressed {
		t.Errorf("the cells carry %v, want 1, none and dressed at 4, 12 and 6", cells)
	}
	d.Style("k52", Style{Detail: Detail{Image: grass}})
	if _, again := d.Details(); again != 2 {
		t.Error("the details were not baked anew after a style changed")
	}
	d.Surface(atlas)
	d.Kinds().ReadPixels(pix)
	if got := kindAt(8, 56); got != 1 {
		t.Errorf("after the bare kind took the grass its cell holds %d, want 1", got)
	}
}
