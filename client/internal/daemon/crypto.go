package daemon

import "wg-client/internal/crypto"

func encryptAES256GCM(key, plaintext []byte) ([]byte, error) {
	return crypto.EncryptAES256GCM(key, plaintext)
}

func decryptAES256GCM(key, data []byte) ([]byte, error) {
	return crypto.DecryptAES256GCM(key, data)
}
