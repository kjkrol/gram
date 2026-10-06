package render

import (
	"strings"
	"testing"
)

// panicOf runs f and returns what it panicked with, failing the test if it did not.
func panicOf(t *testing.T, f func()) (msg string) {
	t.Helper()
	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("expected a panic")
		}
		msg, _ = r.(string)
	}()
	f()
	return ""
}

// overlap reports whether two sprites' sheet rects share any area.
func overlap(a *Atlas, l, r SpriteID) bool {
	lx0, ly0, lx1, ly1 := a.UV(l)
	rx0, ry0, rx1, ry1 := a.UV(r)
	return lx0 < rx1 && rx0 < lx1 && ly0 < ry1 && ry0 < ly1
}

func TestAtlas_RegisterAt_BindsEachDrawerToItsOwnSlot(t *testing.T) {
	first, second := SpriteID(0), SpriteID(1)

	atlas := NewAtlas()
	var drawnFor []SpriteID
	atlas.Add(second, 4, func(*Canvas, int) { drawnFor = append(drawnFor, second) })
	atlas.Add(first, 4, func(*Canvas, int) { drawnFor = append(drawnFor, first) })
	atlas.Close()

	if len(drawnFor) != 2 {
		t.Fatalf("expected both drawers to run, got %v", drawnFor)
	}
	sx0, _, sx1, _ := atlas.UV(second)
	if want := float32(4); sx0 != want || sx1 != want+4 {
		t.Errorf("UV(second) = [%v,%v], want the slot after the first at x=%v", sx0, sx1, want)
	}
}

func TestAtlas_BakesOnlyAtClose_EachSpriteOnceAtItsOwnSize(t *testing.T) {
	atlas := NewAtlas()
	drawn := map[int]int{}
	drawer := func(_ *Canvas, size int) { drawn[size]++ }

	atlas.Add(0, 16, drawer)
	const late = SpriteID(1)
	atlas.Add(late, 100, drawer)
	atlas.Add(5, 2, drawer)
	if len(drawn) != 0 {
		t.Fatalf("drawers ran before Close: %v", drawn)
	}

	atlas.Close()
	atlas.Close()

	if late != 1 {
		t.Errorf("Register took slot %d, want 1 — the next free one", late)
	}
	if len(drawn) != 3 || drawn[16] != 1 || drawn[100] != 1 || drawn[2] != 1 {
		t.Errorf("drawers ran %v, want each once, handed its own size", drawn)
	}
	for id, want := range map[SpriteID]float32{0: 16, 1: 100, 5: 2} {
		x0, y0, x1, y1 := atlas.UV(id)
		if x1-x0 != want || y1-y0 != want {
			t.Errorf("sprite %d sits in a %vx%v rect, want %vx%v", id, x1-x0, y1-y0, want, want)
		}
	}
	for _, pair := range [][2]SpriteID{{0, 1}, {0, 5}, {1, 5}} {
		if overlap(atlas, pair[0], pair[1]) {
			t.Errorf("sprites %d and %d share sheet area", pair[0], pair[1])
		}
	}
}

func TestAtlas_Close_WrapsIntoRowsRatherThanOutgrowATexture(t *testing.T) {
	const size, count = 1000, 9

	atlas := NewAtlas()
	for i := range count {
		atlas.Add(SpriteID(i), size, func(*Canvas, int) {})
	}
	atlas.Close()

	bounds := atlas.Atlas().Bounds()
	if bounds.Dx() > maxAtlasWidth {
		t.Errorf("the sheet is %d wide, want at most %d", bounds.Dx(), maxAtlasWidth)
	}
	if bounds.Dy() < 2*size {
		t.Errorf("the sheet is %d tall, want its sprites wrapped into more than one row", bounds.Dy())
	}
	for l := range SpriteID(count) {
		for r := l + 1; r < count; r++ {
			if overlap(atlas, l, r) {
				t.Fatalf("sprites %d and %d share sheet area", l, r)
			}
		}
	}
}

func TestAtlas_RefusesWhatCannotWork(t *testing.T) {
	nothing := func(*Canvas, int) {}
	for name, tc := range map[string]struct {
		do   func(a *Atlas)
		want string
	}{
		"the sheet before Close": {func(a *Atlas) { a.Atlas() }, "before Close"},
		"a UV before Close":      {func(a *Atlas) { a.Add(0, 4, nothing); a.UV(0) }, "before Close"},
		"adding after Close":     {func(a *Atlas) { a.Close(); a.Add(0, 4, nothing) }, "after Close"},
		"a slot twice":           {func(a *Atlas) { a.Add(3, 4, nothing); a.Add(3, 4, nothing) }, "sprite 3"},
		"a sprite nobody gave":   {func(a *Atlas) { a.Add(2, 4, nothing); a.Close(); a.UV(1) }, "sprite 1"},
		"a sprite past the last": {func(a *Atlas) { a.Add(2, 4, nothing); a.Close(); a.UV(9) }, "sprite 9"},
		"a sprite of no size":    {func(a *Atlas) { a.Add(0, 0, nothing) }, "size 0"},
	} {
		t.Run(name, func(t *testing.T) {
			msg := panicOf(t, func() { tc.do(NewAtlas()) })
			if !strings.Contains(msg, tc.want) {
				t.Errorf("panic %q does not mention %q", msg, tc.want)
			}
		})
	}
}

func TestAtlas_Close_WithNothingRegistered_StillYieldsASheet(t *testing.T) {
	atlas := NewAtlas()
	atlas.Close()

	if atlas.Atlas() == nil {
		t.Error("an empty atlas has no sheet to hand a renderer")
	}
}

func TestAtlas_KeepsAWhitePatchClearOfTheSprites(t *testing.T) {
	atlas := NewAtlas()
	a, b := SpriteID(0), SpriteID(1)
	atlas.Add(a, 8, func(*Canvas, int) {})
	atlas.Add(b, 16, func(*Canvas, int) {})
	atlas.Close()
	u, v := atlas.White()
	for _, id := range []SpriteID{a, b} {
		x0, y0, x1, y1 := atlas.UV(id)
		if u-1.5 < x1 && u+1.5 > x0 && v-1.5 < y1 && v+1.5 > y0 {
			t.Errorf("the white patch round (%v, %v) overlaps sprite %d", u, v, id)
		}
	}
	if w, h := atlas.Atlas().Bounds().Dx(), atlas.Atlas().Bounds().Dy(); u+1.5 > float32(w) || v+1.5 > float32(h) {
		t.Errorf("the white patch round (%v, %v) runs off the %dx%d sheet", u, v, w, h)
	}
}
