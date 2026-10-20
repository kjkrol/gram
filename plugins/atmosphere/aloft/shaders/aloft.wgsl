// A layer of clouds seen from above (aloft.Renderer): the layer the sky shows from below,
// U.AloftBase up, the low heaps where U.AloftLow — with U.AloftBody bodies (cloudHeap) the line of
// sight passes through, lit from above, else one flat sheet on their bases — else the high shreds'
// sheets, each at its own height (cloudHighCross); U.AloftShown at their thickest. With
// U.AloftUnder the eye is under the low heaps' top, and sees them before the ground: as far as its
// line of sight stays under the highest ground (groundHighest), the sky drawing what lies past.
// The line of sight of the pixel px of a view U.AloftView pixels wide is the camera's field
// (camera.RayField): from U.AloftFrom + px.x·U.AloftDX + px.y·U.AloftDY along U.AloftWay +
// px.x·U.AloftWayX + px.y·U.AloftWayY; U.AloftOrtho 1 for lines of sight all one way, whose start
// is any point along them; U.AloftSeeing how far one sees through the air. A cloud thins away into
// the ground (groundHeightAt), and lies as deep as where it begins along the line of sight, through
// the scene's transform (U.AloftViewProj, sunk by U.AloftBend round U.AloftEye).

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

struct Seen {
    @location(0) color: vec4<f32>,
    @builtin(frag_depth) depth: f32,
}

@fragment
fn fs_main(o: Out) -> Seen {
    if U.AloftUnder > 0.5 {
        return aloftBelow(o);
    }
    var seen = aloftSeen(o);
    if U.AloftOrtho > 0.5 || seen.color.a <= 0.0 {
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
    let rgb = mix(seen.color.rgb, U.Fog * seen.color.a, haze);
    seen.color = vec4<f32>(rgb, seen.color.a) * smoothstep(skyLift, 3.0 * skyLift, down);
    return seen;
}

// aloftSeen is the clouds the pixel o sees from above, premultiplied, as deep as where they begin.
fn aloftSeen(o: Out) -> Seen {
    var none = Seen(vec4<f32>(0.0), 0.0);
    let from = U.AloftFrom + o.px.x * U.AloftDX + o.px.y * U.AloftDY;
    let way = U.AloftWay + o.px.x * U.AloftWayX + o.px.y * U.AloftWayY;
    if way.z > -1e-4 {
        return none;
    }
    let ortho = U.AloftOrtho > 0.5;
    let low = U.AloftLow > 0.5;
    var body = U.AloftBody > 0.5;
    var span = cloudBodySpan(low);
    // the bodies reach U.AloftBodiesTo from where the camera looks, a sheet beyond
    if body && U.AloftBodiesTo > 0.0 {
        let at = from + way * ((span.y - from.z) / way.z);
        body = length(at.xy - U.AloftMiddle) < U.AloftBodiesTo;
    }
    if low && !body {
        span = cloudLowBases();
    }
    // the layer along the line of sight, from its top down to its base
    var enter = (span.y - from.z) / way.z;
    let leave = (span.x - from.z) / way.z;
    if !ortho {
        if leave <= 0.0 {
            return none;
        }
        enter = max(enter, 0.0);
    }
    // the ground may stand up into the layer, as high as the highest ground
    let highest = groundHighest();
    let grounded = highest > span.x;
    let light = U.Ambience * cloudsSkyLit + U.LightColor * U.LightStrength * cloudsSunLit;
    if !low || !body {
        // a sheet: the high ones at their own heights, the low ones on their bases
        var t = leave;
        if span.y > span.x {
            t = cloudSheetCross(from, way, enter, leave, low);
            if t < 0.0 && !ortho {
                return none;
            }
        }
        let q = from + way * t;
        var m = cloudLowMass(airAt(q.xy));
        if !low {
            m = cloudHighHere(cloudHighMass(airAt(q.xy)), q.xy);
        }
        if m.x <= 0.0 {
            return none;
        }
        var clear = 1.0;
        if grounded {
            var band = vec2<f32>(0.0, 1e9);
            if low {
                band = cloudLowBand(q.xy);
            }
            clear = aloftClear(q, band);
        }
        var c = cloudHighAt(q.xy, aloftPixel(t));
        if low {
            c = cloudLowAt(q.xy, aloftPixel(t), m.y);
        }
        var a = cloudCoverSoft(c.x, c.y, m) * clear * aloftFlat;
        if !low {
            a *= 0.5 + 0.5 * cloudThick(c.x, m); // thin wisps, thicker only in the middle
        }
        return Seen(vec4<f32>(light * a, a) * U.AloftShown, aloftDepth(q));
    }
    let ray = cloudLowRay(from, way, enter, leave);
    // the air mass once, where the line of sight crosses the layer's middle: it changes slowly
    let m = cloudLowMass(airAt(from.xy + way.xy * 0.5 * (ray.enter + ray.leave)));
    if m.x <= 0.0 || ray.leave <= ray.enter && !ortho {
        return none;
    }
    let step = (ray.leave - ray.enter) / f32(aloftSteps);
    let along = length(way);
    // the steps put off at random a pixel: an even pattern showed as stripes
    let dither = fract(sin(dot(o.px, vec2<f32>(12.9898, 78.233))) * 43758.5453);
    var seen = Seen(vec4<f32>(0.0), 0.0);
    for (var i = 0; i < aloftSteps; i++) {
        let t = ray.enter + (f32(i) + dither) * step;
        let q = from + way * t;
        let band = cloudRayBand(ray, t);
        var clear = 1.0; // how far the point stands off the ground, the heap thinning into it
        if grounded && q.z < highest + aloftGroundSoft * cloudSize {
            clear = aloftClear(q, band);
            if clear <= 0.0 {
                continue;
            }
        }
        let heap = cloudHeap(q.xy, q.z, band, aloftPixel(t), m);
        if heap.x <= 0.0 {
            continue;
        }
        let deep = cloudLumpsTop(band) - band.x;
        let a = 1.0 - exp(-heap.x * clear * aloftThick * step * along / deep);
        let rgb = light * mix(aloftUnder, 1.0, heap.y);
        if seen.color.a <= 0.0 {
            seen.depth = aloftDepth(q);
        }
        seen.color += (1.0 - seen.color.a) * vec4<f32>(rgb * a, a);
        if seen.color.a > 0.97 {
            break;
        }
    }
    seen.color *= U.AloftShown;
    return seen;
}

// aloftBelow is the low heaps the pixel o sees from under their top, before the ground: as far
// as its line of sight stays under the highest ground, lit as the sky lights them, darker
// underneath, hazed with the distance; premultiplied, as deep as where they begin.
fn aloftBelow(o: Out) -> Seen {
    var none = Seen(vec4<f32>(0.0), 0.0);
    let from = U.AloftFrom + o.px.x * U.AloftDX + o.px.y * U.AloftDY;
    let way = U.AloftWay + o.px.x * U.AloftWayX + o.px.y * U.AloftWayY;
    let span = cloudLowSpan();
    let highest = groundHighest();
    if highest <= span.x {
        return none; // nothing stands up into the clouds
    }
    let len = length(way);
    var enter = 0.0;
    var leave = aloftReach() / len;
    if abs(way.z) > 1e-6 {
        let ta = (span.x - from.z) / way.z;
        let tb = (span.y - from.z) / way.z;
        enter = max(min(ta, tb), 0.0);
        leave = min(leave, max(ta, tb));
        if way.z > 0.0 {
            leave = min(leave, (highest - from.z) / way.z); // past it, the sky's
        }
    } else if from.z < span.x || from.z > min(span.y, highest) {
        return none;
    }
    if leave <= enter {
        return none;
    }
    let ray = cloudLowRay(from, way, enter, leave);
    let m = cloudLowMass(airAt(from.xy + way.xy * 0.5 * (ray.enter + ray.leave)));
    if m.x <= 0.0 || ray.leave <= ray.enter {
        return none;
    }
    let sky = U.Ambience * cloudsSkyLit;
    let sun = U.LightColor * U.LightStrength * cloudsSunLit;
    let step = (ray.leave - ray.enter) / f32(aloftBelowSteps);
    let dither = fract(sin(dot(o.px, vec2<f32>(12.9898, 78.233))) * 43758.5453);
    var seen = Seen(vec4<f32>(0.0), 0.0);
    var first = 0.0;
    for (var i = 0; i < aloftBelowSteps; i++) {
        let t = ray.enter + (f32(i) + dither) * step;
        let q = from + way * t;
        let band = cloudRayBand(ray, t);
        var clear = 1.0;
        if q.z < highest + aloftGroundSoft * cloudSize {
            clear = aloftClear(q, band);
            if clear <= 0.0 {
                break; // into the ground: what lies behind it is hidden
            }
        }
        let heap = cloudHeap(q.xy, q.z, band, aloftPixel(t), m);
        let a = heap.x * clear * aloftDense;
        if a <= 0.0 {
            continue;
        }
        if seen.color.a <= 0.0 {
            seen.depth = aloftDepth(q);
            first = t;
        }
        let rgb = sky * mix(aloftUnder, 1.0, heap.y) + sun * mix(aloftBase, 1.0, heap.y);
        seen.color += (1.0 - seen.color.a) * vec4<f32>(rgb * a, a);
        if seen.color.a > 0.97 {
            break;
        }
    }
    if seen.color.a <= 0.0 {
        return none;
    }
    var haze = 0.0;
    if U.AloftSeeing > 0.0 {
        haze = 1.0 - exp(-first * len / U.AloftSeeing);
    }
    seen.color = vec4<f32>(mix(seen.color.rgb, U.Fog * seen.color.a, haze), seen.color.a) *
        U.AloftShown;
    return seen;
}

// aloftClear is how far the point q stands off the ground under it, 0 in it up to 1, the clouds
// of the band band (cloudLowBand) thinning into it over a share of their deck.
fn aloftClear(q: vec3<f32>, band: vec2<f32>) -> f32 {
    let soft = min(aloftGroundSoft * cloudSize, aloftGroundDeck * (band.y - band.x));
    return smoothstep(0.0, soft, q.z - groundHeightAt(q.xy));
}

// aloftReach is how far off a line of sight along the clouds looks for them, world units: as far
// as the air lets one see, past which they are lost in it.
fn aloftReach() -> f32 {
    if U.AloftSeeing > 0.0 {
        return aloftSeeingOut * U.AloftSeeing;
    }
    return aloftSeeingOut * cloudCeilingWide;
}

// aloftDepth is the depth of the world point p through the scene's transform, as the ground's.
fn aloftDepth(p: vec3<f32>) -> f32 {
    let off = p.xy - U.AloftEye.xy;
    let c = U.AloftViewProj * vec4<f32>(p.xy, p.z - U.AloftBend * dot(off, off), 1.0);
    if c.w <= 1e-6 {
        return 0.0;
    }
    return clamp(c.z / c.w, 0.0, 1.0);
}

// aloftPixel is how many world units a pixel spans t along its line of sight, for the level of the
// clouds' tile to read.
fn aloftPixel(t: f32) -> f32 {
    return max(length(U.AloftDX.xy + U.AloftWayX.xy * t),
        length(U.AloftDY.xy + U.AloftWayY.xy * t));
}

// How many steps a line of sight takes through the heaps, from above and from under them; how
// much of what lies behind a heap's whole height it hides, and a sheet, and behind one step from
// under them; how much darker the cloud's shadowed underside is than its sunlit top in the sky's
// light, and its base in the sun's; how high over the ground a cloud thins away into it, in
// clouds' widths, and at most in shares of its deck: a hard edge along the slopes showed their
// lattice; how many times as far as the air lets one see the clouds are looked for along them.
const aloftSteps: i32 = 10;
const aloftBelowSteps: i32 = 12;
const aloftThick: f32 = 1.6;
const aloftFlat: f32 = 0.4;
const aloftDense: f32 = 0.35;
const aloftUnder: f32 = 0.7;
const aloftBase: f32 = 0.55;
const aloftGroundSoft: f32 = 0.15;
const aloftGroundDeck: f32 = 0.5;
const aloftSeeingOut: f32 = 2.0;
