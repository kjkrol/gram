package forest

import (
	"fmt"
	"math/rand/v2"
	"slices"

	"github.com/kjkrol/aabbworld/geom"
	"github.com/kjkrol/ceremony/act"
	"github.com/kjkrol/ceremony/cond"
	"github.com/kjkrol/gram/entity/kind"
	"github.com/kjkrol/gram/internal/f32"
	"github.com/kjkrol/gram/lod"
	"github.com/kjkrol/gram/plugins/board"
	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/world"
	"github.com/kjkrol/uid"
)

// DefaultDensity is how many trees a cell a Stand plants where its Density is zero.
const DefaultDensity = 1.5

// Stand is how a forest plants its trees: on which kinds of cells, how many a cell, how far each
// strays from its plot, and which kinds of trees at which heights of the ground.
type Stand struct {
	// Cells are the names of the kinds of cells its trees grow on; every cell where it names none.
	// No tree stands on a way's band or under a deck, nor within half the widest crown of the Mix
	// and a quarter of the way's width of its band.
	Cells []string
	// Density is how many trees a cell it plants, DefaultDensity where it is zero.
	Density float32
	// Jitter is how far a tree strays from its plot each way, a share of the plots' spacing up to
	// a half.
	Jitter float32
	// Mix is the kinds of trees it plants, each plot's drawn by weight among those growing at the
	// height of the ground there.
	Mix []Share
	// When is what the forest must hold for the stand to plant, an effect of its own say; always
	// where it is nil.
	When cond.Cond
}

// Share is a kind of tree in a Stand's Mix: its name on the Types, its Weight against the others
// growing where it does (1 where it is zero), and the band of the ground's height it grows in,
// From up to To in world height units, zero for no bound.
type Share struct {
	Tree     string
	Weight   float32
	From, To float32
}

// grows reports whether the share grows on ground h high
func (s Share) grows(h float32) bool {
	return (s.From == 0 || h >= s.From) && (s.To == 0 || h < s.To)
}

// Stands makes the roles that plant a game's forests, in the Rules section.
type Stands struct {
	p    *Plugin
	made []*planting
}

// Define defines the role named name, with rules beside, which a kind of forest (a world.Field)
// plays to plant its trees as stand says in each sector of it come down to L3.
func (s *Stands) Define(name string, stand Stand, rules ...act.Rule) {
	pl := s.planting(name, stand)
	s.made = append(s.made, pl)
	all := make([]act.Rule, 0, len(pl.plots)+len(rules))
	for k := range pl.plots {
		all = append(all, act.Branch[world.Unfold]{
			Name: fmt.Sprintf("plant %d", k),
			When: pl.sows(k, stand.When),
			Then: pl.plants(k),
		})
	}
	s.p.worldPlugin.Roles().Define(name, append(all, rules...)...)
}

// Felled is the word of a stretch of a forest folding with fewer trees than stand plants there:
// some were felled, or went otherwise.
func (s *Stands) Felled(stand Stand) cond.Word[world.Fold] {
	pl := s.planting("felled", stand)
	return cond.NewWord("felled", func(f world.Fold) bool {
		return f.Parts < pl.count(f.ID, f.Sector, f.Box)
	})
}

// check panics where a stand mixes in a kind of tree the types never defined
func (s *Stands) check(t *Types) {
	for _, pl := range s.made {
		for _, sh := range pl.mix {
			if _, ok := t.specs[sh.Tree]; !ok {
				panic(fmt.Sprintf("forest: stand %q mixes in %q, which is no kind of tree", pl.name,
					sh.Tree))
			}
		}
	}
}

// planting is a Stand laid over the world's sectors: its plots off the middle of a forest's part
// of a sector, on the board's cells, and each shaken as its tree stands
type planting struct {
	p      *Plugin
	name   string
	cells  []cell.Name
	share  uint64 // of every 1<<16 plots, those sown
	plots  []geom.Vec
	shaken []geom.Vec
	mix    []Share
	banded bool
	crown  float32 // the widest crown of the mix, read once the kinds are defined
}

func (s *Stands) planting(name string, stand Stand) *planting {
	if len(stand.Mix) == 0 {
		panic(fmt.Sprintf("forest: stand %q plants no kind of tree", name))
	}
	if stand.Jitter < 0 || stand.Jitter > 0.5 {
		panic(fmt.Sprintf("forest: stand %q: its Jitter is a share of the plots' spacing up to "+
			"a half", name))
	}
	density := stand.Density
	if density == 0 {
		density = DefaultDensity
	}
	if density < 0 {
		panic(fmt.Sprintf("forest: stand %q plants %v trees a cell", name, density))
	}
	pl := &planting{p: s.p, name: name}
	for _, c := range stand.Cells {
		pl.cells = append(pl.cells, cell.Named(c))
	}
	for _, sh := range stand.Mix {
		if sh.Weight < 0 {
			panic(fmt.Sprintf("forest: stand %q weighs %q at %v", name, sh.Tree, sh.Weight))
		}
		if sh.Weight == 0 {
			sh.Weight = 1
		}
		pl.banded = pl.banded || sh.From != 0 || sh.To != 0
		pl.mix = append(pl.mix, sh)
	}
	// a cell cut in k by k squares, the plots on every other square as the black ones of a
	// chessboard: the least even k sowing density
	k := 2
	for float32(k*k)/2 < density {
		k += 2
	}
	pl.share = uint64(min(f32.Round(density/(float32(k*k)/2)*(1<<16)), 1<<16))
	cw, ch := s.p.board.Read(0).Board().Grid.CellBounds()
	pl.lay(sectorSide(s.p.worldPlugin), cw/float32(k), ch/float32(k), stand.Jitter)
	return pl
}

// lay lays the plots of a sector side wide on squares qx by qy, row pairs one after another, and
// shakes them jitter of a square each way; the shaking is the same in every forest
func (pl *planting) lay(side, qx, qy, jitter float32) {
	cols, rows := int(f32.Ceil(side/qx)), int(f32.Ceil(side/qy))
	shake := rand.New(rand.NewPCG(7, 11))
	jx, jy := jitter*qx, jitter*qy
	for j := 0; 2*j < rows; j++ {
		for i := range cols {
			r := 2*j + i%2
			if r >= rows {
				continue
			}
			at := geom.NewVec((float32(i)+0.5)*qx-side/2, (float32(r)+0.5)*qy-side/2)
			pl.plots = append(pl.plots, at)
			pl.shaken = append(pl.shaken,
				at.Add(geom.NewVec((shake.Float32()*2-1)*jx, (shake.Float32()*2-1)*jy)))
		}
	}
}

// sows is when the k-th plot of a sector of the forest is sown: when holds, then the plot's word
func (pl *planting) sows(k int, when cond.Cond) cond.Cond {
	sown := cond.Self.If(cond.NewWord(fmt.Sprintf("plot %d", k), func(u world.Unfold) bool {
		return pl.sown(u.ID, u.Sector, u.Box, k)
	}))
	if when == nil {
		return sown
	}
	return when.And(sown)
}

// plants is what the k-th plot of a sector of the forest sown does: spawns the tree its seed
// draws among those growing there
func (pl *planting) plants(k int) act.Step {
	spawn := func(sh Share) act.Step {
		tree := kind.EntryOf(sh.Tree, treeRow{})
		return act.Self.Do(world.SpawnPart{Entry: tree, At: pl.shaken[k]})
	}
	if len(pl.mix) == 1 && !pl.banded {
		return spawn(pl.mix[0])
	}
	kinds := make([]act.Step, len(pl.mix))
	for i, sh := range pl.mix {
		drawn := cond.NewWord(fmt.Sprintf("plot %d %s", k, sh.Tree), func(u world.Unfold) bool {
			return pl.drawn(u.ID, u.Sector, u.Box, k) == i
		})
		kinds[i] = act.Branch[world.Unfold]{When: cond.Self.If(drawn), Then: spawn(sh)}
	}
	return act.OneOf(kinds...)
}

// sown reports whether the k-th plot of the forest whole's part box of sector holds a tree: the
// plot in the part, the tree's spot on a cell it grows on and off every road and deck, picked by
// the forest's seed
func (pl *planting) sown(whole uid.UID64, sector int, box geom.AABB, k int) bool {
	mid := middle(box)
	return within(box, mid.Add(pl.plots[k])) && pl.grows(mid.Add(pl.shaken[k])) &&
		pl.seed(whole, sector, k)&(1<<16-1) < pl.share
}

// drawn is the share of the Mix whose tree stands on the k-th plot of the forest whole's part box
// of sector, -1 for none growing at the height of the ground there
func (pl *planting) drawn(whole uid.UID64, sector int, box geom.AABB, k int) int {
	if len(pl.mix) == 1 && !pl.banded {
		return 0
	}
	var h float32
	if heights := pl.p.board.Read(0).Heights(); heights != nil {
		h = heights.At(middle(box).Add(pl.shaken[k]))
	}
	return pick(pl.mix, h, pl.seed(whole, sector, k)>>16)
}

// count is how many trees the forest whole's part box of sector holds as planted
func (pl *planting) count(whole uid.UID64, sector int, box geom.AABB) int {
	n := 0
	for k := range pl.plots {
		if pl.sown(whole, sector, box, k) && pl.drawn(whole, sector, box, k) >= 0 {
			n++
		}
	}
	return n
}

func (pl *planting) seed(whole uid.UID64, sector, k int) uint64 {
	return lod.Seed(whole, uint64(sector*len(pl.plots)+k))
}

// grows reports whether a tree stands at at: on a cell of a kind the trees grow on, clear of the
// ways running across that cell and its neighbours by half its crown and a quarter of the way's
// width, and under no deck
func (pl *planting) grows(at geom.Vec) bool {
	bd := pl.p.board.Read(0).Board()
	c, ok := bd.Grid.CellAt(at)
	if !ok {
		return false
	}
	if len(pl.cells) > 0 && !slices.Contains(pl.cells, bd.Bare(c).Name) {
		return false
	}
	q := at.Sub(bd.Grid.CellCenter(c))
	if pl.onWay(bd, bd.Way(c), q) {
		return false
	}
	for j := range bd.Grid.Directions() {
		if m, ok := bd.Grid.Toward(c, j); ok && pl.onWay(bd, bd.Way(m), q.Sub(bd.Grid.Step(j))) {
			return false
		}
	}
	decked := false
	bd.DecksOver(at, func(cell.ID, cell.Floor) { decked = true })
	return !decked
}

// onWay reports whether q, seen from the middle of the cell way runs across, lies within the
// stand's margin of the way's band: its hub and the strip out to each neighbour it links
func (pl *planting) onWay(bd *board.BoardRead, way cell.Way, q geom.Vec) bool {
	if !way.Runs() {
		return false
	}
	half := way.Width/2 + way.Width/4 + pl.widest()/2
	if q.X*q.X+q.Y*q.Y <= half*half {
		return true
	}
	for i := range bd.Grid.Directions() {
		if !way.Links.Has(i) {
			continue
		}
		s := bd.Grid.Step(i)
		l := f32.Hypot(s.X, s.Y)
		ux, uy := s.X/l, s.Y/l
		along := q.X*ux + q.Y*uy
		if along >= 0 && along <= l/2 && f32.Abs(q.Y*ux-q.X*uy) <= half {
			return true
		}
	}
	return false
}

// widest is the widest crown among the stand's kinds of trees
func (pl *planting) widest() float32 {
	if pl.crown == 0 {
		for _, sh := range pl.mix {
			pl.crown = max(pl.crown, pl.p.types.specs[sh.Tree].Crown)
		}
	}
	return pl.crown
}

// pick is the share of mix a plot on ground h high draws by seed among those growing there, -1
// where none does
func pick(mix []Share, h float32, seed uint64) int {
	var total float32
	for _, sh := range mix {
		if sh.grows(h) {
			total += sh.Weight
		}
	}
	if total == 0 {
		return -1
	}
	r := float32(seed&(1<<24-1)) / (1 << 24) * total
	last := -1
	for i, sh := range mix {
		if !sh.grows(h) {
			continue
		}
		if r < sh.Weight {
			return i
		}
		r -= sh.Weight
		last = i
	}
	return last
}

// sectorSide is the side of w's sectors; one sector holding the world whole where it has none
func sectorSide(w *world.Plugin) float32 {
	if side := w.Read(0).SectorSide(); side > 0 {
		return side
	}
	space := w.Res.Config.Space
	return float32(max(space.Width, space.Height))
}

func middle(box geom.AABB) geom.Vec {
	return geom.NewVec((box.TopLeft.X+box.BottomRight.X)/2, (box.TopLeft.Y+box.BottomRight.Y)/2)
}

func within(box geom.AABB, at geom.Vec) bool {
	return at.X >= box.TopLeft.X && at.X < box.BottomRight.X && at.Y >= box.TopLeft.Y &&
		at.Y < box.BottomRight.Y
}
