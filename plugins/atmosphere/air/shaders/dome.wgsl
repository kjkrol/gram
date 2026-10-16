// The air masses over the world and its sky (air.Dome): levels of cells, level 0 over the world,
// each further one in coarser cells, all in the frame's image AirMasses, a block of pixels a
// level, its cells in order. U.AirLevelNA is where level N's block begins in the world, its cell's
// size (0, no level) and where its block begins across the picture; U.AirLevelNC the rect it is
// read in.

fn airLevelA(k: i32) -> vec4<f32> {
    if k == 0 {
        return U.AirLevel0A;
    }
    if k == 1 {
        return U.AirLevel1A;
    }
    if k == 2 {
        return U.AirLevel2A;
    }
    if k == 3 {
        return U.AirLevel3A;
    }
    if k == 4 {
        return U.AirLevel4A;
    }
    if k == 5 {
        return U.AirLevel5A;
    }
    return vec4<f32>(0.0);
}

fn airLevelC(k: i32) -> vec4<f32> {
    if k == 0 {
        return U.AirLevel0C;
    }
    if k == 1 {
        return U.AirLevel1C;
    }
    if k == 2 {
        return U.AirLevel2C;
    }
    if k == 3 {
        return U.AirLevel3C;
    }
    if k == 4 {
        return U.AirLevel4C;
    }
    return U.AirLevel5C;
}

// airAt is the air mass over the world point p: its cover, billow, falls and damp; the finest
// level read there, given way to the next along its edge, and past the dome its edge. Without a
// dome, the frame's cover and billow everywhere.
fn airAt(p: vec2<f32>) -> vec4<f32> {
    if U.AirLevel0A.z <= 0.0 {
        return vec4<f32>(U.Cover, U.Billow, 0.0, 0.0);
    }
    var last = 0;
    for (var k = 0; k < 6; k++) {
        let a = airLevelA(k);
        if a.z <= 0.0 {
            break;
        }
        last = k;
        let r = airLevelC(k);
        let inside = min(min(p.x - r.x, r.z - p.x), min(p.y - r.y, r.w - p.y)) / a.z;
        if inside < 0.0 {
            continue;
        }
        let m = airLevel(a, p);
        let next = airLevelA(k + 1);
        if inside >= airBand || next.z <= 0.0 {
            return m;
        }
        return mix(airLevel(next, p), m, inside / airBand);
    }
    let r = airLevelC(last);
    return airLevel(airLevelA(last), clamp(p, r.xy, r.zw));
}

// airLevel is the four cells round p of the level a, blended by the picture's filter.
fn airLevel(a: vec4<f32>, p: vec2<f32>) -> vec4<f32> {
    let t = vec2<f32>(a.w, 0.0) + (p - a.xy) / a.z;
    return textureSampleLevel(AirMasses, wrapped, t / vec2<f32>(textureDimensions(AirMasses)), 0.0);
}

// How many of a level's cells along its edge give way to the next level (air.domeBand).
const airBand: f32 = 2.0;
