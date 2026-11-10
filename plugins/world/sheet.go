package world

import (
	"image"
	"math"
	"slices"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/gram/internal/f32"
	"github.com/kjkrol/gram/lod"
	"github.com/kjkrol/gram/plugins/world/sector"
	"github.com/kjkrol/gram/plugins/world/view"
	"github.com/kjkrol/gram/render"
)

// SheetImage is the name the shaders read a sheet's albedo by ([Read.Sheet]), a frame image
// (render.RegisterImage) a renderer binds, at L4 or at L5, where it draws a sector so.
const SheetImage = "Sheet"

// SheetReliefImage is the name the shaders read a sheet's relief by ([Read.Sheet]), bound as
// SheetImage is.
const SheetReliefImage = "SheetRelief"

func init() {
	render.RegisterImage(SheetImage)
	render.RegisterImage(SheetReliefImage)
}

// Sheets has the world photograph what stands into the sheets [Read.Sheet] gives, for a renderer
// drawing them over the ground it draws; a world none asks it of spends nothing on them.
func (p *Plugin) Sheets() { p.renderer.sheet.open() }

const (
	// the most texels a sheet spans a side
	sheetSide = 4096
	// the most sectors photographed a frame, to start with
	sheetPerFrame = 8
)

// tiling is how a world is cut into sectors, as the sheets lay them out
type tiling interface {
	Len() int
	Cols() int
	Box(i int) geom.AABB
	Of(x, y float32) int
	Neighbours(i int, each func(j int))
}

// tierGrid is how a world's tiers of cells lie over its sectors, as sector.Grid lays them
type tierGrid interface {
	Tiers() int
	TierLen(k int) int
	Fit(box geom.AABB, small float32) (tier, cell int)
	Side() float32
}

// sheets are the photographs of what stands over the whole world, perUnit texels a world unit,
// cols by rows at L4 and half as many each way at L5, made as a renderer first asks; by sector the
// version of what stands in it, from 1, and the version its photograph holds, 0 for none; shots,
// how many sectors hold a photograph of something; perFrame, the most sectors photographed a
// frame. What the publishing keeps of them is the game's goroutine's, the darkroom the frame's.
type sheets struct {
	albedo, relief     *render.Image
	albedoL5, reliefL5 *render.Image
	perUnit            float32
	cols, rows         int
	grid               tiling
	version, painted   []uint32
	shots              int
	perFrame           int
	// by sector: what stood in it as it was last signed and what stands in it as this publish
	// finds it, whether its photograph holds anything, whether a view draws it at L4 or coarser
	signed, now   []signature
	inked, wanted []bool
	// the sector the next round of photographs begins from
	next int
	// by sector: the level the view being read draws it at, whether some view draws it at L4
	// or coarser beside one it draws finer, a seam, and whether some view draws it finer than
	// L4; beside marks a neighbour of a finer sector
	seen   []lod.Level
	ring   []bool
	finer  []bool
	beside func(j int)
	// the tiers of cells over the sectors, nil where the grid lays none; the longest a thing
	// standing lives by its middle; where each tier's cells begin among a view's levels of them
	tiers tierGrid
	small float32
	slots []int32
	// what stands in the sectors wanted, a photograph's instance each, and how many records a
	// photograph held may leave out
	cands     []candidate
	droppable int
	// the parts of the fields folded with no look declared at L4, by stretch, and by sector
	// whether some lie in it
	folded  map[place][]shot
	snapped []bool
	// by sector whether its photograph waits for the parts of a field a preview gives, and the
	// stretches of fields with no look declared at L4 a step found neither unfolded nor kept
	pending []bool
	bare    []place
	want    func(i int, box geom.AABB, level lod.Level)
	room    darkroom
	files   SheetsCfg
	// the tiles of the files read and written, nil without a folder
	store *sheetStore
}

// signature is what stands in a sector as a publish signs it: how many, and their identities,
// looks, tops and places mixed
type signature struct {
	n   uint32
	mix uint64
}

// candidate is a thing standing in a sector, as its photograph lays it
type candidate struct {
	sector int
	inst   shot
}

// shot is a thing as a photograph lays it (sheet_photo.wgsl): its middle and half its length and
// width, its sprite's corners on the atlas, its altitude and height, 1 where it lies flat, and the
// way it is headed in radians
type shot [photoFloats]float32

const photoFloats = 12

// photo is a sector to photograph: its texels at L4, its instances among the frame's and the
// most any reaches up or down, for the depth; what stands in it as signed and whether anything does
type photo struct {
	sector   int
	rect     image.Rectangle
	from, to int
	span     float32
	signed   signature
	full     bool
}

// sheetFrame is what a publish hands the frame of the sheets: whether they are photographed, by
// sector the version of its photograph where it holds what stands in it now, something standing
// there, 0 where not, whether any does, the photographs to develop with their instances, whether
// the frame developed them, and by view published the level it draws each cell of the tiers at
type sheetFrame struct {
	on, live  bool
	held      []uint32
	photos    []photo
	instances []float32
	developed bool
	tiered    []tierLevels
}

// tierLevels is the level a view draws each cell of the tiers at, the finest of the sectors under
// it, by its slot: the sectors first, then each tier's cells in turn; none in a view of no sectors
type tierLevels struct {
	v      *view.View
	levels []lod.Level
}

// at is the level of the cell at slot tier-1, lod.Standard in a view of no sectors, as View.Level
func (t *tierLevels) at(tier int32) lod.Level {
	if int(tier) > len(t.levels) {
		return lod.Standard
	}
	return t.levels[tier-1]
}

// tiersOf is the levels v draws the cells of the tiers at, nil where f has none of it
func (f *sheetFrame) tiersOf(v *view.View) *tierLevels {
	for k := range f.tiered {
		if f.tiered[k].v == v {
			return &f.tiered[k]
		}
	}
	return nil
}

// newSheets is the sheets of a world cut by grid, none without: a texel of the ladder's Texel,
// for none the world units a pixel spans at L4's threshold, or a sector's side over 64 without
// one, raised till the world spans sheetSide at most
func newSheets(cfg Config, grid *sector.Grid) sheets {
	if grid == nil {
		return sheets{}
	}
	texel := cfg.LOD.Texel
	if l4 := cfg.LOD.Scales[lod.Strategic]; texel <= 0 && l4 > 0 {
		texel = 1 / l4
	}
	if texel <= 0 {
		texel = grid.Side() / 64
	}
	w, h := float32(cfg.Space.Width), float32(cfg.Space.Height)
	texel = max(texel, max(w, h)/sheetSide)
	s := sheetsOver(grid, 1/texel, min(max(int(f32.Ceil(w/texel)), 1), sheetSide),
		min(max(int(f32.Ceil(h/texel)), 1), sheetSide))
	s.files = cfg.Sheets
	if cfg.Sectors.Small > 0 {
		s.small = cfg.Sectors.Small
	}
	return s
}

// sheetsOver is the sheets of a world cut by grid, perUnit texels a world unit, cols by rows
func sheetsOver(grid tiling, perUnit float32, cols, rows int) sheets {
	n := grid.Len()
	s := sheets{
		perUnit:  perUnit,
		cols:     cols,
		rows:     rows,
		grid:     grid,
		version:  make([]uint32, n),
		painted:  make([]uint32, n),
		perFrame: sheetPerFrame,
		signed:   make([]signature, n),
		now:      make([]signature, n),
		inked:    make([]bool, n),
		wanted:   make([]bool, n),
		snapped:  make([]bool, n),
		pending:  make([]bool, n),
		folded:   map[place][]shot{},
		seen:     make([]lod.Level, n),
		ring:     make([]bool, n),
		finer:    make([]bool, n),
	}
	for i := range s.version {
		s.version[i] = 1
	}
	wanted, seen, ring := s.wanted, s.seen, s.ring
	s.want = func(i int, _ geom.AABB, level lod.Level) {
		seen[i] = level
		if level >= lod.Strategic {
			wanted[i] = true
		}
	}
	s.beside = func(j int) {
		if seen[j] >= lod.Strategic && seen[j] < lod.Omitted {
			ring[j] = true
		}
	}
	if t, ok := grid.(tierGrid); ok {
		s.tiers, s.small = t, t.Side()/4
		for k, at := 0, int32(0); k < t.Tiers(); k++ {
			s.slots = append(s.slots, at)
			at += int32(t.TierLen(k))
		}
	}
	return s
}

// open makes the images, clear, in a world cut into sectors
func (s *sheets) open() {
	if s.cols == 0 || s.albedo != nil {
		return
	}
	s.albedo, s.relief = render.NewImage(s.cols, s.rows), render.NewImage(s.cols, s.rows)
	cols, rows := (s.cols+1)/2, (s.rows+1)/2
	s.albedoL5, s.reliefL5 = render.NewImage(cols, rows), render.NewImage(cols, rows)
	s.openFiles()
}

// bump has sector i photographed anew: what stands in it has changed
func (s *sheets) bump(i int) { s.version[i]++ }

// paint has sector i hold a photograph of what stands in it now, something standing there
func (s *sheets) paint(i int) { s.mark(i, true) }

// loaded has sector i hold a photograph a file kept of what stood in it as signed, something
// standing there where inked
func (s *sheets) loaded(i int, signed signature, inked bool) {
	s.signed[i] = signed
	s.mark(i, inked)
}

// mark has sector i hold a photograph of what stands in it now, full where something does
func (s *sheets) mark(i int, full bool) {
	s.painted[i] = s.version[i]
	switch {
	case full && !s.inked[i]:
		s.shots++
	case !full && s.inked[i]:
		s.shots--
	}
	s.inked[i] = full
}

// holds reports whether sector i holds a photograph of what stands in it now, something standing
// there
func (s *sheets) holds(i int) bool {
	return i >= 0 && i < len(s.painted) && s.painted[i] == s.version[i] && s.inked[i]
}

// held is the version of the photograph sector i holds of what stands in it now, something
// standing there, 0 for none
func (s *sheets) held(i int) uint32 {
	if !s.holds(i) {
		return 0
	}
	return s.painted[i]
}

// live reports whether the images are made and some sector holds a photograph of something
func (s *sheets) live() bool { return s.albedo != nil && s.shots > 0 }

// rect is sector i's texels at L4, the sectors' sharing none and leaving none out
func (s *sheets) rect(i int) image.Rectangle {
	b := s.grid.Box(i)
	edge := func(v float32, n int) int { return min(max(int(f32.Floor(v*s.perUnit)), 0), n) }
	x1, y1 := edge(b.BottomRight.X, s.cols), edge(b.BottomRight.Y, s.rows)
	// the last sector of a row or a column takes the texels past the world's edge
	if s.grid.Of(b.BottomRight.X, b.TopLeft.Y) == i {
		x1 = s.cols
	}
	if s.grid.Of(b.TopLeft.X, b.BottomRight.Y) == i {
		y1 = s.rows
	}
	return image.Rect(edge(b.TopLeft.X, s.cols), edge(b.TopLeft.Y, s.rows), x1, y1)
}

// halfOf is the texels at L5 the texels r at L4 lie in
func halfOf(r image.Rectangle) image.Rectangle {
	return image.Rect(r.Min.X/2, r.Min.Y/2, (r.Max.X+1)/2, (r.Max.Y+1)/2)
}

// recall has the sectors of photographs a frame never developed photographed anew, then clears
// f for this publish
func (s *sheets) recall(f *sheetFrame) {
	if !f.developed {
		for _, p := range f.photos {
			s.painted[p.sector] = 0
		}
	}
	f.photos, f.instances, f.developed = f.photos[:0], f.instances[:0], false
}

// begin readies a publish through views: false where no sheets are made, the records published
// as they are
func (s *sheets) begin(f *sheetFrame, views []*view.View) bool {
	f.on = false
	if s.albedo == nil || s.grid == nil {
		return false
	}
	clear(s.wanted)
	clear(s.now)
	clear(s.ring)
	clear(s.finer)
	s.cands, s.droppable = s.cands[:0], 0
	f.tiered = slices.Grow(f.tiered[:0], len(views))[:len(views)]
	for k, v := range views {
		t := &f.tiered[k]
		t.v, t.levels = v, t.levels[:0]
		if v.Grid() == nil {
			continue
		}
		for i := range s.seen {
			s.seen[i] = lod.Omitted
		}
		v.Sectors(s.want)
		for i, level := range s.seen {
			if level < lod.Strategic {
				s.finer[i] = true
				s.grid.Neighbours(i, s.beside)
			}
		}
		if s.tiers != nil {
			t.levels = s.climb(t.levels)
		}
	}
	return true
}

// climb appends to dst the level the view whose sectors' levels seen holds draws each cell of the
// tiers at, tier by tier, each the finest of the two by two cells of the tier below under it
func (s *sheets) climb(dst []lod.Level) []lod.Level {
	dst = append(dst, s.seen...)
	cols := s.grid.Cols()
	rows, from := len(s.seen)/cols, 0
	for range len(s.slots) - 1 {
		up, down := (cols+1)/2, (rows+1)/2
		for r := range down {
			for c := range up {
				at := from + 2*r*cols + 2*c
				right, below := 2*c+1 < cols, 2*r+1 < rows
				l := dst[at]
				if right {
					l = min(l, dst[at+1])
				}
				if below {
					l = min(l, dst[at+cols])
				}
				if right && below {
					l = min(l, dst[at+cols+1])
				}
				dst = append(dst, l)
			}
		}
		from += cols * rows
		cols, rows = up, down
	}
	return dst
}

// slotOf is the slot among a view's levels of the cells of the tiers of the cell a thing standing
// over box lives in (sector.Grid.Fit)
func (s *sheets) slotOf(box geom.AABB) int32 {
	tier, cell := s.tiers.Fit(box, s.small)
	return s.slots[tier] + int32(cell)
}

// seams reports whether the box of a thing standing whose middle is mid crosses from a sector into
// another, and near, whether it does or its middle's sector is some view's at a seam: else every
// view draws it at its middle's level
func (s *sheets) seams(box geom.AABB, mid geom.Vec) (crosses, near bool) {
	g := s.grid
	crosses = g.Of(box.TopLeft.X, box.TopLeft.Y) != g.Of(box.BottomRight.X, box.BottomRight.Y)
	return crosses, crosses || s.ring[g.Of(mid.X, mid.Y)]
}

// wants reports whether a view draws at L4 or coarser a sector box lies in
func (s *sheets) wants(box geom.AABB) bool {
	g := s.grid
	tl, br := box.TopLeft, box.BottomRight
	return s.wanted[g.Of(tl.X, tl.Y)] || s.wanted[g.Of(br.X, br.Y)] ||
		s.wanted[g.Of(br.X, tl.Y)] || s.wanted[g.Of(tl.X, br.Y)]
}

// stand signs a thing standing over box into each sector it lies in a view draws at L4 or
// coarser, h mixing what it is, a candidate for its photograph laid as at
func (s *sheets) stand(box geom.AABB, h uint64, at *shot) {
	g := s.grid
	first, last := g.Of(box.TopLeft.X, box.TopLeft.Y), g.Of(box.BottomRight.X, box.BottomRight.Y)
	cols := g.Cols()
	for r := first / cols; r <= last/cols; r++ {
		for c := first % cols; c <= last%cols; c++ {
			i := r*cols + c
			if !s.wanted[i] {
				continue
			}
			s.now[i].n++
			s.now[i].mix ^= h
			s.cands = append(s.cands, candidate{sector: i, inst: *at})
		}
	}
}

// signed mixes what a thing standing is: its identity, the sprite it is photographed of, its top
// and its place
func signed(id uint64, sprite render.SpriteID, top float32, at geom.Vec) uint64 {
	return mixed(id^uint64(sprite)<<48) ^
		mixed(uint64(math.Float32bits(top))<<32|uint64(math.Float32bits(at.X+2*at.Y)))
}

// settle has the sectors wanted whose signature changed photographed anew and hands f the
// photographs of this frame, perFrame at most, those wanted first from where the last round
// ended; a sector emptied of what never held anything holds its photograph at once. The
// signature of a sector no view draws beside a finer one takes in what kept, the standers, keep
// of it, over what was signed as the publish walked it.
func (s *sheets) settle(f *sheetFrame, kept *standers) {
	n := len(s.version)
	for i := range n {
		if s.wanted[i] && !s.ring[i] && kept != nil && i < len(kept.sig) {
			s.now[i].n += kept.sig[i].n
			s.now[i].mix ^= kept.sig[i].mix
		}
		if s.wanted[i] && s.now[i] != s.signed[i] {
			s.signed[i] = s.now[i]
			s.version[i]++
		}
	}
	clear(s.snapped)
	for p := range s.folded {
		s.snapped[p.sector] = true
	}
	s.waiting()
	budget := s.perFrame
	for k := range n {
		i := (s.next + k) % n
		if !s.wanted[i] || s.painted[i] == s.version[i] || s.pending[i] {
			continue
		}
		full := s.now[i].n > 0 || s.snapped[i]
		if !full && !s.inked[i] {
			s.mark(i, false)
			continue
		}
		if budget == 0 {
			s.next = i
			break
		}
		budget--
		f.photos = append(f.photos, photo{sector: i, rect: s.rect(i), signed: s.signed[i],
			full: full})
		s.mark(i, full)
	}
	s.gather(f, kept)
	f.on, f.live = true, s.shots > 0
	f.held = f.held[:0]
	for i := range n {
		f.held = append(f.held, s.held(i))
	}
}

// gather lays the instances of f's photographs: what stands in each sector, as the publish
// signed it and as kept, the standers, keep it in a sector beside no finer one, and the parts
// folded there
func (s *sheets) gather(f *sheetFrame, kept *standers) {
	for k := range f.photos {
		p := &f.photos[k]
		p.from, p.span = len(f.instances), 1
		for c := range s.cands {
			if s.cands[c].sector == p.sector {
				f.instances = appendShot(f.instances, &s.cands[c].inst, &p.span)
			}
		}
		if kept != nil && !s.ring[p.sector] {
			f.instances = kept.shots(p.sector, f.instances, &p.span)
		}
		if s.snapped[p.sector] {
			for at, shots := range s.folded {
				if at.sector != p.sector {
					continue
				}
				for j := range shots {
					f.instances = appendShot(f.instances, &shots[j], &p.span)
				}
			}
		}
		p.to = len(f.instances)
	}
}

// appendShot appends at to dst, span the most it reaches up or down from zero at least
func appendShot(dst []float32, at *shot, span *float32) []float32 {
	*span = max(*span, f32.Abs(at[8]+at[9]), f32.Abs(at[8]))
	return append(dst, at[:]...)
}

// fold keeps the parts of the stretch at as the sheets photograph them, none too, at being
// photographed anew
func (s *sheets) fold(at place, parts []shot) {
	s.folded[at] = append(s.folded[at][:0], parts...)
	s.bump(at.sector)
}

// unfold lets go of the parts kept of the stretch at, at being photographed anew
func (s *sheets) unfold(at place) {
	if _, ok := s.folded[at]; ok {
		delete(s.folded, at)
		s.bump(at.sector)
	}
}
