// Package webrtc — screenshare.go
// ScreenShare manages WebRTC session lifecycle for screen sharing.
// The server is NEVER in the media path — it only relays SDP and ICE candidates.
package webrtc

import (
	"encoding/json"
	"fmt"
	"log"

	"github.com/pion/webrtc/v3"

	"umbra/client/ws"
)

// SignalSender sends WebRTC signalling messages through the existing WS connection.
type SignalSender interface {
	Send(env ws.Envelope) error
}

// TrackHandler is called when a remote video track arrives (for the receiver side).
type TrackHandler func(track *webrtc.TrackRemote)

// ScreenShare manages a single WebRTC screen-share session.
type ScreenShare struct {
	myUserID   string
	peerUserID string
	sender     SignalSender
	onTrack    TrackHandler

	pc *webrtc.PeerConnection
}

// NewScreenShare constructs a ScreenShare session between myUserID and peerUserID.
func NewScreenShare(myUserID, peerUserID string, sender SignalSender, onTrack TrackHandler) *ScreenShare {
	return &ScreenShare{
		myUserID:   myUserID,
		peerUserID: peerUserID,
		sender:     sender,
		onTrack:    onTrack,
	}
}

// StartOfferer sets up the peer connection as the offerer (screen sharer).
// Call this on the client that wants to share their screen.
func (s *ScreenShare) StartOfferer(screenTrack *webrtc.TrackLocalStaticSample) error {
	pc, err := s.newPeerConnection()
	if err != nil {
		return err
	}

	if _, err := pc.AddTrack(screenTrack); err != nil {
		pc.Close()
		return fmt.Errorf("screenshare: add track: %w", err)
	}

	offer, err := pc.CreateOffer(nil)
	if err != nil {
		pc.Close()
		return fmt.Errorf("screenshare: create offer: %w", err)
	}
	if err := pc.SetLocalDescription(offer); err != nil {
		pc.Close()
		return fmt.Errorf("screenshare: set local desc: %w", err)
	}

	s.pc = pc
	return s.sendSignal("webrtc_offer", offer)
}

// HandleOffer processes a received SDP offer (answerer side).
func (s *ScreenShare) HandleOffer(env ws.Envelope) error {
	var offer webrtc.SessionDescription
	if err := json.Unmarshal(env.Payload, &offer); err != nil {
		return fmt.Errorf("screenshare: parse offer: %w", err)
	}

	pc, err := s.newPeerConnection()
	if err != nil {
		return err
	}

	if err := pc.SetRemoteDescription(offer); err != nil {
		pc.Close()
		return fmt.Errorf("screenshare: set remote desc: %w", err)
	}

	answer, err := pc.CreateAnswer(nil)
	if err != nil {
		pc.Close()
		return fmt.Errorf("screenshare: create answer: %w", err)
	}
	if err := pc.SetLocalDescription(answer); err != nil {
		pc.Close()
		return fmt.Errorf("screenshare: set local desc (answer): %w", err)
	}

	s.pc = pc
	return s.sendSignal("webrtc_answer", answer)
}

// HandleAnswer processes a received SDP answer (offerer side).
func (s *ScreenShare) HandleAnswer(env ws.Envelope) error {
	if s.pc == nil {
		return fmt.Errorf("screenshare: got answer but no active peer connection")
	}
	var answer webrtc.SessionDescription
	if err := json.Unmarshal(env.Payload, &answer); err != nil {
		return fmt.Errorf("screenshare: parse answer: %w", err)
	}
	return s.pc.SetRemoteDescription(answer)
}

// HandleICE adds a received ICE candidate to the peer connection.
func (s *ScreenShare) HandleICE(env ws.Envelope) error {
	if s.pc == nil {
		return fmt.Errorf("screenshare: got ICE but no active peer connection")
	}
	var candidate webrtc.ICECandidateInit
	if err := json.Unmarshal(env.Payload, &candidate); err != nil {
		return fmt.Errorf("screenshare: parse ICE: %w", err)
	}
	return s.pc.AddICECandidate(candidate)
}

// Close terminates the peer connection.
func (s *ScreenShare) Close() {
	if s.pc != nil {
		s.pc.Close()
		s.pc = nil
	}
}

// ---- internal ----

func (s *ScreenShare) newPeerConnection() (*webrtc.PeerConnection, error) {
	config := webrtc.Configuration{
		ICEServers: []webrtc.ICEServer{
			{URLs: []string{"stun:stun.l.google.com:19302"}},
		},
	}
	pc, err := webrtc.NewPeerConnection(config)
	if err != nil {
		return nil, fmt.Errorf("screenshare: new peer connection: %w", err)
	}

	// Wire ICE candidate trickle.
	pc.OnICECandidate(func(c *webrtc.ICECandidate) {
		if c == nil {
			return
		}
		if err := s.sendSignal("webrtc_ice", c.ToJSON()); err != nil {
			log.Printf("[screenshare] send ICE error: %v", err)
		}
	})

	// Wire remote track handler (receiver side).
	if s.onTrack != nil {
		pc.OnTrack(func(track *webrtc.TrackRemote, _ *webrtc.RTPReceiver) {
			s.onTrack(track)
		})
	}

	pc.OnConnectionStateChange(func(state webrtc.PeerConnectionState) {
		log.Printf("[screenshare] %s → %s", s.peerUserID, state)
	})

	return pc, nil
}

func (s *ScreenShare) sendSignal(msgType string, payload any) error {
	raw, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("screenshare: marshal signal: %w", err)
	}
	return s.sender.Send(ws.Envelope{
		Type:    msgType,
		From:    s.myUserID,
		To:      s.peerUserID,
		Payload: raw,
	})
}
