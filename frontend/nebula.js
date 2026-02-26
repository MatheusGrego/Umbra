// ============================================================================
// NEBULA.JS — WebGL animated space nebula background
// Renders a dark-space nebula with wispy clouds, stars, and floating dust
// using a GPU fragment shader with FBM (Fractal Brownian Motion) noise.
// Designed as the substrate behind Umbra's liquid glass panels.
// ============================================================================

(function () {
  'use strict';

  const VERT_SRC = `
    attribute vec2 a_pos;
    void main() { gl_Position = vec4(a_pos, 0.0, 1.0); }
  `;

  const FRAG_SRC = `
    precision highp float;
    uniform vec2  u_resolution;
    uniform float u_time;
    uniform float u_seed;

    // ── Simplex-style value noise ──────────────────────────────────────
    vec3 mod289(vec3 x) { return x - floor(x * (1.0/289.0)) * 289.0; }
    vec2 mod289(vec2 x) { return x - floor(x * (1.0/289.0)) * 289.0; }
    vec3 permute(vec3 x) { return mod289(((x*34.0)+1.0)*x); }

    float snoise(vec2 v) {
      const vec4 C = vec4(0.211324865405187, 0.366025403784439,
                         -0.577350269189626, 0.024390243902439);
      vec2 i  = floor(v + dot(v, C.yy));
      vec2 x0 = v - i + dot(i, C.xx);
      vec2 i1 = (x0.x > x0.y) ? vec2(1.0, 0.0) : vec2(0.0, 1.0);
      vec4 x12 = x0.xyxy + C.xxzz;
      x12.xy -= i1;
      i = mod289(i);
      vec3 p = permute(permute(i.y + vec3(0.0, i1.y, 1.0))
                                   + i.x + vec3(0.0, i1.x, 1.0));
      vec3 m = max(0.5 - vec3(dot(x0,x0), dot(x12.xy,x12.xy),
                               dot(x12.zw,x12.zw)), 0.0);
      m = m*m; m = m*m;
      vec3 x_ = 2.0 * fract(p * C.www) - 1.0;
      vec3 h  = abs(x_) - 0.5;
      vec3 ox = floor(x_ + 0.5);
      vec3 a0 = x_ - ox;
      m *= 1.79284291400159 - 0.85373472095314 * (a0*a0 + h*h);
      vec3 g;
      g.x = a0.x * x0.x + h.x * x0.y;
      g.yz = a0.yz * x12.xz + h.yz * x12.yw;
      return 130.0 * dot(m, g);
    }

    // ── FBM (Fractal Brownian Motion) — layered noise for cloud detail ─
    float fbm(vec2 p, float t) {
      float val = 0.0;
      float amp = 0.5;
      float freq = 1.0;
      // Slowly drift the noise field over time
      vec2 drift = vec2(t * 0.015, t * 0.008);
      for (int i = 0; i < 6; i++) {
        val += amp * snoise(p * freq + drift);
        drift *= 1.2;          // each octave drifts slightly differently
        freq *= 2.0;
        amp  *= 0.5;
      }
      return val;
    }

    // ── Star field (proper round points) ─────────────────────────────
    float stars(vec2 uv, float density) {
      vec2 cell = floor(uv * density);
      vec2 sub  = fract(uv * density) - 0.5; // -0.5..0.5 from cell center

      // Per-cell random: brightness + offset
      float h  = fract(sin(dot(cell, vec2(127.1, 311.7))) * 43758.5453);
      float h2 = fract(sin(dot(cell, vec2(269.5, 183.3))) * 76125.3214);

      // Only ~1.5% of cells contain a star
      if (h < 0.985) return 0.0;

      // Random sub-pixel offset so stars aren't grid-aligned
      vec2 offset = vec2(
        fract(sin(dot(cell, vec2(419.2, 371.9))) * 29174.1) - 0.5,
        fract(sin(dot(cell, vec2(531.7, 213.3))) * 63281.7) - 0.5
      ) * 0.7;

      float dist = length(sub - offset);

      // Smooth circular falloff (radius ~0.06 of cell)
      float size = 0.03 + h2 * 0.04;
      float star = smoothstep(size, size * 0.2, dist);

      // Subtle twinkle
      star *= 0.7 + 0.3 * sin(u_time * (0.5 + h2 * 2.0) + h * 6.28);

      return star;
    }

    void main() {
      vec2 uv = gl_FragCoord.xy / u_resolution;
      float aspect = u_resolution.x / u_resolution.y;
      vec2 p = vec2(uv.x * aspect, uv.y);

      float t = u_time + u_seed;

      // ── Nebula clouds (multi-layer FBM) ─────────────────────────────
      float n1 = fbm(p * 1.8 + vec2(0.0, 0.0), t);
      float n2 = fbm(p * 2.5 + vec2(5.3, 1.7), t * 0.7);
      float n3 = fbm(p * 3.2 + vec2(8.1, 3.9), t * 0.5);

      // Combine cloud layers — subtler than before
      float cloud = 0.0;
      cloud += smoothstep(-0.1, 0.6, n1) * 0.45;
      cloud += smoothstep(0.0,  0.5, n2) * 0.25;
      cloud += smoothstep(0.1,  0.7, n3) * 0.18;
      cloud = clamp(cloud, 0.0, 1.0);

      // ── Color palette (dark space: deep blue, purple, cool white) ───
      vec3 darkBase   = vec3(0.012, 0.012, 0.028);
      vec3 deepBlue   = vec3(0.04, 0.06, 0.16);
      vec3 nebulaPurp = vec3(0.08, 0.04, 0.16);
      vec3 nebulaBlue = vec3(0.10, 0.16, 0.32);
      vec3 brightCore = vec3(0.35, 0.42, 0.65);

      // Map noise layers to colors
      vec3 col = darkBase;
      col = mix(col, deepBlue,   smoothstep(0.05, 0.35, cloud));
      col = mix(col, nebulaPurp, smoothstep(0.2,  0.55, cloud) * 0.5);
      col = mix(col, nebulaBlue, smoothstep(0.3,  0.7,  cloud) * 0.6);
      col = mix(col, brightCore, smoothstep(0.55, 0.95, cloud) * 0.35);

      // ── Wispy tendrils (sharper detail layer) ───────────────────────
      float wisp = fbm(p * 5.0 + vec2(t * 0.01, -t * 0.015), t * 0.3);
      wisp = smoothstep(0.25, 0.65, wisp) * cloud;
      col += vec3(0.10, 0.14, 0.25) * wisp * 0.3;

      // ── Stars (proper round dots) ──────────────────────────────────
      float s = 0.0;
      s += stars(uv, 90.0) * 0.5;        // fine stars
      s += stars(uv + 0.37, 45.0) * 0.7;  // medium stars
      s += stars(uv + 0.71, 25.0) * 1.0;  // sparse bright stars
      // Dim stars behind nebula clouds
      col += vec3(0.75, 0.80, 1.0) * s * (1.0 - cloud * 0.6);

      // ── Vignette (darken edges) ─────────────────────────────────────
      float vig = 1.0 - length((uv - 0.5) * 1.3);
      vig = smoothstep(0.0, 0.65, vig);
      col *= vig;

      // ── Final tonemap ──────────────────────────────────────────────
      col = pow(col, vec3(0.95));
      col *= 1.05;

      gl_FragColor = vec4(col, 1.0);
    }
  `;

  const FRAG_SRC_LIGHT = `
    precision highp float;
    uniform vec2  u_resolution;
    uniform float u_time;
    uniform float u_seed;

    vec3 mod289(vec3 x) { return x - floor(x * (1.0/289.0)) * 289.0; }
    vec2 mod289(vec2 x) { return x - floor(x * (1.0/289.0)) * 289.0; }
    vec3 permute(vec3 x) { return mod289(((x*34.0)+1.0)*x); }

    float snoise(vec2 v) {
      const vec4 C = vec4(0.211324865405187, 0.366025403784439,
                         -0.577350269189626, 0.024390243902439);
      vec2 i  = floor(v + dot(v, C.yy));
      vec2 x0 = v - i + dot(i, C.xx);
      vec2 i1 = (x0.x > x0.y) ? vec2(1.0, 0.0) : vec2(0.0, 1.0);
      vec4 x12 = x0.xyxy + C.xxzz;
      x12.xy -= i1;
      i = mod289(i);
      vec3 p = permute(permute(i.y + vec3(0.0, i1.y, 1.0))
                                   + i.x + vec3(0.0, i1.x, 1.0));
      vec3 m = max(0.5 - vec3(dot(x0,x0), dot(x12.xy,x12.xy),
                               dot(x12.zw,x12.zw)), 0.0);
      m = m*m; m = m*m;
      vec3 x_ = 2.0 * fract(p * C.www) - 1.0;
      vec3 h  = abs(x_) - 0.5;
      vec3 ox = floor(x_ + 0.5);
      vec3 a0 = x_ - ox;
      m *= 1.79284291400159 - 0.85373472095314 * (a0*a0 + h*h);
      vec3 g;
      g.x = a0.x * x0.x + h.x * x0.y;
      g.yz = a0.yz * x12.xz + h.yz * x12.yw;
      return 130.0 * dot(m, g);
    }

    float fbm(vec2 p, float t) {
      float val = 0.0;
      float amp = 0.5;
      float freq = 1.0;
      vec2 drift = vec2(t * 0.012, t * 0.007);
      for (int i = 0; i < 6; i++) {
        val += amp * snoise(p * freq + drift);
        drift *= 1.2;
        freq *= 2.0;
        amp  *= 0.5;
      }
      return val;
    }

    void main() {
      vec2 uv = gl_FragCoord.xy / u_resolution;
      float aspect = u_resolution.x / u_resolution.y;
      vec2 p = vec2(uv.x * aspect, uv.y);
      float t = u_time + u_seed;

      // Aurora cloud layers
      float n1 = fbm(p * 1.6 + vec2(0.0, 0.0), t);
      float n2 = fbm(p * 2.2 + vec2(4.3, 1.1), t * 0.6);
      float n3 = fbm(p * 3.0 + vec2(7.5, 3.2), t * 0.4);

      float cloud = 0.0;
      cloud += smoothstep(-0.15, 0.55, n1) * 0.40;
      cloud += smoothstep(0.0,   0.45, n2) * 0.22;
      cloud += smoothstep(0.1,   0.65, n3) * 0.16;
      cloud = clamp(cloud, 0.0, 1.0);

      // Aurora color palette — light base with chromatic veins
      vec3 lightBase    = vec3(0.940, 0.950, 0.972);
      vec3 auroraBlue   = vec3(0.700, 0.860, 0.980);
      vec3 auroraTeal   = vec3(0.000, 0.780, 0.690);
      vec3 auroraViolet = vec3(0.650, 0.550, 0.980);
      vec3 brightWhite  = vec3(0.980, 0.990, 1.000);

      vec3 col = lightBase;
      col = mix(col, auroraBlue,   smoothstep(0.05, 0.35, cloud));
      col = mix(col, auroraTeal,   smoothstep(0.25, 0.60, cloud) * 0.45);
      col = mix(col, auroraViolet, smoothstep(0.35, 0.75, cloud) * 0.35);
      col = mix(col, brightWhite,  smoothstep(0.60, 0.95, cloud) * 0.30);

      // Wispy tendrils
      float wisp = fbm(p * 4.5 + vec2(t * 0.008, -t * 0.012), t * 0.25);
      wisp = smoothstep(0.20, 0.60, wisp) * cloud;
      col += vec3(0.15, 0.25, 0.45) * wisp * 0.18;

      // Subtle sparkles (replace dark stars)
      vec2 cell = floor(uv * 60.0);
      float h = fract(sin(dot(cell, vec2(127.1, 311.7))) * 43758.5);
      float sparkle = 0.0;
      if (h > 0.97) {
        vec2 sub = fract(uv * 60.0) - 0.5;
        float dist = length(sub);
        sparkle = smoothstep(0.06, 0.0, dist) * 0.4;
        sparkle *= 0.7 + 0.3 * sin(u_time * (1.0 + h * 3.0));
      }
      col += vec3(1.0) * sparkle;

      // Soft vignette — fade edges toward white
      float vig = length((uv - 0.5) * 1.2);
      col = mix(col, lightBase, smoothstep(0.3, 0.75, vig) * 0.4);

      // Compress shadows, lift exposure for light feel
      col = 1.0 - (1.0 - col) * 0.72;
      col = pow(col, vec3(0.92));

      gl_FragColor = vec4(clamp(col, 0.0, 1.0), 1.0);
    }
  `;

  function buildProgram(gl, fragSrc) {
    function compile(type, src) {
      const s = gl.createShader(type);
      gl.shaderSource(s, src);
      gl.compileShader(s);
      if (!gl.getShaderParameter(s, gl.COMPILE_STATUS)) {
        console.error('[nebula] shader error:', gl.getShaderInfoLog(s));
        gl.deleteShader(s);
        return null;
      }
      return s;
    }
    const vs = compile(gl.VERTEX_SHADER, VERT_SRC);
    const fs = compile(gl.FRAGMENT_SHADER, fragSrc);
    if (!vs || !fs) return null;
    const prog = gl.createProgram();
    gl.attachShader(prog, vs);
    gl.attachShader(prog, fs);
    gl.linkProgram(prog);
    if (!gl.getProgramParameter(prog, gl.LINK_STATUS)) {
      console.error('[nebula] link error:', gl.getProgramInfoLog(prog));
      return null;
    }
    gl.deleteShader(vs);
    gl.deleteShader(fs);
    return prog;
  }

  function initNebula() {
    const container = document.querySelector('.bg-fluid');
    if (!container) return;

    // Create canvas
    const canvas = document.createElement('canvas');
    canvas.id = 'nebula-canvas';
    canvas.style.cssText = 'position:absolute;inset:0;width:100%;height:100%;';
    container.appendChild(canvas);

    const gl = canvas.getContext('webgl', {
      alpha: false,
      antialias: false,
      preserveDrawingBuffer: false,
    });
    if (!gl) {
      console.warn('[nebula] WebGL unavailable, falling back to CSS');
      return;
    }

    // Build initial program (dark by default)
    let prog = buildProgram(gl, FRAG_SRC);
    if (!prog) return;

    // Uniform locations helper — called after each buildProgram
    function getUniforms(p) {
      return {
        uRes:  gl.getUniformLocation(p, 'u_resolution'),
        uTime: gl.getUniformLocation(p, 'u_time'),
        uSeed: gl.getUniformLocation(p, 'u_seed'),
      };
    }
    let uniforms = getUniforms(prog);

    gl.useProgram(prog);

    // Full-screen quad
    const buf = gl.createBuffer();
    gl.bindBuffer(gl.ARRAY_BUFFER, buf);
    gl.bufferData(gl.ARRAY_BUFFER, new Float32Array([
      -1, -1, 1, -1, -1, 1, 1, 1
    ]), gl.STATIC_DRAW);

    const aPos = gl.getAttribLocation(prog, 'a_pos');
    gl.enableVertexAttribArray(aPos);
    gl.vertexAttribPointer(aPos, 2, gl.FLOAT, false, 0, 0);

    const seed = Math.random() * 1000;
    gl.uniform1f(uniforms.uSeed, seed);

    // Resize handling
    let width = 0, height = 0;
    const dpr = Math.min(window.devicePixelRatio || 1, 1.5); // cap at 1.5x for perf

    function resize() {
      const w = Math.floor(container.clientWidth * dpr);
      const h = Math.floor(container.clientHeight * dpr);
      if (w !== width || h !== height) {
        width = w; height = h;
        canvas.width = w;
        canvas.height = h;
        gl.viewport(0, 0, w, h);
        gl.uniform2f(uniforms.uRes, w, h);
      }
    }

    window.addEventListener('resize', resize);
    resize();

    // Animation loop
    const startTime = performance.now();
    let animId;

    // Referências às feTurbulences de displacement (IDs definidos no SVG)
    const disp = [
      document.getElementById('disp-turb-panel'),
      document.getElementById('disp-turb-btn'),
      document.getElementById('disp-turb-pill'),
    ];
    let lastDispUpdate = 0;

    function frame() {
      const elapsed = (performance.now() - startTime) / 1000.0;
      gl.uniform1f(uniforms.uTime, elapsed);
      gl.drawArrays(gl.TRIANGLE_STRIP, 0, 4);

      // Animar displacement turbulences a ~30fps (não precisa de 60fps)
      if (elapsed - lastDispUpdate > 0.033) {
        lastDispUpdate = elapsed;
        const fx = 0.008 + Math.sin(elapsed * 0.6) * 0.004;
        const fy = 0.008 + Math.sin(elapsed * 0.6 + 1.2) * 0.004;
        const freqStr = `${fx.toFixed(4)} ${fy.toFixed(4)}`;
        disp[0]?.setAttribute('baseFrequency', freqStr);
        disp[1]?.setAttribute('baseFrequency', freqStr);
        disp[2]?.setAttribute('baseFrequency', freqStr);
      }

      animId = requestAnimationFrame(frame);
    }

    frame();

    // Expose theme control
    window.setNebulaTheme = function(theme) {
      const src = theme === 'light' ? FRAG_SRC_LIGHT : FRAG_SRC;
      const newProg = buildProgram(gl, src);
      if (!newProg) return;
      gl.deleteProgram(prog);
      prog = newProg;
      uniforms = getUniforms(prog);
      // Re-bind vertex attribute for the new program
      gl.useProgram(prog);
      const newAPos = gl.getAttribLocation(prog, 'a_pos');
      gl.enableVertexAttribArray(newAPos);
      gl.vertexAttribPointer(newAPos, 2, gl.FLOAT, false, 0, 0);
      // Restore uniforms that are set once (not every frame)
      gl.uniform1f(uniforms.uSeed, seed);
      gl.uniform2f(uniforms.uRes, width, height);
    };

    // Expose opacity control for settings panel
    window.setNebulaOpacity = function(v) {
      canvas.style.opacity = Math.max(0, Math.min(1, v));
    };

    // Cleanup on page unload
    window.addEventListener('beforeunload', () => {
      cancelAnimationFrame(animId);
      gl.deleteProgram(prog);
    });
  }

  // Init when DOM is ready
  if (document.readyState === 'loading') {
    document.addEventListener('DOMContentLoaded', initNebula);
  } else {
    initNebula();
  }
})();
