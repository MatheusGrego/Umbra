# Umbra — Style, Voice, Screen Share & Capsule UX — Design Doc

**Data:** 2026-02-25
**Status:** Aprovado

---

## Escopo

Quatro áreas independentes com problemas identificados:

1. **Glass + Distorção** — feImage não funciona no WebView do Wails; distorção sumiu
2. **Voice Call** — sinalização não chega ao callee; sem áudio; sem seleção de dispositivo
3. **Screen Share** — tela preta no receiver; sender sem feedback de "estou compartilhando"
4. **Capsule UX** — sem histórico de enviadas/recebidas; sem indicador no chat

---

## 1. Glass + Distorção

### Causa raiz

`feImage href="#nebula-canvas"` não captura o canvas WebGL dinamicamente no WebView2 (Windows) nem no WebKit2GTK (Linux). O SVG trata feImage como imagem estática; a captura frame-a-frame não acontece automaticamente.

### Solução

Voltar para `feTurbulence` como fonte de displacement, mas animada via JS — sem dependência do canvas do nebula.

### Pipeline SVG (sem mudança de estrutura)

```
feTurbulence(low-freq, animada) → feGaussianBlur(1) → feDisplacementMap
feTurbulence(high-freq, grain estático) → feColorMatrix(alpha 4%) → feMerge(displaced + grain)
```

Scales por tipo (retornam ao HTML; JS pode sobrescrever via slider):
- `glass-panel` feDisplacementMap scale = 65
- `glass-btn` scale = 35
- `glass-pill` scale = 18

### Animação via JS

Um loop em `nebula.js` (junto ao loop WebGL existente) atualiza `baseFrequency` das 3 turbulências de displacement a ~30fps (throttle por delta time):

```
baseFrequency X(t) = 0.008 + sin(t × 0.6) × 0.004
baseFrequency Y(t) = 0.008 + sin(t × 0.6 + 1.2) × 0.004
```

Cada filtro tem seed distinto (42 / 17 / 5), garantindo padrões independentes. O slider de distorção em Configurações continua controlando `feDisplacementMap.scale` via `SettingsUI.#apply()` — sem mudança em `chat.js`.

### Sem mudanças

- Estrutura `::before` / `::after` — não muda
- Grain feTurbulence (high-freq) — não muda
- Tokens CSS de tema — não mudam

---

## 2. Voice Call

### Causa raiz

**Dupla:** (a) sinalização não alcança o callee — possível divergência de tipo de mensagem `voice_offer` no servidor ou handler não registrado; (b) `VoiceChat.go` usa Pion WebRTC sem acesso ao microfone do browser — `getUserMedia()` é API de browser, inacessível ao Go.

### Solução: Go vira sinalização pura, WebRTC move para JS

**Go (`VoiceService.go`):** Mantém apenas o roteamento de envelopes WS. Recebe mensagens `voice_*` do servidor e emite Wails events para o frontend. Envia mensagens `voice_*` para o servidor a partir de chamadas do frontend.

**Novo método Wails (`app.go`):**
```go
SendVoiceSignal(peerID, msgType, payload string) error
```
Encapsula qualquer envelope `voice_*` e envia pelo WebSocket.

**Wails events emitidos pelo Go → frontend:**
- `voice:incoming` `{ peer, sdp }` — oferta recebida
- `voice:answer` `{ peer, sdp }` — resposta recebida
- `voice:ice` `{ peer, candidate }` — candidato ICE recebido
- `voice:rejected` `{ peer }` — chamada rejeitada
- `voice:ended` — chamada encerrada pelo outro lado

**Fluxo caller (JS):**
1. `getUserMedia({ audio: { deviceId: selectedMicId } })`
2. Cria `RTCPeerConnection` com STUN (`stun:stun.l.google.com:19302`)
3. Adiciona audio track
4. `createOffer()` → `SendVoiceSignal(peer, "voice_offer", sdp)`
5. Aguarda `voice:answer` → `setRemoteDescription(answer)`
6. ICE: `onicecandidate` → `SendVoiceSignal(peer, "voice_ice", candidate)`

**Fluxo callee (JS):**
1. Recebe `voice:incoming` → mostra modal de aceitar (UI já existente)
2. No aceite: cria `RTCPeerConnection`, `setRemoteDescription(offer)`
3. `getUserMedia` → adiciona track → `createAnswer()` → `SendVoiceSignal(peer, "voice_answer", sdp)`
4. ICE simétrico
5. `ontrack` → `<audio autoplay hidden>.srcObject = event.streams[0]`

**Seleção de dispositivo:**
- `navigator.mediaDevices.enumerateDevices()` popula dois `<select>` em Configurações: **Microfone** e **Saída de áudio**
- IDs persistidos em `SettingsUI.DEFAULTS` (`micDeviceId: ''`, `speakerDeviceId: ''`)
- `#apply()` não faz nada com eles em tempo real — aplicados na próxima chamada via `getUserMedia` constraints e `setSinkId()` no elemento `<audio>`

**Remoção:** `webrtc/voicechat.go` pode ser deletado ou esvaziado (mantém interface por compatibilidade).

---

## 3. Screen Share

### Causa raiz

`ScreenShareService.go` chama `StartShare()` que cria um offer Pion **sem track de vídeo** — `getDisplayMedia()` não pode ser chamado do Go. O receiver recebe um offer SDP sem mídia, abrindo uma conexão vazia → tela preta.

### Solução: Mesma arquitetura do voice — Go sinaliza, JS captura e conecta

**Go (`ScreenShareService.go`):** Apenas roteamento de envelopes `webrtc_*`.

**Novo método Wails (`app.go`):**
```go
SendScreenSignal(peerID, msgType, payload string) error
```

**Wails events emitidos pelo Go → frontend:**
- `screenshare:incoming` `{ peer, sdp }` — oferta recebida (já existe, agora inclui SDP)
- `screenshare:answer` `{ peer, sdp }` — resposta recebida
- `screenshare:ice` `{ peer, candidate }` — candidato ICE
- `screenshare:rejected` `{ peer }` — rejeitado
- `screenshare:stopped` — sessão encerrada pelo outro lado

**Fluxo sender (JS):**
1. Confirmation modal (já existe) → ao confirmar:
2. `getDisplayMedia({ video: true, audio: false })`
3. Cria `RTCPeerConnection`, adiciona video track
4. `createOffer()` → `SendScreenSignal(peer, "webrtc_offer", sdp)`
5. Mostra **barra de compartilhamento** (novo elemento, topo da tela):
   ```
   ● COMPARTILHANDO COM {peer}    [PARAR]
   ```
6. ICE simétrico via `SendScreenSignal(peer, "webrtc_ice", candidate)`

**Fluxo receiver (JS):**
1. Recebe `screenshare:incoming` → mostra modal de aceitar (já existe)
2. No aceite: cria `RTCPeerConnection`, `setRemoteDescription(offer)`, `createAnswer()`
3. `SendScreenSignal(peer, "webrtc_answer", sdp)`
4. `ontrack` → `<video id="screenshare-video">.srcObject = event.streams[0]`
5. Mostra overlay de recebimento (já existe)

**Stop (sender):** botão PARAR na barra → fecha `RTCPeerConnection`, para tracks de `getDisplayMedia`, `SendScreenSignal(peer, "webrtc_stop", "")` → ambos limpam e fecham overlay.

**Stop (receiver):** botão END SESSION no overlay existente → `SendScreenSignal(peer, "webrtc_stop", "")`.

---

## 4. Capsule UX

### Causa raiz

Cápsulas enviadas não têm rastreamento. Cápsulas recebidas mostram apenas um modal descartável, sem histórico. Não há indicação contextual no chat de que uma cápsula foi enviada para aquele peer.

### Solução: Abas no painel + bolhas no chat

**Painel lateral (`.side-panel`) ganha 3 abas:**

```
[ NOVA ]   [ ENVIADAS ]   [ RECEBIDAS ]
```

**Aba NOVA:** composer existente, sem mudança.

**Aba ENVIADAS:**
- Lista de cápsulas enviadas na sessão atual (em memória, `Map<id, CapsuleEntry>`)
- Cada item: peer ID | delay escolhido | estimativa de entrega (timestamp de envio + delay) | status
- Status: `⧗ pendente` → `✓ entregue` (quando `capsule_ready` chegar — confirma que o servidor liberou)
- Sem persistência entre sessões nesta iteração

**Aba RECEBIDAS:**
- Lista de cápsulas recebidas na sessão
- Cada item: remetente | preview de texto (50 chars) | horário de chegada
- Clicar abre o modal existente com o texto completo

**Bolha no chat:**
- Ao enviar uma cápsula para um peer, uma bolha especial aparece no histórico daquele peer:
  ```
  ⧗  Cápsula enviada · entrega em 1h
  ```
- Quando `capsule_ready` chegar (entregue), a bolha atualiza para:
  ```
  ✓  Cápsula entregue · {horário}
  ```
- Bolhas de cápsula recebida aparecem no chat como bolha distinta:
  ```
  ⧗  Cápsula de {peer} · {horário recebido}
  ```

**Storage:** `Map` em memória no `CapsuleUI` / `CapsuleStore`. Sem localStorage nesta iteração.

---

## Arquivos afetados

| Área | Arquivo | Operação |
|---|---|---|
| Glass | `frontend/index.html` | Reescrever filtros SVG (feImage → feTurbulence) |
| Glass | `frontend/nebula.js` | Adicionar loop de animação das turbulências |
| Voice | `app.go` | Adicionar `SendVoiceSignal()` |
| Voice | `service/voice.go` | Remover Pion; manter só roteamento WS |
| Voice | `frontend/chat.js` | Novo módulo JS WebRTC para voice + device selection |
| Voice | `frontend/index.html` | Selects de microfone/saída em Configurações |
| Voice | `webrtc/voicechat.go` | Deletar ou esvaziar |
| Screen Share | `app.go` | Adicionar `SendScreenSignal()` |
| Screen Share | `service/screenshare.go` | Remover Pion; manter só roteamento WS |
| Screen Share | `frontend/chat.js` | Novo módulo JS WebRTC para screenshare + barra sender |
| Screen Share | `frontend/index.html` | Barra de "compartilhando" (novo elemento) |
| Screen Share | `webrtc/screenshare.go` | Deletar ou esvaziar |
| Capsule | `frontend/index.html` | Abas no painel + bolhas no chat |
| Capsule | `frontend/chat.js` | CapsuleStore em memória + lógica de abas + bolhas |
| Capsule | `frontend/style.css` | Estilos para abas, bolha especial de cápsula |

---

## Sem mudanças

- Protocolo WS e tipos de mensagem — não mudam
- Crypto (AES-256-GCM para capsule) — não muda
- Nebula shader (`nebula.js` shader GLSL) — não muda
- Autenticação e invite flow — não mudam
- Tokens CSS de tema — não mudam (exceto novos tokens para bolha de capsule)
