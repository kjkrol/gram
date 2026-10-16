package render_test

import (
	"slices"
	"testing"

	"github.com/kjkrol/gram/render"
)

func TestSharedUniforms_AreTheLibrarysAndAreACopy(t *testing.T) {
	got := render.SharedUniforms()
	for _, name := range []string{"Toward", "Clock", "Pixel", "Fog"} {
		if !slices.ContainsFunc(got, func(u render.Uniform) bool { return u.Name == name }) {
			t.Errorf("the shared uniforms %v lack the library's %s", got, name)
		}
	}
	got[0].Name = "Changed"
	if render.SharedUniforms()[0].Name == "Changed" {
		t.Error("changing the shared uniforms handed out changed the library's")
	}
}
