// The way to the shore from every corner of a square board's lattice (painter's coast.go), a
// pixel a corner over a target U.PaintTarget large, as the terrain's lattice holds it: the way
// in red and green as (way + 1) / 2, the distance over the reach in blue. The nearest cell that
// does not shine within U.PaintCoast cells, rings round the corner searched out to it; on the
// shore itself the way into the land touching it; open water, the whole reach, beyond.

@vertex
fn vs_main(@builtin(vertex_index) vid: u32) -> @builtin(position) vec4<f32> {
    let uv = vec2<f32>(f32((vid << 1u) & 2u), f32(vid & 2u));
    return vec4<f32>(uv * 2.0 - 1.0, 0.0, 1.0);
}

// dry reports whether the cell c is land: on the board and not shining.
fn dry(c: vec2<f32>) -> bool {
    let b = folded(c);
    return b.z > 0.5 && topOf(b.xy).shine <= 0.0;
}

// nearest is the way from p to the cell c where it is land and nearer than best away (squared),
// with how far squared; best and no way where it is not.
fn nearest(c: vec2<f32>, p: vec2<f32>, best: vec3<f32>) -> vec3<f32> {
    if !dry(c) {
        return best;
    }
    let lo = c * U.PaintCell;
    let d = clamp(p, lo, lo + U.PaintCell) - p;
    let dist = dot(d, d);
    if dist < best.z {
        return vec3<f32>(d, dist);
    }
    return best;
}

@fragment
fn fs_main(@builtin(position) frag: vec4<f32>) -> @location(0) vec4<f32> {
    let g = floor(frag.xy);
    let size = U.PaintCell;
    let reach = U.PaintCoast * size;
    let p = g * size;
    var best = vec3<f32>(0.0, 0.0, reach * reach);
    let rings = i32(U.PaintCoast + 0.5);
    for (var r = 0; r <= rings; r++) {
        let far = f32(r) * size;
        if far * far >= best.z {
            break;
        }
        let lo = -1 - r;
        let hi = r;
        for (var x = lo; x <= hi; x++) {
            best = nearest(g + vec2<f32>(f32(x), f32(lo)), p, best);
            best = nearest(g + vec2<f32>(f32(x), f32(hi)), p, best);
        }
        for (var y = lo + 1; y < hi; y++) {
            best = nearest(g + vec2<f32>(f32(lo), f32(y)), p, best);
            best = nearest(g + vec2<f32>(f32(hi), f32(y)), p, best);
        }
    }
    if best.z >= reach * reach {
        return vec4<f32>(0.5, 0.5, 1.0, 1.0);
    }
    let d = sqrt(best.z);
    var way = best.xy;
    if d == 0.0 {
        // on the shore: into the land of the four cells meeting at the corner
        way = vec2<f32>(0.0);
        for (var k = 0; k < 4; k++) {
            let o = vec2<f32>(f32(k % 2) - 1.0, f32(k / 2) - 1.0);
            if dry(g + o) {
                way = way + (o + 0.5) * size;
            }
        }
    }
    var unit = vec2<f32>(0.0);
    if length(way) > 0.0 {
        unit = way / length(way);
    }
    return vec4<f32>((unit + 1.0) / 2.0, d / reach, 1.0);
}
