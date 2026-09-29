package gpu

import (
	"fmt"

	"github.com/gogpu/wgpu"
)

// staging is a buffer the CPU writes a submission's gathered bytes into while it is mapped, for the
// GPU to copy where they are read. gogpu's Queue.WriteBuffer waits for the GPU to finish all it was
// given, so the bytes go this way instead: mapped anew once the GPU is done with them, in the
// background, a buffer is reused only when it is ready.
type staging struct {
	buf     *wgpu.Buffer
	size    uint64
	pending *wgpu.MapPending // being mapped; nil once mapped
}

// maxStaging is how many staging buffers may be in flight before a submission waits for one.
const maxStaging = 4

// ready reports whether the buffer is mapped, taking a mapping that has come through.
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

// stage is a mapped staging buffer of size bytes at least: a ready one, a new one while fewer than
// maxStaging are in flight, else the first to come through.
func (d *device) stage(size uint64) (*staging, error) {
	for {
		for i, s := range d.stages {
			if !s.ready() {
				continue
			}
			if s.size >= size {
				return s, nil
			}
			s.buf.Release() // too small: made anew below in its place
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

// upload copies the gathered bytes to the buffers the draws read, on the GPU ahead of the draws:
// the commands to submit before them; nil with nothing gathered.
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

// remap maps the staging buffer s again, once the GPU has done with the submission that copied from
// it.
func (s *staging) remap() error {
	p, err := s.buf.MapAsync(wgpu.MapModeWrite, 0, s.size)
	if err != nil {
		return fmt.Errorf("gpu: mapping the staging buffer again: %w", err)
	}
	s.pending = p
	return nil
}
