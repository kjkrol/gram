package spacing

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"hash/fnv"
	"io"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"

	"github.com/kjkrol/gram/plugins/board/cell"
	"github.com/kjkrol/gram/plugins/board/grid"
	"github.com/kjkrol/gram/plugins/navigation/internal/bodies"
	"github.com/kjkrol/gram/plugins/navigation/internal/pathfind"
)

// Cache is where the hierarchies are kept between runs, a file a domain in Dir, read as a
// domain's hierarchy is first asked for and, where Keep, written once built and appended to
// as its clusters are built again; Version names how the game prices its steps (navigation.Cache).
type Cache struct {
	Dir     string
	Keep    bool
	Version string
}

// how long after a save or a dump the clusters built since are dumped, unless DumpEvery says
const dumpEvery = 5 * time.Second

// a file's head: the magic, the Cache's Version, the classes' fingerprint, the side and the
// count of the clusters of level 1, then by cluster the fingerprint of its cells as the file's
// hierarchy was built over them, then hpastar's stream; the magic's last letter moves with the
// nodes' keys, cell.Levels a cell since 2
var cacheMagic = [4]byte{'G', 'R', 'M', '2'}

// the work an Update spends building a hierarchy, in hpastar's steps: a step of a cluster of level
// 1 costs about 33 ns, its board's steps read over relief, and about 11 ns above
// (PathFinder_Hierarchy's board), so 2.5 ms / 33 ns ≈ 75,000 keeps a tick building within 2-3 ms
const buildBudget = 75_000

// a board counting the changes to what a step over its cells reads, all of them and a cell's
type passages interface {
	Passage() uint64
	CellPassage(c cell.ID) uint64
}

// a board telling what a step reads of a cell apart from any other cell's: without one, no
// hierarchy is kept in a file
type fingerprints interface {
	CellFingerprint(c cell.ID) uint64
}

type hierarchyEntry struct {
	key   pathfind.Key
	h     *pathfind.Hierarchy
	since uint64 // the board's passage its clusters were last marked stale against
	used  uint64 // the Update it was last asked for
	ready bool   // none of its clusters stale: routes are climbed through it
	// its file: the path, whether the file holds it whole, how long the whole is and how much
	// the dumps appended, and when it was last written; by cluster of level 1 the fingerprint of
	// its cells as the file has it, where they begin in the file, which are to be written anew
	// and the board's passage they were last found against
	path     string
	saved    bool
	base     int64
	appended int64
	written  time.Time
	prints   []uint64
	printsAt int64
	dirty    []bool
	printed  uint64
}

// Hierarchies are the hierarchies whose far routes are climbed, one a domain serving the classes
// its units are planned as, made as the domain's first far route is asked for. An Update has them
// asked for, staled and built on its own goroutine before any ask is answered; routers answering
// on several goroutines read them as they stand.
type Hierarchies struct {
	finder  *Finder      // the one they are built through; nil: none kept
	board   passages     // the passage of the finder's board
	printer fingerprints // the finder's board's cells told apart; nil: no files
	side    int
	from    float64
	budget  int // the work a Sweep builds, buildBudget
	update  uint64
	entries []hierarchyEntry // a domain each
	// the files kept between runs: where, how many hierarchies were read from them, the writes
	// waiting their turn on the writer's goroutine, and whether a write failed, which ends them
	cache  Cache
	every  time.Duration
	loaded uint64
	writes chan func() error
	done   sync.WaitGroup
	failed atomic.Bool
}

// Bind has hs keep hierarchies of clusters side cells a side, built through f, for routes whose
// ends lie at least from cells apart: f blind to the others and planning by levels over a grid of
// grid.DefaultGrids and a board counting the passage of its cells. Over any other, or a side of
// 0, none.
func (hs *Hierarchies) Bind(f *Finder, side int, from float64) {
	hs.Flush()
	*hs = Hierarchies{side: side, from: from, budget: buildBudget, update: 1, cache: hs.cache,
		every: dumpEvery}
	if _, blind := f.Occupancy.(OpenOccupancy); !blind || side <= 0 || !f.leveled ||
		!laidOut(f.grid) {
		return
	}
	if b, ok := f.Terrain.(passages); ok {
		hs.finder, hs.board = f, b
		hs.printer, _ = f.Terrain.(fingerprints)
	}
}

// whether g is a grid of grid.DefaultGrids, which a hierarchy is laid over; a board's read says
// it of the grid it embeds
func laidOut(g grid.Grid) bool {
	if s, ok := g.(interface{ Shape() (grid.Shape, bool) }); ok {
		_, ok = s.Shape()
		return ok
	}
	_, ok := grid.ShapeOf(g)
	return ok
}

// On reports whether hs keeps hierarchies.
func (hs *Hierarchies) On() bool { return hs.finder != nil }

// Far reports whether the route from from to to is climbed, its ends lying at least as far apart
// as hs was bound with: the question alone says it, so any router answers it alike.
func (hs *Hierarchies) Far(from, to cell.ID) bool {
	return hs.finder != nil && hs.finder.grid.Distance(from, to) >= hs.from
}

// Key is what m's far routes are climbed through: the hierarchy of its domain, which serves the
// class of its body; false where hs keeps none or no class holds its body.
func (hs *Hierarchies) Key(m bodies.Member) (pathfind.Key, bool) {
	if hs.finder == nil || hs.finder.classes == nil {
		return pathfind.Key{}, false
	}
	if _, _, ok := hs.finder.classes.Class(m.Domain, m.Body()); !ok {
		return pathfind.Key{}, false
	}
	return pathfind.Key{Domain: m.Domain}, true
}

// Expect has key's hierarchy wanted this Update, made the first time it is, of the classes of its
// domain.
func (hs *Hierarchies) Expect(key pathfind.Key) {
	if hs.finder == nil || hs.finder.classes == nil {
		return
	}
	for i := range hs.entries {
		if e := &hs.entries[i]; e.key == key {
			e.used = hs.update
			return
		}
	}
	classes := hs.finder.classes.Of(key.Domain)
	if len(classes) == 0 {
		return
	}
	hs.entries = append(hs.entries, hierarchyEntry{key: key,
		h: hs.finder.NewHierarchy(key, hs.side, classes), since: hs.board.Passage(),
		used: hs.update})
	hs.load(&hs.entries[len(hs.entries)-1])
}

// Keep has hs keep its hierarchies in c's files from now on.
func (hs *Hierarchies) Keep(c Cache) { hs.cache = c }

// DumpEvery sets how long after a write the clusters built since are dumped to the file, five
// seconds unless set.
func (hs *Hierarchies) DumpEvery(d time.Duration) { hs.every = d }

// Loaded counts the hierarchies read from the Cache's files, none of their clusters built.
func (hs *Hierarchies) Loaded() uint64 { return hs.loaded }

// Flush waits for the files being written.
func (hs *Hierarchies) Flush() { hs.done.Wait() }

// file is e's path in the cache, "" without one
func (hs *Hierarchies) file(e *hierarchyEntry) string {
	if hs.cache.Dir == "" || hs.printer == nil {
		return ""
	}
	return filepath.Join(hs.cache.Dir, fmt.Sprintf("hierarchy-%d.bin", e.key.Domain))
}

// head is what a file of e's hierarchy opens with, before the clusters' fingerprints: the
// Cache's Version, the classes it serves, the side of its clusters and how many there are
func (hs *Hierarchies) head(e *hierarchyEntry) []byte {
	var b bytes.Buffer
	b.Write(cacheMagic[:])
	binary.Write(&b, binary.LittleEndian, uint32(len(hs.cache.Version)))
	b.WriteString(hs.cache.Version)
	classes := fnv.New64a()
	fmt.Fprintf(classes, "%+v", e.h.Classes())
	binary.Write(&b, binary.LittleEndian, [3]uint64{classes.Sum64(), uint64(e.h.Side()),
		uint64(e.h.Clusters())})
	return b.Bytes()
}

// print is the fingerprint of the cells of cluster k of e's hierarchy as they stand
func (hs *Hierarchies) print(e *hierarchyEntry, k int) uint64 {
	h := fnv.New64a()
	var buf [8]byte
	e.h.EachCell(k, func(c cell.ID) {
		binary.LittleEndian.PutUint64(buf[:], hs.printer.CellFingerprint(c))
		h.Write(buf[:])
	})
	return h.Sum64()
}

// prints has e's clusters' fingerprints as their cells stand now, all of them where all, else
// those whose cells' passage moved since they were last found, each marked dirty
func (hs *Hierarchies) prints(e *hierarchyEntry, all bool) {
	n := e.h.Clusters()
	if len(e.prints) != n {
		e.prints, e.dirty, all = make([]uint64, n), make([]bool, n), true
	}
	if all {
		for k := range n {
			e.prints[k], e.dirty[k] = hs.print(e, k), true
		}
	} else if now := hs.board.Passage(); now != e.printed {
		for k := range n {
			e.dirty[k] = false
		}
		for k := range n {
			e.h.EachCell(k, func(c cell.ID) {
				if !e.dirty[k] && hs.board.CellPassage(c) > e.printed {
					e.dirty[k] = true
				}
			})
			if e.dirty[k] {
				e.prints[k] = hs.print(e, k)
			}
		}
	}
	e.printed = hs.board.Passage()
}

// printsOf is the clusters' fingerprints as bytes, for the file
func printsOf(prints []uint64) []byte {
	b := make([]byte, 8*len(prints))
	for k, v := range prints {
		binary.LittleEndian.PutUint64(b[8*k:], v)
	}
	return b
}

// load reads e's hierarchy from its file where there is one of the same head, e then ready
func (hs *Hierarchies) load(e *hierarchyEntry) {
	e.path = hs.file(e)
	if e.path == "" {
		return
	}
	data, err := os.ReadFile(e.path)
	if errors.Is(err, fs.ErrNotExist) {
		return
	}
	if err != nil {
		log.Printf("navigation: the hierarchy of domain %d kept in %s: %v; built anew",
			e.key.Domain, e.path, err)
		return
	}
	head, n := hs.head(e), e.h.Clusters()
	from := len(head) + 8*n
	if len(data) < from || !bytes.Equal(data[:len(head)], head) {
		log.Printf("navigation: the hierarchy of domain %d kept in %s is of another version or "+
			"shape; built anew", e.key.Domain, e.path)
		return
	}
	began := time.Now()
	if err := hs.finder.Load(e.h, bytes.NewReader(data[from:])); err != nil {
		log.Printf("navigation: the hierarchy of domain %d kept in %s: %v; built anew",
			e.key.Domain, e.path, err)
		return
	}
	// the clusters whose cells stand otherwise than the file's are built again, and the file
	// told of them once they are
	hs.prints(e, true)
	stale := 0
	for k := range n {
		if was := binary.LittleEndian.Uint64(data[len(head)+8*k:]); was == e.prints[k] {
			e.dirty[k] = false
		} else {
			hs.finder.StaleCluster(e.h, k)
			stale++
		}
	}
	e.ready, e.saved, e.base, e.appended = e.h.Ready(), true, int64(len(data)), 0
	e.printsAt, e.written = int64(len(head)), time.Now()
	e.since = hs.board.Passage()
	hs.loaded++
	log.Printf("navigation: the hierarchy of domain %d loaded from %s (%d bytes, %d of %d "+
		"clusters over changed cells) in %v", e.key.Domain, e.path, len(data), stale, n,
		time.Since(began).Round(time.Millisecond))
}

// keep has e's hierarchy written to its file as the Cache asks: whole once it is ready and not
// saved, else what was built since the last write once dumpEvery has passed, whole again where
// the dumps appended would outgrow the base
func (hs *Hierarchies) keep(e *hierarchyEntry) {
	if !hs.cache.Keep || e.path == "" || hs.failed.Load() {
		return
	}
	switch {
	case e.ready && !e.saved:
		hs.save(e)
	case e.saved && time.Since(e.written) >= hs.every:
		hs.dump(e)
	}
}

func (hs *Hierarchies) save(e *hierarchyEntry) {
	var b bytes.Buffer
	head := hs.head(e)
	b.Write(head)
	hs.prints(e, true)
	b.Write(printsOf(e.prints))
	if err := hs.finder.Save(e.h, &b); err != nil {
		hs.fail(e, err)
		return
	}
	clear(e.dirty)
	e.saved, e.base, e.appended, e.written = true, int64(b.Len()), 0, time.Now()
	e.printsAt = int64(len(head))
	path, data := e.path, b.Bytes()
	hs.write(func() error { return writeWhole(path, data) })
}

// dump appends to e's file what was built since it was last written, and writes anew the
// fingerprints of the clusters whose cells changed meanwhile, in their places
func (hs *Hierarchies) dump(e *hierarchyEntry) {
	var b bytes.Buffer
	if _, err := hs.finder.Dump(e.h, &b); err != nil {
		hs.fail(e, err)
		return
	}
	e.written = time.Now()
	hs.prints(e, false)
	// a cluster's fingerprint goes in once every cluster is built, its own among them dumped
	var prints []printed
	if e.h.Ready() {
		for k, dirty := range e.dirty {
			if dirty {
				prints = append(prints, printed{at: e.printsAt + 8*int64(k), v: e.prints[k]})
				e.dirty[k] = false
			}
		}
	}
	if b.Len() == 0 && len(prints) == 0 {
		return
	}
	if e.appended += int64(b.Len()); e.appended > e.base {
		e.saved = false
		hs.save(e)
		return
	}
	path, data := e.path, b.Bytes()
	hs.write(func() error {
		f, err := os.OpenFile(path, os.O_WRONLY, 0o644)
		if err != nil {
			return err
		}
		if _, err = f.Seek(0, io.SeekEnd); err == nil {
			_, err = f.Write(data)
		}
		var buf [8]byte
		for _, p := range prints {
			if err != nil {
				break
			}
			binary.LittleEndian.PutUint64(buf[:], p.v)
			_, err = f.WriteAt(buf[:], p.at)
		}
		if cerr := f.Close(); err == nil {
			err = cerr
		}
		return err
	})
}

// a cluster's fingerprint to write at a place in the file
type printed struct {
	at int64
	v  uint64
}

// fail ends the writing: the first error is told, the files left as they are
func (hs *Hierarchies) fail(e *hierarchyEntry, err error) {
	if !hs.failed.Swap(true) {
		log.Printf("navigation: the hierarchy of domain %d is not kept in %s: %v", e.key.Domain,
			e.path, err)
	}
}

// write has job run on the writer's goroutine, after the writes before it
func (hs *Hierarchies) write(job func() error) {
	if hs.writes == nil {
		hs.writes = make(chan func() error, 16)
		go func() {
			for job := range hs.writes {
				if err := job(); err != nil && !hs.failed.Swap(true) {
					log.Printf("navigation: a hierarchy is not kept: %v", err)
				}
				hs.done.Done()
			}
		}()
	}
	hs.done.Add(1)
	hs.writes <- job
}

// writeWhole makes the file path of data through a file beside it, renamed once whole
func writeWhole(path string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	tmp := path + ".tmp"
	f, err := os.Create(tmp)
	if err != nil {
		return err
	}
	w := bufio.NewWriter(f)
	_, err = w.Write(data)
	if err == nil {
		err = w.Flush()
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

// Due reports whether a Sweep has clusters to mark stale or to build.
func (hs *Hierarchies) Due() bool {
	if hs.finder == nil {
		return false
	}
	v := hs.board.Passage()
	for i := range hs.entries {
		if e := &hs.entries[i]; !e.ready || e.since != v {
			return true
		}
	}
	return false
}

// Sweep readies the hierarchies for an Update's answers: each is marked stale where the board's
// passage moved, as a climb asks, and the one not ready asked for most lately is built
// buildBudget steps of hpastar's work further, the same work on any machine.
func (hs *Hierarchies) Sweep() {
	if hs.finder == nil {
		return
	}
	v := hs.board.Passage()
	for i := range hs.entries {
		if e := &hs.entries[i]; e.since != v {
			hs.finder.Stale(e.h, e.since)
			e.since, e.ready = v, e.h.Ready()
		}
	}
	if k := hs.unbuilt(); k >= 0 {
		e := &hs.entries[k]
		hs.finder.Build(e.h, hs.budget)
		e.ready = e.h.Ready()
	}
	hs.update++
}

// Store has the hierarchies kept in the Cache's files as it asks: each written whole once it is
// ready, then what was built of it since every so often. Call it every Update, Sweep or none.
func (hs *Hierarchies) Store() {
	if hs.finder == nil || !hs.cache.Keep {
		return
	}
	for i := range hs.entries {
		hs.keep(&hs.entries[i])
	}
}

// unbuilt is the entry of the hierarchy not ready asked for most lately, the first of those
// asked for alike; -1 for none
func (hs *Hierarchies) unbuilt() int {
	k := -1
	for i := range hs.entries {
		e := &hs.entries[i]
		if !e.ready && (k < 0 || e.used > hs.entries[k].used) {
			k = i
		}
	}
	return k
}

// Take is key's hierarchy where it is built, for an ask's route to be climbed through it; nil
// while it is not, the route searched. Routers on several goroutines take it at once.
func (hs *Hierarchies) Take(key pathfind.Key) *pathfind.Hierarchy {
	for i := range hs.entries {
		if e := &hs.entries[i]; e.ready && e.key == key {
			return e.h
		}
	}
	return nil
}
