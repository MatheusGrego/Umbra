// Package crypto — cipher.go
// Provides symmetric encryption for Umbra messages.
//
// Key derivation:
//
//	shared_secret = X25519(myPrivate, theirPublic)
//	session_key   = HKDF-SHA256(shared_secret, info="umbra-v1") → 32 bytes
//
// Encryption:
//
//	nonce      = random 12 bytes
//	ciphertext = AES-256-GCM(session_key, nonce, plaintext)
package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"io"

	"golang.org/x/crypto/curve25519"
	"golang.org/x/crypto/hkdf"
)

const (
	nonceSize = 12
	keySize   = 32
	hkdfInfo  = "umbra-v1"
)

// SessionKey is a derived 32-byte AES-256 key for a specific peer pair.
type SessionKey [keySize]byte

// DeriveSessionKey performs X25519 ECDH and HKDF to produce a shared session key.
// myPrivate and theirPublic are raw 32-byte Curve25519 keys.
func DeriveSessionKey(myPrivate, theirPublic [32]byte) (SessionKey, error) {
	shared, err := curve25519.X25519(myPrivate[:], theirPublic[:])
	if err != nil {
		return SessionKey{}, fmt.Errorf("cipher: X25519: %w", err)
	}

	hkdfReader := hkdf.New(sha256.New, shared, nil, []byte(hkdfInfo))
	var key SessionKey
	if _, err := io.ReadFull(hkdfReader, key[:]); err != nil {
		return SessionKey{}, fmt.Errorf("cipher: HKDF: %w", err)
	}
	return key, nil
}

// CiphertextPacket is the wire representation of an encrypted message.
type CiphertextPacket struct {
	Nonce string `json:"nonce"` // base64
	Data  string `json:"data"`  // base64 AES-GCM ciphertext
}

// Encrypt seals plaintext using the session key.
// Returns a CiphertextPacket ready for wire transmission.
func Encrypt(key SessionKey, plaintext []byte) (CiphertextPacket, error) {
	block, err := aes.NewCipher(key[:])
	if err != nil {
		return CiphertextPacket{}, fmt.Errorf("cipher: new AES: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return CiphertextPacket{}, fmt.Errorf("cipher: new GCM: %w", err)
	}

	nonce := make([]byte, nonceSize)
	if _, err := rand.Read(nonce); err != nil {
		return CiphertextPacket{}, fmt.Errorf("cipher: nonce: %w", err)
	}

	ciphertext := gcm.Seal(nil, nonce, plaintext, nil)
	return CiphertextPacket{
		Nonce: base64.StdEncoding.EncodeToString(nonce),
		Data:  base64.StdEncoding.EncodeToString(ciphertext),
	}, nil
}

// Decrypt opens a CiphertextPacket using the session key.
// Returns the original plaintext or an error if authentication fails.
func Decrypt(key SessionKey, pkt CiphertextPacket) ([]byte, error) {
	nonce, err := base64.StdEncoding.DecodeString(pkt.Nonce)
	if err != nil {
		return nil, fmt.Errorf("cipher: decode nonce: %w", err)
	}
	ct, err := base64.StdEncoding.DecodeString(pkt.Data)
	if err != nil {
		return nil, fmt.Errorf("cipher: decode data: %w", err)
	}

	block, err := aes.NewCipher(key[:])
	if err != nil {
		return nil, fmt.Errorf("cipher: new AES: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("cipher: new GCM: %w", err)
	}
	if len(nonce) != gcm.NonceSize() {
		return nil, errors.New("cipher: wrong nonce size")
	}

	plaintext, err := gcm.Open(nil, nonce, ct, nil)
	if err != nil {
		return nil, fmt.Errorf("cipher: decrypt: %w", err)
	}
	return plaintext, nil
}

// KeyFromB64 decodes a base64-encoded 32-byte Curve25519 key.
func KeyFromB64(b64 string) ([32]byte, error) {
	raw, err := base64.StdEncoding.DecodeString(b64)
	if err != nil || len(raw) != 32 {
		return [32]byte{}, errors.New("cipher: invalid key encoding")
	}
	var key [32]byte
	copy(key[:], raw)
	return key, nil
}
