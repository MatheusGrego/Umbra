// Package service — voice.go
// VoiceService manages WebRTC voice call lifecycle.
// It coordinates between the webrtc package, WS transport, and UI events.
package service

import (
	"encoding/json"
	"log"
	"sync"

	"umbra/client/webrtc"
	"umbra/client/ws"
)

// VoiceService manages voice call sessions.
type VoiceService struct {
	myUserID string
	sender   Sender
	emitter  EventEmitter

	mu      sync.Mutex
	session *webrtc.VoiceChat
	peer    string
	muted   bool
}

// NewVoiceService constructs a VoiceService.
func NewVoiceService(myUserID string, sender Sender, emitter EventEmitter) *VoiceService {
	return &VoiceService{
		myUserID: myUserID,
		sender:   sender,
		emitter:  emitter,
	}
}

// ---- Outbound actions (called from Wails UI) ----------------------------

// StartCall initiates a voice call offer to peerID.
func (v *VoiceService) StartCall(peerID string) error {
	v.mu.Lock()
	defer v.mu.Unlock()

	// Send offer signal to peer
	payload, _ := json.Marshal(map[string]string{"from": v.myUserID})
	return v.sender.Send(ws.Envelope{
		Type:    "voice_offer",
		From:    v.myUserID,
		To:      peerID,
		Payload: payload,
	})
}

// AcceptCall accepts an incoming voice call from peerID.
func (v *VoiceService) AcceptCall(peerID string) {
	v.mu.Lock()
	defer v.mu.Unlock()

	v.peer = peerID
	v.emitter.Emit("voice:connected", peerID)
	log.Printf("[voice] accepted call from %s", peerID)
}

// RejectCall sends a rejection to peerID.
func (v *VoiceService) RejectCall(peerID string) error {
	payload, _ := json.Marshal(map[string]string{"reason": "declined"})
	return v.sender.Send(ws.Envelope{
		Type:    "voice_reject",
		From:    v.myUserID,
		To:      peerID,
		Payload: payload,
	})
}

// Hangup tears down the active voice call.
func (v *VoiceService) Hangup() {
	v.mu.Lock()
	peer := v.peer
	session := v.session
	v.session = nil
	v.peer = ""
	v.muted = false
	v.mu.Unlock()

	if session != nil {
		_ = session.SendHangup()
		session.Close()
	}

	if peer != "" {
		v.emitter.Emit("voice:ended", nil)
	}
}

// ToggleMute toggles microphone mute state.
func (v *VoiceService) ToggleMute() bool {
	v.mu.Lock()
	v.muted = !v.muted
	muted := v.muted
	v.mu.Unlock()

	v.emitter.Emit("voice:muted", muted)
	return muted
}

// ---- Inbound handlers (called by dispatcher) ----------------------------

// HandleOffer processes an incoming voice_offer — shows confirmation UI.
func (v *VoiceService) HandleOffer(env ws.Envelope) {
	log.Printf("[voice] incoming call from %s", env.From)
	v.emitter.Emit("voice:incoming", env.From)
}

// HandleAnswer forwards a received SDP answer.
func (v *VoiceService) HandleAnswer(env ws.Envelope) {
	v.mu.Lock()
	ss := v.session
	v.mu.Unlock()

	if ss != nil {
		if err := ss.HandleAnswer(env); err != nil {
			log.Printf("[voice] answer error: %v", err)
		}
	}
}

// HandleICE forwards a received ICE candidate.
func (v *VoiceService) HandleICE(env ws.Envelope) {
	v.mu.Lock()
	ss := v.session
	v.mu.Unlock()

	if ss != nil {
		if err := ss.HandleICE(env); err != nil {
			log.Printf("[voice] ICE error: %v", err)
		}
	}
}

// HandleReject processes a voice call rejection.
func (v *VoiceService) HandleReject(env ws.Envelope) {
	log.Printf("[voice] rejected by %s", env.From)
	v.Hangup()
	v.emitter.Emit("voice:rejected", env.From)
}

// HandleHangup processes a remote hangup.
func (v *VoiceService) HandleHangup(env ws.Envelope) {
	log.Printf("[voice] hangup from %s", env.From)
	v.Hangup()
}
