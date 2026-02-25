package crypto_test

import (
	"os"
	"testing"

	"umbra/client/crypto"
)

func makeTempStore(t *testing.T) *crypto.PeerStore {
	t.Helper()
	dir := t.TempDir()
	s, err := crypto.NewPeerStore(dir)
	if err != nil {
		t.Fatalf("NewPeerStore: %v", err)
	}
	return s
}

func TestGetAllPeers_EmptyStore(t *testing.T) {
	s := makeTempStore(t)
	peers := s.GetAllPeers()
	if len(peers) != 0 {
		t.Errorf("expected 0 peers, got %d", len(peers))
	}
}

func TestSetNickname_UnknownPeer(t *testing.T) {
	s := makeTempStore(t)
	err := s.SetNickname("nonexistent", "John")
	if err == nil {
		t.Error("expected error for unknown peer, got nil")
	}
}

var _ = os.DevNull // suppress unused import
