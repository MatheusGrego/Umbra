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
