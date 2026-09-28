package topography

import (
	"testing"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/kjkrol/gram/render"
)

// The composer's one shader, the topography's water among its materials, compiles.
func TestShader_Compiles(t *testing.T) {
	if _, err := ebiten.NewShader(render.ShaderSource()); err != nil {
		t.Fatal(err)
	}
}
