# Umbra UX & Bug-Fix Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Corrigir 3 bugs críticos (send failure, offline falso, mensagem do remetente) e adicionar nicknames locais, tela de onboarding e painel de configurações visuais com micro-animações.

**Architecture:** Abordagem A — Go é fonte de verdade sobre quais peers existem. `GetAllPeers()` substitui `GetOnlinePeers()` no bootstrap. Eventos de presença controlam apenas status online/offline. Features de frontend puras (settings) usam localStorage sem tocar Go.

**Tech Stack:** Go 1.21+, Wails v2, vanilla JS (ES modules), WebGL (nebula.js), CSS custom properties, SVG filters

---

## Notas gerais

- Após cada mudança em `app.go`, rodar `wails dev` regenera automaticamente `frontend/wailsjs/go/main/App.js` e `App.d.ts` (não commitados — gerados em build time)
- Rodar Go: `go test ./...` na raiz do projeto
- Rodar app: `wails dev` na raiz do projeto
- Cada task tem seu próprio commit

---

## Task 1: PeerInfo + Nickname + GetAllPeers em crypto/peers.go

**Files:**
- Modify: `crypto/peers.go`
- Create: `crypto/peers_test.go`

**Step 1: Escrever o teste falhando**

Criar `crypto/peers_test.go`:

```go
package crypto_test

import (
	"os"
	"testing"

	"umbra/client/crypto"
)

func makeTempStore(t *testing.T) *crypto.PeerStore {
	t.Helper()
	dir := t.TempDir()
	s, err := crypto.NewPeerStore(dir)
	if err != nil {
		t.Fatalf("NewPeerStore: %v", err)
	}
	return s
}

func TestGetAllPeers_EmptyStore(t *testing.T) {
	s := makeTempStore(t)
	peers := s.GetAllPeers()
	if len(peers) != 0 {
		t.Errorf("expected 0 peers, got %d", len(peers))
	}
}

func TestSetNickname_UnknownPeer(t *testing.T) {
	s := makeTempStore(t)
	err := s.SetNickname("nonexistent", "John")
	if err == nil {
		t.Error("expected error for unknown peer, got nil")
	}
}
```

**Step 2: Rodar o teste para confirmar que falha**

```
go test ./crypto/... -v -run "TestGetAllPeers|TestSetNickname"
```
Esperado: FAIL — `s.GetAllPeers undefined`, `s.SetNickname undefined`

**Step 3: Implementar**

Em `crypto/peers.go`, adicionar `Nickname` à struct `Peer` e a struct `PeerInfo`:

```go
// Peer holds everything we know about a remote contact.
type Peer struct {
	UserID    string `json:"user_id"`
	EdPubKey  string `json:"ed_pub_key"`
	X25519Key string `json:"x25519_key"`
	Nickname  string `json:"nickname,omitempty"`
}

// PeerInfo is the read-only view returned to the UI layer.
type PeerInfo struct {
	UserID     string `json:"user_id"`
	Nickname   string `json:"nickname"`
	HasSession bool   `json:"has_session"`
}
```

Adicionar os dois métodos no `PeerStore` (após o método `All()`):

```go
// GetAllPeers returns a PeerInfo snapshot of all known peers.
func (s *PeerStore) GetAllPeers() []PeerInfo {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]PeerInfo, 0, len(s.peers))
	for uid, p := range s.peers {
		_, hasSession := s.sessions[uid]
		out = append(out, PeerInfo{
			UserID:     uid,
			Nickname:   p.Nickname,
			HasSession: hasSession,
		})
	}
	return out
}

// SetNickname assigns a local display name to a known peer and persists it.
func (s *PeerStore) SetNickname(userID, nickname string) error {
	s.mu.Lock()
	p, ok := s.peers[userID]
	if !ok {
		s.mu.Unlock()
		return fmt.Errorf("peers: unknown peer %s", userID)
	}
	p.Nickname = nickname
	s.peers[userID] = p
	s.mu.Unlock()
	return s.save()
}
```

**Step 4: Rodar os testes**

```
go test ./crypto/... -v -run "TestGetAllPeers|TestSetNickname"
```
Esperado: PASS

**Step 5: Commit**

```bash
git add crypto/peers.go crypto/peers_test.go
git commit -m "feat(crypto): add PeerInfo, Nickname field, GetAllPeers, SetNickname"
```

---

## Task 2: Expor GetAllPeers e SetNickname no app.go

**Files:**
- Modify: `app.go`

**Step 1: Adicionar os métodos ao App**

No final da seção `// ---- Helpers`, antes de `showError`, adicionar:

```go
// ---- Peers --------------------------------------------------------------

// GetAllPeers returns all known contacts with their nickname and session status.
// Used by the frontend to bootstrap the peer list.
func (a *App) GetAllPeers() []gc.PeerInfo {
	return a.presence.AllPeers()
}

// SetNickname assigns a local display name to a contact.
func (a *App) SetNickname(userID, nickname string) error {
	return a.presence.SetPeerNickname(userID, nickname)
}
```

Nota: precisamos adicionar os métodos delegadores no `PresenceService` também (próximo passo).

**Step 2: Adicionar delegadores no PresenceService**

Em `service/presence.go`, adicionar após `MyUserID()`:

```go
// AllPeers returns all known peers from the peer store.
func (s *PresenceService) AllPeers() []gc.PeerInfo {
	return s.peers.GetAllPeers()
}

// SetPeerNickname assigns a local nickname to a peer.
func (s *PresenceService) SetPeerNickname(userID, nickname string) error {
	return s.peers.SetNickname(userID, nickname)
}
```

Também adicionar o import correto no `app.go` — verificar se `gc "umbra/client/crypto"` já está presente (está, em `main.go`; em `app.go` não usa diretamente, então adicionar):

No topo de `app.go`, imports:
```go
import (
	"context"
	"log"

	"github.com/wailsapp/wails/v2/pkg/runtime"

	gc "umbra/client/crypto"
	"umbra/client/service"
)
```

**Step 3: Compilar**

```
go build ./...
```
Esperado: sem erros

**Step 4: Commit**

```bash
git add app.go service/presence.go
git commit -m "feat(app): expose GetAllPeers and SetNickname to Wails JS bridge"
```

---

## Task 3: Fix presence.go — peer online após invite

**Files:**
- Modify: `service/presence.go`

**Step 1: Editar HandleInviteResult**

Substituir o trecho final de `HandleInviteResult` (linha ~128-135):

```go
// ANTES:
	if err := s.peers.Add(peer, s.identity.X25519Private); err != nil {
		log.Printf("[presence] failed to add peer %s: %v", p.UserID, err)
		return
	}

	log.Printf("[presence] new contact added: %s", p.UserID)
	s.emitter.Emit("invite:accepted", peer)

// DEPOIS:
	if err := s.peers.Add(peer, s.identity.X25519Private); err != nil {
		log.Printf("[presence] failed to add peer %s: %v", p.UserID, err)
		return
	}

	// Mark the new peer as online — they're clearly connected right now.
	s.mu.Lock()
	s.online[p.UserID] = true
	s.mu.Unlock()

	log.Printf("[presence] new contact added: %s", p.UserID)
	s.emitter.Emit("invite:accepted", gc.PeerInfo{
		UserID:     peer.UserID,
		Nickname:   peer.Nickname,
		HasSession: true,
	})
	s.emitter.Emit("presence:online", p.UserID)
```

Nota: o evento `invite:accepted` agora emite `PeerInfo` (não `Peer`) para ser consistente com `GetAllPeers`. O frontend precisará ser atualizado para usar `peer.user_id` (já usa) e `peer.has_session`.

**Step 2: Compilar**

```
go build ./...
```
Esperado: sem erros

**Step 3: Commit**

```bash
git add service/presence.go
git commit -m "fix(presence): mark new peer as online immediately after invite handshake"
```

---

## Task 4: Fix frontend — mine:true + bootstrap GetAllPeers

**Files:**
- Modify: `frontend/chat.js`

**Step 1: Corrigir o bug mine:true**

Localizar em `chat.js` a linha ~444 (dentro de `this.#bus.on('msg:send', ...)`):

```js
// ANTES:
const m = { from: this.#store.getMyID(), to, text, ts: Date.now() };

// DEPOIS:
const m = { from: this.#store.getMyID(), to, text, ts: Date.now(), mine: true };
```

**Step 2: Adicionar getAllPeers ao WailsBridge**

Na classe `WailsBridge`, após `getOnlinePeers`:

```js
async getAllPeers() { return window.go?.main?.App?.GetAllPeers?.() ?? []; }
async setNickname(userID, nickname) { return window.go?.main?.App?.SetNickname?.(userID, nickname); }
```

**Step 3: Atualizar StateStore para suportar nickname e hasSession**

Na classe `StateStore`, o método `upsertPeer` já aceita patch genérico. Adicionar getter:

```js
// Dentro de StateStore, após getMessages:
getDisplayName(id) {
  const p = this.#peers.get(id);
  return (p?.nickname && p.nickname.trim()) ? p.nickname : id;
}
```

**Step 4: Substituir bootstrap — trocar getOnlinePeers por getAllPeers**

No método `#bootstrap()`, substituir:

```js
// ANTES:
const online = await this.#bridge.getOnlinePeers();
for (const id of (online || [])) this.#store.upsertPeer(id, { online: true });
this.#chatUI.renderPeerList();

// DEPOIS:
const allPeers = await this.#bridge.getAllPeers();
for (const p of (allPeers || [])) {
  this.#store.upsertPeer(p.user_id, {
    nickname: p.nickname || '',
    hasSession: p.has_session,
    online: false,
  });
}
// Status online virá via presence:online_list e presence:online eventos
this.#chatUI.renderPeerList();
```

**Step 5: Atualizar evento invite:accepted**

O evento agora emite `PeerInfo` (com `user_id`, `nickname`, `has_session`). Atualizar o handler em `#bindWails()`:

```js
// ANTES:
E('invite:accepted', peer => {
  this.#store.upsertPeer(peer.user_id, { online: false });
  this.#chatUI.renderPeerList();
  this.#toast.show('Contact added: ' + peer.user_id);
});

// DEPOIS:
E('invite:accepted', peer => {
  this.#store.upsertPeer(peer.user_id, {
    nickname: peer.nickname || '',
    hasSession: peer.has_session,
    online: false,   // presence:online virá logo em seguida
  });
  this.#chatUI.renderPeerList();
  const name = this.#store.getDisplayName(peer.user_id);
  this.#toast.show('Contato adicionado: ' + name);
});
```

**Step 6: Testar manualmente**

Rodar `wails dev`. Enviar uma mensagem. Verificar que o bubble aparece no lado do remetente.

**Step 7: Commit**

```bash
git add frontend/chat.js
git commit -m "fix(frontend): mine:true bug, bootstrap via getAllPeers, PeerInfo in invite:accepted"
```

---

## Task 5: Micro-animações — CSS

**Files:**
- Modify: `frontend/style.css`

**Step 1: Adicionar keyframes e classes de animação**

No final de `style.css`, antes do último comentário/bloco, adicionar:

```css
/* ── Animations ──────────────────────────────────────────────────────── */

@keyframes msg-in {
  from { opacity: 0; transform: translateY(6px); }
  to   { opacity: 1; transform: translateY(0); }
}

@keyframes peer-in {
  from { opacity: 0; transform: scale(0.95); }
  to   { opacity: 1; transform: scale(1); }
}

@keyframes dot-pulse {
  0%   { transform: scale(1);    opacity: 1; }
  40%  { transform: scale(1.55); opacity: 0.7; }
  100% { transform: scale(1);    opacity: 1; }
}

@keyframes fade-in {
  from { opacity: 0; }
  to   { opacity: 1; }
}

@keyframes onboarding-up {
  from { opacity: 0; transform: translateY(8px); }
  to   { opacity: 1; transform: translateY(0); }
}

/* Message bubbles */
.msg {
  animation: msg-in 180ms ease-out both;
}

/* Peer list items */
.peer-item {
  animation: peer-in 200ms ease-out both;
  transition: background 150ms ease, transform 100ms ease;
}
.peer-item:hover  { transform: translateX(2px); }
.peer-item:active { transform: scale(0.98); }

/* Status dot pulse on peer-online event */
.status-dot.pulse {
  animation: dot-pulse 500ms ease-out;
}

/* Buttons — hover lift + active press */
.icon-btn,
.primary-btn,
.ghost-btn,
.header-action-btn,
.sidebar-footer-btn {
  transition: transform 100ms ease, box-shadow 150ms ease, opacity 100ms ease;
}
.icon-btn:hover,
.ghost-btn:hover,
.header-action-btn:hover,
.sidebar-footer-btn:hover {
  transform: translateY(-1px);
}
.icon-btn:active,
.primary-btn:active,
.ghost-btn:active,
.header-action-btn:active,
.sidebar-footer-btn:active {
  transform: scale(0.96);
}
.primary-btn:hover {
  transform: translateY(-1px);
  box-shadow: var(--accent-glow);
}

/* Send button spring */
.send-spring {
  animation: peer-in 120ms ease-out both;
}

/* Nickname inline input */
.nickname-input {
  background: transparent;
  border: none;
  border-bottom: 1px solid var(--accent-dim);
  color: var(--t100);
  font-family: var(--font-ui);
  font-size: inherit;
  font-weight: 600;
  outline: none;
  padding: 0 2px;
  width: 160px;
  animation: fade-in 150ms ease both;
}

/* Peer without session key — muted indicator */
.peer-item.no-session .peer-avatar {
  opacity: 0.4;
  border: 1px dashed var(--t20);
}
.peer-item.no-session .peer-name::after {
  content: ' ·';
  color: var(--t40);
  font-size: 10px;
}

/* Onboarding stagger */
.onboarding-logo   { animation: onboarding-up 300ms 0ms   ease-out both; }
.onboarding-card   { animation: onboarding-up 300ms 80ms  ease-out both; }
.onboarding-btn-p  { animation: onboarding-up 300ms 160ms ease-out both; }
.onboarding-sep    { animation: onboarding-up 300ms 200ms ease-out both; }
.onboarding-btn-g  { animation: onboarding-up 300ms 240ms ease-out both; }

/* Onboarding dissolve out */
@keyframes dissolve-out {
  to { opacity: 0; transform: scale(0.97); }
}
.onboarding-dissolve {
  animation: dissolve-out 220ms ease-in forwards;
}

/* Side panels — slide from right */
.side-panel {
  transition: transform 250ms cubic-bezier(0.16, 1, 0.3, 1),
              opacity   200ms ease;
}
.side-panel.hidden {
  transform: translateX(100%);
  opacity: 0;
  pointer-events: none;
}
.side-panel:not(.hidden) {
  transform: translateX(0);
  opacity: 1;
}
```

**Step 2: Verificar que o CSS não quebra nada visualmente**

Rodar `wails dev` e inspecionar que os elementos existentes ainda aparecem corretamente.

**Step 3: Commit**

```bash
git add frontend/style.css
git commit -m "feat(css): micro-animations — bubbles, peers, buttons, panels, onboarding"
```

---

## Task 6: Nickname UX no frontend

**Files:**
- Modify: `frontend/chat.js`
- Modify: `frontend/index.html`

**Step 1: Adicionar ícone de lápis no chat header (index.html)**

Localizar o `<div class="chat-peer-meta">` e adicionar o botão após `chat-peer-name`:

```html
<div class="chat-peer-meta">
  <div class="chat-peer-name-row">
    <div class="chat-peer-name" id="chat-peer-name">—</div>
    <button class="icon-btn nickname-edit-btn" id="nickname-edit-btn" title="Editar apelido">
      <svg viewBox="0 0 16 16" fill="none">
        <path d="M11.5 2.5a1.414 1.414 0 0 1 2 2L5 13H3v-2L11.5 2.5Z"
              stroke="currentColor" stroke-width="1.3" stroke-linejoin="round"/>
      </svg>
    </button>
  </div>
  <div class="chat-status-row">
    <span class="status-dot" id="chat-status-dot"></span>
    <span class="status-label" id="chat-peer-status">OFFLINE</span>
  </div>
</div>
```

Adicionar CSS mínimo no `style.css` para o `.chat-peer-name-row`:

```css
.chat-peer-name-row {
  display: flex;
  align-items: center;
  gap: 6px;
}
.nickname-edit-btn {
  opacity: 0;
  transition: opacity 150ms ease;
}
#chat-header:hover .nickname-edit-btn {
  opacity: 1;
}
```

**Step 2: Atualizar renderPeerList para usar displayName**

Na classe `ChatUI`, método `renderPeerList`, trocar o conteúdo da `li`:

```js
const name = this.#store.getDisplayName(p.id);
li.innerHTML = `
  <div class="peer-avatar${!p.hasSession ? ' no-session-avatar' : ''}">${name[0].toUpperCase()}</div>
  <div class="peer-info">
    <div class="peer-name">${name}</div>
    <div class="peer-id-short">${p.id}</div>
  </div>
  <span class="status-dot${p.online ? ' online' : ''}"></span>
`;
// Adicionar classe no-session se não tem session key
if (!p.hasSession) li.classList.add('no-session');
```

**Step 3: Implementar edit de nickname no ChatUI**

Adicionar método `#bindNicknameEdit()` na classe `ChatUI` e chamá-lo no constructor:

```js
#bindNicknameEdit() {
  document.getElementById('nickname-edit-btn')?.addEventListener('click', () => {
    const peer = this.#store.getActivePeer();
    if (!peer) return;
    const nameEl = document.getElementById('chat-peer-name');
    const current = nameEl.textContent;

    const input = document.createElement('input');
    input.className = 'nickname-input';
    input.value = current === peer ? '' : current; // se for o ID, começa vazio
    input.placeholder = peer;
    input.maxLength = 32;

    nameEl.replaceWith(input);
    input.focus();
    input.select();

    const save = () => {
      const val = input.value.trim();
      this.#bus.emit('nickname:save', { userID: peer, nickname: val });
      const newName = val || peer;
      const span = document.createElement('div');
      span.className = 'chat-peer-name';
      span.id = 'chat-peer-name';
      span.textContent = newName;
      input.replaceWith(span);
    };

    input.addEventListener('keydown', e => {
      if (e.key === 'Enter') { e.preventDefault(); save(); }
      if (e.key === 'Escape') {
        const span = document.createElement('div');
        span.className = 'chat-peer-name';
        span.id = 'chat-peer-name';
        span.textContent = current;
        input.replaceWith(span);
      }
    });
    input.addEventListener('blur', save);
  });
}
```

Chamar no constructor: `this.#bindNicknameEdit();`

**Step 4: Adicionar handler no App bus**

Em `App.#bindBus()`, adicionar:

```js
this.#bus.on('nickname:save', async ({ userID, nickname }) => {
  try {
    await this.#bridge.setNickname(userID, nickname);
    this.#store.upsertPeer(userID, { nickname });
    this.#chatUI.renderPeerList();
  } catch (e) {
    this.#toast.show('Falha ao salvar apelido');
  }
});
```

**Step 5: Atualizar openChat para usar displayName**

```js
openChat(peerID) {
  const p = this.#store.getPeer(peerID);
  const name = this.#store.getDisplayName(peerID);
  document.getElementById('empty-state').classList.add('hidden');
  const cv = document.getElementById('chat-view');
  cv.classList.remove('hidden');
  document.getElementById('chat-peer-name').textContent = name;
  document.getElementById('chat-avatar').textContent = name[0].toUpperCase();
  // ...resto igual
```

**Step 6: Testar**

`wails dev`. Abrir chat com um peer. Hover no header → ícone de lápis aparece. Clicar → input inline. Digitar apelido → Enter. Verificar que sidebar atualiza.

**Step 7: Commit**

```bash
git add frontend/chat.js frontend/index.html frontend/style.css
git commit -m "feat(frontend): nickname inline edit with pencil icon, displayName everywhere"
```

---

## Task 7: Status dot pulse na animação de peer online

**Files:**
- Modify: `frontend/chat.js`

**Step 1: Adicionar pulse no updatePeerStatus**

Na classe `ChatUI`, método `updatePeerStatus`:

```js
updatePeerStatus(id, online) {
  this.renderPeerList();
  if (online) {
    // Pulsar o dot do peer que acabou de ficar online
    requestAnimationFrame(() => {
      const li = document.querySelector(`[data-id="${id}"] .status-dot`);
      if (li) {
        li.classList.remove('pulse');
        void li.offsetWidth; // reflow para reiniciar animação
        li.classList.add('pulse');
        li.addEventListener('animationend', () => li.classList.remove('pulse'), { once: true });
      }
    });
  }
  if (this.#store.getActivePeer() === id) this.#setStatus(online);
}
```

**Step 2: Commit**

```bash
git add frontend/chat.js
git commit -m "feat(frontend): status dot pulse animation on peer-online event"
```

---

## Task 8: Onboarding screen

**Files:**
- Modify: `frontend/index.html`
- Modify: `frontend/chat.js`
- Modify: `frontend/style.css`

**Step 1: Adicionar HTML do onboarding em index.html**

Substituir o bloco `#empty-state` atual pelo novo (mais o onboarding):

```html
<!-- Empty state (tem peers mas nenhum selecionado) -->
<div id="empty-state" class="hidden">
  <div class="empty-icon">
    <!-- SVG existente -->
  </div>
  <p class="empty-title">No channel selected</p>
  <p class="empty-sub">Select a contact or generate an invite to open an encrypted session.</p>
</div>

<!-- Onboarding (nenhum peer ainda) -->
<div id="onboarding-state" class="hidden">
  <div class="onboarding-wrap">
    <div class="onboarding-logo">
      <svg class="brand-icon brand-icon-lg" viewBox="0 0 28 28" fill="none">
        <path d="M14 2C8.48 2 4 6.48 4 12v13l2.5-2.5L9 25l2.5-2.5L14 25l2.5-2.5L19 25l2.5-2.5L24 25V12C24 6.48 19.52 2 14 2z"
              stroke="currentColor" stroke-width="1.5" stroke-linejoin="round"/>
        <circle cx="10" cy="13" r="1.5" fill="currentColor"/>
        <circle cx="18" cy="13" r="1.5" fill="currentColor"/>
      </svg>
      <div class="onboarding-title">UMBRA</div>
      <div class="brand-sub">CANAL ENCRIPTADO</div>
    </div>

    <div class="onboarding-card">
      <div class="label-xs">SEU ENDEREÇO</div>
      <code class="identity-id onboarding-id" id="onboarding-id-display">——————————</code>
      <button class="icon-btn onboarding-copy-btn" id="onboarding-copy-btn" title="Copiar">
        <svg viewBox="0 0 16 16" fill="none">
          <rect x="5.5" y="5.5" width="7" height="7" rx="1.5" stroke="currentColor" stroke-width="1.3"/>
          <path d="M3.5 10.5V4A1.5 1.5 0 0 1 5 2.5h6.5" stroke="currentColor" stroke-width="1.3" stroke-linecap="round"/>
        </svg>
      </button>
      <p class="onboarding-hint">Compartilhe seu endereço com um amigo para que ele gere um invite.</p>
    </div>

    <button class="primary-btn onboarding-btn-p" id="onboarding-resolve-btn">
      ENTRAR COM INVITE CODE
    </button>

    <div class="onboarding-sep">ou</div>

    <button class="ghost-btn onboarding-btn-g" id="onboarding-create-btn">
      GERAR INVITE
    </button>
  </div>
</div>
```

**Step 2: Adicionar CSS para o onboarding em style.css**

```css
/* ── Onboarding ──────────────────────────────────────────────────────── */
#onboarding-state {
  display: flex;
  align-items: center;
  justify-content: center;
  height: 100%;
}
#onboarding-state.hidden { display: none; }

.onboarding-wrap {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 20px;
  max-width: 340px;
  width: 100%;
  padding: 32px 24px;
}

.brand-icon-lg {
  width: 48px;
  height: 48px;
  color: var(--accent);
}

.onboarding-title {
  font-family: var(--font-mono);
  font-size: 22px;
  font-weight: 700;
  letter-spacing: 0.12em;
  color: var(--t100);
  margin-top: 8px;
}

.onboarding-card {
  width: 100%;
  background: var(--glass-bg);
  border: 1px solid var(--glass-rim);
  border-radius: var(--radius-card);
  padding: 16px 20px;
  display: flex;
  flex-direction: column;
  gap: 8px;
  position: relative;
  text-align: center;
}

.onboarding-id {
  font-size: 13px;
  word-break: break-all;
  color: var(--accent);
}

.onboarding-copy-btn {
  position: absolute;
  top: 10px;
  right: 10px;
}

.onboarding-hint {
  font-size: 11px;
  color: var(--t40);
  line-height: 1.5;
}

.onboarding-sep {
  font-size: 11px;
  color: var(--t20);
  letter-spacing: 0.08em;
}

.onboarding-btn-p,
.onboarding-btn-g {
  width: 100%;
}
```

**Step 3: Lógica de show/hide no App (chat.js)**

No método `#bootstrap()`, após carregar os peers:

```js
async #bootstrap() {
  const myID = await this.#bridge.getMyID();
  this.#store.setMyID(myID);
  document.getElementById('my-id-display').textContent = myID;
  document.getElementById('copy-id-btn')?.addEventListener('click', () => {
    navigator.clipboard.writeText(myID);
    this.#toast.show('Endereço copiado');
  });

  const allPeers = await this.#bridge.getAllPeers();
  for (const p of (allPeers || [])) {
    this.#store.upsertPeer(p.user_id, {
      nickname: p.nickname || '',
      hasSession: p.has_session,
      online: false,
    });
  }
  this.#chatUI.renderPeerList();

  // Mostrar onboarding se não há peers
  if (!allPeers || allPeers.length === 0) {
    this.#showOnboarding(myID);
  } else {
    document.getElementById('empty-state').classList.remove('hidden');
  }
}
```

Adicionar método `#showOnboarding` e `#hideOnboarding` no App:

```js
#showOnboarding(myID) {
  document.getElementById('empty-state').classList.add('hidden');
  const ob = document.getElementById('onboarding-state');
  ob.classList.remove('hidden');
  document.getElementById('onboarding-id-display').textContent = myID;

  document.getElementById('onboarding-copy-btn')?.addEventListener('click', () => {
    navigator.clipboard.writeText(myID);
    this.#toast.show('Endereço copiado');
  });
  document.getElementById('onboarding-resolve-btn')?.addEventListener('click', () => {
    this.#inviteUI.showResolveForm();
  });
  document.getElementById('onboarding-create-btn')?.addEventListener('click', () => {
    this.#bus.emit('invite:create', null);
  });
}

#hideOnboarding(firstPeerID) {
  const ob = document.getElementById('onboarding-state');
  ob.classList.add('onboarding-dissolve');
  ob.addEventListener('animationend', () => {
    ob.classList.add('hidden');
    ob.classList.remove('onboarding-dissolve');
    // Selecionar o primeiro peer automaticamente
    this.#bus.emit('peer:selected', firstPeerID);
    document.getElementById('empty-state').classList.remove('hidden');
  }, { once: true });
}
```

No handler `invite:accepted` em `#bindWails()`, detectar se é o primeiro peer:

```js
E('invite:accepted', peer => {
  const isFirst = this.#store.getAllPeers().length === 0;
  this.#store.upsertPeer(peer.user_id, {
    nickname: peer.nickname || '',
    hasSession: peer.has_session,
    online: false,
  });
  this.#chatUI.renderPeerList();
  const name = this.#store.getDisplayName(peer.user_id);
  this.#toast.show('Contato adicionado: ' + name);
  if (isFirst) this.#hideOnboarding(peer.user_id);
});
```

**Step 4: Testar**

`wails dev` com `peers.json` vazio (renomear temporariamente `~/.umbra/peers.json`). Verificar tela de onboarding com stagger de animação. Testar botões.

**Step 5: Commit**

```bash
git add frontend/index.html frontend/chat.js frontend/style.css
git commit -m "feat(frontend): onboarding screen for first-run with stagger animation"
```

---

## Task 9: nebula.js — expor setNebulaOpacity

**Files:**
- Modify: `frontend/nebula.js`

**Step 1: Expor controle de opacidade**

Dentro do IIFE de `nebula.js`, após `frame()` ser chamado (linha ~252), adicionar antes do `window.addEventListener('beforeunload', ...)`:

```js
// Expose opacity control for settings panel
window.setNebulaOpacity = function(v) {
  canvas.style.opacity = Math.max(0, Math.min(1, v));
};
```

**Step 2: Testar**

`wails dev`, abrir console do browser, digitar `setNebulaOpacity(0.2)`. Verificar que nebula escurece.

**Step 3: Commit**

```bash
git add frontend/nebula.js
git commit -m "feat(nebula): expose window.setNebulaOpacity for settings panel"
```

---

## Task 10: Painel de configurações visuais

**Files:**
- Modify: `frontend/index.html`
- Modify: `frontend/style.css`
- Modify: `frontend/chat.js`

**Step 1: Adicionar HTML do settings panel em index.html**

Após o `#capsule-panel` existente, adicionar:

```html
<!-- ═══════════════ SETTINGS PANEL ═══════════════ -->
<div id="settings-panel" class="side-panel hidden">
  <div class="panel-header">
    <div class="section-title-row">
      <div class="section-accent"></div>
      <span class="label-xs">CONFIGURAÇÕES VISUAIS</span>
    </div>
    <button class="icon-btn" id="settings-close-btn">
      <svg viewBox="0 0 16 16" fill="none">
        <path d="M4 4l8 8M12 4l-8 8" stroke="currentColor" stroke-width="1.5" stroke-linecap="round"/>
      </svg>
    </button>
  </div>
  <div class="panel-body">

    <div class="field-group">
      <label class="field-label" for="setting-distortion">
        DISTORÇÃO DO VIDRO
        <span class="setting-value" id="setting-distortion-val">50%</span>
      </label>
      <input type="range" id="setting-distortion" class="setting-slider" min="0" max="100" value="50"/>
    </div>

    <div class="field-group">
      <label class="field-label" for="setting-nebula">
        INTENSIDADE DO NEBULA
        <span class="setting-value" id="setting-nebula-val">85%</span>
      </label>
      <input type="range" id="setting-nebula" class="setting-slider" min="0" max="100" value="85"/>
    </div>

    <div class="field-group">
      <label class="field-label" for="setting-glass">
        OPACIDADE DO VIDRO
        <span class="setting-value" id="setting-glass-val">50%</span>
      </label>
      <input type="range" id="setting-glass" class="setting-slider" min="10" max="90" value="50"/>
    </div>

    <div class="field-note">
      <svg viewBox="0 0 12 12" fill="none">
        <circle cx="6" cy="6" r="4.5" stroke="currentColor" stroke-width="1"/>
        <path d="M6 4.5v3M6 9v.2" stroke="currentColor" stroke-width="1.2" stroke-linecap="round"/>
      </svg>
      Configurações salvas automaticamente.
    </div>

    <button class="ghost-btn" id="settings-reset-btn">RESTAURAR PADRÕES</button>
  </div>
</div>
```

**Step 2: Adicionar ícone de engrenagem na sidebar (index.html)**

Na `.sidebar-footer`, adicionar o botão antes do `resolve-invite-btn`:

```html
<div class="sidebar-footer">
  <button class="sidebar-footer-btn" id="settings-open-btn">
    <svg viewBox="0 0 14 14" fill="none">
      <circle cx="7" cy="7" r="2" stroke="currentColor" stroke-width="1.2"/>
      <path d="M7 1v1.2M7 11.8V13M1 7h1.2M11.8 7H13M2.93 2.93l.85.85M10.22 10.22l.85.85M2.93 11.07l.85-.85M10.22 3.78l.85-.85"
            stroke="currentColor" stroke-width="1.2" stroke-linecap="round"/>
    </svg>
    Configurações
  </button>
  <button class="sidebar-footer-btn" id="resolve-invite-btn">
    <!-- SVG existente -->
    Enter invite code
  </button>
</div>
```

**Step 3: Adicionar CSS do settings slider em style.css**

```css
/* ── Settings slider ─────────────────────────────────────────────────── */
.setting-slider {
  width: 100%;
  -webkit-appearance: none;
  appearance: none;
  height: 4px;
  border-radius: 2px;
  background: var(--glass-divider);
  outline: none;
  cursor: pointer;
}
.setting-slider::-webkit-slider-thumb {
  -webkit-appearance: none;
  width: 14px;
  height: 14px;
  border-radius: 50%;
  background: var(--accent);
  cursor: pointer;
  transition: transform 100ms ease;
}
.setting-slider::-webkit-slider-thumb:hover {
  transform: scale(1.2);
}
.setting-value {
  float: right;
  font-size: 10px;
  color: var(--accent);
  font-family: var(--font-mono);
}
```

**Step 4: Lógica do settings panel em chat.js**

Adicionar classe `SettingsUI` antes da classe `App`:

```js
/* ─── SettingsUI ─────────────────────────────────────────────────────── */
class SettingsUI {
  static DEFAULTS = { distortion: 50, nebula: 85, glass: 50 };
  static KEY = 'umbra:settings';

  #current = { ...SettingsUI.DEFAULTS };

  constructor() {
    this.#load();
    this.#apply(this.#current);
    this.#bindDOM();
  }

  #load() {
    try {
      const raw = localStorage.getItem(SettingsUI.KEY);
      if (raw) this.#current = { ...SettingsUI.DEFAULTS, ...JSON.parse(raw) };
    } catch {}
  }

  #save() {
    localStorage.setItem(SettingsUI.KEY, JSON.stringify(this.#current));
  }

  #apply({ distortion, nebula, glass }) {
    // Distorção — escala proporcional: panel=base, btn=base*0.6, pill=base*0.4
    const base = (distortion / 100) * 60; // 0-100 → 0-60
    ['glass-panel', 'glass-btn', 'glass-pill'].forEach((id, i) => {
      const factors = [1, 0.6, 0.4];
      const scale = base * factors[i];
      const dm = document.querySelector(`#${id} feDisplacementMap`);
      if (dm) dm.setAttribute('scale', scale.toFixed(1));
    });

    // Nebula
    const nebulaV = nebula / 100; // 0-100 → 0-1
    if (window.setNebulaOpacity) window.setNebulaOpacity(nebulaV);

    // Glass opacity
    const glassAlpha = glass / 100 * 0.8 + 0.1; // 10-90% → 0.1-0.9 alpha
    document.documentElement.style.setProperty(
      '--glass-bg', `rgba(12,12,24,${glassAlpha.toFixed(2)})`
    );
  }

  #bindDOM() {
    document.getElementById('settings-open-btn')
      ?.addEventListener('click', () => {
        this.#syncSliders();
        document.getElementById('settings-panel').classList.remove('hidden');
      });
    document.getElementById('settings-close-btn')
      ?.addEventListener('click', () => {
        document.getElementById('settings-panel').classList.add('hidden');
      });

    const bind = (id, valId, key, min, max) => {
      const slider = document.getElementById(id);
      const valEl = document.getElementById(valId);
      if (!slider) return;
      slider.addEventListener('input', () => {
        const v = parseInt(slider.value, 10);
        this.#current[key] = v;
        valEl.textContent = v + '%';
        this.#apply(this.#current);
        this.#save();
      });
    };
    bind('setting-distortion', 'setting-distortion-val', 'distortion', 0, 100);
    bind('setting-nebula',     'setting-nebula-val',     'nebula',     0, 100);
    bind('setting-glass',      'setting-glass-val',      'glass',      10, 90);

    document.getElementById('settings-reset-btn')?.addEventListener('click', () => {
      this.#current = { ...SettingsUI.DEFAULTS };
      this.#syncSliders();
      this.#apply(this.#current);
      this.#save();
    });
  }

  #syncSliders() {
    const set = (id, valId, key) => {
      const s = document.getElementById(id);
      const v = document.getElementById(valId);
      if (s) s.value = this.#current[key];
      if (v) v.textContent = this.#current[key] + '%';
    };
    set('setting-distortion', 'setting-distortion-val', 'distortion');
    set('setting-nebula',     'setting-nebula-val',     'nebula');
    set('setting-glass',      'setting-glass-val',      'glass');
  }
}
```

No `App.init()`, adicionar:

```js
async init() {
  this.#settings = new SettingsUI();  // ← antes de tudo para aplicar settings no boot
  // ...resto igual
```

Adicionar campo na classe App: `#settings;`

**Step 5: Testar**

`wails dev`. Clicar em "Configurações" na sidebar. Mover sliders. Verificar que distorção, nebula e opacidade do vidro mudam em tempo real. Fechar e reabrir app → configurações persistem.

**Step 6: Commit**

```bash
git add frontend/index.html frontend/style.css frontend/chat.js
git commit -m "feat(frontend): visual settings panel — distortion, nebula, glass opacity sliders"
```

---

## Task 11: Send button spring animation

**Files:**
- Modify: `frontend/chat.js`

**Step 1: Adicionar animação de spring no send**

No `ChatUI.#bindDOM()`, no event listener do form submit, adicionar após `form.requestSubmit()`:

```js
form.addEventListener('submit', e => {
  e.preventDefault();
  const text = input.value.trim();
  if (!text) return;
  // Spring animation no botão de envio
  const btn = document.getElementById('send-btn');
  btn.classList.remove('send-spring');
  void btn.offsetWidth;
  btn.classList.add('send-spring');
  btn.addEventListener('animationend', () => btn.classList.remove('send-spring'), { once: true });
  this.#bus.emit('msg:send', text);
  input.value = '';
});
```

**Step 2: Commit**

```bash
git add frontend/chat.js
git commit -m "feat(frontend): send button spring animation on submit"
```

---

## Verificação final

Após todas as tasks:

1. `go build ./...` — zero erros
2. `go test ./...` — todos os testes passam
3. `wails dev` — checar manualmente:
   - [ ] Enviar mensagem → bubble aparece nos dois lados
   - [ ] Peer fica online → dot pulsa
   - [ ] Invite → peer adicionado online
   - [ ] Primeiro boot (sem peers) → onboarding aparece com stagger
   - [ ] Adicionar primeiro contato → onboarding dissolve, chat abre
   - [ ] Editar nickname → pencil icon, inline input, atualiza sidebar
   - [ ] Settings panel → sliders mudam visual em real-time, persistem
   - [ ] Todos os botões → hover lift + active press

```bash
git log --oneline
```

Esperado: 11 commits limpos após o commit inicial.
