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
	s := NewScene("main", nil, func() *Element { return Layers(plain, own) })
	th := DefaultTheme()
	th.Panel = color.RGBA{B: 255, A: 255}
	s.Theme(th)
	s.Layers()
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
	s := NewScene("main", nil, func() *Element { return Layers(label) })
	th := DefaultTheme()
	th.Font = big
	s.Theme(th)
	s.Layers()
	w, h := label.needs()
	small, _ := DefaultTheme().Font.Measure("Cześć")
	if w <= float64(small) || h < 40 {
		t.Fatalf("the label needs %vx%v, want the big font's size", w, h)
	}
}

func goRegular() []byte { return goregular.TTF }
