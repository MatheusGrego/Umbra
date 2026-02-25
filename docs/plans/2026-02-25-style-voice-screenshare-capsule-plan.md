# Style, Voice, Screen Share & Capsule UX — Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Corrigir distorção glass (feImage → feTurbulence animada), reescrever voice call e screen share com WebRTC puro em JS, e adicionar histórico + bolhas ao Capsule.

**Architecture:** Go cuida apenas de sinalização WS (SendVoiceSignal / SendScreenSignal). Todo WebRTC (getUserMedia, getDisplayMedia, RTCPeerConnection, offer/answer/ICE) vive em JS. Nebula.js anima feTurbulence dos filtros SVG a ~30fps. Capsule ganha abas (NOVA/ENVIADAS/RECEBIDAS) e bolhas no chat.

**Tech Stack:** Go 1.22, Wails v2.11.0, vanilla JS (ES modules), WebGL (GLSL), CSS custom properties, SVG filters (feTurbulence, feDisplacementMap), WebRTC browser API (RTCPeerConnection, getUserMedia, getDisplayMedia)

---

## Notas gerais

- Todos os arquivos frontend em `frontend/`
- Verificação Go: `go build ./...` na raiz de `client/`
- Verificação visual: `wails dev`
- Não há testes automatizados para CSS/WebRTC — verificação é visual
- Cada task tem seu próprio commit

---

## Task 1: Corrigir filtros SVG — feImage → feTurbulence animada

**Arquivos:**
- Modificar: `frontend/index.html`
- Modificar: `frontend/nebula.js`

**Contexto:** `feImage href="#nebula-canvas"` não captura canvas WebGL dinamicamente no WebView do Wails. Reverter para feTurbulence com IDs nos elementos de displacement, e animar `baseFrequency` via JS no loop do nebula.

---

**Step 1: Reescrever os 3 filtros SVG em `frontend/index.html`**

Localizar o bloco `<svg style="display:none">` (linhas 25–83) e substituir pelo seguinte. Diferenças-chave: (a) feImage removido, (b) feTurbulence de displacement com `id` para o JS, (c) grain mantido, (d) comment atualizado.

```xml
  <!-- ============================================================
     LIQUID GLASS SVG FILTERS  (v3 — feTurbulence animada via JS)
     feTurbulence(disp, low-freq) animada em nebula.js a ~30fps.
     feDisplacementMap desloca o backdrop com o mapa de ruído.
     feTurbulence(grain, high-freq) sobrepõe textura de superfície.
     Applied via CSS filter: url(#id) on ::after pseudo-elements.
     ============================================================ -->
  <svg style="display:none" xmlns="http://www.w3.org/2000/svg" aria-hidden="true">
    <defs>
      <!-- Large panel: sidebar, main pane, modals -->
      <filter id="glass-panel" x="0%" y="0%" width="100%" height="100%">
        <feTurbulence id="disp-turb-panel" type="fractalNoise"
                      baseFrequency="0.008 0.008" numOctaves="2" seed="92" result="noise"/>
        <feGaussianBlur in="noise" stdDeviation="0.02" result="blur"/>
        <feDisplacementMap in="SourceGraphic" in2="blur" scale="65"
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
      <!-- Button / bubble: icon controls, message bubbles, pills -->
      <filter id="glass-btn" x="0%" y="0%" width="100%" height="100%">
        <feTurbulence id="disp-turb-btn" type="fractalNoise"
                      baseFrequency="0.012 0.012" numOctaves="2" seed="17" result="noise"/>
        <feGaussianBlur in="noise" stdDeviation="0.02" result="blur"/>
        <feDisplacementMap in="SourceGraphic" in2="blur" scale="35"
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
      <!-- Tight: input area, small cards, inline tokens -->
      <filter id="glass-pill" x="0%" y="0%" width="100%" height="100%">
        <feTurbulence id="disp-turb-pill" type="fractalNoise"
                      baseFrequency="0.018 0.018" numOctaves="2" seed="5" result="noise"/>
        <feGaussianBlur in="noise" stdDeviation="0.015" result="blur"/>
        <feDisplacementMap in="SourceGraphic" in2="blur" scale="18"
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
    </defs>
  </svg>
```

---

**Step 2: Adicionar loop de animação das turbulências em `frontend/nebula.js`**

Localizar (linha ~364):
```js
    let animId;

    function frame() {
      const elapsed = (performance.now() - startTime) / 1000.0;
      gl.uniform1f(uniforms.uTime, elapsed);
      gl.drawArrays(gl.TRIANGLE_STRIP, 0, 4);
      animId = requestAnimationFrame(frame);
    }
```

Substituir por:
```js
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
```

---

**Step 3: Verificar**

```bash
go build ./...
```
Esperado: sem erros.

```
wails dev
```

Verificar:
- Painéis e cards têm distorção visível
- Distorção anima suavemente (ondula devagar, ciclo ~10s)
- Grain sutil visível nas bordas dos painéis
- Slider de Distorção em Configurações ainda controla a intensidade

---

**Step 4: Commit**

```bash
git add frontend/index.html frontend/nebula.js
git commit -m "feat(glass): animate feTurbulence displacement via JS, fix broken feImage approach"
```

---

## Task 2: Voice Call — Go sinalização

**Arquivos:**
- Modificar: `service/voice.go`
- Modificar: `app.go`

**Contexto:** Go para de gerenciar Pion WebRTC. `VoiceService` vira roteador de sinais WS. Novos métodos emitem payloads SDP/ICE para o frontend via Wails events. `app.go` expõe `SendVoiceSignal` para o JS chamar diretamente.

---

**Step 1: Reescrever `service/voice.go`**

Substituir o arquivo completo por:

```go
// Package service — voice.go
// VoiceService roteia mensagens de sinalização WS para voice call.
// WebRTC (getUserMedia, RTCPeerConnection, tracks) vive no frontend JS.
package service

import (
	"encoding/json"
	"log"
	"sync"

	"umbra/client/ws"
)

// VoiceService gerencia estado de sinalização de voice call.
type VoiceService struct {
	myUserID string
	sender   Sender
	emitter  EventEmitter

	mu    sync.Mutex
	peer  string
	muted bool
}

// NewVoiceService constrói um VoiceService.
func NewVoiceService(myUserID string, sender Sender, emitter EventEmitter) *VoiceService {
	return &VoiceService{myUserID: myUserID, sender: sender, emitter: emitter}
}

// SendSignal envia qualquer envelope voice_* pelo WebSocket.
// Chamado pelo frontend JS via app.SendVoiceSignal().
func (v *VoiceService) SendSignal(peerID, msgType, payload string) error {
	return v.sender.Send(ws.Envelope{
		Type:    msgType,
		From:    v.myUserID,
		To:      peerID,
		Payload: json.RawMessage(payload),
	})
}

// AcceptCall registra o peer atual (estado local apenas).
func (v *VoiceService) AcceptCall(peerID string) {
	v.mu.Lock()
	v.peer = peerID
	v.mu.Unlock()
}

// RejectCall envia rejeição ao peer.
func (v *VoiceService) RejectCall(peerID string) error {
	return v.SendSignal(peerID, "voice_reject", `{"reason":"declined"}`)
}

// Hangup encerra a chamada local e envia hangup ao peer.
func (v *VoiceService) Hangup() {
	v.mu.Lock()
	peer := v.peer
	v.peer = ""
	v.muted = false
	v.mu.Unlock()

	if peer != "" {
		_ = v.SendSignal(peer, "voice_hangup", `{}`)
		v.emitter.Emit("voice:ended", nil)
	}
}

// ---- Inbound handlers (chamados pelo dispatcher) -------------------------

// HandleOffer — recebe voice_offer; emite voice:incoming com peer + SDP para o JS.
func (v *VoiceService) HandleOffer(env ws.Envelope) {
	log.Printf("[voice] incoming call from %s", env.From)
	v.mu.Lock()
	v.peer = env.From
	v.mu.Unlock()
	v.emitter.Emit("voice:incoming", map[string]interface{}{
		"peer":    env.From,
		"payload": string(env.Payload),
	})
}

// HandleAnswer — recebe voice_answer; emite voice:answer com SDP para o JS.
func (v *VoiceService) HandleAnswer(env ws.Envelope) {
	v.mu.Lock()
	v.peer = env.From
	v.mu.Unlock()
	v.emitter.Emit("voice:answer", map[string]interface{}{
		"peer":    env.From,
		"payload": string(env.Payload),
	})
}

// HandleICE — recebe voice_ice; emite voice:ice com candidato para o JS.
func (v *VoiceService) HandleICE(env ws.Envelope) {
	v.emitter.Emit("voice:ice", map[string]interface{}{
		"peer":    env.From,
		"payload": string(env.Payload),
	})
}

// HandleReject — peer rejeitou a chamada.
func (v *VoiceService) HandleReject(env ws.Envelope) {
	log.Printf("[voice] rejected by %s", env.From)
	v.mu.Lock()
	v.peer = ""
	v.mu.Unlock()
	v.emitter.Emit("voice:rejected", env.From)
}

// HandleHangup — peer encerrou a chamada.
func (v *VoiceService) HandleHangup(env ws.Envelope) {
	log.Printf("[voice] hangup from %s", env.From)
	v.mu.Lock()
	v.peer = ""
	v.mu.Unlock()
	v.emitter.Emit("voice:ended", nil)
}
```

---

**Step 2: Atualizar métodos de voice em `app.go`**

Localizar o bloco `// ---- Voice Chat` (linhas ~118–143) e substituir por:

```go
// ---- Voice Chat ---------------------------------------------------------

// SendVoiceSignal envia qualquer sinal voice_* pelo WebSocket.
// Chamado pelo frontend JS para offer, answer, ICE, reject e hangup.
func (a *App) SendVoiceSignal(peerID, msgType, payload string) error {
	return a.voice.SendSignal(peerID, msgType, payload)
}

// AcceptVoiceCall registra o peer aceito (estado Go apenas).
func (a *App) AcceptVoiceCall(peerID string) {
	a.voice.AcceptCall(peerID)
}

// RejectVoiceCall envia rejeição ao peer.
func (a *App) RejectVoiceCall(peerID string) error {
	return a.voice.RejectCall(peerID)
}

// HangupVoice encerra a chamada ativa.
func (a *App) HangupVoice() {
	a.voice.Hangup()
}
```

> Remover: `StartVoiceCall`, `ToggleMute` — não são mais necessários.

---

**Step 3: Verificar**

```bash
go build ./...
```
Esperado: sem erros (o `webrtc` package não é mais importado por voice.go).

---

**Step 4: Commit**

```bash
git add service/voice.go app.go
git commit -m "refactor(voice): Go signaling-only, remove Pion WebRTC from voice service"
```

---

## Task 3: Voice Call — JS WebRTC

**Arquivos:**
- Modificar: `frontend/index.html`
- Modificar: `frontend/chat.js`

**Contexto:** Adicionar `VoiceWebRTC` class ao chat.js que gerencia todo o RTCPeerConnection. Reescrever os bus event handlers de voice para usar essa classe. Adicionar device selection (mic/speaker) ao painel de Configurações.

---

**Step 1: Adicionar elementos de HTML em `frontend/index.html`**

**1a. Elemento `<audio>` para playback de voz** — adicionar antes do `<!-- Toast -->` (linha ~552):

```html
  <!-- Áudio de voz (preenchido pelo JS WebRTC no ontrack) -->
  <audio id="voice-audio" autoplay hidden></audio>
```

**1b. Selects de microfone e saída no painel Configurações** — adicionar dentro de `<div class="panel-body">` em `#settings-panel`, após o último `<div class="field-group">` (slider de Tint do Vidro), antes da `<div class="field-note">`:

```html
      <div class="field-group">
        <label class="field-label" for="setting-mic">MICROFONE</label>
        <div class="select-wrap">
          <select id="setting-mic" class="field-select">
            <option value="">Padrão do sistema</option>
          </select>
          <svg class="select-chevron" viewBox="0 0 12 12" fill="none">
            <path d="M3 4.5l3 3 3-3" stroke="currentColor" stroke-width="1.3"
                  stroke-linecap="round" stroke-linejoin="round"/>
          </svg>
        </div>
      </div>

      <div class="field-group">
        <label class="field-label" for="setting-speaker">SAÍDA DE ÁUDIO</label>
        <div class="select-wrap">
          <select id="setting-speaker" class="field-select">
            <option value="">Padrão do sistema</option>
          </select>
          <svg class="select-chevron" viewBox="0 0 12 12" fill="none">
            <path d="M3 4.5l3 3 3-3" stroke="currentColor" stroke-width="1.3"
                  stroke-linecap="round" stroke-linejoin="round"/>
          </svg>
        </div>
      </div>
```

---

**Step 2: Adicionar classe `VoiceWebRTC` em `frontend/chat.js`**

Adicionar ANTES da linha `class WailsBridge {` (linha ~29):

```js
/* ─── VoiceWebRTC ────────────────────────────────────────────────── */
class VoiceWebRTC {
  #pc = null;
  #localStream = null;
  #sendSignal; // async (peerID, type, payload) => {}

  constructor(sendSignal) { this.#sendSignal = sendSignal; }

  async startCall(peerID) {
    const micId = document.getElementById('setting-mic')?.value || '';
    this.#localStream = await navigator.mediaDevices.getUserMedia({
      audio: micId ? { deviceId: { exact: micId } } : true,
    });
    this.#pc = this.#createPC(peerID);
    this.#localStream.getTracks().forEach(t => this.#pc.addTrack(t, this.#localStream));
    const offer = await this.#pc.createOffer();
    await this.#pc.setLocalDescription(offer);
    await this.#sendSignal(peerID, 'voice_offer',
      JSON.stringify({ sdp: offer.sdp, type: offer.type }));
  }

  async acceptCall(peerID, sdpPayload) {
    const { sdp, type } = JSON.parse(sdpPayload);
    const micId = document.getElementById('setting-mic')?.value || '';
    this.#localStream = await navigator.mediaDevices.getUserMedia({
      audio: micId ? { deviceId: { exact: micId } } : true,
    });
    this.#pc = this.#createPC(peerID);
    this.#localStream.getTracks().forEach(t => this.#pc.addTrack(t, this.#localStream));
    await this.#pc.setRemoteDescription({ type, sdp });
    const answer = await this.#pc.createAnswer();
    await this.#pc.setLocalDescription(answer);
    await this.#sendSignal(peerID, 'voice_answer',
      JSON.stringify({ sdp: answer.sdp, type: answer.type }));
  }

  async handleAnswer(sdpPayload) {
    if (!this.#pc) return;
    const { sdp, type } = JSON.parse(sdpPayload);
    await this.#pc.setRemoteDescription({ type, sdp });
  }

  async handleICE(candidatePayload) {
    if (!this.#pc) return;
    try {
      const c = JSON.parse(candidatePayload);
      if (c?.candidate) await this.#pc.addIceCandidate(c);
    } catch (e) { console.warn('[voice] ICE', e); }
  }

  toggleMute() {
    const tracks = this.#localStream?.getAudioTracks() ?? [];
    const muted = tracks[0] ? tracks[0].enabled : false; // enabled=true → not muted
    tracks.forEach(t => { t.enabled = !t.enabled; });
    return !muted; // retorna novo estado de muted
  }

  close() {
    this.#localStream?.getTracks().forEach(t => t.stop());
    this.#pc?.close();
    this.#pc = null;
    this.#localStream = null;
    const audio = document.getElementById('voice-audio');
    if (audio) audio.srcObject = null;
  }

  #createPC(peerID) {
    const pc = new RTCPeerConnection({
      iceServers: [{ urls: 'stun:stun.l.google.com:19302' }],
    });
    pc.onicecandidate = ({ candidate }) => {
      if (candidate) this.#sendSignal(peerID, 'voice_ice', JSON.stringify(candidate))
        .catch(console.warn);
    };
    pc.ontrack = ({ streams }) => {
      const audio = document.getElementById('voice-audio');
      if (!audio) return;
      audio.srcObject = streams[0];
      const speakerId = document.getElementById('setting-speaker')?.value || '';
      if (speakerId && audio.setSinkId) audio.setSinkId(speakerId).catch(console.warn);
    };
    return pc;
  }
}
```

---

**Step 3: Atualizar `WailsBridge` — adicionar `sendVoiceSignal`**

Localizar a classe `WailsBridge` (linha ~30). Adicionar método após os métodos de screenshare existentes:

```js
  async sendVoiceSignal(peerID, msgType, payload) {
    return window.go?.main?.App?.SendVoiceSignal?.(peerID, msgType, payload);
  }
```

---

**Step 4: Adicionar `#voiceWebRTC` em `UmbraApp` e reescrever handlers de voice**

Localizar a linha de declaração dos campos privados em `UmbraApp` (~linha 601):
```js
  #chatUI; #capsuleUI; #inviteUI; #screenShareUI; #voiceUI;
```
Adicionar `#voiceWebRTC`:
```js
  #chatUI; #capsuleUI; #inviteUI; #screenShareUI; #voiceUI; #voiceWebRTC;
```

No construtor de `UmbraApp` (onde `#voiceUI` é construído), adicionar logo após:
```js
    this.#voiceWebRTC = new VoiceWebRTC(
      (peerID, type, payload) => this.#bridge.sendVoiceSignal(peerID, type, payload)
    );
```

Localizar o bloco `// ── Voice bus events ──` (linhas ~736–762) e substituir por:

```js
    // ── Voice bus events ──
    this.#bus.on('voice:start', async peer => {
      try {
        await this.#voiceWebRTC.startCall(peer);
        this.#toast.show('Calling ' + peer + '…');
      } catch (e) { this.#toast.show('Call failed: ' + errMsg(e)); }
    });

    this.#bus.on('voice:accept', async ({ peer, sdpPayload }) => {
      try {
        this.#bridge.acceptVoiceCall(peer); // atualiza estado Go
        this.#voiceUI.showActive(peer);
        await this.#voiceWebRTC.acceptCall(peer, sdpPayload);
      } catch (e) {
        this.#voiceUI.hide();
        this.#toast.show('Call failed: ' + errMsg(e));
      }
    });

    this.#bus.on('voice:reject', async peer => {
      try { await this.#bridge.rejectVoiceCall(peer); }
      catch (e) { this.#toast.show('Reject failed: ' + errMsg(e)); }
    });

    this.#bus.on('voice:mute', () => {
      const muted = this.#voiceWebRTC.toggleMute();
      this.#voiceUI.toggleMuted(muted);
    });

    this.#bus.on('voice:hangup', async () => {
      try {
        this.#voiceWebRTC.close();
        await this.#bridge.hangupVoice();
        this.#voiceUI.hide();
      } catch (e) { this.#toast.show('Hangup failed'); }
    });
```

---

**Step 5: Atualizar `#bindWails` — eventos voice incluem SDP, adicionar voice:answer e voice:ice**

Localizar (linha ~828):
```js
    E('voice:incoming', peer => this.#voiceUI.showIncoming(peer));
    E('voice:connected', peer => this.#voiceUI.showActive(peer));
```

Substituir por:
```js
    E('voice:incoming', ({ peer, payload }) => {
      this.#voiceUI.showIncoming(peer, payload); // passa SDP para o modal
    });
    E('voice:answer', async ({ peer, payload }) => {
      this.#voiceUI.showActive(peer);
      await this.#voiceWebRTC.handleAnswer(payload).catch(e =>
        this.#toast.show('Answer error: ' + errMsg(e))
      );
    });
    E('voice:ice', async ({ payload }) => {
      await this.#voiceWebRTC.handleICE(payload).catch(console.warn);
    });
```

---

**Step 6: Atualizar `VoiceUI.showIncoming` para guardar o SDP e passá-lo ao aceitar**

Localizar a classe `VoiceUI` (linha ~412) e o método `showIncoming`:

```js
  showIncoming(peerID) {
    document.getElementById('voice-incoming-peer').textContent = peerID;
    this.#openModal('modal-voice-incoming');
  }
```

Substituir por (guarda o `sdpPayload` no elemento via `dataset`):

```js
  showIncoming(peerID, sdpPayload = '') {
    const el = document.getElementById('voice-incoming-peer');
    el.textContent = peerID;
    el.dataset.sdp = sdpPayload;
    this.#openModal('modal-voice-incoming');
  }
```

Localizar o listener de `voice-accept-btn` em `VoiceUI.#bind()`:
```js
    document.getElementById('voice-accept-btn')
      ?.addEventListener('click', () => {
        const peer = document.getElementById('voice-incoming-peer').textContent;
        this.#bus.emit('voice:accept', peer);
      });
```

Substituir por:
```js
    document.getElementById('voice-accept-btn')
      ?.addEventListener('click', () => {
        const el = document.getElementById('voice-incoming-peer');
        this.#bus.emit('voice:accept', { peer: el.textContent, sdpPayload: el.dataset.sdp ?? '' });
      });
```

---

**Step 7: Adicionar device enumeration em `SettingsUI`**

Adicionar método `#populateDevices()` na classe `SettingsUI`, logo após o construtor:

```js
  async #populateDevices() {
    try {
      // Precisa de permissão de áudio para ver labels
      await navigator.mediaDevices.getUserMedia({ audio: true }).then(s => s.getTracks().forEach(t => t.stop()));
    } catch { /* sem permissão — lista sem labels */ }
    const devices = await navigator.mediaDevices.enumerateDevices().catch(() => []);
    const micSel = document.getElementById('setting-mic');
    const spkSel = document.getElementById('setting-speaker');
    if (!micSel || !spkSel) return;

    const savedMic = this.#current.micDeviceId ?? '';
    const savedSpk = this.#current.speakerDeviceId ?? '';

    for (const d of devices) {
      if (d.kind === 'audioinput') {
        const opt = document.createElement('option');
        opt.value = d.deviceId;
        opt.textContent = d.label || `Microfone ${micSel.options.length}`;
        if (d.deviceId === savedMic) opt.selected = true;
        micSel.appendChild(opt);
      }
      if (d.kind === 'audiooutput') {
        const opt = document.createElement('option');
        opt.value = d.deviceId;
        opt.textContent = d.label || `Saída ${spkSel.options.length}`;
        if (d.deviceId === savedSpk) opt.selected = true;
        spkSel.appendChild(opt);
      }
    }
  }
```

Chamar `this.#populateDevices()` no final do construtor de `SettingsUI`.

Adicionar `micDeviceId: ''` e `speakerDeviceId: ''` ao `DEFAULTS`:
```js
  static DEFAULTS = { distortion: 50, nebula: 85, glass: 0, theme: 'dark', micDeviceId: '', speakerDeviceId: '' };
```

Adicionar ao final de `#syncSliders()`:
```js
    const mic = document.getElementById('setting-mic');
    const spk = document.getElementById('setting-speaker');
    if (mic) mic.value = this.#current.micDeviceId ?? '';
    if (spk) spk.value = this.#current.speakerDeviceId ?? '';
```

Adicionar ao final de `#bindDOM()` (antes do `}`), após o `settings-reset-btn` listener:
```js
    document.getElementById('setting-mic')?.addEventListener('change', e => {
      this.#current.micDeviceId = e.target.value;
      this.#save();
    });
    document.getElementById('setting-speaker')?.addEventListener('change', e => {
      this.#current.speakerDeviceId = e.target.value;
      this.#save();
    });
```

---

**Step 8: Verificar**

```bash
go build ./...
```
Esperado: sem erros.

```
wails dev
```

Checklist:
- Abrir Configurações → aparecem dois selects de áudio (Microfone / Saída)
- Selects populados com dispositivos (se permissão de mic concedida)
- Em dois dispositivos na mesma rede com `UMBRA_SERVER` apontando para o servidor: ligar para o outro → modal "Incoming Voice Call" aparece no callee com peer ID correto
- Aceitar → ambos mostram barra de chamada ativa; áudio audível nos dois lados

---

**Step 9: Commit**

```bash
git add frontend/index.html frontend/chat.js
git commit -m "feat(voice): JS WebRTC — getUserMedia, RTCPeerConnection, SDP signaling, device selection"
```

---

## Task 4: Screen Share — Go sinalização

**Arquivos:**
- Modificar: `service/screenshare.go`
- Modificar: `app.go`

**Contexto:** Mesmo padrão da Task 2 — Go vira sinalização pura. `ScreenShareService` para de usar Pion; handlers emitem SDP/ICE para JS via Wails events.

---

**Step 1: Reescrever `service/screenshare.go`**

Substituir o arquivo completo por:

```go
// Package service — screenshare.go
// ScreenShareService roteia mensagens de sinalização WS para screen share.
// WebRTC (getDisplayMedia, RTCPeerConnection) vive no frontend JS.
package service

import (
	"encoding/json"
	"log"
	"sync"

	"umbra/client/ws"
)

// ScreenShareService gerencia estado de sinalização de screen share.
type ScreenShareService struct {
	myUserID string
	sender   Sender
	emitter  EventEmitter

	mu   sync.Mutex
	peer string
}

// NewScreenShareService constrói um ScreenShareService.
func NewScreenShareService(myUserID string, sender Sender, emitter EventEmitter) *ScreenShareService {
	return &ScreenShareService{myUserID: myUserID, sender: sender, emitter: emitter}
}

// SendSignal envia qualquer envelope webrtc_* pelo WebSocket.
func (s *ScreenShareService) SendSignal(peerID, msgType, payload string) error {
	return s.sender.Send(ws.Envelope{
		Type:    msgType,
		From:    s.myUserID,
		To:      peerID,
		Payload: json.RawMessage(payload),
	})
}

// AcceptShare registra o peer (estado local).
func (s *ScreenShareService) AcceptShare(peerID string) {
	s.mu.Lock()
	s.peer = peerID
	s.mu.Unlock()
	s.emitter.Emit("screenshare:accepted", peerID)
	log.Printf("[screenshare] accepted share from %s", peerID)
}

// RejectShare envia rejeição ao peer.
func (s *ScreenShareService) RejectShare(peerID string) error {
	return s.SendSignal(peerID, "webrtc_reject", `{"reason":"declined"}`)
}

// StopShare encerra sessão e notifica o peer.
func (s *ScreenShareService) StopShare() {
	s.mu.Lock()
	peer := s.peer
	s.peer = ""
	s.mu.Unlock()

	if peer != "" {
		_ = s.SendSignal(peer, "webrtc_stop", `{}`)
	}
	s.emitter.Emit("screenshare:stopped", nil)
}

// ---- Inbound handlers ---------------------------------------------------

// HandleOffer — recebe webrtc_offer; emite screenshare:incoming com SDP para o JS.
func (s *ScreenShareService) HandleOffer(env ws.Envelope) {
	log.Printf("[screenshare] incoming offer from %s", env.From)
	s.mu.Lock()
	s.peer = env.From
	s.mu.Unlock()
	s.emitter.Emit("screenshare:incoming", map[string]interface{}{
		"peer":    env.From,
		"payload": string(env.Payload),
	})
}

// HandleAnswer — recebe webrtc_answer; emite screenshare:answer para o JS.
func (s *ScreenShareService) HandleAnswer(env ws.Envelope) {
	s.emitter.Emit("screenshare:answer", map[string]interface{}{
		"peer":    env.From,
		"payload": string(env.Payload),
	})
}

// HandleICE — recebe webrtc_ice; emite screenshare:ice para o JS.
func (s *ScreenShareService) HandleICE(env ws.Envelope) {
	s.emitter.Emit("screenshare:ice", map[string]interface{}{
		"peer":    env.From,
		"payload": string(env.Payload),
	})
}

// HandleReject — peer rejeitou o screen share.
func (s *ScreenShareService) HandleReject(env ws.Envelope) {
	log.Printf("[screenshare] rejected by %s", env.From)
	s.mu.Lock()
	s.peer = ""
	s.mu.Unlock()
	s.emitter.Emit("screenshare:rejected", env.From)
}

// HandleStop — peer encerrou o screen share.
func (s *ScreenShareService) HandleStop(env ws.Envelope) {
	log.Printf("[screenshare] stopped by %s", env.From)
	s.mu.Lock()
	s.peer = ""
	s.mu.Unlock()
	s.emitter.Emit("screenshare:stopped", nil)
}
```

---

**Step 2: Atualizar métodos de screen share em `app.go`**

Localizar o bloco `// ---- Screen Share` (linhas ~96–116) e substituir por:

```go
// ---- Screen Share -------------------------------------------------------

// SendScreenSignal envia qualquer sinal webrtc_* pelo WebSocket.
// Chamado pelo frontend JS para offer, answer, ICE e stop.
func (a *App) SendScreenSignal(peerID, msgType, payload string) error {
	return a.screenshare.SendSignal(peerID, msgType, payload)
}

// AcceptScreenShare aceita um screen share e registra o peer.
func (a *App) AcceptScreenShare(peerID string) {
	a.screenshare.AcceptShare(peerID)
}

// RejectScreenShare rejeita um screen share.
func (a *App) RejectScreenShare(peerID string) error {
	return a.screenshare.RejectShare(peerID)
}

// StopScreenShare encerra o screen share ativo.
func (a *App) StopScreenShare() {
	a.screenshare.StopShare()
}
```

> Remover: `StartScreenShare` — não é mais necessário.

---

**Step 3: Registrar dispatcher para `webrtc_stop`**

Procurar onde os handlers de screen share são registrados no dispatcher (provavelmente em `main.go` ou `app.go`, seção que chama `dispatcher.Register`):

```bash
grep -n "webrtc_offer\|webrtc_answer\|webrtc_ice\|webrtc_reject\|webrtc_stop\|Register" /c/Users/Matheus/Dev/Projects/go/umbra/client/main.go
```

Se `webrtc_stop` não estiver registrado, adicionar:
```go
d.Register("webrtc_stop", ws.MessageHandlerFunc(app.screenshare.HandleStop))
```

junto aos outros registros de `webrtc_*`.

---

**Step 4: Verificar**

```bash
go build ./...
```
Esperado: sem erros.

---

**Step 5: Commit**

```bash
git add service/screenshare.go app.go main.go
git commit -m "refactor(screenshare): Go signaling-only, remove Pion WebRTC from screenshare service"
```

---

## Task 5: Screen Share — JS WebRTC

**Arquivos:**
- Modificar: `frontend/index.html`
- Modificar: `frontend/chat.js`

**Contexto:** Adicionar `ScreenWebRTC` class e reescrever handlers de screenshare. Adicionar barra de sender ("compartilhando com...") ao HTML. Receiver usa `ontrack` para conectar stream no `<video>` diretamente.

---

**Step 1: Adicionar barra de sender em `frontend/index.html`**

Adicionar ANTES do `<!-- Screen share overlay -->` (linha ~515):

```html
  <!-- Screen share sender bar (visível apenas para quem está compartilhando) -->
  <div id="screenshare-sender-bar" class="hidden">
    <div class="screenshare-bar">
      <div class="section-title-row">
        <div class="section-accent"></div>
        <span class="label-xs" id="screenshare-sender-label">COMPARTILHANDO COM —</span>
      </div>
      <button class="ghost-btn" id="screenshare-stop-btn">PARAR</button>
    </div>
  </div>
```

---

**Step 2: Adicionar classe `ScreenWebRTC` em `frontend/chat.js`**

Adicionar APÓS a classe `VoiceWebRTC` e ANTES de `class WailsBridge`:

```js
/* ─── ScreenWebRTC ───────────────────────────────────────────────── */
class ScreenWebRTC {
  #pc = null;
  #displayStream = null;
  #sendSignal;

  constructor(sendSignal) { this.#sendSignal = sendSignal; }

  async startShare(peerID) {
    this.#displayStream = await navigator.mediaDevices.getDisplayMedia({
      video: true, audio: false,
    });
    this.#pc = this.#createPC(peerID);
    this.#displayStream.getTracks().forEach(t => this.#pc.addTrack(t, this.#displayStream));
    // Se o usuário fechar o picker nativo do browser, para automaticamente
    this.#displayStream.getVideoTracks()[0].onended = () => {
      this.stop();
      document.getElementById('screenshare-sender-bar')?.classList.add('hidden');
    };
    const offer = await this.#pc.createOffer();
    await this.#pc.setLocalDescription(offer);
    await this.#sendSignal(peerID, 'webrtc_offer',
      JSON.stringify({ sdp: offer.sdp, type: offer.type }));
  }

  async acceptShare(peerID, sdpPayload) {
    const { sdp, type } = JSON.parse(sdpPayload);
    this.#pc = this.#createPC(peerID);
    await this.#pc.setRemoteDescription({ type, sdp });
    const answer = await this.#pc.createAnswer();
    await this.#pc.setLocalDescription(answer);
    await this.#sendSignal(peerID, 'webrtc_answer',
      JSON.stringify({ sdp: answer.sdp, type: answer.type }));
  }

  async handleAnswer(sdpPayload) {
    if (!this.#pc) return;
    const { sdp, type } = JSON.parse(sdpPayload);
    await this.#pc.setRemoteDescription({ type, sdp });
  }

  async handleICE(candidatePayload) {
    if (!this.#pc) return;
    try {
      const c = JSON.parse(candidatePayload);
      if (c?.candidate) await this.#pc.addIceCandidate(c);
    } catch (e) { console.warn('[screenshare] ICE', e); }
  }

  stop() {
    this.#displayStream?.getTracks().forEach(t => t.stop());
    this.#pc?.close();
    this.#pc = null;
    this.#displayStream = null;
  }

  #createPC(peerID) {
    const pc = new RTCPeerConnection({
      iceServers: [{ urls: 'stun:stun.l.google.com:19302' }],
    });
    pc.onicecandidate = ({ candidate }) => {
      if (candidate) this.#sendSignal(peerID, 'webrtc_ice', JSON.stringify(candidate))
        .catch(console.warn);
    };
    pc.ontrack = ({ streams }) => {
      const video = document.getElementById('screenshare-video');
      if (video) video.srcObject = streams[0];
    };
    return pc;
  }
}
```

---

**Step 3: Adicionar `sendScreenSignal` ao `WailsBridge`**

```js
  async sendScreenSignal(peerID, msgType, payload) {
    return window.go?.main?.App?.SendScreenSignal?.(peerID, msgType, payload);
  }
```

---

**Step 4: Adicionar `#screenWebRTC` em `UmbraApp` e reescrever handlers de screenshare**

Na linha de campos privados, adicionar `#screenWebRTC`:
```js
  #chatUI; #capsuleUI; #inviteUI; #screenShareUI; #voiceUI; #voiceWebRTC; #screenWebRTC;
```

No construtor, após instanciar `#voiceWebRTC`:
```js
    this.#screenWebRTC = new ScreenWebRTC(
      (peerID, type, payload) => this.#bridge.sendScreenSignal(peerID, type, payload)
    );
```

Localizar o bloco `// ── Screen share bus events ──` (linhas ~711–734) e substituir por:

```js
    // ── Screen share bus events ──
    this.#bus.on('screenshare:show-confirm', peer => {
      this.#screenShareUI.showConfirm(peer);
    });

    this.#bus.on('screenshare:confirm', async peer => {
      try {
        await this.#screenWebRTC.startShare(peer);
        // Mostrar barra de sender
        document.getElementById('screenshare-sender-label').textContent =
          `COMPARTILHANDO COM ${peer}`;
        document.getElementById('screenshare-sender-bar')?.classList.remove('hidden');
      } catch (e) {
        this.#toast.show('Share failed: ' + errMsg(e));
      }
    });

    this.#bus.on('screenshare:accept', async ({ peer, sdpPayload }) => {
      try {
        this.#bridge.acceptScreenShare(peer);
        this.#screenShareUI.showReceiving(peer);
        await this.#screenWebRTC.acceptShare(peer, sdpPayload);
      } catch (e) { this.#toast.show('Accept failed: ' + errMsg(e)); }
    });

    this.#bus.on('screenshare:reject', async peer => {
      try { await this.#bridge.rejectScreenShare(peer); }
      catch (e) { this.#toast.show('Reject failed: ' + errMsg(e)); }
    });

    this.#bus.on('screenshare:stop', async () => {
      this.#screenWebRTC.stop();
      await this.#bridge.stopScreenShare();
      this.#screenShareUI.hide();
      document.getElementById('screenshare-sender-bar')?.classList.add('hidden');
    });
```

Adicionar listener do botão `screenshare-stop-btn` dentro de `#bindDOM()` (ou no construtor de `ScreenShareUI`):
```js
    document.getElementById('screenshare-stop-btn')
      ?.addEventListener('click', () => this.#bus.emit('screenshare:stop', null));
```

---

**Step 5: Atualizar `#bindWails` para screenshare — incluir SDP e novos eventos**

Localizar (linha ~819):
```js
    E('screenshare:incoming', peer => this.#screenShareUI.showIncoming(peer));
    E('screenshare:rejected', peer => { ... });
    E('screenshare:stopped', () => this.#screenShareUI.hide());
```

Substituir por:
```js
    E('screenshare:incoming', ({ peer, payload }) => {
      this.#screenShareUI.showIncoming(peer, payload);
    });
    E('screenshare:answer', async ({ payload }) => {
      await this.#screenWebRTC.handleAnswer(payload).catch(e =>
        this.#toast.show('Screen answer error: ' + errMsg(e))
      );
    });
    E('screenshare:ice', async ({ payload }) => {
      await this.#screenWebRTC.handleICE(payload).catch(console.warn);
    });
    E('screenshare:rejected', peer => {
      this.#screenShareUI.hide();
      this.#screenWebRTC.stop();
      document.getElementById('screenshare-sender-bar')?.classList.add('hidden');
      this.#toast.show(`${peer} declined screen share`);
    });
    E('screenshare:stopped', () => {
      this.#screenShareUI.hide();
      this.#screenWebRTC.stop();
      document.getElementById('screenshare-sender-bar')?.classList.add('hidden');
    });
```

**Atualizar `ScreenShareUI.showIncoming` para guardar SDP:**

```js
  showIncoming(peerID, sdpPayload = '') {
    const el = document.getElementById('ss-incoming-peer');
    if (el) { el.textContent = peerID; el.dataset.sdp = sdpPayload; }
    this.#openModal('modal-screenshare-incoming');
  }
```

**Atualizar o listener do botão "ACCEPT" em `ScreenShareUI.#bind()`:**

```js
    document.getElementById('ss-accept-btn')
      ?.addEventListener('click', () => {
        const el = document.getElementById('ss-incoming-peer');
        this.#bus.emit('screenshare:accept', {
          peer: el?.textContent ?? '',
          sdpPayload: el?.dataset?.sdp ?? '',
        });
        this.#closeModal();
      });
```

---

**Step 6: Verificar**

```bash
go build ./...
```

```
wails dev
```

Checklist:
- Sender clica no botão de screen share → modal de confirmação aparece → START SHARING → browser pede picker de janela/tela
- Depois de selecionar: barra "COMPARTILHANDO COM {peer}" aparece no topo do sender
- Receiver recebe modal de "screen share incoming" → ACCEPT → overlay com `<video>` abre e mostra tela do sender
- Botão PARAR (sender) ou END SESSION (receiver) encerra ambos os lados

---

**Step 7: Commit**

```bash
git add frontend/index.html frontend/chat.js
git commit -m "feat(screenshare): JS WebRTC — getDisplayMedia, RTCPeerConnection, sender bar indicator"
```

---

## Task 6: Capsule UX — abas, histórico e bolhas no chat

**Arquivos:**
- Modificar: `frontend/index.html`
- Modificar: `frontend/style.css`
- Modificar: `frontend/chat.js`

**Contexto:** Painel de cápsulas ganha 3 abas (NOVA / ENVIADAS / RECEBIDAS). Enviadas/recebidas ficam em listas na sessão. Quando uma cápsula é enviada, uma bolha especial aparece no chat daquele peer.

---

**Step 1: Reescrever o painel de cápsula em `frontend/index.html`**

Localizar `<div id="capsule-panel" class="side-panel hidden">` (linha ~303) e substituir todo o bloco até o `</div>` de fechamento do painel por:

```html
  <!-- ═══════════════ CAPSULE PANEL ═══════════════ -->
  <div id="capsule-panel" class="side-panel hidden">
    <div class="panel-header">
      <div class="section-title-row">
        <div class="section-accent"></div>
        <span class="label-xs">TIME CAPSULE</span>
      </div>
      <button class="icon-btn" id="capsule-close-btn">
        <svg viewBox="0 0 16 16" fill="none">
          <path d="M4 4l8 8M12 4l-8 8" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" />
        </svg>
      </button>
    </div>

    <!-- Tab bar -->
    <div class="capsule-tabs">
      <button class="capsule-tab active" data-tab="new">NOVA</button>
      <button class="capsule-tab" data-tab="sent">ENVIADAS</button>
      <button class="capsule-tab" data-tab="recv">RECEBIDAS</button>
    </div>

    <!-- Tab: NOVA (composer) -->
    <div id="capsule-tab-new" class="panel-body">
      <div class="field-group">
        <label class="field-label" for="capsule-text">MESSAGE</label>
        <textarea id="capsule-text" class="field-textarea" rows="6"
          placeholder="Write a message to deliver in the future..."></textarea>
      </div>
      <div class="field-group">
        <label class="field-label" for="capsule-delay">DELIVERY DELAY</label>
        <div class="select-wrap">
          <select id="capsule-delay" class="field-select">
            <option value="600">10 minutes</option>
            <option value="3600">1 hour</option>
            <option value="21600">6 hours</option>
            <option value="86400">24 hours</option>
          </select>
          <svg class="select-chevron" viewBox="0 0 12 12" fill="none">
            <path d="M3 4.5l3 3 3-3" stroke="currentColor" stroke-width="1.3" stroke-linecap="round"
              stroke-linejoin="round" />
          </svg>
        </div>
      </div>
      <div class="field-note">
        <svg viewBox="0 0 12 12" fill="none">
          <circle cx="6" cy="6" r="4.5" stroke="currentColor" stroke-width="1" />
          <path d="M6 4.5v3M6 9v.2" stroke="currentColor" stroke-width="1.2" stroke-linecap="round" />
        </svg>
        Message is encrypted before leaving this device. Server stores ciphertext only.
      </div>
      <button class="primary-btn" id="send-capsule-btn">SEND CAPSULE</button>
    </div>

    <!-- Tab: ENVIADAS -->
    <div id="capsule-tab-sent" class="panel-body hidden">
      <div class="capsule-list" id="capsule-sent-list">
        <p class="capsule-empty">Nenhuma cápsula enviada nesta sessão.</p>
      </div>
    </div>

    <!-- Tab: RECEBIDAS -->
    <div id="capsule-tab-recv" class="panel-body hidden">
      <div class="capsule-list" id="capsule-recv-list">
        <p class="capsule-empty">Nenhuma cápsula recebida nesta sessão.</p>
      </div>
    </div>
  </div>
```

---

**Step 2: Adicionar CSS em `frontend/style.css`**

Adicionar no final do arquivo:

```css
/* ─── Capsule tabs ─────────────────────────────────────────────────── */
.capsule-tabs {
  display: flex;
  border-bottom: 1px solid var(--glass-divider);
  padding: 0 16px;
  gap: 4px;
}

.capsule-tab {
  background: none;
  border: none;
  border-bottom: 2px solid transparent;
  color: var(--text-muted);
  cursor: pointer;
  font: var(--font-label-xs);
  letter-spacing: 0.08em;
  padding: 8px 10px 6px;
  transition: color 0.15s, border-color 0.15s;
}

.capsule-tab:hover { color: var(--text); }

.capsule-tab.active {
  border-bottom-color: var(--accent);
  color: var(--text);
}

/* ─── Capsule list (ENVIADAS / RECEBIDAS) ──────────────────────────── */
.capsule-list {
  display: flex;
  flex-direction: column;
  gap: 8px;
  padding: 4px 0;
}

.capsule-empty {
  color: var(--text-muted);
  font-size: 11px;
  padding: 16px 0;
  text-align: center;
}

.capsule-item {
  background: var(--glass-bg);
  border: 1px solid var(--glass-rim);
  border-radius: 8px;
  cursor: pointer;
  padding: 10px 12px;
  transition: background 0.15s;
}

.capsule-item:hover { background: var(--glass-bg-hover); }

.capsule-item-header {
  align-items: center;
  display: flex;
  gap: 6px;
  justify-content: space-between;
  margin-bottom: 4px;
}

.capsule-item-peer {
  color: var(--text);
  font-size: 11px;
  font-weight: 600;
  letter-spacing: 0.06em;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.capsule-item-status {
  color: var(--text-muted);
  flex-shrink: 0;
  font-size: 10px;
}

.capsule-item-status.delivered { color: var(--accent); }

.capsule-item-meta {
  color: var(--text-muted);
  font-size: 10px;
}

.capsule-item-preview {
  -webkit-box-orient: vertical;
  -webkit-line-clamp: 2;
  color: var(--text);
  display: -webkit-box;
  font-size: 11px;
  margin-top: 4px;
  overflow: hidden;
}

/* ─── Capsule bubble no chat ───────────────────────────────────────── */
.msg-bubble--capsule {
  align-items: center;
  background: transparent;
  border: 1px solid var(--glass-divider);
  color: var(--text-muted);
  display: flex;
  font-size: 11px;
  gap: 6px;
  letter-spacing: 0.04em;
  padding: 6px 10px;
}

.capsule-bubble-icon {
  flex-shrink: 0;
  font-size: 13px;
}

/* ─── Screenshare sender bar ────────────────────────────────────────── */
#screenshare-sender-bar {
  background: rgba(0, 0, 0, 0.7);
  backdrop-filter: blur(12px);
  -webkit-backdrop-filter: blur(12px);
  left: 0;
  padding: 8px 16px;
  position: fixed;
  right: 0;
  top: 0;
  z-index: 200;
}

#screenshare-sender-bar .screenshare-bar {
  align-items: center;
  display: flex;
  justify-content: space-between;
}
```

---

**Step 3: Adicionar `CapsuleStore` e atualizar `CapsuleUI` em `frontend/chat.js`**

**3a. Adicionar classe `CapsuleStore`** antes de `class CapsuleUI` (localizar pelo nome, ~linha 265):

```js
/* ─── CapsuleStore ───────────────────────────────────────────────── */
class CapsuleStore {
  #sent = new Map();  // id → { id, peer, delay, deliverAt, delivered }
  #recv = new Map();  // id → { id, from, text, receivedAt }

  addSent({ id, peer, delay }) {
    this.#sent.set(id, {
      id, peer, delay,
      deliverAt: Date.now() + delay * 1000,
      delivered: false,
    });
  }

  markDelivered(id) {
    const entry = this.#sent.get(id);
    if (entry) { entry.delivered = true; this.#sent.set(id, entry); }
  }

  addReceived({ id, from, text }) {
    this.#recv.set(id, { id, from, text, receivedAt: Date.now() });
  }

  getSent() { return [...this.#sent.values()]; }
  getReceived() { return [...this.#recv.values()]; }
}
```

**3b. Atualizar `CapsuleUI`** para suportar abas e listas.

Localizar a classe `CapsuleUI` e substituir todo o seu conteúdo por:

```js
class CapsuleUI {
  #bus;
  #store = new CapsuleStore();

  constructor(bus) {
    this.#bus = bus;
    this.#bind();
  }

  show() {
    document.getElementById('capsule-panel')?.classList.remove('hidden');
  }

  hide() {
    document.getElementById('capsule-panel')?.classList.add('hidden');
    document.getElementById('capsule-text').value = '';
  }

  showReceived({ id, from, text }) {
    this.#store.addReceived({ id, from, text });
    this.#renderRecvList();
    // Mostrar modal existente
    const body = document.querySelector('#modal-capsule-received .capsule-msg-body');
    const fromEl = document.querySelector('#modal-capsule-received .capsule-from-id');
    if (body) body.textContent = text;
    if (fromEl) fromEl.textContent = from;
    document.getElementById('modal-backdrop')?.classList.remove('hidden');
    document.getElementById('modal-capsule-received')?.classList.remove('hidden');
  }

  addSent({ id, peer, delay }) {
    this.#store.addSent({ id, peer, delay });
    this.#renderSentList();
  }

  markDelivered(id) {
    this.#store.markDelivered(id);
    this.#renderSentList();
    // Atualizar bolha no chat, se existir
    const bubble = document.getElementById(`capsule-bubble-${id}`);
    if (bubble) {
      bubble.querySelector('.capsule-bubble-icon').textContent = '✓';
      bubble.querySelector('.capsule-bubble-text').textContent = 'Cápsula entregue';
    }
  }

  #renderSentList() {
    const list = document.getElementById('capsule-sent-list');
    if (!list) return;
    const items = this.#store.getSent();
    if (!items.length) {
      list.innerHTML = '<p class="capsule-empty">Nenhuma cápsula enviada nesta sessão.</p>';
      return;
    }
    list.innerHTML = items.map(({ id, peer, deliverAt, delivered }) => {
      const status = delivered ? '✓ entregue' : `⧗ ${this.#formatDelay(deliverAt - Date.now())}`;
      const statusClass = delivered ? 'delivered' : '';
      return `
        <div class="capsule-item">
          <div class="capsule-item-header">
            <span class="capsule-item-peer">${escHTML(peer)}</span>
            <span class="capsule-item-status ${statusClass}">${status}</span>
          </div>
        </div>`;
    }).join('');
  }

  #renderRecvList() {
    const list = document.getElementById('capsule-recv-list');
    if (!list) return;
    const items = this.#store.getReceived();
    if (!items.length) {
      list.innerHTML = '<p class="capsule-empty">Nenhuma cápsula recebida nesta sessão.</p>';
      return;
    }
    list.innerHTML = items.map(({ id, from, text, receivedAt }) => `
      <div class="capsule-item">
        <div class="capsule-item-header">
          <span class="capsule-item-peer">${escHTML(from)}</span>
          <span class="capsule-item-meta">${new Date(receivedAt).toLocaleTimeString()}</span>
        </div>
        <div class="capsule-item-preview">${escHTML(text)}</div>
      </div>`
    ).join('');
  }

  #formatDelay(ms) {
    if (ms <= 0) return 'em breve';
    const s = Math.floor(ms / 1000);
    if (s < 60) return `${s}s`;
    if (s < 3600) return `${Math.floor(s / 60)}m`;
    if (s < 86400) return `${Math.floor(s / 3600)}h`;
    return `${Math.floor(s / 86400)}d`;
  }

  #bind() {
    document.getElementById('capsule-close-btn')
      ?.addEventListener('click', () => this.hide());

    // Troca de abas
    document.querySelectorAll('.capsule-tab').forEach(btn => {
      btn.addEventListener('click', () => {
        document.querySelectorAll('.capsule-tab').forEach(t => t.classList.remove('active'));
        btn.classList.add('active');
        const tab = btn.dataset.tab;
        ['new', 'sent', 'recv'].forEach(t => {
          document.getElementById(`capsule-tab-${t}`)
            ?.classList.toggle('hidden', t !== tab);
        });
        // Re-render ao abrir a aba
        if (tab === 'sent') this.#renderSentList();
        if (tab === 'recv') this.#renderRecvList();
      });
    });

    document.getElementById('send-capsule-btn')?.addEventListener('click', () => {
      const text = document.getElementById('capsule-text').value.trim();
      const delay = parseInt(document.getElementById('capsule-delay').value, 10);
      if (!text) return;
      this.#bus.emit('capsule:send', { text, delay });
      this.hide();
    });
  }
}
```

---

**Step 4: Adicionar `appendCapsuleBubble` ao `ChatUI`**

Localizar a classe `ChatUI` e, após o método `appendMessage`, adicionar:

```js
  appendCapsuleBubble(peer, capsuleId, delayLabel) {
    const active = this.#store.getActivePeer();
    if (peer !== active) return;
    const feed = document.getElementById('messages');
    const el = document.createElement('div');
    el.className = 'msg mine';
    el.id = `capsule-bubble-${capsuleId}`;
    el.innerHTML = `
      <div class="msg-bubble msg-bubble--capsule">
        <span class="capsule-bubble-icon">⧗</span>
        <span class="capsule-bubble-text">Cápsula enviada · entrega em ${escHTML(delayLabel)}</span>
      </div>`;
    feed.appendChild(el);
    feed.scrollTop = feed.scrollHeight;
  }
```

---

**Step 5: Ligar `CapsuleUI.addSent` e `markDelivered` ao fluxo existente em `UmbraApp`**

Localizar em `#bindBus()` (ou onde `capsule:send` é tratado), provavelmente:
```js
    this.#bus.on('capsule:send', async ({ text, delay }) => {
      ...
    });
```

Atualizar para registrar a cápsula enviada e adicionar bolha no chat:

```js
    this.#bus.on('capsule:send', async ({ text, delay }) => {
      const peer = this.#store.getActivePeer();
      if (!peer) return;
      try {
        await this.#bridge.sendCapsule(peer, text, delay);
        const id = `local-${Date.now()}`;
        const delayLabel = delay >= 86400 ? '24h'
          : delay >= 21600 ? '6h'
          : delay >= 3600  ? '1h'
          : '10min';
        this.#capsuleUI.addSent({ id, peer, delay });
        this.#chatUI.appendCapsuleBubble(peer, id, delayLabel);
      } catch (e) { this.#toast.show('Capsule failed: ' + errMsg(e)); }
    });
```

Localizar em `#bindWails()`:
```js
    E('capsule:ready', ({ id, sender_id }) => this.#toast.show(`Capsule ready from ${sender_id}`));
```

Substituir por:
```js
    E('capsule:ready', ({ id, sender_id }) => {
      this.#toast.show(`Capsule ready from ${sender_id}`);
      this.#capsuleUI.markDelivered(id);
    });
```

Localizar:
```js
    E('capsule:received', msg => this.#capsuleUI.showReceived(msg));
```
Este permanece igual — `showReceived` já foi atualizado para registrar na lista.

---

**Step 6: Verificar**

```bash
go build ./...
```

```
wails dev
```

Checklist:
- Abrir painel de cápsula → 3 abas visíveis (NOVA, ENVIADAS, RECEBIDAS)
- NOVA: composer funcionando, SEND CAPSULE fecha o painel
- Após enviar: bolha "⧗ Cápsula enviada · entrega em Xh" aparece no chat
- Aba ENVIADAS: item com peer e status "⧗ Xh"
- Quando servidor liberar a cápsula: status muda para "✓ entregue" e bolha no chat atualiza
- Cápsula recebida: modal abre + aba RECEBIDAS mostra o item com preview

---

**Step 7: Commit**

```bash
git add frontend/index.html frontend/style.css frontend/chat.js
git commit -m "feat(capsule): tabs NOVA/ENVIADAS/RECEBIDAS, sent tracking, chat bubbles"
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
- [ ] Distorção glass animada e visível (ondulação orgânica)
- [ ] Grain sutil na superfície dos painéis
- [ ] Slider de Distorção ainda funciona
- [ ] Voice call: modal aparece em ambos os dispositivos
- [ ] Voice call: áudio audível nos dois lados
- [ ] Voice call: selects de mic/saída em Configurações
- [ ] Screen share: sender mostra barra "COMPARTILHANDO COM" + botão PARAR
- [ ] Screen share: receiver mostra tela real (não preta) no overlay
- [ ] Screen share: parar no sender ou receiver encerra ambos
- [ ] Capsule: 3 abas funcionando, histórico de enviadas/recebidas
- [ ] Capsule: bolha no chat ao enviar
- [ ] Nenhuma regressão no chat, invite, onboarding

```bash
git log --oneline -8
```
Esperado: 6 commits limpos.
