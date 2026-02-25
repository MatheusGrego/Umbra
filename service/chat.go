// Package service — chat.go
// ChatService handles the messaging layer: encrypt before send, decrypt on receive.
// It depends on abstractions (Sender, EventEmitter) — zero WS or UI knowledge.
package service

import (
	"encoding/json"
	"fmt"
	"log"
	"time"

	gc "umbra/client/crypto"
	"umbra/client/ws"
)

// Sender abstracts the WS connection for sending envelopes.
type Sender interface {
	Send(env ws.Envelope) error
}

// EventEmitter pushes events to the UI layer (Wails runtime.EventsEmit).
type EventEmitter interface {
	Emit(event string, data any)
}

// InboundMessage is what the UI receives after decryption.
type InboundMessage struct {
	From      string `json:"from"`
	Plaintext string `json:"text"`
	Timestamp int64  `json:"ts"`
}

// ChatService encrypts/decrypts chat messages.
type ChatService struct {
	identity *gc.Identity
	peers    *gc.PeerStore
	sender   Sender
	emitter  EventEmitter
}

// NewChatService constructs a ChatService.
func NewChatService(id *gc.Identity, peers *gc.PeerStore, sender Sender, emitter EventEmitter) *ChatService {
	return &ChatService{identity: id, peers: peers, sender: sender, emitter: emitter}
}

// Send encrypts and dispatches a plaintext message to toUserID.
func (s *ChatService) Send(toUserID, plaintext string) error {
	key, ok := s.peers.SessionKey(toUserID)
	if !ok {
		return fmt.Errorf("chat.Send: unknown peer %s", toUserID)
	}

	pkt, err := gc.Encrypt(key, []byte(plaintext))
	if err != nil {
		return fmt.Errorf("chat.Send: encrypt: %w", err)
	}

	payload, _ := json.Marshal(pkt)
	env := ws.Envelope{
		Type:    "msg",
		From:    s.identity.UserID,
		To:      toUserID,
		Payload: payload,
	}
	return s.sender.Send(env)
}

// HandleIncoming decrypts a received message envelope and emits a UI event.
func (s *ChatService) HandleIncoming(env ws.Envelope) {
	key, ok := s.peers.SessionKey(env.From)
	if !ok {
		log.Printf("[chat] received msg from unknown peer %s — ignoring", env.From)
		return
	}

	var pkt gc.CiphertextPacket
	if err := json.Unmarshal(env.Payload, &pkt); err != nil {
		log.Printf("[chat] malformed payload from %s: %v", env.From, err)
		return
	}

	plaintext, err := gc.Decrypt(key, pkt)
	if err != nil {
		log.Printf("[chat] decrypt failed from %s: %v", env.From, err)
		return
	}

	s.emitter.Emit("chat:message", InboundMessage{
		From:      env.From,
		Plaintext: string(plaintext),
		Timestamp: time.Now().UnixMilli(),
	})
}
