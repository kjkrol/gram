package world

import "testing"

func TestDrawing_OverlayAsWithShapeTheLayers(t *testing.T) {
	layers := []Appearance{{SpriteID: 1}}
	d := Drawing{Layers: &layers}
	d.Overlay(Appearance{SpriteID: 9})
	d.As(Appearance{SpriteID: 2})
	d.With(func(a Appearance) Appearance { a.SpriteID++; return a })
	if len(layers) != 2 || layers[0].SpriteID != 3 || layers[1].SpriteID != 9 {
		t.Errorf("layers = %v, want the own sprite 2+1 under an overlay of 9", layers)
	}
}
