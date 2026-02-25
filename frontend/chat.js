/**
 * chat.js — Umbra frontend
 *
 * Modules:
 *   EventBus      — decoupled pub/sub
 *   WailsBridge   — all Go↔JS calls isolated here
 *   StateStore    — single source of truth
 *   ChatUI        — message rendering + peer list
 *   CapsuleUI     — capsule panel
 *   InviteUI      — invite modals
 *   ScreenShareUI — share overlay
 *   Toast         — notifications
 *   App           — wires everything, handles Wails events
 */

/* ─── EventBus ───────────────────────────────────────────────────────── */
class EventBus {
  #listeners = new Map();
  on(event, fn) {
    if (!this.#listeners.has(event)) this.#listeners.set(event, []);
    this.#listeners.get(event).push(fn);
  }
  emit(event, data) { this.#listeners.get(event)?.forEach(fn => fn(data)); }
}

/* ─── Helpers ────────────────────────────────────────────────────────── */
const errMsg = e => (e instanceof Error ? e.message : String(e)) || 'unknown error';

/* ─── WailsBridge ────────────────────────────────────────────────────── */
class WailsBridge {
  async getMyID() { return window.go?.main?.App?.GetMyID?.() ?? 'dev-0000000'; }
  async getOnlinePeers() { return window.go?.main?.App?.GetOnlinePeers?.() ?? []; }
  async getAllPeers() { return window.go?.main?.App?.GetAllPeers?.() ?? []; }
  async setNickname(userID, nickname) { return window.go?.main?.App?.SetNickname?.(userID, nickname); }
  async sendMessage(to, text) { return window.go?.main?.App?.SendMessage?.(to, text); }
  async sendCapsule(to, text, secs) { return window.go?.main?.App?.SendCapsule?.(to, text, secs); }
  async createInvite() { return window.go?.main?.App?.CreateInvite?.(); }
  async resolveInvite(token) { return window.go?.main?.App?.ResolveInvite?.(token); }
  // Screen share
  async startScreenShare(peer) { return window.go?.main?.App?.StartScreenShare?.(peer); }
  async acceptScreenShare(peer) { return window.go?.main?.App?.AcceptScreenShare?.(peer); }
  async rejectScreenShare(peer) { return window.go?.main?.App?.RejectScreenShare?.(peer); }
  async stopScreenShare() { return window.go?.main?.App?.StopScreenShare?.(); }
  // Voice
  async startVoiceCall(peer) { return window.go?.main?.App?.StartVoiceCall?.(peer); }
  async acceptVoiceCall(peer) { return window.go?.main?.App?.AcceptVoiceCall?.(peer); }
  async rejectVoiceCall(peer) { return window.go?.main?.App?.RejectVoiceCall?.(peer); }
  async hangupVoice() { return window.go?.main?.App?.HangupVoice?.(); }
  async toggleMute() { return window.go?.main?.App?.ToggleMute?.(); }
}

/* ─── StateStore ─────────────────────────────────────────────────────── */
class StateStore {
  #myID = null;
  #activePeer = null;
  #peers = new Map();   // id → { id, online }
  #messages = new Map();   // peerID → Message[]

  setMyID(id) { this.#myID = id; }
  getMyID() { return this.#myID; }
  setActivePeer(id) { this.#activePeer = id; }
  getActivePeer() { return this.#activePeer; }
  upsertPeer(id, patch = {}) {
    this.#peers.set(id, { id, online: false, ...this.#peers.get(id), ...patch });
  }
  getPeer(id) { return this.#peers.get(id); }
  getAllPeers() { return [...this.#peers.values()]; }
  pushMessage(pid, msg) {
    if (!this.#messages.has(pid)) this.#messages.set(pid, []);
    this.#messages.get(pid).push(msg);
  }
  getMessages(pid) { return this.#messages.get(pid) ?? []; }
  getDisplayName(id) {
    const p = this.#peers.get(id);
    return (p?.nickname && p.nickname.trim()) ? p.nickname : id;
  }
}

/* ─── Toast ──────────────────────────────────────────────────────────── */
class Toast {
  #el = document.getElementById('toast-container');
  show(text, ms = 3200) {
    const t = document.createElement('div');
    t.className = 'toast';
    t.textContent = text;
    this.#el.appendChild(t);
    setTimeout(() => t.remove(), ms);
  }
}

/* ─── ChatUI ─────────────────────────────────────────────────────────── */
class ChatUI {
  #store; #bus;
  constructor(store, bus) { this.#store = store; this.#bus = bus; this.#bindDOM(); this.#bindNicknameEdit(); }

  renderPeerList() {
    const ul = document.getElementById('peer-list');
    ul.innerHTML = '';
    const active = this.#store.getActivePeer();
    for (const p of this.#store.getAllPeers()) {
      const li = document.createElement('li');
      li.className = 'peer-item' + (p.id === active ? ' active' : '');
      li.setAttribute('role', 'option');
      li.dataset.id = p.id;
      const name = this.#store.getDisplayName(p.id);
      li.innerHTML = `
        <div class="peer-avatar${!p.hasSession ? ' no-session-avatar' : ''}">${name[0].toUpperCase()}</div>
        <div class="peer-info">
          <div class="peer-name">${name}</div>
          <div class="peer-id-short">${p.id}</div>
        </div>
        <span class="status-dot${p.online ? ' online' : ''}"></span>
      `;
      if (!p.hasSession) li.classList.add('no-session');
      li.addEventListener('click', () => this.#bus.emit('peer:selected', p.id));
      ul.appendChild(li);
    }
  }

  openChat(peerID) {
    const p = this.#store.getPeer(peerID);
    const name = this.#store.getDisplayName(peerID);
    document.getElementById('empty-state').classList.add('hidden');
    document.getElementById('onboarding-state')?.classList.add('hidden');
    const cv = document.getElementById('chat-view');
    cv.classList.remove('hidden');
    document.getElementById('chat-peer-name').textContent = name;
    document.getElementById('chat-avatar').textContent = name[0].toUpperCase();
    this.#setStatus(p?.online ?? false);
    this.renderMessages(peerID);
    document.getElementById('message-input').focus();
  }

  renderMessages(peerID) {
    const feed = document.getElementById('messages');
    feed.innerHTML = '';
    const myID = this.#store.getMyID();
    for (const m of this.#store.getMessages(peerID)) {
      feed.appendChild(this.#bubble(m, myID));
    }
    feed.scrollTop = feed.scrollHeight;
  }

  appendMessage(msg) {
    const active = this.#store.getActivePeer();
    const peer = msg.mine ? msg.to : msg.from;
    if (peer !== active) return;
    const feed = document.getElementById('messages');
    feed.appendChild(this.#bubble(msg, this.#store.getMyID()));
    const near = feed.scrollHeight - feed.scrollTop - feed.clientHeight < 140;
    if (near) feed.scrollTop = feed.scrollHeight;
  }

  updatePeerStatus(id, online) {
    this.renderPeerList();
    if (this.#store.getActivePeer() === id) this.#setStatus(online);
  }

  #setStatus(online) {
    const dot = document.getElementById('chat-status-dot');
    const lbl = document.getElementById('chat-peer-status');
    dot.className = 'status-dot' + (online ? ' online' : '');
    lbl.textContent = online ? 'ONLINE' : 'OFFLINE';
  }

  #bubble(msg, myID) {
    const mine = msg.from === myID;
    const div = document.createElement('div');
    div.className = 'msg ' + (mine ? 'mine' : 'theirs');
    div.innerHTML = `
      <div class="msg-bubble">${escHTML(msg.text)}</div>
      <div class="msg-meta">${fmtTime(msg.ts)}</div>
    `;
    return div;
  }

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

  #bindDOM() {
    const form = document.getElementById('message-form');
    const input = document.getElementById('message-input');
    form.addEventListener('submit', e => {
      e.preventDefault();
      const text = input.value.trim();
      if (!text) return;
      this.#bus.emit('msg:send', text);
      input.value = '';
    });
    input.addEventListener('keydown', e => {
      if (e.key === 'Enter' && !e.shiftKey) { e.preventDefault(); form.requestSubmit(); }
    });
    // Screen share button → open sender confirmation modal
    document.getElementById('screenshare-btn')
      ?.addEventListener('click', () => {
        const peer = this.#store.getActivePeer();
        if (peer) this.#bus.emit('screenshare:show-confirm', peer);
      });
    // Voice call button → start call
    document.getElementById('voicecall-btn')
      ?.addEventListener('click', () => {
        const peer = this.#store.getActivePeer();
        if (peer) this.#bus.emit('voice:start', peer);
      });
  }
}

/* ─── CapsuleUI ──────────────────────────────────────────────────────── */
class CapsuleUI {
  #bus;
  constructor(bus) { this.#bus = bus; this.#bindDOM(); }

  show() { document.getElementById('capsule-panel').classList.remove('hidden'); }
  hide() {
    document.getElementById('capsule-panel').classList.add('hidden');
    document.getElementById('capsule-text').value = '';
  }
  showReceived({ id, from, text }) {
    this.#openModal('modal-capsule-received');
    document.querySelector('.capsule-from-id').textContent = from;
    document.querySelector('.capsule-msg-body').textContent = text;
  }

  #bindDOM() {
    document.getElementById('capsule-open-btn')
      ?.addEventListener('click', () => this.show());
    document.getElementById('capsule-close-btn')
      ?.addEventListener('click', () => this.hide());
    document.getElementById('send-capsule-btn')
      ?.addEventListener('click', () => {
        const text = document.getElementById('capsule-text').value.trim();
        const delay = parseInt(document.getElementById('capsule-delay').value, 10);
        if (!text) return;
        this.#bus.emit('capsule:send', { text, delay });
        this.hide();
      });
  }
  #openModal(id) {
    document.querySelectorAll('.modal-view').forEach(v => v.classList.add('hidden'));
    document.getElementById(id).classList.remove('hidden');
    document.getElementById('modal-backdrop').classList.remove('hidden');
  }
}

/* ─── InviteUI ───────────────────────────────────────────────────────── */
class InviteUI {
  #bus;
  constructor(bus) { this.#bus = bus; this.#bindDOM(); }

  showToken({ token }) {
    this.#openModal('modal-invite-show');
    document.getElementById('invite-token-display').textContent = token;
  }
  showResolveForm() {
    this.#openModal('modal-invite-resolve');
    document.getElementById('invite-code-input').value = '';
    setTimeout(() => document.getElementById('invite-code-input').focus(), 50);
  }
  close() {
    document.getElementById('modal-backdrop').classList.add('hidden');
    document.querySelectorAll('.modal-view').forEach(v => v.classList.add('hidden'));
  }

  #openModal(id) {
    document.querySelectorAll('.modal-view').forEach(v => v.classList.add('hidden'));
    document.getElementById(id).classList.remove('hidden');
    document.getElementById('modal-backdrop').classList.remove('hidden');
  }
  #bindDOM() {
    document.getElementById('invite-btn')
      ?.addEventListener('click', () => this.#bus.emit('invite:create', null));
    document.getElementById('resolve-invite-btn')
      ?.addEventListener('click', () => this.showResolveForm());
    document.getElementById('submit-invite-btn')
      ?.addEventListener('click', () => {
        const tok = document.getElementById('invite-code-input').value.trim();
        if (!tok) return;
        this.#bus.emit('invite:resolve', tok);
        this.close();
      });
    document.getElementById('copy-token-btn')
      ?.addEventListener('click', () => {
        const tok = document.getElementById('invite-token-display').textContent;
        navigator.clipboard.writeText(tok);
        this.#bus.emit('toast', 'Code copied to clipboard');
      });
    document.querySelectorAll('.modal-close-btn')
      .forEach(btn => btn.addEventListener('click', () => this.close()));
    document.getElementById('modal-backdrop')
      ?.addEventListener('click', e => { if (e.target.id === 'modal-backdrop') this.close(); });
  }
}

/* ─── ScreenShareUI ──────────────────────────────────────────────────── */
class ScreenShareUI {
  #bus;
  constructor(bus) { this.#bus = bus; this.#bindDOM(); }

  showConfirm(peerID) {
    document.getElementById('ss-confirm-peer').textContent = peerID;
    this.#openModal('modal-screenshare-confirm');
  }

  showIncoming(peerID) {
    document.getElementById('ss-incoming-peer').textContent = peerID;
    this.#openModal('modal-screenshare-incoming');
  }

  showReceiving(peerID) {
    document.getElementById('screenshare-label').textContent = `SCREEN SHARE — ${peerID}`;
    document.getElementById('screenshare-overlay').classList.remove('hidden');
  }
  attachStream(stream) { document.getElementById('screenshare-video').srcObject = stream; }
  hide() {
    document.getElementById('screenshare-overlay').classList.add('hidden');
    const v = document.getElementById('screenshare-video');
    v.srcObject = null;
  }

  #openModal(viewId) {
    document.querySelectorAll('.modal-view').forEach(v => v.classList.add('hidden'));
    document.getElementById(viewId).classList.remove('hidden');
    document.getElementById('modal-backdrop').classList.remove('hidden');
  }

  #bindDOM() {
    document.getElementById('stop-screenshare-btn')
      ?.addEventListener('click', () => this.#bus.emit('screenshare:stop', null));

    // Sender confirm modal
    document.getElementById('ss-confirm-yes-btn')
      ?.addEventListener('click', () => {
        const peer = document.getElementById('ss-confirm-peer').textContent;
        this.#bus.emit('screenshare:confirm', peer);
        document.getElementById('modal-backdrop').classList.add('hidden');
      });

    // Receiver accept/reject
    document.getElementById('ss-accept-btn')
      ?.addEventListener('click', () => {
        const peer = document.getElementById('ss-incoming-peer').textContent;
        this.#bus.emit('screenshare:accept', peer);
        document.getElementById('modal-backdrop').classList.add('hidden');
      });
    document.getElementById('ss-reject-btn')
      ?.addEventListener('click', () => {
        const peer = document.getElementById('ss-incoming-peer').textContent;
        this.#bus.emit('screenshare:reject', peer);
        document.getElementById('modal-backdrop').classList.add('hidden');
      });
  }
}

/* ─── VoiceUI ────────────────────────────────────────────────────────── */
class VoiceUI {
  #bus;
  #timerInterval = null;
  #startTime = 0;
  constructor(bus) { this.#bus = bus; this.#bindDOM(); }

  showIncoming(peerID) {
    document.getElementById('voice-incoming-peer').textContent = peerID;
    document.querySelectorAll('.modal-view').forEach(v => v.classList.add('hidden'));
    document.getElementById('modal-voice-incoming').classList.remove('hidden');
    document.getElementById('modal-backdrop').classList.remove('hidden');
  }

  showActive(peerID) {
    document.getElementById('voice-bar-label').textContent = `VOICE CALL — ${peerID}`;
    document.getElementById('voice-call-bar').classList.remove('hidden');
    this.#startTime = Date.now();
    this.#timerInterval = setInterval(() => this.#updateTimer(), 1000);
  }

  hide() {
    document.getElementById('voice-call-bar').classList.add('hidden');
    document.getElementById('voice-call-bar').classList.remove('voice-muted');
    if (this.#timerInterval) clearInterval(this.#timerInterval);
    document.getElementById('voice-bar-timer').textContent = '00:00';
  }

  toggleMuted(muted) {
    document.getElementById('voice-call-bar').classList.toggle('voice-muted', muted);
  }

  #updateTimer() {
    const s = Math.floor((Date.now() - this.#startTime) / 1000);
    const m = String(Math.floor(s / 60)).padStart(2, '0');
    const sec = String(s % 60).padStart(2, '0');
    document.getElementById('voice-bar-timer').textContent = `${m}:${sec}`;
  }

  #bindDOM() {
    document.getElementById('voice-accept-btn')
      ?.addEventListener('click', () => {
        const peer = document.getElementById('voice-incoming-peer').textContent;
        this.#bus.emit('voice:accept', peer);
        document.getElementById('modal-backdrop').classList.add('hidden');
      });
    document.getElementById('voice-reject-btn')
      ?.addEventListener('click', () => {
        const peer = document.getElementById('voice-incoming-peer').textContent;
        this.#bus.emit('voice:reject', peer);
        document.getElementById('modal-backdrop').classList.add('hidden');
      });
    document.getElementById('voice-mute-btn')
      ?.addEventListener('click', () => this.#bus.emit('voice:mute', null));
    document.getElementById('voice-hangup-btn')
      ?.addEventListener('click', () => this.#bus.emit('voice:hangup', null));
  }
}

/* ─── App ────────────────────────────────────────────────────────────── */
class App {
  #bridge = new WailsBridge();
  #bus = new EventBus();
  #store = new StateStore();
  #toast = new Toast();
  #chatUI; #capsuleUI; #inviteUI; #screenShareUI; #voiceUI;

  async init() {
    this.#chatUI = new ChatUI(this.#store, this.#bus);
    this.#capsuleUI = new CapsuleUI(this.#bus);
    this.#inviteUI = new InviteUI(this.#bus);
    this.#screenShareUI = new ScreenShareUI(this.#bus);
    this.#voiceUI = new VoiceUI(this.#bus);
    this.#bindBus();
    this.#bindWails();
    await this.#bootstrap();
  }

  async #bootstrap() {
    const myID = await this.#bridge.getMyID();
    this.#store.setMyID(myID);
    document.getElementById('my-id-display').textContent = myID;
    document.getElementById('copy-id-btn')?.addEventListener('click', () => {
      navigator.clipboard.writeText(myID);
      this.#toast.show('Address copied');
    });
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
  }

  #bindBus() {
    this.#bus.on('peer:selected', id => {
      this.#store.setActivePeer(id);
      this.#chatUI.openChat(id);
      this.#chatUI.renderPeerList();
    });

    this.#bus.on('msg:send', async text => {
      const to = this.#store.getActivePeer();
      if (!to) return;
      try {
        await this.#bridge.sendMessage(to, text);
        const m = { from: this.#store.getMyID(), to, text, ts: Date.now(), mine: true };
        this.#store.pushMessage(to, m);
        this.#chatUI.appendMessage(m);
      } catch (e) { this.#toast.show('Send failed: ' + errMsg(e)); }
    });

    this.#bus.on('capsule:send', async ({ text, delay }) => {
      const to = this.#store.getActivePeer();
      if (!to) return;
      try {
        await this.#bridge.sendCapsule(to, text, delay);
        this.#toast.show('Capsule scheduled');
      } catch (e) { this.#toast.show('Capsule failed: ' + errMsg(e)); }
    });

    this.#bus.on('invite:create', async () => {
      try { await this.#bridge.createInvite(); }
      catch (e) { this.#toast.show('Error: ' + errMsg(e)); }
    });

    this.#bus.on('invite:resolve', async tok => {
      try {
        await this.#bridge.resolveInvite(tok);
        this.#toast.show('Connecting...');
      } catch (e) { this.#toast.show('Invalid code'); }
    });

    // ── Screen share bus events ──
    this.#bus.on('screenshare:show-confirm', peer => {
      this.#screenShareUI.showConfirm(peer);
    });
    this.#bus.on('screenshare:stop', async () => {
      await this.#bridge.stopScreenShare();
      this.#screenShareUI.hide();
    });
    this.#bus.on('screenshare:confirm', async peer => {
      try {
        await this.#bridge.startScreenShare(peer);
        this.#toast.show('Screen share offer sent');
      } catch (e) { this.#toast.show('Share failed: ' + errMsg(e)); }
    });
    this.#bus.on('screenshare:accept', async peer => {
      try {
        await this.#bridge.acceptScreenShare(peer);
        this.#screenShareUI.showReceiving(peer);
      } catch (e) { this.#toast.show('Accept failed: ' + errMsg(e)); }
    });
    this.#bus.on('screenshare:reject', async peer => {
      try { await this.#bridge.rejectScreenShare(peer); }
      catch (e) { this.#toast.show('Reject failed: ' + errMsg(e)); }
    });

    // ── Voice bus events ──
    this.#bus.on('voice:start', async peer => {
      try {
        await this.#bridge.startVoiceCall(peer);
        this.#toast.show('Calling ' + peer + '…');
      } catch (e) { this.#toast.show('Call failed: ' + errMsg(e)); }
    });
    this.#bus.on('voice:accept', async peer => {
      try {
        await this.#bridge.acceptVoiceCall(peer);
        this.#voiceUI.showActive(peer);
      } catch (e) { this.#toast.show('Call failed: ' + errMsg(e)); }
    });
    this.#bus.on('voice:reject', async peer => {
      try { await this.#bridge.rejectVoiceCall(peer); }
      catch (e) { this.#toast.show('Reject failed: ' + errMsg(e)); }
    });
    this.#bus.on('voice:mute', async () => {
      try { await this.#bridge.toggleMute(); }
      catch (e) { this.#toast.show('Mute failed'); }
    });
    this.#bus.on('voice:hangup', async () => {
      try {
        await this.#bridge.hangupVoice();
        this.#voiceUI.hide();
      } catch (e) { this.#toast.show('Hangup failed'); }
    });

    this.#bus.on('nickname:save', async ({ userID, nickname }) => {
      try {
        await this.#bridge.setNickname(userID, nickname);
        this.#store.upsertPeer(userID, { nickname });
        this.#chatUI.renderPeerList();
      } catch (e) {
        this.#toast.show('Falha ao salvar apelido');
      }
    });

    this.#bus.on('toast', msg => this.#toast.show(msg));
  }

  #bindWails() {
    const E = window.runtime?.EventsOn;
    if (!E) { console.warn('[umbra] Wails runtime not found — dev mode'); return; }

    E('chat:message', msg => {
      this.#store.upsertPeer(msg.from);
      this.#store.pushMessage(msg.from, msg);
      this.#chatUI.appendMessage(msg);
    });

    E('presence:online_list', ids => {
      for (const id of ids) this.#store.upsertPeer(id, { online: true });
      this.#chatUI.renderPeerList();
    });

    E('presence:online', id => {
      this.#store.upsertPeer(id, { online: true });
      this.#chatUI.updatePeerStatus(id, true);
    });

    E('presence:offline', id => {
      this.#store.upsertPeer(id, { online: false });
      this.#chatUI.updatePeerStatus(id, false);
    });

    E('invite:token', data => this.#inviteUI.showToken(data));
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

    E('capsule:ready', ({ id, sender_id }) => this.#toast.show(`Capsule ready from ${sender_id}`));
    E('capsule:received', msg => this.#capsuleUI.showReceived(msg));

    // ── Screen share Wails events ──
    E('screenshare:incoming', peer => this.#screenShareUI.showIncoming(peer));
    E('screenshare:rejected', peer => {
      this.#screenShareUI.hide();
      this.#toast.show(`${peer} declined screen share`);
    });
    E('screenshare:stopped', () => this.#screenShareUI.hide());

    // ── Voice Wails events ──
    E('voice:incoming', peer => this.#voiceUI.showIncoming(peer));
    E('voice:connected', peer => this.#voiceUI.showActive(peer));
    E('voice:rejected', peer => {
      this.#voiceUI.hide();
      this.#toast.show(`${peer} declined voice call`);
    });
    E('voice:ended', () => this.#voiceUI.hide());
    E('voice:muted', muted => this.#voiceUI.toggleMuted(muted));
  }
}

/* ─── Helpers ────────────────────────────────────────────────────────── */
function escHTML(s) {
  return s.replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;').replace(/"/g, '&quot;');
}
function fmtTime(ts) {
  return new Date(ts).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' });
}

/* ─── Boot ───────────────────────────────────────────────────────────── */
window.addEventListener('DOMContentLoaded', () => new App().init());
