package compose

import (
	"image"
	"slices"
	"testing"

	"github.com/kjkrol/gram/render/gputest"
)

// level ground of 129 by 129 corners 8 apart: a shade 1024 pixels a side, a pixel a world unit,
// in four by four tiles
var tiled = Lattice{Cols: 129, Rows: 129, Cell: 8}

// plans a frame of the shade of tiled under eastAt45 with casts standing, and the tiles planned
func planSets(b *Baker, casts ...BlockSet) []int {
	b.Plan(tiled, eastAt45, casts)
	return slices.Sorted(slices.Values(b.plan.tiles))
}

// bakes what the last plan said, over level ground without heights
func bakeSets(b *Baker) {
	b.Bake(UniformsOf(map[string]any{"LightWay": eastAt45[:], "LightStrength": []float32{1}}))
}

func post(x, y, top float32) Block { return Block{X: x, Y: y, UX: 1, Along: 4, Wide: 4, Top: top} }

// the sun reaching the ground at the world point (x, y), a pixel a world unit
func sunAt(b *Baker, x, y int) byte {
	pix := make([]byte, 4)
	b.baked.SubImage(image.Rect(x, y, x+1, y+1)).ReadPixels(pix)
	return pix[0]
}

func shadeOf(b *Baker) []byte {
	pix := make([]byte, 4*b.baked.Bounds().Dx()*b.baked.Bounds().Dy())
	b.baked.ReadPixels(pix)
	return pix
}

// the tallest of the posts: a set changing beside it bakes no more than its tiles
var tallest = setOf(post(100, 900, 40))

func TestBaker_BakesAnewTheTilesAChangedSetsShadowsReach(t *testing.T) {
	gputest.Need(t)
	b := &Baker{}
	planSets(b, tallest, setOf())
	if b.plan.from != 0 || b.plan.to != 1024 {
		t.Fatalf("the first plan bakes rows %d to %d, want the whole", b.plan.from, b.plan.to)
	}
	bakeSets(b)
	// 30 tall in a sun 45° high in the east: its shadow 30 west, all within tile (2, 2)
	if got := planSets(b, tallest, setOf(post(600, 600, 30))); !slices.Equal(got, []int{10}) ||
		b.plan.from != b.plan.to {
		t.Errorf("a post added bakes the tiles %v and rows %d to %d, want tile 10 alone", got,
			b.plan.from, b.plan.to)
	}
	bakeSets(b)
	if v := sunAt(b, 585, 600); v > 10 {
		t.Errorf("15 west of the post added the sun reaches %d, want its shadow", v)
	}
	if b.Plan(tiled, eastAt45, []BlockSet{tallest, setOf(post(600, 600, 30))}); b.Pending() {
		t.Error("nothing changed, yet the frame bakes")
	}
	// at the corner of four tiles, and the one it left
	if got := planSets(b, tallest, setOf(post(512, 520, 30))); !slices.Equal(got,
		[]int{5, 6, 9, 10}) {
		t.Errorf("the post moved to a corner bakes the tiles %v, want 5, 6, 9 and 10", got)
	}
	bakeSets(b)
	if v := sunAt(b, 585, 600); v < 245 {
		t.Errorf("where the post stood the sun reaches %d, want all of it", v)
	}
	if got := planSets(b, tallest); !slices.Equal(got, []int{5, 6, 9, 10}) {
		t.Errorf("the post's set gone bakes the tiles %v, want those it shaded", got)
	}
	bakeSets(b)
	if v := sunAt(b, 497, 520); v < 245 {
		t.Errorf("the post's set gone, the sun reaches %d west of it, want all of it", v)
	}
	planSets(b, tallest, setOf(post(600, 600, 30)))
	bakeSets(b)
	if got := planSets(b, tallest, setOf()); !slices.Equal(got, []int{10}) {
		t.Errorf("the post's set emptied bakes the tiles %v, want 10", got)
	}
	bakeSets(b)
	if v := sunAt(b, 585, 600); v < 245 || b.Blocks() != 1 {
		t.Errorf("the post's set emptied, the sun reaches %d west of it, %d blocks stand; want "+
			"all of it and the tallest alone", v, b.Blocks())
	}
}

func TestBaker_BakesTheTilesAFewAFrameTheLongestWaitingFirst(t *testing.T) {
	gputest.Need(t)
	b := &Baker{}
	planSets(b, tallest, setOf())
	bakeSets(b)
	var posts []Block
	for i := range 16 {
		posts = append(posts, post(128+256*float32(i%4), 128+256*float32(i/4), 30))
	}
	if got := planSets(b, tallest, setOf(posts...)); !slices.Equal(got,
		[]int{0, 1, 2, 3, 4, 5, 6, 7}) {
		t.Errorf("a post in every tile bakes first the tiles %v, want the first %d", got,
			tilesPerFrame)
	}
	bakeSets(b)
	// the tallest moved from tile 12, waiting, to tile 3, baked
	moved := setOf(post(900, 100, 40))
	if got := planSets(b, moved, setOf(posts...)); !slices.Equal(got,
		[]int{8, 9, 10, 11, 12, 13, 14, 15}) {
		t.Errorf("next the tiles %v are baked, want the 8 waiting since before", got)
	}
	bakeSets(b)
	if got := planSets(b, moved, setOf(posts...)); !slices.Equal(got, []int{3}) {
		t.Errorf("last the tiles %v are baked, want the tallest's new one, 3", got)
	}
	bakeSets(b)
	if b.Plan(tiled, eastAt45, []BlockSet{moved, setOf(posts...)}); b.Pending() {
		t.Error("every tile baked, yet the frame bakes")
	}
	for i, p := range posts {
		if v := sunAt(b, int(p.X)-15, int(p.Y)); v > 10 {
			t.Errorf("15 west of post %d the sun reaches %d, want its shadow", i, v)
		}
	}
}

// within a pixel's colour a step either way
func nearlyAlike(t *testing.T, what string, got, want []byte) {
	t.Helper()
	for i := range want {
		if d := int(got[i]) - int(want[i]); d > 1 || d < -1 {
			p := i / 4
			t.Fatalf("%s: the pixel (%d, %d) holds %d, want %d", what, p%1024, p/1024, got[i],
				want[i])
		}
	}
}

func TestBaker_BakesATileAsTheWholeIsBaked(t *testing.T) {
	gputest.Need(t)
	// posts here and there, one at a corner of four tiles, a deck across a tile's edge and one
	// lying askew
	blocks := []Block{post(300, 300, 20), post(512, 512, 35), post(700, 260, 25),
		{X: 512, Y: 700, UX: 1, Along: 60, Wide: 8, Bottom: 20, Top: 28},
		{X: 300, Y: 600, UX: 0.6, UY: 0.8, Along: 40, Wide: 6, Bottom: 10, Top: 16}}
	whole := &Baker{}
	planSets(whole, tallest, setOf(blocks...))
	bakeSets(whole)
	b := &Baker{}
	planSets(b, tallest, setOf())
	bakeSets(b)
	if got := planSets(b, tallest, setOf(blocks...)); len(got) == 0 || b.plan.from < b.plan.to {
		t.Fatalf("the blocks added bake the tiles %v and rows %d to %d, want tiles alone", got,
			b.plan.from, b.plan.to)
	}
	for range 16 {
		bakeSets(b)
		planSets(b, tallest, setOf(blocks...))
	}
	nearlyAlike(t, "baked by tiles", shadeOf(b), shadeOf(whole))
}

// a bridge's handle casts its deck's shadow, the pieces casting none: as the pieces would cast it
func TestBaker_BakesABridgesHandleAcrossTilesAsItsPiecesWouldCastIt(t *testing.T) {
	gputest.Need(t)
	handle := Block{X: 512, Y: 400, UX: 1, Along: 48, Wide: 8, Bottom: 30, Top: 40}
	piece := handle
	piece.Along = 16
	var pieces []Block
	for _, x := range []float32{480, 512, 544} {
		piece.X = x
		pieces = append(pieces, piece)
	}
	shade := func(deck ...Block) []byte {
		b := &Baker{}
		planSets(b, tallest, setOf())
		bakeSets(b)
		if got := planSets(b, tallest, setOf(deck...)); !slices.Equal(got, []int{5, 6}) {
			t.Fatalf("the deck bakes the tiles %v, want 5 and 6 either side of x 512", got)
		}
		bakeSets(b)
		return shadeOf(b)
	}
	want := shade(handle)
	nearlyAlike(t, "a deck of three pieces", shade(pieces...), want)
	if v := want[4*(400*1024+470)]; v > 10 {
		t.Errorf("under the deck's shadow the sun reaches %d, want none: the test sees nothing", v)
	}
}
