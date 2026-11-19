// A body photographed into a sprite, a draw a photograph (world's shots): U.ShotBody its row in
// the bodies' table, how wide and how tall it is and how many parts it has; U.ShotFrom the way
// the eye looks, radians against the clock off the body's x, how high over the level it looks
// down from, what the photograph takes, and whether it is taken from straight above. From the
// side the picture is the body thrown along the lines of sight on the upright plane through its
// foot, across the look: as wide as the body, and from under its foot to over its top by what
// the lines climb over half its width (world's shotMargin). From above it is the body's square.
// It takes the body in its tones under the photograph's own light, its paint alone, or the ways
// out of it: across and up the picture, in the red and the green, the third toward the eye, and
// how far up a cone in the blue.

const shotLit: f32 = 0.0;
const shotPaint: f32 = 1.0;
const shotWays: f32 = 2.0;

// the light a body is photographed in for a picture lit once: from the left, over and before
// the picture
const shotLight: vec3<f32> = vec3<f32>(-0.5, 0.62, 0.6);

// ShotView is the eye of a photograph, in the body's frame, whose y runs south as the world's
// does: the way it looks, the picture's way across and the way up it.
struct ShotView {
    way: vec3<f32>,
    right: vec3<f32>,
    up: vec3<f32>,
}

fn shotView() -> ShotView {
    var v: ShotView;
    if U.ShotFrom.w > 0.5 {
        v.way = vec3<f32>(0.0, 0.0, -1.0);
        v.right = vec3<f32>(1.0, 0.0, 0.0);
        v.up = vec3<f32>(0.0, -1.0, 0.0);
        return v;
    }
    let ahead = vec2<f32>(cos(U.ShotFrom.x), sin(U.ShotFrom.x));
    let e = U.ShotFrom.y;
    v.way = vec3<f32>(ahead * cos(e), -sin(e));
    v.right = vec3<f32>(-ahead.y, ahead.x, 0.0);
    v.up = vec3<f32>(ahead * sin(e), cos(e));
    return v;
}

@vertex
fn vs_main(@builtin(vertex_index) vid: u32) -> ShapeCard {
    var c: ShapeCard;
    c.clip = vec4<f32>(0.0, 0.0, -1.0, 1.0);
    let row = U.ShotBody.x;
    let wide = U.ShotBody.y;
    let tall = U.ShotBody.z;
    let slot = f32(vid / 6u);
    if slot >= U.ShotBody.w {
        return c;
    }
    let col = shapeRowTexels + shapePartTexels * slot;
    let t0 = shapeTexel(col, row);
    let t1 = shapeTexel(col + 1.0, row);
    let t2 = shapeTexel(col + 2.0, row);
    if round(t2.x * 255.0) < 0.5 {
        return c;
    }
    let a = shapePoint(t0, wide, tall);
    let b = shapePoint(t1, wide, tall);
    let v = shotView();
    let p = shapeCardOf(&c, vid, a, b, t0.w * wide, t1.w * wide, t2, row, wide, v.way,
        vec3<f32>(0.0), true, slot * 7.31);
    // how deep it lies: nearer the eye the greater, a half at the body's foot
    let reach = 2.0 * (wide + tall);
    var x = p.x / (0.5 * wide);
    var y = -p.y / (0.5 * wide);
    var deep = 0.5 + p.z / reach;
    if U.ShotFrom.w < 0.5 {
        let ahead = vec3<f32>(v.way.xy / max(length(v.way.xy), 1e-6), 0.0);
        let far = dot(p, ahead) / max(dot(v.way, ahead), 1e-6);
        let on = p - v.way * far;
        let margin = 0.5 * wide * tan(U.ShotFrom.y);
        x = dot(on, v.right) / (0.5 * wide);
        y = (on.z + margin) / (tall + 2.0 * margin) * 2.0 - 1.0;
        deep = 0.5 - far / reach;
    }
    c.clip = vec4<f32>(x, y, deep, 1.0);
    c.deep = vec2<f32>(deep, 1.0);
    c.lit.w = -1.0 / reach;
    c.dots.w = 0.0;
    c.light = vec4<f32>(1.0, 1.0, 1.0, 0.0);
    return c;
}

struct Shot {
    @location(0) color: vec4<f32>,
    @builtin(frag_depth) depth: f32,
}

@fragment
fn fs_main(c: ShapeCard) -> Shot {
    let m = shapeMet(c, true);
    if m.far < 0.0 {
        discard;
    }
    let v = shotView();
    var rgb = shapeColour(c, m);
    if U.ShotFrom.z < shotLit + 0.5 {
        let sun = normalize(v.right * shotLight.x + v.up * shotLight.y - v.way * shotLight.z);
        rgb *= shapeTone(shapeFacing(m, sun));
    } else if U.ShotFrom.z > shotWays - 0.5 {
        rgb = vec3<f32>(dot(m.out, v.right) * 0.5 + 0.5, dot(m.out, v.up) * 0.5 + 0.5, m.up);
    }
    let lies = c.deep.x + m.far * c.lit.w;
    return Shot(vec4<f32>(min(rgb, vec3<f32>(1.0)), 1.0), clamp(lies, 0.0, 1.0));
}
