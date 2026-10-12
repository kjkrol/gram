package engine

import (
	"bytes"
	"encoding/binary"
	"encoding/gob"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/kjkrol/goke/v3"
)

// listSaves returns every save found for basePath, "" (quicksave) first.
func listSaves(basePath string) ([]string, error) {
	var labels []string
	if _, err := os.Stat(filePath(basePath, "")); err == nil {
		labels = append(labels, "")
	}

	matches, err := filepath.Glob(basePath + ".game.*.save")
	if err != nil {
		return nil, err
	}
	prefix, suffix := basePath+".game.", ".save"
	var named []string
	for _, m := range matches {
		named = append(named, strings.TrimSuffix(strings.TrimPrefix(m, prefix), suffix))
	}
	sort.Strings(named)

	return append(labels, named...), nil
}

// save pauses ecs and writes groups followed by the ECS snapshot to filePath(basePath, label).
func save(ecs *goke.ECS, basePath, label string, groups map[string][]any) error {
	ecs.Pause()
	defer ecs.Resume()

	tmp, err := os.CreateTemp("", "gram-ecs-*.tmp")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	tmp.Close()
	defer os.Remove(tmpPath)

	if err := ecs.Save(tmpPath); err != nil {
		return err
	}

	out, err := os.Create(filePath(basePath, label))
	if err != nil {
		return err
	}
	defer out.Close()

	if err := saveResources(out, groups); err != nil {
		return err
	}

	ecsData, err := os.Open(tmpPath)
	if err != nil {
		return err
	}
	defer ecsData.Close()
	_, err = io.Copy(out, ecsData)
	return err
}

// load restores a snapshot written by save into groups and a freshly constructed ecs, and hands
// back every group the save holds, encoded, for values tracked after it.
func load(ecs *goke.ECS, basePath, label string, comps []goke.CompToken, groups map[string][]any) (map[string][]byte, error) {
	in, err := os.Open(filePath(basePath, label))
	if err != nil {
		return nil, err
	}
	defer in.Close()

	encoded, err := loadResources(in, groups)
	if err != nil {
		return nil, err
	}

	tmp, err := os.CreateTemp("", "gram-ecs-*.tmp")
	if err != nil {
		return nil, err
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)
	if _, err := io.Copy(tmp, in); err != nil {
		tmp.Close()
		return nil, err
	}
	if err := tmp.Close(); err != nil {
		return nil, err
	}

	return encoded, ecs.Load(tmpPath, comps...)
}

// filePath is the quicksave path when label is empty, else a named save path.
func filePath(basePath, label string) string {
	if label == "" {
		return basePath + ".game.save"
	}
	return basePath + ".game." + label + ".save"
}

// saveResources gob-encodes groups into one frame, each under its own name.
func saveResources(w io.Writer, groups map[string][]any) error {
	encoded := make(map[string][]byte, len(groups))
	for name, targets := range groups {
		var buf bytes.Buffer
		enc := gob.NewEncoder(&buf)
		for _, t := range targets {
			if err := enc.Encode(t); err != nil {
				return fmt.Errorf("gram: encode resource %q: %w", name, err)
			}
		}
		encoded[name] = buf.Bytes()
	}

	var outer bytes.Buffer
	if err := gob.NewEncoder(&outer).Encode(encoded); err != nil {
		return err
	}
	if err := binary.Write(w, binary.BigEndian, uint32(outer.Len())); err != nil {
		return err
	}
	_, err := w.Write(outer.Bytes())
	return err
}

// loadResources restores groups from r by name, leaving untouched any the save does not hold, and
// hands back every group the save holds, encoded.
func loadResources(r io.Reader, groups map[string][]any) (map[string][]byte, error) {
	var n uint32
	if err := binary.Read(r, binary.BigEndian, &n); err != nil {
		return nil, err
	}
	data := make([]byte, n)
	if _, err := io.ReadFull(r, data); err != nil {
		return nil, err
	}

	var encoded map[string][]byte
	if err := gob.NewDecoder(bytes.NewReader(data)).Decode(&encoded); err != nil {
		return nil, err
	}

	for name, targets := range groups {
		if err := decodeGroup(encoded, name, targets); err != nil {
			return nil, err
		}
	}
	return encoded, nil
}

// decodeGroup writes the group called name, when encoded holds it, into targets.
func decodeGroup(encoded map[string][]byte, name string, targets []any) error {
	blob, ok := encoded[name]
	if !ok {
		return nil
	}
	dec := gob.NewDecoder(bytes.NewReader(blob))
	for _, t := range targets {
		if err := dec.Decode(t); err != nil {
			return fmt.Errorf("gram: decode resource %q: %w", name, err)
		}
	}
	return nil
}
