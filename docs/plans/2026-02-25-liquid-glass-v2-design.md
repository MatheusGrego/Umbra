# Liquid Glass v2 — Design Doc

**Data:** 2026-02-25
**Status:** Aprovado

---

## Objetivo

Elevar a qualidade visual do liquid glass do Umbra alinhando com a referência original (daftplug/lucasromerodb): usar o canvas do nebula como displacement map vivo, adicionar grain de superfície e ajustar rim glow.

---

## Decisão de arquitetura

**Abordagem A — feImage nativo com `#nebula-canvas`** *(adotada)*

Todos os filtros SVG substituem o `feTurbulence` por `feImage href="#nebula-canvas"`. O próprio browser captura o canvas ao recompositar o elemento. Como `backdrop-filter: blur(0px)` já força recomposição a cada frame do nebula, o feImage é atualizado automaticamente a 60fps — sem JS extra.

Pipeline de cada filtro:
```
feImage(#nebula-canvas) → feGaussianBlur(suaviza gradientes) → feDisplacementMap(desloca backdrop)
feTurbulence(grain alta-freq) → feColorMatrix(alpha ~4%) → feMerge(displaced + grain)
```

Scales por tamanho do elemento:
- `glass-panel`: 77 *(ponto de partida, ajustar visualmente — nebula escuro pode pedir 50–65)*
- `glass-btn`: 35
- `glass-pill`: 18

---

## Abordagens descartadas

**Abordagem B — feImage com refresh via JavaScript**
JS usa `requestAnimationFrame` + `canvas.toDataURL()` para atualizar o `href` do feImage a cada frame. Garante sincronização mas `toDataURL()` a 60fps copia todos os pixels CPU→GPU — custo desnecessário dado que o browser já faz isso nativamente na Abordagem A.

**Abordagem C — feTurbulence com parâmetros ajustados + grain**
Mantém feTurbulence, sobe o scale (30→77) e adiciona grain. Resolve o rim glow e o grain, mas o displacement não tem correlação com o nebula — descarta o efeito de "lente viva".

---

## Mudanças visuais

### 1. Filtros SVG — feImage + grain
- Todos os 3 filtros em `index.html` reescritos.
- Grain: `feTurbulence` alta frequência (`baseFrequency="0.65"`) com `feColorMatrix` alpha ~4% branco, fundido via `feMerge` sobre o backdrop deslocado.

### 2. `::before` tint — transparente no panel, branco nos cards
- **`glass-panel::before`**: remove `background-color` — vidro puro. O rim shadow + outline + distorção definem o painel.
- **`glass-card::before`**: troca `var(--glass-tint)` por `rgba(255,255,255,0.10)` hardcoded — tint branco leve que diferencia visualmente elementos aninhados do container. Igual em dark e light theme.
- **`--glass-tint`** permanece como token mas seu default vira `transparent`. O slider o controla.

### 3. Rim glow — mais brilhante
Referência usa `rgba(255,255,255,0.7)` no inner glow. Umbra tem fundo escuro, então usamos valores intermediários:
- `glass-panel::before`: inner glow `0.12` → `0.50`
- `glass-card::before`: inner glow `0.07` → `0.35`
- Outros `::before` (msg-bubble, send-btn, etc.): aumento proporcional ~3×

### 4. Slider "Tint do Vidro"
- Label renomeada: "Opacidade do Vidro" → "Tint do Vidro"
- `#apply()` em `chat.js` passa a controlar `--glass-tint` (não `--glass-bg`):
  - dark theme: `rgba(0, 0, 0, alpha)`
  - light theme: `rgba(255, 255, 255, alpha)`
  - Default: slider em 0 → alpha 0 (transparente)
  - Fórmula: `alpha = (glass / 100) * 0.45` → range 0–0.45
- `--glass-bg` permanece no CSS para elementos que o usam diretamente (onboarding-card, etc.)

---

## Sem mudanças

- Estrutura `::after` (backdrop-filter + filter URL) — não muda
- `isolation: isolate` nos elementos principais — não muda
- `outline: 1px solid var(--glass-rim)` — não muda
- Tokens de tema (`--glass-rim`, `--glass-divider`, etc.) — não mudam
- nebula.js — não muda (apenas `#nebula-canvas` é referenciado pelo feImage)

---

## Riscos e mitigações

| Risco | Mitigação |
|---|---|
| feImage não atualiza a 60fps | backdrop-filter já força recomposição; se não funcionar, JS requestAnimationFrame força repaint nos elementos com `will-change: filter` |
| Scale=77 agressivo demais no nebula escuro | Ajuste visual — ponto de partida 77, provavelmente 50–65 para dark theme |
| Grain visível demais | alpha do feColorMatrix ajustável, começar com 0.04 |
| Desempenho com 3 filtros feImage a 60fps | App desktop Wails — aceitável; monitorar se frames caírem |
