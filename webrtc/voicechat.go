// Package webrtc — voicechat.go
// VoiceChat manages a WebRTC voice call session using Opus audio.
// Like ScreenShare, the server only relays SDP/ICE — never touches media.
package webrtc

import (
	"encoding/json"
	"fmt"
	"log"

	"github.com/pion/webrtc/v3"

	"umbra/client/ws"
)

// AudioHandler is called when a remote audio track arrives.
type AudioHandler func(track *webrtc.TrackRemote)

// VoiceChat manages a single WebRTC voice call session.
type VoiceChat struct {
	myUserID   string
	peerUserID string
	sender     SignalSender
	onAudio    AudioHandler

	pc    *webrtc.PeerConnection
	muted bool
}

// NewVoiceChat constructs a VoiceChat session.
func NewVoiceChat(myUserID, peerUserID string, sender SignalSender, onAudio AudioHandler) *VoiceChat {
	return &VoiceChat{
		myUserID:   myUserID,
		peerUserID: peerUserID,
		sender:     sender,
		onAudio:    onAudio,
	}
}

// StartCaller sets up the peer connection as the caller (offerer).
func (v *VoiceChat) StartCaller() error {
	pc, err := v.newPeerConnection()
	if err != nil {
		return err
	}

	// Create an audio track for the call
	audioTrack, err := webrtc.NewTrackLocalStaticRTP(
		webrtc.RTPCodecCapability{MimeType: webrtc.MimeTypeOpus},
		"audio", "umbra-voice",
	)
	if err != nil {
		pc.Close()
		return fmt.Errorf("voicechat: create audio track: %w", err)
	}

	if _, err := pc.AddTrack(audioTrack); err != nil {
		pc.Close()
		return fmt.Errorf("voicechat: add track: %w", err)
	}

	offer, err := pc.CreateOffer(nil)
	if err != nil {
		pc.Close()
		return fmt.Errorf("voicechat: create offer: %w", err)
	}
	if err := pc.SetLocalDescription(offer); err != nil {
		pc.Close()
		return fmt.Errorf("voicechat: set local desc: %w", err)
	}

	v.pc = pc
	return v.sendSignal("voice_offer", offer)
}

// HandleOffer processes a received SDP offer (answerer side).
func (v *VoiceChat) HandleOffer(env ws.Envelope) error {
	var offer webrtc.SessionDescription
	if err := json.Unmarshal(env.Payload, &offer); err != nil {
		return fmt.Errorf("voicechat: parse offer: %w", err)
	}

	pc, err := v.newPeerConnection()
	if err != nil {
		return err
	}

	if err := pc.SetRemoteDescription(offer); err != nil {
		pc.Close()
		return fmt.Errorf("voicechat: set remote desc: %w", err)
	}

	answer, err := pc.CreateAnswer(nil)
	if err != nil {
		pc.Close()
		return fmt.Errorf("voicechat: create answer: %w", err)
	}
	if err := pc.SetLocalDescription(answer); err != nil {
		pc.Close()
		return fmt.Errorf("voicechat: set local desc (answer): %w", err)
	}

	v.pc = pc
	return v.sendSignal("voice_answer", answer)
}

// HandleAnswer processes a received SDP answer (offerer side).
func (v *VoiceChat) HandleAnswer(env ws.Envelope) error {
	if v.pc == nil {
		return fmt.Errorf("voicechat: got answer but no active peer connection")
	}
	var answer webrtc.SessionDescription
	if err := json.Unmarshal(env.Payload, &answer); err != nil {
		return fmt.Errorf("voicechat: parse answer: %w", err)
	}
	return v.pc.SetRemoteDescription(answer)
}

// HandleICE adds a received ICE candidate.
func (v *VoiceChat) HandleICE(env ws.Envelope) error {
	if v.pc == nil {
		return fmt.Errorf("voicechat: got ICE but no active peer connection")
	}
	var candidate webrtc.ICECandidateInit
	if err := json.Unmarshal(env.Payload, &candidate); err != nil {
		return fmt.Errorf("voicechat: parse ICE: %w", err)
	}
	return v.pc.AddICECandidate(candidate)
}

// Close terminates the voice call.
func (v *VoiceChat) Close() {
	if v.pc != nil {
		v.pc.Close()
		v.pc = nil
	}
}

// SendHangup sends a hangup signal to the peer.
func (v *VoiceChat) SendHangup() error {
	return v.sendSignal("voice_hangup", map[string]string{"reason": "hangup"})
}

// ---- internal ----

func (v *VoiceChat) newPeerConnection() (*webrtc.PeerConnection, error) {
	config := webrtc.Configuration{
		ICEServers: []webrtc.ICEServer{
			{URLs: []string{"stun:stun.l.google.com:19302"}},
		},
	}
	pc, err := webrtc.NewPeerConnection(config)
	if err != nil {
		return nil, fmt.Errorf("voicechat: new peer connection: %w", err)
	}

	pc.OnICECandidate(func(c *webrtc.ICECandidate) {
		if c == nil {
			return
		}
		if err := v.sendSignal("voice_ice", c.ToJSON()); err != nil {
			log.Printf("[voicechat] send ICE error: %v", err)
		}
	})

	if v.onAudio != nil {
		pc.OnTrack(func(track *webrtc.TrackRemote, _ *webrtc.RTPReceiver) {
			v.onAudio(track)
		})
	}

	pc.OnConnectionStateChange(func(state webrtc.PeerConnectionState) {
		log.Printf("[voicechat] %s → %s", v.peerUserID, state)
	})

	return pc, nil
}

func (v *VoiceChat) sendSignal(msgType string, payload any) error {
	raw, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("voicechat: marshal signal: %w", err)
	}
	return v.sender.Send(ws.Envelope{
		Type:    msgType,
		From:    v.myUserID,
		To:      v.peerUserID,
		Payload: raw,
	})
}
