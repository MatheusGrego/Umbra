// Package service — capsule.go
// CapsuleService handles time-capsule send/receive on the client side.
package service

import (
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/google/uuid"

	gc "umbra/client/crypto"
	"umbra/client/ws"
)

// CapsuleService manages sending and receiving time capsules.
type CapsuleService struct {
	identity *gc.Identity
	peers    *gc.PeerStore
	sender   Sender
	emitter  EventEmitter
}

// NewCapsuleService constructs a CapsuleService.
func NewCapsuleService(id *gc.Identity, peers *gc.PeerStore, sender Sender, emitter EventEmitter) *CapsuleService {
	return &CapsuleService{identity: id, peers: peers, sender: sender, emitter: emitter}
}

// CapsuleReadyEvent is emitted to the UI when a capsule is available.
type CapsuleReadyEvent struct {
	ID       string `json:"id"`
	SenderID string `json:"sender_id"`
}

// CapsuleReceivedEvent is emitted after the capsule is decrypted and destroyed.
type CapsuleReceivedEvent struct {
	ID         string `json:"id"`
	From       string `json:"from"`
	Plaintext  string `json:"text"`
	ReceivedAt int64  `json:"received_at"`
}

// SendCapsulePayload mirrors the server's capsule_new payload type.
type SendCapsulePayload struct {
	ID        string `json:"id"`
	Data      string `json:"data"`
	Nonce     string `json:"nonce"`
	ReleaseIn int    `json:"release_in"` // seconds
}

// Send encrypts plaintext and dispatches a capsule with the given delay.
func (s *CapsuleService) Send(toUserID, plaintext string, releaseInSecs int) error {
	key, ok := s.peers.SessionKey(toUserID)
	if !ok {
		return fmt.Errorf("capsule.Send: unknown peer %s", toUserID)
	}

	pkt, err := gc.Encrypt(key, []byte(plaintext))
	if err != nil {
		return fmt.Errorf("capsule.Send: encrypt: %w", err)
	}

	payload, _ := json.Marshal(SendCapsulePayload{
		ID:        uuid.New().String(),
		Data:      pkt.Data,
		Nonce:     pkt.Nonce,
		ReleaseIn: releaseInSecs,
	})

	return s.sender.Send(ws.Envelope{
		Type:    "capsule_new",
		From:    s.identity.UserID,
		To:      toUserID,
		Payload: payload,
	})
}

// HandleReady is called when the server notifies that a capsule is available.
// It requests the capsule content from the server.
func (s *CapsuleService) HandleReady(env ws.Envelope) {
	var p struct {
		ID       string `json:"id"`
		SenderID string `json:"sender_id"`
	}
	if err := json.Unmarshal(env.Payload, &p); err != nil {
		log.Printf("[capsule] bad capsule_ready payload: %v", err)
		return
	}

	s.emitter.Emit("capsule:ready", CapsuleReadyEvent{ID: p.ID, SenderID: p.SenderID})

	// Auto-request: trigger read (= destruction + delivery of ciphertext).
	readPayload, _ := json.Marshal(map[string]string{"id": p.ID})
	if err := s.sender.Send(ws.Envelope{
		Type:    "capsule_read",
		From:    s.identity.UserID,
		Payload: readPayload,
	}); err != nil {
		log.Printf("[capsule] failed to send capsule_read: %v", err)
	}
}

// HandleData is called when the server delivers the capsule ciphertext after destruction.
func (s *CapsuleService) HandleData(env ws.Envelope) {
	var p struct {
		ID    string `json:"id"`
		Data  string `json:"data"`
		Nonce string `json:"nonce"`
		From  string `json:"from"`
	}
	if err := json.Unmarshal(env.Payload, &p); err != nil {
		log.Printf("[capsule] bad capsule data payload: %v", err)
		return
	}

	key, ok := s.peers.SessionKey(p.From)
	if !ok {
		log.Printf("[capsule] received capsule from unknown peer %s", p.From)
		return
	}

	plaintext, err := gc.Decrypt(key, gc.CiphertextPacket{Data: p.Data, Nonce: p.Nonce})
	if err != nil {
		log.Printf("[capsule] decrypt failed: %v", err)
		return
	}

	s.emitter.Emit("capsule:received", CapsuleReceivedEvent{
		ID:         p.ID,
		From:       p.From,
		Plaintext:  string(plaintext),
		ReceivedAt: time.Now().UnixMilli(),
	})
}
