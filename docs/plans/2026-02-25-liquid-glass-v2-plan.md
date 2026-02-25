# Liquid Glass v2 — Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Substituir feTurbulence por feImage com o canvas do nebula vivo, adicionar grain de superfície, ajustar tint e rim glow para alinhar com a referência daftplug.

**Architecture:** Três filtros SVG em `index.html` reescritos com pipeline `feImage(#nebula-canvas) → feDisplacementMap + feTurbulence(grain) → feMerge`. CSS: glass-panel::before sem tint, glass-card::before com tint branco 10%. Slider "Tint do Vidro" controla `--glass-tint` via `#apply()` em `chat.js`.

**Tech Stack:** Go 1.21+, Wails v2, vanilla JS (ES modules), WebGL (GLSL), CSS custom properties, SVG filters (feTurbulence, feImage, feDisplacementMap, feColorMatrix, feMerge)

**Referência:** `liquid_reference/liquid.html` + `liquid_reference/style.css` na raiz do projeto.
**Design doc:** `docs/plans/2026-02-25-liquid-glass-v2-design.md`

---

## Notas gerais

- Todos os arquivos estão em `frontend/`
- Rodar `go build ./...` na raiz de `client/` para verificar que nada quebrou no Go
- Verificação visual com `wails dev` — não há testes automatizados para CSS/visual
- Cada task tem seu próprio commit
- O projeto NÃO usa bundler — arquivos servidos diretamente pelo Wails
- **Scale do displacement é ponto de partida** — ajustar visualmente; nebula escuro pode pedir 50–65 no glass-panel

---

## Task 1: Reescrever filtros SVG com feImage + grain pipeline

**Arquivos:**
- Modificar: `frontend/index.html`

**Contexto:** Os três filtros SVG (`glass-panel`, `glass-btn`, `glass-pill`) usam feTurbulence como fonte de displacement. Vamos substituir por feImage apontando para `#nebula-canvas` (o canvas WebGL do nebula, que existe em DOM quando esses filtros são usados). Adicionamos também grain de superfície via feTurbulence de alta frequência fundido com feMerge.

**Step 1: Localizar o bloco de filtros SVG em index.html**

Os três filtros estão num `<svg style="display:none">` no início do body. São eles:
```xml
<filter id="glass-panel" ...>...</filter>
<filter id="glass-btn" ...>...</filter>
<filter id="glass-pill" ...>...</filter>
```

**Step 2: Substituir `#glass-panel`**

Trocar o conteúdo do filter por:
```xml
<filter id="glass-panel" x="0%" y="0%" width="100%" height="100%">
  <!-- Displacement source: live nebula canvas -->
  <feImage href="#nebula-canvas" result="nebula-map"
           x="0%" y="0%" width="100%" height="100%"
           preserveAspectRatio="none"/>
  <feGaussianBlur in="nebula-map" stdDeviation="1" result="disp-blur"/>
  <feDisplacementMap in="SourceGraphic" in2="disp-blur" scale="65"
                     xChannelSelector="R" yChannelSelector="G" result="displaced"/>
  <!-- Surface grain overlay -->
  <feTurbulence type="fractalNoise" baseFrequency="0.65" numOctaves="3"
                seed="42" result="grain"/>
  <feColorMatrix in="grain" type="matrix"
                 values="0 0 0 0 1  0 0 0 0 1  0 0 0 0 1  0 0 0 0.04 0"
                 result="grain-overlay"/>
  <feMerge>
    <feMergeNode in="displaced"/>
    <feMergeNode in="grain-overlay"/>
  </feMerge>
</filter>
```

**Step 3: Substituir `#glass-btn`**

```xml
<filter id="glass-btn" x="0%" y="0%" width="100%" height="100%">
  <feImage href="#nebula-canvas" result="nebula-map"
           x="0%" y="0%" width="100%" height="100%"
           preserveAspectRatio="none"/>
  <feGaussianBlur in="nebula-map" stdDeviation="1" result="disp-blur"/>
  <feDisplacementMap in="SourceGraphic" in2="disp-blur" scale="35"
                     xChannelSelector="R" yChannelSelector="G" result="displaced"/>
  <feTurbulence type="fractalNoise" baseFrequency="0.65" numOctaves="3"
                seed="17" result="grain"/>
  <feColorMatrix in="grain" type="matrix"
                 values="0 0 0 0 1  0 0 0 0 1  0 0 0 0 1  0 0 0 0.04 0"
                 result="grain-overlay"/>
  <feMerge>
    <feMergeNode in="displaced"/>
    <feMergeNode in="grain-overlay"/>
  </feMerge>
</filter>
```

**Step 4: Substituir `#glass-pill`**

```xml
<filter id="glass-pill" x="0%" y="0%" width="100%" height="100%">
  <feImage href="#nebula-canvas" result="nebula-map"
           x="0%" y="0%" width="100%" height="100%"
           preserveAspectRatio="none"/>
  <feGaussianBlur in="nebula-map" stdDeviation="1" result="disp-blur"/>
  <feDisplacementMap in="SourceGraphic" in2="disp-blur" scale="18"
                     xChannelSelector="R" yChannelSelector="G" result="displaced"/>
  <feTurbulence type="fractalNoise" baseFrequency="0.65" numOctaves="3"
                seed="5" result="grain"/>
  <feColorMatrix in="grain" type="matrix"
                 values="0 0 0 0 1  0 0 0 0 1  0 0 0 0 1  0 0 0 0.04 0"
                 result="grain-overlay"/>
  <feMerge>
    <feMergeNode in="displaced"/>
    <feMergeNode in="grain-overlay"/>
  </feMerge>
</filter>
```

**Step 5: Verificar visualmente**

```
wails dev
```

O app deve abrir normalmente com o efeito glass funcionando. Verificar:
- Sidebar e painéis têm distorção visível que muda conforme o nebula anima
- Grain sutil visível na superfície dos painéis (olhar de perto)
- Se a distorção não animar (feImage estático), abrir DevTools e rodar:
  ```js
  // Forçar repaint para verificar se feImage atualiza
  document.querySelectorAll('.glass-panel, .glass-card').forEach(el => {
    el.style.transform = 'translateZ(0)';
  });
  ```
- Se scale=65 parecer agressivo demais, reduzir para 50; se fraco demais, subir para 77

**Step 6: Ajustar distorção do settings panel (chat.js)**

O `SettingsUI.#apply()` em `chat.js` atualmente ajusta o `scale` dos feDisplacementMap:
```js
const base = (distortion / 100) * 60; // 0-100 → 0-60
['glass-panel', 'glass-btn', 'glass-pill'].forEach((id, i) => {
  const factors = [1, 0.6, 0.4];
  const scale = base * factors[i];
  const dm = document.querySelector(`#${id} feDisplacementMap`);
  if (dm) dm.setAttribute('scale', scale.toFixed(1));
});
```

Este código ainda funciona — apenas os valores base mudaram. Verificar que o slider de distorção em Configurações ainda ajusta o scale visualmente.

**Step 7: Commit**

```bash
git add frontend/index.html
git commit -m "feat(glass): replace feTurbulence with live feImage(nebula) + surface grain pipeline"
```

---

## Task 2: Atualizar ::before — tint transparente no panel, branco nos cards

**Arquivos:**
- Modificar: `frontend/style.css`

**Contexto:** `glass-panel::before` tem `background-color: var(--glass-tint)` (escuro 18%). Com feImage+distorção forte, o tint escuro tapa o efeito. Vamos: remover o tint do panel (vidro puro), e definir tint branco 10% hardcoded no card (diferencia elementos aninhados do container). O token `--glass-tint` permanece mas seu default vira `transparent`.

**Step 1: Mudar default de --glass-tint no :root**

Localizar no `:root`:
```css
  --glass-tint:       rgba(12, 12, 24, 0.18);   /* tint do ::before nos painéis */
```

Trocar por:
```css
  --glass-tint:       transparent;               /* tint do ::before — controlado pelo slider */
```

**Step 2: glass-panel::before — manter var(--glass-tint)**

Não muda o seletor — já usa `var(--glass-tint)`. Com o default agora `transparent`, o panel fica sem tint por padrão. O slider vai settar o valor.

**Step 3: glass-card::before — trocar para branco 10%**

Localizar:
```css
.glass-card::before {
  ...
  background-color: var(--glass-tint);
```

Trocar por:
```css
.glass-card::before {
  ...
  background-color: rgba(255, 255, 255, 0.10);
```

**Step 4: Atualizar override de light theme**

O bloco `[data-theme="light"]` já sobrescreve `--glass-tint: rgba(255, 255, 255, 0.22)`. Como o glass-panel::before agora usa a variável e o glass-card::before usa hardcoded, remover o override de --glass-tint do bloco light (não é mais necessário para os ::before):

Localizar em `[data-theme="light"]`:
```css
  --glass-tint:     rgba(255, 255, 255, 0.22);
```

Remover esta linha. (O token --glass-tint ainda existe no :root como transparent; em light mode permanece transparent por padrão, controlado pelo slider.)

**Step 5: Verificar visualmente**

```
wails dev
```

Checklist:
- Sidebar deve ser mais "cristalina" — menos opaca que antes
- Cards (peer list items, bubbles) devem ter leve brilho branco sutil
- Trocar para light mode via botão e verificar que os painéis ainda ficam visíveis

**Step 6: Commit**

```bash
git add frontend/style.css
git commit -m "refactor(css): glass-panel tint transparent by default, glass-card white 10% tint"
```

---

## Task 3: Aumentar brilho do rim glow

**Arquivos:**
- Modificar: `frontend/style.css`

**Contexto:** O inner glow atual (`rgba(255,255,255,0.12)`) é muito sutil comparado à referência (`rgba(255,255,255,0.7)`). Com o fundo escuro do Umbra, usaremos valores intermediários: ~0.50 para panels, ~0.35 para cards/buttons.

**Step 1: glass-panel::before — subir inner glow**

Localizar:
```css
  box-shadow:
    inset 2px 2px 0px -1px var(--glass-rim-inset),
    inset 0 0 3px 1px rgba(255, 255, 255, 0.12),
    inset -1px -1px 0px -1px rgba(255, 255, 255, 0.08);
```

Trocar por:
```css
  box-shadow:
    inset 2px 2px 0px -1px var(--glass-rim-inset),
    inset 0 0 3px 1px rgba(255, 255, 255, 0.50),
    inset -1px -1px 0px -1px rgba(255, 255, 255, 0.08);
```

**Step 2: glass-card::before — subir inner glow**

Localizar:
```css
  box-shadow:
    inset 1.5px 1.5px 0px -1px var(--glass-rim-inset),
    inset 0 0 2px 1px rgba(255, 255, 255, 0.07);
```

Trocar por:
```css
  box-shadow:
    inset 1.5px 1.5px 0px -1px var(--glass-rim-inset),
    inset 0 0 2px 1px rgba(255, 255, 255, 0.35);
```

**Step 3: --glass-rim-inset — subir alpha no dark theme**

O token `--glass-rim-inset: rgba(255, 255, 255, 0.55)` controla o top-left highlight. Referência usa 0.7. Subir:

No `:root`:
```css
  --glass-rim-inset:  rgba(255, 255, 255, 0.70); /* highlight de borda inset */
```

No light theme, `--glass-rim-inset: rgba(255, 255, 255, 0.95)` — não muda.

**Step 4: Verificar visualmente**

```
wails dev
```

Os painéis devem ter brilho interno mais intenso, especialmente ao redor das bordas. Se parecer excessivo em light mode, reduzir `--glass-rim-inset` no `:root` para 0.60.

**Step 5: Commit**

```bash
git add frontend/style.css
git commit -m "feat(css): increase rim glow intensity to match liquid glass reference"
```

---

## Task 4: Atualizar slider "Tint do Vidro" em settings

**Arquivos:**
- Modificar: `frontend/index.html`
- Modificar: `frontend/chat.js`

**Contexto:** O slider "Opacidade do Vidro" atualmente seta `--glass-bg` com cor hardcoded dark. Vamos renomear para "Tint do Vidro", mudar o default para 0 (transparente), e fazer `#apply()` setar `--glass-tint` com a cor correta por tema.

**Step 1: Renomear label no index.html**

Localizar o label do slider de vidro (procurar por "Opacidade do Vidro" ou "glass"):
```html
Opacidade do Vidro
```
Trocar por:
```html
Tint do Vidro
```

**Step 2: Mudar DEFAULTS do SettingsUI em chat.js**

Localizar:
```js
static DEFAULTS = { distortion: 50, nebula: 85, glass: 50, theme: 'dark' };
```

Trocar por:
```js
static DEFAULTS = { distortion: 50, nebula: 85, glass: 0, theme: 'dark' };
```

**Step 3: Atualizar #apply() para controlar --glass-tint**

Localizar o bloco de glass opacity em `#apply()`:
```js
// Glass opacity
const glassAlpha = glass / 100 * 0.8 + 0.1; // 10-90% → 0.1-0.9 alpha
document.documentElement.style.setProperty(
  '--glass-bg', `rgba(12,12,24,${glassAlpha.toFixed(2)})`
);
```

Substituir por:
```js
// Glass tint — theme-aware, default 0 (transparent)
const tintAlpha = (glass / 100) * 0.45; // 0-100 → 0-0.45 alpha
const isDark = (document.documentElement.dataset.theme ?? 'dark') === 'dark';
const tintColor = isDark
  ? `rgba(0, 0, 0, ${tintAlpha.toFixed(2)})`
  : `rgba(255, 255, 255, ${tintAlpha.toFixed(2)})`;
document.documentElement.style.setProperty('--glass-tint', tintColor);
```

**Step 4: Verificar fluxo completo**

```
wails dev
```

Checklist:
- App abre com painéis cristalinos (tint 0 por padrão)
- Slider "Tint do Vidro" em Configurações → mover para direita escurece os painéis no dark mode
- Trocar para light mode → mover slider clareia ainda mais os painéis
- Resetar settings → volta para 0 (transparente)
- Fechar e reabrir → valor persistido

**Step 5: Commit**

```bash
git add frontend/index.html frontend/chat.js
git commit -m "feat(settings): rename glass opacity slider to tint, control --glass-tint token theme-aware"
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
- [ ] Dark theme: distorção dos painéis visível e animada (segue o nebula)
- [ ] Grain sutil visível na superfície dos painéis
- [ ] Rim glow mais brilhante nas bordas superiores dos panels e cards
- [ ] Cards (bubbles, peer list) têm tint branco leve diferenciando do fundo
- [ ] Slider "Tint do Vidro" controla opacidade do tint corretamente em ambos os temas
- [ ] Light theme: efeito Aurora com distorção visível, grain presente
- [ ] Slider de Distorção ainda funciona (ajusta feDisplacementMap scale)
- [ ] Slider de Nebula ainda funciona (opacidade do canvas)
- [ ] Nenhuma regressão em chat, invite, capsule, voice

```bash
git log --oneline
```

Esperado: 4 commits limpos.
