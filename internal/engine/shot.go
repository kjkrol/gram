package engine

import (
	"image"
	"image/png"
	"log"
	"os"
	"strconv"

	"github.com/kjkrol/gram/render"
)

// a frame saved as a PNG for comparing pictures: till then the world takes one step a frame, so
// the frame shows the same moment whatever the machine's pace
type shot struct {
	path  string
	at    int
	frame int
}

func shotFromEnv() *shot {
	path := os.Getenv("GRAM_SHOT")
	if path == "" {
		return nil
	}
	at := 120
	if n, err := strconv.Atoi(os.Getenv("GRAM_SHOT_FRAME")); err == nil && n > 0 {
		at = n
	}
	return &shot{path: path, at: at}
}

// take saves img once its frame has come, reporting whether it did
func (s *shot) take(img *render.Image) bool {
	s.frame++
	if s.frame < s.at {
		return false
	}
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
	pix := make([]byte, 4*w*h)
	img.ReadPixels(pix)
	out := &image.RGBA{Pix: pix, Stride: 4 * w, Rect: image.Rect(0, 0, w, h)}
	f, err := os.Create(s.path)
	if err != nil {
		log.Printf("gram: no shot: %v", err)
		return true
	}
	defer f.Close()
	if err := png.Encode(f, out); err != nil {
		log.Printf("gram: no shot: %v", err)
		return true
	}
	log.Printf("gram: frame %d saved in %s", s.frame, s.path)
	return true
}
