// The clouds seen from above (aloft.Renderer): the layer the sky shows from below, U.AloftBase up,
// laid as aloftSlices thin slices up to aloftDeep clouds' widths over it, one higher up only where
// the cloud is thick, so a cloud stands as a body, lit from above; U.AloftShown at its thickest.
// The line of sight of the pixel px of a view U.AloftView pixels wide is the camera's field
// (camera.RayField): from U.AloftFrom + px.x·U.AloftDX + px.y·U.AloftDY along U.AloftWay +
// px.x·U.AloftWayX + px.y·U.AloftWayY; U.AloftOrtho 1 for lines of sight all one way, whose start
// is any point along them.

struct Out {
    @builtin(position) clip: vec4<f32>,
    @location(0) px: vec2<f32>,
}

@vertex
fn vs_main(@builtin(vertex_index) vid: u32) -> Out {
    var corners = array<vec2<f32>, 3>(
        vec2<f32>(-1.0, -1.0),
        vec2<f32>(3.0, -1.0),
        vec2<f32>(-1.0, 3.0),
    );
    var o: Out;
    o.clip = vec4<f32>(corners[vid], 0.0, 1.0);
    o.px = vec2<f32>(corners[vid].x + 1.0, 1.0 - corners[vid].y) * 0.5 * U.AloftView;
    return o;
}

@fragment
fn fs_main(o: Out) -> @location(0) vec4<f32> {
    let from = U.AloftFrom + o.px.x * U.AloftDX + o.px.y * U.AloftDY;
    let way = U.AloftWay + o.px.x * U.AloftWayX + o.px.y * U.AloftWayY;
    if way.z > -1e-4 {
        return vec4<f32>(0.0);
    }
    let deep = aloftDeep * cloudSize;
    let ortho = U.AloftOrtho > 0.5;
    // the air mass once, where the line of sight crosses the layer's middle: it changes slowly
    let middle = (U.AloftBase + 0.5 * deep - from.z) / way.z;
    if middle <= 0.0 && !ortho {
        return vec4<f32>(0.0);
    }
    let m = airAt(from.xy + way.xy * middle);
    if m.x <= 0.0 {
        return vec4<f32>(0.0);
    }
    let light = U.Ambience * cloudsSkyLit + U.SunColor * U.SunStrength * cloudsSunLit;
    var seen = vec4<f32>(0.0);
    for (var k = aloftSlices - 1; k >= 0; k--) {
        let level = (f32(k) + 0.5) / f32(aloftSlices);
        let t = (U.AloftBase + level * deep - from.z) / way.z;
        if t <= 0.0 && !ortho {
            continue;
        }
        let p = from.xy + way.xy * t;
        // world units a pixel spans there, for the level of the clouds' tile to read
        let pixel = max(length(U.AloftDX.xy + U.AloftWayX.xy * t),
            length(U.AloftDY.xy + U.AloftWayY.xy * t));
        let lod = cloudTileLevel(pixel, cloudTile * cloudSize);
        let torn = cloudTileAt(p, lod, cloudTile * cloudSize).r;
        var core = 0.0;
        if m.y > 0.0 {
            let heapLod = cloudTileLevel(pixel, heapTile * heapSize);
            core = cloudTileAt(p, heapLod, heapTile * heapSize).g;
        }
        let field = cloudFromTile(torn, core, m.y);
        let cover = cloudCoverSoft(field, cloudTileSoft * lod, m);
        let thick = cloudThick(field, m) + 0.5 / f32(aloftSlices);
        let up = smoothstep(level - aloftRound, level, thick);
        let a = cover * up * aloftSlice;
        let rgb = light * mix(aloftUnder, 1.0, level);
        seen += (1.0 - seen.a) * vec4<f32>(rgb * a, a);
    }
    return seen * U.AloftShown;
}

// How many slices a cloud is laid as, how deep the layer is in clouds' widths, how gently a slice
// rounds off where the cloud grows too thin for it, how much of what is behind one slice hides,
// and how much darker the lowest is than the top.
const aloftSlices: i32 = 5;
const aloftDeep: f32 = 0.4;
const aloftRound: f32 = 0.3;
const aloftSlice: f32 = 0.3;
const aloftUnder: f32 = 0.75;
