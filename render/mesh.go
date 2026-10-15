package render

import "github.com/kjkrol/gram/render/gpu"

// Depth is a depth buffer for meshes, reversed (1 nearest), the size of the whole texture the
// image they are drawn into lies on.
type Depth struct{ d *gpu.Depth }

// ClearDepth clears the depth buffer d, fitted to the image's texture.
func (i *Image) ClearDepth(d *Depth) { gpu.ClearDepth(i.gpu(), d.d) }

// NewDepth is a depth buffer, fitted to the image it is drawn with.
func NewDepth() *Depth { return &Depth{gpu.NewDepth()} }

// Readable has the buffer read as well as drawn against (DrawMeshOptions.ReadDepth, CopyDepth);
// asked before its first draw, it keeps what it holds.
func (d *Depth) Readable() *Depth {
	d.d.Readable()
	return d
}

// CopyDepth puts what from holds in place of what to holds, both fitted to the image's texture;
// from must be Readable.
func (i *Image) CopyDepth(from, to *Depth) { gpu.CopyDepth(i.gpu(), from.d, to.d) }

// Size is the buffer's width and height, those of the texture it was last drawn with.
func (d *Depth) Size() (w, h int) { return d.d.Size() }

// Release frees the buffer.
func (d *Depth) Release() { d.d.Release() }

// Indices are a mesh's triangles, three vertex indices each, kept on the GPU once drawn.
type Indices struct{ i *gpu.Indices }

// NewIndices keeps the triangles of indices.
func NewIndices(indices []uint32) *Indices { return &Indices{gpu.NewIndices(indices)} }

// Release frees the indices on the GPU.
func (x *Indices) Release() { x.i.Release() }

// NewMeshShaderWith is a mesh's shader on the composer's library and its materials, as
// NewShaderWith, whose source has a vertex stage of its own: vs_main and fs_main, and fs_prime
// for a mesh drawn Primed.
func NewMeshShaderWith(label string, source []byte, own []Uniform) *Shader {
	s := NewShaderWith(label, source, own)
	s.mesh = true
	return s
}

// Instanced has a mesh's shader drawn once an instance, each handing its vertex stage vec4s
// vectors at locations 0 up. Call it before the first draw.
func (s *Shader) Instanced(vec4s int) *Shader {
	s.vec4s = vec4s
	return s
}

// DrawMeshOptions is how a mesh is drawn: against Depth, written where WriteDepth says, without
// one whole and untested. Primed, its depth is laid first alone by the shader's fs_prime, and its
// colour then only where it lies nearest.
type DrawMeshOptions struct {
	Depth      *Depth
	ClearDepth bool
	WriteDepth bool
	Primed     bool
	Vertices   int       // without indices: how many vertices, in order, three a triangle
	Instances  []float32 // an Instanced shader's, its vec4s each
	// ReadDepth is a depth buffer the shader reads (depthAt), another than Depth.
	ReadDepth *Depth
	Images    [4]*Image
	Uniforms  map[string]any
	// Untested draws everywhere, the Depth kept as it is.
	Untested bool
}

// DrawMesh draws the triangles of idx with s, a mesh's shader, into the image; idx nil,
// op.Vertices vertices in order.
func (i *Image) DrawMesh(idx *Indices, s *Shader, op *DrawMeshOptions) {
	var imgs [4]gpu.Image
	for k, im := range op.Images {
		if im != nil {
			imgs[k] = im.gpu()
		}
	}
	var depth *gpu.Depth
	if op.Depth != nil {
		depth = op.Depth.d
	}
	var indices *gpu.Indices
	var read *gpu.Depth
	if op.ReadDepth != nil {
		read = op.ReadDepth.d
	}
	if idx != nil {
		indices = idx.i
	}
	gpu.Mesh(&gpu.MeshDraw{Target: i.gpu(), Depth: depth, Vertices: uint32(max(op.Vertices, 0)), Instances: op.Instances, ReadDepth: read, ClearDepth: op.ClearDepth, Write: op.WriteDepth, Primed: op.Primed,
		Untested: op.Untested,
		Program:  s.program(), Images: imgs, Uniforms: s.pack(op.Uniforms), Indices: indices,
		Blend: gpu.SourceOver})
}
