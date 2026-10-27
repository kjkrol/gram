package gpu

import "testing"

func TestWritePixels_ABigPartLandsInPlaceAndLeavesTheRest(t *testing.T) {
	needGPU(t)
	const w, h = 300, 300
	tex := NewTexture(w, h)
	all := make([]byte, 4*w*h)
	for i := range w * h {
		all[4*i], all[4*i+3] = byte(i%200), 255
	}
	tex.WritePixels(0, 0, w, h, all)
	const pw, ph, px, py = 260, 260, 20, 20
	if pw*ph <= maxPut {
		t.Fatalf("%dx%d pixels go by the draws' uniforms; the test wants the queue's write", pw, ph)
	}
	part := make([]byte, 4*pw*ph)
	for i := range pw * ph {
		part[4*i+1], part[4*i+3] = byte(i%100), 255
	}
	tex.WritePixels(px, py, pw, ph, part)
	got := make([]byte, 4*w*h)
	tex.ReadPixels(got)
	for y := range h {
		for x := range w {
			want := [4]byte{byte((y*w + x) % 200), 0, 0, 255}
			if x >= px && x < px+pw && y >= py && y < py+ph {
				want = [4]byte{0, byte(((y-py)*pw + x - px) % 100), 0, 255}
			}
			if p := pixel(got, w, x, y); p != want {
				t.Fatalf("pixel (%d, %d) is %v, want %v", x, y, p, want)
			}
		}
	}
}

func TestWritePixels_TwoTexturesWrittenWholeInARowBothLand(t *testing.T) {
	needGPU(t)
	const w, h = 300, 300
	var texs [2]*Texture
	for k := range texs {
		texs[k] = NewTexture(w, h)
		all := make([]byte, 4*w*h)
		for i := range w * h {
			all[4*i+k], all[4*i+3] = byte(i%250), 255
		}
		texs[k].WritePixels(0, 0, w, h, all)
	}
	for k, tex := range texs {
		got := make([]byte, 4*w*h)
		tex.ReadPixels(got)
		for i := range w * h {
			want := [4]byte{0, 0, 0, 255}
			want[k] = byte(i % 250)
			if p := pixel(got, w, i%w, i/w); p != want {
				t.Fatalf("texture %d pixel %d is %v, want %v", k, i, p, want)
			}
		}
	}
}
