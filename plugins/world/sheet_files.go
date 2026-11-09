package world

import (
	"bufio"
	"errors"
	"fmt"
	"image"
	"image/png"
	"io"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"

	"go.yaml.in/yaml/v3"

	"github.com/kjkrol/gram/render"
)

const (
	// the texels a side of a tile of the sheets' files
	sheetTile = 512
	// how long a tile waits after the last photograph developed in it before it is written
	sheetSettled = 2 * time.Second
	// the least time between two tiles written
	sheetPace         = time.Second
	sheetManifestFile = "sheets.yaml"
)

// sheetStore is the sheets' files (SheetsCfg): their tiles read as the sheets are made and put in
// as a draw first reads them, and, kept, a tile a photograph was developed in written back once
// it has settled, a tile at a time on a goroutine of its own. Everything else is the frame's.
type sheetStore struct {
	cfg  SheetsCfg
	head sheetHead
	tile int
	now  func() time.Time
	// tiles across; by tile the sectors whose texels lie in it, by sector the tiles its do
	across int
	cover  [][]int
	spans  []image.Rectangle
	// by sector what its texels hold and what the files do
	texels, files []filed
	// by tile when a photograph was last developed in it, zero where the files hold what it does
	dirty []time.Time
	// when the last tile was read back
	last time.Time
	// the tiles read, to put into the sheets
	loads []tilePixels
	// what has the sheets developed as a draw reads them
	again func(*render.Image)
	// where a tile is read back to, its bytes, and the albedo's and the relief's
	scratch *render.Image
	pix     []byte
	out     [2][]byte
	enc     png.Encoder
	// whether a tile is being written, and whether a write has failed: none is tried again
	busy, failed atomic.Bool
	writing      sync.WaitGroup
}

// filed is a sector's photograph as the store knows it: what stood in it, whether anything did,
// and whether it is known at all
type filed struct {
	signed    signature
	inked, ok bool
}

// tilePixels is a tile's texels at L4, of the albedo and of the relief, the bytes the textures
// hold
type tilePixels struct {
	rect           image.Rectangle
	albedo, relief []byte
}

// sheetHead is what the manifest says of the sheets its tiles were cut from; tiles of any other
// are not read
type sheetHead struct {
	Version string     `yaml:"version"`
	PerUnit float32    `yaml:"per_unit"`
	Cols    int        `yaml:"cols"`
	Rows    int        `yaml:"rows"`
	Tile    int        `yaml:"tile"`
	Space   [2]float32 `yaml:"space,flow"`
	Grid    [2]int     `yaml:"grid,flow"`
	Sector  [2]float32 `yaml:"sector,flow"`
}

// sheetManifest is the sheets' manifest: their head and the sectors the tiles hold photographs of
type sheetManifest struct {
	sheetHead `yaml:",inline"`
	Sectors   []sheetSector `yaml:"sectors"`
}

// sheetSector is a sector the tiles hold a photograph of, what stood in it as signed and whether
// anything did
type sheetSector struct {
	I     int    `yaml:"i"`
	N     uint32 `yaml:"n"`
	Mix   uint64 `yaml:"mix"`
	Inked bool   `yaml:"inked"`
}

// sectorFields is a sheetSector without its MarshalYAML
type sectorFields sheetSector

// MarshalYAML writes the sector on a line of its own.
func (s sheetSector) MarshalYAML() (any, error) {
	var n yaml.Node
	if err := n.Encode(sectorFields(s)); err != nil {
		return nil, err
	}
	n.Style = yaml.FlowStyle
	return &n, nil
}

// openFiles reads the tiles in the sheets' folder, where they have one
func (s *sheets) openFiles() {
	if s.files.Dir == "" {
		return
	}
	s.store = newSheetStore(s, sheetTile)
	s.store.read(s)
}

// newSheetStore is the files of s, tiles tile texels a side
func newSheetStore(s *sheets, tile int) *sheetStore {
	n := s.grid.Len()
	across, down := (s.cols+tile-1)/tile, (s.rows+tile-1)/tile
	first, end := s.grid.Box(0), s.grid.Box(n-1).BottomRight
	st := &sheetStore{
		cfg:  s.files,
		tile: tile,
		now:  time.Now,
		head: sheetHead{
			Version: s.files.Version,
			PerUnit: s.perUnit,
			Cols:    s.cols,
			Rows:    s.rows,
			Tile:    tile,
			Space:   [2]float32{end.X, end.Y},
			Grid:    [2]int{s.grid.Cols(), n / max(s.grid.Cols(), 1)},
			Sector: [2]float32{first.BottomRight.X - first.TopLeft.X,
				first.BottomRight.Y - first.TopLeft.Y},
		},
		across: across,
		cover:  make([][]int, across*down),
		spans:  make([]image.Rectangle, n),
		texels: make([]filed, n),
		files:  make([]filed, n),
		dirty:  make([]time.Time, across*down),
	}
	st.enc.CompressionLevel = png.BestSpeed
	for i := range n {
		r := s.rect(i)
		if r.Empty() {
			continue
		}
		sp := image.Rect(r.Min.X/tile, r.Min.Y/tile, (r.Max.X-1)/tile+1, (r.Max.Y-1)/tile+1)
		st.spans[i] = sp
		for ty := sp.Min.Y; ty < sp.Max.Y; ty++ {
			for tx := sp.Min.X; tx < sp.Max.X; tx++ {
				st.cover[ty*across+tx] = append(st.cover[ty*across+tx], i)
			}
		}
	}
	return st
}

// rect is tile t's texels at L4
func (st *sheetStore) rect(t int, s *sheets) image.Rectangle {
	x, y := t%st.across*st.tile, t/st.across*st.tile
	return image.Rect(x, y, min(x+st.tile, s.cols), min(y+st.tile, s.rows))
}

// path is the file name in the store's folder
func (st *sheetStore) path(name string) string { return filepath.Join(st.cfg.Dir, name) }

// tileName is the file of tile t's albedo or relief
func (st *sheetStore) tileName(of string, t int) string {
	return fmt.Sprintf("%s-%d-%d.png", of, t%st.across, t/st.across)
}

// read has s hold the photographs the folder's tiles keep, if its manifest is of these sheets,
// the tiles put in as a draw first reads s; a tile unread leaves its sectors to be photographed
func (st *sheetStore) read(s *sheets) {
	m, err := st.manifest()
	switch {
	case errors.Is(err, fs.ErrNotExist):
		log.Printf("[world] sheets: none kept in %s yet", st.cfg.Dir)
		return
	case err != nil:
		log.Printf("[world] sheets: %v", err)
		return
	case m.sheetHead != st.head:
		log.Printf("[world] sheets: %s keeps another world's or version's, photographed anew",
			st.cfg.Dir)
		return
	}
	n := len(st.spans)
	tiles := make([]*tilePixels, len(st.cover))
	unread := make([]bool, len(st.cover))
	for _, e := range m.Sectors {
		if e.I < 0 || e.I >= n {
			continue
		}
		st.eachTile(e.I, func(t int) {
			if tiles[t] != nil || unread[t] {
				return
			}
			p, err := st.readTile(t, s)
			if err != nil {
				log.Printf("[world] sheets: %v", err)
				unread[t] = true
				return
			}
			tiles[t] = &p
		})
	}
	loaded := make([]bool, n)
	for _, e := range m.Sectors {
		if e.I < 0 || e.I >= n {
			continue
		}
		whole := true
		st.eachTile(e.I, func(t int) { whole = whole && tiles[t] != nil })
		if !whole {
			continue
		}
		loaded[e.I] = true
		f := filed{signed: signature{n: e.N, mix: e.Mix}, inked: e.Inked, ok: true}
		st.texels[e.I], st.files[e.I] = f, f
		s.loaded(e.I, f.signed, f.inked)
	}
	for t, p := range tiles {
		if p == nil {
			continue
		}
		kept := false
		// what the files hold of a sector not read is not the sheets'
		for _, i := range st.cover[t] {
			if loaded[i] {
				kept = true
			} else {
				p.clear(s.rect(i))
			}
		}
		if kept {
			st.loads = append(st.loads, *p)
		}
	}
	if len(st.loads) > 0 {
		upload := func(*render.Image) { s.upload() }
		for _, img := range [4]*render.Image{s.albedo, s.relief, s.albedoL5, s.reliefL5} {
			img.Defer(upload)
		}
	}
}

// eachTile calls each with every tile sector i's texels lie in
func (st *sheetStore) eachTile(i int, each func(t int)) {
	sp := st.spans[i]
	for ty := sp.Min.Y; ty < sp.Max.Y; ty++ {
		for tx := sp.Min.X; tx < sp.Max.X; tx++ {
			each(ty*st.across + tx)
		}
	}
}

// manifest is the folder's manifest
func (st *sheetStore) manifest() (sheetManifest, error) {
	var m sheetManifest
	b, err := os.ReadFile(st.path(sheetManifestFile))
	if err != nil {
		return m, err
	}
	if err := yaml.Unmarshal(b, &m); err != nil {
		return m, fmt.Errorf("%s: %w", st.path(sheetManifestFile), err)
	}
	return m, nil
}

// readTile is tile t's texels as its files keep them
func (st *sheetStore) readTile(t int, s *sheets) (tilePixels, error) {
	p := tilePixels{rect: st.rect(t, s)}
	var err error
	w, h := p.rect.Dx(), p.rect.Dy()
	if p.albedo, err = readPNG(st.path(st.tileName("albedo", t)), w, h); err != nil {
		return p, err
	}
	p.relief, err = readPNG(st.path(st.tileName("relief", t)), w, h)
	return p, err
}

// clear zeroes the texels r of the tile, those of a sheet never photographed
func (p *tilePixels) clear(r image.Rectangle) {
	r = r.Intersect(p.rect)
	w := p.rect.Dx()
	for y := r.Min.Y; y < r.Max.Y; y++ {
		from := 4 * ((y-p.rect.Min.Y)*w + r.Min.X - p.rect.Min.X)
		clear(p.albedo[from : from+4*r.Dx()])
		clear(p.relief[from : from+4*r.Dx()])
	}
}

// readPNG is the bytes of the PNG at path, w by h, as writePNG wrote them
func readPNG(path string, w, h int) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	img, err := png.Decode(bufio.NewReader(f))
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	if b := img.Bounds(); b != image.Rect(0, 0, w, h) {
		return nil, fmt.Errorf("%s: %v, want %dx%d", path, b, w, h)
	}
	var pix []byte
	var stride int
	switch m := img.(type) {
	case *image.NRGBA:
		pix, stride = m.Pix, m.Stride
	// one written opaque comes back as RGBA, its bytes the same
	case *image.RGBA:
		pix, stride = m.Pix, m.Stride
	default:
		return nil, fmt.Errorf("%s: %T, want RGBA", path, img)
	}
	if stride == 4*w {
		return pix[:4*w*h], nil
	}
	out := make([]byte, 4*w*h)
	for y := range h {
		copy(out[4*w*y:4*w*(y+1)], pix[y*stride:])
	}
	return out, nil
}

// writePNG writes the bytes pix, w by h, to path as they are: an image.RGBA would be taken as
// premultiplied and lose the relief's normal in its alpha
func writePNG(path string, pix []byte, w, h int, enc *png.Encoder) error {
	img := &image.NRGBA{Pix: pix, Stride: 4 * w, Rect: image.Rect(0, 0, w, h)}
	return writeAtomic(path, func(out io.Writer) error { return enc.Encode(out, img) })
}

// writeAtomic has write make the file path through a file beside it, renamed once whole
func writeAtomic(path string, write func(io.Writer) error) error {
	tmp := path + ".tmp"
	f, err := os.Create(tmp)
	if err != nil {
		return err
	}
	b := bufio.NewWriter(f)
	err = write(b)
	if err == nil {
		err = b.Flush()
	}
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	if err == nil {
		err = os.Rename(tmp, path)
	}
	if err != nil {
		os.Remove(tmp)
	}
	return err
}

// took notes the photographs developed: what the texels of their sectors hold now, and every
// tile they lie in to be written
func (st *sheetStore) took(photos []photo) {
	if !st.cfg.Keep {
		return
	}
	now := st.now()
	for _, p := range photos {
		st.texels[p.sector] = filed{signed: p.signed, inked: p.full, ok: true}
		st.eachTile(p.sector, func(t int) { st.dirty[t] = now })
	}
}

// keep writes the tile a photograph was developed in longest ago, if sheetSettled has passed
// since and sheetPace since the last, and has s developed again as a draw next reads it while a
// tile waits
func (st *sheetStore) keep(s *sheets) {
	if !st.cfg.Keep || st.failed.Load() {
		return
	}
	oldest := -1
	for t, at := range st.dirty {
		if !at.IsZero() && (oldest < 0 || at.Before(st.dirty[oldest])) {
			oldest = t
		}
	}
	if oldest < 0 {
		return
	}
	now := st.now()
	if !st.busy.Load() && now.Sub(st.last) >= sheetPace &&
		now.Sub(st.dirty[oldest]) >= sheetSettled {
		st.last = now
		st.write(s, oldest)
	}
	for _, at := range st.dirty {
		if !at.IsZero() && st.again != nil {
			s.schedule(st.again)
			return
		}
	}
}

// write reads tile t back and has it written with the manifest; a sector of it the manifest
// listed is left out of it while the tile is written, and listed again once every tile of it is
func (st *sheetStore) write(s *sheets, t int) {
	r := st.rect(t, s)
	albedo, relief := st.readBack(s, r)
	st.dirty[t] = time.Time{}
	listed := false
	for _, i := range st.cover[t] {
		listed = listed || st.files[i].ok
		whole := true
		st.eachTile(i, func(u int) { whole = whole && st.dirty[u].IsZero() })
		st.files[i] = filed{}
		if whole {
			st.files[i] = st.texels[i]
		}
	}
	var before []sheetSector
	if listed {
		before = st.listed(t)
	}
	after := st.listed(-1)
	st.busy.Store(true)
	st.writing.Add(1)
	go st.save(t, r, albedo, relief, listed, before, after)
}

// listed is the sectors the files hold, but those lying in tile skip
func (st *sheetStore) listed(skip int) []sheetSector {
	out := []sheetSector{}
	for i, f := range st.files {
		if !f.ok || skip >= 0 && image.Pt(skip%st.across, skip/st.across).In(st.spans[i]) {
			continue
		}
		out = append(out, sheetSector{I: i, N: f.signed.n, Mix: f.signed.mix, Inked: f.inked})
	}
	return out
}

// readBack is the texels r of the sheets' albedo and relief, in buffers the store keeps
func (st *sheetStore) readBack(s *sheets, r image.Rectangle) (albedo, relief []byte) {
	tile := st.tile
	if st.scratch == nil {
		st.scratch = render.NewImage(2*tile, tile)
		st.pix = make([]byte, 8*tile*tile)
		st.out = [2][]byte{make([]byte, 4*tile*tile), make([]byte, 4*tile*tile)}
	}
	w, h := r.Dx(), r.Dy()
	st.scratch.SubImage(image.Rect(0, 0, w, h)).CopyFrom(s.albedo.SubImage(r))
	st.scratch.SubImage(image.Rect(tile, 0, tile+w, h)).CopyFrom(s.relief.SubImage(r))
	st.scratch.ReadPixels(st.pix)
	albedo, relief = st.out[0][:4*w*h], st.out[1][:4*w*h]
	for y := range h {
		row := st.pix[8*tile*y:]
		copy(albedo[4*w*y:4*w*(y+1)], row)
		copy(relief[4*w*y:4*w*(y+1)], row[4*tile:])
	}
	return albedo, relief
}

// save writes tile t's texels r and the manifest after, the manifest before first where the tile
// held a sector it listed; the first error is logged and stops the writing
func (st *sheetStore) save(t int, r image.Rectangle, albedo, relief []byte, listed bool,
	before, after []sheetSector) {
	defer st.writing.Done()
	defer st.busy.Store(false)
	err := os.MkdirAll(st.cfg.Dir, 0o755)
	if err == nil && listed {
		err = st.writeManifest(before)
	}
	if err == nil {
		err = writePNG(st.path(st.tileName("albedo", t)), albedo, r.Dx(), r.Dy(), &st.enc)
	}
	if err == nil {
		err = writePNG(st.path(st.tileName("relief", t)), relief, r.Dx(), r.Dy(), &st.enc)
	}
	if err == nil {
		err = st.writeManifest(after)
	}
	if err != nil && st.failed.CompareAndSwap(false, true) {
		log.Printf("[world] sheets: %v; none written more", err)
	}
}

// writeManifest writes the manifest listing sectors
func (st *sheetStore) writeManifest(sectors []sheetSector) error {
	b, err := yaml.Marshal(sheetManifest{sheetHead: st.head, Sectors: sectors})
	if err != nil {
		return err
	}
	return writeAtomic(st.path(sheetManifestFile), func(out io.Writer) error {
		_, err := out.Write(b)
		return err
	})
}
