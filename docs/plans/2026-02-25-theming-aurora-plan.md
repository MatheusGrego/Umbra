# Umbra Theming System — Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Corrigir o bug de opacidade do glass, implementar sistema de temas dark/light com toggle persistido, e criar o Aurora Light Theme com novo shader WebGL.

**Architecture:** Dois novos tokens CSS (`--glass-tint`, `--glass-rim-inset`) propagam os valores hardcoded do liquid glass, permitindo que o bloco `[data-theme="light"]` os sobrescreva. O nebula.js ganha um segundo fragment shader (Aurora) e uma API `window.setNebulaTheme()`. O `SettingsUI` em chat.js gerencia o toggle e persiste em localStorage.

**Tech Stack:** Go 1.21+, Wails v2, vanilla JS (ES modules), WebGL (GLSL), CSS custom properties

---

## Notas gerais

- Todos os arquivos estão em `frontend/`
- Rodar `go build ./...` na raiz de `client/` para verificar que nada quebrou no Go
- Não há testes automatizados para CSS/visual — verificação é `wails dev` + inspeção manual
- Cada task tem seu próprio commit
- O projeto NÃO usa bundler — os arquivos são servidos diretamente pelo Wails

---

## Task 1: Extrair --glass-tint e --glass-rim-inset em style.css

**Arquivos:**
- Modificar: `frontend/style.css`

**Contexto:** Os pseudo-elementos `::before` dos componentes glass usam cores hardcoded. Isso impede que o light theme e o slider de opacidade funcionem. Vamos extrair as duas cores mais críticas para tokens CSS.

**Step 1: Adicionar os dois novos tokens ao bloco `:root`**

Localizar o bloco `:root` (início do arquivo). Após a linha `--glass-divider: rgba(255, 255, 255, 0.08);`, adicionar:

```css
  --glass-tint:       rgba(12, 12, 24, 0.18);   /* tint do ::before nos painéis */
  --glass-rim-inset:  rgba(255, 255, 255, 0.55); /* highlight de borda inset */
```

**Step 2: Propagar --glass-tint nos ::before de glass-panel e glass-card**

Localizar `.glass-panel::before` e trocar:
```css
/* ANTES */
  background-color: rgba(12, 12, 24, 0.18);

/* DEPOIS */
  background-color: var(--glass-tint);
```

Localizar `.glass-card::before` e trocar:
```css
/* ANTES */
  background-color: rgba(12, 12, 24, 0.15);

/* DEPOIS */
  background-color: var(--glass-tint);
```

**Step 3: Propagar --glass-rim-inset nos box-shadow principais**

Localizar `.glass-panel::before` box-shadow e trocar a primeira cor (`rgba(255, 255, 255, 0.55)`):
```css
/* ANTES */
  box-shadow:
    inset 2px 2px 0px -1px rgba(255, 255, 255, 0.55),
    inset 0 0 3px 1px rgba(255, 255, 255, 0.12),
    inset -1px -1px 0px -1px rgba(255, 255, 255, 0.08);

/* DEPOIS */
  box-shadow:
    inset 2px 2px 0px -1px var(--glass-rim-inset),
    inset 0 0 3px 1px rgba(255, 255, 255, 0.12),
    inset -1px -1px 0px -1px rgba(255, 255, 255, 0.08);
```

Localizar `.glass-card::before` box-shadow e trocar a primeira cor:
```css
/* ANTES */
  box-shadow:
    inset 1.5px 1.5px 0px -1px rgba(255, 255, 255, 0.45),
    inset 0 0 2px 1px rgba(255, 255, 255, 0.07);

/* DEPOIS */
  box-shadow:
    inset 1.5px 1.5px 0px -1px var(--glass-rim-inset),
    inset 0 0 2px 1px rgba(255, 255, 255, 0.07);
```

Localizar `.msg.mine .msg-bubble::before` e trocar:
```css
/* ANTES */
  box-shadow:
    inset 1.5px 1.5px 0px -1px rgba(255, 255, 255, 0.45),
    inset 0 0 2px rgba(255, 255, 255, 0.08);

/* DEPOIS */
  box-shadow:
    inset 1.5px 1.5px 0px -1px var(--glass-rim-inset),
    inset 0 0 2px rgba(255, 255, 255, 0.08);
```

Localizar `#send-btn::before` e trocar:
```css
/* ANTES */
  box-shadow: inset 1.5px 1.5px 0px -1px rgba(255, 255, 255, 0.50);

/* DEPOIS */
  box-shadow: inset 1.5px 1.5px 0px -1px var(--glass-rim-inset);
```

**Step 4: Verificar que o dark theme ainda parece idêntico**

```
wails dev
```

O app deve aparecer exatamente igual ao estado anterior. Se algo mudou de cor, revisar os steps acima.

**Step 5: Commit**

```bash
git add frontend/style.css
git commit -m "refactor(css): extract --glass-tint and --glass-rim-inset tokens from hardcoded values"
```

---

## Task 2: Adicionar bloco [data-theme="light"] ao style.css

**Arquivos:**
- Modificar: `frontend/style.css`

**Contexto:** Toda a troca de tema acontece sobrescrevendo tokens no bloco `[data-theme="light"]`. Nunca duplicamos regras — apenas redefinimos os tokens que mudam.

**Step 1: Adicionar `data-theme="dark"` ao `<html>` em index.html**

Localizar a tag `<html lang="en">` e trocar por:
```html
<html lang="en" data-theme="dark">
```

**Step 2: Adicionar o bloco de tokens light ao final de style.css**

Adicionar antes da linha `/* ── Settings slider ──` (ou no final do arquivo antes de `/* ── Utility`):

```css
/* ══════════════════════════════════════════════════════════════════════
   LIGHT THEME — Aurora
   Paleta: branco azulado base, veios teal/lavanda/violeta (nebula.js)
   Ativa com: document.documentElement.dataset.theme = 'light'
   ══════════════════════════════════════════════════════════════════════ */
[data-theme="light"] {
  --void:    #eef0f8;
  --deep:    #f4f5fb;
  --surface: #ffffff;

  --glass-bg:       rgba(255, 255, 255, 0.55);
  --glass-bg-hover: rgba(255, 255, 255, 0.72);
  --glass-tint:     rgba(255, 255, 255, 0.22);
  --glass-rim:      rgba(0, 0, 0, 0.10);
  --glass-rim-s:    rgba(0, 0, 0, 0.06);
  --glass-rim-inset: rgba(255, 255, 255, 0.95);
  --glass-divider:  rgba(0, 0, 0, 0.07);

  --t100: rgba(10,  12,  30,  1.00);
  --t70:  rgba(30,  35,  70,  0.80);
  --t40:  rgba(60,  70,  120, 0.55);
  --t20:  rgba(100, 110, 160, 0.35);

  --accent:      rgba(80, 100, 220, 0.90);
  --accent-dim:  rgba(80, 100, 220, 0.35);
  --accent-glow: 0 0 18px rgba(80, 100, 220, 0.20);

  --online:  #059669;
  --offline: rgba(100, 110, 160, 0.25);
}
```

**Step 3: Testar os tokens básicos**

Em `wails dev`, abrir o DevTools e rodar no console:
```js
document.documentElement.dataset.theme = 'light'
```

A sidebar, o fundo e os textos devem mudar imediatamente para tons claros. O glass ainda pode parecer estranho (vamos corrigir overrides na próxima task), mas a estrutura deve estar lá.

**Step 4: Commit**

```bash
git add frontend/style.css frontend/index.html
git commit -m "feat(css): add [data-theme=light] Aurora token block"
```

---

## Task 3: CSS overrides para elementos hardcoded no light theme

**Arquivos:**
- Modificar: `frontend/style.css`

**Contexto:** Vários elementos usam cores hardcoded (não tokens) nos seus estados hover, active e background. No light theme essas cores brancas ficariam invisíveis ou feias. Adicionamos overrides dentro do bloco `[data-theme="light"]`.

**Step 1: Adicionar overrides de peer list**

Dentro do bloco `[data-theme="light"]` já criado, adicionar:

```css
/* Peer list */
[data-theme="light"] .peer-item:hover {
  background: rgba(0, 0, 0, 0.04);
}
[data-theme="light"] .peer-item.active {
  background: rgba(80, 100, 220, 0.08);
  outline-color: var(--accent-dim);
}
[data-theme="light"] .peer-avatar {
  background: rgba(80, 100, 220, 0.08);
  border-color: rgba(0, 0, 0, 0.10);
}
[data-theme="light"] .peer-avatar-lg {
  background: rgba(80, 100, 220, 0.08);
  border-color: rgba(0, 0, 0, 0.10);
}
```

**Step 2: Adicionar overrides de message bubbles**

```css
/* Bubbles */
[data-theme="light"] .msg.mine .msg-bubble {
  background: rgba(80, 100, 220, 0.10);
  border-color: rgba(80, 100, 220, 0.22);
  color: var(--t100);
}
[data-theme="light"] .msg.theirs .msg-bubble {
  background: rgba(0, 0, 0, 0.04);
  border-color: rgba(0, 0, 0, 0.08);
  color: var(--t70);
}
```

**Step 3: Adicionar overrides de input e send button**

```css
/* Input */
[data-theme="light"] #message-input {
  background: rgba(0, 0, 0, 0.03);
  border-color: rgba(0, 0, 0, 0.10);
  color: var(--t100);
}
[data-theme="light"] #message-input:focus {
  border-color: var(--accent-dim);
  background: rgba(80, 100, 220, 0.05);
}
[data-theme="light"] #message-input::placeholder {
  color: var(--t20);
}

/* Send button */
[data-theme="light"] #send-btn {
  background: rgba(80, 100, 220, 0.10);
  border-color: rgba(80, 100, 220, 0.28);
}
[data-theme="light"] #send-btn:hover {
  background: rgba(80, 100, 220, 0.18);
  border-color: rgba(80, 100, 220, 0.45);
}
```

**Step 4: Adicionar overrides de botões de ação**

```css
/* Header action buttons */
[data-theme="light"] .header-action-btn {
  background: rgba(0, 0, 0, 0.02);
  border-color: rgba(0, 0, 0, 0.08);
}
[data-theme="light"] .header-action-btn:hover {
  background: rgba(80, 100, 220, 0.06);
  border-color: var(--accent-dim);
}

/* Sidebar footer buttons */
[data-theme="light"] .sidebar-footer-btn {
  border-color: rgba(0, 0, 0, 0.08);
}
[data-theme="light"] .sidebar-footer-btn:hover {
  background: rgba(80, 100, 220, 0.04);
  border-color: var(--accent-dim);
}

/* Icon buttons */
[data-theme="light"] .icon-btn {
  background: rgba(0, 0, 0, 0.02);
  border-color: rgba(0, 0, 0, 0.08);
}
[data-theme="light"] .icon-btn:hover {
  background: rgba(80, 100, 220, 0.06);
  border-color: var(--accent-dim);
}
```

**Step 5: Adicionar overrides de status dot e misc**

```css
/* Status dot online */
[data-theme="light"] .status-dot.online {
  box-shadow: 0 0 6px #059669;
}

/* Identity card */
[data-theme="light"] .identity-card {
  background: rgba(255, 255, 255, 0.60);
  border-color: rgba(0, 0, 0, 0.08);
}

/* Modal box */
[data-theme="light"] #modal-box {
  background: rgba(255, 255, 255, 0.80);
  border-color: rgba(0, 0, 0, 0.10);
}

/* Scrollbars */
[data-theme="light"] ::-webkit-scrollbar-thumb {
  background: rgba(0, 0, 0, 0.15);
}
```

**Step 6: Verificar visualmente**

```
wails dev
```

Testar no DevTools:
```js
document.documentElement.dataset.theme = 'light'
// Verificar sidebar, chat, modais, bubbles
document.documentElement.dataset.theme = 'dark'
// Verificar que o dark ainda está correto
```

**Step 7: Commit**

```bash
git add frontend/style.css
git commit -m "feat(css): light theme overrides for hardcoded element colors"
```

---

## Task 4: Aurora Light Shader em nebula.js + setNebulaTheme()

**Arquivos:**
- Modificar: `frontend/nebula.js`

**Contexto:** O nebula.js usa um IIFE com um único fragment shader hardcoded dark. Vamos adicionar um segundo shader Aurora (light) e uma função `window.setNebulaTheme()` que recompila o GL program em runtime sem interromper o loop de animação.

**Step 1: Adicionar a constante FRAG_SRC_LIGHT após FRAG_SRC**

Localizar a linha `const FRAG_SRC = \`` no início do IIFE e, após o fechamento da template string (antes de `function initNebula()`), adicionar:

```js
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
```

**Step 2: Refatorar initNebula para extrair a compilação do program**

Localizar o bloco de compilação do shader dentro de `initNebula` (onde `gl.createShader`, `gl.shaderSource`, `gl.linkProgram` são chamados) e extraí-lo para uma função interna `buildProgram(fragSrc)`:

```js
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
```

Adicionar essa função dentro do IIFE, antes de `initNebula`.

**Step 3: Refatorar initNebula para usar buildProgram e expor setNebulaTheme**

Dentro de `initNebula`, substituir o bloco de compilação do programa por uma chamada a `buildProgram`. Em seguida, após `frame()`, implementar `setNebulaTheme`:

```js
    // Build initial program (dark by default)
    let prog = buildProgram(gl, FRAG_SRC);
    if (!prog) return;

    // Uniform locations helper — chamado após cada buildProgram
    function getUniforms(p) {
      return {
        uRes:  gl.getUniformLocation(p, 'u_resolution'),
        uTime: gl.getUniformLocation(p, 'u_time'),
        uSeed: gl.getUniformLocation(p, 'u_seed'),
      };
    }
    let uniforms = getUniforms(prog);

    // Atualizar as referências de uniform no loop
    // (substituir as referências diretas a uRes/uTime/uSeed por uniforms.*)
```

No loop `frame()`, substituir as referências diretas `uRes`, `uTime`, `uSeed` por `uniforms.uRes`, `uniforms.uTime`, `uniforms.uSeed`.

Após `frame()`, adicionar:

```js
    // Expose theme control
    window.setNebulaTheme = function(theme) {
      const src = theme === 'light' ? FRAG_SRC_LIGHT : FRAG_SRC;
      const newProg = buildProgram(gl, src);
      if (!newProg) return;
      gl.deleteProgram(prog);
      prog = newProg;
      uniforms = getUniforms(prog);
      // Re-bind vertex attribute para o novo programa
      gl.useProgram(prog);
      const aPos = gl.getAttribLocation(prog, 'a_pos');
      gl.enableVertexAttribArray(aPos);
      gl.vertexAttribPointer(aPos, 2, gl.FLOAT, false, 0, 0);
    };
```

**Step 4: Verificar que o shader dark ainda funciona**

```
wails dev
```

O nebula dark deve aparecer idêntico. Testar no console:
```js
setNebulaTheme('light')
// Nebula deve mudar para Aurora (tons claros, veios teal/violeta)
setNebulaTheme('dark')
// Deve voltar ao dark
```

**Step 5: Commit**

```bash
git add frontend/nebula.js
git commit -m "feat(nebula): Aurora light shader + setNebulaTheme() hot-swap in runtime"
```

---

## Task 5: Theme toggle — HTML + SettingsUI + persistência

**Arquivos:**
- Modificar: `frontend/index.html`
- Modificar: `frontend/chat.js`

**Contexto:** Adicionamos um botão de toggle na sidebar e conectamos tudo no `SettingsUI`. O tema é persistido em localStorage junto com as outras settings.

**Step 1: Adicionar botão theme-toggle na sidebar (index.html)**

Localizar o `<div class="sidebar-footer">` e adicionar o botão ANTES do `settings-open-btn`:

```html
<button class="sidebar-footer-btn" id="theme-toggle-btn">
  <svg id="theme-icon" viewBox="0 0 14 14" fill="none">
    <!-- ícone lua (dark mode padrão) -->
    <path d="M7 2a5 5 0 1 0 5 5 3.5 3.5 0 0 1-5-5Z"
          stroke="currentColor" stroke-width="1.2" stroke-linejoin="round"/>
  </svg>
  <span id="theme-label">Light Mode</span>
</button>
```

**Step 2: Adicionar campo `theme` nos DEFAULTS do SettingsUI (chat.js)**

Localizar a linha:
```js
  static DEFAULTS = { distortion: 50, nebula: 85, glass: 50 };
```

Substituir por:
```js
  static DEFAULTS = { distortion: 50, nebula: 85, glass: 50, theme: 'dark' };
```

**Step 3: Adicionar método #applyTheme() ao SettingsUI**

Dentro da classe `SettingsUI`, adicionar após `#syncSliders()`:

```js
  #applyTheme(theme) {
    document.documentElement.dataset.theme = theme;
    window.setNebulaTheme?.(theme);
    const btn = document.getElementById('theme-toggle-btn');
    const icon = document.getElementById('theme-icon');
    const label = document.getElementById('theme-label');
    if (!btn) return;
    if (theme === 'light') {
      // Ícone sol
      icon.innerHTML = `<circle cx="7" cy="7" r="2.5" stroke="currentColor" stroke-width="1.2"/>
        <path d="M7 1v1.5M7 11.5V13M1 7h1.5M11.5 7H13M2.93 2.93l1.06 1.06M10.01 10.01l1.06 1.06M2.93 11.07l1.06-1.06M10.01 3.99l1.06-1.06"
              stroke="currentColor" stroke-width="1.2" stroke-linecap="round"/>`;
      label.textContent = 'Dark Mode';
    } else {
      // Ícone lua
      icon.innerHTML = `<path d="M7 2a5 5 0 1 0 5 5 3.5 3.5 0 0 1-5-5Z"
          stroke="currentColor" stroke-width="1.2" stroke-linejoin="round"/>`;
      label.textContent = 'Light Mode';
    }
  }
```

**Step 4: Chamar #applyTheme no constructor e no #apply**

No `constructor()` do `SettingsUI`, após `this.#apply(this.#current)`:
```js
    this.#applyTheme(this.#current.theme);
```

No método `#bindDOM()`, adicionar o bind do botão de toggle (antes do bind dos sliders):
```js
    document.getElementById('theme-toggle-btn')
      ?.addEventListener('click', () => {
        this.#current.theme = this.#current.theme === 'dark' ? 'light' : 'dark';
        this.#applyTheme(this.#current.theme);
        this.#save();
      });
```

**Step 5: Verificar fluxo completo**

```
wails dev
```

Checklist manual:
- [ ] App abre no dark theme (padrão)
- [ ] Clicar "Light Mode" → nebula muda para Aurora, cores mudam para light
- [ ] Botão muda para "Dark Mode" com ícone sol
- [ ] Clicar "Dark Mode" → volta para dark
- [ ] Fechar e reabrir o app → tema persistido
- [ ] Slider de opacidade do glass agora funciona nos dois temas
- [ ] Slider de distorção continua funcionando

**Step 6: Commit**

```bash
git add frontend/index.html frontend/chat.js
git commit -m "feat(frontend): theme toggle — dark/light with Aurora nebula, persisted in localStorage"
```

---

## Verificação Final

```bash
go build ./...
```
Esperado: sem erros.

```
wails dev
```

Checklist completo:
- [ ] Dark theme: visual idêntico ao estado anterior (nenhuma regressão)
- [ ] Light theme: fundo Aurora claro, veios teal/violeta visíveis
- [ ] Liquid glass: distorção visível nos dois temas
- [ ] Slider "Opacidade do Vidro": agora muda visualmente (bug corrigido)
- [ ] Slider "Distorção do Vidro": continua funcionando nos dois temas
- [ ] Slider "Intensidade do Nebula": continua funcionando nos dois temas
- [ ] Toggle persiste após fechar e reabrir
- [ ] Nenhuma regressão em chat, invite, capsule, voice

```bash
git log --oneline
```

Esperado: 5 commits limpos após o commit do design doc.
