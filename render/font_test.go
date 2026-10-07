package render_test

import (
	"image/color"
	"os"
	"testing"

	"github.com/kjkrol/gram/render"
	"github.com/kjkrol/gram/render/gpu"
)

func TestFont_MeasureGrowsWithTheTextAndItsLines(t *testing.T) {
	f := render.DefaultFont()
	w1, h1 := f.Measure("Cześć")
	w2, _ := f.Measure("Cześć, wędrowcze!")
	_, h3 := f.Measure("Cześć\nwędrowcze")
	if w1 <= 0 || w2 <= w1 || h3 != 2*h1 || h1 != f.LineHeight() {
		t.Fatalf("widths %v, %v; heights %v, %v (line %v); want growing, two lines twice one", w1, w2, h1, h3, f.LineHeight())
	}
}

func TestFont_TheDefaultHasThePolishLetters(t *testing.T) {
	f := render.DefaultFont()
	for _, r := range "ąćęłńóśźżĄĆĘŁŃÓŚŹŻ" {
		if !f.Has(r) {
			t.Errorf("no glyph for %q", r)
		}
	}
}

func TestDrawText_DrawsInsideTheBoxItMeasures(t *testing.T) {
	if err := gpu.Headless(os.Getenv("GRAM_GPU") == "software"); err != nil {
		t.Skipf("no GPU: %v", err)
	}
	f := render.DefaultFont()
	const x, y = 10, 10
	s := "Źdźbło"
	w, h := f.Measure(s)
	dst := render.NewImage(200, 60)
	dst.Clear()
	render.DrawText(dst, f, s, x, y, color.White)
	pix := make([]byte, 4*200*60)
	dst.ReadPixels(pix)
	inside, outside := 0, 0
	for py := 0; py < 60; py++ {
		for px := 0; px < 200; px++ {
			if pix[4*(py*200+px)+3] == 0 {
				continue
			}
			if px >= x && px < x+int(w)+1 && py >= y && py < y+int(h)+1 {
				inside++
			} else {
				outside++
			}
		}
	}
	if inside == 0 || outside != 0 {
		t.Fatalf("%d pixels drawn inside the measured box, %d outside; want some inside, none outside", inside, outside)
	}
}
