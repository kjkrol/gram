// Package distant puts the distant light into the composer's shader: its uniforms and lightWay
// (shaders/light.wgsl), for the light plugin and what bakes and casts under it alike.
package distant

import (
	"embed"

	"github.com/kjkrol/gram/render"
)

//go:embed shaders/*.wgsl
var shaders embed.FS

var _ = render.RegisterMaterials(render.Files(shaders, "shaders/light.wgsl"), []render.Uniform{
	{Name: "LightWay", Size: 3},
	{Name: "LightStrength", Size: 1},
	{Name: "LightColor", Size: 3},
	{Name: "AmbientColor", Size: 3},
	{Name: "Ambience", Size: 3},
})
