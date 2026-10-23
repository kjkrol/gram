package compose

import (
	_ "embed"
	"fmt"
	"strings"

	"github.com/kjkrol/gram/render/internal/gfx"
	"github.com/kjkrol/gram/render/internal/wgsl"
)

//go:embed compose.wgsl
var composeWGSL string

var composer = gfx.NewShaderWith("composer", []byte(composeWGSL+"\n"), nil)

// ShaderSource is the composer's shader as it compiles.
func ShaderSource() string { return composer.Source() }

// Compile compiles the composer's shader now rather than at its first draw.
func Compile() error {
	if err := composer.Compile(); err != nil {
		return fmt.Errorf("render: the composer's shader, with %s: %w",
			strings.Join(wgsl.Entries(), ", "), err)
	}
	return nil
}
