package topography

import (
	"testing"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/kjkrol/gram/plugins/topography/heightfield"
	"github.com/kjkrol/gram/render"
)

// The composer's one shader, the topography's water among its materials, compiles.
func TestShader_Compiles(t *testing.T) {
	if _, err := ebiten.NewShader(render.ShaderSource()); err != nil {
		t.Fatal(err)
	}
}

// The ground traced on the GPU compiles on the composer's library, the topography's water among
// the materials it calls.
func TestHeightfieldShader_Compiles(t *testing.T) {
	if _, err := ebiten.NewShader(heightfield.Source()); err != nil {
		t.Fatal(err)
	}
}
