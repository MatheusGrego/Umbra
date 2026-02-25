// Package service — screenshare.go
// ScreenShareService manages WebRTC screen-sharing lifecycle.
// It coordinates between the webrtc package, WS transport, and UI events.
package service

import (
	"encoding/json"
	"log"
	"sync"

	"umbra/client/webrtc"
	"umbra/client/ws"
)

// ScreenShareService manages screen sharing sessions.
type ScreenShareService struct {
	myUserID string
	sender   Sender
	emitter  EventEmitter

	mu      sync.Mutex
	session *webrtc.ScreenShare
	peer    string // current peer being shared with/from
}

// NewScreenShareService constructs a ScreenShareService.
func NewScreenShareService(myUserID string, sender Sender, emitter EventEmitter) *ScreenShareService {
	return &ScreenShareService{
		myUserID: myUserID,
		sender:   sender,
		emitter:  emitter,
	}
}

// ---- Outbound actions (called from Wails UI) ----------------------------

// StartShare initiates a screen share offer to peerID.
// The actual media capture happens via the WebRTC getDisplayMedia API on the frontend;
// here we just set up the signalling and emit the event to prompt the receiver.
func (s *ScreenShareService) StartShare(peerID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Send offer signal to peer — the frontend will trigger getDisplayMedia
	payload, _ := json.Marshal(map[string]string{"from": s.myUserID})
	return s.sender.Send(ws.Envelope{
		Type:    "webrtc_offer",
		From:    s.myUserID,
		To:      peerID,
		Payload: payload,
	})
}

// AcceptShare accepts an incoming screen share offer from peerID.
func (s *ScreenShareService) AcceptShare(peerID string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.peer = peerID
	s.emitter.Emit("screenshare:accepted", peerID)
	log.Printf("[screenshare] accepted share from %s", peerID)
}

// RejectShare sends a rejection to peerID.
func (s *ScreenShareService) RejectShare(peerID string) error {
	payload, _ := json.Marshal(map[string]string{"reason": "declined"})
	return s.sender.Send(ws.Envelope{
		Type:    "webrtc_reject",
		From:    s.myUserID,
		To:      peerID,
		Payload: payload,
	})
}

// StopShare tears down the active screen share session.
func (s *ScreenShareService) StopShare() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.session != nil {
		s.session.Close()
		s.session = nil
	}
	s.peer = ""
	s.emitter.Emit("screenshare:stopped", nil)
}

// ---- Inbound handlers (called by dispatcher) ----------------------------

// HandleOffer processes an incoming webrtc_offer — shows confirmation UI.
func (s *ScreenShareService) HandleOffer(env ws.Envelope) {
	log.Printf("[screenshare] incoming offer from %s", env.From)
	s.emitter.Emit("screenshare:incoming", env.From)
}

// HandleAnswer forwards a received SDP answer to the active session.
func (s *ScreenShareService) HandleAnswer(env ws.Envelope) {
	s.mu.Lock()
	ss := s.session
	s.mu.Unlock()

	if ss != nil {
		if err := ss.HandleAnswer(env); err != nil {
			log.Printf("[screenshare] answer error: %v", err)
		}
	}
}

// HandleICE forwards a received ICE candidate to the active session.
func (s *ScreenShareService) HandleICE(env ws.Envelope) {
	s.mu.Lock()
	ss := s.session
	s.mu.Unlock()

	if ss != nil {
		if err := ss.HandleICE(env); err != nil {
			log.Printf("[screenshare] ICE error: %v", err)
		}
	}
}

// HandleReject processes a screen share rejection from peer.
func (s *ScreenShareService) HandleReject(env ws.Envelope) {
	log.Printf("[screenshare] rejected by %s", env.From)
	s.StopShare()
	s.emitter.Emit("screenshare:rejected", env.From)
}
