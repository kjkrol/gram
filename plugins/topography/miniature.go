package topography

import (
	"github.com/kjkrol/gram/plugins/minimap"
	"github.com/kjkrol/gram/render"
)

var _ minimap.Ground = (*Write)(nil)

// Miniature is the board painted flat from above, perUnit pixels a world unit, for a minimap to
// lay under its marks: the painter's on a square grid, the hexes' tops on a hex one; nil before
// the board's atlas is closed.
func (w *Write) Miniature() (img *render.Image, perUnit float32) {
	p := w.p
	if p.hexes != nil {
		return p.hexes.Miniature()
	}
	atlas := p.boardPlugin.Read(0).Atlas()
	if atlas == nil {
		return nil, 0
	}
	albedo, _, px, _ := p.painter.Surface(atlas)
	_, _, cell, _, _ := p.relief.Lattice()
	if albedo == nil || cell <= 0 {
		return nil, 0
	}
	return albedo, float32(px) / cell
}
