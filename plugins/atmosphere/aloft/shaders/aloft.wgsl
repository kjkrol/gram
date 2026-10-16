// A layer of clouds seen from above (aloft.Renderer): the layer the sky shows from below,
// U.AloftBase up, the low heaps where U.AloftLow — with U.AloftBody bodies (cloudHeap) the line of
// sight passes through, lit from above, else one flat sheet — else the high shreds' sheets, each
// at its own height (cloudHighCross); U.AloftShown at their thickest. The line of sight of the
// pixel px of a view U.AloftView pixels wide is the camera's field (camera.RayField): from
// U.AloftFrom + px.x·U.AloftDX + px.y·U.AloftDY along U.AloftWay + px.x·U.AloftWayX +
// px.y·U.AloftWayY; U.AloftOrtho 1 for lines of sight all one way, whose start is any point along
// them; U.AloftSeeing how far one sees through the air. A cloud thins away into the ground
// (groundHeightAt).

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
    let seen = aloftSeen(o);
    if U.AloftOrtho > 0.5 || seen.a <= 0.0 {
        return seen;
    }
    // through a perspective, fading along the horizon and turning to the fog as far off as the air
    // hides them: seen edge on they showed as a band
    let from = U.AloftFrom + o.px.x * U.AloftDX + o.px.y * U.AloftDY;
    let way = U.AloftWay + o.px.x * U.AloftWayX + o.px.y * U.AloftWayY;
    let down = -way.z / length(way);
    var haze = 0.0;
    if U.AloftSeeing > 0.0 {
        haze = 1.0 - exp(-((from.z - U.AloftBase) / max(down, 1e-3)) / U.AloftSeeing);
    }
    let rgb = mix(seen.rgb, U.Fog * seen.a, haze);
    return vec4<f32>(rgb, seen.a) * smoothstep(skyLift, 3.0 * skyLift, down);
}

// aloftSeen is the clouds the pixel o sees from above, premultiplied.
fn aloftSeen(o: Out) -> vec4<f32> {
    let from = U.AloftFrom + o.px.x * U.AloftDX + o.px.y * U.AloftDY;
    let way = U.AloftWay + o.px.x * U.AloftWayX + o.px.y * U.AloftWayY;
    if way.z > -1e-4 {
        return vec4<f32>(0.0);
    }
    let ortho = U.AloftOrtho > 0.5;
    let low = U.AloftLow > 0.5;
    let body = U.AloftBody > 0.5;
    var span = cloudBodySpan(low);
    if low && !body {
        span.y = span.x;
    }
    // the layer along the line of sight, from its top down to its base
    var enter = (span.y - from.z) / way.z;
    let leave = (span.x - from.z) / way.z;
    if !ortho {
        if leave <= 0.0 {
            return vec4<f32>(0.0);
        }
        enter = max(enter, 0.0);
    }
    // the ground may stand up into the layer, as high as the highest ground
    let highest = groundHighest();
    let grounded = highest > span.x;
    let light = U.Ambience * cloudsSkyLit + U.SunColor * U.SunStrength * cloudsSunLit;
    if !low || !body {
        // a sheet: the high ones at their own heights, the low ones flat on their base
        var t = leave;
        if !low {
            t = cloudHighCross(from, way, enter, leave);
            if t < 0.0 && !ortho {
                return vec4<f32>(0.0);
            }
        }
        let q = from + way * t;
        var m = cloudLowMass(airAt(q.xy));
        if !low {
            m = cloudHighHere(cloudHighMass(airAt(q.xy)), q.xy);
        }
        if m.x <= 0.0 {
            return vec4<f32>(0.0);
        }
        var clear = 1.0;
        if grounded {
            clear = smoothstep(0.0, aloftGroundSoft * cloudSize, q.z - groundHeightAt(q.xy));
        }
        var c = cloudHighAt(q.xy, aloftPixel(t));
        if low {
            c = cloudLowAt(q.xy, aloftPixel(t), m.y);
        }
        var a = cloudCoverSoft(c.x, c.y, m) * clear * aloftFlat;
        if !low {
            a *= 0.5 + 0.5 * cloudThick(c.x, m); // thin wisps, thicker only in the middle
        }
        return vec4<f32>(light * a, a) * U.AloftShown;
    }
    // the air mass once, where the line of sight crosses the layer's middle: it changes slowly
    let m = cloudLowMass(airAt(from.xy + way.xy * 0.5 * (enter + leave)));
    if m.x <= 0.0 {
        return vec4<f32>(0.0);
    }
    let deep = span.y - span.x;
    let step = (leave - enter) / f32(aloftSteps);
    let along = length(way);
    // the steps put off at random a pixel: an even pattern showed as stripes
    let dither = fract(sin(dot(o.px, vec2<f32>(12.9898, 78.233))) * 43758.5453);
    var seen = vec4<f32>(0.0);
    for (var i = 0; i < aloftSteps; i++) {
        let t = enter + (f32(i) + dither) * step;
        let q = from + way * t;
        var clear = 1.0; // how far the point stands off the ground, the heap thinning into it
        if grounded && q.z < highest + aloftGroundSoft * cloudSize {
            clear = smoothstep(0.0, aloftGroundSoft * cloudSize, q.z - groundHeightAt(q.xy));
            if clear <= 0.0 {
                continue;
            }
        }
        let heap = cloudHeap(q.xy, q.z, aloftPixel(t), m);
        if heap.x <= 0.0 {
            continue;
        }
        let a = 1.0 - exp(-heap.x * clear * aloftThick * step * along / deep);
        let rgb = light * mix(aloftUnder, 1.0, heap.y);
        seen += (1.0 - seen.a) * vec4<f32>(rgb * a, a);
        if seen.a > 0.97 {
            break;
        }
    }
    return seen * U.AloftShown;
}

// aloftPixel is how many world units a pixel spans t along its line of sight, for the level of the
// clouds' tile to read.
fn aloftPixel(t: f32) -> f32 {
    return max(length(U.AloftDX.xy + U.AloftWayX.xy * t),
        length(U.AloftDY.xy + U.AloftWayY.xy * t));
}

// How many steps a line of sight takes through the heaps; how much of what lies behind a heap's
// whole height it hides, and a sheet; how much darker the cloud's
// shadowed underside is than its sunlit top; how high over the ground a cloud thins away into
// it, in clouds' widths: a hard edge along the slopes showed their lattice.
const aloftSteps: i32 = 12;
const aloftThick: f32 = 1.6;
const aloftFlat: f32 = 0.4;
const aloftUnder: f32 = 0.7;
const aloftGroundSoft: f32 = 0.15;
