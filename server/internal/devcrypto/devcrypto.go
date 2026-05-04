package devcrypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"io"

	"golang.org/x/crypto/curve25519"
	"golang.org/x/crypto/hkdf"
)

// GenerateKeyPair generates an X25519 key pair. Returns (privateB64, publicB64, error).
func GenerateKeyPair() (string, string, error) {
	var priv [32]byte
	if _, err := rand.Read(priv[:]); err != nil {
		return "", "", fmt.Errorf("generate private key: %w", err)
	}
	// Clamp per RFC 7748
	priv[0] &= 248
	priv[31] &= 127
	priv[31] |= 64

	pub, err := curve25519.X25519(priv[:], curve25519.Basepoint)
	if err != nil {
		return "", "", fmt.Errorf("derive public key: %w", err)
	}
	return base64.StdEncoding.EncodeToString(priv[:]),
		base64.StdEncoding.EncodeToString(pub), nil
}

// DeriveSharedKey computes the AES-256 symmetric key from an X25519 key pair.
// privB64 is the local private key, peerPubB64 is the remote public key (both base64).
func DeriveSharedKey(privB64, peerPubB64 string) ([]byte, error) {
	priv, err := base64.StdEncoding.DecodeString(privB64)
	if err != nil || len(priv) != 32 {
		return nil, fmt.Errorf("decode private key")
	}
	pub, err := base64.StdEncoding.DecodeString(peerPubB64)
	if err != nil || len(pub) != 32 {
		return nil, fmt.Errorf("decode public key")
	}

	shared, err := curve25519.X25519(priv, pub)
	if err != nil {
		return nil, fmt.Errorf("x25519: %w", err)
	}

	// HKDF-SHA256 to derive a 32-byte AES key
	h := hkdf.New(sha256.New, shared, nil, []byte("proidentity-device-v1"))
	key := make([]byte, 32)
	if _, err := io.ReadFull(h, key); err != nil {
		return nil, fmt.Errorf("hkdf: %w", err)
	}
	return key, nil
}

// Encrypt encrypts plaintext with AES-256-GCM.
// aad is additional authenticated data (e.g. device ID).
// Returns base64(nonce || ciphertext).
func Encrypt(key []byte, plaintext, aad []byte) (string, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return "", err
	}
	ct := gcm.Seal(nonce, nonce, plaintext, aad)
	return base64.StdEncoding.EncodeToString(ct), nil
}

// Decrypt decrypts a value produced by Encrypt.
func Decrypt(key []byte, b64ct string, aad []byte) ([]byte, error) {
	ct, err := base64.StdEncoding.DecodeString(b64ct)
	if err != nil {
		return nil, fmt.Errorf("base64 decode: %w", err)
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	ns := gcm.NonceSize()
	if len(ct) < ns {
		return nil, fmt.Errorf("ciphertext too short")
	}
	return gcm.Open(nil, ct[:ns], ct[ns:], aad)
}
