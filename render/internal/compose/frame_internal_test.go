package compose

import (
	"image/color"
	"testing"

	"github.com/kjkrol/gram/plugins/cameras/cameratest"
)

func pieces(f *Frame) [][3]float32 {
	var out [][3]float32
	f.Each(func(at At, v []Vertex) {
		top := float32(0)
		if at.top {
			top = 1
		}
		out = append(out, [3]float32{top, at.key, v[0].DstX})
	})
	return out
}

func TestFrame_AppendTakesBackWhatABranchGatheredInOrder(t *testing.T) {
	cam := cameratest.NewFromSpace(100, 100, 0)
	quad := func(x float32) Corners { return Corners{{x, 0}, {x + 1, 0}, {x, 1}, {x + 1, 1}} }
	draw := func(f *Frame, from, to int) {
		for i := from; i < to; i++ {
			x := float32(i)
			f.Soft(h(x), quad(x), color.RGBA{A: 255}, Fade{})
			f.Line(h(x), x, 0, x+1, 1, 1, color.RGBA{R: 255, A: 255})
			f.SpriteFan(OnTop(0), sheet{}, 0, [][2]float32{{x, 0}, {x + 1, 0}, {x + 1, 1}},
				[4]float32{x, 0, x + 1, 1}, 1)
			f.Uniform("Seen", x)
		}
	}
	var whole Frame
	whole.Reset(cam)
	whole.time = 3
	draw(&whole, 0, 8)

	var main, a, b Frame
	main.Reset(cam)
	main.time = 3
	draw(&main, 0, 2)
	main.Branch(&a)
	main.Branch(&b)
	if a.Time() != 3 || a.cam != cam {
		t.Fatal("a branch does not run at the frame's time through its camera")
	}
	draw(&a, 2, 5)
	draw(&b, 5, 8)
	main.Append(&a)
	main.Append(&b)

	if got, want := pieces(&main), pieces(&whole); len(got) != len(want) || main.Len() != whole.Len() {
		t.Fatalf("appended frame holds %d pieces (%d counted), whole %d (%d)", len(got), main.Len(), len(want), whole.Len())
	} else {
		for i := range got {
			if got[i] != want[i] {
				t.Fatalf("piece %d is %v, want %v", i, got[i], want[i])
			}
		}
	}
	if len(main.items) != len(whole.items) {
		t.Errorf("appended frame holds %d items, whole %d: the runs differ", len(main.items), len(whole.items))
	}
	if len(main.uniforms) != 1 || main.uniforms[0].v[0] != 7 {
		t.Errorf("uniforms %v, want the branches' Seen set on the frame, the last one 7", main.uniforms)
	}
}
