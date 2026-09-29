package camera

// Transform is how a camera's picture lies on the GPU: a world point p, first sunk under the eye's
// level by Bend·d² — d how far it lies along the ground from Eye — goes to clip space as M·(p, 1),
// M column by column as WGSL's mat4x4 holds it. The depth is reversed: 1 nearest, 0 furthest.
type Transform struct {
	M    [16]float32
	Eye  [3]float32
	Bend float32
}

// TransformOf is the transform drawing the rays f onto a viewport w by h pixels: a parallel
// projection (DDX and DDY zero) with its depth from near to far along the rays, a perspective
// (DX and DY zero) with near the nearest it draws and nothing too far; false for rays that do not
// span the space.
func TransformOf(f RayField, w, h, near, far float32) (Transform, bool) {
	t := Transform{Eye: f.Origin, Bend: f.Bend}
	// the screen is (sx, sy) pixels; clip x = 2sx/w - 1, y = 1 - 2sy/h
	if f.DDX == ([3]float32{}) && f.DDY == ([3]float32{}) {
		// parallel: p = O + sx·DX + sy·DY + s·Dir, so (sx, sy, s) = A⁻¹(p - O), A = [DX DY Dir]
		inv, ok := inverse3(f.DX, f.DY, f.Dir)
		if !ok || far <= near {
			return t, false
		}
		// rows of the map from p to (sx, sy, s), with the translation by -O
		rows := [3][4]float32{}
		for r := range 3 {
			rows[r] = [4]float32{inv[r][0], inv[r][1], inv[r][2], -dot(inv[r], f.Origin)}
		}
		// clip = (2sx/w - 1, 1 - 2sy/h, (far - s)/(far - near), 1)
		x := scaleRow(rows[0], 2/w, -1)
		y := scaleRow(rows[1], -2/h, 1)
		z := scaleRow(rows[2], -1/(far-near), far/(far-near))
		t.M = columns(x, y, z, [4]float32{0, 0, 0, 1})
		return t, true
	}
	// perspective: p - E = s·Dir + s·sx·DDX + s·sy·DDY, so (s·sx, s·sy, s) = B⁻¹(p - E),
	// B = [DDX DDY Dir]; s is how far p lies along the rays, the clip w
	inv, ok := inverse3(f.DDX, f.DDY, f.Dir)
	if !ok || near <= 0 {
		return t, false
	}
	rows := [3][4]float32{}
	for r := range 3 {
		rows[r] = [4]float32{inv[r][0], inv[r][1], inv[r][2], -dot(inv[r], f.Origin)}
	}
	X, Y, W := rows[0], rows[1], rows[2]
	// clip = (2X/w - W, W - 2Y/h, near, W): the depth near/s, 1 at near, nothing past infinity
	x := sub(scaleRow(X, 2/w, 0), W)
	y := sub(W, scaleRow(Y, 2/h, 0))
	z := [4]float32{0, 0, 0, near}
	t.M = columns(x, y, z, W)
	return t, true
}

// SceneTransform is the transform every mesh of a scene is drawn through, the rays f onto a
// viewport w by h, so their depths meet: along parallel rays from well before where they start to
// far past the ground, in perspective from a quarter of a unit off the eye.
func SceneTransform(f RayField, w, h float32) (Transform, bool) {
	near, far := float32(-1e4), float32(1e5)
	if f.DDX != ([3]float32{}) || f.DDY != ([3]float32{}) {
		near = 0.25
	}
	return TransformOf(f, w, h, near, far)
}

// Apply is where t draws the world point (x, y, z) — sunk by its Bend — on a viewport w by h
// pixels, and its depth; false behind the eye.
func (t Transform) Apply(x, y, z, w, h float32) (sx, sy, depth float32, ok bool) {
	dx, dy := x-t.Eye[0], y-t.Eye[1]
	z -= t.Bend * (dx*dx + dy*dy)
	m := t.M
	cx := m[0]*x + m[4]*y + m[8]*z + m[12]
	cy := m[1]*x + m[5]*y + m[9]*z + m[13]
	cz := m[2]*x + m[6]*y + m[10]*z + m[14]
	cw := m[3]*x + m[7]*y + m[11]*z + m[15]
	if cw <= 0 {
		return 0, 0, 0, false
	}
	return (cx/cw + 1) * w / 2, (1 - cy/cw) * h / 2, cz / cw, true
}

func dot(a [3]float32, b [3]float32) float32 { return a[0]*b[0] + a[1]*b[1] + a[2]*b[2] }

// scaleRow is k·r plus c in the constant term.
func scaleRow(r [4]float32, k, c float32) [4]float32 {
	return [4]float32{r[0] * k, r[1] * k, r[2] * k, r[3]*k + c}
}

func sub(a, b [4]float32) [4]float32 {
	return [4]float32{a[0] - b[0], a[1] - b[1], a[2] - b[2], a[3] - b[3]}
}

// columns is the matrix whose rows are x, y, z and w, column by column.
func columns(x, y, z, w [4]float32) [16]float32 {
	var m [16]float32
	for c := range 4 {
		m[4*c], m[4*c+1], m[4*c+2], m[4*c+3] = x[c], y[c], z[c], w[c]
	}
	return m
}

// inverse3 is the inverse of the matrix whose columns are a, b and c, by rows.
func inverse3(a, b, c [3]float32) ([3][3]float32, bool) {
	// rows of the inverse are the cross products over the determinant
	cross := func(u, v [3]float32) [3]float32 {
		return [3]float32{u[1]*v[2] - u[2]*v[1], u[2]*v[0] - u[0]*v[2], u[0]*v[1] - u[1]*v[0]}
	}
	bc, ca, ab := cross(b, c), cross(c, a), cross(a, b)
	det := dot(a, bc)
	if det == 0 {
		return [3][3]float32{}, false
	}
	k := 1 / det
	return [3][3]float32{
		{bc[0] * k, bc[1] * k, bc[2] * k},
		{ca[0] * k, ca[1] * k, ca[2] * k},
		{ab[0] * k, ab[1] * k, ab[2] * k},
	}, true
}

// Unproject is the inverse of t's matrix, column by column: from clip space — a screen point and
// its depth — back to the world point drawn there, sunk as t sinks it; false where there is none.
func (t Transform) Unproject() ([16]float32, bool) {
	var m [16]float64
	for i, v := range t.M {
		m[i] = float64(v)
	}
	var inv [16]float64
	inv[0] = m[5]*m[10]*m[15] - m[5]*m[11]*m[14] - m[9]*m[6]*m[15] + m[9]*m[7]*m[14] + m[13]*m[6]*m[11] - m[13]*m[7]*m[10]
	inv[4] = -m[4]*m[10]*m[15] + m[4]*m[11]*m[14] + m[8]*m[6]*m[15] - m[8]*m[7]*m[14] - m[12]*m[6]*m[11] + m[12]*m[7]*m[10]
	inv[8] = m[4]*m[9]*m[15] - m[4]*m[11]*m[13] - m[8]*m[5]*m[15] + m[8]*m[7]*m[13] + m[12]*m[5]*m[11] - m[12]*m[7]*m[9]
	inv[12] = -m[4]*m[9]*m[14] + m[4]*m[10]*m[13] + m[8]*m[5]*m[14] - m[8]*m[6]*m[13] - m[12]*m[5]*m[10] + m[12]*m[6]*m[9]
	inv[1] = -m[1]*m[10]*m[15] + m[1]*m[11]*m[14] + m[9]*m[2]*m[15] - m[9]*m[3]*m[14] - m[13]*m[2]*m[11] + m[13]*m[3]*m[10]
	inv[5] = m[0]*m[10]*m[15] - m[0]*m[11]*m[14] - m[8]*m[2]*m[15] + m[8]*m[3]*m[14] + m[12]*m[2]*m[11] - m[12]*m[3]*m[10]
	inv[9] = -m[0]*m[9]*m[15] + m[0]*m[11]*m[13] + m[8]*m[1]*m[15] - m[8]*m[3]*m[13] - m[12]*m[1]*m[11] + m[12]*m[3]*m[9]
	inv[13] = m[0]*m[9]*m[14] - m[0]*m[10]*m[13] - m[8]*m[1]*m[14] + m[8]*m[2]*m[13] + m[12]*m[1]*m[10] - m[12]*m[2]*m[9]
	inv[2] = m[1]*m[6]*m[15] - m[1]*m[7]*m[14] - m[5]*m[2]*m[15] + m[5]*m[3]*m[14] + m[13]*m[2]*m[7] - m[13]*m[3]*m[6]
	inv[6] = -m[0]*m[6]*m[15] + m[0]*m[7]*m[14] + m[4]*m[2]*m[15] - m[4]*m[3]*m[14] - m[12]*m[2]*m[7] + m[12]*m[3]*m[6]
	inv[10] = m[0]*m[5]*m[15] - m[0]*m[7]*m[13] - m[4]*m[1]*m[15] + m[4]*m[3]*m[13] + m[12]*m[1]*m[7] - m[12]*m[3]*m[5]
	inv[14] = -m[0]*m[5]*m[14] + m[0]*m[6]*m[13] + m[4]*m[1]*m[14] - m[4]*m[2]*m[13] - m[12]*m[1]*m[6] + m[12]*m[2]*m[5]
	inv[3] = -m[1]*m[6]*m[11] + m[1]*m[7]*m[10] + m[5]*m[2]*m[11] - m[5]*m[3]*m[10] - m[9]*m[2]*m[7] + m[9]*m[3]*m[6]
	inv[7] = m[0]*m[6]*m[11] - m[0]*m[7]*m[10] - m[4]*m[2]*m[11] + m[4]*m[3]*m[10] + m[8]*m[2]*m[7] - m[8]*m[3]*m[6]
	inv[11] = -m[0]*m[5]*m[11] + m[0]*m[7]*m[9] + m[4]*m[1]*m[11] - m[4]*m[3]*m[9] - m[8]*m[1]*m[7] + m[8]*m[3]*m[5]
	inv[15] = m[0]*m[5]*m[10] - m[0]*m[6]*m[9] - m[4]*m[1]*m[10] + m[4]*m[2]*m[9] + m[8]*m[1]*m[6] - m[8]*m[2]*m[5]
	det := m[0]*inv[0] + m[1]*inv[4] + m[2]*inv[8] + m[3]*inv[12]
	var out [16]float32
	if det == 0 {
		return out, false
	}
	for i := range inv {
		out[i] = float32(inv[i] / det)
	}
	return out, true
}
