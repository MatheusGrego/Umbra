// Package crypto — peers.go
// PeerStore manages known contacts: their public keys and derived session keys.
// Persisted as a simple JSON file in ~/.umbra/peers.json.
package crypto

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

// Peer holds everything we know about a remote contact.
type Peer struct {
	UserID    string `json:"user_id"`
	EdPubKey  string `json:"ed_pub_key"` // base64 Ed25519
	X25519Key string `json:"x25519_key"` // base64 X25519
}

// PeerStore is a thread-safe, persisted contact book.
type PeerStore struct {
	mu       sync.RWMutex
	path     string
	peers    map[string]Peer       // userID → Peer
	sessions map[string]SessionKey // userID → derived AES key (in-memory only)
}

// NewPeerStore loads or creates the peer file in baseDir.
func NewPeerStore(baseDir string) (*PeerStore, error) {
	s := &PeerStore{
		path:     filepath.Join(baseDir, "peers.json"),
		peers:    make(map[string]Peer),
		sessions: make(map[string]SessionKey),
	}
	if err := s.load(); err != nil {
		return nil, err
	}
	return s, nil
}

// Add registers a new peer and derives their session key immediately.
// myX25519Private is this client's private X25519 key.
func (s *PeerStore) Add(p Peer, myX25519Private [32]byte) error {
	theirPub, err := KeyFromB64(p.X25519Key)
	if err != nil {
		return fmt.Errorf("peers: bad X25519 key for %s: %w", p.UserID, err)
	}
	sessionKey, err := DeriveSessionKey(myX25519Private, theirPub)
	if err != nil {
		return fmt.Errorf("peers: derive session: %w", err)
	}

	s.mu.Lock()
	s.peers[p.UserID] = p
	s.sessions[p.UserID] = sessionKey
	s.mu.Unlock()

	return s.save()
}

// SessionKey returns the pre-derived AES session key for a peer.
func (s *PeerStore) SessionKey(userID string) (SessionKey, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	k, ok := s.sessions[userID]
	return k, ok
}

// Get returns the peer record or false.
func (s *PeerStore) Get(userID string) (Peer, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	p, ok := s.peers[userID]
	return p, ok
}

// All returns a snapshot of all known peers.
func (s *PeerStore) All() []Peer {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]Peer, 0, len(s.peers))
	for _, p := range s.peers {
		out = append(out, p)
	}
	return out
}

// RehydrateSessions re-derives all session keys after loading from disk.
// Call once on startup with the local identity.
func (s *PeerStore) RehydrateSessions(myX25519Private [32]byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for uid, peer := range s.peers {
		pub, err := KeyFromB64(peer.X25519Key)
		if err != nil {
			return fmt.Errorf("peers: rehydrate %s: %w", uid, err)
		}
		key, err := DeriveSessionKey(myX25519Private, pub)
		if err != nil {
			return fmt.Errorf("peers: rehydrate derive %s: %w", uid, err)
		}
		s.sessions[uid] = key
	}
	return nil
}

func (s *PeerStore) load() error {
	data, err := os.ReadFile(s.path)
	if os.IsNotExist(err) {
		return nil // first run, empty store
	}
	if err != nil {
		return fmt.Errorf("peers: read file: %w", err)
	}
	return json.Unmarshal(data, &s.peers)
}

func (s *PeerStore) save() error {
	s.mu.RLock()
	data, err := json.MarshalIndent(s.peers, "", "  ")
	s.mu.RUnlock()
	if err != nil {
		return fmt.Errorf("peers: marshal: %w", err)
	}
	return os.WriteFile(s.path, data, 0600)
}
