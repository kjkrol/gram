package render_test

import (
	"testing"

	"github.com/kjkrol/gram/render"
)

type outline struct{ width int }

func (outline) Style() {}

type tint struct{ shade int }

func (tint) Style() {}

func TestStyles_KeepOneStyleOfEachTypeASprite(t *testing.T) {
	var s render.Styles
	s.Set(3, outline{width: 2})
	s.Set(3, tint{shade: 7})
	s.Set(3, outline{width: 5})
	if got, ok := render.StyleOf[outline](&s, 3); !ok || got.width != 5 {
		t.Errorf("outline of sprite 3: %v, %v; want width 5, the later declared", got, ok)
	}
	if got, ok := render.StyleOf[tint](&s, 3); !ok || got.shade != 7 {
		t.Errorf("tint of sprite 3: %v, %v; want shade 7", got, ok)
	}
	if _, ok := render.StyleOf[outline](&s, 4); ok {
		t.Error("sprite 4 has an outline it was never given")
	}
	if _, ok := render.StyleOf[outline](nil, 3); ok {
		t.Error("nil Styles has an outline")
	}
}
