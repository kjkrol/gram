package gpu

import (
	"fmt"

	"github.com/gogpu/wgpu"
)

// gogpu's Queue.WriteBuffer waits for the GPU to finish all it was given: the gathered bytes go
// through a mapped staging buffer instead
type staging struct {
	buf     *wgpu.Buffer
	size    uint64
	pending *wgpu.MapPending // nil once mapped
}

// staging buffers in flight before a submission waits for one
const maxStaging = 4

func (s *staging) ready() bool {
	if s.pending == nil {
		return true
	}
	done, err := s.pending.Status()
	if !done {
		return false
	}
	s.pending.Release()
	s.pending = nil
	return err == nil
}

func (d *device) stage(size uint64) (*staging, error) {
	for {
		for i, s := range d.stages {
			if !s.ready() {
				continue
			}
			if s.size >= size {
				return s, nil
			}
			s.buf.Release()
			d.stages = append(d.stages[:i], d.stages[i+1:]...)
			break
		}
		if len(d.stages) < maxStaging {
			n := uint64(1 << 16)
			for n < size {
				n *= 2
			}
			buf, err := d.dev.CreateBuffer(&wgpu.BufferDescriptor{Label: "staging", Size: n, Usage: wgpu.BufferUsageMapWrite | wgpu.BufferUsageCopySrc, MappedAtCreation: true})
			if err != nil {
				return nil, fmt.Errorf("gpu: a staging buffer of %d bytes: %w", n, err)
			}
			s := &staging{buf: buf, size: n}
			d.stages = append(d.stages, s)
			return s, nil
		}
		d.dev.Poll(wgpu.PollWait)
	}
}

func (d *device) upload() (*wgpu.CommandBuffer, *staging, error) {
	parts := []struct {
		dst  *wgpu.Buffer
		data []byte
	}{{d.vbuf, d.verts}, {d.ibuf, d.indices}, {d.dbuf, d.draws}, {d.ubuf, d.uniforms}}
	var offs [4]uint64
	total := uint64(0)
	for k, p := range parts {
		offs[k] = total
		total += alignUp(uint64(len(p.data)), 256)
	}
	if total == 0 {
		return nil, nil, nil
	}
	s, err := d.stage(total)
	if err != nil {
		return nil, nil, err
	}
	rng, err := s.buf.MappedRange(0, total)
	if err != nil {
		return nil, nil, fmt.Errorf("gpu: the staging buffer's range: %w", err)
	}
	mem := rng.Bytes()
	for k, p := range parts {
		copy(mem[offs[k]:], p.data)
	}
	if err := s.buf.Unmap(); err != nil {
		return nil, nil, fmt.Errorf("gpu: unmapping the staging buffer: %w", err)
	}
	enc, err := d.dev.CreateCommandEncoder(nil)
	if err != nil {
		return nil, nil, fmt.Errorf("gpu: an encoder for the upload: %w", err)
	}
	for k, p := range parts {
		if n := alignUp(uint64(len(p.data)), 4); n > 0 {
			enc.CopyBufferToBuffer(s.buf, offs[k], p.dst, 0, n)
		}
	}
	cmds, err := enc.Finish()
	if err != nil {
		return nil, nil, fmt.Errorf("gpu: finishing the upload: %w", err)
	}
	return cmds, s, nil
}

func (d *device) writeGathered() error {
	for _, w := range []struct {
		buf  *wgpu.Buffer
		data []byte
	}{{d.vbuf, d.verts}, {d.ibuf, d.indices}, {d.dbuf, d.draws}, {d.ubuf, d.uniforms}} {
		// a copy: gogpu may read it after the submission, the gathered slices are reused
		if len(w.data) > 0 {
			if err := d.queue.WriteBuffer(w.buf, 0, append([]byte(nil), w.data...)); err != nil {
				return fmt.Errorf("gpu: writing a buffer: %w", err)
			}
		}
	}
	return nil
}

func (s *staging) remap() error {
	p, err := s.buf.MapAsync(wgpu.MapModeWrite, 0, s.size)
	if err != nil {
		return fmt.Errorf("gpu: mapping the staging buffer again: %w", err)
	}
	s.pending = p
	return nil
}
