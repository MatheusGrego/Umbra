// Package service — presence.go
// PresenceService tracks which known peers are currently online,
// handles peer_online/peer_offline events, and manages invite flow.
package service

import (
	"encoding/json"
	"log"
	"sync"

	gc "umbra/client/crypto"
	"umbra/client/ws"
)

// PresenceService manages peer online/offline state.
type PresenceService struct {
	myUserID string
	identity *gc.Identity
	peers    *gc.PeerStore
	sender   Sender
	emitter  EventEmitter

	mu     sync.RWMutex
	online map[string]bool
}

// NewPresenceService constructs a PresenceService.
func NewPresenceService(id *gc.Identity, peers *gc.PeerStore, sender Sender, emitter EventEmitter) *PresenceService {
	return &PresenceService{
		myUserID: id.UserID,
		identity: id,
		peers:    peers,
		sender:   sender,
		emitter:  emitter,
		online:   make(map[string]bool),
	}
}

// MyUserID returns the local identity's user ID.
func (s *PresenceService) MyUserID() string { return s.myUserID }

// OnlinePeers returns IDs of known peers that are currently connected.
func (s *PresenceService) OnlinePeers() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]string, 0)
	for id, up := range s.online {
		if up {
			out = append(out, id)
		}
	}
	return out
}

// HandleAuthOK seeds the initial online list from the server's auth_ok payload.
func (s *PresenceService) HandleAuthOK(env ws.Envelope) {
	var p struct {
		OnlineFriends []string `json:"online_friends"`
	}
	if err := json.Unmarshal(env.Payload, &p); err != nil {
		log.Printf("[presence] bad auth_ok payload: %v", err)
		return
	}

	s.mu.Lock()
	for _, id := range p.OnlineFriends {
		s.online[id] = true
	}
	s.mu.Unlock()

	s.emitter.Emit("presence:online_list", p.OnlineFriends)
}

// HandlePeerOnline marks a peer as online and notifies the UI.
func (s *PresenceService) HandlePeerOnline(env ws.Envelope) {
	s.mu.Lock()
	s.online[env.From] = true
	s.mu.Unlock()
	s.emitter.Emit("presence:online", env.From)
}

// HandlePeerOffline marks a peer as offline and notifies the UI.
func (s *PresenceService) HandlePeerOffline(env ws.Envelope) {
	s.mu.Lock()
	s.online[env.From] = false
	s.mu.Unlock()
	s.emitter.Emit("presence:offline", env.From)
}

// RequestInviteToken asks the server to create a one-time invite token.
func (s *PresenceService) RequestInviteToken() error {
	return s.sender.Send(ws.Envelope{
		Type: "invite_create",
		From: s.myUserID,
	})
}

// HandleInviteToken receives the token from the server and shows it in the UI.
func (s *PresenceService) HandleInviteToken(env ws.Envelope) {
	var p struct {
		Token     string `json:"token"`
		ExpiresAt int64  `json:"expires_at"`
	}
	if err := json.Unmarshal(env.Payload, &p); err != nil {
		log.Printf("[presence] bad invite_create payload: %v", err)
		return
	}
	s.emitter.Emit("invite:token", p)
}

// HandleInviteResult processes a completed friend exchange and saves the new peer.
func (s *PresenceService) HandleInviteResult(env ws.Envelope) {
	var p struct {
		UserID    string `json:"user_id"`
		PublicKey string `json:"public_key"`
		X25519Key string `json:"x25519_key"`
	}
	if err := json.Unmarshal(env.Payload, &p); err != nil {
		log.Printf("[presence] bad invite_result payload: %v", err)
		return
	}

	peer := gc.Peer{
		UserID:    p.UserID,
		EdPubKey:  p.PublicKey,
		X25519Key: p.X25519Key,
	}
	if err := s.peers.Add(peer, s.identity.X25519Private); err != nil {
		log.Printf("[presence] failed to add peer %s: %v", p.UserID, err)
		return
	}

	log.Printf("[presence] new contact added: %s", p.UserID)
	s.emitter.Emit("invite:accepted", peer)
}

// ResolveInvite sends an invite_resolve with the given token.
func (s *PresenceService) ResolveInvite(token string) error {
	payload, _ := json.Marshal(map[string]string{
		"token":      token,
		"public_key": s.identity.PublicKeyB64(),
		"x25519_key": s.identity.X25519PublicB64(),
	})
	return s.sender.Send(ws.Envelope{
		Type:    "invite_resolve",
		From:    s.myUserID,
		Payload: payload,
	})
}
