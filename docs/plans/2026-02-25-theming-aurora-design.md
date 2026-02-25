# Umbra Theming System — Design Document

> **For Claude:** REQUIRED SUB-SKILL: Use `superpowers:writing-plans` to turn this into an implementation plan.

**Goal:** Corrigir o bug de opacidade do glass, implementar sistema de temas (`dark`/`light`) com toggle persistido, e criar o Aurora Light Theme com novo shader WebGL.

**Futuros temas anotados (fora de escopo):** Dawn (warm, pêssego/índigo), Pearl (iridescente neutro/sem WebGL).

---

## 1. Diagnóstico do sistema atual

### 1.1 Design Tokens (`:root`)

Todos os tokens vivem em `frontend/style.css` no bloco `:root`:

```
Backgrounds
  --void:     #03030a   ← cor mais escura, fundo do body
  --deep:     #07070f
  --surface:  #0c0c18

Glass
  --glass-bg:       rgba(12, 12, 24, 0.45)   ← bg de cards/onboarding
  --glass-bg-hover: rgba(18, 18, 32, 0.58)
  --glass-rim:      rgba(255, 255, 255, 0.14) ← borda outline
  --glass-rim-s:    rgba(255, 255, 255, 0.08) ← borda suave
  --glass-divider:  rgba(255, 255, 255, 0.08) ← separadores

Tipografia (branco em 4 níveis de alpha)
  --t100: rgba(255, 255, 255, 1.00)
  --t70:  rgba(200, 210, 240, 0.80)
  --t40:  rgba(140, 150, 190, 0.55)
  --t20:  rgba(100, 110, 160, 0.35)

Accent (cold white-blue)
  --accent:      rgba(180, 200, 255, 0.85)
  --accent-dim:  rgba(140, 160, 220, 0.40)
  --accent-glow: 0 0 18px rgba(150, 170, 255, 0.25)

Status
  --online:  #58d68d
  --offline: rgba(140, 150, 190, 0.25)

Border radius
  --radius-panel: 20px
  --radius-card:  14px
  --radius-btn:   10px
  --radius-pill:  9999px

Typography
  --font-mono: 'JetBrains Mono', 'Fira Code', monospace
  --font-ui:   'Inter', -apple-system, system-ui, sans-serif
```

### 1.2 Técnica Liquid Glass

A técnica usada é a **daftplug displacement glass**:

- O elemento em si: sem `background`, sem `backdrop-filter`. Apenas `position` e `border-radius`.
- `::before` (`z-index: 1`): rim de inset box-shadow + tint de fundo. Cria o brilho de borda.
- `::after` (`z-index: -1`): `backdrop-filter: blur(0px)` + `filter: url(#glass-*)`. O blur é **propositalmente zero** — serve apenas para forçar GPU layer. O efeito visual vem do SVG `feDisplacementMap` com `feTurbulence`, que distorce os pixels do backdrop.

Três filtros SVG em `index.html`:
| ID | `baseFrequency` | `scale` padrão | Usado em |
|---|---|---|---|
| `glass-panel` | 0.008 | 30 | sidebar, header, input area, modais |
| `glass-btn` | 0.012 | 18 | botões, avatares, bubbles mine, send btn |
| `glass-pill` | 0.018 | 12 | identity card, bubbles theirs, toast |

### 1.3 Bug: glass opacity não funciona no settings panel

**Causa:** `SettingsUI.#apply()` muda `--glass-bg` via `setProperty`, mas os pseudo-elementos `::before` dos painéis principais usam cores **hardcoded** (`rgba(12,12,24, 0.18)`, etc.) em vez do token. Apenas `.onboarding-card` e `.identity-card` usam `var(--glass-bg)`.

**Também hardcoded:**
- `.glass-panel::before` → `rgba(12, 12, 24, 0.18)`
- `.glass-card::before` → `rgba(12, 12, 24, 0.15)`
- `.identity-card::before` → box-shadow rim hardcoded com branco
- Todos os `box-shadow inset` de rim usam `rgba(255,255,255, X)` fixo

**Fix:** Extrair dois novos tokens:
- `--glass-tint`: alpha do tint no `::before` de cada elemento
- `--glass-rim-inset`: cor do highlight de borda inset

### 1.4 Nebula WebGL (nebula.js)

Shader GLSL com paleta hardcoded dark-space:
```
darkBase    = vec3(0.012, 0.012, 0.028)
deepBlue    = vec3(0.04,  0.06,  0.16)
nebulaPurp  = vec3(0.08,  0.04,  0.16)
nebulaBlue  = vec3(0.10,  0.16,  0.32)
brightCore  = vec3(0.35,  0.42,  0.65)
```
FBM 6-octave, wispy tendrils, star field com twinkle, vignette.

Para o light theme: novo fragment shader com paleta Aurora injetado em runtime. `window.setNebulaTheme('dark'|'light')` recompila o GL program e continua o loop de animação sem recarregar a página.

---

## 2. Sistema de Temas

### 2.1 Mecanismo

```html
<html data-theme="dark">   <!-- padrão -->
<html data-theme="light">
```

CSS:
```css
:root { /* tokens dark — sem alteração */ }

[data-theme="light"] {
  /* sobrescreve apenas os tokens que mudam */
}
```

JS (`SettingsUI`):
- Adicionar campo `theme: 'dark' | 'light'` em `DEFAULTS` e `KEY`
- Toggle aplica `document.documentElement.dataset.theme = theme`
- Chama `window.setNebulaTheme(theme)`
- Persiste em `localStorage`

### 2.2 Tokens do Light Theme (Aurora)

| Token | Dark | Light |
|---|---|---|
| `--void` | `#03030a` | `#eef0f8` |
| `--deep` | `#07070f` | `#f4f5fb` |
| `--surface` | `#0c0c18` | `#ffffff` |
| `--glass-bg` | `rgba(12,12,24, 0.45)` | `rgba(255,255,255, 0.55)` |
| `--glass-bg-hover` | `rgba(18,18,32, 0.58)` | `rgba(255,255,255, 0.72)` |
| `--glass-tint` *(novo)* | `rgba(12,12,24, 0.18)` | `rgba(255,255,255, 0.22)` |
| `--glass-rim` | `rgba(255,255,255, 0.14)` | `rgba(0,0,0, 0.10)` |
| `--glass-rim-s` | `rgba(255,255,255, 0.08)` | `rgba(0,0,0, 0.06)` |
| `--glass-rim-inset` *(novo)* | `rgba(255,255,255, 0.55)` | `rgba(255,255,255, 0.95)` |
| `--glass-divider` | `rgba(255,255,255, 0.08)` | `rgba(0,0,0, 0.07)` |
| `--t100` | `rgba(255,255,255, 1)` | `rgba(10,12,30, 1)` |
| `--t70` | `rgba(200,210,240, 0.80)` | `rgba(30,35,70, 0.80)` |
| `--t40` | `rgba(140,150,190, 0.55)` | `rgba(60,70,120, 0.55)` |
| `--t20` | `rgba(100,110,160, 0.35)` | `rgba(100,110,160, 0.35)` |
| `--accent` | `rgba(180,200,255, 0.85)` | `rgba(80,100,220, 0.90)` |
| `--accent-dim` | `rgba(140,160,220, 0.40)` | `rgba(80,100,220, 0.35)` |
| `--accent-glow` | `0 0 18px rgba(150,170,255,0.25)` | `0 0 18px rgba(80,100,220,0.20)` |
| `--online` | `#58d68d` | `#059669` |
| `--offline` | `rgba(140,150,190, 0.25)` | `rgba(100,110,160, 0.25)` |

### 2.3 Ajustes adicionais de CSS para light

Elementos com cores hardcoded que precisam de override em `[data-theme="light"]`:

- `.peer-item:hover` → `rgba(0,0,0,0.04)` (era branco)
- `.peer-item.active` → `rgba(80,100,220,0.08)` + outline accent
- `.peer-avatar`, `.peer-avatar-lg` → `rgba(80,100,220,0.08)` bg
- `.msg.mine .msg-bubble` → `rgba(80,100,220,0.10)` bg + borda accent
- `.msg.theirs .msg-bubble` → `rgba(0,0,0,0.04)` bg
- `#message-input` → `rgba(0,0,0,0.03)` bg
- `#message-input:focus` → `rgba(80,100,220,0.06)` bg
- `#send-btn` → accent invertido
- `.header-action-btn:hover` → `rgba(80,100,220,0.06)`
- `.sidebar-footer-btn:hover` → `rgba(80,100,220,0.04)`
- `.status-dot.online` → `box-shadow: 0 0 6px #059669`
- `body` → `background: var(--void)` (já usa token, funciona)

### 2.4 Propagação do --glass-tint (bug fix)

Substituir todas as ocorrências de valores hardcoded nos `::before`:

```css
/* ANTES (hardcoded) */
.glass-panel::before  { background-color: rgba(12, 12, 24, 0.18); }
.glass-card::before   { background-color: rgba(12, 12, 24, 0.15); }

/* DEPOIS (token) */
.glass-panel::before  { background-color: var(--glass-tint); }
.glass-card::before   { background-color: var(--glass-tint); }
```

E os inset box-shadows de rim (branco hardcoded):
```css
/* ANTES */
box-shadow: inset 2px 2px 0px -1px rgba(255,255,255,0.55), ...

/* DEPOIS — usa --glass-rim-inset para a cor principal */
box-shadow: inset 2px 2px 0px -1px var(--glass-rim-inset), ...
```

---

## 3. Aurora Light Nebula

### 3.1 Paleta GLSL

```glsl
vec3 lightBase    = vec3(0.940, 0.950, 0.972); // branco azulado
vec3 auroraBlue   = vec3(0.700, 0.860, 0.980); // azul céu
vec3 auroraTeal   = vec3(0.000, 0.780, 0.690); // teal vívido
vec3 auroraViolet = vec3(0.650, 0.550, 0.980); // lavanda
vec3 brightWhite  = vec3(0.980, 0.990, 1.000); // pico
```

### 3.2 Diferenças do shader dark

| | Dark | Aurora Light |
|---|---|---|
| Base | `vec3(0.012, 0.012, 0.028)` | `vec3(0.940, 0.950, 0.972)` |
| Clouds | escuros sobre void | veios coloridos sobre branco |
| Stars | `stars()` com twinkle | removidas (substituídas por sparkles) |
| Vignette | escurece bordas | suaviza bordas (fade to white) |
| Exposição | `col *= 1.05` | `col = 1.0 - (1.0-col)*0.7` (comprime sombras) |

### 3.3 API JavaScript

```js
// nebula.js exporta:
window.setNebulaTheme = function(theme) {
  // recompila o GL program com o FRAG_SRC correto (dark ou light)
  // continua o animation loop sem interrupção visível
};
```

Chamado por `SettingsUI` durante o toggle.

---

## 4. Toggle de tema na UI

### 4.1 HTML (index.html)

Botão no `.sidebar-footer` antes do settings:

```html
<button class="sidebar-footer-btn" id="theme-toggle-btn">
  <!-- ícone sol (light) ou lua (dark) trocado por JS -->
  <svg id="theme-icon"> ... </svg>
  <span id="theme-label">Light Mode</span>
</button>
```

### 4.2 JS (SettingsUI)

```js
static DEFAULTS = { distortion: 50, nebula: 85, glass: 50, theme: 'dark' };

#applyTheme(theme) {
  document.documentElement.dataset.theme = theme;
  window.setNebulaTheme?.(theme);
  // atualiza ícone e label do botão
}
```

---

## 5. Arquitetura de Arquivos

| Arquivo | Mudança |
|---|---|
| `frontend/style.css` | Extrair `--glass-tint`, `--glass-rim-inset`; propagar tokens; adicionar bloco `[data-theme="light"]` |
| `frontend/nebula.js` | Adicionar `FRAG_SRC_LIGHT`, implementar `setNebulaTheme()` |
| `frontend/index.html` | Adicionar `data-theme="dark"` no `<html>`; botão theme toggle na sidebar |
| `frontend/chat.js` | `SettingsUI`: campo `theme`, `#applyTheme()`, bind do toggle btn |

---

## 6. Temas futuros (fora de escopo)

### Dawn Theme
- Backgrounds: `#faf5ef` → `#fff8f2`
- Accent: `rgba(220, 100, 60, 0.85)` (terracota/laranja)
- Nebula: FBM com rosa, pêssego, azul-índigo nas bordas, base creme
- Adequado para uso diurno quente

### Pearl Theme
- Backgrounds: `#f8f8fc` → `#ffffff`
- Accent: `rgba(130, 100, 200, 0.85)` (lilás)
- Sem nebula animada — gradiente CSS estático com shimmer iridescente
- Mais leve em GPU, mais minimalista
