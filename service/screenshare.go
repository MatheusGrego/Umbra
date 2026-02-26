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
