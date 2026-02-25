// Package crypto — identity.go
// Manages the local identity: Ed25519 signing key and X25519 exchange key.
// Keys are generated once on first run and persisted in ~/.umbra/.
// They NEVER leave the device unencrypted.
package crypto

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/pem"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"golang.org/x/crypto/curve25519"
)

const (
	identityDir   = ".umbra"
	edKeyFile     = "identity.ed25519"
	x25519KeyFile = "identity.x25519"
)

// Identity holds the complete local identity for one Umbra installation.
type Identity struct {
	UserID        string // hex(SHA256(Ed25519 pubkey))[:12]
	EdPublicKey   ed25519.PublicKey
	EdPrivateKey  ed25519.PrivateKey
	X25519Public  [32]byte
	X25519Private [32]byte
}

// PublicKeyB64 returns the Ed25519 public key as base64 (for wire protocol).
func (id *Identity) PublicKeyB64() string {
	return base64.StdEncoding.EncodeToString(id.EdPublicKey)
}

// X25519PublicB64 returns the X25519 public key as base64.
func (id *Identity) X25519PublicB64() string {
	return base64.StdEncoding.EncodeToString(id.X25519Public[:])
}

// Sign signs msg with the Ed25519 private key and returns base64.
func (id *Identity) Sign(msg []byte) string {
	sig := ed25519.Sign(id.EdPrivateKey, msg)
	return base64.StdEncoding.EncodeToString(sig)
}

// IdentityStore loads or creates the local identity.
type IdentityStore struct {
	baseDir string
}

// NewIdentityStore uses ~/.umbra as the default directory.
func NewIdentityStore() *IdentityStore {
	home, _ := os.UserHomeDir()
	return &IdentityStore{baseDir: filepath.Join(home, identityDir)}
}

// NewIdentityStoreWithDir uses a custom directory for identity files.
func NewIdentityStoreWithDir(dir string) *IdentityStore {
	return &IdentityStore{baseDir: dir}
}

// Load loads an existing identity or generates a new one on first run.
func (s *IdentityStore) Load() (*Identity, error) {
	if err := os.MkdirAll(s.baseDir, 0700); err != nil {
		return nil, fmt.Errorf("identity: mkdir: %w", err)
	}

	id := &Identity{}

	edPub, edPriv, err := s.loadOrGenEd25519()
	if err != nil {
		return nil, err
	}
	id.EdPublicKey = edPub
	id.EdPrivateKey = edPriv

	x25519Priv, err := s.loadOrGenX25519()
	if err != nil {
		return nil, err
	}
	id.X25519Private = x25519Priv
	curve25519.ScalarBaseMult(&id.X25519Public, &x25519Priv)

	hash := sha256.Sum256(edPub)
	id.UserID = fmt.Sprintf("%x", hash[:6]) // 12 hex chars

	return id, nil
}

func (s *IdentityStore) loadOrGenEd25519() (ed25519.PublicKey, ed25519.PrivateKey, error) {
	path := filepath.Join(s.baseDir, edKeyFile)
	if data, err := os.ReadFile(path); err == nil {
		return decodeEd25519PEM(data)
	}
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, nil, fmt.Errorf("identity: generate Ed25519: %w", err)
	}
	if err := writeKeyFile(path, "ED25519 PRIVATE KEY", priv); err != nil {
		return nil, nil, err
	}
	return pub, priv, nil
}

func (s *IdentityStore) loadOrGenX25519() ([32]byte, error) {
	path := filepath.Join(s.baseDir, x25519KeyFile)
	if data, err := os.ReadFile(path); err == nil {
		return decode32BytePEM(data)
	}
	var priv [32]byte
	if _, err := rand.Read(priv[:]); err != nil {
		return [32]byte{}, fmt.Errorf("identity: generate X25519: %w", err)
	}
	// Clamp per RFC 7748.
	priv[0] &= 248
	priv[31] &= 127
	priv[31] |= 64

	if err := writeKeyFile(path, "X25519 PRIVATE KEY", priv[:]); err != nil {
		return [32]byte{}, err
	}
	return priv, nil
}

// ---- PEM helpers ----

func writeKeyFile(path, pemType string, data []byte) error {
	block := &pem.Block{Type: pemType, Bytes: data}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0600)
	if err != nil {
		return fmt.Errorf("identity: create key file %s: %w", path, err)
	}
	defer f.Close()
	return pem.Encode(f, block)
}

func decodeEd25519PEM(data []byte) (ed25519.PublicKey, ed25519.PrivateKey, error) {
	block, _ := pem.Decode(data)
	if block == nil {
		return nil, nil, errors.New("identity: invalid Ed25519 PEM")
	}
	if len(block.Bytes) != ed25519.PrivateKeySize {
		return nil, nil, errors.New("identity: wrong Ed25519 key size")
	}
	priv := ed25519.PrivateKey(block.Bytes)
	return priv.Public().(ed25519.PublicKey), priv, nil
}

func decode32BytePEM(data []byte) ([32]byte, error) {
	block, _ := pem.Decode(data)
	if block == nil || len(block.Bytes) != 32 {
		return [32]byte{}, errors.New("identity: invalid X25519 PEM")
	}
	var key [32]byte
	copy(key[:], block.Bytes)
	return key, nil
}
