# Umbra Client — UX & Bug-Fix Design
**Data:** 2026-02-25
**Status:** Aprovado

---

## Contexto

App de mensagens encriptadas E2E (tipo Discord) em Wails/Go + vanilla JS.
Distribuído como `.exe` para amigos via Radmin VPN (migração futura para VPS).
Problemas reportados: bugs de estado/mensagem + features de UX faltantes.

---

## Problemas identificados

### Bug 1 — "unknown peer" ao enviar (`chat.go:51`)

**Raiz:** O servidor emite `peer_online` para peers que existem no lado do servidor mas não no `peers.json` local (ex: após reinstalação ou perda de dados). O frontend adiciona o peer ao estado JS via esse evento, mas o Go não tem a session key derivada. Resultado: peer visível na UI, `chat.Send` falha com `unknown peer <id>`.

**Fix:** Expor `GetAllPeers() []PeerInfo` no App. O bootstrap JS passa a carregar peers do Go (fonte de verdade), não de eventos de presença. Peers sem session key ficam visíveis com indicador "sem chave" e envio bloqueado.

---

### Bug 2 — Peer aparece offline após invite

**Raiz:** `HandleInviteResult` adiciona o peer e emite `invite:accepted` com `online: false`. O servidor não emite `peer_online` automaticamente após o handshake de invite. Resultado: contato recém adicionado aparece offline mesmo estando conectado.

**Fix:** Em `presence.go`, após `HandleInviteResult` adicionar o peer com sucesso, emitir `presence:online` com o `UserID` do novo contato imediatamente.

---

### Bug 3 — Remetente não vê a própria mensagem

**Raiz:** Em `chat.js`, ao criar a mensagem de saída:
```js
const m = { from: myID, to, text, ts: Date.now() };
```
A função `appendMessage` usa `msg.mine ? msg.to : msg.from` para determinar o peer da conversa. Como `mine` está ausente (falsy), usa `msg.from = myID`, que nunca bate com o `activePeer`. A mensagem não é renderizada — mas o destinatário a recebe normalmente.

**Fix:** Adicionar `mine: true` na mensagem de saída:
```js
const m = { from: myID, to, text, ts: Date.now(), mine: true };
```

---

## Feature 1 — Nicknames locais

**Escopo:** 100% local, não trafega pela rede. Apenas o dono do cliente vê o apelido.

### Go
- Adicionar `Nickname string` na struct `Peer` (`crypto/peers.go`) — persiste em `peers.json`
- Adicionar `SetNickname(userID, nickname string) error` no `PeerStore`
- Expor `SetNickname(userID, nickname string) error` no `App`
- `PeerInfo` struct: `{ UserID, Nickname, HasSession bool }`

### Frontend
- `StateStore`: campo `nickname` por peer, carregado via `getAllPeers()`
- Exibição: sempre `peer.nickname || peer.id` (fallback pro ID)
- Avatar: primeira letra do nickname se definido, senão primeira letra do ID

### UX — edição
- Ícone de lápis (SVG, estilo consistente) no header do chat ao lado do nome
- Click no lápis: nome vira `<input>` inline com valor atual
- `Enter` ou `blur`: salva via `SetNickname` → atualiza store → re-renderiza sidebar + header
- `Escape`: cancela sem salvar

### Animação
- Campo inline aparece com `width: 0 → auto` + `opacity: 0 → 1` (~150ms)
- Ao salvar: texto retorna com `fade-in` (~100ms)

---

## Feature 2 — Tela de onboarding

**Quando aparece:** `getAllPeers()` retorna vazio na inicialização. Com ao menos 1 peer, vai direto para a interface normal.

### Estrutura (no `#main`, sidebar continua visível)
1. Logo SVG do brand (maior)
2. `"UMBRA"` + subtítulo `"CANAL ENCRIPTADO"`
3. Card "Seu endereço" — ID copiável com texto: *"Compartilhe este endereço com um amigo para que ele gere um invite."*
4. Botão primário — `"ENTRAR COM INVITE CODE"` → abre modal de resolve invite
5. Separador `"ou"`
6. Botão ghost — `"GERAR INVITE"` → abre modal de create invite

### Comportamento
- `invite:accepted` → tela dissolve, peer selecionado automaticamente, chat abre
- Não requer nenhuma rota ou state machine — é apenas uma view condicional no `#main`

### Animação
- Entrada em stagger: logo → card ID → botões, cada um com `fade-in + translateY(8px → 0)` com delay crescente (~80ms entre elementos)
- Saída ao adicionar primeiro peer: `fade-out` da tela antes da interface principal aparecer

---

## Feature 3 — Painel de configurações visuais

**Acesso:** Ícone de engrenagem no sidebar inferior, ao lado do botão "Enter invite code". Side panel deslizando da direita — mesmo padrão do `#capsule-panel`.

### Controles

| Slider | Alvo | Range | Default |
|--------|------|-------|---------|
| Distorção | `scale` dos 3 `<feDisplacementMap>` SVG | 0 – 60 | 30 / 18 / 12 (proporcionais) |
| Nebula | Opacidade do `#nebula-canvas` | 0 – 1 | 0.85 |
| Vidro | Alpha do `--glass-bg` | 0.1 – 0.9 | 0.45 |

### Persistência
- `localStorage` com chave `umbra:settings` (JSON)
- `applySettings()` chamado antes do primeiro frame no boot

### Implementação
- Distorção: `setAttribute('scale', v)` nos elementos `<feDisplacementMap>` dos 3 filtros SVG. Escala proporcional: `panel = v`, `btn = v * 0.6`, `pill = v * 0.4`
- Nebula: `nebula.js` expõe `setNebulaOpacity(v)` chamado pelo settings
- Vidro: `document.documentElement.style.setProperty('--glass-bg', `rgba(12,12,24,${v})`)`
- Botão "RESTAURAR PADRÕES" no rodapé do painel

### Animação do painel
- Abrir: `translateX(100% → 0)` com `ease-out` ~250ms
- Fechar: `translateX(0 → 100%)` com `ease-in` ~200ms

---

## Micro-animações globais

### Chat
- Bubble enviado: `slide-up + fade-in` (~180ms `ease-out`)
- Bubble recebido: mesmo efeito, origin da esquerda
- Scroll ao appender: `scroll-behavior: smooth`

### Sidebar / Peers
- Peer novo (`invite:accepted`): item aparece com `fade-in + scale(0.95 → 1)` (~200ms)
- Peer online: dot pulsa uma vez (`pulse` keyframe) antes de ficar estático verde
- Peer offline: dot faz `transition` suave para cinza (~300ms)

### Botões (todos)
- Hover: `translateY(-1px)` + leve brilho no glass rim
- Active/click: `scale(0.96)` ~100ms
- Botão envio (submit): `scale(0.9 → 1)` rápido ao submeter

---

## Arquivos afetados

| Arquivo | Mudança |
|---------|---------|
| `crypto/peers.go` | Campo `Nickname`, método `SetNickname`, struct `PeerInfo` |
| `app.go` | `GetAllPeers()`, `SetNickname()` expostos ao JS |
| `service/presence.go` | Emitir `presence:online` após `HandleInviteResult` |
| `frontend/chat.js` | Bug `mine: true`, bootstrap `getAllPeers`, nickname UX, onboarding, settings panel |
| `frontend/index.html` | Elemento settings panel, gear icon, onboarding view |
| `frontend/style.css` | Animações, estilos onboarding, settings panel, nickname input, peer sem session key |
| `frontend/nebula.js` | Expor `setNebulaOpacity(v)` |
