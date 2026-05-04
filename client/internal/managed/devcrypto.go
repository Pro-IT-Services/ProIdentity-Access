package managed

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"

	"golang.org/x/crypto/curve25519"
	"golang.org/x/crypto/hkdf"
)

// GenerateX25519KeyPair returns (privateB64, publicB64).
func GenerateX25519KeyPair() (string, string, error) {
	var priv [32]byte
	if _, err := rand.Read(priv[:]); err != nil {
		return "", "", fmt.Errorf("generate private key: %w", err)
	}
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

// DeriveAESKey derives a 32-byte AES key from an X25519 key pair.
func DeriveAESKey(privB64, peerPubB64 string) ([]byte, error) {
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
	h := hkdf.New(sha256.New, shared, nil, []byte("proidentity-device-v1"))
	key := make([]byte, 32)
	if _, err := io.ReadFull(h, key); err != nil {
		return nil, fmt.Errorf("hkdf: %w", err)
	}
	return key, nil
}

// encryptBody encrypts JSON body bytes with AES-256-GCM.
// Returns JSON: {"ct":"base64(nonce||ciphertext)"}
func encryptBody(key, plaintext []byte, aad []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, err
	}
	ct := gcm.Seal(nonce, nonce, plaintext, aad)
	env := map[string]string{"ct": base64.StdEncoding.EncodeToString(ct)}
	return json.Marshal(env)
}

// decryptBody decrypts a response envelope {"ct":"..."}.
func decryptBody(key, envelope []byte, aad []byte) ([]byte, error) {
	var env struct {
		CT string `json:"ct"`
	}
	if err := json.Unmarshal(envelope, &env); err != nil {
		return nil, fmt.Errorf("invalid envelope: %w", err)
	}
	raw, err := base64.StdEncoding.DecodeString(env.CT)
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
	if len(raw) < ns {
		return nil, fmt.Errorf("ciphertext too short")
	}
	return gcm.Open(nil, raw[:ns], raw[ns:], aad)
}
