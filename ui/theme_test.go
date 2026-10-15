package ui

import (
	"image/color"
	"testing"

	"golang.org/x/image/font/gofont/goregular"

	"github.com/kjkrol/gram/render"
)

var red = color.RGBA{R: 255, A: 255}

func TestTheme_ElementsTakeTheScenesColoursWhereTheySayNone(t *testing.T) {
	plain := Panel(Label("a"))
	own := Panel(Label("b")).Fill(red)
	s := NewScene("main", Layers(plain, own))
	th := DefaultTheme()
	th.Panel = color.RGBA{B: 255, A: 255}
	s.Theme(th)
	s.Drawers()
	if plain.fillColor() != th.Panel {
		t.Errorf("a panel saying no colour is %v, want the theme's %v", plain.fillColor(), th.Panel)
	}
	if own.fillColor() != red {
		t.Errorf("a panel with a Fill of its own is %v, want its own", own.fillColor())
	}
}

func TestTheme_ALabelMeasuresInTheThemesFont(t *testing.T) {
	big, err := render.NewFont(goRegular(), 40)
	if err != nil {
		t.Fatal(err)
	}
	label := Label("Cześć")
	s := NewScene("main", Layers(label))
	th := DefaultTheme()
	th.Font = big
	s.Theme(th)
	s.Drawers()
	w, h := label.needs()
	small, _ := DefaultTheme().Font.Measure("Cześć")
	if w <= float64(small) || h < 40 {
		t.Fatalf("the label needs %vx%v, want the big font's size", w, h)
	}
}

func goRegular() []byte { return goregular.TTF }

func TestTheme_AHeadingMeasuresInTheThemesBoldFont(t *testing.T) {
	big, err := render.NewFont(goRegular(), 40)
	if err != nil {
		t.Fatal(err)
	}
	heading, label := Heading("General"), Label("General")
	s := NewScene("main", Layers(heading, label))
	th := DefaultTheme()
	th.Bold = big
	s.Theme(th)
	s.Drawers()
	hw, hh := heading.needs()
	lw, _ := label.needs()
	if hw <= lw || hh < 40 {
		t.Fatalf("the heading needs %vx%v against the label's %v, want the bold font's", hw, hh, lw)
	}
}

func TestTheme_ATitleMeasuresInTheThemesLargeFont(t *testing.T) {
	title, heading := Title("Shortcuts"), Heading("Shortcuts")
	s := NewScene("main", Layers(title, heading))
	s.Drawers()
	tw, th := title.needs()
	hw, hh := heading.needs()
	if tw <= hw || th <= hh {
		t.Fatalf("the title needs %vx%v against the heading's %vx%v, want larger", tw, th, hw, hh)
	}
}
