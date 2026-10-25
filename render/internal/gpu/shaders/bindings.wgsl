// What every program reads: the draw (the target's size, the images' rectangles), the program's
// uniforms (U), up to four images and two samplers, and the helpers Kage had built in.

struct Draw {
    target: vec4<f32>, // xy the target texture's size
    rect0: vec4<f32>,  // each image's rectangle on its texture: xy its origin, zw its size
    rect1: vec4<f32>,
    rect2: vec4<f32>,
    rect3: vec4<f32>,
    place: vec4<f32>, // a triangle draw's vertices scaled by xy and moved by zw (gpu.Draw.Place)
    tint: vec4<f32>,  // a triangle draw's plain colours times it (gpu.Draw.Tint)
    // gpu.Draw.Sight: the rays (origin.w 1 for a sight), the matrix's rows z and w, the eye and
    // the bend (eye.w)
    origin: vec4<f32>,
    dx: vec4<f32>,
    dy: vec4<f32>,
    dir: vec4<f32>,
    ddx: vec4<f32>,
    ddy: vec4<f32>,
    rowZ: vec4<f32>,
    rowW: vec4<f32>,
    eye: vec4<f32>,
}

@group(0) @binding(0) var<uniform> D: Draw;
@group(0) @binding(1) var<uniform> U: Uniforms;
@group(1) @binding(0) var image0: texture_2d<f32>;
@group(1) @binding(1) var image1: texture_2d<f32>;
@group(1) @binding(2) var image2: texture_2d<f32>;
@group(1) @binding(3) var image3: texture_2d<f32>;
@group(1) @binding(4) var nearest: sampler;
@group(1) @binding(5) var linear: sampler;
@group(2) @binding(0) var depthImage: texture_depth_2d;
// the frame's images (gpu.Draw.Frame) are declared by whoever names them, bindings 0 up to 7; a
// sampler filtering them, repeated past their edges
@group(3) @binding(8) var wrapped: sampler;

// inside reports whether p lies in the rectangle r (origin, size).
fn inside(p: vec2<f32>, r: vec4<f32>) -> bool {
    return p.x >= r.x && p.y >= r.y && p.x < r.x + r.z && p.y < r.y + r.w;
}

fn imageSrc0At(p: vec2<f32>) -> vec4<f32> {
    if !inside(p, D.rect0) {
        return vec4<f32>(0.0);
    }
    return textureLoad(image0, vec2<i32>(floor(p)), 0);
}

fn imageSrc1At(p: vec2<f32>) -> vec4<f32> {
    if !inside(p, D.rect1) {
        return vec4<f32>(0.0);
    }
    return textureLoad(image1, vec2<i32>(floor(p)), 0);
}

fn imageSrc2At(p: vec2<f32>) -> vec4<f32> {
    if !inside(p, D.rect2) {
        return vec4<f32>(0.0);
    }
    return textureLoad(image2, vec2<i32>(floor(p)), 0);
}

fn imageSrc3At(p: vec2<f32>) -> vec4<f32> {
    if !inside(p, D.rect3) {
        return vec4<f32>(0.0);
    }
    return textureLoad(image3, vec2<i32>(floor(p)), 0);
}

// depthAt is the depth the meshes drawn before left at the pixel p of the texture they were drawn
// into, for a mesh reading their depth buffer (gpu.MeshDraw.ReadDepth): 0 where none was drawn,
// 1 nearest.
fn depthAt(p: vec2<f32>) -> f32 {
    return textureLoad(depthImage, vec2<i32>(floor(p)), 0);
}

// heightDepth is the depth of what lies at height h under the target's pixel px, through the
// draw's sight: where the line of sight meets h, sunk as the camera sinks the far ground.
fn heightDepth(px: vec2<f32>, h: f32) -> f32 {
    let o = D.origin.xyz + px.x * D.dx.xyz + px.y * D.dy.xyz;
    let way = D.dir.xyz + px.x * D.ddx.xyz + px.y * D.ddy.xyz;
    if abs(way.z) < 1e-9 {
        return 0.0;
    }
    var z = h;
    if D.eye.w != 0.0 {
        let d = (o + way * ((h - o.z) / way.z)).xy - D.eye.xy;
        z = h - D.eye.w * dot(d, d);
    }
    let p = vec4<f32>(o + way * ((z - o.z) / way.z), 1.0);
    return dot(D.rowZ, p) / dot(D.rowW, p);
}

fn imageSrc0Origin() -> vec2<f32> { return D.rect0.xy; }
fn imageSrc0Size() -> vec2<f32> { return D.rect0.zw; }
fn imageSrc1Origin() -> vec2<f32> { return D.rect1.xy; }
fn imageSrc2Origin() -> vec2<f32> { return D.rect2.xy; }
fn imageSrc3Origin() -> vec2<f32> { return D.rect3.xy; }

// imageSrcNLinear is image N blended between the four pixels round p, as imageSrcNAt places them,
// by the GPU's filter: held within the image's rectangle, read in any stage.
fn imageSrc0Linear(p: vec2<f32>) -> vec4<f32> {
    let q = clamp(p, D.rect0.xy + 0.5, D.rect0.xy + D.rect0.zw - 0.5);
    return textureSampleLevel(image0, linear, q / vec2<f32>(textureDimensions(image0)), 0.0);
}

fn imageSrc1Linear(p: vec2<f32>) -> vec4<f32> {
    let q = clamp(p, D.rect1.xy + 0.5, D.rect1.xy + D.rect1.zw - 0.5);
    return textureSampleLevel(image1, linear, q / vec2<f32>(textureDimensions(image1)), 0.0);
}

fn imageSrc2Linear(p: vec2<f32>) -> vec4<f32> {
    let q = clamp(p, D.rect2.xy + 0.5, D.rect2.xy + D.rect2.zw - 0.5);
    return textureSampleLevel(image2, linear, q / vec2<f32>(textureDimensions(image2)), 0.0);
}

fn imageSrc3Linear(p: vec2<f32>) -> vec4<f32> {
    let q = clamp(p, D.rect3.xy + 0.5, D.rect3.xy + D.rect3.zw - 0.5);
    return textureSampleLevel(image3, linear, q / vec2<f32>(textureDimensions(image3)), 0.0);
}

// modulo is GLSL's and Kage's mod: x - y·floor(x/y), unlike WGSL's %, which truncates.
fn modulo(x: f32, y: f32) -> f32 { return x - y * floor(x / y); }
fn modulo2(x: vec2<f32>, y: f32) -> vec2<f32> { return x - y * floor(x / y); }
fn modulo3(x: vec3<f32>, y: f32) -> vec3<f32> { return x - y * floor(x / y); }
